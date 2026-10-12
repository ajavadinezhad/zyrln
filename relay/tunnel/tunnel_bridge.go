package tunnel

import (
	"context"
	"encoding/base64"
	"io"
	"net"
	"sync"
	"time"

	"zyrln/relay/core"
)

const localPollWait = 5 * time.Millisecond

var b64EncPool = sync.Pool{
	New: func() any {
		b := make([]byte, base64.StdEncoding.EncodedLen(TunnelChunkSize))
		return &b
	},
}

func encodeTXChunk(chunk []byte) string {
	need := base64.StdEncoding.EncodedLen(len(chunk))
	if need <= TunnelChunkSize*4/3+8 {
		if p := b64EncPool.Get(); p != nil {
			bufPtr := p.(*[]byte)
			buf := *bufPtr
			if cap(buf) >= need {
				base64.StdEncoding.Encode(buf[:need], chunk)
				out := string(buf[:need])
				b64EncPool.Put(bufPtr)
				return out
			}
			b64EncPool.Put(bufPtr)
		}
	}
	return base64.StdEncoding.EncodeToString(chunk)
}

// RunTunnelBridge copies bytes between local and a relay tunnel session until both sides finish.
// TX and RX use separate Apps Script requests so uploads are not blocked waiting for polls.
func RunTunnelBridge(ctx context.Context, local io.ReadWriter, sess *TunnelSession, target string, opTimeout time.Duration) {
	if sess == nil || sess.client == nil {
		core.Log("error", "tunnel bridge %s: nil session", target)
		return
	}
	sess.client.beginBridge()
	defer sess.client.endBridge()

	if opTimeout <= 0 {
		opTimeout = sess.client.timeout
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	defer sess.Close(context.Background())

	buf := make([]byte, TunnelChunkSize)
	pending := make([]byte, 0, TunnelChunkSize*2)
	localDone := false
	lastDataAt := time.Now()
	localConn, _ := local.(net.Conn)

	rxFailed := make(chan error, 1)
	var rxWG sync.WaitGroup
	rxWG.Add(1)
	go runTunnelRXLoop(ctx, local, sess, target, opTimeout, rxFailed, &rxWG)
	defer func() {
		cancel()
		// Close before waiting: RX may be blocked writing to the local pipe.
		// Also wake its peer so bridge errors cannot become silent TLS timeouts.
		if closer, ok := local.(io.Closer); ok {
			_ = closer.Close()
		}
		rxWG.Wait()
	}()

	shouldFlushTX := func() bool {
		if len(pending) == 0 {
			return false
		}
		if localDone {
			return true
		}
		if len(pending) >= TunnelChunkSize*MaxTXPerBatch {
			return true
		}
		return time.Since(lastDataAt) >= txCoalesceMaxWait
	}

	absorbLocal := func() {
		if localDone || localConn == nil {
			return
		}
		for len(pending) < TunnelChunkSize*MaxTXPerBatch {
			_ = localConn.SetReadDeadline(time.Now().Add(time.Millisecond))
			n, err := localConn.Read(buf)
			_ = localConn.SetReadDeadline(time.Time{})
			if n > 0 {
				pending = append(pending, buf[:n]...)
				lastDataAt = time.Now()
			}
			if err != nil {
				if err != io.EOF && ctx.Err() == nil {
					if ne, ok := err.(net.Error); !ok || !ne.Timeout() {
						core.Log("error", "tunnel local read %s: %v", target, err)
						localDone = true
						return
					}
				}
				if err != io.EOF {
					if ne, ok := err.(net.Error); ok && ne.Timeout() {
						return
					}
				}
				localDone = true
				return
			}
			if n == 0 {
				return
			}
		}
	}

	prependOpen := func(ops []TunnelRequest) []TunnelRequest {
		if sess.opened.Load() || len(ops) == 0 {
			return ops
		}
		sess.openSent.Store(true)
		return append([]TunnelRequest{{Op: TunnelOpOpen, ID: sess.id, Target: sess.target}}, ops...)
	}

	appendTXOps := func(ops []TunnelRequest) []TunnelRequest {
		for len(pending) >= TunnelChunkSize && len(ops) < MaxTXPerBatch {
			chunk := pending[:TunnelChunkSize]
			pending = pending[TunnelChunkSize:]
			ops = append(ops, TunnelRequest{
				Op:   TunnelOpTX,
				Data: encodeTXChunk(chunk),
			})
		}
		if len(ops) < MaxTXPerBatch && len(pending) > 0 && (localDone || len(ops) == 0) {
			chunk := append([]byte(nil), pending...)
			pending = pending[:0]
			ops = append(ops, TunnelRequest{
				Op:   TunnelOpTX,
				Data: encodeTXChunk(chunk),
			})
		}
		return ops
	}

	markOpened := func(ops []TunnelRequest) {
		if sess.opened.Load() {
			return
		}
		for _, op := range ops {
			if op.Op == TunnelOpOpen {
				sess.opened.Store(true)
				return
			}
		}
	}

	exchangeTX := func(ops []TunnelRequest) error {
		if len(ops) == 0 {
			return nil
		}
		ops = prependOpen(ops)
		exCtx, exCancel := context.WithTimeout(ctx, opTimeout)
		_, err := sess.Exchange(exCtx, ops)
		exCancel()
		if err != nil {
			return err
		}
		markOpened(ops)
		return nil
	}

	flushTX := func() error {
		for {
			ops := appendTXOps(nil)
			if len(ops) == 0 {
				return nil
			}
			moreTX := len(pending) >= TunnelChunkSize && !localDone
			if err := exchangeTX(ops); err != nil {
				return err
			}
			if !moreTX {
				return nil
			}
		}
	}

	stopBridge := func() {
		cancel()
	}

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-rxFailed:
			stopBridge()
			if err != nil {
				core.Log("error", "tunnel rx %s: %v", target, err)
			}
			return
		default:
		}

		if !localDone {
			if localConn != nil {
				_ = localConn.SetReadDeadline(time.Now().Add(localPollWait))
				n, err := localConn.Read(buf)
				_ = localConn.SetReadDeadline(time.Time{})
				if n > 0 {
					pending = append(pending, buf[:n]...)
					lastDataAt = time.Now()
				}
				if err != nil {
					if err != io.EOF && ctx.Err() == nil {
						if ne, ok := err.(net.Error); !ok || !ne.Timeout() {
							core.Log("error", "tunnel local read %s: %v", target, err)
							stopBridge()
							return
						}
					}
					if err == io.EOF {
						localDone = true
					} else if ne, ok := err.(net.Error); !ok || !ne.Timeout() {
						localDone = true
					}
				}
				absorbLocal()
			} else {
				n, err := local.Read(buf)
				if n > 0 {
					pending = append(pending, buf[:n]...)
					lastDataAt = time.Now()
				}
				if err != nil {
					localDone = true
					if err != io.EOF && ctx.Err() == nil {
						core.Log("error", "tunnel local read %s: %v", target, err)
						stopBridge()
						return
					}
				}
			}
		}

		if len(pending) > 0 {
			absorbLocal()
			if !shouldFlushTX() {
				continue
			}
			if err := flushTX(); err != nil {
				if ctx.Err() == nil {
					core.Log("error", "tunnel flush %s: %v", target, err)
				}
				stopBridge()
				return
			}
			continue
		}

		if localDone {
			stopBridge()
			return
		}

		// RX goroutine polls the tunnel; avoid busy-spin when idle.
		time.Sleep(localPollWait)
	}
}

func runTunnelRXLoop(ctx context.Context, local io.Writer, sess *TunnelSession, target string, opTimeout time.Duration, failed chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()

	rxWait := tunnelMinReadWait
	fail := func(err error) {
		if err == nil || ctx.Err() != nil {
			return
		}
		select {
		case failed <- err:
		default:
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		ops := []TunnelRequest{{
			Op:     TunnelOpRX,
			WaitMS: tunnelRXWaitMS(false, rxWait),
		}}
		if !sess.opened.Load() {
			sess.openSent.Store(true)
			ops = append([]TunnelRequest{{Op: TunnelOpOpen, ID: sess.id, Target: sess.target}}, ops...)
		}

		exCtx, exCancel := context.WithTimeout(ctx, opTimeout)
		resps, err := sess.Exchange(exCtx, ops)
		exCancel()
		if err != nil {
			fail(err)
			return
		}
		for _, op := range ops {
			if op.Op == TunnelOpOpen {
				sess.opened.Store(true)
				break
			}
		}

		resp := resps[len(resps)-1]
		if resp.Data == "" {
			if rxWait < tunnelMaxReadWait {
				rxWait += tunnelMinReadWait
				if rxWait > tunnelMaxReadWait {
					rxWait = tunnelMaxReadWait
				}
			}
			continue
		}
		rxWait = tunnelMinReadWait

		data, err := base64.StdEncoding.DecodeString(resp.Data)
		if err != nil {
			fail(err)
			return
		}
		if _, err := local.Write(data); err != nil {
			fail(err)
			return
		}
	}
}
