package tunnel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTunnelClient_OpenWriteReadClose(t *testing.T) {
	var mu sync.Mutex
	sessions := map[string][]byte{}

	apps := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var env TunnelEnvelope
		if err := json.Unmarshal(body, &env); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if env.Key != "secret" {
			http.Error(w, "bad key", http.StatusUnauthorized)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		switch env.Req.Op {
		case TunnelOpOpen:
			sessions[env.Req.ID] = nil
			_ = json.NewEncoder(w).Encode(TunnelResponse{OK: true})
		case TunnelOpTX:
			data, err := base64.StdEncoding.DecodeString(env.Req.Data)
			if err != nil {
				_ = json.NewEncoder(w).Encode(TunnelResponse{Error: "bad base64"})
				return
			}
			sessions[env.Req.ID] = append(sessions[env.Req.ID], data...)
			_ = json.NewEncoder(w).Encode(TunnelResponse{OK: true})
		case TunnelOpRX:
			buf := sessions[env.Req.ID]
			if len(buf) == 0 {
				_ = json.NewEncoder(w).Encode(TunnelResponse{OK: true})
				return
			}
			_ = json.NewEncoder(w).Encode(TunnelResponse{OK: true, Data: base64.StdEncoding.EncodeToString(buf)})
			sessions[env.Req.ID] = nil
		case TunnelOpClose:
			delete(sessions, env.Req.ID)
			_ = json.NewEncoder(w).Encode(TunnelResponse{OK: true})
		default:
			_ = json.NewEncoder(w).Encode(TunnelResponse{Error: "bad op"})
		}
	}))
	defer apps.Close()

	client := NewTunnelClient(apps.Client(), []string{apps.URL}, testFrontDomain(apps), "secret", 5*time.Second)
	sess, err := client.OpenSession(context.Background(), "example.com:443")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := sess.Write(context.Background(), []byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := sess.Read(context.Background(), 100*time.Millisecond)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("read = %q", data)
	}
	sess.Close(context.Background())
}

func TestTunnelClient_FailoverURL(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		var env TunnelEnvelope
		_ = json.NewDecoder(r.Body).Decode(&env)
		if calls.Load() == 1 {
			_ = json.NewEncoder(w).Encode(TunnelResponse{Error: "Service invoked too many times for one day: urlfetch"})
			return
		}
		if env.Req.Op == TunnelOpOpen {
			_ = json.NewEncoder(w).Encode(TunnelResponse{OK: true})
		}
	}))
	defer srv.Close()

	// Two entries so the client retries after the first JSON error.
	client := NewTunnelClient(srv.Client(), []string{srv.URL, srv.URL}, testFrontDomain(srv), "k", 5*time.Second)
	_, err := client.OpenSession(context.Background(), "example.com:443")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if calls.Load() < 2 {
		t.Fatalf("expected retry, calls=%d", calls.Load())
	}
}

func TestTunnelClient_BatchFailoverQuota(t *testing.T) {
	quotaBody := []byte(`{"e":"Exception: Service invoked too many times for one day: urlfetch."}`)
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/s/exhausted/exec" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(quotaBody)
			return
		}
		var env TunnelEnvelope
		_ = json.NewDecoder(r.Body).Decode(&env)
		if env.Req.Op == TunnelOpOpen {
			_ = json.NewEncoder(w).Encode(TunnelResponse{OK: true})
			return
		}
		if len(env.Batch) > 0 {
			_ = json.NewEncoder(w).Encode(TunnelBatchResponse{Results: []TunnelResponse{{OK: true}}})
			return
		}
		_ = json.NewEncoder(w).Encode(TunnelResponse{OK: true})
	}))
	defer srv.Close()

	urls := []string{srv.URL + "/s/exhausted/exec", srv.URL + "/s/ok/exec"}
	client := NewTunnelClient(srv.Client(), urls, testFrontDomain(srv), "k", 5*time.Second)
	sess, err := client.OpenSession(context.Background(), "example.com:443")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sess.urlIdx.Store(0)
	_, err = sess.Exchange(context.Background(), []TunnelRequest{
		{Op: TunnelOpTX, Data: base64.StdEncoding.EncodeToString([]byte("x"))},
	})
	if err != nil {
		t.Fatalf("exchange after quota failover: %v", err)
	}
	if sess.urlIdx.Load() != 1 {
		t.Fatalf("urlIdx = %d, want 1 after failover", sess.urlIdx.Load())
	}
	sess.Close(context.Background())
}

func TestTunnelClient_BatchBadURLFallback(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var env TunnelEnvelope
		_ = json.NewDecoder(r.Body).Decode(&env)
		if len(env.Batch) > 0 {
			_ = json.NewEncoder(w).Encode(map[string]string{"e": "bad url"})
			return
		}
		if env.Req.Op == TunnelOpOpen {
			_ = json.NewEncoder(w).Encode(TunnelResponse{OK: true})
			return
		}
		if env.Req.Op == TunnelOpTX {
			_ = json.NewEncoder(w).Encode(TunnelResponse{OK: true})
			return
		}
		if env.Req.Op == TunnelOpRX {
			_ = json.NewEncoder(w).Encode(TunnelResponse{OK: true, Data: base64.StdEncoding.EncodeToString([]byte("pong"))})
			return
		}
		http.Error(w, "unexpected", 400)
	}))
	defer srv.Close()

	client := NewTunnelClient(srv.Client(), []string{srv.URL}, testFrontDomain(srv), "k", 5*time.Second)
	sess, err := client.OpenSession(context.Background(), "example.com:443")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	resps, err := sess.Exchange(context.Background(), []TunnelRequest{
		{Op: TunnelOpTX, Data: base64.StdEncoding.EncodeToString([]byte("ping"))},
		{Op: TunnelOpRX, WaitMS: 10},
	})
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if len(resps) != 2 || resps[1].Data == "" {
		t.Fatalf("resps = %+v", resps)
	}
	sess.Close(context.Background())
}

func TestTunnelClient_BatchExchange(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var env TunnelEnvelope
		_ = json.NewDecoder(r.Body).Decode(&env)
		if len(env.Req.Op) > 0 && env.Req.Op == TunnelOpOpen {
			_ = json.NewEncoder(w).Encode(TunnelResponse{OK: true})
			return
		}
		if len(env.Batch) != 2 {
			http.Error(w, "expected batch", 400)
			return
		}
		_ = json.NewEncoder(w).Encode(TunnelBatchResponse{Results: []TunnelResponse{
			{OK: true},
			{OK: true, Data: base64.StdEncoding.EncodeToString([]byte("ok"))},
		}})
	}))
	defer srv.Close()

	client := NewTunnelClient(srv.Client(), []string{srv.URL}, testFrontDomain(srv), "k", 3*time.Second)
	sess, err := client.OpenSession(context.Background(), "example.com:443")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	resps, err := sess.Exchange(context.Background(), []TunnelRequest{
		{Op: TunnelOpTX, Data: base64.StdEncoding.EncodeToString([]byte("x"))},
		{Op: TunnelOpRX, WaitMS: 100},
	})
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	if len(resps) != 2 || resps[1].Data == "" {
		t.Fatalf("results: %+v", resps)
	}
}

func TestTunnelClient_WarmupAndPingLatency(t *testing.T) {
	var calls atomic.Int32
	apps := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"s":204}`))
	}))
	defer apps.Close()

	client := NewTunnelClient(apps.Client(), []string{apps.URL}, testFrontDomain(apps), "k", 5*time.Second)
	client.Warmup()
	d, err := client.PingLatency(context.Background())
	if err != nil {
		t.Fatalf("PingLatency: %v", err)
	}
	if d < 0 {
		t.Fatalf("negative latency")
	}
	time.Sleep(50 * time.Millisecond)
	if calls.Load() == 0 {
		t.Fatal("expected warmup or ping to hit Apps Script")
	}
	client.Stop()
}

func TestTunnelClient_InvalidTarget(t *testing.T) {
	c := NewTunnelClient(http.DefaultClient, []string{"https://script.google.com/macros/s/x/exec"}, "", "k", time.Second)
	_, err := c.OpenSession(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "invalid tunnel target") {
		t.Fatalf("err = %v", err)
	}
}

// TestTunnelClient_RequiresAppsScriptURLs documents the Iran constraint: relay traffic
// must not bypass Apps Script (no direct client→VPS URL).
func TestTunnelClient_RequiresAppsScriptURLs(t *testing.T) {
	c := NewTunnelClient(http.DefaultClient, nil, "www.google.com", "k", time.Second)
	_, err := c.OpenSession(context.Background(), "example.com:443")
	if err == nil || !strings.Contains(err.Error(), "Apps Script") {
		t.Fatalf("err = %v, want Apps Script URLs required", err)
	}
}
