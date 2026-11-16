package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"time"
	"zyrln/relay/appscript"
	"zyrln/relay/deploy"
	"zyrln/relay/tunnel"
)

var setupWorkerHost = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.workers\.dev$`)
var setupScriptURL = regexp.MustCompile(`^https://script\.google\.com/macros/s/[A-Za-z0-9_-]{1,256}/exec$`)
var setupKey = regexp.MustCompile(`^[a-f0-9]{64}$`)

type setupRequest struct {
	Host string `json:"host"`
	URL  string `json:"url"`
	Key  string `json:"key"`
}

func setupInput(w http.ResponseWriter, r *http.Request) (setupRequest, bool) {
	var input setupRequest
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "POST required", http.StatusMethodNotAllowed)
		return input, false
	}
	contentType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if contentType != "application/json" || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "Invalid setup request", http.StatusBadRequest)
		return input, false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	dec.DisallowUnknownFields()
	if dec.Decode(&input) != nil || dec.Decode(new(any)) != io.EOF {
		http.Error(w, "Invalid setup request", http.StatusBadRequest)
		return input, false
	}
	return input, true
}

func registerSetup(mux *http.ServeMux, check func(context.Context, string, string) string) {
	mux.HandleFunc("GET /api/setup/script", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		io.WriteString(w, deploy.AppsScript)
	})
	mux.HandleFunc("/api/setup/worker", func(w http.ResponseWriter, r *http.Request) {
		input, ok := setupInput(w, r)
		if !ok {
			return
		}
		if !setupWorkerHost.MatchString(input.Host) {
			http.Error(w, "Enter the full Worker hostname, such as zyrln-exit.example.workers.dev.", http.StatusBadRequest)
			return
		}
		name, _, _ := strings.Cut(input.Host, ".")
		const workerPlaceholder = `const WORKER_HOST = "CHANGE_ME_WORKER_HOST";`
		const namePlaceholder = `name = "mine"`
		if !strings.Contains(deploy.Worker, workerPlaceholder) || !strings.Contains(deploy.Wrangler, namePlaceholder) {
			http.Error(w, "Worker template could not be prepared", http.StatusInternalServerError)
			return
		}
		worker := strings.Replace(deploy.Worker, workerPlaceholder, `const WORKER_HOST = "`+input.Host+`";`, 1)
		config := strings.Replace(deploy.Wrangler, namePlaceholder, `name = "`+name+`"`, 1)
		var archive bytes.Buffer
		z := zip.NewWriter(&archive)
		for _, file := range []struct{ name, body string }{{"worker.js", worker}, {"wrangler.toml", config}} {
			f, err := z.Create(file.name)
			if err != nil {
				http.Error(w, "Could not prepare Worker files", http.StatusInternalServerError)
				return
			}
			if _, err := io.WriteString(f, file.body); err != nil {
				http.Error(w, "Could not prepare Worker files", http.StatusInternalServerError)
				return
			}
		}
		if z.Close() != nil {
			http.Error(w, "Could not prepare Worker files", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="zyrln-worker.zip"`)
		w.Write(archive.Bytes())
	})
	mux.HandleFunc("/api/setup/check", func(w http.ResponseWriter, r *http.Request) {
		input, ok := setupInput(w, r)
		if !ok {
			return
		}
		if !setupScriptURL.MatchString(input.URL) || !setupKey.MatchString(input.Key) {
			http.Error(w, "Enter the Google web app link ending in /exec.", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		result := check(r.Context(), input.URL, input.Key)
		json.NewEncoder(w).Encode(map[string]any{"ok": result == "", "issue": result})
	})
}

const setupTimeout = 16 * time.Second

func checkSetupRelay(ctx context.Context, endpoint, key string) string {
	client := appscript.NewHTTPClient(setupTimeout)
	defer client.CloseIdleConnections()
	return checkSetupWithClient(ctx, client, endpoint, key)
}

func checkSetupWithClient(ctx context.Context, client *http.Client, endpoint, key string) string {
	payload := appscript.BuildRelayPayload(key, "GET", "https://www.gstatic.com/generate_204", nil, nil)
	resp, err := appscript.TryOneURL(ctx, client, endpoint, "www.google.com", payload, setupTimeout)
	if err != nil {
		msg := strings.ToLower(err.Error())
		switch {
		case strings.Contains(msg, "service invoked too many times"), strings.Contains(msg, "quota exceeded"):
			return "quota"
		case strings.Contains(msg, "unauthorized"), strings.Contains(msg, "relay error: forbidden"):
			return "keys"
		case strings.Contains(msg, "html"), strings.Contains(msg, "returned 404"), strings.Contains(msg, "returned 403"):
			return "google"
		default:
			return "relay"
		}
	}
	if resp.Status != http.StatusNoContent {
		return "relay"
	}
	var nonce [16]byte
	rand.Read(nonce[:])
	id := hex.EncodeToString(nonce[:])
	callTunnel := func(ctx context.Context, op, target string, timeout time.Duration) bool {
		payload, _ := json.Marshal(tunnel.TunnelEnvelope{
			Key: key,
			Req: tunnel.TunnelRequest{Op: op, ID: id, Target: target},
		})
		raw, err := appscript.AppsScriptRoundTrip(ctx, client, endpoint, "www.google.com", string(payload), timeout)
		var response tunnel.TunnelResponse
		return err == nil && json.Unmarshal(raw, &response) == nil && response.OK && response.Error == ""
	}

	opened := callTunnel(ctx, tunnel.TunnelOpOpen, "www.gstatic.com:443", setupTimeout)
	// Even when the open response was lost, try to close a session the exit may have created.
	callTunnel(context.WithoutCancel(ctx), tunnel.TunnelOpClose, "", 3*time.Second)
	if !opened {
		return "tunnel"
	}
	return ""
}
