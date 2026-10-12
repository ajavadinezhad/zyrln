// Package core is the stable import path for desktop and Android (gomobile).
// Implementation is split across relay/route, relay/appscript, relay/mitm, and helpers.
package core

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"zyrln/relay/appscript"
	"zyrln/relay/conn"
	"zyrln/relay/log"
	"zyrln/relay/mitm"
	"zyrln/relay/netdial"
	"zyrln/relay/route"
)

// Re-exported types used by platforms and tunnel.
type (
	Coalescer         = appscript.Coalescer
	RelayResponse     = appscript.RelayResponse
	CertAuthority     = mitm.CertAuthority
	SOCKSServer       = mitm.SOCKSServer
	ConnectRoute      = route.ConnectRoute
	DirectProbeReport = route.DirectProbeReport
	BufferedConn      = conn.BufferedConn
)

const (
	MaxProxyRequestBody = appscript.MaxProxyRequestBody
	RouteDirectFragment = route.RouteDirectFragment
	RouteDomesticPlain  = route.RouteDomesticPlain
	RouteRelay          = route.RouteRelay
)

var DirectFronts = route.DirectFronts

func ActiveURLIdxLoad() int64 { return appscript.ActiveURLIdx.Load() }
func ActiveURLIdxStore(v int64) { appscript.ActiveURLIdx.Store(v) }

func SetOnRequest(f func(method, url string)) { log.OnRequest = f }
func SetLogFunc(f func(level, msg string))    { log.SetLogFunc(f) }
func Log(level, format string, args ...any)    { log.Log(level, format, args...) }

func SetSocketProtectFunc(fn func(fd int) error) { netdial.SetSocketProtectFunc(fn) }
func SetDirectEnabled(v bool)              { route.SetDirectEnabled(v) }
func GetDirectEnabled() bool               { return route.GetDirectEnabled() }
func SetCacheDir(dir string)               { route.SetCacheDir(dir) }

func ParseURLList(raw string) []string                      { return appscript.ParseURLList(raw) }
func NewHTTPClient(timeout time.Duration) *http.Client      { return appscript.NewHTTPClient(timeout) }
func GenerateCA(certPath, keyPath string) error             { return mitm.GenerateCA(certPath, keyPath) }
func LoadCA(certPath, keyPath string) (*CertAuthority, error) { return mitm.LoadCA(certPath, keyPath) }

func HostFromConnectTarget(hostport string) string            { return route.HostFromConnectTarget(hostport) }
func ConnectRouteForHost(host string) ConnectRoute            { return route.ConnectRouteForHost(host) }
func ApplyBypassConnect(clientConn net.Conn, targetHostport string, r ConnectRoute) bool {
	return route.ApplyBypassConnect(clientConn, targetHostport, r)
}
func ConnFromReadWriter(rw io.ReadWriter) net.Conn { return route.ConnFromReadWriter(rw) }

func IsGoogleDomain(host string) bool          { return route.IsGoogleDomain(host) }
func ShouldUseDomesticBypass(host string) bool { return route.ShouldUseDomesticBypass(host) }
func EnsureDomesticRules()                     { route.EnsureDomesticRules() }
func DialFragment(addr string) (net.Conn, bool) { return route.DialFragment(addr) }

func DirectProfiles() []route.DirectProfile { return route.DirectProfiles() }
func ProbeDirectProfiles(ctx context.Context, targets, fronts []string, repeat int, timeout time.Duration) DirectProbeReport {
	return route.ProbeDirectProfiles(ctx, targets, fronts, repeat, timeout)
}

func RelayRequestMulti(client *http.Client, appScriptURLs []string, frontDomain, authKey, method, targetURL string, headers map[string]string, body []byte, timeout time.Duration) (RelayResponse, error) {
	return appscript.RelayRequestMulti(client, appScriptURLs, frontDomain, authKey, method, targetURL, headers, body, timeout)
}

func AppsScriptRoundTrip(ctx context.Context, client *http.Client, appScriptURL, frontDomain, payload string, timeout time.Duration) ([]byte, error) {
	return appscript.AppsScriptRoundTrip(ctx, client, appScriptURL, frontDomain, payload, timeout)
}
func BuildRelayPayload(authKey, method, targetURL string, headers map[string]string, body []byte) string {
	return appscript.BuildRelayPayload(authKey, method, targetURL, headers, body)
}
func TryOneURL(ctx context.Context, client *http.Client, appScriptURL, frontDomain, payload string, timeout time.Duration) (RelayResponse, error) {
	return appscript.TryOneURL(ctx, client, appScriptURL, frontDomain, payload, timeout)
}
func PreviewBytes(b []byte, max int) string { return appscript.PreviewBytes(b, max) }
func PerURLTimeout(total time.Duration, n int) time.Duration {
	return appscript.PerURLTimeout(total, n)
}

func ServeProxyWithSOCKS(httpListenAddr, socksListenAddr string, appScriptURLs []string, frontDomain, authKey string, ca *CertAuthority, client *http.Client, timeout time.Duration) error {
	return mitm.ServeProxyWithSOCKS(httpListenAddr, socksListenAddr, appScriptURLs, frontDomain, authKey, ca, client, timeout)
}
func StartDirectProxy(listenAddr string) *http.Server { return mitm.StartDirectProxy(listenAddr) }
func StartProxyWithSOCKSAndCoalescer(httpListenAddr, socksListenAddr string, appScriptURLs []string, frontDomain, authKey string, ca *CertAuthority, client *http.Client, timeout time.Duration) (*http.Server, net.Listener, *SOCKSServer, net.Listener, *Coalescer, error) {
	return mitm.StartProxyWithSOCKSAndCoalescer(httpListenAddr, socksListenAddr, appScriptURLs, frontDomain, authKey, ca, client, timeout)
}
