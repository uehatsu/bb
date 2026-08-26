// Package pr implements `bb pr` subcommands.
package pr

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/gitctx"
	"github.com/uehatsu/bb/internal/iostreams"
)

// NewCmdPR returns the `pr` command group.
func NewCmdPR(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr <command>",
		Args:  cobra.ArbitraryArgs,
		RunE:  cmdutil.GroupRunE, // unknown subcommands must fail, not print help with exit 0
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

// prCoreFields is enough for every action command (approve, merge, decline,
// checkout, ...). `pr view` fetches the full object instead.
const prCoreFields = "id,title,description,state,draft,author,source.branch.name,source.repository.full_name,source.repository.uuid,destination.branch.name,destination.repository.full_name,close_source_branch,reviewers,participants,comment_count,task_count,created_on,updated_on,merge_commit.hash,links.html.href"

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

// fetchPR gets a PR by number with the core fields; pass fields="" for the
// full object.
func fetchPR(ctx context.Context, client *api.Client, repo cmdutil.Repo, id int, fields string) (*bitbucket.PullRequest, error) {
	var pr bitbucket.PullRequest
	req := api.Request{Path: prPath(repo, id, "")}
	if fields != "" {
		req.Query = map[string][]string{"fields": {fields}}
	}
	if _, err := client.Do(ctx, req, &pr); err != nil {
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
		Limit:  1,
		Query:  fmt.Sprintf("source.branch.name=%s", api.BBQLQuote(branch)),
		Extra:  map[string][]string{"state": {"OPEN"}},
		Fields: "values." + strings.ReplaceAll(prCoreFields, ",", ",values.") + ",next",
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
	return &found[0], nil
}

// baseRepoFor returns the repository a PR selector refers to. A pull request
// URL names its repository, so no checkout, -R or BB_REPO is needed (and none
// of them is consulted); any other selector uses the base repository.
func baseRepoFor(f *cmdutil.Factory, selector string) (cmdutil.Repo, error) {
	sel, err := parsePRSelector(strings.TrimSpace(selector))
	if err != nil {
		return cmdutil.Repo{}, err
	}
	if sel.repo != nil {
		return *sel.repo, nil
	}
	return f.BaseRepo()
}

// resolvePR resolves a PR from a selector: a number, a branch name, a PR URL,
// or "" for the current branch, and returns it together with the repository
// it belongs to. The repository comes from the URL when one is given (like
// gh, never aliased onto the current repository); otherwise from --repo,
// BB_REPO, or the git remotes (see baseRepoFor). Only the core fields are
// fetched.
func resolvePR(ctx context.Context, f *cmdutil.Factory, client *api.Client, selector string) (*bitbucket.PullRequest, cmdutil.Repo, error) {
	return resolvePRFields(ctx, f, client, selector, prCoreFields)
}

// resolvePRFull resolves a PR with every field (for `pr view`).
func resolvePRFull(ctx context.Context, f *cmdutil.Factory, client *api.Client, selector string) (*bitbucket.PullRequest, cmdutil.Repo, error) {
	return resolvePRFields(ctx, f, client, selector, "")
}

func resolvePRFields(ctx context.Context, f *cmdutil.Factory, client *api.Client, selector, fields string) (*bitbucket.PullRequest, cmdutil.Repo, error) {
	selector = strings.TrimSpace(selector)
	repo, err := baseRepoFor(f, selector)
	if err != nil {
		return nil, cmdutil.Repo{}, err
	}
	if selector == "" {
		if f.GitClient == nil {
			return nil, repo, errors.New("pull request number or branch required")
		}
		g, err := f.GitClient()
		if err != nil {
			return nil, repo, errors.New("pull request number or branch required")
		}
		branch, err := g.CurrentBranch(ctx)
		if err != nil {
			return nil, repo, fmt.Errorf("could not determine current branch: %w", err)
		}
		pr, err := findPRForBranch(ctx, client, repo, branch)
		return pr, repo, err
	}
	sel, _ := parsePRSelector(selector) // validated by baseRepoFor
	if sel.number > 0 {
		pr, err := fetchPR(ctx, client, repo, sel.number, fields)
		return pr, repo, err
	}
	pr, err := findPRForBranch(ctx, client, repo, selector)
	return pr, repo, err
}

// prSelector is a parsed PR selector.
type prSelector struct {
	number int           // > 0 when the selector names a PR by number
	repo   *cmdutil.Repo // set when the selector was a URL
}

// parsePRSelector accepts "42", "#42", a branch name, or a bitbucket.org
// pull request URL (https://bitbucket.org/{ws}/{slug}/pull-requests/{n}...).
// URLs for other hosts or with an unexpected path are rejected rather than
// silently reduced to a number.
func parsePRSelector(s string) (prSelector, error) {
	if strings.Contains(s, "://") {
		u, err := url.Parse(s)
		if err != nil {
			return prSelector{}, fmt.Errorf("invalid pull request URL %q", s)
		}
		host := strings.ToLower(u.Hostname())
		if u.Scheme != "https" || (host != "bitbucket.org" && host != "www.bitbucket.org") {
			return prSelector{}, fmt.Errorf("not a bitbucket.org pull request URL: %q", s)
		}
		parts := strings.Split(strings.Trim(u.Path, "/"), "/")
		if len(parts) < 4 || parts[2] != "pull-requests" || !gitctx.ValidName(parts[0]) || !gitctx.ValidName(parts[1]) {
			return prSelector{}, fmt.Errorf("not a pull request URL: %q (expected https://bitbucket.org/WORKSPACE/REPO/pull-requests/NUMBER)", s)
		}
		n, err := strconv.Atoi(parts[3])
		if err != nil || n <= 0 {
			return prSelector{}, fmt.Errorf("invalid pull request number in URL %q", s)
		}
		return prSelector{number: n, repo: &cmdutil.Repo{Workspace: parts[0], Slug: parts[1]}}, nil
	}
	if n, err := strconv.Atoi(strings.TrimPrefix(s, "#")); err == nil {
		if n <= 0 {
			return prSelector{}, fmt.Errorf("not a pull request number: %q", s)
		}
		return prSelector{number: n}, nil
	}
	return prSelector{}, nil // branch name
}

// parsePRNumber returns the PR number for numeric or URL selectors.
func parsePRNumber(s string) (int, error) {
	sel, err := parsePRSelector(s)
	if err != nil {
		return 0, err
	}
	if sel.number == 0 {
		return 0, fmt.Errorf("not a pull request number: %q", s)
	}
	return sel.number, nil
}

// currentUser fetches the authenticated user (for @me).
func currentUser(ctx context.Context, client *api.Client) (*bitbucket.User, error) {
	var u bitbucket.User
	if _, err := client.Do(ctx, api.Request{Path: "/user"}, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func stateColor(cs *iostreams.ColorScheme, pr *bitbucket.PullRequest) func(string) string {
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
