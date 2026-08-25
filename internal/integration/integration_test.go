// Package integration runs smoke tests against the real Bitbucket Cloud API.
// They execute only when BB_INTEGRATION=1 and credentials are provided via
// BB_EMAIL/BB_TOKEN (or BB_TOKEN alone for access tokens). BB_INTEGRATION_REPO
// (WORKSPACE/REPO) selects a repository the token can read.
package integration

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/config"
)

func client(t *testing.T) *api.Client {
	t.Helper()
	if os.Getenv("BB_INTEGRATION") != "1" {
		t.Skip("set BB_INTEGRATION=1 to run integration tests")
	}
	cred, ok, err := config.EnvCredential(os.Getenv)
	if err != nil || !ok {
		t.Skipf("BB_TOKEN not configured: %v", err)
	}
	return api.NewClient(api.NewAuthenticator(cred))
}

func TestUserAndWorkspaces(t *testing.T) {
	c := client(t)
	ctx := context.Background()
	var u bitbucket.User
	if _, err := c.Do(ctx, api.Request{Path: "/user"}, &u); err != nil {
		t.Fatalf("GET /user: %v", err)
	}
	if u.UUID == "" {
		t.Error("user uuid empty")
	}
	n := 0
	if err := api.Paginate(ctx, c, "/user/workspaces", api.ListOptions{Limit: 5, Fields: "values.workspace.slug,next"}, func(m bitbucket.WorkspaceMembership) error {
		if m.Workspace.Slug == "" {
			t.Error("workspace slug empty")
		}
		n++
		return nil
	}); err != nil {
		t.Fatalf("GET /user/workspaces: %v", err)
	}
	if n == 0 {
		t.Log("user belongs to no workspaces")
	}
}

func TestRepositoryAndPullRequests(t *testing.T) {
	c := client(t)
	full := os.Getenv("BB_INTEGRATION_REPO")
	if full == "" {
		t.Skip("BB_INTEGRATION_REPO not set")
	}
	ws, slug, ok := strings.Cut(full, "/")
	if !ok {
		t.Fatalf("BB_INTEGRATION_REPO must be WORKSPACE/REPO")
	}
	ctx := context.Background()
	var r bitbucket.Repository
	if _, err := c.Do(ctx, api.Request{Path: "/repositories/" + ws + "/" + slug}, &r); err != nil {
		t.Fatalf("GET repository: %v", err)
	}
	if r.FullName == "" || r.CloneURL("https") == "" {
		t.Errorf("repository fields missing: %+v", r)
	}
	if err := api.Paginate(ctx, c, "/repositories/"+ws+"/"+slug+"/pullrequests", api.ListOptions{Limit: 3, Extra: map[string][]string{"state": {"OPEN", "MERGED"}}}, func(p bitbucket.PullRequest) error {
		if p.ID == 0 || p.Title == "" {
			t.Errorf("pull request fields missing: %+v", p)
		}
		return nil
	}); err != nil {
		t.Fatalf("GET pullrequests: %v", err)
	}
	// Ensure the removed cross-workspace API really is gone (CHANGE-2770);
	// if it comes back this test documents the drift.
	_, err := c.Do(ctx, api.Request{Path: "/user/permissions/repositories", Query: map[string][]string{"pagelen": {"1"}}}, nil)
	if err == nil {
		t.Log("NOTE: /user/permissions/repositories responded 2xx; CHANGE-2770 assumptions may be outdated")
	}
}

// TestPullRequestPartialUpdate verifies the assumption that PUT on a pull
// request accepts {title, draft} (bb pr ready) and {title, description}
// (bb pr edit). Requires BB_INTEGRATION_WRITE=1 and BB_INTEGRATION_PR=<id>
// on BB_INTEGRATION_REPO; the PR is left unchanged (draft toggled twice).
func TestPullRequestPartialUpdate(t *testing.T) {
	c := client(t)
	if os.Getenv("BB_INTEGRATION_WRITE") != "1" || os.Getenv("BB_INTEGRATION_PR") == "" {
		t.Skip("set BB_INTEGRATION_WRITE=1 and BB_INTEGRATION_PR to run")
	}
	ws, slug, _ := strings.Cut(os.Getenv("BB_INTEGRATION_REPO"), "/")
	path := "/repositories/" + ws + "/" + slug + "/pullrequests/" + os.Getenv("BB_INTEGRATION_PR")
	ctx := context.Background()
	var pr bitbucket.PullRequest
	if _, err := c.Do(ctx, api.Request{Path: path}, &pr); err != nil {
		t.Fatal(err)
	}
	for _, draft := range []bool{!pr.Draft, pr.Draft} {
		var out bitbucket.PullRequest
		if _, err := c.Do(ctx, api.Request{Method: "PUT", Path: path, Body: map[string]any{"title": pr.Title, "draft": draft}}, &out); err != nil {
			t.Fatalf("PUT {title, draft}: %v", err)
		}
		if out.Draft != draft {
			t.Errorf("draft not applied: want %v got %v", draft, out.Draft)
		}
	}
	var out bitbucket.PullRequest
	if _, err := c.Do(ctx, api.Request{Method: "PUT", Path: path, Body: map[string]any{"title": pr.Title, "description": pr.Description}}, &out); err != nil {
		t.Fatalf("PUT {title, description}: %v", err)
	}
}

// TestPipelineCommitTarget verifies the pipeline_commit_target payload shape
// by triggering and immediately stopping a pipeline. Requires
// BB_INTEGRATION_WRITE=1 and BB_INTEGRATION_COMMIT=<hash>.
func TestPipelineCommitTarget(t *testing.T) {
	c := client(t)
	if os.Getenv("BB_INTEGRATION_WRITE") != "1" || os.Getenv("BB_INTEGRATION_COMMIT") == "" {
		t.Skip("set BB_INTEGRATION_WRITE=1 and BB_INTEGRATION_COMMIT to run")
	}
	ws, slug, _ := strings.Cut(os.Getenv("BB_INTEGRATION_REPO"), "/")
	base := "/repositories/" + ws + "/" + slug + "/pipelines"
	ctx := context.Background()
	body := map[string]any{"target": map[string]any{"type": "pipeline_commit_target", "commit": map[string]string{"type": "commit", "hash": os.Getenv("BB_INTEGRATION_COMMIT")}}}
	var p bitbucket.Pipeline
	if _, err := c.Do(ctx, api.Request{Method: "POST", Path: base, Body: body}, &p); err != nil {
		t.Fatalf("POST pipeline_commit_target: %v", err)
	}
	if _, err := c.Do(ctx, api.Request{Method: "POST", Path: base + "/" + p.UUID + "/stopPipeline"}, nil); err != nil {
		t.Logf("could not stop pipeline #%d: %v", p.BuildNumber, err)
	}
}

// TestWorkspaceMembersFilter documents whether /workspaces/{ws}/members
// accepts a q= filter on user.nickname (used by bb pr create --reviewer).
func TestWorkspaceMembersFilter(t *testing.T) {
	c := client(t)
	full := os.Getenv("BB_INTEGRATION_REPO")
	if full == "" {
		t.Skip("BB_INTEGRATION_REPO not set")
	}
	ws, _, _ := strings.Cut(full, "/")
	ctx := context.Background()
	var u bitbucket.User
	if _, err := c.Do(ctx, api.Request{Path: "/user"}, &u); err != nil {
		t.Fatal(err)
	}
	n := 0
	err := api.Paginate(ctx, c, "/workspaces/"+ws+"/members", api.ListOptions{Limit: 5, Query: "user.nickname=" + api.BBQLQuote(u.Nickname), Fields: "values.user.uuid,values.user.nickname,next"}, func(m bitbucket.WorkspaceMembership) error {
		n++
		return nil
	})
	if err != nil {
		t.Logf("members q= filter NOT supported (%v); bb falls back to scanning", err)
		return
	}
	t.Logf("members q= filter supported; %d match(es) for %s", n, u.Nickname)
}
