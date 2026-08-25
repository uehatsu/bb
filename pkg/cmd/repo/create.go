package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/gitctx"
)

// CreateOptions for `repo create`.
type CreateOptions struct {
	Name        string
	Workspace   string
	Project     string
	Description string
	Private     bool
	Public      bool
	ForkPolicy  string
	Language    string
	Clone       bool
}

// NewCmdCreate returns `repo create`.
func NewCmdCreate(f *cmdutil.Factory) *cobra.Command {
	opts := &CreateOptions{}
	cmd := &cobra.Command{
		Use:   "create [<name>]",
		Short: "Create a new repository",
		Long: `Create a new repository in a workspace.

The name may be given as WORKSPACE/NAME or NAME (with --workspace or the
configured default workspace). Every Bitbucket repository belongs to a
project; when --project is omitted, Bitbucket assigns the repository to the
workspace's oldest project.`,
		Example: `  $ bb repo create widgets --workspace acme --private --project PROJ
  $ bb repo create acme/widgets --public --clone`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Name = args[0]
			}
			if opts.Private && opts.Public {
				return cmdutil.FlagErrorf("specify only one of --private or --public")
			}
			switch opts.ForkPolicy {
			case "", "allow_forks", "no_public_forks", "no_forks":
			default:
				return cmdutil.FlagErrorf("invalid --fork-policy %q (allow_forks|no_public_forks|no_forks)", opts.ForkPolicy)
			}
			if opts.Name == "" {
				if !f.IOStreams.CanPrompt() {
					return cmdutil.FlagErrorf("repository name required when not running interactively")
				}
				if err := promptCreate(f, opts); err != nil {
					return err
				}
			} else if !opts.Private && !opts.Public {
				if f.IOStreams.CanPrompt() {
					v, err := f.Prompter.Select("Visibility", []string{"private", "public"})
					if err != nil {
						return cmdutil.ErrCancel
					}
					opts.Private = v == "private"
				} else {
					return cmdutil.FlagErrorf("--private or --public is required when not running interactively")
				}
			}
			return runCreate(cmd.Context(), f, opts)
		},
	}
	cmd.Flags().StringVarP(&opts.Workspace, "workspace", "w", "", "Workspace to create the repository in")
	cmd.Flags().StringVarP(&opts.Project, "project", "p", "", "Project key to place the repository in")
	cmd.Flags().StringVarP(&opts.Description, "description", "d", "", "Description of the repository")
	cmd.Flags().BoolVar(&opts.Private, "private", false, "Make the new repository private")
	cmd.Flags().BoolVar(&opts.Public, "public", false, "Make the new repository public")
	cmd.Flags().StringVar(&opts.ForkPolicy, "fork-policy", "", "Fork policy: {allow_forks|no_public_forks|no_forks}")
	cmd.Flags().StringVar(&opts.Language, "language", "", "Primary programming language")
	cmd.Flags().BoolVarP(&opts.Clone, "clone", "c", false, "Clone the new repository to the current directory")
	return cmd
}

func promptCreate(f *cmdutil.Factory, opts *CreateOptions) error {
	var err error
	if opts.Name, err = f.Prompter.Input("Repository name", "my-repo"); err != nil {
		return cmdutil.ErrCancel
	}
	if opts.Description, err = f.Prompter.Input("Description", ""); err != nil {
		return cmdutil.ErrCancel
	}
	v, err := f.Prompter.Select("Visibility", []string{"private", "public"})
	if err != nil {
		return cmdutil.ErrCancel
	}
	opts.Private = v == "private"
	return nil
}

func runCreate(ctx context.Context, f *cmdutil.Factory, opts *CreateOptions) error {
	ios := f.IOStreams
	ws := opts.Workspace
	name := opts.Name
	if strings.Contains(name, "/") {
		parts := strings.SplitN(name, "/", 2)
		if ws != "" && ws != parts[0] {
			return errors.New("workspace given both in the name and --workspace")
		}
		ws, name = parts[0], parts[1]
	}
	if ws == "" {
		ws = cmdutil.DefaultWorkspace(f)
	}
	if ws == "" {
		return errors.New("workspace required: use WORKSPACE/NAME, --workspace, or `bb config set workspace <slug>`")
	}
	if !gitctx.ValidName(ws) || !gitctx.ValidName(name) {
		return fmt.Errorf("invalid repository name %q", opts.Name)
	}
	client, err := f.APIClient()
	if err != nil {
		return err
	}
	body := map[string]any{
		"scm":        "git",
		"is_private": opts.Private || !opts.Public,
	}
	if opts.Description != "" {
		body["description"] = opts.Description
	}
	if opts.Project != "" {
		body["project"] = map[string]string{"key": opts.Project}
	}
	if opts.ForkPolicy != "" {
		body["fork_policy"] = opts.ForkPolicy
	}
	if opts.Language != "" {
		body["language"] = opts.Language
	}
	var created bitbucket.Repository
	slug := strings.ToLower(name)
	if _, err := client.Do(ctx, api.Request{Method: "POST", Path: fmt.Sprintf("/repositories/%s/%s", ws, slug), Body: body}, &created); err != nil {
		return err
	}
	cs := ios.ColorScheme()
	fmt.Fprintf(ios.ErrOut, "%s Created repository %s on Bitbucket\n", cs.SuccessIcon(), cs.Bold(created.FullName))
	if created.Project == nil && opts.Project == "" {
		fmt.Fprintf(ios.ErrOut, "  (assigned to the workspace's default project)\n")
	}
	fmt.Fprintln(ios.Out, created.Links.HTML())
	if opts.Clone {
		return cloneRepo(ctx, f, &created, "", nil)
	}
	return nil
}
