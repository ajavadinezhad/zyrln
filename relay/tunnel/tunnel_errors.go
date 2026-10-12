package tunnel

import (
	"encoding/json"
	"fmt"
	"strings"
)

// isRetryableAppsScriptError reports whether we should try the next fronted script URL.
func isRetryableAppsScriptError(msg string) bool {
	m := strings.ToLower(strings.TrimSpace(msg))
	if m == "" {
		return false
	}
	if m == "unauthorized" || strings.Contains(m, "bad request") {
		return false
	}
	return strings.Contains(m, "urlfetch") ||
		strings.Contains(m, "too many times") ||
		strings.Contains(m, "quota") ||
		strings.Contains(m, "rate limit") ||
		strings.Contains(m, "service unavailable") ||
		strings.Contains(m, "html instead of json") ||
		strings.Contains(m, "session exists")
}

// tunnelBodyShouldRetry inspects an Apps Script HTTP body before treating it as success.
// Returns retry=true for quota/transient errors that may succeed on another deployment.
func tunnelBodyShouldRetry(raw []byte) (retry bool, err error) {
	trim := strings.TrimSpace(string(raw))
	if trim == "" {
		return true, fmt.Errorf("empty Apps Script response")
	}
	if strings.HasPrefix(trim, "<") {
		m := strings.ToLower(trim)
		if strings.Contains(m, "quota exceeded") || strings.Contains(m, "daily quota") ||
			strings.Contains(m, "service invoked too many times for one day") ||
			strings.Contains(m, "service invoked too many times: urlfetch") ||
			strings.Contains(m, "service using too much computer time for one day") {
			return true, fmt.Errorf("Apps Script quota exceeded")
		}
		return true, fmt.Errorf("Apps Script returned HTML instead of JSON")
	}

	var top struct {
		E       string           `json:"e"`
		OK      bool             `json:"ok"`
		Results []TunnelResponse `json:"results"`
	}
	if err := json.Unmarshal(raw, &top); err != nil {
		return true, fmt.Errorf("invalid tunnel JSON: %w", err)
	}

	if top.E != "" && len(top.Results) == 0 {
		err := fmt.Errorf("%s", top.E)
		return isRetryableAppsScriptError(top.E), err
	}

	if len(top.Results) > 0 {
		for _, r := range top.Results {
			if r.Error != "" {
				err := fmt.Errorf("%s", r.Error)
				if isRetryableAppsScriptError(r.Error) {
					return true, err
				}
				return false, err
			}
			if !r.OK {
				err := fmt.Errorf("tunnel request failed")
				if r.Error != "" {
					err = fmt.Errorf("%s", r.Error)
				}
				return false, err
			}
		}
		return false, nil
	}

	var single TunnelResponse
	if err := json.Unmarshal(raw, &single); err != nil {
		return true, fmt.Errorf("invalid tunnel JSON: %w", err)
	}
	if single.Error != "" {
		err := fmt.Errorf("%s", single.Error)
		return isRetryableAppsScriptError(single.Error), err
	}
	if !single.OK {
		return false, fmt.Errorf("tunnel request failed")
	}
	return false, nil
}

// batchErrorNeedsSequential reports whether batched Apps Script ops should be retried one-by-one.
// Older deployments handle single "t" ops but not "tb" batches (they return "bad url").
func batchErrorNeedsSequential(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "bad url") ||
		strings.Contains(msg, "invalid tunnel batch")
}
