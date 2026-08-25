package pr

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/output"
)

// NewCmdChecks returns `pr checks`, showing commit statuses (builds).
func NewCmdChecks(f *cmdutil.Factory) *cobra.Command {
	var watch bool
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "checks [<number> | <branch> | <url>]",
		Short: "Show build statuses for a pull request",
		Long: `List the commit statuses (pipelines and other builds) reported on a pull
request. Exit code 8 indicates checks are still in progress, and 1 that some
failed.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sel := cmdutil.OptionalArg(args)
			return withPR(cmd.Context(), f, sel, func(ctx context.Context, c *api.Client, repo cmdutil.Repo, pr *bitbucket.PullRequest) error {
				var pending, failed int
				report := func(ctx context.Context) (bool, error) {
					statuses, err := fetchStatuses(ctx, c, repo, pr.ID)
					if err != nil {
						return false, err
					}
					pending, failed = printChecks(f, statuses)
					return pending == 0, nil
				}
				if watch {
					if err := api.Poll(ctx, api.PollOptions{Initial: interval, Max: interval, Factor: 1}, report); err != nil {
						return err
					}
				} else if _, err := report(ctx); err != nil {
					return err
				}
				if failed > 0 {
					return cmdutil.ErrSilent
				}
				if pending > 0 {
					return cmdutil.ErrPending
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&watch, "watch", false, "Watch checks until they finish")
	cmd.Flags().DurationVarP(&interval, "interval", "i", 10*time.Second, "Refresh interval when watching")
	return cmd
}

func fetchStatuses(ctx context.Context, c *api.Client, repo cmdutil.Repo, prID int) ([]bitbucket.CommitStatus, error) {
	var out []bitbucket.CommitStatus
	err := api.Paginate(ctx, c, prPath(repo, prID, "statuses"), api.ListOptions{Fields: "values.key,values.name,values.state,values.description,values.url,next"}, func(s bitbucket.CommitStatus) error {
		out = append(out, s)
		return nil
	})
	return out, err
}

func printChecks(f *cmdutil.Factory, statuses []bitbucket.CommitStatus) (pending, failed int) {
	ios := f.IOStreams
	cs := ios.ColorScheme()
	if len(statuses) == 0 {
		fmt.Fprintln(ios.ErrOut, "No checks reported on this pull request")
		return 0, 0
	}
	tp := output.NewTablePrinter(ios)
	for _, s := range statuses {
		var icon string
		var color func(string) string
		switch s.State {
		case "SUCCESSFUL":
			icon, color = "✓", cs.Green
		case "FAILED", "STOPPED":
			icon, color = "✗", cs.Red
			failed++
		default:
			icon, color = "*", cs.Yellow
			pending++
		}
		name := s.Name
		if name == "" {
			name = s.Key
		}
		if tp.IsTTY() {
			tp.AddField(icon, color)
		} else {
			tp.AddField(s.State, nil)
		}
		tp.AddField(name, nil)
		tp.AddField(s.Description, cs.Gray)
		tp.AddField(s.URL, cs.Gray)
		tp.EndRow()
	}
	_ = tp.Render()
	if tp.IsTTY() {
		fmt.Fprintf(ios.Out, "\n%d successful, %d failed, %d pending\n", len(statuses)-failed-pending, failed, pending)
	}
	return pending, failed
}
