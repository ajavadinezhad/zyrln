// Package mobile exposes the relay proxy for gomobile binding.
package mobile

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"syscall"
	"time"

	"zyrln/relay/core"
	"zyrln/relay/tun"
	"zyrln/relay/tunnel"
)

func init() {
	// Raise the open-file limit so the proxy can handle many concurrent
	// connections without hitting the default Android per-process limit.
	var rl syscall.Rlimit
	if syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rl) == nil {
		rl.Cur = rl.Max
		syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rl)
	}
}

const (
	defaultFrontDomain = "www.google.com"
	defaultTimeout     = 45 * time.Second
	maxLogEntries      = 200
)

// LogEntry is a single log line returned by PollLogs.
type LogEntry struct {
	Level   string // "info" | "error" | "system"
	Message string
}

var (
	mu       sync.Mutex
	server   *http.Server
	listener net.Listener
	tunEng   *tun.Engine
	lastErr  string

	logMu      sync.Mutex
	logBuf     []LogEntry
	logSeq     int // increments with every new entry
	logReadSeq int // last seq returned to caller
)

func emitLog(level, msg string) {
	logMu.Lock()
	entry := LogEntry{Level: level, Message: msg}
	logBuf = append(logBuf, entry)
	if len(logBuf) > maxLogEntries {
		logBuf = logBuf[1:]
	}
	logSeq++
	logMu.Unlock()
}

// PollLogs returns all log entries added since the last call.
// Returns newline-separated "level\tmessage" strings, or "" if none.
func PollLogs() string {
	logMu.Lock()
	defer logMu.Unlock()
	if logSeq == logReadSeq {
		return ""
	}
	total := len(logBuf)
	if total == 0 {
		logReadSeq = logSeq
		return ""
	}
	missed := logSeq - logReadSeq
	// If the buffer wrapped and we missed more than it holds, return what we have
	if missed > total {
		missed = total
	}
	start := total - missed
	var sb strings.Builder
	for _, e := range logBuf[start:] {
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(e.Level)
		sb.WriteByte('\t')
		sb.WriteString(e.Message)
	}
	logReadSeq = logSeq
	return sb.String()
}

// StartDirect starts direct-only mode: fragmented TLS to Google, plain pipe for others.
func StartDirect(listenAddr string) string {
	mu.Lock()
	defer mu.Unlock()

	if server != nil {
		return ""
	}

	core.SetLogFunc(func(level, msg string) { emitLog(level, msg) })
	core.SetDirectEnabled(true)

	srv := core.StartDirectProxy(listenAddr)
	if err := bindAndServe(srv, listenAddr); err != nil {
		_ = srv.Close()
		lastErr = err.Error()
		emitLog("error", lastErr)
		return lastErr
	}

	lastErr = ""
	emitLog("system", fmt.Sprintf("Direct proxy started on %s", listenAddr))
	return ""
}

// WarmupTunnel pre-warms Apps Script before Connect (call when user selects a profile).
func WarmupTunnel(appScriptURL, authKey string) {
	urls := parseURLList(appScriptURL)
	if len(urls) == 0 || strings.TrimSpace(authKey) == "" {
		return
	}
	if tc := tunnel.ActiveTunnelClient(); tc != nil {
		tc.Warmup()
		return
	}
	tunnel.Prewarm(core.NewHTTPClient(defaultTimeout), urls, defaultFrontDomain, authKey, defaultTimeout)
}

// StartTunnel starts the local HTTP CONNECT proxy backed by raw TCP-over-HTTP (no CA/MITM).
func StartTunnel(appScriptURL, authKey, listenAddr string) string {
	mu.Lock()
	defer mu.Unlock()

	if server != nil {
		return ""
	}

	urls := parseURLList(appScriptURL)
	if len(urls) == 0 {
		lastErr = "no relay URL configured"
		emitLog("error", lastErr)
		return lastErr
	}
	if strings.TrimSpace(authKey) == "" {
		lastErr = "auth key required"
		emitLog("error", lastErr)
		return lastErr
	}

	core.SetLogFunc(func(level, msg string) { emitLog(level, msg) })

	client := core.NewHTTPClient(defaultTimeout)
	srv, err := tunnel.StartTunnelProxy(listenAddr, urls, defaultFrontDomain, authKey, client, defaultTimeout)
	if err != nil {
		lastErr = err.Error()
		emitLog("error", lastErr)
		return lastErr
	}

	if err := bindAndServe(srv, listenAddr); err != nil {
		_ = srv.Close()
		tunnel.StopActiveTunnel()
		lastErr = err.Error()
		emitLog("error", lastErr)
		return lastErr
	}

	lastErr = ""
	emitLog("system", fmt.Sprintf("Tunnel proxy started on %s", listenAddr))
	return ""
}

func bindAndServe(srv *http.Server, listenAddr string) error {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	srv.ErrorLog = log.New(&logWriter{}, "", 0)
	go func() { _ = srv.Serve(ln) }()
	server = srv
	listener = ln
	return nil
}

type logWriter struct{}

func (lw *logWriter) Write(p []byte) (int, error) {
	msg := strings.TrimSpace(string(p))
	if msg != "" {
		emitLog("error", msg)
	}
	return len(p), nil
}

// GetAllLogs returns the entire log buffer including debug entries, for use in
// bug reports. Unlike PollLogs it does not advance the read cursor.
func GetAllLogs() string {
	logMu.Lock()
	defer logMu.Unlock()
	var sb strings.Builder
	for _, e := range logBuf {
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(e.Level)
		sb.WriteByte('\t')
		sb.WriteString(e.Message)
	}
	return sb.String()
}

func parseURLList(raw string) []string {
	return core.ParseURLList(raw)
}

// Ping measures relay latency via the tunnel path.
func Ping(appScriptURL, authKey string) string {
	urls := parseURLList(appScriptURL)
	if len(urls) == 0 {
		return "error: no relay URL configured"
	}
	tc := tunnel.ActiveTunnelClient()
	owned := false
	if tc == nil {
		tc = tunnel.NewTunnelClient(core.NewHTTPClient(defaultTimeout), urls, defaultFrontDomain, authKey, defaultTimeout)
		owned = true
	}
	if owned {
		defer tc.Stop()
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	d, err := tc.PingLatency(ctx)
	if err != nil {
		return "error: " + err.Error()
	}
	return fmt.Sprintf("%d ms", d.Milliseconds())
}

// Stop shuts down the relay proxy.
// Close the listener first so IsRunning() clears immediately on Android disconnect.
func Stop() {
	mu.Lock()
	defer mu.Unlock()
	if listener != nil {
		_ = listener.Close()
		listener = nil
	}
	if server != nil {
		_ = server.Close()
		server = nil
	}
	tunnel.StopActiveTunnel()
	stopTUNLocked()
	emitLog("system", "Proxy stopped")
}

// IsRunning returns true if the proxy is currently running.
func IsRunning() bool {
	mu.Lock()
	defer mu.Unlock()
	return server != nil
}

// LastError returns the last error from StartTunnel/StartDirect, or "" if none.
func LastError() string {
	mu.Lock()
	defer mu.Unlock()
	return lastErr
}

// SetCacheDir sets the directory used to persist the remembered direct profile
// across app restarts. Call once at startup with the app's files directory.
func SetCacheDir(dir string) {
	core.SetCacheDir(dir)
}

// SetDirectEnabled controls whether Google domains bypass the relay via TLS
// fragmentation. Enabled by default. Safe to call at any time.
func SetDirectEnabled(enabled bool) {
	core.SetDirectEnabled(enabled)
}

// IsDirectEnabled returns the current state of the direct-mode flag.
func IsDirectEnabled() bool {
	return core.GetDirectEnabled()
}

// PingDirect measures a fragmented direct connection to a Google front.
// This exercises the same path live traffic uses: TCP to an open Google domain
// with a fragmented ClientHello to bypass SNI inspection.
// Returns round-trip time as "142 ms" or an error string prefixed with "error: ".
func PingDirect() string {
	start := time.Now()
	conn, ok := core.DialFragment("www.youtube.com:443")
	if !ok {
		return "error: direct connection failed"
	}
	conn.Close()
	return fmt.Sprintf("%d ms", time.Since(start).Milliseconds())
}

// SocketProtector is implemented by the Android VpnService to protect sockets
// from being routed through the VPN tunnel.
type SocketProtector interface {
	Protect(fd int64) bool
}

// SetSocketProtector registers the VPN socket protector so relay HTTP client
// sockets bypass the TUN and don't loop back through the local proxy.
// Call this before StartTunnel/StartDirect. Pass nil to clear.
func SetSocketProtector(p SocketProtector) {
	if p == nil {
		core.SetSocketProtectFunc(nil)
		return
	}
	core.SetSocketProtectFunc(func(fd int) { p.Protect(int64(fd)) })
}

func stopTUNLocked() {
	if tunEng != nil {
		tunEng.Stop()
		tunEng = nil
	}
}

// AttachTUN starts the TUN TCP forwarder on an Android VpnService fd.
// Call after StartTunnel/StartDirect and VPN establish().
func AttachTUN(tunFD int64) string {
	mu.Lock()
	defer mu.Unlock()
	stopTUNLocked()
	if server == nil {
		lastErr = "proxy not running"
		return lastErr
	}
	cfg := tun.Config{
		Tunnel:     tunnel.ActiveTunnelClient(),
		DirectOnly: tunnel.ActiveTunnelClient() == nil,
		Timeout:    defaultTimeout,
	}
	eng, err := tun.Start(int(tunFD), cfg)
	if err != nil {
		lastErr = err.Error()
		emitLog("error", lastErr)
		return lastErr
	}
	tunEng = eng
	emitLog("system", "TUN TCP forwarder started")
	return ""
}
