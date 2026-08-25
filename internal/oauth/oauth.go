// Package oauth implements the Bitbucket Cloud OAuth 2.0 authorization code
// flow for a CLI: a one-shot loopback listener on 127.0.0.1 receives the
// callback, the code is exchanged for tokens, and tokens can be refreshed.
//
// Bitbucket does not support PKCE or the device flow and requires a client
// secret, so users must register their own OAuth consumer (workspace
// settings → OAuth consumers) with callback URL http://127.0.0.1:<port>/callback.
package oauth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Default endpoints.
const (
	DefaultAuthURL  = "https://bitbucket.org/site/oauth2/authorize"
	DefaultTokenURL = "https://bitbucket.org/site/oauth2/access_token"
	DefaultPort     = 8976
	CallbackPath    = "/callback"
)

// Config describes the OAuth consumer.
type Config struct {
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
	Port         int
	HTTP         *http.Client
	// Timeout bounds how long to wait for the browser callback.
	Timeout time.Duration
}

func (c Config) withDefaults() Config {
	if c.AuthURL == "" {
		c.AuthURL = DefaultAuthURL
	}
	if c.TokenURL == "" {
		c.TokenURL = DefaultTokenURL
	}
	if c.Port == 0 {
		c.Port = DefaultPort
	}
	if c.HTTP == nil {
		c.HTTP = &http.Client{Timeout: 30 * time.Second}
	}
	if c.Timeout == 0 {
		c.Timeout = 5 * time.Minute
	}
	return c
}

// Token is the result of an authorization or refresh.
type Token struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	Scopes       string
}

// CallbackURL returns the redirect URI the consumer must be registered with.
func (c Config) CallbackURL() string {
	c = c.withDefaults()
	return fmt.Sprintf("http://127.0.0.1:%d%s", c.Port, CallbackPath)
}

func randomState() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Authorize runs the authorization code flow. openURL is called with the
// authorization URL (typically opening a browser). It returns once the
// callback has been received and exchanged, or when ctx/Timeout expires.
func Authorize(ctx context.Context, cfg Config, openURL func(string) error) (*Token, error) {
	cfg = cfg.withDefaults()
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, errors.New("OAuth consumer key and secret are required")
	}
	state, err := randomState()
	if err != nil {
		return nil, err
	}

	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", cfg.Port))
	if err != nil {
		return nil, fmt.Errorf("cannot listen on 127.0.0.1:%d (is another login in progress?): %w", cfg.Port, err)
	}

	type result struct {
		code string
		err  error
	}
	results := make(chan result, 1)
	mux := http.NewServeMux()
	mux.HandleFunc(CallbackPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(state)) != 1 {
			// Ignore stray or forged requests: keep waiting for the genuine
			// callback instead of letting any local page abort the login.
			http.Error(w, "state mismatch; this request was ignored", http.StatusBadRequest)
			return
		}
		if e := q.Get("error"); e != "" {
			http.Error(w, "authorization failed: "+e, http.StatusBadRequest)
			select {
			case results <- result{err: fmt.Errorf("authorization denied: %s (%s)", e, q.Get("error_description"))}:
			default:
			}
			return
		}
		code := q.Get("code")
		if code == "" {
			http.Error(w, "missing code", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, "<!doctype html><title>bb</title><body style='font-family:sans-serif'><h2>Authentication complete.</h2><p>You may close this window and return to the terminal.</p></body>")
		select {
		case results <- result{code: code}:
		default:
		}
	})
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	authURL := cfg.AuthURL + "?" + url.Values{
		"client_id":     {cfg.ClientID},
		"response_type": {"code"},
		"state":         {state},
	}.Encode()
	if err := openURL(authURL); err != nil {
		return nil, err
	}

	timer := time.NewTimer(cfg.Timeout)
	defer timer.Stop()
	select {
	case r := <-results:
		if r.err != nil {
			return nil, r.err
		}
		return exchange(ctx, cfg, url.Values{"grant_type": {"authorization_code"}, "code": {r.code}})
	case <-timer.C:
		return nil, errors.New("timed out waiting for the browser to complete authorization")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Refresh obtains a new access token using a refresh token.
func Refresh(ctx context.Context, cfg Config, refreshToken string) (*Token, error) {
	cfg = cfg.withDefaults()
	if refreshToken == "" {
		return nil, errors.New("no refresh token available; run `bb auth login --web` again")
	}
	return exchange(ctx, cfg, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}})
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Scopes       string `json:"scopes"`
	TokenType    string `json:"token_type"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

func exchange(ctx context.Context, cfg Config, form url.Values) (*Token, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(cfg.ClientID, cfg.ClientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := cfg.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	var tr tokenResponse
	_ = json.Unmarshal(body, &tr)
	if resp.StatusCode != http.StatusOK || tr.AccessToken == "" {
		msg := tr.ErrorDesc
		if msg == "" {
			msg = tr.Error
		}
		if msg == "" {
			msg = strings.TrimSpace(string(body))
		}
		return nil, fmt.Errorf("token request failed (HTTP %d): %s", resp.StatusCode, msg)
	}
	t := &Token{AccessToken: tr.AccessToken, RefreshToken: tr.RefreshToken, Scopes: tr.Scopes}
	if tr.ExpiresIn > 0 {
		t.ExpiresAt = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	} else {
		t.ExpiresAt = time.Now().Add(2 * time.Hour) // Bitbucket default lifetime
	}
	return t, nil
}
