package repo

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

// ListOptions for `repo list`.
type ListOptions struct {
	Workspace  string
	Role       string
	Limit      int
	Language   string
	Visibility string
	Sort       string
	Query      string
}

// NewCmdList returns `repo list`.
func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	opts := &ListOptions{}
	var buildExporter func() (*output.Exporter, error)
	cmd := &cobra.Command{
		Use:   "list [<workspace>]",
		Short: "List repositories in a workspace",
		Long: `List repositories in a workspace.

When no workspace is given, the configured default workspace is used; if none
is configured, every workspace the user belongs to is listed in turn.

Note: Bitbucket removed the cross-workspace repository listing API in 2026, so
--role member is no longer available; use contributor, admin, or owner.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Workspace = args[0]
			}
			switch opts.Role {
			case "", "contributor", "admin", "owner":
			case "member":
				return cmdutil.FlagErrorf("--role member was removed by Bitbucket (CHANGE-2770); use contributor, admin, or owner")
			default:
				return cmdutil.FlagErrorf("invalid --role %q (contributor|admin|owner)", opts.Role)
			}
			switch opts.Visibility {
			case "", "public", "private":
			default:
				return cmdutil.FlagErrorf("invalid --visibility %q (public|private)", opts.Visibility)
			}
			exporter, err := buildExporter()
			if err != nil {
				return err
			}
			return runList(f, opts, exporter)
		},
	}
	cmd.Flags().StringVar(&opts.Role, "role", "", "Filter by the user's role: {contributor|admin|owner}")
	cmd.Flags().IntVarP(&opts.Limit, "limit", "L", 30, "Maximum number of repositories to list")
	cmd.Flags().StringVarP(&opts.Language, "language", "l", "", "Filter by primary coding language")
	cmd.Flags().StringVar(&opts.Visibility, "visibility", "", "Filter by visibility: {public|private}")
	cmd.Flags().StringVar(&opts.Sort, "sort", "-updated_on", "Sort field, prefix with - for descending")
	cmd.Flags().StringVar(&opts.Query, "search", "", "Additional BBQL filter, e.g. 'name ~ \"api\"'")
	buildExporter = cmdutil.AddJSONFlags(cmd, bitbucket.RepositoryFields.Validate, bitbucket.RepositoryFields.Names())
	return cmd
}

func runList(f *cmdutil.Factory, opts *ListOptions, exporter *output.Exporter) error {
	ctx := context.Background()
	client, err := f.APIClient()
	if err != nil {
		return err
	}
	workspaces := []string{opts.Workspace}
	if opts.Workspace == "" {
		if ws := cmdutil.DefaultWorkspace(f); ws != "" {
			workspaces = []string{ws}
		} else {
			workspaces, err = userWorkspaces(ctx, client)
			if err != nil {
				return err
			}
			if len(workspaces) == 0 {
				return fmt.Errorf("you are not a member of any workspace")
			}
		}
	}

	var clauses []string
	if opts.Language != "" {
		clauses = append(clauses, "language="+api.BBQLQuote(opts.Language))
	}
	if opts.Visibility != "" {
		clauses = append(clauses, fmt.Sprintf("is_private=%t", opts.Visibility == "private"))
	}
	clauses = append(clauses, opts.Query)
	query := api.BBQLAnd(clauses...)

	var repos []bitbucket.Repository
	remaining := opts.Limit
	for _, ws := range workspaces {
		if opts.Limit > 0 && remaining <= 0 {
			break
		}
		lo := api.ListOptions{Limit: remaining, Fields: repoListFields, Query: query, Sort: opts.Sort}
		if opts.Role != "" {
			lo.Extra = map[string][]string{"role": {opts.Role}}
		}
		err := api.Paginate(ctx, client, "/repositories/"+ws, lo, func(r bitbucket.Repository) error {
			repos = append(repos, r)
			return nil
		})
		if err != nil {
			return err
		}
		if opts.Limit > 0 {
			remaining = opts.Limit - len(repos)
		}
	}

	ios := f.IOStreams
	if exporter != nil {
		return exporter.Write(ios, bitbucket.RepositoryFields.ExportAll(repos, exporter.Fields))
	}
	if len(repos) == 0 {
		fmt.Fprintln(ios.ErrOut, "No repositories found")
		return nil
	}
	cs := ios.ColorScheme()
	tp := output.NewTablePrinter(ios)
	if tp.IsTTY() {
		fmt.Fprintf(ios.Out, "\nShowing %d repositories\n\n", len(repos))
	}
	now := time.Now()
	for _, r := range repos {
		tp.AddField(r.FullName, cs.Bold)
		tp.AddField(r.Description, nil)
		tp.AddField(visibility(&r), cs.Gray)
		if tp.IsTTY() {
			tp.AddField(output.TimeAgo(now, r.UpdatedOn), cs.Gray)
		} else {
			tp.AddField(r.UpdatedOn.Format(time.RFC3339), nil)
		}
		tp.EndRow()
	}
	return tp.Render()
}

func userWorkspaces(ctx context.Context, client *api.Client) ([]string, error) {
	var out []string
	err := api.Paginate(ctx, client, "/user/workspaces", api.ListOptions{Fields: "values.workspace.slug,next"}, func(m bitbucket.WorkspaceMembership) error {
		out = append(out, m.Workspace.Slug)
		return nil
	})
	return out, err
}
