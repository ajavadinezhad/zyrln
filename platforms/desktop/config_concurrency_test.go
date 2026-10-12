package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"zyrln/relay/core"
)

func postConfigTest(handler http.Handler, path string, body string) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body)))
	return response
}
func TestGUIConfigPartialSavePreservesOtherSettings(t *testing.T) {
	dir := t.TempDir()
	handler := newTestGUIHandler(t, dir, nil)
	initial := `{"listen":"127.0.0.1:18085","socks-listen":"127.0.0.1:11080","direct-enabled":"true"}`
	if r := postConfigTest(handler, "/api/config", initial); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	if r := postConfigTest(handler, "/api/config", `{"auth-key":"new-key"}`); r.Code != 200 {
		t.Fatal(r.Body.String())
	}
	got := loadConfig(filepath.Join(dir, "config.env"))
	if got["listen"] != "127.0.0.1:18085" || got["socks-listen"] != "127.0.0.1:11080" || got["direct-enabled"] != "true" || got["auth-key"] != "new-key" {
		t.Fatalf("settings lost: %v", got)
	}
}
func TestGUIConcurrentConfigAndSettingsPreserveListeners(t *testing.T) {
	original := core.GetDirectEnabled()
	t.Cleanup(func() { core.SetDirectEnabled(original) })
	dir := t.TempDir()
	handler := newTestGUIHandler(t, dir, nil)
	body := `{"listen":"127.0.0.1:18085","socks-listen":"127.0.0.1:11080","auth-key":"test-key"}`
	for iteration := 0; iteration < 100; iteration++ {
		var wg sync.WaitGroup
		for path, payload := range map[string]string{"/api/config": body, "/api/settings": `{"directEnabled":true}`} {
			wg.Add(1)
			go func(path, payload string) {
				defer wg.Done()
				r := postConfigTest(handler, path, payload)
				if r.Code != 200 {
					t.Errorf("%s: %d %s", path, r.Code, r.Body.String())
				}
			}(path, payload)
		}
		wg.Wait()
		got := loadConfig(filepath.Join(dir, "config.env"))
		if got["listen"] != "127.0.0.1:18085" || got["socks-listen"] != "127.0.0.1:11080" || got["direct-enabled"] != "true" || got["auth-key"] != "test-key" {
			t.Fatalf("iteration %d lost settings: %v", iteration, got)
		}
	}
}
func TestGUIConcurrentProfileCreationRetainsEveryProfile(t *testing.T) {
	dir := t.TempDir()
	handler := newTestGUIHandler(t, dir, nil)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			body := fmt.Sprintf(`{"id":"profile-%d","name":"test","config":{"auth-key":"dummy"}}`, i)
			r := postConfigTest(handler, "/api/profiles", body)
			if r.Code != 200 {
				t.Errorf("save: %s", r.Body.String())
			}
		}(i)
	}
	wg.Wait()
	if got := len(loadProfiles(filepath.Join(dir, "config.env"))); got != 20 {
		t.Fatalf("retained %d of 20 profiles", got)
	}
}
func TestGUISettingsFailedSaveDoesNotChangeRuntime(t *testing.T) {
	original := core.GetDirectEnabled()
	t.Cleanup(func() { core.SetDirectEnabled(original) })
	dir := t.TempDir()
	// A directory in place of the config makes atomic replacement fail reliably.
	if err := os.Mkdir(filepath.Join(dir, "config.env"), 0700); err != nil {
		t.Fatal(err)
	}
	handler := newTestGUIHandler(t, dir, nil)
	body, _ := json.Marshal(map[string]bool{"directEnabled": !original})
	r := postConfigTest(handler, "/api/settings", string(body))
	if r.Code != 500 {
		t.Fatalf("save failure hidden: %d", r.Code)
	}
	if core.GetDirectEnabled() != original {
		t.Fatal("runtime setting changed despite failed save")
	}
}
func TestAtomicConfigSavePreservesPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not enforced on Windows")
	}
	path := filepath.Join(t.TempDir(), "config.env")
	if err := os.WriteFile(path, []byte("auth-key=old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := saveConfig(path, map[string]string{"auth-key": "new"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permissions changed to %o", info.Mode().Perm())
	}
}
