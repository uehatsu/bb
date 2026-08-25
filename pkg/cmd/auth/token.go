package auth

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/config"
)

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
			cred, err := config.ResolveFreshCredential(cmd.Context(), cfg.Credentials(), config.DefaultHost, os.Getenv, time.Now())
			if errors.Is(err, config.ErrNotFound) {
				return cmdutil.NewAuthError("no token found")
			}
			if err != nil {
				return err
			}
			if f.IOStreams.IsStdoutTTY() {
				fmt.Fprintf(f.IOStreams.ErrOut, "%s Printing a secret to the terminal.\n", f.IOStreams.ColorScheme().WarningIcon())
			}
			fmt.Fprintln(f.IOStreams.Out, cred.Token)
			return nil
		},
	}
}
