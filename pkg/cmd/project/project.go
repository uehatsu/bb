// Package project implements `bb project`.
package project

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/gitctx"
	"github.com/uehatsu/bb/internal/output"
)

// NewCmdProject returns the `project` command group.
func NewCmdProject(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "project <command>",
		Short: "Manage projects in a workspace",
		Long:  "Bitbucket groups repositories into projects. Every repository belongs to exactly one project.",
	}
	cmd.PersistentFlags().StringP("workspace", "w", "", "Workspace (default: configured workspace or current repository's)")
	cmd.AddCommand(NewCmdList(f), NewCmdView(f), NewCmdCreate(f))
	return cmd
}

func workspaceFrom(cmd *cobra.Command, f *cmdutil.Factory) (string, error) {
	ws, _ := cmd.Flags().GetString("workspace")
	if ws == "" {
		ws = cmdutil.DefaultWorkspace(f)
	}
	if ws == "" {
		if repo, err := f.BaseRepo(); err == nil {
			ws = repo.Workspace
		}
	}
	if ws == "" {
		return "", cmdutil.FlagErrorf("--workspace required (or `bb config set workspace`)")
	}
	if !gitctx.ValidName(ws) {
		return "", fmt.Errorf("invalid workspace %q", ws)
	}
	return ws, nil
}

const projectFields = "values.key,values.name,values.uuid,values.description,values.is_private,values.created_on,values.updated_on,values.links.html.href,next"

// NewCmdList returns `project list`.
func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	var limit int
	var buildExporter func() (*output.Exporter, error)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects in a workspace",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			exporter, err := buildExporter()
			if err != nil {
				return err
			}
			ws, err := workspaceFrom(cmd, f)
			if err != nil {
				return err
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			var projects []bitbucket.Project
			if err := api.Paginate(cmd.Context(), client, "/workspaces/"+ws+"/projects", api.ListOptions{Limit: limit, Fields: projectFields, Sort: "name"}, func(p bitbucket.Project) error {
				projects = append(projects, p)
				return nil
			}); err != nil {
				return err
			}
			ios := f.IOStreams
			if exporter != nil {
				return exporter.Write(ios, bitbucket.ProjectFields.ExportAll(projects, exporter.Fields))
			}
			cs := ios.ColorScheme()
			tp := output.NewTablePrinter(ios)
			for _, p := range projects {
				tp.AddField(p.Key, cs.Bold)
				tp.AddField(p.Name, nil)
				tp.AddField(p.Description, cs.Gray)
				vis := "public"
				if p.IsPrivate {
					vis = "private"
				}
				tp.AddField(vis, cs.Gray)
				tp.EndRow()
			}
			return tp.Render()
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "L", 50, "Maximum number of projects to list")
	buildExporter = cmdutil.AddJSONFlags(cmd, bitbucket.ProjectFields.Validate, bitbucket.ProjectFields.Names())
	return cmd
}

// NewCmdView returns `project view`.
func NewCmdView(f *cmdutil.Factory) *cobra.Command {
	var buildExporter func() (*output.Exporter, error)
	cmd := &cobra.Command{
		Use:   "view <key>",
		Short: "View a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			exporter, err := buildExporter()
			if err != nil {
				return err
			}
			ws, err := workspaceFrom(cmd, f)
			if err != nil {
				return err
			}
			key := strings.ToUpper(args[0])
			if !gitctx.ValidName(key) {
				return fmt.Errorf("invalid project key %q", args[0])
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			var p bitbucket.Project
			if _, err := client.Do(cmd.Context(), api.Request{Path: "/workspaces/" + ws + "/projects/" + key}, &p); err != nil {
				return err
			}
			ios := f.IOStreams
			if exporter != nil {
				return exporter.Write(ios, bitbucket.ProjectFields.Export(p, exporter.Fields))
			}
			cs := ios.ColorScheme()
			fmt.Fprintf(ios.Out, "%s (%s)\n", cs.Bold(p.Name), p.Key)
			if p.Description != "" {
				fmt.Fprintln(ios.Out, p.Description)
			}
			vis := "public"
			if p.IsPrivate {
				vis = "private"
			}
			fmt.Fprintf(ios.Out, "%s %s\n", cs.Gray("Visibility:"), vis)
			fmt.Fprintf(ios.Out, "%s\n", cs.Gray("View this project on Bitbucket: "+p.Links.HTML()))
			return nil
		},
	}
	buildExporter = cmdutil.AddJSONFlags(cmd, bitbucket.ProjectFields.Validate, bitbucket.ProjectFields.Names())
	return cmd
}

// NewCmdCreate returns `project create`.
func NewCmdCreate(f *cmdutil.Factory) *cobra.Command {
	var name, description string
	var private, public bool
	cmd := &cobra.Command{
		Use:   "create <key>",
		Short: "Create a project",
		Long:  "Create a project. Requires the admin:project:bitbucket scope.",
		Example: `  $ bb project create PROJ --name "My Project" --private
  $ bb project create TOOLS --name Tools -w acme --public`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if private && public {
				return cmdutil.FlagErrorf("specify only one of --private or --public")
			}
			if name == "" {
				return cmdutil.FlagErrorf("--name is required")
			}
			ws, err := workspaceFrom(cmd, f)
			if err != nil {
				return err
			}
			key := strings.ToUpper(args[0])
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			body := map[string]any{"key": key, "name": name, "is_private": !public}
			if description != "" {
				body["description"] = description
			}
			var created bitbucket.Project
			if _, err := client.Do(cmd.Context(), api.Request{Method: "POST", Path: "/workspaces/" + ws + "/projects", Body: body}, &created); err != nil {
				return err
			}
			fmt.Fprintf(f.IOStreams.ErrOut, "%s Created project %s (%s) in %s\n", f.IOStreams.ColorScheme().SuccessIcon(), created.Name, created.Key, ws)
			fmt.Fprintln(f.IOStreams.Out, created.Links.HTML())
			return nil
		},
	}
	cmd.Flags().StringVarP(&name, "name", "n", "", "Project name")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Project description")
	cmd.Flags().BoolVar(&private, "private", false, "Make the project private (default)")
	cmd.Flags().BoolVar(&public, "public", false, "Make the project public")
	return cmd
}
