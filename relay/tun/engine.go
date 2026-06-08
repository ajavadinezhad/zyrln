package tun

import (
	"context"
	"fmt"
	"os"
	"sync"
	"syscall"
	"time"

	"zyrln/relay/core"
)

// Engine reads IPv4 packets from a TUN fd and forwards TCP to the tunnel/direct dialer.
type Engine struct {
	tun     *os.File
	cfg     Config
	flows   map[flowKey]*tcpFlow
	mu      sync.Mutex
	writeMu sync.Mutex
	dnsSem  chan struct{}
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	stopOnce sync.Once
}

// Start attaches to an existing TUN file descriptor (Android VpnService).
func Start(tunFD int, cfg Config) (*Engine, error) {
	if tunFD < 0 {
		return nil, fmt.Errorf("invalid tun fd")
	}
	core.EnsureDomesticRules()
	// Dup so closing our os.File does not close the fd owned by Android VpnService.
	dupFD, err := syscall.Dup(tunFD)
	if err != nil {
		return nil, fmt.Errorf("dup tun fd: %w", err)
	}
	f := os.NewFile(uintptr(dupFD), "tun")
	if f == nil {
		_ = syscall.Close(dupFD)
		return nil, fmt.Errorf("open tun fd")
	}
	ctx, cancel := context.WithCancel(context.Background())
	e := &Engine{
		tun:    f,
		cfg:    cfg,
		flows:  make(map[flowKey]*tcpFlow),
		dnsSem: make(chan struct{}, maxDNSInflight),
		ctx:    ctx,
		cancel: cancel,
	}
	e.wg.Add(1)
	go e.readLoop()
	return e, nil
}

func (e *Engine) readLoop() {
	defer e.wg.Done()
	buf := make([]byte, 65535)
	for {
		select {
		case <-e.ctx.Done():
			return
		default:
		}
		n, err := e.tun.Read(buf)
		if err != nil {
			return
		}
		if n == 0 {
			continue
		}
		e.handlePacket(buf[:n])
	}
}

func (e *Engine) handlePacket(raw []byte) {
	ip, err := parseIPv4(raw)
	if err != nil {
		return
	}
	switch ip.proto {
	case 6:
		seg, err := parseTCP(ip.payload)
		if err != nil {
			return
		}
		e.handleTCP(ip, seg)
	case 17:
		d, err := parseUDP(ip.payload)
		if err != nil {
			return
		}
		e.handleDNS(ip, d)
	}
}

// Stop shuts down the engine and closes our dup of the TUN fd.
func (e *Engine) Stop() {
	e.stopOnce.Do(func() { e.stop() })
}

func (e *Engine) stop() {
	e.cancel()
	e.mu.Lock()
	stopping := make([]*tcpFlow, 0, len(e.flows))
	for _, flow := range e.flows {
		stopping = append(stopping, flow)
	}
	e.flows = make(map[flowKey]*tcpFlow)
	e.mu.Unlock()

	for _, flow := range stopping {
		flow.mu.Lock()
		flow.closeLocked()
		flow.mu.Unlock()
	}
	if e.tun != nil {
		_ = e.tun.Close()
		e.tun = nil
	}
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
}
