package gitctx

import "testing"

func TestParseRemoteURL(t *testing.T) {
	cases := map[string]Repo{
		"https://bitbucket.org/acme/widgets.git":              {"acme", "widgets"},
		"https://ueno@bitbucket.org/acme/widgets":             {"acme", "widgets"},
		"git@bitbucket.org:acme/widgets.git":                  {"acme", "widgets"},
		"git@ssh.bitbucket.org:acme/widgets.git":              {"acme", "widgets"},
		"ssh://git@ssh.bitbucket.org/acme/widgets.git":        {"acme", "widgets"},
		"ssh://git@altssh.bitbucket.org:443/acme/widgets.git": {"acme", "widgets"},
		"https://bitbucket.org/acme/widgets/src/main/":        {"acme", "widgets"},
	}
	for in, want := range cases {
		got, ok := ParseRemoteURL(in)
		if !ok || got != want {
			t.Errorf("%s: got %+v ok=%v", in, got, ok)
		}
	}
	for _, bad := range []string{"git@github.com:acme/widgets.git", "https://gitlab.com/a/b", "https://bitbucket.org/onlyws", "", "not a url"} {
		if _, ok := ParseRemoteURL(bad); ok {
			t.Errorf("%s should not parse", bad)
		}
	}
}

func TestParseRepoArg(t *testing.T) {
	if r, err := ParseRepoArg("acme/widgets", ""); err != nil || r.FullName() != "acme/widgets" {
		t.Errorf("%v %v", r, err)
	}
	if r, err := ParseRepoArg("widgets", "acme"); err != nil || r.FullName() != "acme/widgets" {
		t.Errorf("%v %v", r, err)
	}
	if _, err := ParseRepoArg("widgets", ""); err == nil {
		t.Error("expected error without default workspace")
	}
	if _, err := ParseRepoArg("a/b/c", ""); err == nil {
		t.Error("expected error for 3 parts")
	}
	if _, err := ParseRepoArg("../etc/passwd", "ws"); err == nil {
		t.Error("expected error for invalid chars")
	}
	if r, err := ParseRepoArg("https://bitbucket.org/acme/widgets", ""); err != nil || r.Slug != "widgets" {
		t.Errorf("url form: %v %v", r, err)
	}
	if !ValidName("{5c3c6b2a-1234-4bcd-9abc-1234567890ab}") {
		t.Error("uuid should be valid")
	}
}

func TestPickRemote(t *testing.T) {
	rs := []Remote{{"fork", Repo{"me", "x"}}, {"origin", Repo{"acme", "x"}}, {"upstream", Repo{"org", "x"}}}
	if r, _ := PickRemote(rs); r.Name != "upstream" {
		t.Error("upstream should win")
	}
	if r, _ := PickRemote(rs[:2]); r.Name != "origin" {
		t.Error("origin should win over fork")
	}
	if r, _ := PickRemote(rs[:1]); r.Name != "fork" {
		t.Error("fallback to first")
	}
	if _, err := PickRemote(nil); err != ErrNoRemote {
		t.Error("expected ErrNoRemote")
	}
}

func TestWebURLs(t *testing.T) {
	if RepoWebURL("acme", "widgets") != "https://bitbucket.org/acme/widgets" {
		t.Error(RepoWebURL("acme", "widgets"))
	}
	if PullRequestWebURL("acme", "widgets", 42) != "https://bitbucket.org/acme/widgets/pull-requests/42" {
		t.Error("pr url")
	}
	if PipelineWebURL("acme", "widgets", 7) != "https://bitbucket.org/acme/widgets/pipelines/results/7" {
		t.Error("pipeline url")
	}
	if got := NewPullRequestWebURL("acme", "widgets", "feat/x", "main", "Hi there"); got != "https://bitbucket.org/acme/widgets/pull-requests/new?dest=main&source=feat%2Fx&title=Hi+there" {
		t.Error(got)
	}
	// CJK (multi-byte) branch name: must be percent-encoded.
	if BranchWebURL("acme", "widgets", "feat/日本") != "https://bitbucket.org/acme/widgets/branch/feat/%E6%97%A5%E6%9C%AC" {
		t.Error(BranchWebURL("acme", "widgets", "feat/日本"))
	}
	if r, ok := ParseFullName("acme/widgets"); !ok || r.Slug != "widgets" {
		t.Error("ParseFullName")
	}
	for _, bad := range []string{"acme", "a/b/c", "../x/y", "", "acme/"} {
		if _, ok := ParseFullName(bad); ok {
			t.Errorf("%q should fail", bad)
		}
	}
}

func TestMoreWebURLs(t *testing.T) {
	if WorkspaceWebURL("acme") != "https://bitbucket.org/acme/" {
		t.Error(WorkspaceWebURL("acme"))
	}
	if PullRequestsWebURL("acme", "w") != "https://bitbucket.org/acme/w/pull-requests/" {
		t.Error(PullRequestsWebURL("acme", "w"))
	}
	if CloneURL(Repo{"a", "b"}, "https") != "https://bitbucket.org/a/b.git" || CloneURL(Repo{"a", "b"}, "ssh") != "git@ssh.bitbucket.org:a/b.git" {
		t.Error("CloneURL")
	}
	if _, ok := NormalizeCloneURL("https://github.com/a/b.git", "https"); ok {
		t.Error("foreign host must not normalize")
	}
}
