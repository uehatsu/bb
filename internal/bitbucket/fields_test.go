package bitbucket

import (
	"encoding/json"
	"testing"
)

// Every field function must run without panicking on a zero value and
// produce JSON-encodable output; this catches nil dereferences early.
func TestFieldsZeroValueSafe(t *testing.T) {
	check := func(name string, m map[string]any) {
		t.Helper()
		if _, err := json.Marshal(m); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	check("repo", RepositoryFields.Export(Repository{}, nil))
	check("pr", PullRequestFields.Export(PullRequest{}, nil))
	check("pipeline", PipelineFields.Export(Pipeline{}, nil))
	check("workspace", WorkspaceFields.Export(Workspace{}, nil))
	check("project", ProjectFields.Export(Project{}, nil))
	check("branch", BranchFields.Export(Branch{}, nil))
	check("comment", CommentFields.Export(Comment{}, nil))
}

func TestFieldsValidate(t *testing.T) {
	if err := PullRequestFields.Validate([]string{"id", "title"}); err != nil {
		t.Fatal(err)
	}
	if err := PullRequestFields.Validate([]string{"id", "nope"}); err == nil {
		t.Fatal("expected error for unknown field")
	}
	got := PullRequestFields.Export(PullRequest{ID: 7, Title: "x"}, []string{"id"})
	if len(got) != 1 || got["id"] != 7 {
		t.Errorf("export subset: %v", got)
	}
}

func TestRepositoryCloneLinks(t *testing.T) {
	raw := `{"name":"r","slug":"r","full_name":"ws/r","links":{"html":{"href":"https://bitbucket.org/ws/r"},"clone":[{"name":"https","href":"https://bitbucket.org/ws/r.git"},{"name":"ssh","href":"git@bitbucket.org:ws/r.git"}]},"mainbranch":{"name":"main"}}`
	var r Repository
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		t.Fatal(err)
	}
	if r.CloneURL("ssh") != "git@bitbucket.org:ws/r.git" || r.Links.HTML() != "https://bitbucket.org/ws/r" || r.MainBranch.Name != "main" {
		t.Errorf("parse: %+v", r)
	}
	out, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var back Repository
	if err := json.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	if back.CloneURL("https") != "https://bitbucket.org/ws/r.git" {
		t.Errorf("roundtrip clone links lost: %s", out)
	}
}
