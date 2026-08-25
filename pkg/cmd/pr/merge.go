package pr

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
)

// MergeOptions for `pr merge`.
type MergeOptions struct {
	Selector     string
	MergeCommit  bool
	Squash       bool
	Rebase       bool
	Strategy     string
	DeleteBranch bool
	Message      string
	Timeout      time.Duration
	Yes          bool
}

var mergeStrategies = map[string]bool{
	"merge_commit": true, "squash": true, "fast_forward": true,
	"squash_fast_forward": true, "rebase_fast_forward": true, "rebase_merge": true,
}

// NewCmdMerge returns `pr merge`.
func NewCmdMerge(f *cmdutil.Factory) *cobra.Command {
	opts := &MergeOptions{}
	cmd := &cobra.Command{
		Use:   "merge [<number> | <branch> | <url>]",
		Short: "Merge a pull request",
		Long: `Merge a pull request on Bitbucket.

The strategy defaults to the merge_strategy config value (merge_commit).
--merge, --squash and --rebase mirror GitHub CLI; Bitbucket's full set of
strategies is available through --strategy: merge_commit, squash,
fast_forward, squash_fast_forward, rebase_fast_forward, rebase_merge.

Bitbucket may perform the merge asynchronously; in that case bb polls the
merge task until it completes (see --timeout).`,
		Example: `  $ bb pr merge 42 --squash --delete-branch
  $ bb pr merge --strategy rebase_fast_forward -m "Rebase and merge"`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Selector = args[0]
			}
			n := 0
			for _, b := range []bool{opts.MergeCommit, opts.Squash, opts.Rebase, opts.Strategy != ""} {
				if b {
					n++
				}
			}
			if n > 1 {
				return cmdutil.FlagErrorf("only one of --merge, --squash, --rebase, or --strategy may be given")
			}
			if opts.Strategy != "" && !mergeStrategies[opts.Strategy] {
				return cmdutil.FlagErrorf("invalid --strategy %q", opts.Strategy)
			}
			return runMerge(f, opts)
		},
	}
	cmd.Flags().BoolVarP(&opts.MergeCommit, "merge", "m", false, "Merge with a merge commit")
	cmd.Flags().BoolVarP(&opts.Squash, "squash", "s", false, "Squash the commits into one commit and merge")
	cmd.Flags().BoolVarP(&opts.Rebase, "rebase", "r", false, "Rebase the commits onto the base branch and merge (rebase_merge)")
	cmd.Flags().StringVar(&opts.Strategy, "strategy", "", "Bitbucket merge strategy (see help)")
	cmd.Flags().BoolVarP(&opts.DeleteBranch, "delete-branch", "d", false, "Delete the source branch after merge")
	cmd.Flags().StringVar(&opts.Message, "message", "", "Commit message for the merge commit")
	cmd.Flags().DurationVar(&opts.Timeout, "timeout", 5*time.Minute, "How long to wait for an asynchronous merge")
	cmd.Flags().BoolVarP(&opts.Yes, "yes", "y", false, "Skip the confirmation prompt")
	// gh uses -m for --body on some commands; here --message has no shorthand conflict
	cmd.Flags().Lookup("message").Shorthand = ""
	return cmd
}

func runMerge(f *cmdutil.Factory, opts *MergeOptions) error {
	ctx := context.Background()
	ios := f.IOStreams
	cs := ios.ColorScheme()
	repo, err := f.BaseRepo()
	if err != nil {
		return err
	}
	client, err := f.APIClient()
	if err != nil {
		return err
	}
	pr, err := resolvePR(ctx, f, client, repo, opts.Selector)
	if err != nil {
		return err
	}
	if pr.State != "OPEN" {
		return fmt.Errorf("pull request #%d is %s and cannot be merged", pr.ID, pr.State)
	}

	strategy := opts.Strategy
	switch {
	case opts.MergeCommit:
		strategy = "merge_commit"
	case opts.Squash:
		strategy = "squash"
	case opts.Rebase:
		strategy = "rebase_merge"
	}
	if strategy == "" {
		if cfg, err := f.Config(); err == nil {
			strategy, _ = cfg.Get("merge_strategy")
		}
		if strategy == "" {
			strategy = "merge_commit"
		}
		if ios.CanPrompt() && !opts.Yes {
			choice, err := f.Prompter.Select("What merge strategy would you like to use?", []string{"merge_commit", "squash", "fast_forward", "squash_fast_forward", "rebase_fast_forward", "rebase_merge"})
			if err != nil {
				return cmdutil.ErrCancel
			}
			strategy = choice
		}
	}
	if ios.CanPrompt() && !opts.Yes {
		ok, err := f.Prompter.Confirm(fmt.Sprintf("Merge pull request #%d (%s) into %s using %s?", pr.ID, pr.Title, pr.Destination.Branch.Name, strategy), true)
		if err != nil || !ok {
			return cmdutil.ErrCancel
		}
	}

	body := map[string]any{
		"merge_strategy":      strategy,
		"close_source_branch": opts.DeleteBranch || pr.CloseSourceBranch,
	}
	if opts.Message != "" {
		body["message"] = opts.Message
	}
	resp, err := client.DoRaw(ctx, api.Request{Method: "POST", Path: prPath(repo, pr.ID, "merge"), Body: body, Query: map[string][]string{"async": {"true"}}})
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// merged synchronously
	case http.StatusAccepted:
		loc := resp.Header.Get("Location")
		if loc == "" {
			return errors.New("merge accepted but no task status URL was returned")
		}
		fmt.Fprintf(ios.ErrOut, "Merge in progress...\n")
		if err := waitForMerge(ctx, client, loc, opts.Timeout); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unexpected response %s from merge", resp.Status)
	}

	merged, err := fetchPR(ctx, client, repo, pr.ID)
	if err != nil {
		return err
	}
	if merged.State != "MERGED" {
		return fmt.Errorf("merge finished but pull request state is %s", merged.State)
	}
	fmt.Fprintf(ios.ErrOut, "%s Merged pull request #%d (%s)\n", cs.Magenta("✓"), pr.ID, pr.Title)
	if body["close_source_branch"] == true {
		fmt.Fprintf(ios.ErrOut, "%s Deleted branch %s\n", cs.Red("✓"), pr.Source.Branch.Name)
	}
	return nil
}

// mergeTaskStatus is the polling response for asynchronous merges.
type mergeTaskStatus struct {
	TaskStatus string `json:"task_status"` // PENDING | SUCCESS | FAILED (observed values)
	Error      *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
	MergeResult *bitbucket.PullRequest `json:"merge_result,omitempty"`
}

func waitForMerge(ctx context.Context, client *api.Client, statusURL string, timeout time.Duration) error {
	return api.Poll(ctx, api.PollOptions{Initial: 2 * time.Second, Max: 15 * time.Second, Timeout: timeout}, func(ctx context.Context) (bool, error) {
		var st mergeTaskStatus
		if _, err := client.Do(ctx, api.Request{Path: statusURL}, &st); err != nil {
			return false, err
		}
		switch st.TaskStatus {
		case "SUCCESS", "COMPLETED", "SUCCESSFUL":
			return true, nil
		case "FAILED", "ERROR":
			if st.Error != nil {
				return false, fmt.Errorf("merge failed: %s", st.Error.Message)
			}
			return false, errors.New("merge failed")
		}
		return false, nil
	})
}
