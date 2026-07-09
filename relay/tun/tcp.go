package tun

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"zyrln/relay/log"
	"zyrln/relay/tunnel"
)

// tcpMSS is the max TCP payload per TUN frame (MTU 1500 − IP − TCP headers).
const (
	tcpMSS           = 1400
	maxPendingBytes  = 64 * 1024
)

type flowKey struct {
	src     [4]byte
	dst     [4]byte
	srcPort uint16
	dstPort uint16
}

func (k flowKey) target() string {
	return fmt.Sprintf("%s:%d", ipToString(k.dst), k.dstPort)
}

type tcpFlow struct {
	key          flowKey
	engine       *Engine
	clientSeq    uint32
	synSeq       uint32
	ourSeq       uint32
	synAckSent   bool
	backend      net.Conn
	bridgeCancel context.CancelFunc
	pending      []byte
	mu           sync.Mutex
	closed       bool
}

func (e *Engine) handleTCP(ip ipv4Packet, seg tcpSegment) {
	key := flowKey{src: ip.src, dst: ip.dst, srcPort: seg.srcPort, dstPort: seg.dstPort}

	if seg.flags&tcpFlagRST != 0 {
		e.mu.Lock()
		flow := e.flows[key]
		e.mu.Unlock()
		if flow != nil {
			flow.shutdown()
		} else {
			e.removeFlow(key)
		}
		return
	}

	isSYN := seg.flags&tcpFlagSYN != 0 && seg.flags&tcpFlagACK == 0

	if isSYN && e.ctx.Err() != nil {
		return
	}

	e.mu.Lock()
	flow := e.flows[key]
	if flow == nil && isSYN {
		synSeq := randomISN()
		flow = &tcpFlow{
			key:       key,
			engine:    e,
			clientSeq: seg.seq + 1,
			synSeq:    synSeq,
			ourSeq:    synSeq,
		}
		e.flows[key] = flow
		e.mu.Unlock()
		go flow.connect(seg.seq)
		return
	}
	e.mu.Unlock()

	if flow == nil {
		return
	}
	flow.handleSegment(seg)
}

func (f *tcpFlow) connect(clientSYN uint32) {
	target := f.key.target()
	cfg := f.engine.cfg

	if NeedsTunnel(cfg, target) {
		f.connectTunnel(clientSYN, target)
		return
	}

	ctx, cancel := context.WithTimeout(f.engine.ctx, cfg.timeout())
	defer cancel()

	conn, err := DialPlainBackend(ctx, target)
	if err != nil {
		f.failConnect(clientSYN, err)
		return
	}

	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		_ = conn.Close()
		f.removeFromEngine()
		return
	}
	f.backend = conn
	f.mu.Unlock()

	log.Logf("debug", "tun ready %s", target)
	f.completeConnect(clientSYN)
	go f.pumpBackend()
}

func (f *tcpFlow) connectTunnel(clientSYN uint32, target string) {
	cfg := f.engine.cfg
	warmCtx, warmCancel := context.WithTimeout(f.engine.ctx, cfg.timeout())
	cfg.Tunnel.WaitWarmup(warmCtx)
	cfg.Tunnel.WaitWarmupBeforeFirstConnect(warmCtx)
	warmCancel()

	openCtx, openCancel := context.WithTimeout(f.engine.ctx, cfg.timeout())
	sess, err := cfg.Tunnel.OpenSession(openCtx, target)
	openCancel()
	if err != nil {
		f.failConnect(clientSYN, err)
		return
	}

	local, remote := net.Pipe()
	bridgeCtx, bridgeCancel := context.WithCancel(f.engine.ctx)

	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		bridgeCancel()
		_ = local.Close()
		_ = remote.Close()
		closeTunnelSession(f.engine.ctx, sess)
		f.removeFromEngine()
		return
	}
	f.backend = local
	f.bridgeCancel = bridgeCancel
	f.mu.Unlock()

	go tunnel.RunTunnelBridge(bridgeCtx, remote, sess, target, cfg.timeout())

	log.Logf("debug", "tun ready %s", target)
	f.completeConnect(clientSYN)
	go f.pumpBackend()
}

func (f *tcpFlow) failConnect(clientSYN uint32, err error) {
	log.Logf("error", "tun dial %s: %v", f.key.target(), err)
	f.mu.Lock()
	seq := f.ourSeq
	f.mu.Unlock()
	f.engine.sendTCP(f.key.dst, f.key.src, f.key.dstPort, f.key.srcPort,
		seq, clientSYN+1, tcpFlagRST|tcpFlagACK, nil)
	f.shutdown()
}

func (f *tcpFlow) completeConnect(clientSYN uint32) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}
	f.engine.sendTCP(f.key.dst, f.key.src, f.key.dstPort, f.key.srcPort,
		f.synSeq, clientSYN+1, tcpFlagSYN|tcpFlagACK, nil)
	f.synAckSent = true
	f.ourSeq = f.synSeq + 1
	f.flushPendingLocked()
}

func (f *tcpFlow) flushPendingLocked() {
	if f.backend == nil || len(f.pending) == 0 {
		return
	}
	if _, err := f.backend.Write(f.pending); err != nil {
		f.closeLocked()
		f.removeFromEngine()
		return
	}
	f.clientSeq += uint32(len(f.pending))
	f.pending = nil
	f.engine.sendTCP(f.key.dst, f.key.src, f.key.dstPort, f.key.srcPort,
		f.ourSeq, f.clientSeq, tcpFlagACK, nil)
}

func (f *tcpFlow) handleSegment(seg tcpSegment) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return
	}

	if seg.flags&tcpFlagSYN != 0 && seg.flags&tcpFlagACK == 0 {
		f.clientSeq = seg.seq + 1
		if f.synAckSent {
			f.engine.sendTCP(f.key.dst, f.key.src, f.key.dstPort, f.key.srcPort,
				f.synSeq, f.clientSeq, tcpFlagSYN|tcpFlagACK, nil)
		}
		return
	}

	if len(seg.payload) > 0 {
		if seg.seq != f.clientSeq {
			if seg.seq+uint32(len(seg.payload)) <= f.clientSeq {
				f.engine.sendTCP(f.key.dst, f.key.src, f.key.dstPort, f.key.srcPort,
					f.ourSeq, f.clientSeq, tcpFlagACK, nil)
			}
			return
		}
		if f.backend == nil {
			if len(f.pending)+len(seg.payload) > maxPendingBytes {
				f.closeLocked()
				f.removeFromEngine()
				return
			}
			f.pending = append(f.pending, seg.payload...)
			return
		}
		if _, err := f.backend.Write(seg.payload); err != nil {
			if !f.closed && !isPipeClosed(err) {
				log.Logf("error", "tun write %s: %v", f.key.target(), err)
			}
			f.closeLocked()
			f.removeFromEngine()
			return
		}
		f.clientSeq += uint32(len(seg.payload))
		f.engine.sendTCP(f.key.dst, f.key.src, f.key.dstPort, f.key.srcPort,
			f.ourSeq, f.clientSeq, tcpFlagACK, nil)
	}

	if seg.flags&tcpFlagFIN != 0 {
		f.clientSeq++
		f.engine.sendTCP(f.key.dst, f.key.src, f.key.dstPort, f.key.srcPort,
			f.ourSeq, f.clientSeq, tcpFlagFIN|tcpFlagACK, nil)
		f.ourSeq++
		f.closeLocked()
		f.removeFromEngine()
	}
}

func (f *tcpFlow) pumpBackend() {
	buf := make([]byte, 32*1024)
	for {
		f.mu.Lock()
		if f.closed || f.backend == nil {
			f.mu.Unlock()
			return
		}
		conn := f.backend
		f.mu.Unlock()

		n, err := conn.Read(buf)
		if n > 0 {
			f.sendToClient(buf[:n])
		}
		if err != nil {
			f.mu.Lock()
			closed := f.closed
			f.mu.Unlock()
			if !closed && !isPipeClosed(err) && err != io.EOF {
				log.Logf("error", "tun read %s: %v", f.key.target(), err)
			}
			f.mu.Lock()
			if !f.closed {
				f.engine.sendTCP(f.key.dst, f.key.src, f.key.dstPort, f.key.srcPort,
					f.ourSeq, f.clientSeq, tcpFlagFIN|tcpFlagACK, nil)
				f.ourSeq++
				f.closeLocked()
			}
			f.mu.Unlock()
			f.removeFromEngine()
			return
		}
	}
}

func (f *tcpFlow) sendToClient(payload []byte) {
	for off := 0; off < len(payload); off += tcpMSS {
		end := off + tcpMSS
		if end > len(payload) {
			end = len(payload)
		}
		chunk := payload[off:end]

		f.mu.Lock()
		if f.closed {
			f.mu.Unlock()
			return
		}
		f.engine.sendTCP(f.key.dst, f.key.src, f.key.dstPort, f.key.srcPort,
			f.ourSeq, f.clientSeq, tcpFlagACK|tcpFlagPSH, chunk)
		f.ourSeq += uint32(len(chunk))
		f.mu.Unlock()
	}
}

func (f *tcpFlow) closeLocked() {
	if f.closed {
		return
	}
	f.closed = true
	if f.bridgeCancel != nil {
		f.bridgeCancel()
		f.bridgeCancel = nil
	}
	if f.backend != nil {
		_ = f.backend.Close()
		f.backend = nil
	}
}

func (f *tcpFlow) shutdown() {
	f.mu.Lock()
	f.closeLocked()
	f.mu.Unlock()
	f.removeFromEngine()
}

// removeFromEngine deletes this flow from the engine map. Must not be called while holding e.mu.
func (f *tcpFlow) removeFromEngine() {
	f.engine.removeFlow(f.key)
}

func (e *Engine) removeFlow(key flowKey) {
	e.mu.Lock()
	delete(e.flows, key)
	e.mu.Unlock()
}

func (e *Engine) sendTCP(src, dst [4]byte, srcPort, dstPort uint16, seq, ack uint32, flags uint8, payload []byte) {
	pkt := buildIPv4TCP(src, dst, srcPort, dstPort, seq, ack, flags, payload)
	e.writeTUN(pkt)
}

func (e *Engine) writeTUN(pkt []byte) {
	e.writeMu.Lock()
	defer e.writeMu.Unlock()
	if e.tun == nil {
		return
	}
	if _, err := e.tun.Write(pkt); err != nil {
		log.Logf("error", "tun write: %v", err)
	}
}

func randomISN() uint32 {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return binary.BigEndian.Uint32(b[:])
}

func closeTunnelSession(ctx context.Context, sess *tunnel.TunnelSession) {
	if sess == nil {
		return
	}
	closeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	sess.Close(closeCtx)
}

func isPipeClosed(err error) bool {
	return errors.Is(err, io.ErrClosedPipe) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, io.EOF)
}
