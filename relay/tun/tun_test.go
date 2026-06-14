package tun

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"testing"
)

func TestParseBuildIPv4TCP(t *testing.T) {
	var src, dst [4]byte
	copy(src[:], net.ParseIP("10.99.0.2").To4())
	copy(dst[:], net.ParseIP("91.108.8.6").To4())

	payload := []byte("hello")
	raw := buildIPv4TCP(src, dst, 12345, 443, 100, 200, tcpFlagACK|tcpFlagPSH, payload)

	ip, err := parseIPv4(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ip.proto != 6 {
		t.Fatalf("proto=%d", ip.proto)
	}
	seg, err := parseTCP(ip.payload)
	if err != nil {
		t.Fatal(err)
	}
	if seg.srcPort != 12345 || seg.dstPort != 443 {
		t.Fatalf("ports %d %d", seg.srcPort, seg.dstPort)
	}
	if string(seg.payload) != "hello" {
		t.Fatalf("payload=%q", seg.payload)
	}
}

func TestIPChecksum(t *testing.T) {
	hdr := make([]byte, 20)
	hdr[0] = 0x45
	binary.BigEndian.PutUint16(hdr[2:4], 20)
	hdr[9] = 6
	copy(hdr[12:16], net.ParseIP("10.0.0.1").To4())
	copy(hdr[16:20], net.ParseIP("10.0.0.2").To4())
	c := ipChecksum(hdr)
	if c == 0 {
		t.Fatal("expected non-zero checksum")
	}
}

func TestTCPMSS(t *testing.T) {
	if tcpMSS <= 0 || tcpMSS > 1460 {
		t.Fatalf("tcpMSS=%d", tcpMSS)
	}
	if maxPendingBytes <= tcpMSS {
		t.Fatalf("maxPendingBytes=%d", maxPendingBytes)
	}
}

func TestFlowKeyTarget(t *testing.T) {
	var dst [4]byte
	copy(dst[:], net.ParseIP("1.2.3.4").To4())
	k := flowKey{dst: dst, dstPort: 443}
	if k.target() != "1.2.3.4:443" {
		t.Fatalf("target=%q", k.target())
	}
}

func TestParseIPv4_Errors(t *testing.T) {
	cases := []struct {
		name string
		raw  []byte
	}{
		{"empty", nil},
		{"short", []byte{0x45, 0x00}},
		{"not v4", []byte{0x60, 0x00, 0x00, 0x00, 0x00, 0x14}},
		{"bad ihl", []byte{0x41, 0x00, 0x00, 0x14, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0a, 0x00, 0x00, 0x02, 0x08, 0x08, 0x08, 0x08}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseIPv4(tc.raw); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestParseTCP_ErrorsAndFields(t *testing.T) {
	if _, err := parseTCP(nil); err == nil {
		t.Fatal("expected error for empty tcp")
	}
	if _, err := parseTCP(make([]byte, tcpHeaderMin-1)); err == nil {
		t.Fatal("expected error for short tcp")
	}

	hdr := make([]byte, tcpHeaderMin)
	binary.BigEndian.PutUint16(hdr[0:2], 8080)
	binary.BigEndian.PutUint16(hdr[2:4], 443)
	binary.BigEndian.PutUint32(hdr[4:8], 100)
	binary.BigEndian.PutUint32(hdr[8:12], 200)
	hdr[12] = 0x50
	hdr[13] = tcpFlagSYN
	seg, err := parseTCP(hdr)
	if err != nil {
		t.Fatal(err)
	}
	if seg.srcPort != 8080 || seg.dstPort != 443 || seg.seq != 100 || seg.ack != 200 {
		t.Fatalf("seg=%+v", seg)
	}
	if seg.flags&tcpFlagSYN == 0 {
		t.Fatal("expected SYN flag")
	}
}

func TestParseBuildIPv4UDP(t *testing.T) {
	var src, dst [4]byte
	copy(src[:], net.ParseIP("10.99.0.2").To4())
	copy(dst[:], net.ParseIP("8.8.8.8").To4())
	payload := []byte{0x00, 0x01, 0x02, 0x03}
	raw := buildIPv4UDP(src, dst, 53000, 53, payload)

	ip, err := parseIPv4(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ip.proto != 17 {
		t.Fatalf("proto=%d", ip.proto)
	}
	d, err := parseUDP(ip.payload)
	if err != nil {
		t.Fatal(err)
	}
	if d.srcPort != 53000 || d.dstPort != 53 {
		t.Fatalf("ports %d %d", d.srcPort, d.dstPort)
	}
	if string(d.payload) != string(payload) {
		t.Fatalf("payload=%q", d.payload)
	}
}

func TestParseUDP_Errors(t *testing.T) {
	if _, err := parseUDP([]byte{0, 1, 2}); err == nil {
		t.Fatal("expected short udp error")
	}
	bad := make([]byte, 8)
	binary.BigEndian.PutUint16(bad[4:6], 20) // length larger than buffer
	if _, err := parseUDP(bad); err == nil {
		t.Fatal("expected bad length error")
	}
}

func TestIPToString(t *testing.T) {
	var ip [4]byte
	copy(ip[:], net.ParseIP("192.168.1.1").To4())
	if got := ipToString(ip); got != "192.168.1.1" {
		t.Fatalf("got %q", got)
	}
}

func TestTCPAndUDPChecksumNonZero(t *testing.T) {
	var src, dst [4]byte
	copy(src[:], net.ParseIP("10.0.0.1").To4())
	copy(dst[:], net.ParseIP("10.0.0.2").To4())

	tcp := make([]byte, tcpHeaderMin)
	binary.BigEndian.PutUint16(tcp[16:18], 0)
	if c := tcpChecksum(src, dst, tcp); c == 0 {
		t.Fatal("expected non-zero tcp checksum")
	}

	udp := make([]byte, 8)
	binary.BigEndian.PutUint16(udp[6:8], 0)
	if c := udpChecksum(src, dst, udp); c == 0 {
		t.Fatal("expected non-zero udp checksum")
	}
}

func TestIsPipeClosed(t *testing.T) {
	if !isPipeClosed(io.EOF) {
		t.Fatal("expected EOF to count as closed pipe")
	}
	if isPipeClosed(fmt.Errorf("other")) {
		t.Fatal("unexpected closed for generic error")
	}
}
