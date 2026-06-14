package tun

import (
	"net/http"
	"testing"
	"time"

	"zyrln/relay/core"
	"zyrln/relay/tunnel"
)

func testTunnelClient(t *testing.T) *tunnel.TunnelClient {
	t.Helper()
	return tunnel.NewTunnelClient(http.DefaultClient, []string{"https://script.google.com/macros/s/ABC/exec"}, "www.google.com", "secret", 5*time.Second)
}

func TestConfig_timeout(t *testing.T) {
	if got := (Config{}).timeout(); got != 45*time.Second {
		t.Fatalf("default timeout = %v, want 45s", got)
	}
	if got := (Config{Timeout: 10 * time.Second}).timeout(); got != 10*time.Second {
		t.Fatalf("custom timeout = %v", got)
	}
}

func TestNeedsTunnel_DirectOnly(t *testing.T) {
	tc := testTunnelClient(t)
	if NeedsTunnel(Config{DirectOnly: true, Tunnel: tc}, "youtube.com:443") {
		t.Fatal("direct-only should not use tunnel")
	}
}

func TestNeedsTunnel_NoTunnelClient(t *testing.T) {
	if NeedsTunnel(Config{DirectOnly: false, Tunnel: nil}, "youtube.com:443") {
		t.Fatal("nil tunnel client should not use tunnel")
	}
}

func TestNeedsTunnel_RelayHost(t *testing.T) {
	orig := core.GetDirectEnabled()
	t.Cleanup(func() { core.SetDirectEnabled(orig) })
	core.SetDirectEnabled(false)

	tc := testTunnelClient(t)
	if !NeedsTunnel(Config{DirectOnly: false, Tunnel: tc}, "youtube.com:443") {
		t.Fatal("relay host should use tunnel when direct is off")
	}
}

func TestNeedsTunnel_GoogleDirect(t *testing.T) {
	orig := core.GetDirectEnabled()
	t.Cleanup(func() { core.SetDirectEnabled(orig) })
	core.SetDirectEnabled(true)

	tc := testTunnelClient(t)
	if NeedsTunnel(Config{DirectOnly: false, Tunnel: tc}, "www.google.com:443") {
		t.Fatal("google with direct enabled should fragment, not tunnel")
	}
}

func TestNeedsTunnel_DomesticBypass(t *testing.T) {
	core.EnsureDomesticRules()
	tc := testTunnelClient(t)
	if NeedsTunnel(Config{DirectOnly: false, Tunnel: tc}, "digikala.com:443") {
		t.Fatal("domestic host should not use tunnel")
	}
}

func TestDialPlainBackend_EmptyTarget(t *testing.T) {
	_, err := DialPlainBackend(t.Context(), "  ")
	if err == nil {
		t.Fatal("expected error for empty target")
	}
}
