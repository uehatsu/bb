package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// HTTPError is returned for non-2xx responses.
type HTTPError struct {
	StatusCode int
	Method     string
	URL        string
	Message    string // error.message from the Bitbucket error envelope
	Detail     string // error.detail
	Body       string // raw body (truncated), for --verbose
}

func (e *HTTPError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "HTTP %d", e.StatusCode)
	if e.Message != "" {
		fmt.Fprintf(&b, ": %s", e.Message)
	} else {
		fmt.Fprintf(&b, ": %s", http.StatusText(e.StatusCode))
	}
	if e.Detail != "" && e.Detail != e.Message {
		fmt.Fprintf(&b, " (%s)", e.Detail)
	}
	fmt.Fprintf(&b, " [%s %s]", e.Method, e.URL)
	return b.String()
}

// IsNotFound reports whether the error is a 404.
func (e *HTTPError) IsNotFound() bool { return e.StatusCode == http.StatusNotFound }

// bitbucketErrorEnvelope matches {"type":"error","error":{"message":...,"detail":...}}.
type bitbucketErrorEnvelope struct {
	Type  string `json:"type"`
	Error struct {
		Message string `json:"message"`
		Detail  string `json:"detail"`
	} `json:"error"`
}

const maxErrorBody = 4096

func newHTTPError(resp *http.Response) *HTTPError {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	e := &HTTPError{
		StatusCode: resp.StatusCode,
		Method:     resp.Request.Method,
		URL:        maskURL(resp.Request.URL),
		Body:       string(body),
	}
	var env bitbucketErrorEnvelope
	if json.Unmarshal(body, &env) == nil && env.Error.Message != "" {
		e.Message = env.Error.Message
		e.Detail = env.Error.Detail
	} else if s := strings.TrimSpace(string(body)); s != "" && !strings.HasPrefix(s, "<") && len(s) < 200 {
		e.Message = s
	}
	return e
}
