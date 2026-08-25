package config

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestResolveFreshCredentialRefreshesAndPersists(t *testing.T) {
	dir := t.TempDir()
	s := NewFileCredentialStore(dir)
	past := time.Now().Add(-time.Minute)
	_ = s.Set(DefaultHost, Credential{Method: AuthOAuth, Token: "old", RefreshToken: "RT", ClientID: "c", ClientSecret: "s", ExpiresAt: &past})
	orig := RefreshOAuth
	defer func() { RefreshOAuth = orig }()
	calls := 0
	RefreshOAuth = func(ctx context.Context, c Credential) (Credential, error) {
		calls++
		future := time.Now().Add(2 * time.Hour)
		c.Token, c.RefreshToken, c.ExpiresAt = "new", "RT2", &future
		return c, nil
	}
	env := func(string) string { return "" }
	got, err := ResolveFreshCredential(context.Background(), s, DefaultHost, env, time.Now())
	if err != nil || got.Token != "new" || calls != 1 {
		t.Fatalf("refresh: %v %+v calls=%d", err, got, calls)
	}
	stored, _ := s.Get(DefaultHost)
	if stored.Token != "new" || stored.RefreshToken != "RT2" {
		t.Errorf("not persisted: %+v", stored)
	}
	// second call: still fresh, no refresh
	if _, err := ResolveFreshCredential(context.Background(), s, DefaultHost, env, time.Now()); err != nil || calls != 1 {
		t.Errorf("unexpected second refresh: %v calls=%d", err, calls)
	}
	// failure surfaces as ErrRefreshFailed
	_ = s.Set(DefaultHost, Credential{Method: AuthOAuth, Token: "old", RefreshToken: "RT", ClientID: "c", ClientSecret: "s", ExpiresAt: &past})
	RefreshOAuth = func(ctx context.Context, c Credential) (Credential, error) { return c, errors.New("boom") }
	var rf *ErrRefreshFailed
	if _, err := ResolveFreshCredential(context.Background(), s, DefaultHost, env, time.Now()); !errors.As(err, &rf) {
		t.Errorf("expected ErrRefreshFailed, got %v", err)
	}
	// api tokens are untouched
	_ = s.Set(DefaultHost, Credential{Method: AuthAPIToken, Email: "e", Token: "t", ExpiresAt: &past})
	if c, err := ResolveFreshCredential(context.Background(), s, DefaultHost, env, time.Now()); err != nil || c.Token != "t" {
		t.Errorf("api token: %v %+v", err, c)
	}
}
