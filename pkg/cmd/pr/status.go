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

// NewCmdStatus returns `pr status`.
func NewCmdStatus(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show status of relevant pull requests",
		Long:  "Show the pull request for the current branch, your open pull requests, and pull requests awaiting your review.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(f)
		},
	}
}

func runStatus(f *cmdutil.Factory) error {
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
	me, err := currentUser(ctx, client)
	if err != nil {
		return err
	}

	fmt.Fprintf(ios.Out, "\nRelevant pull requests in %s\n\n", cs.Bold(repo.FullName()))

	fmt.Fprintln(ios.Out, cs.Bold("Current branch"))
	branch := ""
	if f.GitClient != nil {
		if g, err := f.GitClient(); err == nil {
			branch, _ = g.CurrentBranch(ctx)
		}
	}
	if branch == "" {
		fmt.Fprintln(ios.Out, cs.Gray("  Not on a branch"))
	} else if pr, err := findPRForBranch(ctx, client, repo, branch); err == nil {
		printPRLine(f, pr)
	} else {
		fmt.Fprintf(ios.Out, "  There is no pull request associated with %s\n", cs.Cyan("["+branch+"]"))
	}
	fmt.Fprintln(ios.Out)

	fmt.Fprintln(ios.Out, cs.Bold("Created by you"))
	mine, err := listPRs(ctx, client, repo, "author.uuid="+api.BBQLQuote(me.UUID))
	if err != nil {
		return err
	}
	if len(mine) == 0 {
		fmt.Fprintln(ios.Out, cs.Gray("  You have no open pull requests"))
	}
	for i := range mine {
		printPRLine(f, &mine[i])
	}
	fmt.Fprintln(ios.Out)

	fmt.Fprintln(ios.Out, cs.Bold("Requesting a code review from you"))
	review, err := listPRs(ctx, client, repo, "reviewers.uuid="+api.BBQLQuote(me.UUID))
	if err != nil {
		return err
	}
	if len(review) == 0 {
		fmt.Fprintln(ios.Out, cs.Gray("  You have no pull requests to review"))
	}
	for i := range review {
		printPRLine(f, &review[i])
	}
	fmt.Fprintln(ios.Out)
	return nil
}

func listPRs(ctx context.Context, client *api.Client, repo cmdutil.Repo, query string) ([]bitbucket.PullRequest, error) {
	var out []bitbucket.PullRequest
	err := api.Paginate(ctx, client, prPath(repo, 0, ""), api.ListOptions{Limit: 10, Fields: prListFields, Query: query, Sort: "-updated_on", Extra: map[string][]string{"state": {"OPEN"}}}, func(p bitbucket.PullRequest) error {
		out = append(out, p)
		return nil
	})
	return out, err
}

func printPRLine(f *cmdutil.Factory, pr *bitbucket.PullRequest) {
	cs := f.IOStreams.ColorScheme()
	fmt.Fprintf(f.IOStreams.Out, "  %s  %s %s %s\n", stateColor(cs, pr)(fmt.Sprintf("#%d", pr.ID)), output.Truncate(pr.Title, 60), cs.Cyan("["+pr.Source.Branch.Name+"]"), cs.Gray(output.TimeAgo(time.Now(), pr.UpdatedOn)))
}
