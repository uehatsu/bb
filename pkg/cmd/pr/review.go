package pr

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
)

// NewCmdReview returns `pr review`.
func NewCmdReview(f *cmdutil.Factory) *cobra.Command {
	var approve, requestChanges, comment bool
	var body, bodyFile string
	cmd := &cobra.Command{
		Use:   "review [<number> | <branch> | <url>]",
		Short: "Add a review to a pull request",
		Long: `Approve, request changes on, or comment on a pull request.

Bitbucket records approvals and change requests separately from comments,
so a body given with --approve or --request-changes is posted as a comment
in addition to the review action.`,
		Example: `  $ bb pr review 42 --approve
  $ bb pr review --request-changes -b "Please add tests"
  $ bb pr review --comment -b "Looks reasonable"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			n := 0
			for _, b := range []bool{approve, requestChanges, comment} {
				if b {
					n++
				}
			}
			if n != 1 {
				return cmdutil.FlagErrorf("specify exactly one of --approve, --request-changes, or --comment")
			}
			if bodyFile != "" {
				b, err := readBodyFile(bodyFile, f.IOStreams.In)
				if err != nil {
					return err
				}
				body = b
			}
			if comment && strings.TrimSpace(body) == "" {
				return cmdutil.FlagErrorf("--comment requires a body (--body or --body-file)")
			}
			sel := cmdutil.OptionalArg(args)
			return withPR(cmd.Context(), f, sel, func(ctx context.Context, c *api.Client, repo cmdutil.Repo, pr *bitbucket.PullRequest) error {
				cs := f.IOStreams.ColorScheme()
				prID := pr.ID
				if strings.TrimSpace(body) != "" {
					if _, err := postComment(ctx, c, repo, prID, body); err != nil {
						return err
					}
				}
				switch {
				case approve:
					if _, err := c.Do(ctx, api.Request{Method: "POST", Path: prPath(repo, prID, "approve")}, nil); err != nil {
						return err
					}
					fmt.Fprintf(f.IOStreams.ErrOut, "%s Approved pull request #%d\n", cs.SuccessIcon(), prID)
				case requestChanges:
					if _, err := c.Do(ctx, api.Request{Method: "POST", Path: prPath(repo, prID, "request-changes")}, nil); err != nil {
						return err
					}
					fmt.Fprintf(f.IOStreams.ErrOut, "%s Requested changes on pull request #%d\n", cs.Yellow("✓"), prID)
				default:
					fmt.Fprintf(f.IOStreams.ErrOut, "%s Commented on pull request #%d\n", cs.SuccessIcon(), prID)
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVarP(&approve, "approve", "a", false, "Approve the pull request")
	cmd.Flags().BoolVarP(&requestChanges, "request-changes", "r", false, "Request changes on the pull request")
	cmd.Flags().BoolVarP(&comment, "comment", "c", false, "Comment on the pull request")
	cmd.Flags().StringVarP(&body, "body", "b", "", "Body of the review comment")
	cmd.Flags().StringVarP(&bodyFile, "body-file", "F", "", "Read body text from file (use \"-\" for stdin)")
	return cmd
}

// NewCmdComment returns `pr comment`.
func NewCmdComment(f *cmdutil.Factory) *cobra.Command {
	var body, bodyFile, path string
	var line int
	var editor bool
	cmd := &cobra.Command{
		Use:   "comment [<number> | <branch> | <url>]",
		Short: "Add a comment to a pull request",
		Example: `  $ bb pr comment 42 --body "Thanks!"
  $ bb pr comment --body-file note.md
  $ bb pr comment 42 --path src/main.go --line 10 -b "typo"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if bodyFile != "" {
				b, err := readBodyFile(bodyFile, f.IOStreams.In)
				if err != nil {
					return err
				}
				body = b
			}
			if body == "" {
				if !editor && !f.IOStreams.CanPrompt() {
					return cmdutil.FlagErrorf("--body or --body-file is required when not running interactively")
				}
				v, err := f.Prompter.Editor("Comment", "")
				if err != nil {
					return cmdutil.PromptError(err)
				}
				body = v
			}
			if strings.TrimSpace(body) == "" {
				return cmdutil.FlagErrorf("comment body must not be empty")
			}
			if line > 0 && path == "" {
				return cmdutil.FlagErrorf("--line requires --path")
			}
			sel := cmdutil.OptionalArg(args)
			return withPR(cmd.Context(), f, sel, func(ctx context.Context, c *api.Client, repo cmdutil.Repo, pr *bitbucket.PullRequest) error {
				prID := pr.ID
				var created *bitbucket.Comment
				var err error
				if path != "" {
					created, err = postInlineComment(ctx, c, repo, prID, body, path, line)
				} else {
					created, err = postComment(ctx, c, repo, prID, body)
				}
				if err != nil {
					return err
				}
				fmt.Fprintln(f.IOStreams.Out, created.Links.HTML())
				return nil
			})
		},
	}
	cmd.Flags().StringVarP(&body, "body", "b", "", "The comment body text")
	cmd.Flags().StringVarP(&bodyFile, "body-file", "F", "", "Read body text from file (use \"-\" for stdin)")
	cmd.Flags().BoolVarP(&editor, "editor", "e", false, "Open an editor to write the comment")
	cmd.Flags().StringVar(&path, "path", "", "File path for an inline comment")
	cmd.Flags().IntVar(&line, "line", 0, "Line number (in the new file) for an inline comment")
	return cmd
}

func postComment(ctx context.Context, c *api.Client, repo cmdutil.Repo, prID int, body string) (*bitbucket.Comment, error) {
	var out bitbucket.Comment
	payload := map[string]any{"content": map[string]string{"raw": body}}
	if _, err := c.Do(ctx, api.Request{Method: "POST", Path: prPath(repo, prID, "comments"), Body: payload}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func postInlineComment(ctx context.Context, c *api.Client, repo cmdutil.Repo, prID int, body, path string, line int) (*bitbucket.Comment, error) {
	var out bitbucket.Comment
	inline := map[string]any{"path": path}
	if line > 0 {
		inline["to"] = line
	}
	payload := map[string]any{"content": map[string]string{"raw": body}, "inline": inline}
	if _, err := c.Do(ctx, api.Request{Method: "POST", Path: prPath(repo, prID, "comments"), Body: payload}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
