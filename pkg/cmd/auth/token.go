package auth

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/config"
)

func cmdCtx() context.Context { return context.Background() }

// NewCmdToken prints the stored token (for scripting).
func NewCmdToken(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "token",
		Short: "Print the authentication token bb is configured to use",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			cred, err := config.ResolveCredential(cfg.Credentials(), config.DefaultHost, os.Getenv)
			if err != nil {
				return cmdutil.NewAuthError("no token found")
			}
			if f.IOStreams.IsStdoutTTY() {
				fmt.Fprintf(f.IOStreams.ErrOut, "%s Printing a secret to the terminal.\n", f.IOStreams.ColorScheme().WarningIcon())
			}
			fmt.Fprintln(f.IOStreams.Out, cred.Token)
			return nil
		},
	}
}
