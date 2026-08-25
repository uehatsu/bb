package pr

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/output"
)

// ListOptions for `pr list`.
type ListOptions struct {
	State  string
	Author string
	Search string
	Base   string
	Head   string
	Limit  int
	Sort   string
	Web    bool
}

// NewCmdList returns `pr list`.
func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	opts := &ListOptions{}
	var buildExporter func() (*output.Exporter, error)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List pull requests in a repository",
		Example: `  $ bb pr list
  $ bb pr list --state merged --limit 50
  $ bb pr list --author @me
  $ bb pr list --search 'title ~ "fix"' --json id,title`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			exporter, err := buildExporter()
			if err != nil {
				return err
			}
			return runList(cmd.Context(), f, opts, exporter)
		},
	}
	cmd.Flags().StringVarP(&opts.State, "state", "s", "open", "Filter by state: {open|merged|declined|superseded|all}")
	cmd.Flags().StringVarP(&opts.Author, "author", "A", "", "Filter by author nickname (@me for yourself)")
	cmd.Flags().StringVarP(&opts.Search, "search", "S", "", "Additional BBQL filter expression")
	cmd.Flags().StringVarP(&opts.Base, "base", "B", "", "Filter by destination branch")
	cmd.Flags().StringVarP(&opts.Head, "head", "H", "", "Filter by source branch")
	cmd.Flags().IntVarP(&opts.Limit, "limit", "L", 30, "Maximum number of items to fetch")
	cmd.Flags().StringVar(&opts.Sort, "sort", "-updated_on", "Sort field, prefix with - for descending")
	cmd.Flags().BoolVarP(&opts.Web, "web", "w", false, "Open the pull request list in the browser")
	buildExporter = cmdutil.AddJSONFlags(cmd, bitbucket.PullRequestFields.Validate, bitbucket.PullRequestFields.Names())
	return cmd
}

func stateParams(state string) ([]string, error) {
	switch strings.ToLower(state) {
	case "open":
		return []string{"OPEN"}, nil
	case "merged":
		return []string{"MERGED"}, nil
	case "declined", "closed":
		return []string{"DECLINED"}, nil
	case "superseded":
		return []string{"SUPERSEDED"}, nil
	case "all":
		return []string{"OPEN", "MERGED", "DECLINED", "SUPERSEDED"}, nil
	}
	return nil, cmdutil.FlagErrorf("invalid --state %q (open|merged|declined|superseded|all)", state)
}

func runList(ctx context.Context, f *cmdutil.Factory, opts *ListOptions, exporter *output.Exporter) error {
	repo, err := f.BaseRepo()
	if err != nil {
		return err
	}
	if opts.Web {
		return openInBrowser(f, "https://bitbucket.org/"+repo.FullName()+"/pull-requests/")
	}
	states, err := stateParams(opts.State)
	if err != nil {
		return err
	}
	client, err := f.APIClient()
	if err != nil {
		return err
	}
	var clauses []string
	if opts.Author != "" {
		if opts.Author == "@me" {
			u, err := currentUser(ctx, client)
			if err != nil {
				return err
			}
			clauses = append(clauses, "author.uuid="+api.BBQLQuote(u.UUID))
		} else {
			clauses = append(clauses, "author.nickname="+api.BBQLQuote(opts.Author))
		}
	}
	if opts.Base != "" {
		clauses = append(clauses, "destination.branch.name="+api.BBQLQuote(opts.Base))
	}
	if opts.Head != "" {
		clauses = append(clauses, "source.branch.name="+api.BBQLQuote(opts.Head))
	}
	clauses = append(clauses, opts.Search)

	lo := api.ListOptions{
		Limit:  opts.Limit,
		Fields: prListFields,
		Query:  api.BBQLAnd(clauses...),
		Sort:   opts.Sort,
		Extra:  map[string][]string{"state": states},
	}
	var prs []bitbucket.PullRequest
	if err := api.Paginate(ctx, client, prPath(repo, 0, ""), lo, func(p bitbucket.PullRequest) error {
		prs = append(prs, p)
		return nil
	}); err != nil {
		return err
	}

	ios := f.IOStreams
	if exporter != nil {
		return exporter.Write(ios, bitbucket.PullRequestFields.ExportAll(prs, exporter.Fields))
	}
	if len(prs) == 0 {
		fmt.Fprintf(ios.ErrOut, "No pull requests match your search in %s\n", repo.FullName())
		return nil
	}
	cs := ios.ColorScheme()
	tp := output.NewTablePrinter(ios)
	if tp.IsTTY() {
		fmt.Fprintf(ios.Out, "\nShowing %d pull requests in %s\n\n", len(prs), repo.FullName())
	}
	now := time.Now()
	for i := range prs {
		p := &prs[i]
		tp.AddField(fmt.Sprintf("#%d", p.ID), stateColor(cs, p))
		tp.AddField(p.Title, nil)
		tp.AddField(p.Source.Branch.Name, cs.Cyan)
		if tp.IsTTY() {
			tp.AddField(prStateLabel(p), stateColor(cs, p))
			tp.AddField(output.TimeAgo(now, p.UpdatedOn), cs.Gray)
		} else {
			tp.AddField(prStateLabel(p), nil)
			tp.AddField(p.UpdatedOn.Format(time.RFC3339), nil)
		}
		tp.EndRow()
	}
	return tp.Render()
}
