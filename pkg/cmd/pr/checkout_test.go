package pr

import (
	"errors"
	"strings"
	"testing"

	"github.com/uehatsu/bb/internal/testutil"
)

func TestCheckoutNewBranch(t *testing.T) {
	h := testutil.NewHarness(t)
	g := h.UseGit()
	g.Errors["rev-parse --verify --quiet refs/heads/feat/login"] = errors.New("missing")
	h.JSON("GET", "/repositories/acme/widgets/pullrequests/42", 200, prJSON)
	cmd := NewCmdCheckout(h.Factory)
	cmd.SetArgs([]string{"42"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"git fetch origin +refs/heads/feat/login:refs/remotes/origin/feat/login",
		"git rev-parse --verify --quiet refs/heads/feat/login",
		"git checkout -b feat/login --track origin/feat/login",
	}
	if got := strings.Join(g.Calls, "\n"); got != strings.Join(want, "\n") {
		t.Errorf("git calls:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
	}
	for _, c := range g.Calls {
		if strings.Contains(c, " -- ") {
			t.Errorf("commit-ish must not follow --: %s", c)
		}
	}
}

func TestCheckoutExistingForceAndDetach(t *testing.T) {
	h := testutil.NewHarness(t)
	g := h.UseGit()
	h.JSON("GET", "/repositories/acme/widgets/pullrequests/42", 200, prJSON)
	cmd := NewCmdCheckout(h.Factory)
	cmd.SetArgs([]string{"42", "--force", "-b", "local"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(g.Calls, "\n")
	if !strings.Contains(joined, "git checkout local\ngit reset --hard origin/feat/login") {
		t.Errorf("force path:\n%s", joined)
	}
	g.Calls = nil
	cmd = NewCmdCheckout(h.Factory)
	cmd.SetArgs([]string{"42", "--detach"})
	_ = cmd.Execute()
	if g.Calls[len(g.Calls)-1] != "git checkout --detach origin/feat/login" {
		t.Errorf("detach: %v", g.Calls)
	}
	g.Calls = nil
	cmd = NewCmdCheckout(h.Factory)
	cmd.SetArgs([]string{"42"})
	_ = cmd.Execute()
	if g.Calls[len(g.Calls)-1] != "git merge --ff-only origin/feat/login" {
		t.Errorf("ff path: %v", g.Calls)
	}
}

func TestCheckoutForkRemoteNameCollision(t *testing.T) {
	h := testutil.NewHarness(t)
	g := h.UseGit()
	g.Errors["rev-parse --verify --quiet refs/heads/feat/login"] = errors.New("missing")
	// "alice" remote already exists but points elsewhere
	g.Outputs["config --get remote.alice.url"] = "https://bitbucket.org/alice/other.git"
	fork := strings.Replace(prJSON, `"repository":{"full_name":"acme/widgets","workspace":{"slug":"acme"}}`, `"repository":{"full_name":"alice/widgets-fork"}`, 1)
	h.JSON("GET", "/repositories/acme/widgets/pullrequests/42", 200, fork)
	cmd := NewCmdCheckout(h.Factory)
	cmd.SetArgs([]string{"42"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(g.Calls, "\n")
	if !strings.Contains(joined, "git remote add fork-alice https://bitbucket.org/alice/widgets-fork.git") || !strings.Contains(joined, "--track fork-alice/feat/login") {
		t.Errorf("expected distinct remote name:\n%s", joined)
	}
}

func TestCheckoutFork(t *testing.T) {
	h := testutil.NewHarness(t)
	g := h.UseGit()
	g.Errors["rev-parse --verify --quiet refs/heads/feat/login"] = errors.New("missing")
	fork := strings.Replace(prJSON, `"repository":{"full_name":"acme/widgets","workspace":{"slug":"acme"}}`, `"repository":{"full_name":"alice/widgets-fork","type":"repository"}`, 1)
	h.JSON("GET", "/repositories/acme/widgets/pullrequests/42", 200, fork)
	_ = h.Config.Set("git_protocol", "ssh")
	cmd := NewCmdCheckout(h.Factory)
	cmd.SetArgs([]string{"42"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(g.Calls, "\n")
	for _, want := range []string{
		"git config --get remote.alice.url",
		"git remote add alice git@ssh.bitbucket.org:alice/widgets-fork.git",
		"git fetch alice +refs/heads/feat/login:refs/remotes/alice/feat/login",
		"git checkout -b feat/login --track alice/feat/login",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in:\n%s", want, joined)
		}
	}
}
