package factory

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/config"
	"github.com/uehatsu/bb/internal/iostreams"
	"github.com/uehatsu/bb/internal/testutil"
)

func TestRefreshingAuthRefreshesMidRun(t *testing.T) {
	testutil.IsolateEnv(t)
	cfg, err := config.LoadFrom(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	soon := time.Now().Add(30 * time.Second) // within the 1 minute refresh window
	cred := config.Credential{Method: config.AuthOAuth, Token: "stale", RefreshToken: "RT", ClientID: "c", ClientSecret: "s", ExpiresAt: &soon}
	_ = cfg.Credentials().Set(config.DefaultHost, cred)
	orig := config.RefreshOAuth
	defer func() { config.RefreshOAuth = orig }()
	calls := 0
	config.RefreshOAuth = func(ctx context.Context, c config.Credential) (config.Credential, error) {
		calls++
		future := time.Now().Add(2 * time.Hour)
		c.Token, c.ExpiresAt = "fresh", &future
		return c, nil
	}
	ios, _, _, errOut := iostreams.Test()
	a := &refreshingAuth{cfg: cfg, cred: cred, errOut: ios.ErrOut}
	req, _ := http.NewRequest("GET", "https://api.bitbucket.org/2.0/user", nil)
	a.Apply(req)
	a.Apply(req)
	if req.Header.Get("Authorization") != "Bearer fresh" || calls != 1 {
		t.Errorf("auth=%q calls=%d err=%s", req.Header.Get("Authorization"), calls, errOut.String())
	}
}

func TestRefreshingAuthWarnsOnceAndBacksOff(t *testing.T) {
	testutil.IsolateEnv(t)
	cfg, _ := config.LoadFrom(t.TempDir())
	past := time.Now().Add(-time.Minute)
	cred := config.Credential{Method: config.AuthOAuth, Token: "stale", RefreshToken: "RT", ClientID: "c", ClientSecret: "s", ExpiresAt: &past}
	_ = cfg.Credentials().Set(config.DefaultHost, cred)
	orig := config.RefreshOAuth
	defer func() { config.RefreshOAuth = orig }()
	calls := 0
	config.RefreshOAuth = func(ctx context.Context, c config.Credential) (config.Credential, error) {
		calls++
		return c, errors.New("boom")
	}
	ios, _, _, errOut := iostreams.Test()
	clock := time.Now()
	a := &refreshingAuth{cfg: cfg, cred: cred, errOut: ios.ErrOut, now: func() time.Time { return clock }}
	req, _ := http.NewRequest("GET", "https://api.bitbucket.org/2.0/user", nil)
	for i := 0; i < 5; i++ {
		a.Apply(req)
	}
	if calls != 1 || strings.Count(errOut.String(), "warning:") != 1 {
		t.Errorf("expected 1 refresh attempt and 1 warning within the retry interval, got calls=%d stderr=%q", calls, errOut.String())
	}
	clock = clock.Add(refreshRetryInterval)
	a.Apply(req)
	if calls != 2 {
		t.Errorf("expected a retry after the interval, calls=%d", calls)
	}
}

func TestNewWiresRealDependencies(t *testing.T) {
	testutil.IsolateEnv(t)
	dir := t.TempDir()
	t.Setenv("BB_CONFIG_DIR", dir)
	t.Setenv("BB_PAGER", "cat")

	// no credentials → AuthError from APIClient
	f := New()
	if f.IOStreams == nil || f.Prompter == nil || f.Config == nil || f.GitClient == nil || f.BaseRepo == nil {
		t.Fatal("factory not fully wired")
	}
	cfg, err := f.Config()
	if err != nil {
		t.Fatal(err)
	}
	if cfg2, _ := f.Config(); cfg2 != cfg {
		t.Error("Config must be cached")
	}
	_, err = f.APIClient()
	var authErr *cmdutil.AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError without credentials, got %v", err)
	}

	// with BB_TOKEN → client, cached
	t.Setenv("BB_TOKEN", "tok")
	t.Setenv("BB_DEBUG", "1")
	f = New()
	c1, err := f.APIClient()
	if err != nil || c1 == nil || c1.Logger == nil {
		t.Fatalf("APIClient: %v (logger enabled by BB_DEBUG=1: %v)", err, c1 != nil && c1.Logger != nil)
	}
	if c2, _ := f.APIClient(); c2 != c1 {
		t.Error("APIClient must be cached")
	}
	req, _ := http.NewRequest("GET", "https://api.bitbucket.org/2.0/user", nil)
	c1.Auth.Apply(req)
	if req.Header.Get("Authorization") != "Bearer tok" {
		t.Errorf("BB_TOKEN alone should be bearer: %q", req.Header.Get("Authorization"))
	}

	// expired OAuth credential whose refresh fails → AuthError with hint
	past := time.Now().Add(-time.Minute)
	_ = cfg.Credentials().Set(config.DefaultHost, config.Credential{Method: config.AuthOAuth, Token: "x", RefreshToken: "RT", ClientID: "c", ClientSecret: "s", ExpiresAt: &past})
	t.Setenv("BB_TOKEN", "")
	orig := config.RefreshOAuth
	defer func() { config.RefreshOAuth = orig }()
	config.RefreshOAuth = func(ctx context.Context, c config.Credential) (config.Credential, error) {
		return c, errors.New("boom")
	}
	f = New()
	if _, err := f.APIClient(); !errors.As(err, &authErr) || !strings.Contains(err.Error(), "bb auth login --web") {
		t.Errorf("expected refresh-failure AuthError, got %v", err)
	}
}

func TestApplyPagerPrecedence(t *testing.T) {
	testutil.IsolateEnv(t)
	cfg, _ := config.LoadFrom(t.TempDir())
	newF := func() (*cmdutil.Factory, *iostreams.IOStreams) {
		ios, _, _, _ := iostreams.Test()
		return &cmdutil.Factory{IOStreams: ios, Config: func() (*config.Config, error) { return cfg, nil }}, ios
	}
	pagerOf := func(ios *iostreams.IOStreams) string { return ios.Pager() }

	t.Setenv("PAGER", "less")
	f, ios := newF()
	applyPager(f, ios)
	if pagerOf(ios) != "less" {
		t.Errorf("PAGER fallback: %q", pagerOf(ios))
	}
	_ = cfg.Set("pager", "more")
	f, ios = newF()
	applyPager(f, ios)
	if pagerOf(ios) != "more" {
		t.Errorf("config pager should beat PAGER: %q", pagerOf(ios))
	}
	t.Setenv("BB_PAGER", "bat")
	f, ios = newF()
	applyPager(f, ios)
	if pagerOf(ios) != "bat" {
		t.Errorf("BB_PAGER should win: %q", pagerOf(ios))
	}
}
