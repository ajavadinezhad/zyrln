package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"zyrln/relay/core"
)

// --- loadConfig ---

func TestLoadConfig_BasicParsing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.env")
	os.WriteFile(path, []byte("foo = bar\nbaz=qux\n"), 0644)

	got := loadConfig(path)
	if got["foo"] != "bar" {
		t.Errorf("foo = %q, want bar", got["foo"])
	}
	if got["baz"] != "qux" {
		t.Errorf("baz = %q, want qux", got["baz"])
	}
}

func TestLoadConfig_IgnoresComments(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.env")
	os.WriteFile(path, []byte("# comment\nkey=value\n"), 0644)

	got := loadConfig(path)
	if _, ok := got["# comment"]; ok {
		t.Error("comment line should be ignored")
	}
	if got["key"] != "value" {
		t.Errorf("key = %q, want value", got["key"])
	}
}

func TestLoadConfig_IgnoresEmptyLines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.env")
	os.WriteFile(path, []byte("\n\nkey=value\n\n"), 0644)

	got := loadConfig(path)
	if len(got) != 1 {
		t.Errorf("expected 1 entry, got %d", len(got))
	}
}

func TestLoadConfig_ValueWithEquals(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cfg.env")
	os.WriteFile(path, []byte("token=abc=def=ghi\n"), 0644)

	got := loadConfig(path)
	if got["token"] != "abc=def=ghi" {
		t.Errorf("token = %q, want abc=def=ghi", got["token"])
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	got := loadConfig("/nonexistent/path/config.env")
	if got != nil && len(got) != 0 {
		t.Error("expected nil or empty map for missing file")
	}
}

func TestShouldStartGUIByDefault_WindowsNoArgs(t *testing.T) {
	if !shouldStartGUIByDefault("windows", []string{"zyrln-windows-amd64.exe"}) {
		t.Error("expected Windows no-arg launch to start GUI")
	}
}

func TestShouldStartGUIByDefault_PreservesExplicitCLIArgs(t *testing.T) {
	if shouldStartGUIByDefault("windows", []string{"zyrln-windows-amd64.exe", "-init-ca"}) {
		t.Error("expected explicit Windows CLI args to preserve CLI mode")
	}
}

func TestShouldStartGUIByDefault_NonWindows(t *testing.T) {
	if shouldStartGUIByDefault("linux", []string{"zyrln-linux-amd64"}) {
		t.Error("expected non-Windows no-arg launch to preserve existing default mode")
	}
}

func TestShouldStartGUIByDefault_EmptyArgsEdgeCase(t *testing.T) {
	if !shouldStartGUIByDefault("windows", []string{}) {
		t.Error("expected Windows with empty args to also trigger GUI")
	}
}

// --- filterProbes ---

func TestFilterProbes_EmptyCategory(t *testing.T) {
	probes := []probe{
		{ID: "a", Category: "baseline"},
		{ID: "b", Category: "api"},
	}
	got := filterProbes(probes, "")
	if len(got) != 2 {
		t.Errorf("expected all probes, got %d", len(got))
	}
}

func TestFilterProbes_MatchesCategory(t *testing.T) {
	probes := []probe{
		{ID: "a", Category: "baseline"},
		{ID: "b", Category: "api"},
		{ID: "c", Category: "baseline"},
	}
	got := filterProbes(probes, "baseline")
	if len(got) != 2 {
		t.Errorf("expected 2, got %d", len(got))
	}
}

func TestFilterProbes_MultipleCategories(t *testing.T) {
	probes := []probe{
		{ID: "a", Category: "baseline"},
		{ID: "b", Category: "api"},
		{ID: "c", Category: "serverless"},
	}
	got := filterProbes(probes, "baseline,api")
	if len(got) != 2 {
		t.Errorf("expected 2, got %d", len(got))
	}
}

func TestFilterProbes_CaseInsensitive(t *testing.T) {
	probes := []probe{
		{ID: "a", Category: "Baseline"},
	}
	got := filterProbes(probes, "baseline")
	if len(got) != 1 {
		t.Errorf("expected 1, got %d", len(got))
	}
}

func TestFilterProbes_NoMatch(t *testing.T) {
	probes := []probe{
		{ID: "a", Category: "baseline"},
	}
	got := filterProbes(probes, "api")
	if len(got) != 0 {
		t.Errorf("expected 0, got %d", len(got))
	}
}

// --- addQuery ---

func TestAddQuery_NoExistingQuery(t *testing.T) {
	got := addQuery("https://example.com/path", "a=1")
	if got != "https://example.com/path?a=1" {
		t.Errorf("got %q", got)
	}
}

func TestAddQuery_ExistingQuery(t *testing.T) {
	got := addQuery("https://example.com/path?x=1", "a=1")
	if got != "https://example.com/path?x=1&a=1" {
		t.Errorf("got %q", got)
	}
}

// --- summarize ---

func TestSummarize_Counts(t *testing.T) {
	results := []result{
		{OK: true, Probe: probe{Category: "baseline"}},
		{OK: true, Probe: probe{Category: "baseline"}},
		{OK: false, Probe: probe{Category: "api"}},
	}
	s := summarize(results)
	if s.Total != 3 {
		t.Errorf("Total = %d, want 3", s.Total)
	}
	if s.Reachable != 2 {
		t.Errorf("Reachable = %d, want 2", s.Reachable)
	}
	if s.Failed != 1 {
		t.Errorf("Failed = %d, want 1", s.Failed)
	}
	if s.Categories["baseline"] != 2 {
		t.Errorf("Categories[baseline] = %d, want 2", s.Categories["baseline"])
	}
}

func TestSummarize_Empty(t *testing.T) {
	s := summarize(nil)
	if s.Total != 0 || s.Reachable != 0 || s.Failed != 0 {
		t.Error("expected all zeros for empty results")
	}
}

// --- truncate ---

func TestTruncate_ShortString(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate short: got %q", got)
	}
}

func TestTruncate_LongString(t *testing.T) {
	if got := truncate("hello world", 8); got != "hello..." {
		t.Errorf("truncate long: got %q", got)
	}
}

func TestTruncate_ExactLength(t *testing.T) {
	if got := truncate("hello", 5); got != "hello" {
		t.Errorf("truncate exact: got %q", got)
	}
}

// --- preview ---

func TestPreview_Short(t *testing.T) {
	if got := preview([]byte("hello"), 100); got != "hello" {
		t.Errorf("got %q", got)
	}
}

func TestPreview_Truncated(t *testing.T) {
	got := preview([]byte("hello world extra"), 10)
	if !strings.HasSuffix(got, "...") {
		t.Errorf("expected ellipsis, got %q", got)
	}
	if len(got) != 10 {
		t.Errorf("expected len 10, got %d", len(got))
	}
}

func TestPreview_StripsNewlines(t *testing.T) {
	got := preview([]byte("line1\nline2\r\nline3"), 100)
	if strings.ContainsAny(got, "\n\r") {
		t.Errorf("expected no newlines, got %q", got)
	}
}

func TestPreview_Empty(t *testing.T) {
	if got := preview(nil, 10); got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

// --- parseProxyConfig ---

func TestParseProxyConfig_Direct(t *testing.T) {
	for _, v := range []string{"direct", "Direct", "DIRECT", "none", "", "  "} {
		cfg, err := parseProxyConfig(v)
		if err != nil {
			t.Errorf("parseProxyConfig(%q): unexpected error: %v", v, err)
		}
		if cfg.label != "direct" {
			t.Errorf("parseProxyConfig(%q).label = %q, want direct", v, cfg.label)
		}
		if cfg.proxyFunc != nil {
			t.Errorf("parseProxyConfig(%q): expected nil proxyFunc for direct", v)
		}
	}
}

func TestParseProxyConfig_ValidURL(t *testing.T) {
	cfg, err := parseProxyConfig("http://127.0.0.1:8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.proxyFunc == nil {
		t.Error("expected non-nil proxyFunc")
	}
	if !strings.Contains(cfg.proxyHost, "8080") {
		t.Errorf("proxyHost = %q, expected port 8080", cfg.proxyHost)
	}
}

func TestParseProxyConfig_InvalidURL(t *testing.T) {
	_, err := parseProxyConfig("not-a-url")
	if err == nil {
		t.Error("expected error for URL without scheme/host")
	}
}

// --- frontedAppScriptProbes ---

func TestFrontedAppScriptProbes_RejectsHTTP(t *testing.T) {
	_, err := frontedAppScriptProbes("http://script.google.com/x", "www.google.com", "", "https://t.com")
	if err == nil {
		t.Error("expected error for http scheme")
	}
}

func TestFrontedAppScriptProbes_RejectsMissingHost(t *testing.T) {
	_, err := frontedAppScriptProbes("https:///path", "www.google.com", "", "https://t.com")
	if err == nil {
		t.Error("expected error for missing host")
	}
}

func TestFrontedAppScriptProbes_BasicProbes(t *testing.T) {
	probes, err := frontedAppScriptProbes(
		"https://script.google.com/macros/s/ABC/exec",
		"www.google.com", "", "https://t.com",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(probes) < 2 {
		t.Errorf("expected at least 2 probes, got %d", len(probes))
	}
	for _, p := range probes {
		if !strings.Contains(p.URL, "www.google.com") {
			t.Errorf("probe URL %q should contain front domain", p.URL)
		}
		if p.Host != "script.google.com" {
			t.Errorf("probe Host = %q, want script.google.com", p.Host)
		}
	}
}

func TestFrontedAppScriptProbes_WithAuthKeyAddsRelayProbe(t *testing.T) {
	probes, err := frontedAppScriptProbes(
		"https://script.google.com/macros/s/ABC/exec",
		"www.google.com", "secretkey", "https://t.com",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(probes) < 3 {
		t.Errorf("expected 3+ probes with auth key, got %d", len(probes))
	}
	found := false
	for _, p := range probes {
		if p.ID == "fronted-relay-post" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected fronted-relay-post probe when auth key provided")
	}
}

func TestFrontedAppScriptProbes_DefaultFrontDomain(t *testing.T) {
	probes, err := frontedAppScriptProbes(
		"https://script.google.com/macros/s/ABC/exec",
		"", "", "https://t.com",
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range probes {
		if !strings.Contains(p.URL, "www.google.com") {
			t.Errorf("expected default front domain www.google.com in URL, got %q", p.URL)
		}
	}
}

// --- GUI API ---

func TestGUIStatusDefault(t *testing.T) {
	resetGUIStateForTest(t)
	handler := newTestGUIHandler(t, t.TempDir(), nil)

	var got map[string]any
	getJSON(t, handler, "/api/status", &got)

	if got["running"] != false {
		t.Fatalf("running = %v, want false", got["running"])
	}
	if got["uptime"] != "00:00:00" {
		t.Fatalf("uptime = %v, want 00:00:00", got["uptime"])
	}
}

func TestGUIConfigRoundTrip(t *testing.T) {
	resetGUIStateForTest(t)
	dir := t.TempDir()
	handler := newTestGUIHandler(t, dir, nil)

	body := bytes.NewBufferString(`{"fronted-appscript-url":"https://script.google.com/macros/s/ABC/exec","auth-key":"secret","listen":"127.0.0.1:8085"}`)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/config", body))
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/config status = %d, body=%s", resp.Code, resp.Body.String())
	}

	var got map[string]string
	getJSON(t, handler, "/api/config", &got)
	if got["fronted-appscript-url"] != "https://script.google.com/macros/s/ABC/exec" {
		t.Errorf("url = %q", got["fronted-appscript-url"])
	}
	if got["auth-key"] != "secret" {
		t.Errorf("auth-key = %q", got["auth-key"])
	}
	if got["listen"] != "127.0.0.1:8085" {
		t.Errorf("listen = %q", got["listen"])
	}
}

func TestGUIProfilesSaveActivateDelete(t *testing.T) {
	resetGUIStateForTest(t)
	dir := t.TempDir()
	writeConfigForTest(t, dir, "listen = 127.0.0.1:9090\nsocks-listen = 127.0.0.1:1090\n")
	handler := newTestGUIHandler(t, dir, nil)

	body := bytes.NewBufferString(`{"name":"Work","config":{"fronted-appscript-url":"https://script.google.com/macros/s/ABC/exec","auth-key":"secret","listen":"127.0.0.1:8085","socks-listen":"127.0.0.1:1080"}}`)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/profiles", body))
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/profiles status = %d, body=%s", resp.Code, resp.Body.String())
	}

	var saved desktopProfile
	if err := json.Unmarshal(resp.Body.Bytes(), &saved); err != nil {
		t.Fatalf("decode saved profile: %v", err)
	}
	if saved.ID == "" || saved.Name != "Work" {
		t.Fatalf("unexpected saved profile: %#v", saved)
	}
	if saved.Config["listen"] != "" || saved.Config["socks-listen"] != "" {
		t.Fatalf("profile saved global listen settings: %#v", saved.Config)
	}

	var profiles []desktopProfile
	getJSON(t, handler, "/api/profiles", &profiles)
	if len(profiles) != 1 || profiles[0].ID != saved.ID {
		t.Fatalf("profiles = %#v, want saved profile", profiles)
	}

	body = bytes.NewBufferString(fmt.Sprintf(`{"id":%q}`, saved.ID))
	resp = httptest.NewRecorder()
	guiMu.Lock()
	guiProxyServer = &http.Server{}
	guiMu.Unlock()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/profiles/activate", body))
	if resp.Code != http.StatusConflict {
		t.Fatalf("activate while running status = %d, want %d", resp.Code, http.StatusConflict)
	}
	guiMu.Lock()
	guiProxyServer = nil
	guiMu.Unlock()

	body = bytes.NewBufferString(fmt.Sprintf(`{"id":%q}`, saved.ID))
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/profiles/activate", body))
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/profiles/activate status = %d, body=%s", resp.Code, resp.Body.String())
	}

	var cfg map[string]string
	getJSON(t, handler, "/api/config", &cfg)
	if cfg["fronted-appscript-url"] != "https://script.google.com/macros/s/ABC/exec" {
		t.Fatalf("activated url = %q", cfg["fronted-appscript-url"])
	}
	if cfg["listen"] != "127.0.0.1:9090" {
		t.Fatalf("activated listen = %q", cfg["listen"])
	}
	if cfg["socks-listen"] != "127.0.0.1:1090" {
		t.Fatalf("activated socks-listen = %q", cfg["socks-listen"])
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodDelete, "/api/profiles?id="+saved.ID, nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("DELETE /api/profiles status = %d, body=%s", resp.Code, resp.Body.String())
	}
	getJSON(t, handler, "/api/profiles", &profiles)
	if len(profiles) != 0 {
		t.Fatalf("profiles after delete = %#v, want empty", profiles)
	}
}

func TestGUIExportReadsConfig(t *testing.T) {
	resetGUIStateForTest(t)
	dir := t.TempDir()
	writeConfigForTest(t, dir, "fronted-appscript-url = https://script.google.com/macros/s/ABC/exec\nauth-key = secret\n")
	handler := newTestGUIHandler(t, dir, nil)

	var got map[string]string
	getJSON(t, handler, "/api/export", &got)

	if got["url"] != "https://script.google.com/macros/s/ABC/exec" {
		t.Errorf("url = %q", got["url"])
	}
	if got["key"] != "secret" {
		t.Errorf("key = %q", got["key"])
	}
}

func TestGUIInitCAAndDownload(t *testing.T) {
	resetGUIStateForTest(t)
	dir := t.TempDir()
	handler := newTestGUIHandler(t, dir, nil)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/init-ca", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/init-ca status = %d, body=%s", resp.Code, resp.Body.String())
	}
	var initResp map[string]string
	if err := json.Unmarshal(resp.Body.Bytes(), &initResp); err != nil {
		t.Fatalf("decode init response: %v", err)
	}
	if initResp["status"] != "ok" || initResp["serial"] == "" || initResp["serial"] == "unknown" {
		t.Fatalf("unexpected init response: %#v", initResp)
	}

	download := httptest.NewRecorder()
	handler.ServeHTTP(download, httptest.NewRequest(http.MethodGet, "/api/download-ca", nil))
	if download.Code != http.StatusOK {
		t.Fatalf("GET /api/download-ca status = %d, body=%s", download.Code, download.Body.String())
	}
	if got := download.Header().Get("Cache-Control"); !strings.Contains(got, "no-store") {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
	if !strings.Contains(download.Body.String(), "BEGIN CERTIFICATE") {
		t.Error("downloaded CA does not look like PEM")
	}
}

func TestGUIStartRequiresConfig(t *testing.T) {
	resetGUIStateForTest(t)
	core.SetDirectEnabled(false) // direct mode must be off for relay-required validation
	t.Cleanup(func() { core.SetDirectEnabled(true) })
	dir := t.TempDir()
	handler := newTestGUIHandler(t, dir, nil)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/start", nil))
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("empty config status = %d, want %d", resp.Code, http.StatusBadRequest)
	}

	writeConfigForTest(t, dir, "fronted-appscript-url = https://script.google.com/macros/s/ABC/exec\n")
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/start", nil))
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("missing auth key status = %d, want %d", resp.Code, http.StatusBadRequest)
	}
}

func TestGUIStartRequiresExistingCA(t *testing.T) {
	resetGUIStateForTest(t)
	dir := t.TempDir()
	writeConfigForTest(t, dir, "fronted-appscript-url = https://script.google.com/macros/s/ABC/exec\nauth-key = secret\n")
	handler := newTestGUIHandler(t, dir, nil)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/start", nil))
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("missing CA status = %d, want %d; body=%s", resp.Code, http.StatusBadRequest, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "Install certificate first") {
		t.Fatalf("missing CA body = %q", resp.Body.String())
	}
}

func TestGUIStartStopWithInjectedStarter(t *testing.T) {
	resetGUIStateForTest(t)
	dir := t.TempDir()
	writeConfigForTest(t, dir, "fronted-appscript-url = https://script.google.com/macros/s/ABC/exec\nauth-key = secret\nlisten = 127.0.0.1:9090\n")

	var started atomic.Bool
	starter := func(listen, socksListen string, urls []string, key string, ca *core.CertAuthority) (*http.Server, net.Listener, *core.SOCKSServer, net.Listener, error) {
		started.Store(true)
		if listen != "127.0.0.1:9090" {
			t.Errorf("listen = %q", listen)
		}
		if socksListen != "127.0.0.1:1080" {
			t.Errorf("socksListen = %q", socksListen)
		}
		if len(urls) != 1 || urls[0] != "https://script.google.com/macros/s/ABC/exec" {
			t.Errorf("urls = %v", urls)
		}
		if key != "secret" {
			t.Errorf("key = %q", key)
		}
		if ca == nil {
			t.Error("ca is nil")
		}
		return &http.Server{}, noopListener{}, &core.SOCKSServer{}, noopListener{}, nil
	}
	handler := newTestGUIHandler(t, dir, starter)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/init-ca", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/init-ca status = %d, body=%s", resp.Code, resp.Body.String())
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/start", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/start status = %d, body=%s", resp.Code, resp.Body.String())
	}
	if !started.Load() {
		t.Fatal("starter was not called")
	}

	var status map[string]any
	getJSON(t, handler, "/api/status", &status)
	if status["running"] != true {
		t.Fatalf("running after start = %v, want true", status["running"])
	}

	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/stop", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/stop status = %d, body=%s", resp.Code, resp.Body.String())
	}
	getJSON(t, handler, "/api/status", &status)
	if status["running"] != false {
		t.Fatalf("running after stop = %v, want false", status["running"])
	}
}

func TestGUILogsEmpty(t *testing.T) {
	resetGUIStateForTest(t)
	resetGUILogForTest(t)
	dir := t.TempDir()
	handler := newTestGUIHandler(t, dir, nil)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/logs?seq=0", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d", resp.Code)
	}
	if body := resp.Body.String(); body != "" {
		t.Errorf("expected empty body, got %q", body)
	}
	if seq := resp.Header().Get("X-Log-Seq"); seq != "0" {
		t.Errorf("X-Log-Seq = %q, want 0", seq)
	}
}

func TestGUILogsReturnNewEntries(t *testing.T) {
	resetGUIStateForTest(t)
	resetGUILogForTest(t)
	dir := t.TempDir()
	handler := newTestGUIHandler(t, dir, nil)

	guiEmitLog("info", "hello world")
	guiEmitLog("error", "something broke")

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/logs?seq=0", nil))
	body := resp.Body.String()
	if !strings.Contains(body, "info\thello world") {
		t.Errorf("missing info line, got: %q", body)
	}
	if !strings.Contains(body, "error\tsomething broke") {
		t.Errorf("missing error line, got: %q", body)
	}
	seq := resp.Header().Get("X-Log-Seq")
	if seq != "2" {
		t.Errorf("X-Log-Seq = %q, want 2", seq)
	}
}

func TestGUILogsSeqCursorReturnsOnlyNew(t *testing.T) {
	resetGUIStateForTest(t)
	resetGUILogForTest(t)
	dir := t.TempDir()
	handler := newTestGUIHandler(t, dir, nil)

	guiEmitLog("info", "first")
	guiEmitLog("info", "second")

	// First poll: get both
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/logs?seq=0", nil))
	seq := resp.Header().Get("X-Log-Seq") // should be "2"

	// Add another entry
	guiEmitLog("system", "third")

	// Second poll: only "third" should appear
	resp = httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/logs?seq="+seq, nil))
	body := resp.Body.String()
	if strings.Contains(body, "first") || strings.Contains(body, "second") {
		t.Errorf("cursor should skip already-seen entries, got: %q", body)
	}
	if !strings.Contains(body, "system\tthird") {
		t.Errorf("missing new entry, got: %q", body)
	}
}

func TestGUILogsNegativeSeqClamped(t *testing.T) {
	resetGUIStateForTest(t)
	resetGUILogForTest(t)
	dir := t.TempDir()
	handler := newTestGUIHandler(t, dir, nil)

	guiEmitLog("info", "entry")

	// negative seq should be treated as 0 — all entries returned
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/logs?seq=-99", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d", resp.Code)
	}
	body := resp.Body.String()
	if !strings.Contains(body, "info\tentry") {
		t.Errorf("expected entry with negative seq clamped, got: %q", body)
	}
}

func TestGUILogsResetOnStart(t *testing.T) {
	resetGUIStateForTest(t)
	resetGUILogForTest(t)
	dir := t.TempDir()
	writeConfigForTest(t, dir, "fronted-appscript-url = https://script.google.com/macros/s/ABC/exec\nauth-key = secret\n")

	starter := func(string, string, []string, string, *core.CertAuthority) (*http.Server, net.Listener, *core.SOCKSServer, net.Listener, error) {
		return &http.Server{}, noopListener{}, &core.SOCKSServer{}, noopListener{}, nil
	}
	handler := newTestGUIHandler(t, dir, starter)

	// emit logs before start (simulates a previous run)
	guiEmitLog("info", "old log")
	oldSeq := guiLogSeq

	// init-ca then start
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/init-ca", nil))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/start", nil))

	guiLogMu.Lock()
	newSeq := guiLogSeq
	bufLen := len(guiLogBuf)
	guiLogMu.Unlock()

	if newSeq != 0 {
		t.Errorf("guiLogSeq after start = %d, want 0 (reset)", newSeq)
	}
	if bufLen != 0 {
		t.Errorf("guiLogBuf len after start = %d, want 0 (reset)", bufLen)
	}
	_ = oldSeq
}

func TestIsRedirect(t *testing.T) {
	redirectCodes := []int{301, 302, 303, 307, 308}
	for _, code := range redirectCodes {
		if !isRedirect(code) {
			t.Errorf("isRedirect(%d) = false, want true", code)
		}
	}
	nonRedirect := []int{200, 204, 400, 404, 500}
	for _, code := range nonRedirect {
		if isRedirect(code) {
			t.Errorf("isRedirect(%d) = true, want false", code)
		}
	}
}

func TestCompactError_URLError(t *testing.T) {
	inner := fmt.Errorf("connection refused")
	wrapped := &url.Error{Op: "Get", URL: "http://example.com", Err: inner}
	got := compactError(wrapped)
	if got != "connection refused" {
		t.Errorf("compactError url.Error = %q, want %q", got, "connection refused")
	}
}

func TestCompactError_PlainError(t *testing.T) {
	err := fmt.Errorf("something\nwent wrong")
	got := compactError(err)
	if strings.Contains(got, "\n") {
		t.Errorf("compactError should strip newlines, got: %q", got)
	}
	if !strings.Contains(got, "something") {
		t.Errorf("compactError should preserve message, got: %q", got)
	}
}

func TestCompactError_Nil(t *testing.T) {
	if got := compactError(nil); got != "" {
		t.Errorf("compactError(nil) = %q, want empty", got)
	}
}

func TestProfileDisplayName_ExplicitName(t *testing.T) {
	got := profileDisplayName("My Profile", map[string]string{})
	if got != "My Profile" {
		t.Errorf("got %q, want %q", got, "My Profile")
	}
}

func TestProfileDisplayName_FromURL(t *testing.T) {
	cfg := map[string]string{"fronted-appscript-url": "https://script.google.com/macros/s/ABC/exec"}
	got := profileDisplayName("", cfg)
	if got != "script.google.com" {
		t.Errorf("got %q, want script.google.com", got)
	}
}

func TestProfileDisplayName_FallbackNoURL(t *testing.T) {
	got := profileDisplayName("  ", map[string]string{})
	if got != "Profile" {
		t.Errorf("got %q, want Profile", got)
	}
}

func TestApplyProfileConfig_MergesProfileOverrides(t *testing.T) {
	base := map[string]string{
		"listen":                 "127.0.0.1:8085",
		"fronted-appscript-url":  "https://old.example/exec",
		"auth-key":               "old-key",
	}
	profile := map[string]string{
		"name":                   "Work",
		"listen":                 "127.0.0.1:9999",
		"fronted-appscript-url":  "https://new.example/exec",
		"auth-key":               "new-key",
	}
	got := applyProfileConfig(base, profile)
	if got["listen"] != "127.0.0.1:8085" {
		t.Errorf("listen = %q, want base preserved", got["listen"])
	}
	if got["fronted-appscript-url"] != "https://new.example/exec" {
		t.Errorf("url = %q", got["fronted-appscript-url"])
	}
	if got["auth-key"] != "new-key" {
		t.Errorf("auth-key = %q", got["auth-key"])
	}
}

func TestProfileConfig_StripsGlobalKeys(t *testing.T) {
	got := profileConfig(map[string]string{
		"listen":                "127.0.0.1:9090",
		"socks-listen":          "127.0.0.1:1090",
		"fronted-appscript-url": "https://script.google.com/macros/s/ABC/exec",
	})
	if _, ok := got["listen"]; ok {
		t.Error("listen should be stripped from profile config")
	}
	if _, ok := got["socks-listen"]; ok {
		t.Error("socks-listen should be stripped from profile config")
	}
	if got["fronted-appscript-url"] == "" {
		t.Error("expected relay url in profile config")
	}
}

func TestParseURLList_CommaSeparated(t *testing.T) {
	got := parseURLList(" https://a.example/exec , https://b.example/exec ")
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0] != "https://a.example/exec" || got[1] != "https://b.example/exec" {
		t.Fatalf("urls = %v", got)
	}
}

func TestGUISettingsDirectEnabledRoundTrip(t *testing.T) {
	resetGUIStateForTest(t)
	orig := core.GetDirectEnabled()
	t.Cleanup(func() { core.SetDirectEnabled(orig) })

	dir := t.TempDir()
	handler := newTestGUIHandler(t, dir, nil)

	var got map[string]any
	getJSON(t, handler, "/api/settings", &got)
	if got["directEnabled"] != true {
		t.Fatalf("default directEnabled = %v, want true", got["directEnabled"])
	}

	body := bytes.NewBufferString(`{"directEnabled":false}`)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/settings", body))
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/settings status = %d, body=%s", resp.Code, resp.Body.String())
	}
	if core.GetDirectEnabled() {
		t.Fatal("expected direct disabled after POST")
	}

	cfg := loadConfig(filepath.Join(dir, "config.env"))
	if cfg["direct-enabled"] != "false" {
		t.Fatalf("config direct-enabled = %q", cfg["direct-enabled"])
	}

	getJSON(t, handler, "/api/settings", &got)
	if got["directEnabled"] != false {
		t.Fatalf("directEnabled after POST = %v", got["directEnabled"])
	}
}

func TestGUIPingDirectMode(t *testing.T) {
	resetGUIStateForTest(t)
	dir := t.TempDir()
	handler := newTestGUIHandler(t, dir, nil)

	body := bytes.NewBufferString(`{"direct":true}`)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodPost, "/api/ping", body))
	if resp.Code != http.StatusOK {
		t.Fatalf("POST /api/ping status = %d, body=%s", resp.Code, resp.Body.String())
	}
	var got map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if _, ok := got["ok"]; !ok {
		t.Fatalf("missing ok field: %#v", got)
	}
}

func TestGUIDownloadCANotFound(t *testing.T) {
	resetGUIStateForTest(t)
	dir := t.TempDir()
	handler := newTestGUIHandler(t, dir, nil)

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/download-ca", nil))
	if resp.Code != http.StatusNotFound {
		t.Fatalf("GET /api/download-ca status = %d, want 404", resp.Code)
	}
}

func TestFrontedRedirectProbe_AbsoluteLocation(t *testing.T) {
	orig := probe{
		ID: "p1", Name: "Test", Category: "cat",
		URL: "https://www.google.com/path", FrontDomain: "www.google.com",
	}
	got, err := frontedRedirectProbe(orig, "https://accounts.google.com/signin")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Host != "accounts.google.com" {
		t.Errorf("Host = %q, want accounts.google.com", got.Host)
	}
	if !strings.Contains(got.URL, "www.google.com") {
		t.Errorf("URL should use front domain, got %q", got.URL)
	}
	if got.FrontDomain != "www.google.com" {
		t.Errorf("FrontDomain = %q, want www.google.com", got.FrontDomain)
	}
}

func TestFrontedRedirectProbe_RelativeLocation(t *testing.T) {
	orig := probe{
		ID: "p1", Name: "Test", Category: "cat",
		URL: "https://www.google.com/path", FrontDomain: "www.google.com",
	}
	got, err := frontedRedirectProbe(orig, "/newpath")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got.URL, "/newpath") {
		t.Errorf("URL should contain resolved path, got %q", got.URL)
	}
}

func TestWriteJSONReport(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "report.json")
	rep := report{
		GeneratedAt: "2024-01-01T00:00:00Z",
		Summary:     summary{Total: 1, Reachable: 1, Categories: map[string]int{"test": 1}},
	}
	if err := writeJSONReport(path, rep); err != nil {
		t.Fatalf("writeJSONReport: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var got report
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Summary.Total != 1 {
		t.Errorf("Total = %d, want 1", got.Summary.Total)
	}
}

func TestWriteBody(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "body.bin")
	data := []byte("hello world")
	if err := writeBody(path, data); err != nil {
		t.Fatalf("writeBody: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("got %q, want %q", got, data)
	}
}

func TestGuiEmitLog_RingBuffer(t *testing.T) {
	resetGUILogForTest(t)
	// Fill past the cap
	for i := 0; i < maxGUILogEntries+10; i++ {
		guiEmitLog("info", fmt.Sprintf("msg%d", i))
	}
	guiLogMu.Lock()
	length := len(guiLogBuf)
	seq := guiLogSeq
	guiLogMu.Unlock()
	if length != maxGUILogEntries {
		t.Errorf("buf len = %d, want %d", length, maxGUILogEntries)
	}
	if seq != maxGUILogEntries+10 {
		t.Errorf("seq = %d, want %d", seq, maxGUILogEntries+10)
	}
}

func resetGUILogForTest(t *testing.T) {
	t.Helper()
	guiLogMu.Lock()
	guiLogBuf = nil
	guiLogSeq = 0
	guiLogMu.Unlock()
}

func newTestGUIHandler(t *testing.T, dir string, starter guiProxyStarter) http.Handler {
	t.Helper()
	if starter == nil {
		starter = func(string, string, []string, string, *core.CertAuthority) (*http.Server, net.Listener, *core.SOCKSServer, net.Listener, error) {
			t.Fatal("unexpected proxy start")
			return nil, nil, nil, nil, nil
		}
	}
	configPath := filepath.Join(dir, "config.env")
	caCertPath := filepath.Join(dir, "certs", "zyrln-ca.pem")
	caKeyPath := filepath.Join(dir, "certs", "zyrln-ca-key.pem")
	return newGUIHandler(configPath, caCertPath, caKeyPath, starter, func(string) {})
}

func resetGUIStateForTest(t *testing.T) {
	t.Helper()
	guiMu.Lock()
	if guiProxyLn != nil {
		_ = guiProxyLn.Close()
	}
	if guiSOCKSLn != nil {
		_ = guiSOCKSLn.Close()
	}
	if guiProxyServer != nil {
		_ = guiProxyServer.Close()
	}
	guiProxyServer = nil
	guiProxyLn = nil
	guiSOCKSLn = nil
	guiProxyStartTime = time.Time{}
	guiMu.Unlock()
	atomic.StoreInt64(&guiRequestCount, 0)
}

func writeConfigForTest(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "config.env"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func getJSON(t *testing.T, handler http.Handler, path string, out any) {
	t.Helper()
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, path, nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("GET %s status = %d, body=%s", path, resp.Code, resp.Body.String())
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("decode %s JSON: %v; body=%s", path, err, data)
	}
}

type noopListener struct{}

func (noopListener) Accept() (net.Conn, error) { return nil, net.ErrClosed }
func (noopListener) Close() error              { return nil }
func (noopListener) Addr() net.Addr            { return noopAddr("noop") }

type noopAddr string

func (a noopAddr) Network() string { return string(a) }
func (a noopAddr) String() string  { return string(a) }
