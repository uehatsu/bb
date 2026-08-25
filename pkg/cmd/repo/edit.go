package repo

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
)

// NewCmdEdit returns `repo edit`.
func NewCmdEdit(f *cmdutil.Factory) *cobra.Command {
	var description, visibility, forkPolicy, defaultBranch, language, name string
	cmd := &cobra.Command{
		Use:   "edit [<repository>]",
		Short: "Edit repository settings",
		Example: `  $ bb repo edit --description "New description"
  $ bb repo edit acme/widgets --visibility private --default-branch main`,
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
			body := map[string]any{}
			if cmd.Flags().Changed("description") {
				body["description"] = description
			}
			if cmd.Flags().Changed("name") {
				body["name"] = name
			}
			if cmd.Flags().Changed("language") {
				body["language"] = language
			}
			switch visibility {
			case "":
			case "public":
				body["is_private"] = false
			case "private":
				body["is_private"] = true
			default:
				return cmdutil.FlagErrorf("invalid --visibility %q (public|private)", visibility)
			}
			switch forkPolicy {
			case "":
			case "allow_forks", "no_public_forks", "no_forks":
				body["fork_policy"] = forkPolicy
			default:
				return cmdutil.FlagErrorf("invalid --fork-policy %q", forkPolicy)
			}
			if defaultBranch != "" {
				body["mainbranch"] = map[string]string{"name": defaultBranch}
			}
			if len(body) == 0 {
				return cmdutil.FlagErrorf("specify at least one setting to change")
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			var updated bitbucket.Repository
			if _, err := client.Do(cmd.Context(), api.Request{Method: "PUT", Path: fmt.Sprintf("/repositories/%s/%s", repo.Workspace, repo.Slug), Body: body}, &updated); err != nil {
				return err
			}
			fmt.Fprintf(f.IOStreams.ErrOut, "%s Edited repository %s\n", f.IOStreams.ColorScheme().SuccessIcon(), updated.FullName)
			return nil
		},
	}
	cmd.Flags().StringVarP(&description, "description", "d", "", "Description of the repository")
	cmd.Flags().StringVar(&name, "name", "", "Rename the repository (changes the slug)")
	cmd.Flags().StringVar(&visibility, "visibility", "", "Change visibility: {public|private}")
	cmd.Flags().StringVar(&forkPolicy, "fork-policy", "", "Fork policy: {allow_forks|no_public_forks|no_forks}")
	cmd.Flags().StringVar(&defaultBranch, "default-branch", "", "Set the default (main) branch")
	cmd.Flags().StringVar(&language, "language", "", "Primary programming language")
	cmdutil.EnableRepoOverride(cmd, f)
	return cmd
}
