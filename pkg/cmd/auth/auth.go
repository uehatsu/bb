// Package auth implements `bb auth` subcommands.
package auth

import (
	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/cmdutil"
)

// NewCmdAuth returns the `auth` command group.
func NewCmdAuth(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth <command>",
		Args:  cobra.ArbitraryArgs,
		RunE:  cmdutil.GroupRunE, // unknown subcommands must fail, not print help with exit 0
		Short: "Authenticate bb with Bitbucket Cloud",
		Long: `Manage authentication state for Bitbucket Cloud.

bb uses Atlassian API tokens (with scopes). App passwords were removed by
Atlassian in July 2026 and are not supported.`,
	}
	cmd.AddCommand(
		NewCmdLogin(f),
		NewCmdLogout(f),
		NewCmdStatus(f),
		NewCmdToken(f),
		NewCmdRefresh(f),
		NewCmdSetupGit(f),
		NewCmdGitCredential(f),
	)
	return cmd
}

// TokenURL is where users create API tokens.
const TokenURL = "https://id.atlassian.com/manage-profile/security/api-tokens"

// RecommendedScopes lists the API token scopes bb needs for full functionality.
var RecommendedScopes = []struct{ Scope, Purpose string }{
	{"read:user:bitbucket", "identify the logged in user (auth status)"},
	{"read:workspace:bitbucket", "list workspaces and members"},
	{"read:project:bitbucket", "list projects"},
	{"admin:project:bitbucket", "create projects (optional)"},
	{"read:repository:bitbucket", "list/view/clone repositories, branches, source"},
	{"write:repository:bitbucket", "create branches, push"},
	{"admin:repository:bitbucket", "create/edit repositories (optional)"},
	{"delete:repository:bitbucket", "delete repositories (optional)"},
	{"read:pullrequest:bitbucket", "list/view pull requests and comments"},
	{"write:pullrequest:bitbucket", "create/approve/merge/decline pull requests"},
	{"read:pipeline:bitbucket", "list/view pipelines and logs"},
	{"write:pipeline:bitbucket", "run/stop pipelines"},
}
