// Package branch implements `bb branch`.
package branch

import (
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/output"
)

// NewCmdBranch returns the `branch` command group.
func NewCmdBranch(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "branch <command>",
		Args:  cobra.ArbitraryArgs,
		RunE:  cmdutil.GroupRunE, // unknown subcommands must fail, not print help with exit 0
		Short: "Manage branches on Bitbucket",
		Example: `  $ bb branch list
  $ bb branch create feat/x --from main
  $ bb branch delete feat/x`,
	}
	cmdutil.EnableRepoOverride(cmd, f)
	cmd.AddCommand(NewCmdList(f), NewCmdCreate(f), NewCmdDelete(f))
	return cmd
}

func refsPath(repo cmdutil.Repo, name string) string {
	p := fmt.Sprintf("/repositories/%s/%s/refs/branches", repo.Workspace, repo.Slug)
	if name != "" {
		p += "/" + url.PathEscape(name)
	}
	return p
}

// NewCmdList returns `branch list`.
func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	var limit int
	var sort, search string
	var buildExporter func() (*output.Exporter, error)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List branches",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			exporter, err := buildExporter()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			q := ""
			if search != "" {
				q = "name ~ " + api.BBQLQuote(search)
			}
			var branches []bitbucket.Branch
			if err := api.Paginate(ctx, client, refsPath(repo, ""), api.ListOptions{Limit: limit, Sort: sort, Query: q, Fields: "values.name,values.target.hash,values.target.date,values.target.message,values.target.author.raw,values.links.html.href,next"}, func(b bitbucket.Branch) error {
				branches = append(branches, b)
				return nil
			}); err != nil {
				return err
			}
			ios := f.IOStreams
			if exporter != nil {
				return exporter.Write(ios, bitbucket.BranchFields.ExportAll(branches, exporter.Fields))
			}
			cs := ios.ColorScheme()
			tp := output.NewTablePrinter(ios)
			now := time.Now()
			for _, b := range branches {
				tp.AddField(b.Name, cs.Cyan)
				if b.Target != nil {
					tp.AddField(b.Target.ShortHash(), cs.Gray)
					msg, _, _ := cutLine(b.Target.Message)
					tp.AddField(msg, nil)
					if tp.IsTTY() {
						tp.AddField(output.TimeAgo(now, b.Target.Date), cs.Gray)
					} else {
						tp.AddField(b.Target.Date.Format(time.RFC3339), nil)
					}
				}
				tp.EndRow()
			}
			return tp.Render()
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "L", 30, "Maximum number of branches to list")
	cmd.Flags().StringVar(&sort, "sort", "-target.date", "Sort field (e.g. name, -target.date)")
	cmd.Flags().StringVar(&search, "search", "", "Filter branches whose name contains this text")
	buildExporter = cmdutil.AddJSONFlags(cmd, bitbucket.BranchFields.Validate, bitbucket.BranchFields.Names())
	return cmd
}

func cutLine(s string) (string, string, bool) {
	for i, r := range s {
		if r == '\n' {
			return s[:i], s[i+1:], true
		}
	}
	return s, "", false
}

// NewCmdCreate returns `branch create`.
func NewCmdCreate(f *cmdutil.Factory) *cobra.Command {
	var from string
	cmd := &cobra.Command{
		Use:   "create <name>",
		Short: "Create a branch on Bitbucket",
		Long:  "Create a branch from a commit hash or another branch (default: the repository's main branch).",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			target := from
			if target == "" {
				if target, err = cmdutil.MainBranch(ctx, client, repo); err != nil {
					return fmt.Errorf("%w; use --from", err)
				}
			}
			var created bitbucket.Branch
			if _, err := client.Do(ctx, api.Request{Method: "POST", Path: refsPath(repo, ""), Body: map[string]any{"name": args[0], "target": map[string]string{"hash": target}}}, &created); err != nil {
				return err
			}
			hash := ""
			if created.Target != nil {
				hash = created.Target.ShortHash()
			}
			fmt.Fprintf(f.IOStreams.ErrOut, "%s Created branch %s at %s\n", f.IOStreams.ColorScheme().SuccessIcon(), created.Name, hash)
			return nil
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "Branch name or commit hash to start from")
	return cmd
}

// NewCmdDelete returns `branch delete`.
func NewCmdDelete(f *cmdutil.Factory) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a branch on Bitbucket",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			if !yes {
				if !f.IOStreams.CanPrompt() {
					return cmdutil.FlagErrorf("--yes required to delete a branch when not running interactively")
				}
				ok, err := f.Prompter.Confirm(fmt.Sprintf("Delete branch %s in %s?", args[0], repo.FullName()), false)
				if err != nil || !ok {
					return cmdutil.PromptError(err)
				}
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			if _, err := client.Do(ctx, api.Request{Method: "DELETE", Path: refsPath(repo, args[0])}, nil); err != nil {
				return err
			}
			fmt.Fprintf(f.IOStreams.ErrOut, "%s Deleted branch %s\n", f.IOStreams.ColorScheme().SuccessIcon(), args[0])
			return nil
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "Skip the confirmation prompt")
	return cmd
}
