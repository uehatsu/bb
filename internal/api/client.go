// Package api implements a thin, testable HTTP client for the Bitbucket
// Cloud REST API 2.0: authentication, JSON (de)serialization, cursor
// pagination via `next` links, rate-limit aware retries, and polling.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/uehatsu/bb/internal/build"
)

// DefaultBaseURL is the Bitbucket Cloud API root.
const DefaultBaseURL = "https://api.bitbucket.org/2.0"

// Client performs authenticated requests against the Bitbucket API.
type Client struct {
	BaseURL   *url.URL
	HTTP      *http.Client
	Auth      Authenticator
	UserAgent string
	Logger    io.Writer // when non-nil, request/response lines are logged
	LogBodies bool      // BB_DEBUG=2: also log response bodies
	NoRetry   bool      // BB_NO_RETRY=1: fail fast on 429/5xx
	now       func() time.Time
	sleep     func(context.Context, time.Duration) error
}

// Option customizes a Client.
type Option func(*Client)

// WithBaseURL overrides the API root (tests).
func WithBaseURL(u string) Option {
	return func(c *Client) {
		if p, err := url.Parse(u); err == nil {
			c.BaseURL = p
		}
	}
}

// WithHTTPClient sets the underlying http.Client.
func WithHTTPClient(h *http.Client) Option { return func(c *Client) { c.HTTP = h } }

// WithLogger enables verbose request logging.
func WithLogger(w io.Writer, bodies bool) Option {
	return func(c *Client) { c.Logger = w; c.LogBodies = bodies }
}

// WithNoRetry disables automatic retries.
func WithNoRetry(b bool) Option { return func(c *Client) { c.NoRetry = b } }

// NewClient constructs a Client with the given authenticator.
func NewClient(auth Authenticator, opts ...Option) *Client {
	base, _ := url.Parse(DefaultBaseURL)
	c := &Client{
		BaseURL:   base,
		HTTP:      &http.Client{Timeout: 60 * time.Second},
		Auth:      auth,
		UserAgent: "bb/" + build.Version,
		now:       time.Now,
		sleep:     sleepCtx,
	}
	if auth == nil {
		c.Auth = NoAuth{}
	}
	if os.Getenv("BB_NO_RETRY") == "1" {
		c.NoRetry = true
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// Resolve turns a path (absolute API path, relative path, or full URL) into
// an absolute URL under BaseURL. Full URLs are returned as-is so that
// server-provided `next`/`Location` links can be followed.
func (c *Client) Resolve(p string) (*url.URL, error) {
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
		return url.Parse(p)
	}
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimPrefix(p, "2.0/")
	rawQuery := ""
	if i := strings.IndexByte(p, '?'); i >= 0 {
		p, rawQuery = p[:i], p[i+1:]
	}
	unescaped, err := url.PathUnescape(p)
	if err != nil {
		return nil, fmt.Errorf("invalid path %q: %w", p, err)
	}
	u := *c.BaseURL
	base := strings.TrimSuffix(u.Path, "/")
	u.Path = base + "/" + unescaped
	if unescaped != p {
		// preserve caller-provided escapes such as %2F in branch names
		u.RawPath = base + "/" + p
	}
	u.RawQuery = rawQuery
	return &u, nil
}

// sameOrigin reports whether u belongs to the configured API host. Credentials
// are only attached to same-origin requests so that a malicious `next` link
// cannot exfiltrate the token.
func (c *Client) sameOrigin(u *url.URL) bool {
	return strings.EqualFold(u.Scheme, c.BaseURL.Scheme) && strings.EqualFold(u.Host, c.BaseURL.Host)
}

// Request describes a single API call.
type Request struct {
	Method  string
	Path    string
	Query   url.Values
	Body    any               // marshaled as JSON when non-nil (io.Reader is sent raw)
	Headers map[string]string // extra headers
	Accept  string            // overrides Accept (e.g. "text/plain" for diffs)
}

// Do executes req and decodes a JSON response into out (may be nil).
// It returns the response for callers that need headers or status.
func (c *Client) Do(ctx context.Context, req Request, out any) (*http.Response, error) {
	resp, err := c.doRaw(ctx, req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp, nil
	}
	if resp.StatusCode == http.StatusNoContent {
		return resp, nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil && err != io.EOF {
		return resp, fmt.Errorf("decoding response from %s %s: %w", req.Method, req.Path, err)
	}
	return resp, nil
}

// DoRaw executes req and returns the response without decoding. The caller
// must close resp.Body. Non-2xx responses are converted to *HTTPError.
func (c *Client) DoRaw(ctx context.Context, req Request) (*http.Response, error) {
	return c.doRaw(ctx, req)
}

func (c *Client) doRaw(ctx context.Context, req Request) (*http.Response, error) {
	u, err := c.Resolve(req.Path)
	if err != nil {
		return nil, err
	}
	if len(req.Query) > 0 {
		q := u.Query()
		for k, vs := range req.Query {
			for _, v := range vs {
				q.Add(k, v)
			}
		}
		u.RawQuery = q.Encode()
	}
	if !c.sameOrigin(u) {
		return nil, fmt.Errorf("refusing to send request to %s: not the Bitbucket API host", u.Host)
	}

	var payload []byte
	contentType := ""
	switch b := req.Body.(type) {
	case nil:
	case io.Reader:
		payload, err = io.ReadAll(b)
		if err != nil {
			return nil, err
		}
		contentType = "application/json"
	case []byte:
		payload = b
		contentType = "application/json"
	default:
		payload, err = json.Marshal(b)
		if err != nil {
			return nil, fmt.Errorf("encoding request body: %w", err)
		}
		contentType = "application/json"
	}
	if ct, ok := req.Headers["Content-Type"]; ok {
		contentType = ct
	}

	method := req.Method
	if method == "" {
		method = http.MethodGet
	}

	for attempt := 0; ; attempt++ {
		var body io.Reader
		if payload != nil {
			body = bytes.NewReader(payload)
		}
		hr, err := http.NewRequestWithContext(ctx, method, u.String(), body)
		if err != nil {
			return nil, err
		}
		hr.Header.Set("User-Agent", c.UserAgent)
		hr.Header.Set("Accept", "application/json")
		if req.Accept != "" {
			hr.Header.Set("Accept", req.Accept)
		}
		if contentType != "" {
			hr.Header.Set("Content-Type", contentType)
		}
		for k, v := range req.Headers {
			hr.Header.Set(k, v)
		}
		c.Auth.Apply(hr)

		start := c.now()
		c.logRequest(hr)
		resp, err := c.HTTP.Do(hr)
		if err != nil {
			return nil, err
		}
		c.logResponse(resp, c.now().Sub(start))

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return resp, nil
		}
		if !c.NoRetry {
			if delay, ok := retryDelay(resp, attempt, c.now()); ok {
				_, _ = io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if c.Logger != nil {
					fmt.Fprintf(c.Logger, "* retrying in %s (attempt %d/%d)\n", delay, attempt+1, maxRetries)
				}
				if err := c.sleep(ctx, delay); err != nil {
					return nil, err
				}
				continue
			}
		}
		herr := newHTTPError(resp)
		resp.Body.Close()
		return nil, herr
	}
}

// --- logging (secrets masked) ---

func (c *Client) logRequest(r *http.Request) {
	if c.Logger == nil {
		return
	}
	fmt.Fprintf(c.Logger, "> %s %s\n", r.Method, maskURL(r.URL))
	for k, vs := range r.Header {
		for _, v := range vs {
			fmt.Fprintf(c.Logger, "> %s: %s\n", k, maskHeader(k, v))
		}
	}
}

func (c *Client) logResponse(r *http.Response, d time.Duration) {
	if c.Logger == nil {
		return
	}
	fmt.Fprintf(c.Logger, "< %s (%s)\n", r.Status, d.Round(time.Millisecond))
	for k, vs := range r.Header {
		for _, v := range vs {
			fmt.Fprintf(c.Logger, "< %s: %s\n", k, maskHeader(k, v))
		}
	}
	if c.LogBodies && r.Body != nil {
		b, _ := io.ReadAll(r.Body)
		r.Body.Close()
		r.Body = io.NopCloser(bytes.NewReader(b))
		fmt.Fprintf(c.Logger, "< body: %s\n", MaskSecrets(string(b)))
	}
}

var sensitiveHeaders = map[string]bool{
	"authorization":       true,
	"proxy-authorization": true,
	"cookie":              true,
	"set-cookie":          true,
	"x-api-key":           true,
}

func maskHeader(k, v string) string {
	if sensitiveHeaders[strings.ToLower(k)] {
		return "********"
	}
	return v
}

var secretJSONKey = regexp.MustCompile(`(?i)("(?:[a-z_]*token|[a-z_]*secret|[a-z_]*password|(?:api_)?key)"\s*:\s*")(?:[^"\\]|\\.)*(")`)

// MaskSecrets blanks the values of JSON keys that look like credentials
// (access_token, refresh_token, password, secret, ...).
func MaskSecrets(s string) string {
	return secretJSONKey.ReplaceAllString(s, "${1}********${2}")
}

// MaskHeader hides the value of sensitive headers.
func MaskHeader(k, v string) string { return maskHeader(k, v) }

// maskURL hides credentials embedded in the URL and any access_token query.
func maskURL(u *url.URL) string {
	cp := *u
	if cp.User != nil {
		cp.User = url.UserPassword("****", "****")
	}
	q := cp.Query()
	changed := false
	for k := range q {
		if strings.Contains(strings.ToLower(k), "token") {
			q.Set(k, "********")
			changed = true
		}
	}
	if changed {
		cp.RawQuery = q.Encode()
	}
	return cp.String()
}
