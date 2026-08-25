package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/uehatsu/bb/internal/config"
)

func newTestClient(t *testing.T, h http.Handler, opts ...Option) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	cred := config.Credential{Method: config.AuthAPIToken, Email: "me@example.com", Token: "s3cr3tvalue"}
	c := NewClient(NewAuthenticator(cred), append([]Option{WithBaseURL(srv.URL + "/2.0"), WithHTTPClient(srv.Client())}, opts...)...)
	c.sleep = func(context.Context, time.Duration) error { return nil }
	return c, srv
}

func TestBasicAndBearerAuth(t *testing.T) {
	var gotAuth string
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{"ok":true}`))
	}))
	var out map[string]bool
	if _, err := c.Do(context.Background(), Request{Path: "/user"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(gotAuth, "Basic ") || !out["ok"] {
		t.Errorf("basic auth not applied: %q", gotAuth)
	}
	c.Auth = NewAuthenticator(config.Credential{Method: config.AuthBearer, Token: "abc"})
	_, _ = c.Do(context.Background(), Request{Path: "user"}, nil)
	if gotAuth != "Bearer abc" {
		t.Errorf("bearer auth = %q", gotAuth)
	}
}

func TestResolvePaths(t *testing.T) {
	c := NewClient(nil)
	for in, want := range map[string]string{
		"/user":                                  "https://api.bitbucket.org/2.0/user",
		"user":                                   "https://api.bitbucket.org/2.0/user",
		"/2.0/repositories/ws":                   "https://api.bitbucket.org/2.0/repositories/ws",
		"https://api.bitbucket.org/2.0/x?page=2": "https://api.bitbucket.org/2.0/x?page=2",
	} {
		u, err := c.Resolve(in)
		if err != nil || u.String() != want {
			t.Errorf("Resolve(%q) = %v, %v; want %q", in, u, err, want)
		}
	}
}

func TestRefusesForeignHost(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	_, err := c.Do(context.Background(), Request{Path: "https://evil.example.com/steal"}, nil)
	if err == nil || !strings.Contains(err.Error(), "refusing") {
		t.Fatalf("expected refusal, got %v", err)
	}
}

func TestErrorEnvelope(t *testing.T) {
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"type":"error","error":{"message":"Repository not found","detail":"nope"}}`))
	}))
	_, err := c.Do(context.Background(), Request{Path: "/repositories/a/b"}, nil)
	var herr *HTTPError
	if !errors.As(err, &herr) {
		t.Fatalf("expected HTTPError, got %v", err)
	}
	if herr.StatusCode != 404 || herr.Message != "Repository not found" || herr.Detail != "nope" || !herr.IsNotFound() {
		t.Errorf("unexpected: %+v", herr)
	}
}

func TestRetryOn429WithRetryAfter(t *testing.T) {
	var calls int32
	var slept []time.Duration
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Retry-After", "3")
			w.WriteHeader(429)
			return
		}
		w.Write([]byte(`{}`))
	}))
	c.sleep = func(_ context.Context, d time.Duration) error { slept = append(slept, d); return nil }
	if _, err := c.Do(context.Background(), Request{Method: "POST", Path: "/x"}, nil); err != nil {
		t.Fatal(err)
	}
	if calls != 2 || len(slept) != 1 || slept[0] != 3*time.Second {
		t.Errorf("calls=%d slept=%v", calls, slept)
	}
}

func TestNoRetryOnPost5xx(t *testing.T) {
	var calls int32
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(502)
	}))
	_, err := c.Do(context.Background(), Request{Method: "POST", Path: "/merge"}, nil)
	if err == nil || calls != 1 {
		t.Errorf("POST must not be retried on 5xx: calls=%d err=%v", calls, err)
	}
	calls = 0
	_, err = c.Do(context.Background(), Request{Method: "GET", Path: "/x"}, nil)
	if err == nil || calls != maxRetries+1 {
		t.Errorf("GET should retry %d times: calls=%d err=%v", maxRetries, calls, err)
	}
}

func TestNoRetryFlag(t *testing.T) {
	var calls int32
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(429)
	}), WithNoRetry(true))
	_, err := c.Do(context.Background(), Request{Path: "/x"}, nil)
	if err == nil || calls != 1 {
		t.Errorf("expected single call, got %d, err=%v", calls, err)
	}
}

func TestRetryAfterParsing(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	if d, ok := parseRetryAfter("120", now); !ok || d != 120*time.Second {
		t.Errorf("seconds: %v %v", d, ok)
	}
	if d, ok := parseRetryAfter(now.Add(10*time.Second).UTC().Format(http.TimeFormat), now); !ok || d != 10*time.Second {
		t.Errorf("http-date: %v %v", d, ok)
	}
	// past date -> 0, huge -> clamp
	if clampDelay(-5*time.Second) != 0 || clampDelay(time.Hour) != maxRetryAfter {
		t.Error("clamp")
	}
	if _, ok := parseRetryAfter("garbage", now); ok {
		t.Error("garbage should not parse")
	}
}

func TestPaginateFollowsNextAndLimit(t *testing.T) {
	var paths []string
	c, srv := newTestClient(t, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("/2.0/items", func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.RequestURI())
		switch r.URL.Query().Get("page") {
		case "":
			w.Write([]byte(`{"pagelen":2,"next":"` + srv.URL + `/2.0/items?page=ab12&pagelen=2&fields=values.id","values":[{"id":1},{"id":2}]}`))
		case "ab12":
			w.Write([]byte(`{"pagelen":2,"values":[{"id":3},{"id":4}]}`))
		}
	})
	srv.Config.Handler = mux

	type item struct {
		ID int `json:"id"`
	}
	all, err := List[item](context.Background(), c, "/items", ListOptions{Fields: "values.id", Query: `x="1"`})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 4 || all[3].ID != 4 {
		t.Errorf("got %+v", all)
	}
	if len(paths) != 2 || !strings.Contains(paths[0], "fields=values.id") || !strings.Contains(paths[0], "q=x%3D%221%22") || !strings.Contains(paths[0], "pagelen=50") {
		t.Errorf("first request query wrong: %v", paths)
	}
	if strings.Contains(paths[1], "q=") {
		t.Errorf("next link must be followed verbatim: %s", paths[1])
	}

	paths = nil
	limited, err := List[item](context.Background(), c, "/items", ListOptions{Limit: 1})
	if err != nil || len(limited) != 1 || len(paths) != 1 || !strings.Contains(paths[0], "pagelen=1") {
		t.Errorf("limit: %v %v %v", limited, err, paths)
	}
}

func TestBBQL(t *testing.T) {
	if got := BBQLQuote(`fea"ture\x`); got != `"fea\"ture\\x"` {
		t.Errorf("quote = %s", got)
	}
	if got := BBQLAnd(`state="OPEN"`, "", "a OR b"); got != `(state="OPEN") AND (a OR b)` {
		t.Errorf("and = %s", got)
	}
}

func TestPoll(t *testing.T) {
	n := 0
	var slept []time.Duration
	sleep := func(_ context.Context, d time.Duration) error { slept = append(slept, d); return nil }
	err := pollWith(context.Background(), PollOptions{Initial: time.Second, Max: 3 * time.Second, Factor: 2}, sleep, func(ctx context.Context) (bool, error) {
		n++
		if n == 2 {
			return false, &HTTPError{StatusCode: 429}
		}
		return n >= 4, nil
	})
	if err != nil || n != 4 {
		t.Fatalf("err=%v n=%d", err, n)
	}
	if len(slept) != 3 || slept[2] != 3*time.Second {
		t.Errorf("intervals: %v", slept)
	}
	boom := errors.New("boom")
	err = pollWith(context.Background(), PollOptions{}, sleep, func(ctx context.Context) (bool, error) { return false, boom })
	if err != boom {
		t.Errorf("non-429 error should abort: %v", err)
	}
}

func TestPollTimeout(t *testing.T) {
	err := Poll(context.Background(), PollOptions{Initial: time.Millisecond, Max: time.Millisecond, Timeout: 20 * time.Millisecond}, func(ctx context.Context) (bool, error) { return false, nil })
	if !errors.Is(err, ErrPollTimeout) {
		t.Errorf("expected timeout, got %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = Poll(ctx, PollOptions{}, func(ctx context.Context) (bool, error) { return false, nil })
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected canceled, got %v", err)
	}
}

func TestLogMasksSecrets(t *testing.T) {
	var log strings.Builder
	c, _ := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Set-Cookie", "session=abc")
		w.Write([]byte(`{"token":"leak"}`))
	}), WithLogger(&log, false))
	_, _ = c.Do(context.Background(), Request{Path: "/x", Query: map[string][]string{"access_token": {"zzz"}}, Headers: map[string]string{"X-Api-Key": "k"}}, nil)
	s := log.String()
	for _, bad := range []string{"s3cr3tvalue", "zzz", "session=abc", "leak", "bWU"} {
		if strings.Contains(s, bad) {
			t.Errorf("log leaks %q:\n%s", bad, s)
		}
	}
	if !strings.Contains(s, "Authorization: ********") {
		t.Errorf("authorization not masked:\n%s", s)
	}
}

func TestResolvePreservesEscapes(t *testing.T) {
	c := NewClient(nil)
	u, err := c.Resolve("/repositories/ws/r/refs/branches/feat%2Fx")
	if err != nil {
		t.Fatal(err)
	}
	if u.String() != "https://api.bitbucket.org/2.0/repositories/ws/r/refs/branches/feat%2Fx" {
		t.Errorf("got %s", u.String())
	}
	u, _ = c.Resolve("/repositories/ws/r/pipelines/{abc}")
	if u.String() != "https://api.bitbucket.org/2.0/repositories/ws/r/pipelines/%7Babc%7D" {
		t.Errorf("uuid braces: %s", u.String())
	}
}

func TestMaskSecrets(t *testing.T) {
	in := `{"access_token":"abc","refresh_token":"def","scopes":"x","user":{"password":"p"},"api_key":"k1","key":"k\"2","n":1}`
	got := MaskSecrets(in)
	for _, bad := range []string{`"abc"`, `"def"`, `"p"`, `"k1"`, `k\"2`} {
		if strings.Contains(got, bad) {
			t.Errorf("leaked %s in %s", bad, got)
		}
	}
	if !strings.Contains(got, `"scopes":"x"`) {
		t.Errorf("non-secret altered: %s", got)
	}
}
