package config

import (
	"context"
	"fmt"
	"time"

	"github.com/uehatsu/bb/internal/oauth"
)

// RefreshOAuth exchanges an OAuth refresh token for a new access token. It
// is a variable so tests can substitute it.
var RefreshOAuth = func(ctx context.Context, cred Credential) (Credential, error) {
	tok, err := oauth.Refresh(ctx, oauth.Config{ClientID: cred.ClientID, ClientSecret: cred.ClientSecret}, cred.RefreshToken)
	if err != nil {
		return cred, err
	}
	cred.Token = tok.AccessToken
	if tok.RefreshToken != "" {
		cred.RefreshToken = tok.RefreshToken
	}
	cred.ExpiresAt = &tok.ExpiresAt
	return cred, nil
}

// ErrRefreshFailed wraps a failed OAuth refresh.
type ErrRefreshFailed struct{ Err error }

func (e *ErrRefreshFailed) Error() string {
	return fmt.Sprintf("OAuth access token expired and refresh failed: %v (run `bb auth login --web` to re-authenticate)", e.Err)
}

func (e *ErrRefreshFailed) Unwrap() error { return e.Err }

// ResolveFreshCredential resolves the credential like ResolveCredential and,
// for OAuth credentials that are expired or about to expire, refreshes and
// persists them first. Every consumer of a token (API client, git credential
// helper, `bb auth token`) should use this so they never hand out a stale
// access token.
func ResolveFreshCredential(ctx context.Context, store CredentialStore, host string, getenv func(string) string, now time.Time) (Credential, error) {
	cred, err := ResolveCredential(store, host, getenv)
	if err != nil {
		return cred, err
	}
	if !cred.NeedsRefresh(now) {
		return cred, nil
	}
	fresh, err := RefreshOAuth(ctx, cred)
	if err != nil {
		return cred, &ErrRefreshFailed{Err: err}
	}
	if serr := store.Set(host, fresh); serr != nil {
		return fresh, fmt.Errorf("refreshed OAuth token but could not save it: %w", serr)
	}
	return fresh, nil
}
