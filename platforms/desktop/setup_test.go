package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSetupWorkerArchive(t *testing.T) {
	mux := http.NewServeMux()
	registerSetup(mux, nil)
	req := httptest.NewRequest("POST", "/api/setup/worker", strings.NewReader(`{"host":"zyrln-exit.test-account.workers.dev"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("got %d: %s", w.Code, w.Body)
	}
	reader, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 2 {
		t.Fatal("unexpected deployment files")
	}
	files := map[string]string{}
	for _, entry := range reader.File {
		f, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		files[entry.Name] = string(data)
	}
	if !strings.Contains(files["worker.js"], `const WORKER_HOST = "zyrln-exit.test-account.workers.dev";`) ||
		!strings.Contains(files["wrangler.toml"], `name = "zyrln-exit"`) ||
		!strings.Contains(files["wrangler.toml"], `name = "TUNNEL_HUB"`) {
		t.Fatal("deployment files did not preserve the selected hostname and tunnel binding")
	}
}

func TestSetupWorkerNameCanMatchTemplate(t *testing.T) {
	mux := http.NewServeMux()
	registerSetup(mux, nil)
	req := httptest.NewRequest("POST", "/api/setup/worker", strings.NewReader(`{"host":"mine.test-account.workers.dev"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("valid hostname rejected: %d", w.Code)
	}
}

func TestSetupCheckValidationAndResult(t *testing.T) {
	mux := http.NewServeMux()
	valid := "https://script.google.com/macros/s/demo/exec"
	key := strings.Repeat("a", 64)
	calls := 0
	registerSetup(mux, func(_ context.Context, url, submittedKey string) string {
		calls++
		if url != valid || submittedKey != key {
			t.Fatal("unexpected check input")
		}
		return "tunnel"
	})
	for _, tc := range []struct {
		name, url, site string
		want            int
	}{
		{"valid", valid, "same-origin", 200},
		{"outside Google", "https://example.com/macros/s/demo/exec", "same-origin", 400},
		{"looks like Google", "https://script.google.com.example.com/macros/s/demo/exec", "same-origin", 400},
		{"development link", strings.TrimSuffix(valid, "exec") + "dev", "same-origin", 400},
		{"cross-site POST", valid, "cross-site", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := json.Marshal(setupRequest{URL: tc.url, Key: key})
			req := httptest.NewRequest("POST", "/api/setup/check", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Sec-Fetch-Site", tc.site)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)
			if w.Code != tc.want {
				t.Fatalf("got %d, want %d", w.Code, tc.want)
			}
			if tc.want == 200 {
				var data struct {
					OK    bool
					Issue string
				}
				if err := json.NewDecoder(w.Body).Decode(&data); err != nil || data.OK || data.Issue != "tunnel" {
					t.Fatalf("expected tunnel failure, got %+v (%v)", data, err)
				}
			}
			if strings.Contains(w.Body.String(), key) {
				t.Fatal("response echoed the app key")
			}
		})
	}
	if calls != 1 {
		t.Fatalf("unexpected number of relay checks: %d", calls)
	}
}

type setupTestTransport func(*http.Request) (*http.Response, error)

func (f setupTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSetupRequiresRelayAndTunnel(t *testing.T) {
	for _, tc := range []struct {
		name, web, tunnel, want string
	}{
		{"success", `{"s":204}`, `{"ok":true}`, ""},
		{"failed tunnel", `{"s":204}`, `{"ok":false,"e":"missing binding"}`, "tunnel"},
		{"auth rejection", `{"e":"unauthorized"}`, "", "keys"},
		{"HTML deployment page", `<html>Sign in</html>`, "", "google"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests []string
			var openID string
			client := &http.Client{Transport: setupTestTransport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "www.google.com" || r.Host != "script.google.com" {
					t.Fatalf("request did not use the Google bridge: %s / %s", r.URL.Host, r.Host)
				}
				var body struct {
					URL    string                          `json:"u"`
					Tunnel struct{ Op, ID, Target string } `json:"t"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatal(err)
				}
				response := tc.web
				switch body.Tunnel.Op {
				case "":
					if body.URL != "https://www.gstatic.com/generate_204" {
						t.Fatalf("expected real web fetch, got %q", body.URL)
					}
					requests = append(requests, "fetch")
				case "open":
					requests = append(requests, "open")
					openID = body.Tunnel.ID
					if openID == "" || body.Tunnel.Target != "www.gstatic.com:443" {
						t.Fatal("invalid tunnel open")
					}
					response = tc.tunnel
				case "close":
					requests = append(requests, "close")
					if body.Tunnel.ID != openID {
						t.Fatal("closed the wrong tunnel")
					}
					response = `{"ok":true}`
				default:
					t.Fatalf("unexpected request: %+v", body)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(response)), Header: make(http.Header)}, nil
			})}
			if got := checkSetupWithClient(context.Background(), client, "https://script.google.com/macros/s/demo/exec", strings.Repeat("a", 64)); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
			want := "fetch"
			if tc.tunnel != "" {
				want += ",open,close"
			}
			if strings.Join(requests, ",") != want {
				t.Fatalf("got %v, want %s", requests, want)
			}
		})
	}
}
