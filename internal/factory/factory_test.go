package factory

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

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
