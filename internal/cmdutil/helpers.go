package cmdutil

import (
	"context"
	"errors"
	"fmt"

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
