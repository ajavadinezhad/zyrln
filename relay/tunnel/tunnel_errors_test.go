package tunnel

import (
	"errors"
	"testing"
)

func TestBatchErrorNeedsSequential(t *testing.T) {
	if !batchErrorNeedsSequential(errors.New("bad url")) {
		t.Fatal("expected bad url to be sequential")
	}
	if batchErrorNeedsSequential(errors.New("unauthorized")) {
		t.Fatal("unauthorized should not be sequential")
	}
}

func TestIsRetryableAppsScriptError(t *testing.T) {
	tests := []struct {
		msg   string
		retry bool
	}{
		{"Exception: Service invoked too many times for one day: urlfetch.", true},
		{"quota exceeded", true},
		{"unauthorized", false},
		{"bad request", false},
		{"simulated failure", false},
	}
	for _, tc := range tests {
		if got := isRetryableAppsScriptError(tc.msg); got != tc.retry {
			t.Errorf("isRetryableAppsScriptError(%q) = %v, want %v", tc.msg, got, tc.retry)
		}
	}
}

func TestTunnelBodyShouldRetry_QuotaEnvelope(t *testing.T) {
	raw := []byte(`{"e":"Exception: Service invoked too many times for one day: urlfetch."}`)
	retry, err := tunnelBodyShouldRetry(raw)
	if !retry || err == nil {
		t.Fatalf("retry=%v err=%v", retry, err)
	}
}

func TestTunnelBodyShouldRetry_ErrorField(t *testing.T) {
	raw := []byte(`{"ok":false,"e":"quota"}`)
	retry, err := tunnelBodyShouldRetry(raw)
	if !retry || err == nil {
		t.Fatalf("retry=%v err=%v", retry, err)
	}
}

func TestTunnelBodyShouldRetry_OKBatch(t *testing.T) {
	raw := []byte(`{"results":[{"ok":true},{"ok":true,"data":""}]}`)
	retry, err := tunnelBodyShouldRetry(raw)
	if retry || err != nil {
		t.Fatalf("retry=%v err=%v", retry, err)
	}
}

func TestTunnelBodyShouldRetry_EmptyAndHTML(t *testing.T) {
	retry, err := tunnelBodyShouldRetry([]byte("  "))
	if !retry || err == nil {
		t.Fatalf("empty: retry=%v err=%v", retry, err)
	}
	retry, err = tunnelBodyShouldRetry([]byte("<html>quota</html>"))
	if !retry || err == nil {
		t.Fatalf("html: retry=%v err=%v", retry, err)
	}
}

func TestTunnelBodyShouldRetry_BatchItemQuota(t *testing.T) {
	raw := []byte(`{"results":[{"ok":false,"e":"quota exceeded"}]}`)
	retry, err := tunnelBodyShouldRetry(raw)
	if !retry || err == nil {
		t.Fatalf("retry=%v err=%v", retry, err)
	}
}

func TestTunnelBodyShouldRetry_BatchItemFatal(t *testing.T) {
	raw := []byte(`{"results":[{"ok":false,"e":"unauthorized"}]}`)
	retry, err := tunnelBodyShouldRetry(raw)
	if retry || err == nil {
		t.Fatalf("retry=%v err=%v", retry, err)
	}
}

func TestTunnelBodyShouldRetry_BareOKFalse(t *testing.T) {
	raw := []byte(`{"ok":false}`)
	retry, err := tunnelBodyShouldRetry(raw)
	if retry || err == nil {
		t.Fatalf("retry=%v err=%v", retry, err)
	}
}

func TestNormalizeHostPort(t *testing.T) {
	if got := NormalizeHostPort("example.com", "443"); got != "example.com:443" {
		t.Fatalf("host only = %q", got)
	}
	if got := NormalizeHostPort("example.com:8080", "443"); got != "example.com:8080" {
		t.Fatalf("with port = %q", got)
	}
	if got := NormalizeHostPort("", "443"); got != "" {
		t.Fatalf("empty = %q", got)
	}
}

func TestTunnelHTMLQuotaClassification(t *testing.T) {
	cases := []struct{ body, want string }{
		{"<html>Service invoked too many times for one day: urlfetch.</html>", "Apps Script quota exceeded"},
		{"<html>Quota exceeded</html>", "Apps Script quota exceeded"},
		{"<html>Service invoked too many times: UrlFetch.</html>", "Apps Script quota exceeded"},
		{"<html>Limit exceeded: URLFetch URL Length.</html>", "Apps Script returned HTML instead of JSON"},
		{"<html>Service Unavailable</html>", "Apps Script returned HTML instead of JSON"},
		{"<html>Sign in to continue</html>", "Apps Script returned HTML instead of JSON"},
		{"<html>Service invoked too many times in a short time</html>", "Apps Script returned HTML instead of JSON"},
	}
	for _, c := range cases {
		retry, err := tunnelBodyShouldRetry([]byte(c.body))
		if !retry || err == nil || err.Error() != c.want {
			t.Errorf("%q: retry=%v err=%v", c.body, retry, err)
		}
	}
}
