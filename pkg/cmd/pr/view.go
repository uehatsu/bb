package pr

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/browser"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/output"
)

// NewCmdView returns `pr view`.
func NewCmdView(f *cmdutil.Factory) *cobra.Command {
	var web, comments bool
	var buildExporter func() (*output.Exporter, error)
	cmd := &cobra.Command{
		Use:   "view [<number> | <branch> | <url>]",
		Short: "View a pull request",
		Long: `Display the title, body, and other information about a pull request.

Without an argument, the pull request for the current branch is shown.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sel := ""
			if len(args) > 0 {
				sel = args[0]
			}
			exporter, err := buildExporter()
			if err != nil {
				return err
			}
			return runView(cmd.Context(), f, sel, web, comments, exporter)
		},
	}
	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open the pull request in the browser")
	cmd.Flags().BoolVarP(&comments, "comments", "c", false, "View pull request comments")
	buildExporter = cmdutil.AddJSONFlags(cmd, bitbucket.PullRequestFields.Validate, bitbucket.PullRequestFields.Names())
	return cmd
}

func openInBrowser(f *cmdutil.Factory, u string) error {
	if f.IOStreams.IsStdoutTTY() {
		fmt.Fprintf(f.IOStreams.ErrOut, "Opening %s in your browser.\n", u)
	}
	configured := ""
	if cfg, err := f.Config(); err == nil {
		configured, _ = cfg.Get("browser")
	}
	return browser.New(configured).Browse(u)
}

func runView(ctx context.Context, f *cmdutil.Factory, selector string, web, withComments bool, exporter *output.Exporter) error {
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
	if web {
		return openInBrowser(f, pr.Links.HTML())
	}
	ios := f.IOStreams
	if exporter != nil {
		return exporter.Write(ios, bitbucket.PullRequestFields.Export(*pr, exporter.Fields))
	}
	var comments []bitbucket.Comment
	if withComments {
		if err := api.Paginate(ctx, client, prPath(repo, pr.ID, "comments"), api.ListOptions{Sort: "created_on", Fields: "values.id,values.content.raw,values.user.nickname,values.user.display_name,values.deleted,values.created_on,values.inline,values.links.html.href,next"}, func(c bitbucket.Comment) error {
			if !c.Deleted {
				comments = append(comments, c)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	if err := ios.StartPager(); err == nil {
		defer ios.StopPager()
	}
	printPR(f, pr, comments)
	return nil
}

func printPR(f *cmdutil.Factory, pr *bitbucket.PullRequest, comments []bitbucket.Comment) {
	ios := f.IOStreams
	cs := ios.ColorScheme()
	out := ios.Out
	if !ios.IsStdoutTTY() {
		fmt.Fprintf(out, "title:\t%s\nstate:\t%s\nauthor:\t%s\nnumber:\t%d\nhead:\t%s\nbase:\t%s\nurl:\t%s\n--\n%s\n",
			pr.Title, prStateLabel(pr), pr.Author.Name(), pr.ID, pr.Source.Branch.Name, pr.Destination.Branch.Name, pr.Links.HTML(), pr.Description)
		for _, c := range comments {
			fmt.Fprintf(out, "--\ncomment by %s (%s):\n%s\n", c.User.Name(), c.CreatedOn.Format(time.RFC3339), c.Content.Raw)
		}
		return
	}
	fmt.Fprintf(out, "%s %s\n", cs.Bold(pr.Title), cs.Gray(fmt.Sprintf("%s#%d", repoName(pr), pr.ID)))
	fmt.Fprintf(out, "%s • %s wants to merge %s into %s\n",
		stateColor(cs, pr)(prStateLabel(pr)), pr.Author.Name(), cs.Cyan(pr.Source.Branch.Name), cs.Cyan(pr.Destination.Branch.Name))
	var meta []string
	if pr.CommentCount > 0 {
		meta = append(meta, fmt.Sprintf("%d comments", pr.CommentCount))
	}
	if pr.TaskCount > 0 {
		meta = append(meta, fmt.Sprintf("%d open tasks", pr.TaskCount))
	}
	var approved, changes []string
	for _, p := range pr.Participants {
		switch p.State {
		case "approved":
			approved = append(approved, p.User.Name())
		case "changes_requested":
			changes = append(changes, p.User.Name())
		}
	}
	if len(approved) > 0 {
		meta = append(meta, cs.Green("approved by "+strings.Join(approved, ", ")))
	}
	if len(changes) > 0 {
		meta = append(meta, cs.Red("changes requested by "+strings.Join(changes, ", ")))
	}
	if len(pr.Reviewers) > 0 {
		names := make([]string, len(pr.Reviewers))
		for i, r := range pr.Reviewers {
			names[i] = r.Name()
		}
		meta = append(meta, "reviewers: "+strings.Join(names, ", "))
	}
	if len(meta) > 0 {
		fmt.Fprintln(out, cs.Gray(strings.Join(meta, " • ")))
	}
	fmt.Fprintln(out)
	if strings.TrimSpace(pr.Description) != "" {
		fmt.Fprintln(out, strings.TrimRight(pr.Description, "\n"))
	} else {
		fmt.Fprintln(out, cs.Gray("No description provided"))
	}
	fmt.Fprintln(out)
	for _, c := range comments {
		fmt.Fprintf(out, "%s %s\n", cs.Bold(c.User.Name()), cs.Gray("commented "+output.TimeAgo(time.Now(), c.CreatedOn)))
		if c.Inline != nil {
			line := ""
			if c.Inline.To != nil {
				line = fmt.Sprintf(":%d", *c.Inline.To)
			}
			fmt.Fprintf(out, "%s\n", cs.Gray("  on "+c.Inline.Path+line))
		}
		fmt.Fprintln(out, indent(strings.TrimRight(c.Content.Raw, "\n"), "  "))
		fmt.Fprintln(out)
	}
	fmt.Fprintln(out, cs.Gray("View this pull request on Bitbucket: "+pr.Links.HTML()))
}

func repoName(pr *bitbucket.PullRequest) string {
	if pr.Destination.Repository != nil {
		return pr.Destination.Repository.FullName
	}
	return ""
}

func indent(s, prefix string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}
