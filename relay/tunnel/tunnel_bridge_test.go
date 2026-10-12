package tunnel

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestRunTunnelBridge_CoalescesLocalWrites(t *testing.T) {
	var mu sync.Mutex
	var txChunks [][]byte

	apps := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var env TunnelEnvelope
		_ = json.NewDecoder(r.Body).Decode(&env)
		if len(env.Batch) > 0 {
			results := make([]TunnelResponse, len(env.Batch))
			for i, op := range env.Batch {
				results[i] = TunnelResponse{OK: true}
				if op.Op == TunnelOpTX {
					data, _ := base64.StdEncoding.DecodeString(op.Data)
					mu.Lock()
					txChunks = append(txChunks, data)
					mu.Unlock()
				}
			}
			_ = json.NewEncoder(w).Encode(TunnelBatchResponse{Results: results})
			return
		}
		if env.Req.Op == TunnelOpOpen {
			_ = json.NewEncoder(w).Encode(TunnelResponse{OK: true})
			return
		}
		_ = json.NewEncoder(w).Encode(TunnelResponse{OK: true})
	}))
	defer apps.Close()

	client := NewTunnelClient(apps.Client(), []string{apps.URL}, testFrontDomain(apps), "k", 5*time.Second)
	sess, err := client.OpenSession(context.Background(), "example.com:443")
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		RunTunnelBridge(ctx, serverConn, sess, "example.com:443", client.timeout)
		close(done)
	}()

	// Two small writes — bridge should coalesce into one TX per round trip when possible.
	if _, err := clientConn.Write([]byte("ab")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if _, err := clientConn.Write([]byte("cd")); err != nil {
		t.Fatal(err)
	}
	_ = clientConn.Close()

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("bridge timeout")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(txChunks) == 0 {
		t.Fatal("no tx sent")
	}
	total := 0
	for _, c := range txChunks {
		total += len(c)
	}
	if total < 4 {
		t.Fatalf("tx total = %d, want >= 4", total)
	}
	// Best case: both writes in one chunk.
	if len(txChunks) > 2 {
		t.Fatalf("too many tx round trips: %d chunks", len(txChunks))
	}
}

func TestRunTunnelBridge_ForwardsRemoteData(t *testing.T) {
	rxPayload := base64.StdEncoding.EncodeToString([]byte("pong"))

	apps := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var env TunnelEnvelope
		_ = json.NewDecoder(r.Body).Decode(&env)
		if env.Req.Op == TunnelOpOpen {
			_ = json.NewEncoder(w).Encode(TunnelResponse{OK: true})
			return
		}
		if len(env.Batch) > 0 {
			results := make([]TunnelResponse, len(env.Batch))
			for i, op := range env.Batch {
				results[i] = TunnelResponse{OK: true}
				if op.Op == TunnelOpRX {
					results[i].Data = rxPayload
				}
			}
			_ = json.NewEncoder(w).Encode(TunnelBatchResponse{Results: results})
			return
		}
		_ = json.NewEncoder(w).Encode(TunnelResponse{OK: true})
	}))
	defer apps.Close()

	client := NewTunnelClient(apps.Client(), []string{apps.URL}, testFrontDomain(apps), "k", 5*time.Second)
	sess, err := client.OpenSession(context.Background(), "example.com:443")
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	go RunTunnelBridge(ctx, serverConn, sess, "example.com:443", client.timeout)

	if _, err := io.WriteString(clientConn, "ping"); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(clientConn, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "pong" {
		t.Fatalf("got %q", buf)
	}
	_ = clientConn.Close()
}

func TestRunTunnelBridge_RemoteFailureClosesLocalReader(t *testing.T) {
	apps := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var env TunnelEnvelope
		_ = json.NewDecoder(r.Body).Decode(&env)
		if len(env.Batch) > 0 {
			_ = json.NewEncoder(w).Encode(TunnelBatchResponse{Results: []TunnelResponse{{Error: "unknown session"}}})
			return
		}
		_ = json.NewEncoder(w).Encode(TunnelResponse{OK: true})
	}))
	defer apps.Close()
	client := NewTunnelClient(apps.Client(), []string{apps.URL}, testFrontDomain(apps), "k", time.Second)
	defer client.Stop()
	sess := &TunnelSession{client: client, id: "failure-test", target: "example.com:443"}
	sess.opened.Store(true)
	reader, bridge := net.Pipe()
	defer reader.Close()
	defer bridge.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() { RunTunnelBridge(ctx, bridge, sess, sess.target, time.Second); close(done) }()
	reader.SetReadDeadline(time.Now().Add(time.Second))
	_, err := reader.Read(make([]byte, 1))
	if err != io.EOF {
		t.Fatalf("remote failure must close local reader promptly; got %v", err)
	}
	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("bridge did not finish")
	}
}

func TestRunTunnelBridge_CancelUnblocksLocalWrite(t *testing.T) {
	delivered := make(chan struct{}, 1)
	apps := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var env TunnelEnvelope
		_ = json.NewDecoder(r.Body).Decode(&env)
		if len(env.Batch) > 0 {
			select {
			case delivered <- struct{}{}:
			default:
			}
			_ = json.NewEncoder(w).Encode(TunnelBatchResponse{Results: []TunnelResponse{{OK: true, Data: base64.StdEncoding.EncodeToString([]byte("blocked-data"))}}})
			return
		}
		_ = json.NewEncoder(w).Encode(TunnelResponse{OK: true})
	}))
	defer apps.Close()
	client := NewTunnelClient(apps.Client(), []string{apps.URL}, testFrontDomain(apps), "k", time.Second)
	defer client.Stop()
	sess := &TunnelSession{client: client, id: "blocked-test", target: "example.com:443"}
	sess.opened.Store(true)
	reader, bridge := net.Pipe()
	defer reader.Close()
	defer bridge.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { RunTunnelBridge(ctx, bridge, sess, sess.target, time.Second); close(done) }()
	select {
	case <-delivered:
	case <-time.After(time.Second):
		t.Fatal("RX response was not delivered")
	}
	// No peer reads: the receive goroutine's local write must not prevent cancellation.
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled bridge blocked on local write")
	}
}
