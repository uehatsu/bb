// Package root assembles the top-level `bb` command tree.
package root

import (
	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/build"
	"github.com/uehatsu/bb/internal/cmdutil"
	apiCmd "github.com/uehatsu/bb/pkg/cmd/api"
	authCmd "github.com/uehatsu/bb/pkg/cmd/auth"
	branchCmd "github.com/uehatsu/bb/pkg/cmd/branch"
	browseCmd "github.com/uehatsu/bb/pkg/cmd/browse"
	configCmd "github.com/uehatsu/bb/pkg/cmd/config"
	pipelineCmd "github.com/uehatsu/bb/pkg/cmd/pipeline"
	prCmd "github.com/uehatsu/bb/pkg/cmd/pr"
	projectCmd "github.com/uehatsu/bb/pkg/cmd/project"
	repoCmd "github.com/uehatsu/bb/pkg/cmd/repo"
	versionCmd "github.com/uehatsu/bb/pkg/cmd/version"
	workspaceCmd "github.com/uehatsu/bb/pkg/cmd/workspace"
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
		// Same behaviour as the command groups: no args prints help, an
		// unknown subcommand fails with a usage error (exit 1).
		Args: cobra.ArbitraryArgs,
		RunE: cmdutil.GroupRunE,
	}
	cmd.SetVersionTemplate(versionCmd.Format(build.Version, build.Date))
	cmd.PersistentFlags().Bool("help", false, "Show help for command")
	cmd.Flags().Bool("version", false, "Show bb version")

	cmd.AddCommand(
		authCmd.NewCmdAuth(f),
		apiCmd.NewCmdAPI(f),
		browseCmd.NewCmdBrowse(f),
		repoCmd.NewCmdRepo(f),
		prCmd.NewCmdPR(f),
		pipelineCmd.NewCmdPipeline(f),
		branchCmd.NewCmdBranch(f),
		workspaceCmd.NewCmdWorkspace(f),
		projectCmd.NewCmdProject(f),
		configCmd.NewCmdConfig(f),
		versionCmd.NewCmdVersion(f),
	)

	cmd.SetHelpCommand(&cobra.Command{Hidden: true})
	return cmd
}
