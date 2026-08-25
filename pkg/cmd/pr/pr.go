// Package pr implements `bb pr` subcommands.
package pr

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
)

// NewCmdPR returns the `pr` command group.
func NewCmdPR(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr <command>",
		Short: "Manage pull requests",
		Long:  "Work with Bitbucket pull requests.",
		Example: `  $ bb pr list
  $ bb pr create --title "Fix bug" --body "Details"
  $ bb pr checkout 42
  $ bb pr merge 42 --squash --delete-branch`,
	}
	cmdutil.EnableRepoOverride(cmd, f)
	cmd.AddCommand(
		NewCmdList(f),
		NewCmdView(f),
		NewCmdCreate(f),
		NewCmdCheckout(f),
		NewCmdMerge(f),
		NewCmdDecline(f),
		NewCmdReopen(f),
		NewCmdApprove(f),
		NewCmdUnapprove(f),
		NewCmdReview(f),
		NewCmdComment(f),
		NewCmdDiff(f),
		NewCmdStatus(f),
		NewCmdChecks(f),
		NewCmdEdit(f),
		NewCmdReady(f),
	)
	return cmd
}

const prListFields = "values.id,values.title,values.state,values.draft,values.author.nickname,values.author.display_name,values.author.uuid,values.source.branch.name,values.source.repository.full_name,values.destination.branch.name,values.updated_on,values.created_on,values.comment_count,values.task_count,values.close_source_branch,values.links.html.href,next"

func prPath(repo cmdutil.Repo, id int, suffix string) string {
	p := fmt.Sprintf("/repositories/%s/%s/pullrequests", repo.Workspace, repo.Slug)
	if id > 0 {
		p += "/" + strconv.Itoa(id)
	}
	if suffix != "" {
		p += "/" + strings.TrimPrefix(suffix, "/")
	}
	return p
}

// fetchPR gets a PR by number.
func fetchPR(ctx context.Context, client *api.Client, repo cmdutil.Repo, id int) (*bitbucket.PullRequest, error) {
	var pr bitbucket.PullRequest
	if _, err := client.Do(ctx, api.Request{Path: prPath(repo, id, "")}, &pr); err != nil {
		var herr *api.HTTPError
		if errors.As(err, &herr) && herr.IsNotFound() {
			return nil, fmt.Errorf("pull request #%d not found in %s", id, repo.FullName())
		}
		return nil, err
	}
	return &pr, nil
}

// findPRForBranch returns the open PR whose source branch matches.
func findPRForBranch(ctx context.Context, client *api.Client, repo cmdutil.Repo, branch string) (*bitbucket.PullRequest, error) {
	var found []bitbucket.PullRequest
	opts := api.ListOptions{
		Limit: 1,
		Query: fmt.Sprintf("source.branch.name=%s", api.BBQLQuote(branch)),
		Extra: map[string][]string{"state": {"OPEN"}},
	}
	err := api.Paginate(ctx, client, prPath(repo, 0, ""), opts, func(p bitbucket.PullRequest) error {
		found = append(found, p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("no open pull request found for branch %q", branch)
	}
	return fetchPR(ctx, client, repo, found[0].ID)
}

// resolvePR resolves a PR from a selector: a number, a branch name, a PR URL,
// or "" for the current branch.
func resolvePR(ctx context.Context, f *cmdutil.Factory, client *api.Client, repo cmdutil.Repo, selector string) (*bitbucket.PullRequest, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		if f.GitClient == nil {
			return nil, errors.New("pull request number or branch required")
		}
		g, err := f.GitClient()
		if err != nil {
			return nil, errors.New("pull request number or branch required")
		}
		branch, err := g.CurrentBranch(ctx)
		if err != nil {
			return nil, fmt.Errorf("could not determine current branch: %w", err)
		}
		return findPRForBranch(ctx, client, repo, branch)
	}
	if n, err := parsePRNumber(selector); err == nil {
		return fetchPR(ctx, client, repo, n)
	}
	return findPRForBranch(ctx, client, repo, selector)
}

// parsePRNumber accepts "42", "#42", or a bitbucket.org pull request URL.
func parsePRNumber(s string) (int, error) {
	s = strings.TrimPrefix(s, "#")
	if i := strings.Index(s, "/pull-requests/"); i >= 0 {
		s = s[i+len("/pull-requests/"):]
		if j := strings.IndexAny(s, "/?#"); j >= 0 {
			s = s[:j]
		}
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("not a pull request number: %q", s)
	}
	return n, nil
}

// currentUser fetches the authenticated user (for @me).
func currentUser(ctx context.Context, client *api.Client) (*bitbucket.User, error) {
	var u bitbucket.User
	if _, err := client.Do(ctx, api.Request{Path: "/user"}, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func stateColor(cs interface {
	Green(string) string
	Red(string) string
	Magenta(string) string
	Gray(string) string
}, pr *bitbucket.PullRequest) func(string) string {
	switch pr.State {
	case "OPEN":
		if pr.Draft {
			return cs.Gray
		}
		return cs.Green
	case "MERGED":
		return cs.Magenta
	case "DECLINED":
		return cs.Red
	}
	return cs.Gray
}

func prStateLabel(pr *bitbucket.PullRequest) string {
	if pr.State == "OPEN" && pr.Draft {
		return "DRAFT"
	}
	return pr.State
}
