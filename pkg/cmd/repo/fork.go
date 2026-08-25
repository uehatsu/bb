package repo

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
)

// NewCmdFork returns `repo fork`.
func NewCmdFork(f *cmdutil.Factory) *cobra.Command {
	var workspace, name string
	var clone bool
	cmd := &cobra.Command{
		Use:   "fork [<repository>]",
		Short: "Create a fork of a repository",
		Long: `Create a fork of a repository. The fork is created in --workspace (or the
configured default workspace). Use --name to rename the fork, which is
required when forking into the same workspace.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			arg := ""
			if len(args) > 0 {
				arg = args[0]
			}
			repo, err := resolveRepoArg(f, arg)
			if err != nil {
				return err
			}
			if workspace == "" {
				workspace = cmdutil.DefaultWorkspace(f)
			}
			if workspace == "" {
				return cmdutil.FlagErrorf("--workspace is required (or set a default with `bb config set workspace`)")
			}
			return runFork(cmd.Context(), f, repo, workspace, name, clone)
		},
	}
	cmd.Flags().StringVarP(&workspace, "workspace", "w", "", "Workspace to create the fork in")
	cmd.Flags().StringVar(&name, "name", "", "Name for the fork")
	cmd.Flags().BoolVar(&clone, "clone", false, "Clone the fork after creating it")
	cmdutil.EnableRepoOverride(cmd, f)
	return cmd
}

func runFork(ctx context.Context, f *cmdutil.Factory, repo cmdutil.Repo, workspace, name string, clone bool) error {
	client, err := f.APIClient()
	if err != nil {
		return err
	}
	body := map[string]any{"workspace": map[string]string{"slug": workspace}}
	if name != "" {
		body["name"] = name
	}
	var fork bitbucket.Repository
	if _, err := client.Do(ctx, api.Request{Method: "POST", Path: fmt.Sprintf("/repositories/%s/%s/forks", repo.Workspace, repo.Slug), Body: body}, &fork); err != nil {
		return err
	}
	cs := f.IOStreams.ColorScheme()
	fmt.Fprintf(f.IOStreams.ErrOut, "%s Created fork %s\n", cs.SuccessIcon(), cs.Bold(fork.FullName))
	fmt.Fprintln(f.IOStreams.Out, fork.Links.HTML())
	if clone {
		if fork.Parent == nil {
			fork.Parent = &bitbucket.Repository{FullName: repo.FullName()}
		}
		return cloneRepo(ctx, f, &fork, "", nil)
	}
	return nil
}
