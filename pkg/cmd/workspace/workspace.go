// Package workspace implements `bb workspace`.
package workspace

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/gitctx"
	"github.com/uehatsu/bb/internal/output"
)

// NewCmdWorkspace returns the `workspace` command group.
func NewCmdWorkspace(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "workspace <command>",
		Args:    cobra.NoArgs, // unknown subcommands must fail, not print help with exit 0
		Aliases: []string{"ws", "org"},
		Short:   "Manage workspaces",
	}
	cmd.AddCommand(NewCmdList(f), NewCmdView(f), NewCmdMembers(f))
	return cmd
}

// NewCmdList returns `workspace list`.
func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	var limit int
	var buildExporter func() (*output.Exporter, error)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List workspaces you belong to",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			exporter, err := buildExporter()
			if err != nil {
				return err
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			var members []bitbucket.WorkspaceMembership
			if err := api.Paginate(cmd.Context(), client, "/user/workspaces", api.ListOptions{Limit: limit, Fields: "values.permission,values.workspace.slug,values.workspace.name,values.workspace.uuid,values.workspace.is_private,values.workspace.is_personal,values.workspace.created_on,values.workspace.links.html.href,next"}, func(m bitbucket.WorkspaceMembership) error {
				members = append(members, m)
				return nil
			}); err != nil {
				return err
			}
			ios := f.IOStreams
			if exporter != nil {
				ws := make([]bitbucket.Workspace, len(members))
				for i, m := range members {
					ws[i] = m.Workspace
				}
				return exporter.Write(ios, bitbucket.WorkspaceFields.ExportAll(ws, exporter.Fields))
			}
			cs := ios.ColorScheme()
			tp := output.NewTablePrinter(ios)
			for _, m := range members {
				tp.AddField(m.Workspace.Slug, cs.Bold)
				tp.AddField(m.Workspace.Name, nil)
				tp.AddField(m.Permission, cs.Gray)
				tp.EndRow()
			}
			return tp.Render()
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "L", 50, "Maximum number of workspaces to list")
	buildExporter = cmdutil.AddJSONFlags(cmd, bitbucket.WorkspaceFields.Validate, bitbucket.WorkspaceFields.Names())
	return cmd
}

func resolveWorkspace(f *cmdutil.Factory, arg string) (string, error) {
	if arg == "" {
		arg = cmdutil.DefaultWorkspace(f)
	}
	if arg == "" {
		if repo, err := f.BaseRepo(); err == nil {
			arg = repo.Workspace
		}
	}
	if arg == "" {
		return "", cmdutil.FlagErrorf("workspace required (argument, `bb config set workspace`, or a Bitbucket git remote)")
	}
	if !gitctx.ValidName(arg) {
		return "", fmt.Errorf("invalid workspace %q", arg)
	}
	return arg, nil
}

// NewCmdView returns `workspace view`.
func NewCmdView(f *cmdutil.Factory) *cobra.Command {
	var web bool
	var buildExporter func() (*output.Exporter, error)
	cmd := &cobra.Command{
		Use:   "view [<workspace>]",
		Short: "View a workspace",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			exporter, err := buildExporter()
			if err != nil {
				return err
			}
			arg := cmdutil.OptionalArg(args)
			ws, err := resolveWorkspace(f, arg)
			if err != nil {
				return err
			}
			if web {
				return cmdutil.OpenBrowser(f, gitctx.WorkspaceWebURL(ws))
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			var w bitbucket.Workspace
			if _, err := client.Do(cmd.Context(), api.Request{Path: "/workspaces/" + ws}, &w); err != nil {
				return err
			}
			ios := f.IOStreams
			if exporter != nil {
				return exporter.Write(ios, bitbucket.WorkspaceFields.Export(w, exporter.Fields))
			}
			cs := ios.ColorScheme()
			fmt.Fprintf(ios.Out, "%s (%s)\n", cs.Bold(w.Name), w.Slug)
			kind := "team"
			if w.IsPersonal {
				kind = "personal"
			}
			vis := "public"
			if w.IsPrivate {
				vis = "private"
			}
			fmt.Fprintf(ios.Out, "%s %s, %s\n", cs.Gray("Type:"), kind, vis)
			fmt.Fprintf(ios.Out, "%s %s\n", cs.Gray("UUID:"), w.UUID)
			if !w.CreatedOn.IsZero() {
				fmt.Fprintf(ios.Out, "%s %s\n", cs.Gray("Created:"), w.CreatedOn.Format("2006-01-02"))
			}
			fmt.Fprintf(ios.Out, "%s\n", cs.Gray("View this workspace on Bitbucket: "+w.Links.HTML()))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open the workspace in the browser")
	buildExporter = cmdutil.AddJSONFlags(cmd, bitbucket.WorkspaceFields.Validate, bitbucket.WorkspaceFields.Names())
	return cmd
}

// NewCmdMembers returns `workspace members`.
func NewCmdMembers(f *cmdutil.Factory) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "members [<workspace>]",
		Short: "List members of a workspace",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			arg := cmdutil.OptionalArg(args)
			ws, err := resolveWorkspace(f, arg)
			if err != nil {
				return err
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			ios := f.IOStreams
			cs := ios.ColorScheme()
			tp := output.NewTablePrinter(ios)
			if err := api.Paginate(cmd.Context(), client, "/workspaces/"+ws+"/members", api.ListOptions{Limit: limit, Fields: "values.user.nickname,values.user.display_name,values.user.uuid,next"}, func(m bitbucket.WorkspaceMembership) error {
				tp.AddField(m.User.Nickname, cs.Bold)
				tp.AddField(m.User.DisplayName, nil)
				tp.AddField(m.User.UUID, cs.Gray)
				tp.EndRow()
				return nil
			}); err != nil {
				return err
			}
			return tp.Render()
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "L", 100, "Maximum number of members to list")
	return cmd
}
