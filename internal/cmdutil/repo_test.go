package cmdutil

import (
	"context"
	"os/exec"
	"testing"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/config"
	"github.com/uehatsu/bb/internal/git"
	"github.com/uehatsu/bb/internal/iostreams"
)

func realGitRepo(t *testing.T, remotes map[string]string) git.Runner {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	g, err := git.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := g.Output(ctx, "init", "-q"); err != nil {
		t.Fatal(err)
	}
	for name, u := range remotes {
		if _, err := g.Output(ctx, "remote", "add", name, u); err != nil {
			t.Fatal(err)
		}
	}
	return g
}

func newFactory(t *testing.T, g git.Runner) *Factory {
	t.Helper()
	ios, _, _, _ := iostreams.Test()
	cfg, _ := config.LoadFrom(t.TempDir())
	return &Factory{
		IOStreams: ios,
		Config:    func() (*config.Config, error) { return cfg, nil },
		GitClient: func() (git.Runner, error) { return g, nil },
	}
}

func TestRepoFromRemotes(t *testing.T) {
	g := realGitRepo(t, map[string]string{
		"fork":     "git@bitbucket.org:me/widgets.git",
		"origin":   "https://bitbucket.org/acme/widgets.git",
		"github":   "git@github.com:acme/widgets.git",
		"upstream": "ssh://git@ssh.bitbucket.org/org/widgets.git",
	})
	f := newFactory(t, g)
	repo, err := RepoFromRemotes(f)
	if err != nil || repo.FullName() != "org/widgets" {
		t.Errorf("upstream should win: %+v %v", repo, err)
	}
	only := realGitRepo(t, map[string]string{"github": "git@github.com:acme/widgets.git"})
	if _, err := RepoFromRemotes(newFactory(t, only)); err == nil {
		t.Error("expected ErrNoRemote without bitbucket remotes")
	}
	if _, err := RepoFromRemotes(&Factory{}); err == nil {
		t.Error("expected error without GitClient")
	}
}

func TestEnableRepoOverridePrecedence(t *testing.T) {
	g := realGitRepo(t, map[string]string{"origin": "https://bitbucket.org/acme/widgets.git"})
	f := newFactory(t, g)
	cfg, _ := f.Config()
	_ = cfg.Set("workspace", "dflt")
	cmd := &cobra.Command{Use: "x", RunE: func(*cobra.Command, []string) error { return nil }}
	EnableRepoOverride(cmd, f)

	t.Setenv("BB_REPO", "")
	cmd.SetArgs([]string{})
	_ = cmd.Execute()
	if r, err := f.BaseRepo(); err != nil || r.FullName() != "acme/widgets" {
		t.Errorf("remote fallback: %+v %v", r, err)
	}
	t.Setenv("BB_REPO", "env/repo")
	if r, _ := f.BaseRepo(); r.FullName() != "env/repo" {
		t.Errorf("BB_REPO: %+v", r)
	}
	cmd.SetArgs([]string{"--repo", "bare"})
	_ = cmd.Execute()
	if r, _ := f.BaseRepo(); r.FullName() != "dflt/bare" {
		t.Errorf("--repo with default workspace: %+v", r)
	}
	if DefaultWorkspace(f) != "dflt" {
		t.Error("DefaultWorkspace")
	}
}

func TestPromptErrorAndOptionalArg(t *testing.T) {
	if PromptError(nil) != ErrCancel || PromptError(ErrCancel) != ErrCancel {
		t.Error("nil/cancel must map to ErrCancel")
	}
	other := context.DeadlineExceeded
	if PromptError(other) != other {
		t.Error("other errors pass through")
	}
	if OptionalArg(nil) != "" || OptionalArg([]string{"a", "b"}) != "a" {
		t.Error("OptionalArg")
	}
	if !IsUserCancellation(ErrCancel) || IsUserCancellation(other) {
		t.Error("IsUserCancellation")
	}
	if (Repo{Workspace: "a", Slug: "b"}).FullName() != "a/b" {
		t.Error("FullName")
	}
	if NewAuthError("").Error() == "" {
		t.Error("NewAuthError default message")
	}
}
