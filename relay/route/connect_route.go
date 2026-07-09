package route

import (
	"io"
	"net"
	"strings"

	"zyrln/relay/conn"
	"zyrln/relay/log"
)

// ConnectRoute is how an HTTPS CONNECT target should leave the local proxy.
// Relay (MITM/coalescer) vs tunnel (Apps Script) is chosen by the caller after bypass routes.
type ConnectRoute int

const (
	RouteDirectFragment ConnectRoute = iota // Google + direct enabled → TLS fragmentation
	RouteDomesticPlain                      // .ir / bundled list → protected plain TCP
	RouteRelay                              // foreign / YouTube / etc. → relay or tunnel
)

// HostFromConnectTarget returns the hostname from a CONNECT host:port (or bare host).
func HostFromConnectTarget(hostport string) string {
	hostport = strings.TrimSpace(hostport)
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return hostport
	}
	return host
}

// ConnectRouteForHost picks direct fragment, domestic plain, or relay/tunnel.
// Order: Google direct (when enabled) → domestic bypass (always) → relay/tunnel.
func ConnectRouteForHost(host string) ConnectRoute {
	if IsDirectDomain(host) {
		return RouteDirectFragment
	}
	if ShouldUseDomesticBypass(host) {
		return RouteDomesticPlain
	}
	return RouteRelay
}

// ApplyBypassConnect handles RouteDirectFragment and RouteDomesticPlain on a hijacked CONNECT.
// Returns true if the client connection is fully handled (caller should not tunnel/relay).
func ApplyBypassConnect(clientConn net.Conn, targetHostport string, route ConnectRoute) bool {
	switch route {
	case RouteDirectFragment:
		log.Log("debug", "direct CONNECT %s", targetHostport)
		HandleDirectConnect(clientConn, targetHostport)
		return true
	case RouteDomesticPlain:
		log.Log("debug", "domestic CONNECT %s", targetHostport)
		HandlePlainConnect(clientConn, targetHostport)
		return true
	default:
		return false
	}
}

// ConnFromReadWriter returns a net.Conn when rw is or wraps one (e.g. after HTTP hijack).
func ConnFromReadWriter(rw io.ReadWriter) net.Conn {
	if c, ok := rw.(net.Conn); ok {
		return c
	}
	if bc, ok := rw.(*conn.BufferedConn); ok && bc.Conn != nil {
		return bc
	}
	return nil
}
