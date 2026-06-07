package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const maxBodyBytes = 32 * 1024 * 1024

type relayRequest struct {
	URL      string            `json:"u"`
	Method   string            `json:"m"`
	Headers  map[string]string `json:"h"`
	Body     string            `json:"b"`
	Redirect bool              `json:"r"`
}

type relayResponse struct {
	Status  int                 `json:"s,omitempty"`
	Headers map[string][]string `json:"h,omitempty"`
	Body    string              `json:"b,omitempty"`
	Error   string              `json:"e,omitempty"`
}

func main() {
	listen := flag.String("listen", envDefault("ZYRLN_RELAY_LISTEN", "127.0.0.1:8787"), "listen address")
	key := flag.String("key", os.Getenv("ZYRLN_RELAY_KEY"), "exit relay key (X-Relay-Key header from Apps Script)")
	timeout := flag.Duration("timeout", 45*time.Second, "target request timeout")
	flag.Parse()

	client := &http.Client{
		Timeout: *timeout,
		Transport: &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          256,
			MaxIdleConnsPerHost:   64,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   15 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	mux := http.NewServeMux()
	tunnelHub := newTunnelHub(*timeout)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/relay", func(w http.ResponseWriter, r *http.Request) {
		handleRelay(w, r, client, *key, *timeout)
	})
	mux.HandleFunc("/tunnel", func(w http.ResponseWriter, r *http.Request) {
		handleTunnel(w, r, tunnelHub, *key, *timeout)
	})

	server := &http.Server{
		Addr:              *listen,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	log.Printf("zyrln relay listening on http://%s (/relay + /tunnel)", *listen)
	if strings.TrimSpace(*key) == "" {
		log.Printf("warning: ZYRLN_RELAY_KEY is empty; /relay and /tunnel accept any caller")
	}
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func handleRelay(w http.ResponseWriter, r *http.Request, client *http.Client, key string, timeout time.Duration) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, relayResponse{Error: "POST required"})
		return
	}
	if strings.TrimSpace(key) != "" && r.Header.Get("X-Relay-Key") != key {
		writeJSON(w, http.StatusUnauthorized, relayResponse{Error: "unauthorized"})
		return
	}

	var req relayRequest
	decoder := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	if err := decoder.Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, relayResponse{Error: "bad json: " + err.Error()})
		return
	}

	target, err := url.Parse(req.URL)
	if err != nil || target.Scheme == "" || target.Host == "" {
		writeJSON(w, http.StatusBadRequest, relayResponse{Error: "bad url"})
		return
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		writeJSON(w, http.StatusBadRequest, relayResponse{Error: "unsupported scheme"})
		return
	}

	method := strings.ToUpper(strings.TrimSpace(req.Method))
	if method == "" {
		method = http.MethodGet
	}

	var body []byte
	if req.Body != "" {
		body, err = base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, relayResponse{Error: "bad base64 body"})
			return
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	outReq, err := http.NewRequestWithContext(ctx, method, target.String(), bytes.NewReader(body))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, relayResponse{Error: err.Error()})
		return
	}
	for hk, hv := range req.Headers {
		if !skipForwardedHeader(hk) {
			outReq.Header.Set(hk, hv)
		}
	}

	localClient := *client
	if !req.Redirect {
		localClient.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	start := time.Now()
	resp, err := localClient.Do(outReq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, relayResponse{Error: err.Error()})
		log.Printf("%s %s -> error %s", method, target.String(), err)
		return
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxBodyBytes)
	respBody, err := io.ReadAll(limited)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, relayResponse{Error: "read response: " + err.Error()})
		return
	}

	headers := map[string][]string{}
	for hk, values := range resp.Header {
		headers[strings.ToLower(hk)] = values
	}

	writeJSON(w, http.StatusOK, relayResponse{
		Status:  resp.StatusCode,
		Headers: headers,
		Body:    base64.StdEncoding.EncodeToString(respBody),
	})
	log.Printf("%s %s -> %d %dB %s", method, target.String(), resp.StatusCode, len(respBody), time.Since(start).Round(time.Millisecond))
}

func skipForwardedHeader(key string) bool {
	switch strings.ToLower(key) {
	case "host", "connection", "content-length", "proxy-connection", "proxy-authorization", "transfer-encoding", "accept-encoding":
		return true
	default:
		return false
	}
}

func writeJSON(w http.ResponseWriter, status int, resp relayResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		fmt.Fprintf(w, `{"e":%q}`, err.Error())
	}
}

func envDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
