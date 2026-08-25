package auth

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/cmdutil"
)

// credentialKey is the host-scoped git config key for the helper.
const credentialKey = "credential.https://bitbucket.org.helper"

// NewCmdSetupGit configures git to use bb as a credential helper.
func NewCmdSetupGit(f *cmdutil.Factory) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "setup-git",
		Short: "Configure git to use bb as a credential helper for bitbucket.org",
		Long: `Registers 'bb auth git-credential' as the git credential helper for
https://bitbucket.org only. An empty helper entry is written first so that
previously configured global helpers (osxkeychain, manager, ...) holding stale
app passwords are bypassed for this host.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, err := f.GitClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			exe := f.Executable
			if exe == "" {
				exe = "bb"
			}
			helper := fmt.Sprintf("!%q auth git-credential", exe)
			existing, err := g.ConfigGet(ctx, "--global", credentialKey)
			if err != nil {
				return err
			}
			if existing != "" && !force {
				fmt.Fprintf(f.IOStreams.ErrOut, "%s credential helper for bitbucket.org already configured (%s). Use --force to overwrite.\n", f.IOStreams.ColorScheme().WarningIcon(), existing)
				return nil
			}
			if err := g.ConfigUnsetAll(ctx, "--global", credentialKey); err != nil {
				return err
			}
			if err := g.ConfigAdd(ctx, "--global", credentialKey, ""); err != nil {
				return err
			}
			if err := g.ConfigAdd(ctx, "--global", credentialKey, helper); err != nil {
				return err
			}
			fmt.Fprintf(f.IOStreams.ErrOut, "%s Configured git to use bb as the credential helper for https://bitbucket.org\n", f.IOStreams.ColorScheme().SuccessIcon())
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite an existing helper configuration")
	return cmd
}
