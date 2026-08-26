// Package root assembles the top-level `bb` command tree.
package root

import (
	"strings"

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
		// No Args/RunE on purpose: cobra then validates the first argument
		// before parsing flags (`bb bogus --help` is still an unknown
		// command) and prints help when there is none; main maps its
		// "unknown command" error to a usage error, exit 1.
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

	// `bb help [command]` works like gh, but the command is kept out of the
	// command list. cobra's usage template lists a command named "help" even
	// when it is hidden, so drop that clause from the template (a no-op if
	// cobra ever rewords it).
	cmd.SetHelpCommand(&cobra.Command{
		Use:    "help [command]",
		Short:  "Help about any command",
		Hidden: true,
		RunE: func(c *cobra.Command, args []string) error {
			target, rest, err := c.Root().Find(args)
			if target == nil || err != nil || len(rest) > 0 {
				// Like an unknown command: usage error, exit 1 (gh does the same).
				return cmdutil.FlagErrorf("unknown help topic %q for %q", strings.Join(args, " "), c.Root().CommandPath())
			}
			target.InitDefaultHelpFlag()
			target.InitDefaultVersionFlag()
			return target.Help()
		},
	})
	cmd.SetUsageTemplate(strings.ReplaceAll(cmd.UsageTemplate(), `(or .IsAvailableCommand (eq .Name "help"))`, `.IsAvailableCommand`))
	return cmd
}
