package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/config"
	"github.com/uehatsu/bb/internal/oauth"
)

func TestLoginWebStoresOAuthCredential(t *testing.T) {
	f, cfg, _, _, errOut := testFactory(t, userHandler(t, "Bearer "))
	t.Setenv("BB_OAUTH_CLIENT_SECRET", "sec")
	exp := time.Now().Add(2 * time.Hour)
	var gotCfg oauth.Config
	opts := &LoginOptions{Web: true, ClientID: "cid", Port: 9999,
		authorize: func(ctx context.Context, c oauth.Config, open func(string) error) (*oauth.Token, error) {
			gotCfg = c
			return &oauth.Token{AccessToken: "AT", RefreshToken: "RT", ExpiresAt: exp, Scopes: "account repository"}, nil
		},
		newClient: func(c config.Credential) *api.Client {
			return api.NewClient(api.NewAuthenticator(c), api.WithBaseURL(getenvTestServer(t)+"/2.0"))
		},
	}
	if err := runLoginWeb(t.Context(), f, opts); err != nil {
		t.Fatal(err)
	}
	if gotCfg.ClientID != "cid" || gotCfg.ClientSecret != "sec" || gotCfg.CallbackURL() != "http://127.0.0.1:9999/callback" {
		t.Errorf("oauth config: %+v", gotCfg)
	}
	cred, err := cfg.Credentials().Get(config.DefaultHost)
	if err != nil {
		t.Fatal(err)
	}
	if cred.Method != config.AuthOAuth || cred.Token != "AT" || cred.RefreshToken != "RT" || cred.ClientSecret != "sec" || cred.User != "hatsuhito" || cred.ExpiresAt == nil {
		t.Errorf("stored: %+v", cred)
	}
	if !strings.Contains(errOut.String(), "via OAuth") || !strings.Contains(errOut.String(), "127.0.0.1:9999/callback") {
		t.Errorf("stderr: %s", errOut.String())
	}
	// non-interactive without consumer
	t.Setenv("BB_OAUTH_CLIENT_SECRET", "")
	_ = cfg.Credentials().Delete(config.DefaultHost)
	if err := runLoginWeb(t.Context(), f, &LoginOptions{Web: true}); err == nil {
		t.Error("expected error without consumer")
	}
}

func TestRefreshCommandAndNeedsRefresh(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "refresh_token" || r.Form.Get("refresh_token") != "RT" {
			w.WriteHeader(400)
			return
		}
		w.Write([]byte(`{"access_token":"AT2","refresh_token":"RT2","expires_in":7200}`))
	}))
	defer ts.Close()
	past := time.Now().Add(-time.Minute)
	cred := config.Credential{Method: config.AuthOAuth, Token: "AT", RefreshToken: "RT", ClientID: "cid", ClientSecret: "sec", ExpiresAt: &past}
	if !cred.NeedsRefresh(time.Now()) {
		t.Error("expired oauth token should need refresh")
	}
	if (config.Credential{Method: config.AuthAPIToken, ExpiresAt: &past}).NeedsRefresh(time.Now()) {
		t.Error("api tokens are never refreshed")
	}
	tok, err := oauth.Refresh(context.Background(), oauth.Config{ClientID: "cid", ClientSecret: "sec", TokenURL: ts.URL}, cred.RefreshToken)
	if err != nil || tok.AccessToken != "AT2" || tok.RefreshToken != "RT2" {
		t.Fatalf("refresh: %v %+v", err, tok)
	}

	f, cfg, _, _, _ := testFactory(t, nil)
	_ = cfg.Credentials().Set(config.DefaultHost, config.Credential{Method: config.AuthAPIToken, Email: "e", Token: "t"})
	if err := NewCmdRefresh(f).Execute(); err == nil || !strings.Contains(err.Error(), "only OAuth") {
		t.Errorf("expected only-OAuth error, got %v", err)
	}
}
