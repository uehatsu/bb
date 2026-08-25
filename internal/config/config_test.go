package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestConfigDefaultsAndWrite(t *testing.T) {
	dir := t.TempDir()
	c, err := LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := c.Get("git_protocol"); v != "https" {
		t.Errorf("default git_protocol = %q", v)
	}
	if err := c.Set("git_protocol", "ftp"); err == nil {
		t.Error("expected validation error")
	}
	if err := c.Set("nope", "x"); err == nil {
		t.Error("expected invalid key error")
	}
	if err := c.Set("workspace", "acme"); err != nil {
		t.Fatal(err)
	}
	if err := c.Write(); err != nil {
		t.Fatal(err)
	}
	c2, err := LoadFrom(dir)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := c2.Get("workspace"); v != "acme" {
		t.Errorf("workspace = %q", v)
	}
	assertPerm(t, filepath.Join(dir, "config.yml"), 0o600)
}

func TestFileCredentialStore(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bb")
	s := NewFileCredentialStore(dir)
	if _, err := s.Get(DefaultHost); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	exp := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
	cred := Credential{Method: AuthAPIToken, Email: "me@example.com", Token: "secret", ExpiresAt: &exp}
	if err := s.Set(DefaultHost, cred); err != nil {
		t.Fatal(err)
	}
	assertPerm(t, filepath.Join(dir, "hosts.yml"), 0o600)
	assertPerm(t, dir, 0o700)
	got, err := s.Get(DefaultHost)
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "me@example.com" || got.Token != "secret" || got.Method != AuthAPIToken || !got.ExpiresAt.Equal(exp) {
		t.Errorf("roundtrip mismatch: %+v", got)
	}
	if got.GitUsername() != "x-bitbucket-api-token-auth" {
		t.Errorf("git username = %q", got.GitUsername())
	}
	if err := s.Delete(DefaultHost); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(DefaultHost); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	// no leftover temp files
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("leftover temp file %s", e.Name())
		}
	}
}

func TestBearerGitUsername(t *testing.T) {
	if (Credential{Method: AuthBearer}).GitUsername() != "x-token-auth" {
		t.Error("bearer git username")
	}
}

func TestEnvCredential(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	cases := []struct {
		name   string
		env    map[string]string
		ok     bool
		method AuthMethod
		err    bool
	}{
		{"unset", map[string]string{}, false, "", false},
		{"token+email", map[string]string{"BB_TOKEN": "t", "BB_EMAIL": "e"}, true, AuthAPIToken, false},
		{"token only", map[string]string{"BB_TOKEN": "t"}, true, AuthBearer, false},
		{"explicit bearer", map[string]string{"BB_TOKEN": "t", "BB_EMAIL": "e", "BB_AUTH_METHOD": "bearer"}, true, AuthBearer, false},
		{"api_token no email", map[string]string{"BB_TOKEN": "t", "BB_AUTH_METHOD": "api_token"}, false, "", true},
		{"bad method", map[string]string{"BB_TOKEN": "t", "BB_AUTH_METHOD": "magic"}, false, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, ok, err := EnvCredential(env(tc.env))
			if (err != nil) != tc.err {
				t.Fatalf("err = %v", err)
			}
			if ok != tc.ok {
				t.Fatalf("ok = %v", ok)
			}
			if ok && c.Method != tc.method {
				t.Errorf("method = %q", c.Method)
			}
		})
	}
}

func TestResolveCredentialPrecedence(t *testing.T) {
	dir := t.TempDir()
	s := NewFileCredentialStore(dir)
	_ = s.Set(DefaultHost, Credential{Method: AuthAPIToken, Email: "file@x", Token: "filetok"})
	c, err := ResolveCredential(s, DefaultHost, func(k string) string {
		if k == "BB_TOKEN" {
			return "envtok"
		}
		return ""
	})
	if err != nil {
		t.Fatal(err)
	}
	if c.Token != "envtok" || c.Method != AuthBearer {
		t.Errorf("env should win: %+v", c)
	}
	c, _ = ResolveCredential(s, DefaultHost, func(string) string { return "" })
	if c.Token != "filetok" {
		t.Errorf("file fallback: %+v", c)
	}
}

func TestExpiry(t *testing.T) {
	now := time.Date(2026, 8, 25, 0, 0, 0, 0, time.UTC)
	soon := now.Add(3 * 24 * time.Hour)
	c := Credential{ExpiresAt: &soon}
	if c.IsExpired(now) || !c.ExpiresWithin(now, 7*24*time.Hour) {
		t.Error("expected expiring within 7d")
	}
	past := now.Add(-time.Hour)
	if !(Credential{ExpiresAt: &past}).IsExpired(now) {
		t.Error("expected expired")
	}
}

func TestDirEnv(t *testing.T) {
	t.Setenv("BB_CONFIG_DIR", "/tmp/bbcfg")
	if Dir() != "/tmp/bbcfg" {
		t.Error("BB_CONFIG_DIR not honored")
	}
	t.Setenv("BB_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg")
	if Dir() != filepath.Join("/tmp/xdg", "bb") {
		t.Error("XDG_CONFIG_HOME not honored")
	}
}

func assertPerm(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	if runtime.GOOS == "windows" {
		return
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != want {
		t.Errorf("%s perm = %o, want %o", path, fi.Mode().Perm(), want)
	}
}

func TestLoadUsesConfigDirAndKeys(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "bb")
	t.Setenv("BB_CONFIG_DIR", dir)
	t.Setenv("BB_CREDENTIAL_STORE", "")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Credentials().(*FileCredentialStore); !ok {
		t.Errorf("default store: %T", c.Credentials())
	}
	keys := c.Keys()
	if len(keys) < 5 || keys[0] > keys[1] {
		t.Errorf("Keys should be sorted and complete: %v", keys)
	}
	if (&ErrRefreshFailed{Err: ErrNotFound}).Unwrap() != ErrNotFound {
		t.Error("ErrRefreshFailed.Unwrap")
	}
}
