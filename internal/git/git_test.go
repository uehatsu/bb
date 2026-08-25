package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// newRepo initializes a real git repository in a temp dir. Tests skip when
// git is not installed.
func newRepo(t *testing.T) *Client {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Isolate from the developer's global config.
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(dir, "gitconfig-global"))
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	t.Setenv("HOME", dir)
	if _, err := c.Output(context.Background(), "init", "-q", "-b", "main"); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestRemotesAndCurrentBranch(t *testing.T) {
	c := newRepo(t)
	ctx := context.Background()
	if _, err := c.Output(ctx, "remote", "add", "origin", "git@ssh.bitbucket.org:acme/widgets.git"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Output(ctx, "remote", "add", "upstream", "https://bitbucket.org/org/widgets.git"); err != nil {
		t.Fatal(err)
	}
	remotes, err := c.Remotes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(remotes) != 2 {
		t.Fatalf("remotes: %+v", remotes)
	}
	seen := map[string]string{}
	for _, r := range remotes {
		seen[r.Name] = r.URL
	}
	if seen["origin"] != "git@ssh.bitbucket.org:acme/widgets.git" || seen["upstream"] != "https://bitbucket.org/org/widgets.git" {
		t.Errorf("remotes: %v", seen)
	}
	branch, err := c.CurrentBranch(ctx)
	if err != nil || branch != "main" {
		t.Errorf("branch=%q err=%v", branch, err)
	}
}

func TestConfigRoundTrip(t *testing.T) {
	c := newRepo(t)
	ctx := context.Background()
	const key = "bb.test.helper"
	if v, err := c.ConfigGet(ctx, "", key); err != nil || v != "" {
		t.Fatalf("unset key: %q %v (exit 1 must map to empty)", v, err)
	}
	if err := c.ConfigUnsetAll(ctx, "", key); err != nil {
		t.Fatalf("unset-all on absent key must be nil (exit 5): %v", err)
	}
	if err := c.ConfigAdd(ctx, "", key, ""); err != nil {
		t.Fatal(err)
	}
	if err := c.ConfigAdd(ctx, "", key, "!bb auth git-credential"); err != nil {
		t.Fatal(err)
	}
	// Output trims whitespace, so the leading empty entry is not visible here;
	// use -z to count entries reliably.
	out, err := c.Output(ctx, "config", "--get-all", "-z", key)
	if err != nil || strings.Count(out, "\x00")+1 < 2 || !strings.Contains(out, "!bb auth git-credential") {
		t.Errorf("get-all: %q %v", out, err)
	}
	if err := c.ConfigSet(ctx, "", key, "single"); err != nil {
		t.Fatal(err)
	}
	if v, _ := c.ConfigGet(ctx, "", key); v != "single" {
		t.Errorf("after set: %q", v)
	}
	if err := c.ConfigUnsetAll(ctx, "", key); err != nil {
		t.Fatal(err)
	}
	if v, _ := c.ConfigGet(ctx, "", key); v != "" {
		t.Errorf("after unset: %q", v)
	}
}

func TestOutputErrorAndInDir(t *testing.T) {
	c := newRepo(t)
	ctx := context.Background()
	_, err := c.Output(ctx, "rev-parse", "--verify", "--quiet", "refs/heads/nope")
	var gerr *Error
	if err == nil || !asError(err, &gerr) {
		t.Fatalf("expected *Error, got %v", err)
	}
	sub := c.InDir(t.TempDir())
	if _, err := sub.Output(ctx, "rev-parse", "--is-inside-work-tree"); err == nil {
		t.Error("InDir must run in the other directory (not a repo)")
	}
}

func asError(err error, target **Error) bool {
	e, ok := err.(*Error)
	if ok {
		*target = e
	}
	return ok
}

func TestStubRecordsEverything(t *testing.T) {
	s := NewStub()
	ctx := context.Background()
	if _, err := s.CurrentBranch(ctx); err == nil {
		t.Error("empty Branch should error")
	}
	s.Branch = "main"
	if b, _ := s.CurrentBranch(ctx); b != "main" {
		t.Error("Branch")
	}
	_ = s.ConfigSet(ctx, "--global", "k", "v")
	_ = s.ConfigAdd(ctx, "", "k", "v2")
	_ = s.ConfigUnsetAll(ctx, "--global", "k")
	_ = s.Run(ctx, "status")
	want := "git config --global --replace-all k v\ngit config --add k v2\ngit config --global --unset-all k\ngit status"
	if got := strings.Join(s.Calls, "\n"); got != want {
		t.Errorf("calls:\n%s\nwant:\n%s", got, want)
	}
	if (&Error{Args: []string{"x"}, Err: context.Canceled}).Unwrap() != context.Canceled {
		t.Error("Unwrap")
	}
}

func TestRunStreamsToTerminal(t *testing.T) {
	c := newRepo(t)
	devnull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	defer devnull.Close()
	c.Stdout, c.Stderr = devnull, devnull
	if err := c.Run(context.Background(), "status"); err != nil {
		t.Errorf("Run: %v", err)
	}
	if err := c.Run(context.Background(), "definitely-not-a-git-command"); err == nil {
		t.Error("Run must surface failures")
	}
}
