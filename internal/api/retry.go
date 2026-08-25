package api

import (
	"context"
	"net/http"
	"strconv"
	"time"
)

const (
	maxRetries       = 3
	maxRetryAfter    = 60 * time.Second
	initialBackoff   = 1 * time.Second
	backoffMultipler = 2
)

// isIdempotent reports whether a request may be safely retried after a
// server error. POST/PATCH are excluded to avoid double-executing merges,
// pipeline runs, and PR creation.
func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodPut, http.MethodDelete:
		return true
	}
	return false
}

// retryDelay decides whether resp warrants a retry and how long to wait.
// 429 is retried for any method (the request was not executed); 5xx only
// for idempotent methods.
func retryDelay(resp *http.Response, attempt int, now time.Time) (time.Duration, bool) {
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
	case resp.StatusCode >= 500 && isIdempotent(resp.Request.Method):
	default:
		return 0, false
	}
	if attempt >= maxRetries {
		return 0, false
	}
	if d, ok := parseRetryAfter(resp.Header.Get("Retry-After"), now); ok {
		return clampDelay(d), true
	}
	d := initialBackoff
	for i := 0; i < attempt; i++ {
		d *= backoffMultipler
	}
	return clampDelay(d), true
}

func clampDelay(d time.Duration) time.Duration {
	if d < 0 {
		return 0
	}
	if d > maxRetryAfter {
		return maxRetryAfter
	}
	return d
}

// parseRetryAfter handles both delay-seconds and HTTP-date forms.
func parseRetryAfter(v string, now time.Time) (time.Duration, bool) {
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.ParseFloat(v, 64); err == nil {
		return time.Duration(secs * float64(time.Second)), true
	}
	if t, err := http.ParseTime(v); err == nil {
		return t.Sub(now), true
	}
	return 0, false
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
