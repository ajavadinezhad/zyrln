// Package netdial provides protected TCP and HTTP clients (VpnService.protect on Android).
package netdial

import (
	"net"
	"sync/atomic"
	"syscall"
	"time"
)

var socketProtectFuncVal atomic.Value // stores func(fd int) error

// SetSocketProtectFunc registers the per-socket protect callback.
// Return an error to abort the connection if protection fails.
// Pass nil to clear it. Thread-safe.
func SetSocketProtectFunc(fn func(fd int) error) {
	if fn == nil {
		socketProtectFuncVal.Store((func(int) error)(nil))
	} else {
		socketProtectFuncVal.Store(fn)
	}
}

// ProtectedDialer returns a net.Dialer whose Control hook calls the registered
// socket protect function (VpnService.protect on Android).
func ProtectedDialer(timeout time.Duration) *net.Dialer {
	return &net.Dialer{
		Timeout:   timeout,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			if fn, _ := socketProtectFuncVal.Load().(func(int) error); fn != nil {
				var protectErr error
				if err := c.Control(func(fd uintptr) { protectErr = fn(int(fd)) }); err != nil {
					return err
				}
				return protectErr
			}
			return nil
		},
	}
}
