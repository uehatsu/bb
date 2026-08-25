package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/zalando/go-keyring"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/config"
	"github.com/uehatsu/bb/internal/git"
	"github.com/uehatsu/bb/internal/oauth"
	"github.com/uehatsu/bb/internal/prompt"
)

func TestLoginCommandFlags(t *testing.T) {
	f, _, _, _, _ := testFactory(t, userHandler(t, "Basic "))
	cmd := NewCmdLogin(f)
	cmd.SetArgs([]string{}) // non-interactive, no --with-token
	var flagErr *cmdutil.FlagError
	if err := cmd.Execute(); !errors.As(err, &flagErr) {
		t.Errorf("expected FlagError, got %v", err)
	}
	cmd = NewCmdLogin(f)
	cmd.SetArgs([]string{"--web"}) // non-interactive, no consumer
	if err := cmd.Execute(); err == nil {
		t.Error("expected consumer error")
	}
	if !strings.Contains(cmd.Long, "read:user:bitbucket") {
		t.Error("help must list scopes")
	}
}

func TestLoginInteractiveAPIToken(t *testing.T) {
	f, cfg, _, _, errOut := testFactory(t, userHandler(t, "Basic "))
	f.IOStreams.SetStdinTTY(true)
	f.IOStreams.SetStdoutTTY(true)
	f.Prompter = &prompt.Stub{Inputs: []string{"me@example.com", "30d"}, Passwords: []string{"ATATT-x"}}
	opts := &LoginOptions{newClient: func(c config.Credential) *api.Client {
		return api.NewClient(api.NewAuthenticator(c), api.WithBaseURL(getenvTestServer(t)+"/2.0"))
	}}
	if err := runLogin(t.Context(), f, opts); err != nil {
		t.Fatal(err)
	}
	cred, _ := cfg.Credentials().Get(config.DefaultHost)
	if cred.Email != "me@example.com" || cred.Token != "ATATT-x" || cred.ExpiresAt == nil {
		t.Errorf("stored: %+v", cred)
	}
	if !strings.Contains(errOut.String(), "Token expiry recorded") {
		t.Errorf("stderr: %s", errOut.String())
	}

	// prompt cancelled → ErrCancel
	f.Prompter = &prompt.Stub{}
	if err := runLogin(t.Context(), f, opts); !errors.Is(err, cmdutil.ErrCancel) {
		t.Errorf("expected ErrCancel, got %v", err)
	}
	// empty token
	f.Prompter = &prompt.Stub{Inputs: []string{"me@example.com", ""}, Passwords: []string{"   "}}
	if err := runLogin(t.Context(), f, opts); err == nil || !strings.Contains(err.Error(), "empty") {
		t.Errorf("expected empty token error, got %v", err)
	}
	// api token without email (with-token, no --email)
	f2, _, in, _, _ := testFactory(t, userHandler(t, "Basic "))
	in.WriteString("justtoken\n")
	if err := runLogin(t.Context(), f2, &LoginOptions{WithToken: true, newClient: opts.newClient}); err == nil || !strings.Contains(err.Error(), "email") {
		t.Errorf("expected email required error, got %v", err)
	}
	// bad lifetime
	f3, _, in3, _, _ := testFactory(t, userHandler(t, "Basic "))
	in3.WriteString("a@b.c:tok\n")
	if err := runLogin(t.Context(), f3, &LoginOptions{WithToken: true, ExpiresIn: "soon", newClient: opts.newClient}); err == nil {
		t.Error("expected lifetime error")
	}
	// network failure (not 401) wraps generically
	f4, _, in4, _, _ := testFactory(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(500) }))
	in4.WriteString("a@b.c:tok\n")
	if err := runLogin(t.Context(), f4, &LoginOptions{WithToken: true, newClient: func(c config.Credential) *api.Client {
		return api.NewClient(api.NewAuthenticator(c), api.WithBaseURL(getenvTestServer(t)+"/2.0"), api.WithNoRetry(true))
	}}); err == nil || !strings.Contains(err.Error(), "verification failed") {
		t.Errorf("expected verification failure, got %v", err)
	}
}

func TestLoginWebInteractiveAndReuse(t *testing.T) {
	f, cfg, _, _, _ := testFactory(t, userHandler(t, "Bearer "))
	f.IOStreams.SetStdinTTY(true)
	f.IOStreams.SetStdoutTTY(true)
	t.Setenv("BROWSER", "true")
	exp := time.Now().Add(2 * time.Hour)
	authorize := func(ctx context.Context, c oauth.Config, open func(string) error) (*oauth.Token, error) {
		_ = open("https://bitbucket.org/site/oauth2/authorize?x")
		return &oauth.Token{AccessToken: "AT", RefreshToken: "RT", ExpiresAt: exp}, nil
	}
	newClient := func(c config.Credential) *api.Client {
		return api.NewClient(api.NewAuthenticator(c), api.WithBaseURL(getenvTestServer(t)+"/2.0"))
	}
	// prompted consumer key + secret
	f.Prompter = &prompt.Stub{Inputs: []string{"cid"}, Passwords: []string{"sec"}}
	if err := runLoginWeb(t.Context(), f, &LoginOptions{Web: true, authorize: authorize, newClient: newClient}); err != nil {
		t.Fatal(err)
	}
	cred, _ := cfg.Credentials().Get(config.DefaultHost)
	if cred.ClientID != "cid" || cred.ClientSecret != "sec" {
		t.Errorf("consumer not stored: %+v", cred)
	}
	// second login reuses the stored consumer without prompting
	f.Prompter = &prompt.Stub{}
	if err := runLoginWeb(t.Context(), f, &LoginOptions{Web: true, authorize: authorize, newClient: newClient}); err != nil {
		t.Errorf("reuse of stored consumer failed: %v", err)
	}
	// oauth_port config: valid and invalid
	_ = cfg.Set("oauth_port", "9000")
	var gotPort int
	if err := runLoginWeb(t.Context(), f, &LoginOptions{Web: true, newClient: newClient, authorize: func(ctx context.Context, c oauth.Config, open func(string) error) (*oauth.Token, error) {
		gotPort = c.Port
		return &oauth.Token{AccessToken: "AT", ExpiresAt: exp}, nil
	}}); err != nil || gotPort != 9000 {
		t.Errorf("oauth_port: err=%v port=%d", err, gotPort)
	}
	_ = cfg.Set("oauth_port", "abc")
	if err := runLoginWeb(t.Context(), f, &LoginOptions{Web: true, authorize: authorize, newClient: newClient}); err == nil || !strings.Contains(err.Error(), "oauth_port") {
		t.Errorf("expected invalid oauth_port error, got %v", err)
	}
	_ = cfg.Set("oauth_port", "")
	// authorize failure and verification failure
	if err := runLoginWeb(t.Context(), f, &LoginOptions{Web: true, newClient: newClient, authorize: func(context.Context, oauth.Config, func(string) error) (*oauth.Token, error) {
		return nil, errors.New("denied")
	}}); err == nil || !strings.Contains(err.Error(), "denied") {
		t.Errorf("authorize error: %v", err)
	}
	f5, _, _, _, _ := testFactory(t, userHandler(t, "Basic ")) // server rejects Bearer
	t.Setenv("BB_OAUTH_CLIENT_ID", "cid")
	t.Setenv("BB_OAUTH_CLIENT_SECRET", "sec")
	if err := runLoginWeb(t.Context(), f5, &LoginOptions{Web: true, authorize: authorize, newClient: func(c config.Credential) *api.Client {
		return api.NewClient(api.NewAuthenticator(c), api.WithBaseURL(getenvTestServer(t)+"/2.0"), api.WithNoRetry(true))
	}}); err == nil || !strings.Contains(err.Error(), "account") {
		t.Errorf("expected scope hint on verification failure, got %v", err)
	}
	// cancelled prompts
	f6, _, _, _, _ := testFactory(t, nil)
	f6.IOStreams.SetStdinTTY(true)
	f6.IOStreams.SetStdoutTTY(true)
	t.Setenv("BB_OAUTH_CLIENT_ID", "")
	t.Setenv("BB_OAUTH_CLIENT_SECRET", "")
	f6.Prompter = &prompt.Stub{}
	if err := runLoginWeb(t.Context(), f6, &LoginOptions{Web: true}); !errors.Is(err, cmdutil.ErrCancel) {
		t.Errorf("expected ErrCancel, got %v", err)
	}
}

func TestLogoutNotLoggedIn(t *testing.T) {
	f, _, _, _, _ := testFactory(t, nil)
	if err := NewCmdLogout(f).Execute(); err == nil || !strings.Contains(err.Error(), "not logged in") {
		t.Errorf("expected not logged in, got %v", err)
	}
}

func TestRefreshCommand(t *testing.T) {
	f, cfg, _, _, errOut := testFactory(t, nil)
	if err := NewCmdRefresh(f).Execute(); err == nil {
		t.Error("expected auth error when not logged in")
	}
	past := time.Now().Add(-time.Minute)
	_ = cfg.Credentials().Set(config.DefaultHost, config.Credential{Method: config.AuthOAuth, Token: "old", RefreshToken: "RT", ClientID: "c", ClientSecret: "s", ExpiresAt: &past})
	orig := config.RefreshOAuth
	defer func() { config.RefreshOAuth = orig }()
	config.RefreshOAuth = func(ctx context.Context, c config.Credential) (config.Credential, error) {
		fut := time.Now().Add(2 * time.Hour)
		c.Token, c.ExpiresAt = "new", &fut
		return c, nil
	}
	if err := NewCmdRefresh(f).Execute(); err != nil {
		t.Fatal(err)
	}
	if cred, _ := cfg.Credentials().Get(config.DefaultHost); cred.Token != "new" {
		t.Errorf("not persisted: %+v", cred)
	}
	if !strings.Contains(errOut.String(), "Refreshed OAuth token") {
		t.Errorf("stderr: %s", errOut.String())
	}
	config.RefreshOAuth = func(ctx context.Context, c config.Credential) (config.Credential, error) {
		return c, errors.New("boom")
	}
	if err := NewCmdRefresh(f).Execute(); err == nil || !strings.Contains(err.Error(), "bb auth login --web") {
		t.Errorf("expected hint, got %v", err)
	}
	if _, err := RefreshCredential(t.Context(), config.Credential{}); err == nil {
		t.Error("RefreshCredential should propagate failure")
	}
}

func TestSetupGit(t *testing.T) {
	f, _, _, _, errOut := testFactory(t, nil)
	f.Executable = "/opt/homebrew/bin/bb"
	stub := git.NewStub()
	f.GitClient = func() (git.Runner, error) { return stub, nil }
	if err := NewCmdSetupGit(f).Execute(); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(stub.Calls, "\n")
	for _, want := range []string{
		"git config --global --get credential.https://bitbucket.org.helper",
		"git config --global --unset-all credential.https://bitbucket.org.helper",
		"git config --global --add credential.https://bitbucket.org.helper",
		"!'/opt/homebrew/bin/bb' auth git-credential",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
	// already configured → warning, no changes unless --force
	stub.Calls = nil
	stub.Outputs["config --global --get credential.https://bitbucket.org.helper"] = "!bb auth git-credential"
	if err := NewCmdSetupGit(f).Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errOut.String(), "already configured") || len(stub.Calls) != 1 {
		t.Errorf("expected warning only: calls=%v stderr=%s", stub.Calls, errOut.String())
	}
	stub.Calls = nil
	cmd := NewCmdSetupGit(f)
	cmd.SetArgs([]string{"--force"})
	if err := cmd.Execute(); err != nil || len(stub.Calls) < 3 {
		t.Errorf("--force should rewrite: %v %v", err, stub.Calls)
	}
	// git errors surface
	stub.Errors["config --global --get credential.https://bitbucket.org.helper"] = errors.New("no git")
	if err := NewCmdSetupGit(f).Execute(); err == nil {
		t.Error("expected git error")
	}
	f.GitClient = func() (git.Runner, error) { return nil, errors.New("git missing") }
	if err := NewCmdSetupGit(f).Execute(); err == nil {
		t.Error("expected missing git error")
	}
}

func TestStatusVariants(t *testing.T) {
	// invalid token + expired record
	f, cfg, _, out, _ := testFactory(t, userHandler(t, "Bearer "))
	past := time.Now().Add(-24 * time.Hour)
	_ = cfg.Credentials().Set(config.DefaultHost, config.Credential{Method: config.AuthAPIToken, Email: "e", Token: "short", ExpiresAt: &past})
	if err := runStatus(t.Context(), f, false); !errors.Is(err, cmdutil.ErrSilent) {
		t.Errorf("expected ErrSilent, got %v", err)
	}
	if !strings.Contains(out.String(), "is invalid") || !strings.Contains(out.String(), "recorded expiry") {
		t.Errorf("out: %s", out.String())
	}

	// BB_TOKEN source, bearer, show-token, expiry not recorded
	f2, _, _, out2, _ := testFactory(t, userHandler(t, "Bearer "))
	t.Setenv("BB_TOKEN", "envtoken123456")
	if err := runStatus(t.Context(), f2, true); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Credential source: BB_TOKEN", "Access token (Bearer)", "x-token-auth", "Token: envtoken123456", "not recorded"} {
		if !strings.Contains(out2.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out2.String())
		}
	}
	t.Setenv("BB_TOKEN", "")

	// keyring source + OAuth method + expired display
	keyring.MockInit()
	f3, _, _, out3, _ := testFactory(t, userHandler(t, "Bearer "))
	t.Setenv("BB_CREDENTIAL_STORE", "keyring")
	cfg3, err := config.LoadFrom(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f3.Config = func() (*config.Config, error) { return cfg3, nil }
	_ = cfg3.Credentials().Set(config.DefaultHost, config.Credential{Method: config.AuthOAuth, Token: "AT", RefreshToken: "RT", ClientID: "c", ClientSecret: "s", ExpiresAt: &past})
	orig := config.RefreshOAuth
	defer func() { config.RefreshOAuth = orig }()
	config.RefreshOAuth = func(ctx context.Context, c config.Credential) (config.Credential, error) {
		fut := time.Now().Add(2 * time.Hour)
		c.Token, c.ExpiresAt = "AT2", &fut
		return c, nil
	}
	if err := runStatus(t.Context(), f3, false); err != nil {
		t.Fatalf("%v\n%s", err, out3.String())
	}
	for _, want := range []string{"OAuth 2.0", "Credential source: keyring"} {
		if !strings.Contains(out3.String(), want) {
			t.Errorf("missing %q in:\n%s", want, out3.String())
		}
	}
	if maskToken("short") != "********" || maskToken("ATATT1234567890") != "ATAT****7890" {
		t.Error("maskToken")
	}
}

func TestTokenCommand(t *testing.T) {
	f, cfg, _, out, errOut := testFactory(t, nil)
	if err := NewCmdToken(f).Execute(); err == nil {
		t.Error("expected auth error")
	}
	_ = cfg.Credentials().Set(config.DefaultHost, config.Credential{Method: config.AuthAPIToken, Email: "e", Token: "sekret"})
	f.IOStreams.SetStdoutTTY(true)
	if err := NewCmdToken(f).Execute(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "sekret\n" || !strings.Contains(errOut.String(), "secret to the terminal") {
		t.Errorf("out=%q err=%q", out.String(), errOut.String())
	}
	// refresh failure surfaces
	past := time.Now().Add(-time.Minute)
	_ = cfg.Credentials().Set(config.DefaultHost, config.Credential{Method: config.AuthOAuth, Token: "AT", RefreshToken: "RT", ClientID: "c", ClientSecret: "s", ExpiresAt: &past})
	orig := config.RefreshOAuth
	defer func() { config.RefreshOAuth = orig }()
	config.RefreshOAuth = func(ctx context.Context, c config.Credential) (config.Credential, error) {
		return c, errors.New("boom")
	}
	if err := NewCmdToken(f).Execute(); err == nil {
		t.Error("expected refresh failure")
	}
}
