package mitm

import (
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
	"zyrln/relay/appscript"
	"zyrln/relay/route"
)

func TestDesktopRelayStallTimesOutAndRecovers(t *testing.T) {
	old := route.GetDirectEnabled()
	route.SetDirectEnabled(false)
	defer route.SetDirectEnabled(old)
	var stall atomic.Bool
	stall.Store(true)
	release := make(chan struct{})
	exit := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if stall.Load() {
			select {
			case <-r.Context().Done():
			case <-release:
			}
			return
		}
		fmt.Fprintf(w, `{"s":200,"h":{"content-type":["text/plain"]},"b":"%s"}`, base64.StdEncoding.EncodeToString([]byte("recovered")))
	}))
	defer exit.Close()
	defer close(release)
	client := exit.Client()
	client.Timeout = 250 * time.Millisecond
	coal := appscript.NewCoalescer(client, []string{exit.URL}, exit.Listener.Addr().String(), "test-key", 250*time.Millisecond)
	defer coal.Stop()
	proxy := httptest.NewServer(buildHTTPProxyServer("", coal, nil).Handler)
	defer proxy.Close()
	proxyURL, _ := url.Parse(proxy.URL)
	c := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: 3 * time.Second}
	defer c.CloseIdleConnections()
	started := time.Now()
	resp, err := c.Get("http://example.org/stalled")
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 502 {
		t.Fatalf("stall status=%d, want 502", resp.StatusCode)
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("relay stall exceeded timeout budget")
	}
	stall.Store(false)
	resp, err = c.Get("http://example.org/recovered")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "recovered" {
		t.Fatalf("recovery=%d %q", resp.StatusCode, body)
	}
}

func TestDesktopStalledUploadDoesNotBlockOtherRequestsAndClose(t *testing.T) {
	old := route.GetDirectEnabled()
	route.SetDirectEnabled(false)
	defer route.SetDirectEnabled(old)
	exit := fakeAppScript(t, "healthy", 200)
	defer exit.Close()
	coal := fakeCoalescer(t, exit)
	defer coal.Stop()
	srv := buildHTTPProxyServer("", coal, nil)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go srv.Serve(ln)
	defer srv.Close()
	blocked, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer blocked.Close()
	fmt.Fprint(blocked, "POST http://example.org/upload HTTP/1.1\r\nHost: example.org\r\nContent-Length: 100000\r\n\r\nx")
	proxyURL, _ := url.Parse("http://" + ln.Addr().String())
	c := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: 3 * time.Second}
	defer c.CloseIdleConnections()
	resp, err := c.Get("http://example.org/healthy")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "healthy" {
		t.Fatalf("unrelated request=%d %q", resp.StatusCode, body)
	}
	started := time.Now()
	if err = srv.Close(); err != nil {
		t.Fatal(err)
	}
	if time.Since(started) > time.Second {
		t.Fatal("close blocked on stalled upload")
	}
	blocked.SetReadDeadline(time.Now().Add(time.Second))
	var b [1]byte
	if _, err = blocked.Read(b[:]); err == nil {
		t.Fatal("stalled socket remained open after close")
	} else if e, ok := err.(net.Error); ok && e.Timeout() {
		t.Fatal("stalled socket timed out rather than closing")
	}
}
