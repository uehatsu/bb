package auth

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/config"
)

// NewCmdStatus returns the status command.
func NewCmdStatus(f *cmdutil.Factory) *cobra.Command {
	var showToken bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show authentication status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStatus(f, showToken)
		},
	}
	cmd.Flags().BoolVarP(&showToken, "show-token", "t", false, "Display the token")
	return cmd
}

func runStatus(f *cmdutil.Factory, showToken bool) error {
	io := f.IOStreams
	cs := io.ColorScheme()
	cfg, err := f.Config()
	if err != nil {
		return err
	}
	source := "hosts.yml"
	if os.Getenv("BB_TOKEN") != "" {
		source = "BB_TOKEN"
	}
	cred, err := config.ResolveCredential(cfg.Credentials(), config.DefaultHost, os.Getenv)
	if errors.Is(err, config.ErrNotFound) {
		fmt.Fprintf(io.Out, "bitbucket.org\n  %s Not logged in. Run `bb auth login`.\n", cs.FailureIcon())
		return cmdutil.SilentError
	}
	if err != nil {
		return err
	}

	fmt.Fprintln(io.Out, "bitbucket.org")
	client, err := f.APIClient()
	if err != nil {
		return err
	}
	var user bitbucket.User
	if _, err := client.Do(cmdCtx(), api.Request{Path: "/user"}, &user); err != nil {
		fmt.Fprintf(io.Out, "  %s Token (%s) is invalid: %v\n", cs.FailureIcon(), source, err)
		if cred.IsExpired(time.Now()) {
			fmt.Fprintf(io.Out, "  %s The recorded expiry (%s) has passed. Create a new token and run `bb auth login`.\n", cs.WarningIcon(), cred.ExpiresAt.Format("2006-01-02"))
		}
		return cmdutil.SilentError
	}
	fmt.Fprintf(io.Out, "  %s Logged in as %s", cs.SuccessIcon(), cs.Bold(user.Name()))
	if user.Email != "" {
		fmt.Fprintf(io.Out, " (%s)", user.Email)
	}
	fmt.Fprintln(io.Out)

	method := "API token (Basic: email + token)"
	if cred.Method == config.AuthBearer {
		method = "Access token (Bearer)"
	}
	fmt.Fprintf(io.Out, "  - Auth method: %s\n", method)
	fmt.Fprintf(io.Out, "  - Credential source: %s\n", source)
	fmt.Fprintf(io.Out, "  - Git username: %s\n", cred.GitUsername())
	switch {
	case cred.ExpiresAt == nil:
		fmt.Fprintf(io.Out, "  - Token expiry: not recorded\n")
	case cred.IsExpired(time.Now()):
		fmt.Fprintf(io.Out, "  - Token expiry: %s %s\n", cred.ExpiresAt.Format("2006-01-02"), cs.Red("(expired)"))
	case cred.ExpiresWithin(time.Now(), 7*24*time.Hour):
		fmt.Fprintf(io.Out, "  - Token expiry: %s %s\n", cred.ExpiresAt.Format("2006-01-02"), cs.Yellow("(expires soon!)"))
	default:
		fmt.Fprintf(io.Out, "  - Token expiry: %s\n", cred.ExpiresAt.Format("2006-01-02"))
	}
	if showToken {
		fmt.Fprintf(io.Out, "  - Token: %s\n", cred.Token)
	} else {
		fmt.Fprintf(io.Out, "  - Token: %s\n", maskToken(cred.Token))
	}

	var ws []bitbucket.WorkspaceMembership
	if err := api.Paginate(cmdCtx(), client, "/user/workspaces", api.ListOptions{Fields: "values.workspace.slug,values.permission,next"}, func(m bitbucket.WorkspaceMembership) error {
		ws = append(ws, m)
		return nil
	}); err == nil && len(ws) > 0 {
		fmt.Fprintf(io.Out, "  - Workspaces:")
		for _, m := range ws {
			fmt.Fprintf(io.Out, " %s", m.Workspace.Slug)
			if m.Permission != "" {
				fmt.Fprintf(io.Out, "(%s)", m.Permission)
			}
		}
		fmt.Fprintln(io.Out)
	}
	return nil
}

func maskToken(t string) string {
	if len(t) <= 8 {
		return "********"
	}
	return t[:4] + "****" + t[len(t)-4:]
}
