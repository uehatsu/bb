package pr

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/gitctx"
)

// CreateOptions for `pr create`.
type CreateOptions struct {
	Title             string
	Body              string
	BodyFile          string
	Base              string
	Head              string
	Reviewers         []string
	Draft             bool
	CloseSourceBranch bool
	Fill              bool
	Web               bool
	NoDefaultReviewer bool
}

// NewCmdCreate returns `pr create`.
func NewCmdCreate(f *cmdutil.Factory) *cobra.Command {
	opts := &CreateOptions{}
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a pull request",
		Long: `Create a pull request on Bitbucket.

The source branch defaults to the current git branch and the destination to
the repository's main branch. When --title is omitted in a terminal, the
title and body are prompted for; --fill derives them from the commits.

Reviewers are given by nickname and resolved against workspace members. The
repository's effective default reviewers are added automatically unless
--no-default-reviewers is set.`,
		Example: `  $ bb pr create --title "Fix login" --body "Closes JIRA-1"
  $ bb pr create --fill --reviewer alice --reviewer bob
  $ bb pr create --base develop --draft
  $ bb pr create --web`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.Body != "" && opts.BodyFile != "" {
				return cmdutil.FlagErrorf("specify only one of --body or --body-file")
			}
			if opts.BodyFile != "" {
				b, err := readBodyFile(opts.BodyFile, f.IOStreams.In)
				if err != nil {
					return err
				}
				opts.Body = b
			}
			return runCreate(cmd.Context(), f, opts)
		},
	}
	cmd.Flags().StringVarP(&opts.Title, "title", "t", "", "Title for the pull request")
	cmd.Flags().StringVarP(&opts.Body, "body", "b", "", "Body for the pull request")
	cmd.Flags().StringVarP(&opts.BodyFile, "body-file", "F", "", "Read body text from file (use \"-\" to read from standard input)")
	cmd.Flags().StringVarP(&opts.Base, "base", "B", "", "The branch into which you want your code merged")
	cmd.Flags().StringVarP(&opts.Head, "head", "H", "", "The branch that contains commits for your pull request (default: current branch)")
	cmd.Flags().StringSliceVarP(&opts.Reviewers, "reviewer", "r", nil, "Request reviews from people by nickname")
	cmd.Flags().BoolVarP(&opts.Draft, "draft", "d", false, "Mark pull request as a draft")
	cmd.Flags().BoolVar(&opts.CloseSourceBranch, "close-source-branch", false, "Delete the source branch after merge")
	cmd.Flags().BoolVarP(&opts.Fill, "fill", "f", false, "Use commit info for title and body")
	cmd.Flags().BoolVarP(&opts.Web, "web", "w", false, "Open the browser to create a pull request")
	cmd.Flags().BoolVar(&opts.NoDefaultReviewer, "no-default-reviewers", false, "Do not add the repository's default reviewers")
	return cmd
}

func readBodyFile(path string, stdin io.Reader) (string, error) {
	if path == "-" {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func runCreate(ctx context.Context, f *cmdutil.Factory, opts *CreateOptions) error {
	ios := f.IOStreams
	cs := ios.ColorScheme()
	repo, err := f.BaseRepo()
	if err != nil {
		return err
	}

	head := opts.Head
	if head == "" {
		g, err := f.GitClient()
		if err != nil {
			return errors.New("--head is required outside a git repository")
		}
		head, err = g.CurrentBranch(ctx)
		if err != nil {
			return fmt.Errorf("could not determine current branch: %w", err)
		}
	}

	if opts.Web {
		return cmdutil.OpenBrowser(f, gitctx.NewPullRequestWebURL(repo.Workspace, repo.Slug, head, opts.Base, opts.Title))
	}

	client, err := f.APIClient()
	if err != nil {
		return err
	}

	base := opts.Base
	if base == "" {
		if base, err = cmdutil.MainBranch(ctx, client, repo); err != nil {
			return err
		}
	}
	if head == base {
		return fmt.Errorf("source and destination branch are both %q; use --base or --head", head)
	}

	title, body := opts.Title, opts.Body
	if opts.Fill && title == "" {
		title, body, err = fillFromCommits(ctx, client, repo, head, base, body)
		if err != nil {
			return err
		}
	}
	if title == "" {
		if !ios.CanPrompt() {
			return cmdutil.FlagErrorf("--title (or --fill) is required when not running interactively")
		}
		fmt.Fprintf(ios.ErrOut, "\nCreating pull request for %s into %s in %s\n\n", cs.Cyan(head), cs.Cyan(base), repo.FullName())
		if title, err = f.Prompter.Input("Title", ""); err != nil {
			return cmdutil.PromptError(err)
		}
		if strings.TrimSpace(title) == "" {
			return errors.New("title must not be empty")
		}
		if body == "" {
			if body, err = f.Prompter.Editor("Body", ""); err != nil {
				return cmdutil.PromptError(err)
			}
		}
	}

	reviewers, err := resolveReviewers(ctx, client, repo, opts.Reviewers, !opts.NoDefaultReviewer)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"title":               title,
		"source":              map[string]any{"branch": map[string]string{"name": head}},
		"close_source_branch": opts.CloseSourceBranch,
		"draft":               opts.Draft,
	}
	if base != "" {
		payload["destination"] = map[string]any{"branch": map[string]string{"name": base}}
	}
	if body != "" {
		payload["description"] = body
	}
	if len(reviewers) > 0 {
		rs := make([]map[string]string, len(reviewers))
		for i, r := range reviewers {
			rs[i] = map[string]string{"uuid": r}
		}
		payload["reviewers"] = rs
	}
	var created bitbucket.PullRequest
	if _, err := client.Do(ctx, api.Request{Method: "POST", Path: prPath(repo, 0, ""), Body: payload}, &created); err != nil {
		return err
	}
	fmt.Fprintln(ios.Out, created.Links.HTML())
	return nil
}

// fillFromCommits derives a title/body from the commits on head not in base.
func fillFromCommits(ctx context.Context, client *api.Client, repo cmdutil.Repo, head, base, body string) (string, string, error) {
	var commits []bitbucket.Commit
	lo := api.ListOptions{Limit: 50, Fields: "values.hash,values.message,next", Extra: map[string][]string{"include": {head}, "exclude": {base}}}
	if err := api.Paginate(ctx, client, fmt.Sprintf("/repositories/%s/%s/commits", repo.Workspace, repo.Slug), lo, func(c bitbucket.Commit) error {
		commits = append(commits, c)
		return nil
	}); err != nil {
		return "", "", err
	}
	if len(commits) == 0 {
		return "", "", fmt.Errorf("no commits between %s and %s", base, head)
	}
	if len(commits) == 1 {
		subject, rest, _ := strings.Cut(strings.TrimSpace(commits[0].Message), "\n")
		if body == "" {
			body = strings.TrimSpace(rest)
		}
		return subject, body, nil
	}
	// oldest first for the body
	var lines []string
	for i := len(commits) - 1; i >= 0; i-- {
		subject, _, _ := strings.Cut(strings.TrimSpace(commits[i].Message), "\n")
		lines = append(lines, "- "+subject)
	}
	if body == "" {
		body = strings.Join(lines, "\n")
	}
	return head, body, nil
}

// resolveReviewers maps nicknames to UUIDs and optionally merges in the
// repository's effective default reviewers.
func resolveReviewers(ctx context.Context, client *api.Client, repo cmdutil.Repo, nicknames []string, includeDefaults bool) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	add := func(uuid string) {
		if uuid != "" && !seen[uuid] {
			seen[uuid] = true
			out = append(out, uuid)
		}
	}
	if includeDefaults {
		err := api.Paginate(ctx, client, fmt.Sprintf("/repositories/%s/%s/effective-default-reviewers", repo.Workspace, repo.Slug), api.ListOptions{Fields: "values.user.uuid,next"}, func(p struct {
			User bitbucket.Account `json:"user"`
		}) error {
			add(p.User.UUID)
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("fetching default reviewers (use --no-default-reviewers to skip): %w", err)
		}
	}
	if len(nicknames) == 0 {
		return out, nil
	}
	want := map[string]bool{}
	for _, n := range nicknames {
		n = strings.TrimSpace(strings.TrimPrefix(n, "@"))
		if n != "" {
			want[strings.ToLower(n)] = true
		}
	}
	if len(want) == 0 {
		return out, nil
	}
	found := map[string]string{}
	err := api.Paginate(ctx, client, fmt.Sprintf("/workspaces/%s/members", repo.Workspace), api.ListOptions{Fields: "values.user.uuid,values.user.nickname,values.user.display_name,next"}, func(m bitbucket.WorkspaceMembership) error {
		key := strings.ToLower(m.User.Nickname)
		if want[key] {
			found[key] = m.User.UUID
		} else if want[strings.ToLower(m.User.DisplayName)] {
			found[strings.ToLower(m.User.DisplayName)] = m.User.UUID
		}
		if len(found) == len(want) {
			return api.ErrStop
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("looking up reviewers: %w", err)
	}
	var missing []string
	for n := range want {
		if uuid, ok := found[n]; ok {
			add(uuid)
		} else {
			missing = append(missing, n)
		}
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("reviewer(s) not found in workspace %s: %s", repo.Workspace, strings.Join(missing, ", "))
	}
	return out, nil
}
