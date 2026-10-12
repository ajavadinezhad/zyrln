package netdial

import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestProtectedDialer(t *testing.T) {
	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	denied := errors.New("protection denied")
	for _, tc := range []struct {
		name    string
		protect func(int) error
		wantErr error
	}{
		{"failure aborts dial", func(int) error { return denied }, denied},
		{"success permits dial", func(int) error { return nil }, nil},
		{"cleared callback permits dial", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			SetSocketProtectFunc(tc.protect)
			t.Cleanup(func() { SetSocketProtectFunc(nil) })
			c, err := ProtectedDialer(time.Second).Dial("tcp4", ln.Addr().String())
			if c != nil {
				defer c.Close()
			}
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("dial error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil && c != nil {
				t.Fatal("failed protection returned a connection")
			}
			if tc.wantErr == nil && c == nil {
				t.Fatal("successful dial returned no connection")
			}
		})
	}
}
