package pr

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
)

// NewCmdDecline returns `pr decline` (alias: close).
func NewCmdDecline(f *cmdutil.Factory) *cobra.Command {
	var deleteBranch bool
	cmd := &cobra.Command{
		Use:     "decline [<number> | <branch> | <url>]",
		Aliases: []string{"close"},
		Short:   "Decline (close) a pull request",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sel := cmdutil.OptionalArg(args)
			return withPR(cmd.Context(), f, sel, func(ctx context.Context, c *api.Client, repo cmdutil.Repo, pr *bitbucket.PullRequest) error {
				if pr.State != "OPEN" {
					return fmt.Errorf("pull request #%d is already %s", pr.ID, pr.State)
				}
				if _, err := c.Do(ctx, api.Request{Method: "POST", Path: prPath(repo, pr.ID, "decline")}, nil); err != nil {
					return err
				}
				fmt.Fprintf(f.IOStreams.ErrOut, "%s Declined pull request #%d (%s)\n", f.IOStreams.ColorScheme().Red("✓"), pr.ID, pr.Title)
				if deleteBranch {
					branch := pr.Source.Branch.Name
					// The source branch lives in the source repository, which for
					// fork PRs is not the base repository; never delete a same-named
					// branch in the base repo by mistake.
					if pr.Source.Repository != nil && pr.Source.Repository.FullName != "" && pr.Source.Repository.FullName != repo.FullName() {
						cs := f.IOStreams.ColorScheme()
						fmt.Fprintf(f.IOStreams.ErrOut, "%s Branch %s was not deleted: it belongs to fork %s. Delete it in the fork (e.g. `bb branch delete %s -R %s` if you have write access).\n",
							cs.WarningIcon(), branch, pr.Source.Repository.FullName, branch, pr.Source.Repository.FullName)
						return nil
					}
					if _, err := c.Do(ctx, api.Request{Method: "DELETE", Path: fmt.Sprintf("/repositories/%s/%s/refs/branches/%s", repo.Workspace, repo.Slug, url.PathEscape(branch))}, nil); err != nil {
						return fmt.Errorf("declined, but could not delete branch %s: %w", branch, err)
					}
					fmt.Fprintf(f.IOStreams.ErrOut, "%s Deleted branch %s\n", f.IOStreams.ColorScheme().Red("✓"), branch)
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVarP(&deleteBranch, "delete-branch", "d", false, "Delete the source branch after declining")
	return cmd
}

// NewCmdReopen exists for gh parity but Bitbucket cannot reopen declined PRs.
func NewCmdReopen(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:    "reopen [<number>]",
		Short:  "Reopen a pull request (not supported by Bitbucket)",
		Hidden: true,
		Long: `Bitbucket Cloud has no API to reopen a declined pull request; this command
exists only so that GitHub CLI users get an explanation instead of an unknown
command error. Create a new pull request from the same branch instead.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("declined pull requests cannot be reopened on Bitbucket Cloud; create a new pull request from the same branch instead: `bb pr create`")
		},
	}
}

// NewCmdApprove returns `pr approve`.
func NewCmdApprove(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "approve [<number> | <branch> | <url>]",
		Short: "Approve a pull request",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sel := cmdutil.OptionalArg(args)
			return withPR(cmd.Context(), f, sel, func(ctx context.Context, c *api.Client, repo cmdutil.Repo, pr *bitbucket.PullRequest) error {
				if _, err := c.Do(ctx, api.Request{Method: "POST", Path: prPath(repo, pr.ID, "approve")}, nil); err != nil {
					return err
				}
				fmt.Fprintf(f.IOStreams.ErrOut, "%s Approved pull request #%d (%s)\n", f.IOStreams.ColorScheme().SuccessIcon(), pr.ID, pr.Title)
				return nil
			})
		},
	}
}

// NewCmdUnapprove returns `pr unapprove`.
func NewCmdUnapprove(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "unapprove [<number> | <branch> | <url>]",
		Short: "Remove your approval from a pull request",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sel := cmdutil.OptionalArg(args)
			return withPR(cmd.Context(), f, sel, func(ctx context.Context, c *api.Client, repo cmdutil.Repo, pr *bitbucket.PullRequest) error {
				if _, err := c.Do(ctx, api.Request{Method: "DELETE", Path: prPath(repo, pr.ID, "approve")}, nil); err != nil {
					return err
				}
				fmt.Fprintf(f.IOStreams.ErrOut, "%s Removed approval from pull request #%d (%s)\n", f.IOStreams.ColorScheme().SuccessIcon(), pr.ID, pr.Title)
				return nil
			})
		},
	}
}

// NewCmdReady marks a draft PR as ready for review (or back to draft with --undo).
func NewCmdReady(f *cmdutil.Factory) *cobra.Command {
	var undo bool
	cmd := &cobra.Command{
		Use:   "ready [<number> | <branch> | <url>]",
		Short: "Mark a pull request as ready for review",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sel := cmdutil.OptionalArg(args)
			return withPR(cmd.Context(), f, sel, func(ctx context.Context, c *api.Client, repo cmdutil.Repo, pr *bitbucket.PullRequest) error {
				// Bitbucket's PUT requires the title even for partial updates.
				body := map[string]any{"title": pr.Title, "draft": undo}
				if _, err := c.Do(ctx, api.Request{Method: "PUT", Path: prPath(repo, pr.ID, ""), Body: body}, nil); err != nil {
					return err
				}
				if undo {
					fmt.Fprintf(f.IOStreams.ErrOut, "%s Pull request #%d is marked as draft\n", f.IOStreams.ColorScheme().SuccessIcon(), pr.ID)
				} else {
					fmt.Fprintf(f.IOStreams.ErrOut, "%s Pull request #%d is marked as ready for review\n", f.IOStreams.ColorScheme().SuccessIcon(), pr.ID)
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&undo, "undo", false, "Convert a pull request back to a draft")
	return cmd
}

// prFunc is the body of a command operating on one resolved pull request.
type prFunc func(ctx context.Context, c *api.Client, repo cmdutil.Repo, pr *bitbucket.PullRequest) error

// withPR resolves the PR from selector and runs fn.
func withPR(ctx context.Context, f *cmdutil.Factory, selector string, fn prFunc) error {
	repo, err := f.BaseRepo()
	if err != nil {
		return err
	}
	client, err := f.APIClient()
	if err != nil {
		return err
	}
	pr, repo, err := resolvePR(ctx, f, client, repo, selector)
	if err != nil {
		return err
	}
	return fn(ctx, client, repo, pr)
}
