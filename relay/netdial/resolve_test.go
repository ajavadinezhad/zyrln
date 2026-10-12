package netdial

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestUnderlyingResolutionPreservesSocketProtection(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	_, port, _ := net.SplitHostPort(ln.Addr().String())
	SetHostResolver(func(host string) ([]net.IP, error) {
		if host != "bridge.invalid" {
			t.Errorf("host = %s", host)
		}
		return []net.IP{net.ParseIP("127.0.0.1")}, nil
	})
	t.Cleanup(func() { SetHostResolver(nil); SetSocketProtectFunc(nil) })
	denied := errors.New("protect denied")
	SetSocketProtectFunc(func(int) error { return denied })
	dial := DialContext(ProtectedDialer(time.Second))
	if conn, err := dial(context.Background(), "tcp4", net.JoinHostPort("bridge.invalid", port)); !errors.Is(err, denied) {
		if conn != nil {
			conn.Close()
		}
		t.Fatalf("protection error = %v", err)
	}
	SetSocketProtectFunc(nil)
	conn, err := dial(context.Background(), "tcp4", net.JoinHostPort("bridge.invalid", port))
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	// IP literals bypass DNS, including when the registered resolver is unavailable.
	SetHostResolver(func(string) ([]net.IP, error) {
		t.Error("IP literal entered DNS")
		return nil, errors.New("DNS unavailable")
	})
	conn, err = dial(context.Background(), "tcp4", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
	SetHostResolver(nil)
	conn, err = dial(context.Background(), "tcp4", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn.Close()
}

func TestUnderlyingResolutionCancellationAndFailure(t *testing.T) {
	blocked, release, completed := make(chan struct{}), make(chan struct{}), make(chan struct{})
	SetHostResolver(func(string) ([]net.IP, error) { close(blocked); <-release; close(completed); return nil, nil })
	t.Cleanup(func() { SetHostResolver(nil) })
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, err := DialContext(ProtectedDialer(time.Second))(ctx, "tcp4", "bridge.invalid:443")
		result <- err
	}()
	<-blocked
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("DNS cancellation blocked")
	}
	close(release)
	<-completed
	for _, resolve := range []HostResolver{
		func(string) ([]net.IP, error) { return nil, errors.New("no network") },
		func(string) ([]net.IP, error) { return nil, nil },
		func(string) ([]net.IP, error) { return []net.IP{net.ParseIP("::1")}, nil },
	} {
		SetHostResolver(resolve)
		conn, err := DialContext(ProtectedDialer(time.Second))(context.Background(), "tcp4", "bridge.invalid:443")
		if err == nil || conn != nil {
			t.Fatalf("invalid DNS result: conn=%v err=%v", conn, err)
		}
	}
}
