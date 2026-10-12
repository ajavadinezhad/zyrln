package mobile

import (
	"net"
	"strings"
	"testing"
	"time"
	"zyrln/relay/netdial"
)

type rejectingProtector struct{ called bool }

func (p *rejectingProtector) Protect(fd int64) bool { p.called = true; return false }

func TestSocketProtectionFailure(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	p := &rejectingProtector{}
	SetSocketProtector(p)
	t.Cleanup(func() { SetSocketProtector(nil) })
	c, err := netdial.ProtectedDialer(time.Second).Dial("tcp4", ln.Addr().String())
	if c != nil {
		c.Close()
		t.Fatal("unprotected socket connected")
	}
	if !p.called {
		t.Fatal("protector was not called")
	}
	if err == nil || !strings.Contains(err.Error(), "VPN socket protection failed") {
		t.Fatalf("unexpected dial error: %v", err)
	}
	if !strings.Contains(GetAllLogs(), "VPN socket protection failed") {
		t.Fatal("protection failure was not logged")
	}
}
