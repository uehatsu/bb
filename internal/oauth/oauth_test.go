package oauth

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port
}

func tokenServer(t *testing.T, wantGrant string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, p, ok := r.BasicAuth()
		if !ok || u != "cid" || p != "csecret" {
			w.WriteHeader(401)
			w.Write([]byte(`{"error":"invalid_client"}`))
			return
		}
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != wantGrant {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"unsupported_grant_type"}`))
			return
		}
		if wantGrant == "authorization_code" && r.Form.Get("code") != "the-code" {
			w.WriteHeader(400)
			w.Write([]byte(`{"error":"invalid_grant","error_description":"bad code"}`))
			return
		}
		w.Write([]byte(`{"access_token":"AT","refresh_token":"RT","expires_in":7200,"scopes":"repository pullrequest"}`))
	}))
}

func TestAuthorizeHappyPath(t *testing.T) {
	ts := tokenServer(t, "authorization_code")
	defer ts.Close()
	port := freePort(t)
	cfg := Config{ClientID: "cid", ClientSecret: "csecret", TokenURL: ts.URL, Port: port, Timeout: 5 * time.Second}
	var authURL string
	open := func(u string) error {
		authURL = u
		go func() {
			parsed, _ := url.Parse(u)
			state := parsed.Query().Get("state")
			// simulate browser redirect
			http.Get(cfg.CallbackURL() + "?code=the-code&state=" + state) //nolint:errcheck
		}()
		return nil
	}
	tok, err := Authorize(context.Background(), cfg, open)
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "AT" || tok.RefreshToken != "RT" || time.Until(tok.ExpiresAt) < time.Hour {
		t.Errorf("token: %+v", tok)
	}
	if !strings.HasPrefix(authURL, DefaultAuthURL+"?") || !strings.Contains(authURL, "client_id=cid") || !strings.Contains(authURL, "response_type=code") || !strings.Contains(authURL, "state=") {
		t.Errorf("auth url: %s", authURL)
	}
}

func TestAuthorizeIgnoresBadStateAndKeepsWaiting(t *testing.T) {
	ts := tokenServer(t, "authorization_code")
	defer ts.Close()
	port := freePort(t)
	cfg := Config{ClientID: "cid", ClientSecret: "csecret", TokenURL: ts.URL, Port: port, Timeout: 5 * time.Second}
	open := func(u string) error {
		go func() {
			// forged request first: must be answered 400 and ignored
			resp, err := http.Get(cfg.CallbackURL() + "?code=evil&state=wrong")
			if err == nil {
				if resp.StatusCode != 400 {
					t.Errorf("forged callback status %d", resp.StatusCode)
				}
				resp.Body.Close()
			}
			// then the genuine one
			resp, err = http.Get(cfg.CallbackURL() + "?code=the-code&state=" + mustState(u))
			if err == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}
	tok, err := Authorize(context.Background(), cfg, open)
	if err != nil || tok.AccessToken != "AT" {
		t.Fatalf("expected success after ignoring forged callback, got %v %+v", err, tok)
	}
}

func TestAuthorizeDeniedAndTimeout(t *testing.T) {
	port := freePort(t)
	cfg := Config{ClientID: "cid", ClientSecret: "csecret", Port: port, Timeout: 5 * time.Second}
	open := func(u string) error {
		go func() {
			state := mustState(u)
			resp, err := http.Get(cfg.CallbackURL() + "?error=access_denied&error_description=nope&state=" + state)
			if err == nil {
				resp.Body.Close()
			}
		}()
		return nil
	}
	if _, err := Authorize(context.Background(), cfg, open); err == nil || !strings.Contains(err.Error(), "access_denied") {
		t.Errorf("expected denial, got %v", err)
	}
	cfg.Timeout = 50 * time.Millisecond
	if _, err := Authorize(context.Background(), cfg, func(string) error { return nil }); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Errorf("expected timeout, got %v", err)
	}
	if _, err := Authorize(context.Background(), Config{}, nil); err == nil {
		t.Error("expected missing consumer error")
	}
}

func TestRefresh(t *testing.T) {
	ts := tokenServer(t, "refresh_token")
	defer ts.Close()
	cfg := Config{ClientID: "cid", ClientSecret: "csecret", TokenURL: ts.URL}
	tok, err := Refresh(context.Background(), cfg, "RT")
	if err != nil || tok.AccessToken != "AT" {
		t.Fatalf("refresh: %v %+v", err, tok)
	}
	if _, err := Refresh(context.Background(), cfg, ""); err == nil {
		t.Error("expected error without refresh token")
	}
	bad := Config{ClientID: "cid", ClientSecret: "wrong", TokenURL: ts.URL}
	if _, err := Refresh(context.Background(), bad, "RT"); err == nil || !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401, got %v", err)
	}
}

func mustState(u string) string {
	p, _ := url.Parse(u)
	return p.Query().Get("state")
}
