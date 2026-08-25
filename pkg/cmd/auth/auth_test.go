package auth

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/config"
	"github.com/uehatsu/bb/internal/iostreams"
	"github.com/uehatsu/bb/internal/prompt"
)

func testFactory(t *testing.T, handler http.Handler) (*cmdutil.Factory, *config.Config, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	cfg, err := config.LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	io, in, out, errOut := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: io, Config: func() (*config.Config, error) { return cfg, nil }}
	if handler != nil {
		srv := httptest.NewServer(handler)
		t.Cleanup(srv.Close)
		f.APIClient = func() (*api.Client, error) {
			cred, err := config.ResolveCredential(cfg.Credentials(), config.DefaultHost, func(string) string { return "" })
			if err != nil {
				return nil, cmdutil.NewAuthError("")
			}
			return api.NewClient(api.NewAuthenticator(cred), api.WithBaseURL(srv.URL+"/2.0")), nil
		}
		t.Setenv("BB_TEST_SERVER", srv.URL)
	}
	return f, cfg, in, out, errOut
}

func userHandler(t *testing.T, wantAuthPrefix string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/2.0/user", func(w http.ResponseWriter, r *http.Request) {
		if wantAuthPrefix != "" && !strings.HasPrefix(r.Header.Get("Authorization"), wantAuthPrefix) {
			w.WriteHeader(401)
			w.Write([]byte(`{"type":"error","error":{"message":"Access token expired."}}`))
			return
		}
		w.Write([]byte(`{"nickname":"hatsuhito","display_name":"H U","email":"me@example.com","uuid":"{u}"}`))
	})
	mux.HandleFunc("/2.0/user/workspaces", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"values":[{"permission":"owner","workspace":{"slug":"acme"}}]}`))
	})
	return mux
}

func TestLoginWithTokenStdin(t *testing.T) {
	f, cfg, in, _, errOut := testFactory(t, userHandler(t, "Basic "))
	in.WriteString("me@example.com:ATATT-secret\n")
	opts := &LoginOptions{WithToken: true, ExpiresIn: "90d", newClient: func(c config.Credential) *api.Client {
		return api.NewClient(api.NewAuthenticator(c), api.WithBaseURL(getenvTestServer(t)+"/2.0"))
	}}
	if err := runLogin(t.Context(), f, opts); err != nil {
		t.Fatal(err)
	}
	cred, err := cfg.Credentials().Get(config.DefaultHost)
	if err != nil {
		t.Fatal(err)
	}
	if cred.Email != "me@example.com" || cred.Token != "ATATT-secret" || cred.Method != config.AuthAPIToken || cred.User != "hatsuhito" {
		t.Errorf("stored: %+v", cred)
	}
	if cred.ExpiresAt == nil || time.Until(*cred.ExpiresAt) < 89*24*time.Hour {
		t.Errorf("expiry not recorded: %v", cred.ExpiresAt)
	}
	if !strings.Contains(errOut.String(), "Logged in to bitbucket.org as hatsuhito") {
		t.Errorf("output: %s", errOut.String())
	}
}

func TestLoginBearerInteractive(t *testing.T) {
	f, cfg, _, _, _ := testFactory(t, userHandler(t, "Bearer "))
	f.IOStreams.SetStdinTTY(true)
	f.IOStreams.SetStdoutTTY(true)
	f.Prompter = &prompt.Stub{Passwords: []string{"acc-token"}, Inputs: []string{""}}
	opts := &LoginOptions{Bearer: true, newClient: func(c config.Credential) *api.Client {
		return api.NewClient(api.NewAuthenticator(c), api.WithBaseURL(getenvTestServer(t)+"/2.0"))
	}}
	if err := runLogin(t.Context(), f, opts); err != nil {
		t.Fatal(err)
	}
	cred, _ := cfg.Credentials().Get(config.DefaultHost)
	if cred.Method != config.AuthBearer || cred.Token != "acc-token" || cred.GitUsername() != "x-token-auth" {
		t.Errorf("stored: %+v", cred)
	}
}

func TestLoginRejectsBadToken(t *testing.T) {
	f, cfg, in, _, _ := testFactory(t, userHandler(t, "Bearer "))
	in.WriteString("me@example.com:wrong\n")
	opts := &LoginOptions{WithToken: true, newClient: func(c config.Credential) *api.Client {
		return api.NewClient(api.NewAuthenticator(c), api.WithBaseURL(getenvTestServer(t)+"/2.0"))
	}}
	err := runLogin(t.Context(), f, opts)
	if err == nil || !strings.Contains(err.Error(), "verification failed") {
		t.Fatalf("expected verification failure, got %v", err)
	}
	if _, err := cfg.Credentials().Get(config.DefaultHost); err != config.ErrNotFound {
		t.Error("bad token must not be stored")
	}
}

func TestReadTokenFromStdin(t *testing.T) {
	e, tok, _ := readTokenFromStdin(strings.NewReader("a@b.c:tok:with:colons\n"))
	if e != "a@b.c" || tok != "tok:with:colons" {
		t.Errorf("%q %q", e, tok)
	}
	e, tok, _ = readTokenFromStdin(strings.NewReader("justtoken"))
	if e != "" || tok != "justtoken" {
		t.Errorf("%q %q", e, tok)
	}
	if _, _, err := readTokenFromStdin(strings.NewReader("\n")); err == nil {
		t.Error("empty should error")
	}
}

func TestParseLifetime(t *testing.T) {
	for in, want := range map[string]time.Duration{"90d": 90 * 24 * time.Hour, "2w": 14 * 24 * time.Hour, "1y": 365 * 24 * time.Hour, "36h": 36 * time.Hour} {
		if got, err := parseLifetime(in); err != nil || got != want {
			t.Errorf("%s: %v %v", in, got, err)
		}
	}
	if _, err := parseLifetime("soon"); err == nil {
		t.Error("expected error")
	}
}

func TestStatusAndLogout(t *testing.T) {
	f, cfg, _, out, _ := testFactory(t, userHandler(t, "Basic "))
	soon := time.Now().Add(2 * 24 * time.Hour)
	_ = cfg.Credentials().Set(config.DefaultHost, config.Credential{Method: config.AuthAPIToken, Email: "me@example.com", Token: "ATATT1234567890", ExpiresAt: &soon})
	if err := runStatus(f, false); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{"Logged in as hatsuhito", "API token", "x-bitbucket-api-token-auth", "expires soon", "ATAT****7890", "acme(owner)"} {
		if !strings.Contains(s, want) {
			t.Errorf("status missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "ATATT1234567890") {
		t.Error("token must be masked by default")
	}

	logout := NewCmdLogout(f)
	if err := logout.Execute(); err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.Credentials().Get(config.DefaultHost); err != config.ErrNotFound {
		t.Error("logout should delete credential")
	}
	out.Reset()
	if err := runStatus(f, false); err != cmdutil.ErrSilent || !strings.Contains(out.String(), "Not logged in") {
		t.Errorf("status after logout: %v %s", err, out.String())
	}
}

func TestGitCredentialHelper(t *testing.T) {
	cred := config.Credential{Method: config.AuthAPIToken, Email: "e", Token: "tok"}
	resolve := func() (config.Credential, error) { return cred, nil }
	run := func(op, input string) string {
		var out bytes.Buffer
		if err := gitCredential(op, strings.NewReader(input), &out, resolve); err != nil {
			t.Fatal(err)
		}
		return out.String()
	}
	got := run("get", "protocol=https\nhost=bitbucket.org\n\n")
	if !strings.Contains(got, "username=x-bitbucket-api-token-auth\n") || !strings.Contains(got, "password=tok\n") {
		t.Errorf("get: %q", got)
	}
	if run("get", "protocol=https\nhost=bitbucket.org:443\n\n") == "" {
		t.Error("port form should match")
	}
	for _, input := range []string{
		"protocol=https\nhost=github.com\n\n",
		"protocol=http\nhost=bitbucket.org\n\n",
		"protocol=https\nhost=evil.bitbucket.org\n\n",
		"protocol=https\nhost=bitbucket.org\nusername=someoneelse\n\n",
	} {
		if got := run("get", input); got != "" {
			t.Errorf("must stay silent for %q, got %q", input, got)
		}
	}
	if run("store", "protocol=https\nhost=bitbucket.org\n\n") != "" || run("erase", "protocol=https\nhost=bitbucket.org\n\n") != "" {
		t.Error("store/erase must be no-ops")
	}
	cred.Method = config.AuthBearer
	if !strings.Contains(run("get", "protocol=https\nhost=bitbucket.org\n\n"), "username=x-token-auth") {
		t.Error("bearer username")
	}
}

func getenvTestServer(t *testing.T) string {
	t.Helper()
	return mustEnv(t, "BB_TEST_SERVER")
}

func TestGitCredentialRefreshesOAuth(t *testing.T) {
	f, cfg, in, out, _ := testFactory(t, nil)
	past := time.Now().Add(-time.Minute)
	_ = cfg.Credentials().Set(config.DefaultHost, config.Credential{Method: config.AuthOAuth, Token: "stale", RefreshToken: "RT", ClientID: "c", ClientSecret: "s", ExpiresAt: &past})
	orig := config.RefreshOAuth
	defer func() { config.RefreshOAuth = orig }()
	config.RefreshOAuth = func(ctx context.Context, c config.Credential) (config.Credential, error) {
		future := time.Now().Add(2 * time.Hour)
		c.Token, c.ExpiresAt = "fresh", &future
		return c, nil
	}
	in.WriteString("protocol=https\nhost=bitbucket.org\n\n")
	cmd := NewCmdGitCredential(f)
	cmd.SetArgs([]string{"get"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "username=x-token-auth\npassword=fresh\n") {
		t.Errorf("helper must return refreshed token: %q", out.String())
	}
	stored, _ := cfg.Credentials().Get(config.DefaultHost)
	if stored.Token != "fresh" {
		t.Error("refreshed token not persisted")
	}
}
