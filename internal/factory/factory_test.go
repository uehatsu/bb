package factory

import (
	"context"
	"net/http"
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
