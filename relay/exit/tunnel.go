package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type tunnelRequest struct {
	Op     string `json:"op"`
	ID     string `json:"id"`
	Target string `json:"target,omitempty"`
	Data   string `json:"data,omitempty"`
	WaitMS int    `json:"wait_ms,omitempty"`
}

type tunnelResponse struct {
	OK    bool   `json:"ok"`
	Data  string `json:"data,omitempty"`
	Error string `json:"e,omitempty"`
}

type tunnelBatchRequest struct {
	Ops []tunnelRequest `json:"ops"`
}

type tunnelBatchResponse struct {
	Results []tunnelResponse `json:"results"`
}

type tunnelSession struct {
	conn     net.Conn
	writeMu  sync.Mutex
	readMu   sync.Mutex
	lastSeen time.Time
}

type tunnelHub struct {
	mu       sync.Mutex
	sessions map[string]*tunnelSession
	timeout  time.Duration
}

func newTunnelHub(timeout time.Duration) *tunnelHub {
	h := &tunnelHub{
		sessions: make(map[string]*tunnelSession),
		timeout:  timeout,
	}
	go h.cleanupLoop()
	return h
}

func handleTunnel(w http.ResponseWriter, r *http.Request, hub *tunnelHub, key string, timeout time.Duration) {
	if r.Method != http.MethodPost {
		writeTunnelJSON(w, http.StatusMethodNotAllowed, tunnelResponse{Error: "POST required"})
		return
	}
	if strings.TrimSpace(key) != "" && r.Header.Get("X-Relay-Key") != key {
		writeTunnelJSON(w, http.StatusUnauthorized, tunnelResponse{Error: "unauthorized"})
		return
	}

	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		writeTunnelJSON(w, http.StatusBadRequest, tunnelResponse{Error: "read body: " + err.Error()})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	var batch tunnelBatchRequest
	if err := json.Unmarshal(raw, &batch); err == nil && len(batch.Ops) > 0 {
		writeTunnelBatch(w, hub.handleBatch(ctx, batch.Ops, timeout))
		return
	}

	var req tunnelRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		writeTunnelJSON(w, http.StatusBadRequest, tunnelResponse{Error: "bad json: " + err.Error()})
		return
	}

	resp := hub.handle(ctx, req, timeout)
	status := http.StatusOK
	if !resp.OK && resp.Error != "" {
		status = http.StatusBadGateway
	}
	writeTunnelJSON(w, status, resp)
}

func (h *tunnelHub) handleBatch(ctx context.Context, ops []tunnelRequest, timeout time.Duration) tunnelBatchResponse {
	out := make([]tunnelResponse, len(ops))
	for i, op := range ops {
		out[i] = h.handle(ctx, op, timeout)
		if !out[i].OK {
			break
		}
	}
	return tunnelBatchResponse{Results: out}
}

func writeTunnelJSON(w http.ResponseWriter, status int, resp tunnelResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(resp)
}

func writeTunnelBatch(w http.ResponseWriter, batch tunnelBatchResponse) {
	status := http.StatusOK
	for _, resp := range batch.Results {
		if !resp.OK && resp.Error != "" {
			status = http.StatusBadGateway
			break
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(batch)
}

func (h *tunnelHub) handle(ctx context.Context, req tunnelRequest, timeout time.Duration) tunnelResponse {
	req.Op = strings.ToLower(strings.TrimSpace(req.Op))
	req.ID = strings.TrimSpace(req.ID)

	switch req.Op {
	case "open":
		if req.ID == "" || !validTunnelTarget(req.Target) {
			return tunnelResponse{Error: "bad request"}
		}
		if h.get(req.ID) != nil {
			return tunnelResponse{Error: "session exists"}
		}
		dialer := net.Dialer{Timeout: minDuration(timeout, 15 * time.Second)}
		conn, err := dialer.DialContext(ctx, tunnelDialNetwork(req.Target), req.Target)
		if err != nil {
			return tunnelResponse{Error: err.Error()}
		}
		h.mu.Lock()
		if old := h.sessions[req.ID]; old != nil {
			h.mu.Unlock()
			_ = conn.Close()
			return tunnelResponse{Error: "session exists"}
		}
		h.sessions[req.ID] = &tunnelSession{conn: conn, lastSeen: time.Now()}
		h.mu.Unlock()
		log.Printf("tunnel open %s -> %s", shortTunnelID(req.ID), req.Target)
		return tunnelResponse{OK: true}

	case "tx":
		sess := h.get(req.ID)
		if sess == nil {
			return tunnelResponse{Error: "unknown session"}
		}
		data, err := base64.StdEncoding.DecodeString(req.Data)
		if err != nil {
			return tunnelResponse{Error: "bad base64"}
		}
		writeDeadline := minDuration(timeout, 15*time.Second)
		sess.writeMu.Lock()
		_ = sess.conn.SetWriteDeadline(time.Now().Add(writeDeadline))
		_, err = sess.conn.Write(data)
		_ = sess.conn.SetWriteDeadline(time.Time{})
		sess.writeMu.Unlock()
		h.touch(req.ID)
		if err != nil {
			h.close(req.ID)
			return tunnelResponse{Error: err.Error()}
		}
		return tunnelResponse{OK: true}

	case "rx":
		sess := h.get(req.ID)
		if sess == nil {
			return tunnelResponse{Error: "unknown session"}
		}
		wait := time.Duration(req.WaitMS) * time.Millisecond
		const maxRXWait = 3 * time.Second
		if wait < 0 {
			wait = 0
		}
		if wait > maxRXWait {
			wait = maxRXWait
		}
		if wait == 0 {
			wait = time.Millisecond
		}
		buf := make([]byte, 128*1024)
		sess.readMu.Lock()
		_ = sess.conn.SetReadDeadline(time.Now().Add(wait))
		n, err := sess.conn.Read(buf)
		_ = sess.conn.SetReadDeadline(time.Time{})
		sess.readMu.Unlock()
		h.touch(req.ID)
		if n > 0 {
			return tunnelResponse{OK: true, Data: base64.StdEncoding.EncodeToString(buf[:n])}
		}
		if ne, ok := err.(net.Error); ok && ne.Timeout() {
			return tunnelResponse{OK: true}
		}
		if err != nil {
			h.close(req.ID)
			return tunnelResponse{Error: err.Error()}
		}
		return tunnelResponse{OK: true}

	case "close":
		h.close(req.ID)
		return tunnelResponse{OK: true}

	default:
		return tunnelResponse{Error: "bad request"}
	}
}

func (h *tunnelHub) get(id string) *tunnelSession {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.sessions[id]
}

func (h *tunnelHub) close(id string) {
	h.mu.Lock()
	sess := h.sessions[id]
	delete(h.sessions, id)
	h.mu.Unlock()
	if sess != nil {
		sess.writeMu.Lock()
		sess.readMu.Lock()
		_ = sess.conn.Close()
		sess.readMu.Unlock()
		sess.writeMu.Unlock()
	}
}

func (h *tunnelHub) touch(id string) {
	h.mu.Lock()
	if sess := h.sessions[id]; sess != nil {
		sess.lastSeen = time.Now()
	}
	h.mu.Unlock()
}

func (h *tunnelHub) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		idle := h.timeout
		if idle < 2*time.Minute {
			idle = 2 * time.Minute
		}
		cutoff := time.Now().Add(-idle)
		h.mu.Lock()
		var stale []string
		for id, sess := range h.sessions {
			if sess.lastSeen.Before(cutoff) {
				stale = append(stale, id)
			}
		}
		h.mu.Unlock()
		for _, id := range stale {
			h.close(id)
		}
	}
}

func validTunnelTarget(target string) bool {
	host, port, err := net.SplitHostPort(strings.TrimSpace(target))
	return err == nil && host != "" && port != ""
}

func tunnelDialNetwork(target string) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(target))
	if err != nil {
		return "tcp"
	}
	if ip := net.ParseIP(host); ip != nil {
		if ip.To4() != nil {
			return "tcp4"
		}
		return "tcp"
	}
	// Hostname: use dual-stack dial (tcp) so IPv6-only DNS does not fail on VPS with working v6.
	return "tcp"
}

func shortTunnelID(id string) string {
	if len(id) <= 8 {
		return id
	}
	return id[:8]
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
