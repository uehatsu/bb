// Package root assembles the top-level `bb` command tree.
package root

import (
	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/build"
	"github.com/uehatsu/bb/internal/cmdutil"
	authCmd "github.com/uehatsu/bb/pkg/cmd/auth"
	versionCmd "github.com/uehatsu/bb/pkg/cmd/version"
)

// NewCmdRoot builds the root command with all subcommands attached.
func NewCmdRoot(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bb <command> <subcommand> [flags]",
		Short: "Bitbucket Cloud CLI",
		Long:  "Work seamlessly with Bitbucket Cloud from the command line.",
		Example: `  $ bb auth login
  $ bb repo clone myworkspace/myrepo
  $ bb pr create --title "Fix bug" --body "Details"
  $ bb pipeline watch 42`,
		SilenceErrors: true,
		SilenceUsage:  true,
		Version:       build.Version,
	}
	cmd.SetVersionTemplate(versionCmd.Format(build.Version, build.Date))
	cmd.PersistentFlags().Bool("help", false, "Show help for command")
	cmd.Flags().Bool("version", false, "Show bb version")

	cmd.AddCommand(
		authCmd.NewCmdAuth(f),
		versionCmd.NewCmdVersion(f),
	)

	cmd.SetHelpCommand(&cobra.Command{Hidden: true})
	return cmd
}
