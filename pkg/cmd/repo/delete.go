package repo

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/cmdutil"
)

// NewCmdDelete returns `repo delete`.
func NewCmdDelete(f *cmdutil.Factory) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete [<repository>]",
		Short: "Delete a repository",
		Long: `Delete a Bitbucket repository. This cannot be undone.

Without --yes you must type the repository's full name to confirm. The token
needs the delete:repository:bitbucket scope.`,
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
			if !yes {
				if !f.IOStreams.CanPrompt() {
					return cmdutil.FlagErrorf("--yes required when not running interactively")
				}
				typed, err := f.Prompter.Input(fmt.Sprintf("Type %s to confirm deletion", repo.FullName()), "")
				if err != nil {
					return cmdutil.CancelError
				}
				if typed != repo.FullName() {
					return fmt.Errorf("confirmation did not match %q; aborting", repo.FullName())
				}
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			if _, err := client.Do(context.Background(), api.Request{Method: "DELETE", Path: fmt.Sprintf("/repositories/%s/%s", repo.Workspace, repo.Slug)}, nil); err != nil {
				return err
			}
			fmt.Fprintf(f.IOStreams.ErrOut, "%s Deleted repository %s\n", f.IOStreams.ColorScheme().SuccessIcon(), repo.FullName())
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm deletion without prompting")
	cmdutil.EnableRepoOverride(cmd, f)
	return cmd
}
