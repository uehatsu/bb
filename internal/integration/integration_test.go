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
