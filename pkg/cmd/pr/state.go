package pr

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
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
			sel := ""
			if len(args) > 0 {
				sel = args[0]
			}
			return withPR(f, sel, func(ctx context.Context, c *api.Client, repo cmdutil.Repo, prID int, title string, state string) error {
				if state != "OPEN" {
					return fmt.Errorf("pull request #%d is already %s", prID, state)
				}
				if _, err := c.Do(ctx, api.Request{Method: "POST", Path: prPath(repo, prID, "decline")}, nil); err != nil {
					return err
				}
				fmt.Fprintf(f.IOStreams.ErrOut, "%s Declined pull request #%d (%s)\n", f.IOStreams.ColorScheme().Red("✓"), prID, title)
				if deleteBranch {
					pr, err := fetchPR(ctx, c, repo, prID)
					if err != nil {
						return err
					}
					branch := pr.Source.Branch.Name
					if _, err := c.Do(ctx, api.Request{Method: "DELETE", Path: fmt.Sprintf("/repositories/%s/%s/refs/branches/%s", repo.Workspace, repo.Slug, branch)}, nil); err != nil {
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
		Use:   "reopen [<number>]",
		Short: "Reopen a pull request (not supported by Bitbucket)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return errors.New("bitbucket Cloud cannot reopen a declined pull request. Create a new pull request from the same branch instead: `bb pr create`")
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
			sel := ""
			if len(args) > 0 {
				sel = args[0]
			}
			return withPR(f, sel, func(ctx context.Context, c *api.Client, repo cmdutil.Repo, prID int, title, state string) error {
				if _, err := c.Do(ctx, api.Request{Method: "POST", Path: prPath(repo, prID, "approve")}, nil); err != nil {
					return err
				}
				fmt.Fprintf(f.IOStreams.ErrOut, "%s Approved pull request #%d (%s)\n", f.IOStreams.ColorScheme().SuccessIcon(), prID, title)
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
			sel := ""
			if len(args) > 0 {
				sel = args[0]
			}
			return withPR(f, sel, func(ctx context.Context, c *api.Client, repo cmdutil.Repo, prID int, title, state string) error {
				if _, err := c.Do(ctx, api.Request{Method: "DELETE", Path: prPath(repo, prID, "approve")}, nil); err != nil {
					return err
				}
				fmt.Fprintf(f.IOStreams.ErrOut, "%s Removed approval from pull request #%d (%s)\n", f.IOStreams.ColorScheme().SuccessIcon(), prID, title)
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
			sel := ""
			if len(args) > 0 {
				sel = args[0]
			}
			return withPR(f, sel, func(ctx context.Context, c *api.Client, repo cmdutil.Repo, prID int, title, state string) error {
				if _, err := c.Do(ctx, api.Request{Method: "PUT", Path: prPath(repo, prID, ""), Body: map[string]any{"draft": undo}}, nil); err != nil {
					return err
				}
				if undo {
					fmt.Fprintf(f.IOStreams.ErrOut, "%s Pull request #%d is marked as draft\n", f.IOStreams.ColorScheme().SuccessIcon(), prID)
				} else {
					fmt.Fprintf(f.IOStreams.ErrOut, "%s Pull request #%d is marked as ready for review\n", f.IOStreams.ColorScheme().SuccessIcon(), prID)
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&undo, "undo", false, "Convert a pull request back to a draft")
	return cmd
}

// withPR resolves the PR and runs fn with the essentials.
func withPR(f *cmdutil.Factory, selector string, fn func(ctx context.Context, c *api.Client, repo cmdutil.Repo, prID int, title, state string) error) error {
	ctx := context.Background()
	repo, err := f.BaseRepo()
	if err != nil {
		return err
	}
	client, err := f.APIClient()
	if err != nil {
		return err
	}
	pr, err := resolvePR(ctx, f, client, repo, selector)
	if err != nil {
		return err
	}
	return fn(ctx, client, repo, pr.ID, pr.Title, pr.State)
}
