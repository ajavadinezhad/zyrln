package tunnel

import (
	"context"
	"io"
	"net/http"
	"sync/atomic"
	"time"

	"zyrln/relay/core"
)

// tunnelBundle holds the raw TCP tunnel client for Android VPN mode.
type tunnelBundle struct {
	tunnel *TunnelClient
}

var activeTunnel atomic.Pointer[TunnelClient]

// ActiveTunnelClient returns the tunnel client used by the running proxy, if any.
func ActiveTunnelClient() *TunnelClient {
	return activeTunnel.Load()
}

// StopActiveTunnel stops keepalive on the proxy's tunnel client.
func StopActiveTunnel() {
	if t := activeTunnel.Swap(nil); t != nil {
		t.Stop()
	}
	StopPrewarm()
}

func newTunnelBundle(appScriptURLs []string, frontDomain, authKey string, client *http.Client, timeout time.Duration) (*tunnelBundle, error) {
	core.EnsureDomesticRules()
	pb := &tunnelBundle{}
	if len(appScriptURLs) > 0 {
		pb.tunnel = adoptOrCreateTunnelClient(client, appScriptURLs, frontDomain, authKey, timeout)
		activeTunnel.Store(pb.tunnel)
	}
	return pb, nil
}

// StartTunnelProxy builds the local HTTP CONNECT proxy backed by the raw TCP tunnel.
func StartTunnelProxy(listenAddr string, appScriptURLs []string, frontDomain, authKey string, client *http.Client, timeout time.Duration) (*http.Server, error) {
	pb, err := newTunnelBundle(appScriptURLs, frontDomain, authKey, client, timeout)
	if err != nil {
		return nil, err
	}
	return buildTunnelHTTPProxyServer(listenAddr, pb), nil
}

func buildTunnelHTTPProxyServer(listenAddr string, pb *tunnelBundle) *http.Server {
	return &http.Server{
		Addr: listenAddr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodConnect {
				handleTunnelConnect(w, r, pb)
			} else {
				handleTunnelHTTP(w, r, pb)
			}
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}
}

func handleTunnelHTTP(w http.ResponseWriter, r *http.Request, pb *tunnelBundle) {
	if pb.tunnel == nil {
		http.Error(w, "no tunnel configured", http.StatusBadGateway)
		return
	}
	http.Error(w, "plain HTTP requires HTTPS CONNECT tunnel", http.StatusBadGateway)
}

func handleTunnelConnect(w http.ResponseWriter, r *http.Request, pb *tunnelBundle) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}

	rawConn, rw, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer rawConn.Close()
	local := &core.BufferedConn{Conn: rawConn, Reader: rw.Reader}
	handleRelayTunnelConnect(local, r.Host, pb)
}

func handleRelayTunnelConnect(local io.ReadWriter, targetHost string, pb *tunnelBundle) {
	target := NormalizeHostPort(targetHost, "443")
	host := core.HostFromConnectTarget(targetHost)

	if c := core.ConnFromReadWriter(local); c != nil {
		if core.ApplyBypassConnect(c, target, core.ConnectRouteForHost(host)) {
			return
		}
	}
	if pb.tunnel == nil {
		if c := core.ConnFromReadWriter(local); c != nil {
			_, _ = c.Write([]byte("HTTP/1.1 502 No tunnel configured\r\n\r\n"))
		}
		return
	}
	if core.IsGoogleDomain(host) && !core.GetDirectEnabled() {
		core.Log("debug", "tunnel CONNECT %s (direct bypass off)", target)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pb.tunnel.waitWarmupBeforeFirstConnect(ctx)

	sess, err := pb.tunnel.OpenSession(ctx, target)
	if err != nil {
		if c := core.ConnFromReadWriter(local); c != nil {
			_, _ = c.Write([]byte("HTTP/1.1 502 Bad Gateway\r\nConnection: close\r\n\r\n"))
		}
		core.Log("error", "tunnel CONNECT session %s: %v", target, err)
		return
	}
	if c := core.ConnFromReadWriter(local); c != nil {
		_, _ = c.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	}
	core.Log("debug", "tunnel CONNECT %s", target)
	RunTunnelBridge(ctx, local, sess, target, pb.tunnel.timeout)
}
