package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/config"
)

// NewCmdRefresh forces an OAuth access token refresh.
func NewCmdRefresh(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Refresh the stored OAuth access token",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			cred, err := cfg.Credentials().Get(config.DefaultHost)
			if errors.Is(err, config.ErrNotFound) {
				return cmdutil.NewAuthError("not logged in")
			}
			if err != nil {
				return err
			}
			if cred.Method != config.AuthOAuth {
				return errors.New("only OAuth credentials can be refreshed (API tokens must be recreated at id.atlassian.com)")
			}
			updated, err := RefreshCredential(cmd.Context(), cred)
			if err != nil {
				return err
			}
			if err := cfg.Credentials().Set(config.DefaultHost, updated); err != nil {
				return err
			}
			fmt.Fprintf(f.IOStreams.ErrOut, "%s Refreshed OAuth token (expires %s)\n", f.IOStreams.ColorScheme().SuccessIcon(), updated.ExpiresAt.Format(time.RFC3339))
			return nil
		},
	}
}

// RefreshCredential exchanges the refresh token for a new access token.
func RefreshCredential(ctx context.Context, cred config.Credential) (config.Credential, error) {
	fresh, err := config.RefreshOAuth(ctx, cred)
	if err != nil {
		return cred, fmt.Errorf("refreshing OAuth token: %w (run `bb auth login --web` to re-authenticate)", err)
	}
	return fresh, nil
}
