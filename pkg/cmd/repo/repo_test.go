package repo

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/testutil"
)

const repoJSON = `{"name":"Widgets","slug":"widgets","full_name":"acme/widgets","description":"Gadgets","is_private":true,"language":"go","fork_policy":"allow_forks","updated_on":"2026-08-01T00:00:00Z","mainbranch":{"name":"main"},"project":{"key":"PROJ","name":"Project"},"links":{"html":{"href":"https://bitbucket.org/acme/widgets"},"clone":[{"name":"https","href":"https://ueno@bitbucket.org/acme/widgets.git"},{"name":"ssh","href":"git@bitbucket.org:acme/widgets.git"}]}}`

func TestListWorkspaceWithFilters(t *testing.T) {
	h := testutil.NewHarness(t)
	var got *http.Request
	h.Handle("/repositories/acme", func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.Write([]byte(`{"values":[` + repoJSON + `]}`))
	})
	cmd := NewCmdList(h.Factory)
	cmd.SetArgs([]string{"acme", "--role", "admin", "--language", "go", "--visibility", "private", "-L", "5"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	q := got.URL.Query()
	if q.Get("role") != "admin" || q.Get("pagelen") != "5" || q.Get("sort") != "-updated_on" {
		t.Errorf("query: %v", q)
	}
	if !strings.Contains(q.Get("q"), `language="go"`) || !strings.Contains(q.Get("q"), "is_private=true") {
		t.Errorf("q: %s", q.Get("q"))
	}
	if !strings.HasPrefix(h.Out.String(), "acme/widgets\tGadgets\tprivate\t2026-08-01") {
		t.Errorf("out: %q", h.Out.String())
	}
}

func TestListRejectsMemberRole(t *testing.T) {
	h := testutil.NewHarness(t)
	cmd := NewCmdList(h.Factory)
	cmd.SetArgs([]string{"acme", "--role", "member"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "CHANGE-2770") {
		t.Errorf("expected CHANGE-2770 error, got %v", err)
	}
}

func TestListAllWorkspacesAndJSON(t *testing.T) {
	h := testutil.NewHarness(t)
	h.JSON("GET", "/user/workspaces", 200, `{"values":[{"workspace":{"slug":"acme"}},{"workspace":{"slug":"other"}}]}`)
	h.JSON("GET", "/repositories/acme", 200, `{"values":[`+repoJSON+`]}`)
	h.JSON("GET", "/repositories/other", 200, `{"values":[{"full_name":"other/x","slug":"x"}]}`)
	cmd := NewCmdList(h.Factory)
	cmd.SetArgs([]string{"--json", "fullName,mainBranch"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if h.Out.String() != `[{"fullName":"acme/widgets","mainBranch":"main"},{"fullName":"other/x","mainBranch":null}]`+"\n" {
		t.Errorf("json: %s", h.Out.String())
	}
	// limit across workspaces
	h.Out.Reset()
	cmd = NewCmdList(h.Factory)
	cmd.SetArgs([]string{"-L", "1"})
	_ = cmd.Execute()
	if strings.Count(h.Out.String(), "\n") != 1 {
		t.Errorf("limit should cap total: %q", h.Out.String())
	}
}

func TestView(t *testing.T) {
	h := testutil.NewHarness(t)
	h.JSON("GET", "/repositories/acme/widgets", 200, repoJSON)
	h.Handle("/repositories/acme/widgets/src/main/README.md", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("# Widgets\nHello"))
	})
	cmd := NewCmdView(h.Factory)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	s := h.Out.String()
	for _, want := range []string{"acme/widgets", "Gadgets", "Project (PROJ)", "private", "Main branch: main", "# Widgets", "https://bitbucket.org/acme/widgets"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in:\n%s", want, s)
		}
	}
	h.Out.Reset()
	cmd = NewCmdView(h.Factory)
	cmd.SetArgs([]string{"acme/widgets", "--json", "cloneUrls", "--jq", ".cloneUrls[1].href"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if h.Out.String() != "git@bitbucket.org:acme/widgets.git\n" {
		t.Errorf("jq: %q", h.Out.String())
	}
}

func TestCreate(t *testing.T) {
	h := testutil.NewHarness(t)
	var body string
	h.Handle("/repositories/acme/widgets", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(201)
		w.Write([]byte(repoJSON))
	})
	cmd := NewCmdCreate(h.Factory)
	cmd.SetArgs([]string{"acme/Widgets", "--private", "--project", "PROJ", "-d", "Gadgets", "--fork-policy", "no_forks"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"scm":"git"`, `"is_private":true`, `"project":{"key":"PROJ"}`, `"description":"Gadgets"`, `"fork_policy":"no_forks"`} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %s: %s", want, body)
		}
	}
	if h.Out.String() != "https://bitbucket.org/acme/widgets\n" {
		t.Errorf("out: %q", h.Out.String())
	}
	// non-interactive without visibility
	cmd = NewCmdCreate(h.Factory)
	cmd.SetArgs([]string{"acme/x"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error without --private/--public")
	}
	// default workspace from config
	_ = h.Config.Set("workspace", "acme")
	cmd = NewCmdCreate(h.Factory)
	cmd.SetArgs([]string{"widgets", "--public"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `"is_private":false`) {
		t.Errorf("public: %s", body)
	}
}

func TestDeleteConfirmation(t *testing.T) {
	h := testutil.NewHarness(t)
	deleted := false
	h.Handle("/repositories/acme/widgets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			deleted = true
			w.WriteHeader(204)
		}
	})
	cmd := NewCmdDelete(h.Factory)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error: non-interactive without --yes")
	}
	h.SetTTY(true)
	h.Prompt.Inputs = []string{"wrong"}
	cmd = NewCmdDelete(h.Factory)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil || deleted {
		t.Error("mismatch must abort")
	}
	h.Prompt.Inputs = []string{"acme/widgets"}
	cmd = NewCmdDelete(h.Factory)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil || !deleted {
		t.Errorf("delete: %v %v", err, deleted)
	}
}

func TestEditAndFork(t *testing.T) {
	h := testutil.NewHarness(t)
	var body, method string
	h.Handle("/repositories/acme/widgets", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body, method = string(b), r.Method
		w.Write([]byte(repoJSON))
	})
	cmd := NewCmdEdit(h.Factory)
	cmd.SetArgs([]string{"--visibility", "public", "--default-branch", "develop", "-d", ""})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if method != "PUT" || !strings.Contains(body, `"is_private":false`) || !strings.Contains(body, `"mainbranch":{"name":"develop"}`) || !strings.Contains(body, `"description":""`) {
		t.Errorf("%s %s", method, body)
	}
	cmd = NewCmdEdit(h.Factory)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error with no changes")
	}

	h.Handle("/repositories/acme/widgets/forks", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(201)
		w.Write([]byte(`{"full_name":"me/widgets-fork","links":{"html":{"href":"https://bitbucket.org/me/widgets-fork"}}}`))
	})
	cmd = NewCmdFork(h.Factory)
	cmd.SetArgs([]string{"--workspace", "me", "--name", "widgets-fork"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `"workspace":{"slug":"me"}`) || !strings.Contains(body, `"name":"widgets-fork"`) {
		t.Errorf("fork body: %s", body)
	}
}

func TestCloneRunsGitAndAddsUpstream(t *testing.T) {
	h := testutil.NewHarness(t)
	g := h.UseGit()
	h.JSON("GET", "/repositories/me/widgets", 200, `{"full_name":"me/widgets","parent":{"full_name":"acme/widgets"},"links":{"clone":[{"name":"https","href":"https://me@bitbucket.org/me/widgets.git"}]}}`)
	cmd := NewCmdClone(h.Factory)
	cmd.SetArgs([]string{"me/widgets", "--", "--depth", "1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"git clone --depth 1 -- https://bitbucket.org/me/widgets.git",
		"cd widgets",
		"git remote add upstream https://bitbucket.org/acme/widgets.git",
	}
	if strings.Join(g.Calls, "\n") != strings.Join(want, "\n") {
		t.Errorf("git calls:\n%s\nwant:\n%s", strings.Join(g.Calls, "\n"), strings.Join(want, "\n"))
	}
}

func TestCloneURLFor(t *testing.T) {
	var r bitbucket.Repository
	_ = jsonUnmarshal(repoJSON, &r)
	if got, _ := cloneURLFor(&r, "https"); got != "https://bitbucket.org/acme/widgets.git" {
		t.Errorf("https: %s", got)
	}
	if got, _ := cloneURLFor(&r, "ssh"); got != "git@ssh.bitbucket.org:acme/widgets.git" {
		t.Errorf("ssh must use ssh.bitbucket.org: %s", got)
	}
	evil := bitbucket.Repository{FullName: "acme/widgets"}
	_ = jsonUnmarshal(`{"full_name":"acme/widgets","links":{"clone":[{"name":"https","href":"https://evil.example.com/x.git"}]}}`, &evil)
	if got, err := cloneURLFor(&evil, "https"); err != nil || got != "https://bitbucket.org/acme/widgets.git" {
		t.Errorf("foreign clone link must be ignored: %s %v", got, err)
	}
}
