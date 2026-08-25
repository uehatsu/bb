// Package config implements `bb config`.
package config

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/cmdutil"
	bbconfig "github.com/uehatsu/bb/internal/config"
)

// NewCmdConfig returns the `config` command group.
func NewCmdConfig(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config <command>",
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
  prompt          enabled (default) | disabled`, bbconfig.Dir()),
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
			if err := cfg.Set(args[0], args[1]); err != nil {
				return cmdutil.FlagErrorWrap(err)
			}
			return cfg.Write()
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
