package tunnel

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"zyrln/relay/core"
)

// tunnelKeepaliveInterval is how often we ping Apps Script while the VPN/proxy is up.
// Shorter than the legacy coalescer interval so instances stay warm between page loads.
const tunnelKeepaliveInterval = 75 * time.Second

// tunnelKeepaliveDeepEvery: every Nth tick also runs a batched tunnel open+close so the
// /tunnel path on Apps Script and the VPS stay warm, not just the relay HEAD path.
const tunnelKeepaliveDeepEvery = 3

// TunnelClient carries raw TCP frames through domain-fronted Apps Script to the exit /tunnel.
//
// Iran constraint: every round trip uses core.AppsScriptRoundTrip (script.google.com, TLS to a
// Google front). The VPS/Worker is only contacted from Apps Script (Code.gs UrlFetchApp),
// never from this client. Do not add a direct exit-URL dial path here.
//
// TLS to the final target stays end-to-end; no local CA is required.
type TunnelClient struct {
	client          atomic.Value // *http.Client
	appScriptURLs   []string
	frontDomain     string
	authKey         string
	timeout         time.Duration
	stopCh          chan struct{}
	stopOnce        sync.Once
	keepaliveOnce   sync.Once
	warmOnce        sync.Once
	warmDone        chan struct{}
	warmReady       atomic.Bool
	warmMu          sync.Mutex
	warmCancel      context.CancelFunc
	deferKeepalive  bool
	activeBridges   atomic.Int32
	lastTraffic     atomic.Int64
	connectWarmOnce sync.Once
}

// Warmup pre-warms Apps Script and the VPS /tunnel path in the background.
// Idempotent; also started automatically from NewTunnelClient.
func (c *TunnelClient) Warmup() {
	c.startWarmup()
}

// WarmReady reports whether the initial warmup pass finished (success or failure).
func (c *TunnelClient) WarmReady() bool {
	return c.warmReady.Load()
}

// WaitWarmup blocks until warmup completes or ctx is canceled.
func (c *TunnelClient) WaitWarmup(ctx context.Context) error {
	if c == nil {
		return nil
	}
	if c.WarmReady() {
		return nil
	}
	c.startWarmup()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.warmDone:
		return nil
	}
}

// WaitWarmupBeforeFirstConnect blocks only on the first tunnel open after startup.
func (c *TunnelClient) WaitWarmupBeforeFirstConnect(ctx context.Context) {
	c.waitWarmupBeforeFirstConnect(ctx)
}

// waitWarmupBeforeFirstConnect blocks only on the first tunnel CONNECT after startup.
func (c *TunnelClient) waitWarmupBeforeFirstConnect(ctx context.Context) {
	if c == nil || c.WarmReady() {
		return
	}
	c.connectWarmOnce.Do(func() {
		warmCtx, cancel := context.WithTimeout(ctx, tunnelFirstConnectWarmWait)
		defer cancel()
		_ = c.WaitWarmup(warmCtx)
	})
}

// UseHTTPClient replaces the HTTP client (e.g. after VPN socket protect is registered).
func (c *TunnelClient) UseHTTPClient(client *http.Client) {
	if c != nil && client != nil {
		c.client.Store(client)
	}
}

func (c *TunnelClient) httpClient() *http.Client {
	if c == nil {
		return http.DefaultClient
	}
	if v := c.client.Load(); v != nil {
		if hc, ok := v.(*http.Client); ok && hc != nil {
			return hc
		}
	}
	return http.DefaultClient
}

// Activate starts the keepalive loop for a prewarm client adopted by the running proxy.
func (c *TunnelClient) Activate() {
	if c == nil {
		return
	}
	c.startKeepalive()
}

func (c *TunnelClient) beginBridge() {
	c.activeBridges.Add(1)
	c.noteTraffic()
	c.abortWarmupForUser()
}

func (c *TunnelClient) endBridge() {
	c.activeBridges.Add(-1)
	c.noteTraffic()
}

func (c *TunnelClient) noteTraffic() {
	c.lastTraffic.Store(time.Now().UnixNano())
}

func (c *TunnelClient) userBusy() bool {
	if c.activeBridges.Load() > 0 {
		return true
	}
	last := c.lastTraffic.Load()
	if last == 0 {
		return false
	}
	return time.Since(time.Unix(0, last)) < tunnelUserIdleGrace
}

func (c *TunnelClient) abortWarmupForUser() {
	c.warmMu.Lock()
	cancel := c.warmCancel
	c.warmMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Stop ends the keepalive loop. Safe to call multiple times.
func (c *TunnelClient) Stop() {
	if c == nil {
		return
	}
	c.stopOnce.Do(func() { close(c.stopCh) })
}

// PingLatency measures Apps Script round-trip via light ping (fallback: legacy HEAD relay).
func (c *TunnelClient) PingLatency(ctx context.Context) (time.Duration, error) {
	if c == nil || len(c.appScriptURLs) == 0 {
		return 0, fmt.Errorf("no Apps Script URLs configured")
	}
	start := time.Now()
	err := c.pingAppsScript(ctx)
	return time.Since(start), err
}

func (c *TunnelClient) startWarmup() {
	if c == nil || len(c.appScriptURLs) == 0 {
		return
	}
	c.warmOnce.Do(func() {
		c.warmDone = make(chan struct{})
		go c.runWarmup()
	})
}

func (c *TunnelClient) runWarmup() {
	defer close(c.warmDone)
	defer c.warmReady.Store(true)

	ctx, cancel := context.WithTimeout(context.Background(), tunnelWarmupTimeout)
	c.warmMu.Lock()
	c.warmCancel = cancel
	c.warmMu.Unlock()
	defer func() {
		cancel()
		c.warmMu.Lock()
		c.warmCancel = nil
		c.warmMu.Unlock()
	}()

	if c.userBusy() {
		return
	}
	_ = c.pingAppsScript(ctx)
	if ctx.Err() != nil || c.userBusy() {
		return
	}
	c.warmupTunnelBatched(ctx)
}

func (c *TunnelClient) pingTimeout() time.Duration {
	if c.timeout <= 0 || c.timeout > 15*time.Second {
		return 15 * time.Second
	}
	return c.timeout
}

func (c *TunnelClient) pingAppsScript(ctx context.Context) error {
	return c.pingAllAppsScript(ctx)
}

func (c *TunnelClient) pingAllAppsScript(ctx context.Context) error {
	if err := c.pingAppsScriptLight(ctx); err == nil {
		return nil
	}
	return c.pingAppsScriptLegacy(ctx)
}

func (c *TunnelClient) pingAppsScriptLight(ctx context.Context) error {
	if c == nil || len(c.appScriptURLs) == 0 {
		return fmt.Errorf("no Apps Script URLs configured")
	}
	payload, err := json.Marshal(TunnelEnvelope{
		Key: c.authKey,
		Req: TunnelRequest{Op: TunnelOpPing},
	})
	if err != nil {
		return err
	}
	pingTimeout := c.pingTimeout()
	idx := int(core.ActiveURLIdxLoad()) % len(c.appScriptURLs)
	if idx < 0 {
		idx = 0
	}
	raw, err := core.AppsScriptRoundTrip(ctx, c.httpClient(), c.appScriptURLs[idx], c.frontDomain, string(payload), pingTimeout)
	if err != nil {
		return err
	}
	var resp TunnelResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Errorf("invalid ping JSON: %w", err)
	}
	if !resp.OK {
		if resp.Error == "" {
			resp.Error = "ping failed"
		}
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (c *TunnelClient) pingAppsScriptLegacy(ctx context.Context) error {
	payload := core.BuildRelayPayload(c.authKey, "HEAD", "https://www.gstatic.com/generate_204", map[string]string{}, nil)
	pingTimeout := c.pingTimeout()
	idx := int(core.ActiveURLIdxLoad()) % len(c.appScriptURLs)
	if idx < 0 {
		idx = 0
	}
	_, err := core.TryOneURL(ctx, c.httpClient(), c.appScriptURLs[idx], c.frontDomain, payload, pingTimeout)
	return err
}

func (c *TunnelClient) warmupTunnelBatched(ctx context.Context) {
	id, err := newTunnelSessionID()
	if err != nil {
		return
	}
	ops := []TunnelRequest{
		{Op: TunnelOpOpen, ID: id, Target: "1.1.1.1:443"},
		{Op: TunnelOpClose, ID: id},
	}
	idx := int(core.ActiveURLIdxLoad()) % len(c.appScriptURLs)
	if idx < 0 {
		idx = 0
	}
	_, _ = c.roundTripBatchPinned(ctx, idx, ops, func(int) {})
}

func (c *TunnelClient) keepaliveLoop() {
	select {
	case <-c.warmDone:
	case <-time.After(tunnelWarmupTimeout):
	case <-c.stopCh:
		return
	}
	ticker := time.NewTicker(tunnelKeepaliveInterval)
	defer ticker.Stop()
	n := 0
	for {
		select {
		case <-ticker.C:
			n++
			c.keepaliveTick(n%tunnelKeepaliveDeepEvery == 0)
		case <-c.stopCh:
			return
		}
	}
}

func (c *TunnelClient) keepaliveTick(deep bool) {
	if c.userBusy() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = c.pingAllAppsScript(ctx)
	if deep && !c.userBusy() {
		c.warmupTunnelBatched(ctx)
	}
}

func (c *TunnelClient) startKeepalive() {
	c.keepaliveOnce.Do(func() { go c.keepaliveLoop() })
}

// NewTunnelClient builds a tunnel client using the same Apps Script transport as HTTP relay.
func NewTunnelClient(client *http.Client, appScriptURLs []string, frontDomain, authKey string, timeout time.Duration) *TunnelClient {
	if client == nil {
		client = &http.Client{
			Timeout: timeout,
			Transport: &http.Transport{
				MaxIdleConns:        32,
				MaxIdleConnsPerHost: 8,
				IdleConnTimeout:     90 * time.Second,
			},
		}
	}
	c := &TunnelClient{
		appScriptURLs: append([]string(nil), appScriptURLs...),
		frontDomain:   frontDomain,
		authKey:       authKey,
		timeout:       timeout,
		stopCh:        make(chan struct{}),
	}
	c.UseHTTPClient(client)
	if len(c.appScriptURLs) > 0 {
		c.startWarmup()
		if !c.deferKeepalive {
			c.startKeepalive()
		}
	}
	return c
}

// TunnelSession is one multiplexed TCP flow on the exit relay.
type TunnelSession struct {
	client   *TunnelClient
	id       string
	target   string
	urlIdx   atomic.Int64
	opened   atomic.Bool
	openSent atomic.Bool
	closed   atomic.Bool
}

// NewSession allocates a tunnel session id without opening (bridge batches open with first TX).
func (c *TunnelClient) NewSession(target string) (*TunnelSession, error) {
	target = NormalizeHostPort(target, "443")
	if !ValidTunnelTarget(target) {
		return nil, fmt.Errorf("invalid tunnel target %q", target)
	}
	id, err := newTunnelSessionID()
	if err != nil {
		return nil, err
	}
	n := len(c.appScriptURLs)
	idx := 0
	if n > 0 {
		idx = int(core.ActiveURLIdxLoad()) % n
		if idx < 0 {
			idx = 0
		}
	}
	s := &TunnelSession{client: c, id: id, target: target}
	s.urlIdx.Store(int64(idx))
	return s, nil
}

// OpenSession dials target through the relay tunnel.
func (c *TunnelClient) OpenSession(ctx context.Context, target string) (*TunnelSession, error) {
	s, err := c.NewSession(target)
	if err != nil {
		return nil, err
	}
	if err := c.openSession(ctx, s, target); err != nil {
		return nil, err
	}
	s.opened.Store(true)
	return s, nil
}

// openSession tries Apps Script URLs in order (one UrlFetch per attempt). Parallel open
// was removed — it burned 4× quota on every CONNECT.
func (c *TunnelClient) openSession(ctx context.Context, s *TunnelSession, target string) error {
	n := len(c.appScriptURLs)
	if n == 0 {
		return fmt.Errorf("no Apps Script URLs configured")
	}
	req := TunnelRequest{Op: TunnelOpOpen, ID: s.id, Target: target}
	start := int(s.urlIdx.Load()) % n
	if start < 0 {
		start = 0
	}
	var lastErr error
	for i := 0; i < n; i++ {
		idx := (start + i) % n
		_, err := c.tryTunnelURL(ctx, idx, req)
		if err == nil {
			s.urlIdx.Store(int64(idx))
			core.ActiveURLIdxStore(int64(idx))
			return nil
		}
		lastErr = err
		if isRetryableAppsScriptError(err.Error()) {
			c.bestEffortClose(ctx, idx, s.id)
			continue
		}
		return err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("tunnel open failed")
	}
	return lastErr
}

func (c *TunnelClient) tryTunnelURL(ctx context.Context, idx int, req TunnelRequest) (TunnelResponse, error) {
	if idx < 0 || idx >= len(c.appScriptURLs) {
		return TunnelResponse{}, fmt.Errorf("bad script url index")
	}
	payload, err := json.Marshal(TunnelEnvelope{Key: c.authKey, Req: req})
	if err != nil {
		return TunnelResponse{}, err
	}
	opCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	raw, err := core.AppsScriptRoundTrip(opCtx, c.httpClient(), c.appScriptURLs[idx], c.frontDomain, string(payload), c.timeout)
	if err != nil {
		return TunnelResponse{}, err
	}
	if retry, bodyErr := tunnelBodyShouldRetry(raw); retry {
		return TunnelResponse{}, bodyErr
	} else if bodyErr != nil {
		return TunnelResponse{}, bodyErr
	}
	var resp TunnelResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return TunnelResponse{}, fmt.Errorf("invalid tunnel JSON: %w", err)
	}
	if !resp.OK {
		if resp.Error == "" {
			resp.Error = "tunnel request failed"
		}
		err := fmt.Errorf("%s", resp.Error)
		if strings.Contains(resp.Error, "bad url") {
			err = fmt.Errorf("%w (Apps Script missing tunnel handler or EXIT_TUNNEL_URL points at /relay)", err)
		}
		return TunnelResponse{}, err
	}
	return resp, nil
}

// Write sends bytes to the remote TCP connection.
func (s *TunnelSession) Write(ctx context.Context, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	return s.roundTrip(ctx, TunnelRequest{
		Op:   TunnelOpTX,
		ID:   s.id,
		Data: base64.StdEncoding.EncodeToString(data),
	})
}

// Read polls the remote side. An empty slice with nil error means no data yet (timeout).
func (s *TunnelSession) Read(ctx context.Context, wait time.Duration) ([]byte, error) {
	resp, err := s.roundTripRaw(ctx, TunnelRequest{
		Op:     TunnelOpRX,
		ID:     s.id,
		WaitMS: int(clampTunnelReadWait(wait) / time.Millisecond),
	})
	if err != nil {
		return nil, err
	}
	if resp.Data == "" {
		return nil, nil
	}
	data, err := base64.StdEncoding.DecodeString(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("tunnel rx base64: %w", err)
	}
	return data, nil
}

// Exchange runs one or more tunnel ops in a single Apps Script HTTP request.
func (s *TunnelSession) Exchange(ctx context.Context, ops []TunnelRequest) ([]TunnelResponse, error) {
	if len(ops) == 0 {
		return nil, nil
	}
	for i := range ops {
		ops[i].ID = s.id
		if ops[i].Op == TunnelOpOpen && ops[i].Target == "" {
			ops[i].Target = s.target
		}
	}
	raw, err := s.client.roundTripBatchPinned(ctx, int(s.urlIdx.Load()), ops, func(idx int) { s.urlIdx.Store(int64(idx)) })
	if err != nil {
		if batchErrorNeedsSequential(err) {
			if resps, seqErr := s.exchangeSequential(ctx, ops); seqErr == nil {
				return resps, nil
			}
		}
		s.cleanupAfterBatchError(ops)
		return nil, err
	}
	var batch TunnelBatchResponse
	if err := json.Unmarshal(raw, &batch); err != nil || len(batch.Results) == 0 {
		batchErr := fmt.Errorf("invalid tunnel batch JSON: %w; body=%s", err, core.PreviewBytes(raw, 256))
		if batchErrorNeedsSequential(batchErr) {
			if resps, seqErr := s.exchangeSequential(ctx, ops); seqErr == nil {
				return resps, nil
			}
		}
		return nil, batchErr
	}
	if len(batch.Results) != len(ops) {
		batchErr := fmt.Errorf("tunnel batch size mismatch: got %d results for %d ops", len(batch.Results), len(ops))
		if batchErrorNeedsSequential(batchErr) {
			if resps, seqErr := s.exchangeSequential(ctx, ops); seqErr == nil {
				return resps, nil
			}
		}
		return nil, batchErr
	}
	for i, resp := range batch.Results {
		if !resp.OK {
			if resp.Error == "" {
				resp.Error = "tunnel request failed"
			}
			opErr := fmt.Errorf("op %d: %s", i, resp.Error)
			if batchErrorNeedsSequential(opErr) {
				if resps, seqErr := s.exchangeSequential(ctx, ops); seqErr == nil {
					return resps, nil
				}
			}
			return batch.Results, opErr
		}
	}
	return batch.Results, nil
}

func (s *TunnelSession) exchangeOne(ctx context.Context, op TunnelRequest) ([]TunnelResponse, error) {
	resp, err := s.roundTripRaw(ctx, op)
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		if resp.Error == "" {
			resp.Error = "tunnel request failed"
		}
		return []TunnelResponse{resp}, fmt.Errorf("%s", resp.Error)
	}
	return []TunnelResponse{resp}, nil
}

func (s *TunnelSession) exchangeSequential(ctx context.Context, ops []TunnelRequest) ([]TunnelResponse, error) {
	resps := make([]TunnelResponse, 0, len(ops))
	for i, op := range ops {
		one, err := s.exchangeOne(ctx, op)
		if err != nil {
			return resps, fmt.Errorf("op %d: %w", i, err)
		}
		resps = append(resps, one...)
	}
	return resps, nil
}

// Close ends the remote session.
func (s *TunnelSession) Close(ctx context.Context) {
	if !s.closed.CompareAndSwap(false, true) {
		return
	}
	if !s.opened.Load() && !s.openSent.Load() {
		return
	}
	s.client.bestEffortClose(ctx, int(s.urlIdx.Load()), s.id)
}

func (s *TunnelSession) cleanupAfterBatchError(ops []TunnelRequest) {
	for _, op := range ops {
		if op.Op == TunnelOpOpen {
			s.openSent.Store(true)
			s.client.bestEffortClose(context.Background(), int(s.urlIdx.Load()), s.id)
			return
		}
	}
}

func (s *TunnelSession) roundTrip(ctx context.Context, req TunnelRequest) error {
	resp, err := s.roundTripRaw(ctx, req)
	if err != nil {
		return err
	}
	if !resp.OK {
		if resp.Error == "" {
			resp.Error = "tunnel request failed"
		}
		return fmt.Errorf("%s", resp.Error)
	}
	return nil
}

func (s *TunnelSession) roundTripRaw(ctx context.Context, req TunnelRequest) (TunnelResponse, error) {
	if s.closed.Load() {
		return TunnelResponse{}, fmt.Errorf("tunnel session closed")
	}
	return s.client.roundTripPinned(ctx, int(s.urlIdx.Load()), req, func(idx int) { s.urlIdx.Store(int64(idx)) })
}

func (c *TunnelClient) roundTripPinned(ctx context.Context, startIdx int, req TunnelRequest, onPin func(int)) (TunnelResponse, error) {
	if len(c.appScriptURLs) == 0 {
		return TunnelResponse{}, fmt.Errorf("no Apps Script URLs configured")
	}
	payload, err := json.Marshal(TunnelEnvelope{Key: c.authKey, Req: req})
	if err != nil {
		return TunnelResponse{}, err
	}
	n := len(c.appScriptURLs)
	start := startIdx % n
	if start < 0 {
		start = 0
	}
	var lastErr error
	for i := 0; i < n; i++ {
		idx := (start + i) % n
		opCtx, cancel := context.WithTimeout(ctx, core.PerURLTimeout(c.timeout, n))
		raw, err := core.AppsScriptRoundTrip(opCtx, c.httpClient(), c.appScriptURLs[idx], c.frontDomain, string(payload), c.timeout)
		cancel()
		if err != nil {
			if !strings.Contains(err.Error(), "context canceled") {
				lastErr = err
			}
			continue
		}
		if retry, bodyErr := tunnelBodyShouldRetry(raw); retry {
			if req.Op == TunnelOpOpen {
				c.bestEffortClose(ctx, idx, req.ID)
			}
			lastErr = bodyErr
			continue
		} else if bodyErr != nil {
			return TunnelResponse{}, bodyErr
		}
		var resp TunnelResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			lastErr = fmt.Errorf("invalid tunnel JSON: %w", err)
			continue
		}
		if !resp.OK {
			if resp.Error == "" {
				resp.Error = "tunnel request failed"
			}
			lastErr = fmt.Errorf("%s", resp.Error)
			if isRetryableAppsScriptError(resp.Error) || (req.Op == TunnelOpOpen && strings.Contains(resp.Error, "session exists")) {
				if req.Op == TunnelOpOpen {
					c.bestEffortClose(ctx, idx, req.ID)
				}
				continue
			}
			return TunnelResponse{}, lastErr
		}
		if idx != start {
			core.ActiveURLIdxStore(int64(idx))
		}
		onPin(idx)
		return resp, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("tunnel request failed")
	}
	return TunnelResponse{}, lastErr
}

func (c *TunnelClient) bestEffortClose(ctx context.Context, urlIdx int, id string) {
	_, _ = c.roundTripPinned(ctx, urlIdx, TunnelRequest{Op: TunnelOpClose, ID: id}, func(int) {})
}

func (c *TunnelClient) maybeCloseBatchOnRetry(ops []TunnelRequest, urlIdx int, err error) {
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "session exists") {
		return
	}
	for _, op := range ops {
		if op.Op == TunnelOpOpen && op.ID != "" {
			c.bestEffortClose(context.Background(), urlIdx, op.ID)
			return
		}
	}
}

func (c *TunnelClient) roundTripBatchPinned(ctx context.Context, startIdx int, ops []TunnelRequest, onPin func(int)) ([]byte, error) {
	return c.roundTripPayloadPinned(ctx, startIdx, TunnelEnvelope{Key: c.authKey, Batch: ops}, onPin)
}

func (c *TunnelClient) roundTripPayloadPinned(ctx context.Context, startIdx int, env TunnelEnvelope, onPin func(int)) ([]byte, error) {
	if len(c.appScriptURLs) == 0 {
		return nil, fmt.Errorf("no Apps Script URLs configured")
	}
	n := len(c.appScriptURLs)
	start := startIdx % n
	if start < 0 {
		start = 0
	}

	payload, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for i := 0; i < n; i++ {
		idx := (start + i) % n
		opCtx, cancel := context.WithTimeout(ctx, core.PerURLTimeout(c.timeout, n))
		raw, err := core.AppsScriptRoundTrip(opCtx, c.httpClient(), c.appScriptURLs[idx], c.frontDomain, string(payload), c.timeout)
		cancel()
		if err != nil {
			if !strings.Contains(err.Error(), "context canceled") {
				lastErr = err
			}
			continue
		}
		if retry, bodyErr := tunnelBodyShouldRetry(raw); retry {
			c.maybeCloseBatchOnRetry(env.Batch, idx, bodyErr)
			lastErr = bodyErr
			continue
		} else if bodyErr != nil {
			return nil, bodyErr
		}
		if idx != start {
			core.ActiveURLIdxStore(int64(idx))
		}
		onPin(idx)
		return raw, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("tunnel request failed")
	}
	return nil, lastErr
}

func newTunnelSessionID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
