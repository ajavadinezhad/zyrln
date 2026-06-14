package tun

import (
	"os"
	"testing"
	"time"
)

func newTestEngine(t *testing.T, cfg Config) (*Engine, *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	t.Cleanup(func() {
		_ = w.Close()
		_ = r.Close()
	})
	eng, err := Start(int(r.Fd()), cfg)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(eng.Stop)
	return eng, w
}

func TestEngine_HandlePacket_RejectsNonIPv4(t *testing.T) {
	_, w := newTestEngine(t, Config{DirectOnly: true, Timeout: time.Second})
	if _, err := w.Write([]byte{0x60}); err != nil { // IPv6 version nibble
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
}

func TestEngine_HandlePacket_RejectsShortPacket(t *testing.T) {
	_, w := newTestEngine(t, Config{DirectOnly: true, Timeout: time.Second})
	if _, err := w.Write([]byte{0x45, 0x00}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
}

func TestEngine_HandlePacket_IgnoresNonTCPUDP(t *testing.T) {
	var src, dst [4]byte
	copy(src[:], []byte{10, 99, 0, 2})
	copy(dst[:], []byte{1, 2, 3, 4})
	raw := buildIPv4ICMP(src, dst)
	_, w := newTestEngine(t, Config{DirectOnly: true, Timeout: time.Second})
	if _, err := w.Write(raw); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
}

func TestEngine_HandlePacket_DNSNonPort53Ignored(t *testing.T) {
	var src, dst [4]byte
	copy(src[:], []byte{10, 99, 0, 2})
	copy(dst[:], []byte{8, 8, 8, 8})
	raw := buildIPv4UDP(src, dst, 54321, 54, []byte("query"))
	_, w := newTestEngine(t, Config{DirectOnly: true, Timeout: time.Second})
	if _, err := w.Write(raw); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
}

func TestEngine_Stop_Idempotent(t *testing.T) {
	eng, _ := newTestEngine(t, Config{DirectOnly: true, Timeout: time.Second})
	eng.Stop()
	eng.Stop()
}

// buildIPv4ICMP builds a minimal IPv4 ICMP echo packet for proto filtering tests.
func buildIPv4ICMP(src, dst [4]byte) []byte {
	const total = ipv4HeaderLen + 8
	buf := make([]byte, total)
	buf[0] = 0x45
	buf[2] = byte(total >> 8)
	buf[3] = byte(total)
	buf[8] = 64
	buf[9] = 1 // ICMP
	copy(buf[12:16], src[:])
	copy(buf[16:20], dst[:])
	return buf
}
