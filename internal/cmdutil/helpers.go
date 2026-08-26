package cmdutil

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/prompt"
)

// OptionalArg returns args[0] or "" — the ubiquitous optional selector.
func OptionalArg(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return ""
}

// PromptError maps prompt outcomes to command errors: a nil error or a
// user abort becomes ErrCancel (exit 2); anything else is passed through.
func PromptError(err error) error {
	if err == nil || errors.Is(err, prompt.ErrCancelled) {
		return ErrCancel
	}
	return err
}

// MainBranch returns the repository's main branch name.
func MainBranch(ctx context.Context, client *api.Client, repo Repo) (string, error) {
	var r bitbucket.Repository
	if _, err := client.Do(ctx, api.Request{Path: fmt.Sprintf("/repositories/%s/%s", repo.Workspace, repo.Slug), Query: map[string][]string{"fields": {"mainbranch.name"}}}, &r); err != nil {
		return "", err
	}
	if r.MainBranch == nil || r.MainBranch.Name == "" {
		return "", fmt.Errorf("repository %s has no main branch", repo.FullName())
	}
	return r.MainBranch.Name, nil
}

// GroupRunE is the RunE for command groups (auth, pr, repo, ...): with no
// arguments it prints help; with an unknown subcommand it fails with a usage
// error (exit 1) instead of cobra's default of printing help and exiting 0.
// The "Did you mean" hint mirrors cobra's unexported findSuggestions, which
// is what the root command gets from cobra itself.
func GroupRunE(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	suggest := ""
	if !cmd.DisableSuggestions && args[0] != "" {
		if cmd.SuggestionsMinimumDistance <= 0 {
			cmd.SuggestionsMinimumDistance = 2
		}
		if s := cmd.SuggestionsFor(args[0]); len(s) > 0 {
			suggest = "\n\nDid you mean this?\n\t" + strings.Join(s, "\n\t")
		}
	}
	return FlagErrorf("unknown command %q for %q%s", args[0], cmd.CommandPath(), suggest)
}
