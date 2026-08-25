package config

import (
	"errors"
	"testing"

	"github.com/zalando/go-keyring"

	bbconfig "github.com/uehatsu/bb/internal/config"
	"github.com/uehatsu/bb/internal/testutil"
)

func TestConfigCommands(t *testing.T) {
	h := testutil.NewHarness(t)
	c := NewCmdConfig(h.Factory)
	c.SetArgs([]string{"set", "workspace", "acme"})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	c = NewCmdConfig(h.Factory)
	c.SetArgs([]string{"get", "workspace"})
	if err := c.Execute(); err != nil || h.Out.String() != "acme\n" {
		t.Errorf("get: %v %q", err, h.Out.String())
	}
	c = NewCmdConfig(h.Factory)
	c.SetArgs([]string{"set", "git_protocol", "ftp"})
	if err := c.Execute(); err == nil {
		t.Error("expected validation error")
	}
	c = NewCmdConfig(h.Factory)
	c.SetArgs([]string{"get", "nope"})
	if err := c.Execute(); err == nil {
		t.Error("expected unknown key error")
	}
	h.Out.Reset()
	c = NewCmdConfig(h.Factory)
	c.SetArgs([]string{"list"})
	_ = c.Execute()
	if got := h.Out.String(); !contains(got, "git_protocol=https\n") || !contains(got, "workspace=acme\n") || !contains(got, "merge_strategy=merge_commit\n") {
		t.Errorf("list: %q", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}

func TestCredentialStoreMigration(t *testing.T) {
	keyring.MockInit()
	h := testutil.NewHarness(t)
	cred := bbconfig.Credential{Method: bbconfig.AuthAPIToken, Email: "e", Token: "tok"}
	if err := h.Config.Credentials().Set(bbconfig.DefaultHost, cred); err != nil {
		t.Fatal(err)
	}
	c := NewCmdConfig(h.Factory)
	c.SetArgs([]string{"set", "credential_store", "keyring"})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if !contains(h.ErrOut.String(), "Moved credential from file to keyring") {
		t.Errorf("stderr: %s", h.ErrOut.String())
	}
	if got, err := bbconfig.NewKeyringCredentialStore().Get(bbconfig.DefaultHost); err != nil || got.Token != "tok" {
		t.Errorf("keyring should hold the credential: %+v %v", got, err)
	}
	if _, err := h.Config.Credentials().Get(bbconfig.DefaultHost); err != bbconfig.ErrNotFound {
		t.Error("file store should be emptied after migration")
	}
	// same target → no-op; no credential → no-op; invalid → error
	c = NewCmdConfig(h.Factory)
	c.SetArgs([]string{"set", "credential_store", "keyring"})
	if err := c.Execute(); err != nil {
		t.Errorf("same store: %v", err)
	}
	c = NewCmdConfig(h.Factory)
	c.SetArgs([]string{"set", "credential_store", "vault"})
	if err := c.Execute(); err == nil {
		t.Error("invalid store must error")
	}
	// migrating back with nothing stored in the current (file) store is a no-op
	h2 := testutil.NewHarness(t)
	c = NewCmdConfig(h2.Factory)
	c.SetArgs([]string{"set", "credential_store", "keyring"})
	if err := c.Execute(); err != nil || contains(h2.ErrOut.String(), "Moved") {
		t.Errorf("no credential: err=%v stderr=%s", err, h2.ErrOut.String())
	}
}

func TestCredentialStoreMigrationRollsBackOnWriteFailure(t *testing.T) {
	keyring.MockInit()
	h := testutil.NewHarness(t)
	cred := bbconfig.Credential{Method: bbconfig.AuthAPIToken, Email: "e", Token: "tok"}
	_ = h.Config.Credentials().Set(bbconfig.DefaultHost, cred)
	orig := writeConfig
	defer func() { writeConfig = orig }()
	writeConfig = func(*bbconfig.Config) error { return errors.New("disk full") }

	c := NewCmdConfig(h.Factory)
	c.SetArgs([]string{"set", "credential_store", "keyring"})
	if err := c.Execute(); err == nil || !contains(err.Error(), "disk full") {
		t.Fatalf("expected write failure, got %v", err)
	}
	// old store still has the credential; config still says file; keyring copy removed
	if got, err := h.Config.Credentials().Get(bbconfig.DefaultHost); err != nil || got.Token != "tok" {
		t.Errorf("old store must keep the credential after a failed migration: %+v %v", got, err)
	}
	if v, _ := h.Config.Get("credential_store"); v != "file" {
		t.Errorf("config must be rolled back, got %q", v)
	}
	if _, err := bbconfig.NewKeyringCredentialStore().Get(bbconfig.DefaultHost); err != bbconfig.ErrNotFound {
		t.Errorf("target copy must be rolled back, got %v", err)
	}
	// with a working write the migration completes and the config is saved
	writeConfig = orig
	c = NewCmdConfig(h.Factory)
	c.SetArgs([]string{"set", "credential_store", "keyring"})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if v, _ := h.Config.Get("credential_store"); v != "keyring" {
		t.Errorf("config not updated: %q", v)
	}
	if _, err := bbconfig.NewKeyringCredentialStore().Get(bbconfig.DefaultHost); err != nil {
		t.Errorf("credential must now live in the keyring: %v", err)
	}
}
