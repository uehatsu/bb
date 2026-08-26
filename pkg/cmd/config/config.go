// Package config implements `bb config`.
package config

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/cmdutil"
	bbconfig "github.com/uehatsu/bb/internal/config"
)

// NewCmdConfig returns the `config` command group.
func NewCmdConfig(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config <command>",
		Args:  cobra.NoArgs, // unknown subcommands must fail, not print help with exit 0
		Short: "Manage configuration for bb",
		Long: fmt.Sprintf(`Display or change configuration settings for bb.

Configuration is stored in %s/config.yml.

Keys:
  workspace       default workspace for repository names without a workspace
  git_protocol    protocol for clone/checkout: https (default) | ssh
  merge_strategy  default for 'bb pr merge': merge_commit (default) | squash |
                  fast_forward | squash_fast_forward | rebase_fast_forward | rebase_merge
  editor          editor for multi-line prompts
  pager           pager for long output (e.g. "less -R")
  browser         browser command for --web
  prompt          enabled (default) | disabled
  credential_store  file (default) | keyring   (where tokens are stored)
  oauth_client_id   OAuth consumer key for 'bb auth login --web'
  oauth_port        loopback port for the OAuth callback (default 8976)`, bbconfig.Dir()),
	}
	cmd.AddCommand(newCmdGet(f), newCmdSet(f), newCmdList(f))
	return cmd
}

func newCmdGet(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "get <key>",
		Short: "Print the value of a configuration key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			v, err := cfg.Get(args[0])
			if err != nil {
				return cmdutil.FlagErrorWrap(err)
			}
			if v != "" {
				fmt.Fprintln(f.IOStreams.Out, v)
			}
			return nil
		},
	}
}

func newCmdSet(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Update a configuration value",
		Example: `  $ bb config set workspace acme
  $ bb config set git_protocol ssh
  $ bb config set merge_strategy squash`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			if args[0] == "credential_store" {
				return migrateCredentials(f, cfg, args[1])
			}
			if err := cfg.Set(args[0], args[1]); err != nil {
				return cmdutil.FlagErrorWrap(err)
			}
			return writeConfig(cfg)
		},
	}
}

func newCmdList(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Print a list of configuration keys and values",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			for _, k := range cfg.Keys() {
				v, _ := cfg.Get(k)
				fmt.Fprintf(f.IOStreams.Out, "%s=%s\n", k, v)
			}
			return nil
		},
	}
}

// writeConfig persists the configuration; a variable so tests can inject
// write failures.
var writeConfig = func(cfg *bbconfig.Config) error { return cfg.Write() }

// migrateCredentials switches the credential store, moving the stored
// credential along. The order is chosen so that no step can leave the user
// logged out:
//
//  1. copy the credential into the target store
//  2. persist the config change (on failure: remove the copy, keep the old)
//  3. verify the credential resolves from the target store
//  4. only then delete it from the old store (failure here is a warning)
func migrateCredentials(f *cmdutil.Factory, cfg *bbconfig.Config, target string) error {
	if err := bbconfig.ValidateValue("credential_store", target); err != nil {
		return cmdutil.FlagErrorWrap(err)
	}
	current, _ := cfg.Get("credential_store")
	if current == target {
		return nil
	}
	old := cfg.Credentials()
	cred, err := old.Get(bbconfig.DefaultHost)
	hasCred := err == nil
	if err != nil && !errors.Is(err, bbconfig.ErrNotFound) {
		return err
	}
	var dst bbconfig.CredentialStore
	if target == bbconfig.StoreKeyring {
		dst = bbconfig.NewKeyringCredentialStore()
	} else {
		dst = bbconfig.NewFileCredentialStore(bbconfig.Dir())
	}

	// 1. copy
	if hasCred {
		if err := dst.Set(bbconfig.DefaultHost, cred); err != nil {
			return fmt.Errorf("copying credential to %s store: %w", target, err)
		}
	}
	// 2. persist config; roll back the copy on failure
	if err := cfg.Set("credential_store", target); err != nil {
		return cmdutil.FlagErrorWrap(err)
	}
	if err := writeConfig(cfg); err != nil {
		_ = cfg.Set("credential_store", current)
		if hasCred {
			_ = dst.Delete(bbconfig.DefaultHost)
		}
		return fmt.Errorf("saving config: %w (credential store left unchanged)", err)
	}
	if !hasCred {
		return nil
	}
	// 3. verify the new store serves the credential before removing the old copy
	if got, err := dst.Get(bbconfig.DefaultHost); err != nil || got.Token != cred.Token {
		return fmt.Errorf("credential store switched to %s, but the credential could not be read back (%v); the old copy in %s was kept — run `bb auth login` if commands report you are logged out", target, err, current)
	}
	// 4. remove the old copy
	if err := old.Delete(bbconfig.DefaultHost); err != nil {
		fmt.Fprintf(f.IOStreams.ErrOut, "warning: credential copied to %s but could not be removed from %s: %v\n", target, current, err)
	}
	fmt.Fprintf(f.IOStreams.ErrOut, "%s Moved credential from %s to %s store\n", f.IOStreams.ColorScheme().SuccessIcon(), current, target)
	return nil
}
