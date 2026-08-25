package auth

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/config"
)

// NewCmdLogout returns the logout command.
func NewCmdLogout(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove stored Bitbucket credentials",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			store := cfg.Credentials()
			if _, err := store.Get(config.DefaultHost); errors.Is(err, config.ErrNotFound) {
				return errors.New("not logged in to bitbucket.org")
			} else if err != nil {
				return err
			}
			if err := store.Delete(config.DefaultHost); err != nil {
				return err
			}
			fmt.Fprintf(f.IOStreams.ErrOut, "%s Logged out of bitbucket.org\n", f.IOStreams.ColorScheme().SuccessIcon())
			return nil
		},
	}
}
