package config

import (
	"testing"

	"github.com/zalando/go-keyring"
)

func TestKeyringStore(t *testing.T) {
	keyring.MockInit()
	s := NewKeyringCredentialStore()
	if _, err := s.Get(DefaultHost); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	cred := Credential{Method: AuthOAuth, Token: "AT", RefreshToken: "RT", ClientID: "cid", ClientSecret: "sec"}
	if err := s.Set(DefaultHost, cred); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(DefaultHost)
	if err != nil || got.Token != "AT" || got.RefreshToken != "RT" || got.ClientSecret != "sec" || got.Method != AuthOAuth {
		t.Errorf("roundtrip: %+v %v", got, err)
	}
	if got.GitUsername() != "x-token-auth" {
		t.Error("oauth git username")
	}
	if err := s.Delete(DefaultHost); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(DefaultHost); err != nil {
		t.Errorf("second delete should be no-op: %v", err)
	}
	if _, err := s.Get(DefaultHost); err != ErrNotFound {
		t.Error("expected ErrNotFound after delete")
	}
}

func TestSelectStore(t *testing.T) {
	keyring.MockInit()
	t.Setenv("BB_CREDENTIAL_STORE", "")
	if s, err := selectStore(t.TempDir(), ""); err != nil {
		t.Fatal(err)
	} else if _, ok := s.(*FileCredentialStore); !ok {
		t.Errorf("default should be file, got %T", s)
	}
	if s, _ := selectStore(t.TempDir(), "keyring"); s == nil {
		t.Error("keyring by config")
	} else if _, ok := s.(*KeyringCredentialStore); !ok {
		t.Errorf("got %T", s)
	}
	t.Setenv("BB_CREDENTIAL_STORE", "file")
	if s, _ := selectStore(t.TempDir(), "keyring"); s != nil {
		if _, ok := s.(*FileCredentialStore); !ok {
			t.Error("env must override config")
		}
	}
	t.Setenv("BB_CREDENTIAL_STORE", "vault")
	if _, err := selectStore(t.TempDir(), ""); err == nil {
		t.Error("invalid store should error")
	}
	t.Setenv("BB_CREDENTIAL_STORE", "")
	c, err := LoadFrom(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Set("credential_store", "nope"); err == nil {
		t.Error("expected validation error")
	}
}
