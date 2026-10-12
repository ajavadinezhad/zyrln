package tun

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

type observedWriteConn struct {
	net.Conn
	entered chan struct{}
	once    sync.Once
}

func (c *observedWriteConn) Write(b []byte) (int, error) {
	c.once.Do(func() { close(c.entered) })
	return c.Conn.Write(b)
}
func socketTestFlow(t *testing.T, timeout time.Duration) (*Engine, *tcpFlow, net.Conn, chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	e := &Engine{ctx: ctx, cancel: cancel, cfg: Config{Timeout: timeout}, flows: make(map[flowKey]*tcpFlow)}
	local, peer := net.Pipe()
	entered := make(chan struct{})
	f := &tcpFlow{engine: e, backend: &observedWriteConn{Conn: local, entered: entered}, clientSeq: 100, recvSeq: 100, writeReady: make(chan struct{}, 1)}
	e.flows[f.key] = f
	t.Cleanup(func() { peer.Close(); e.Stop() })
	go f.pumpClient()
	return e, f, peer, entered
}
func TestBlockedBackendWriteDoesNotBlockPacketHandlingOrStop(t *testing.T) {
	e, f, _, entered := socketTestFlow(t, time.Minute)
	handled := make(chan struct{})
	go func() { f.handleSegment(tcpSegment{seq: 100, payload: []byte("blocked upload")}); close(handled) }()
	select {
	case <-handled:
	case <-time.After(time.Second):
		t.Fatal("socket write blocked packet handling")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("writer never reached backend")
	}
	stopped := make(chan struct{})
	go func() { e.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("shutdown blocked on socket write")
	}
}
func TestBlockedBackendWriteTimesOutAndRemovesFlow(t *testing.T) {
	e, f, _, entered := socketTestFlow(t, 30*time.Millisecond)
	f.handleSegment(tcpSegment{seq: 100, payload: []byte("timeout")})
	<-entered
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		e.mu.Lock()
		remaining := len(e.flows)
		e.mu.Unlock()
		if remaining == 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed-out flow was not removed")
}
func TestQueuedPayloadIsDeliveredBeforeFIN(t *testing.T) {
	_, f, peer, _ := socketTestFlow(t, time.Second)
	f.handleSegment(tcpSegment{seq: 100, flags: tcpFlagFIN, payload: []byte("complete payload")})
	peer.SetReadDeadline(time.Now().Add(time.Second))
	got, err := io.ReadAll(peer)
	if err != nil || string(got) != "complete payload" {
		t.Fatalf("payload lost before FIN: %q, %v", got, err)
	}
}
func TestPendingUploadQueueIsBounded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	e := &Engine{ctx: ctx, cancel: cancel, flows: make(map[flowKey]*tcpFlow)}
	f := &tcpFlow{engine: e, clientSeq: 100, recvSeq: 100, writeReady: make(chan struct{}, 1)}
	f.handleSegment(tcpSegment{seq: 100, payload: make([]byte, maxPendingBytes)})
	f.handleSegment(tcpSegment{seq: 100 + maxPendingBytes, payload: []byte("excess")})
	if len(f.pending) != maxPendingBytes || f.recvSeq != 100+maxPendingBytes {
		t.Fatal("queue exceeded limit or accepted excess bytes")
	}
}
