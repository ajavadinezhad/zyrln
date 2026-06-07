package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandleRelay_RejectsNonPOST(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/relay", nil)

	handleRelay(w, r, http.DefaultClient, "", time.Second)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
	assertRelayErrorContains(t, w.Body.Bytes(), "POST required")
}

func TestHandleRelay_RequiresRelayKey(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/relay", strings.NewReader(`{"u":"https://example.com"}`))

	handleRelay(w, r, http.DefaultClient, "secret", time.Second)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
	assertRelayErrorContains(t, w.Body.Bytes(), "unauthorized")
}

func TestHandleRelay_RejectsBadURL(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"missing", `{}`},
		{"relative", `{"u":"/local/path"}`},
		{"unsupported scheme", `{"u":"file:///etc/passwd"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/relay", strings.NewReader(tc.body))
			handleRelay(w, r, http.DefaultClient, "", time.Second)

			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
			}
		})
	}
}

func TestHandleRelay_ForwardsRequestAndEncodesResponse(t *testing.T) {
	var gotMethod, gotBody, gotCustom, gotConnection string
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotCustom = r.Header.Get("X-Custom")
		gotConnection = r.Header.Get("Connection")
		data, _ := io.ReadAll(r.Body)
		gotBody = string(data)

		w.Header().Add("Set-Cookie", "a=1")
		w.Header().Add("Set-Cookie", "b=2")
		w.Header().Set("X-Reply", "ok")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("target response"))
	}))
	defer target.Close()

	payload := relayRequest{
		URL:    target.URL + "/submit",
		Method: http.MethodPost,
		Headers: map[string]string{
			"X-Custom":   "kept",
			"Connection": "close",
		},
		Body: base64.StdEncoding.EncodeToString([]byte("request body")),
	}
	raw, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/relay", bytes.NewReader(raw))
	handleRelay(w, r, target.Client(), "", time.Second)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	if gotMethod != http.MethodPost {
		t.Errorf("target method = %q, want POST", gotMethod)
	}
	if gotBody != "request body" {
		t.Errorf("target body = %q, want request body", gotBody)
	}
	if gotCustom != "kept" {
		t.Errorf("X-Custom = %q, want kept", gotCustom)
	}
	if gotConnection != "" {
		t.Errorf("Connection header should be stripped, got %q", gotConnection)
	}

	var resp relayResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode relay response: %v", err)
	}
	if resp.Status != http.StatusCreated {
		t.Errorf("relay status = %d, want %d", resp.Status, http.StatusCreated)
	}
	body, err := base64.StdEncoding.DecodeString(resp.Body)
	if err != nil {
		t.Fatalf("response body is not base64: %v", err)
	}
	if string(body) != "target response" {
		t.Errorf("decoded body = %q, want target response", body)
	}
	if resp.Headers["x-reply"][0] != "ok" {
		t.Errorf("x-reply header = %v, want ok", resp.Headers["x-reply"])
	}
	if got := resp.Headers["set-cookie"]; len(got) != 2 {
		t.Errorf("set-cookie values = %v, want two values", got)
	}
}

func TestHandleRelay_RedirectMode(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/final" {
			_, _ = w.Write([]byte("followed"))
			return
		}
		http.Redirect(w, r, "/final", http.StatusFound)
	}))
	defer target.Close()

	t.Run("manual", func(t *testing.T) {
		resp := postRelayRequest(t, target.Client(), relayRequest{
			URL:      target.URL + "/start",
			Method:   http.MethodGet,
			Redirect: false,
		})
		if resp.Status != http.StatusFound {
			t.Fatalf("relay status = %d, want %d", resp.Status, http.StatusFound)
		}
		if resp.Headers["location"][0] != "/final" {
			t.Errorf("location = %v, want /final", resp.Headers["location"])
		}
	})

	t.Run("follow", func(t *testing.T) {
		resp := postRelayRequest(t, target.Client(), relayRequest{
			URL:      target.URL + "/start",
			Method:   http.MethodGet,
			Redirect: true,
		})
		if resp.Status != http.StatusOK {
			t.Fatalf("relay status = %d, want %d", resp.Status, http.StatusOK)
		}
		body, _ := base64.StdEncoding.DecodeString(resp.Body)
		if string(body) != "followed" {
			t.Errorf("decoded body = %q, want followed", body)
		}
	})
}

func TestHandleRelay_RejectsInvalidJSON(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/relay", strings.NewReader(`not-json`))
	handleRelay(w, r, http.DefaultClient, "", time.Second)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	assertRelayErrorContains(t, w.Body.Bytes(), "bad json")
}

func TestHandleRelay_RejectsMissingMethodDefault(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()

	resp := postRelayRequest(t, target.Client(), relayRequest{
		URL: target.URL,
	})
	if resp.Status != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.Status)
	}
}

func TestHandleRelay_RejectsBadBase64Body(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/relay", strings.NewReader(`{"u":"https://example.com","b":"%%%"}`))
	handleRelay(w, r, http.DefaultClient, "", time.Second)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	assertRelayErrorContains(t, w.Body.Bytes(), "bad base64 body")
}

func postRelayRequest(t *testing.T, client *http.Client, payload relayRequest) relayResponse {
	t.Helper()
	raw, _ := json.Marshal(payload)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/relay", bytes.NewReader(raw))
	handleRelay(w, r, client, "", time.Second)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", w.Code, http.StatusOK, w.Body.String())
	}
	var resp relayResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode relay response: %v", err)
	}
	return resp
}

func assertRelayErrorContains(t *testing.T, raw []byte, want string) {
	t.Helper()
	var resp relayResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("decode relay error: %v", err)
	}
	if !strings.Contains(resp.Error, want) {
		t.Fatalf("error = %q, want containing %q", resp.Error, want)
	}
}
