package netdial

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
)

// HostResolver resolves using the physical network on VPN platforms.
// Implementations may block; callers remain cancellable and concurrent lookups are bounded.
type HostResolver func(string) ([]net.IP, error)

var hostResolver atomic.Pointer[HostResolver]
var resolveSlots = make(chan struct{}, 16)

func SetHostResolver(fn HostResolver) {
	if fn == nil {
		hostResolver.Store(nil)
		return
	}
	hostResolver.Store(&fn)
}

// DialContext keeps the original TLS hostname while resolving outside the VPN.
// Without a platform resolver it retains the standard Go dialer behavior.
func DialContext(dialer *net.Dialer) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		resolver := hostResolver.Load()
		host, port, err := net.SplitHostPort(address)
		if resolver == nil || err != nil || net.ParseIP(host) != nil {
			return dialer.DialContext(ctx, network, address)
		}
		if dialer.Timeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, dialer.Timeout)
			defer cancel()
		}
		select {
		case resolveSlots <- struct{}{}:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		type result struct {
			ips []net.IP
			err error
		}
		done := make(chan result, 1)
		go func() { defer func() { <-resolveSlots }(); ips, err := (*resolver)(host); done <- result{ips, err} }()
		var resolved result
		select {
		case resolved = <-done:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if resolved.err != nil {
			return nil, fmt.Errorf("underlying DNS %s: %w", host, resolved.err)
		}
		var lastErr error
		for _, ip := range resolved.ips {
			if ip == nil || (network == "tcp4" && ip.To4() == nil) || (network == "tcp6" && ip.To4() != nil) {
				continue
			}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("underlying DNS %s returned no usable addresses", host)
	}
}
