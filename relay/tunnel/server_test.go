package tunnel

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"zyrln/relay/core"
)

func TestConnectHost(t *testing.T) {
	if got := core.HostFromConnectTarget("www.google.com:443"); got != "www.google.com" {
		t.Fatalf("HostFromConnectTarget = %q", got)
	}
}

func TestTunnelConnectRoute(t *testing.T) {
	core.SetDirectEnabled(true)
	defer core.SetDirectEnabled(true)

	if got := core.ConnectRouteForHost("shop.example.ir"); got != core.RouteDomesticPlain {
		t.Fatalf(".ir = %v, want domestic", got)
	}
	if got := core.ConnectRouteForHost("www.google.com"); got != core.RouteDirectFragment {
		t.Fatalf("google = %v, want direct", got)
	}
	if got := core.ConnectRouteForHost("www.youtube.com"); got != core.RouteRelay {
		t.Fatalf("youtube = %v, want relay", got)
	}
}

func TestTunnelProxyPlainHTTP(t *testing.T) {
	stack := newTunnelTestStack(t)
	defer stack.Close()
	proxy, err := StartTunnelProxy("127.0.0.1:0", []string{stack.appsURL}, stack.front, "k", stack.client, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer StopActiveTunnel()
	listener := httptest.NewServer(proxy.Handler)
	defer listener.Close()

	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || r.RequestURI != "/demo%2Fpath?query=1" || r.Header.Get("Proxy-Authorization") != "" || r.Header.Get("X-Hop") != "" {
			http.Error(w, "bad forwarded request", http.StatusBadRequest)
			return
		}
		w.Header().Set("X-Upstream", "seen")
		w.Header().Set("Keep-Alive", "timeout=3")
		w.WriteHeader(http.StatusAccepted)
		io.WriteString(w, r.Method+":"+string(body))
	}))
	defer origin.Close()

	local, err := url.Parse(listener.URL)
	if err != nil {
		t.Fatal(err)
	}
	transport := &http.Transport{Proxy: http.ProxyURL(local)}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	for _, tc := range []struct{ method, body string }{{"GET", ""}, {"POST", "payload"}} {
		t.Run(tc.method, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, origin.URL+"/demo%2Fpath?query=1", strings.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Proxy-Authorization", "must not be forwarded")
			req.Header.Set("Connection", "X-Hop")
			req.Header.Set("X-Hop", "must not be forwarded")
			resp, err := client.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil || resp.StatusCode != http.StatusAccepted || string(body) != tc.method+":"+tc.body ||
				resp.Header.Get("X-Upstream") != "seen" || resp.Header.Get("Keep-Alive") != "" {
				t.Fatalf("unexpected response: %s %q %v (%v)", resp.Status, body, resp.Header, err)
			}
		})
	}
}

func TestTunnelProxyRejectsInvalidPlainHTTP(t *testing.T) {
	for _, target := range []string{"/relative", "https://example.com/", "http://user:secret@example.com/"} {
		w := httptest.NewRecorder()
		handleTunnelHTTP(w, httptest.NewRequest("GET", target, nil), &tunnelBundle{})
		if w.Code != http.StatusBadRequest {
			t.Errorf("%q: got %d, want 400", target, w.Code)
		}
	}
	w := httptest.NewRecorder()
	handleTunnelHTTP(w, httptest.NewRequest("GET", "http://example.invalid/", nil), &tunnelBundle{})
	if w.Code != http.StatusBadGateway {
		t.Errorf("without configured relay: got %d, want 502", w.Code)
	}
}
