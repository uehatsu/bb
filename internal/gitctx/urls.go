package gitctx

import (
	"net/url"
	"strconv"
	"strings"
)

// WebBase is the Bitbucket Cloud web UI origin. Every user-facing URL bb
// produces is derived from it here, so a change on Bitbucket's side (or a
// future host option) is a one-line edit. cmdutil.OpenBrowser only opens
// URLs under this host.
const WebBase = "https://bitbucket.org"

// ParseFullName parses "workspace/slug" into a Repo, validating both parts.
func ParseFullName(fullName string) (Repo, bool) {
	ws, slug, ok := strings.Cut(strings.Trim(fullName, "/"), "/")
	if !ok || !ValidName(ws) || !ValidName(slug) || strings.Contains(slug, "/") {
		return Repo{}, false
	}
	return Repo{Workspace: ws, Slug: slug}, true
}

// RepoWebURL returns the repository's web page.
func RepoWebURL(workspace, slug string) string {
	return WebBase + "/" + url.PathEscape(workspace) + "/" + url.PathEscape(slug)
}

// WorkspaceWebURL returns the workspace overview page.
func WorkspaceWebURL(workspace string) string {
	return WebBase + "/" + url.PathEscape(workspace) + "/"
}

// PullRequestWebURL returns a pull request page.
func PullRequestWebURL(workspace, slug string, id int) string {
	return RepoWebURL(workspace, slug) + "/pull-requests/" + strconv.Itoa(id)
}

// PullRequestsWebURL returns the pull request list page.
func PullRequestsWebURL(workspace, slug string) string {
	return RepoWebURL(workspace, slug) + "/pull-requests/"
}

// NewPullRequestWebURL returns the "create pull request" page pre-filled
// with source/destination/title.
func NewPullRequestWebURL(workspace, slug, source, dest, title string) string {
	q := url.Values{"source": {source}}
	if dest != "" {
		q.Set("dest", dest)
	}
	if title != "" {
		q.Set("title", title)
	}
	return RepoWebURL(workspace, slug) + "/pull-requests/new?" + q.Encode()
}

// PipelineWebURL returns a pipeline result page by build number.
func PipelineWebURL(workspace, slug string, buildNumber int) string {
	return RepoWebURL(workspace, slug) + "/pipelines/results/" + strconv.Itoa(buildNumber)
}

// BranchWebURL returns the branch overview page.
func BranchWebURL(workspace, slug, branch string) string {
	return RepoWebURL(workspace, slug) + "/branch/" + EscapePath(branch)
}

// EscapePath escapes each path segment while keeping "/" separators.
func EscapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}
