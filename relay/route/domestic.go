package route

import (
	"context"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"zyrln/relay/log"
	"zyrln/relay/netdial"
)

// SetDomesticBypassEnabled is a no-op kept for API compatibility; domestic bypass is always on.
func SetDomesticBypassEnabled(bool) {}

// GetDomesticBypassEnabled always returns true (domestic bypass cannot be disabled).
func GetDomesticBypassEnabled() bool { return true }

var (
	domesticRules   atomic.Pointer[domesticMatcher]
	domesticRefresh sync.Mutex
	domesticOnce    sync.Once
)

type domesticMatcher struct {
	// roots: domain entries from the bundled list (e.g. digikala.com).
	// A host matches if it equals a root or is a subdomain (www.digikala.com).
	roots map[string]struct{}
}

func (m *domesticMatcher) addRoot(domain string) {
	if m.roots == nil {
		m.roots = make(map[string]struct{})
	}
	h := normalizeHost(domain)
	if h != "" {
		m.roots[h] = struct{}{}
	}
}

func (m *domesticMatcher) matchHost(host string) bool {
	h := normalizeHost(host)
	if h == "" {
		return false
	}
	if matchIRTLD(h) {
		return true
	}
	if m == nil || len(m.roots) == 0 {
		return false
	}
	for _, name := range parentDomains(h) {
		if _, ok := m.roots[name]; ok {
			return true
		}
	}
	return false
}

// parentDomains returns the host and registrable parent names (not bare TLDs).
// e.g. www.digikala.com → [www.digikala.com, digikala.com]
// .ir hosts are handled separately by matchIRTLD.
func parentDomains(host string) []string {
	labels := strings.Split(host, ".")
	if len(labels) < 2 {
		return []string{host}
	}
	out := make([]string, 0, len(labels)-1)
	for i := 0; i < len(labels)-1; i++ {
		out = append(out, strings.Join(labels[i:], "."))
	}
	return out
}

func matchIRTLD(host string) bool {
	return host == "ir" || strings.HasSuffix(host, ".ir") ||
		strings.HasSuffix(host, ".xn--mgba3a4f16a") // .ir punycode
}

func normalizeHost(host string) string {
	h := strings.TrimSpace(strings.ToLower(host))
	if h == "" {
		return ""
	}
	if strings.Contains(h, ":") {
		if name, _, err := net.SplitHostPort(h); err == nil {
			h = name
		}
	}
	return strings.TrimSuffix(h, ".")
}

func ensureDomesticRefresh() {
	domesticOnce.Do(reloadDomesticRules)
}

// EnsureDomesticRules loads the bundled domain list once (call at proxy startup).
func EnsureDomesticRules() {
	ensureDomesticRefresh()
}

func reloadDomesticRules() {
	domesticRefresh.Lock()
	defer domesticRefresh.Unlock()
	if err := loadBundledDomesticRules(); err != nil {
		log.Logf("error", "domestic rules: %v", err)
		if domesticRules.Load() == nil {
			domesticRules.Store(&domesticMatcher{})
		}
		return
	}
	n := 0
	if m := domesticRules.Load(); m != nil {
		n = len(m.roots)
	}
	log.Logf("system", "domestic bypass: %d domains (bundled)", n)
}

// ShouldUseDomesticBypass reports whether host should skip relay/tunnel and dial locally.
// Always active when rules match (.ir and bundled domain list). Google domains are excluded.
func ShouldUseDomesticBypass(host string) bool {
	ensureDomesticRefresh()
	h := normalizeHost(host)
	if h == "" {
		return false
	}
	if IsGoogleDomain(h) {
		return false
	}
	if m := domesticRules.Load(); m != nil {
		return m.matchHost(h)
	}
	return matchIRTLD(h)
}

// HandlePlainConnect dials the target directly (protected on Android) without relay or tunnel.
func HandlePlainConnect(clientConn net.Conn, targetHost string) {
	handlePlainConnect(clientConn, targetHost)
}

func dialProtectedTCP(targetHost string) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return netdial.ProtectedDialer(15 * time.Second).DialContext(ctx, "tcp", targetHost)
}

// DialProtectedTCPConn dials via the protected dialer (VpnService.protect on Android).
func DialProtectedTCPConn(targetHost string) (net.Conn, bool) {
	c, err := dialProtectedTCP(targetHost)
	if err != nil {
		return nil, false
	}
	return c, true
}

// handlePlainConnect dials the target directly (protected on Android) without relay or TLS fragmentation.
func handlePlainConnect(clientConn net.Conn, targetHost string) {
	serverConn, err := dialProtectedTCP(targetHost)
	if err != nil {
		log.Logf("error", "domestic %s: %v", targetHost, err)
		_, _ = clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		return
	}
	defer serverConn.Close()
	_, _ = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	log.Logf("debug", "domestic %s", targetHost)
	pipe(clientConn, serverConn)
}

// DialPlainDirect dials a domestic target without relay or tunnel (protected on Android).
func DialPlainDirect(targetHost string) (net.Conn, bool) {
	conn, err := dialProtectedTCP(targetHost)
	if err != nil {
		log.Logf("error", "domestic %s: %v", targetHost, err)
		return nil, false
	}
	log.Logf("debug", "domestic %s", targetHost)
	return conn, true
}
