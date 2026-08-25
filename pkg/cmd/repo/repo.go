// Package repo implements `bb repo` subcommands.
package repo

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/gitctx"
)

// NewCmdRepo returns the `repo` command group.
func NewCmdRepo(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo <command>",
		Short: "Manage repositories",
		Long:  "Work with Bitbucket repositories.",
		Example: `  $ bb repo list acme
  $ bb repo view acme/widgets
  $ bb repo create widgets --workspace acme --private
  $ bb repo clone acme/widgets`,
	}
	cmd.AddCommand(
		NewCmdList(f),
		NewCmdView(f),
		NewCmdCreate(f),
		NewCmdClone(f),
		NewCmdFork(f),
		NewCmdDelete(f),
		NewCmdEdit(f),
	)
	return cmd
}

// resolveRepoArg returns the repo from an optional positional argument or
// the base repository.
func resolveRepoArg(f *cmdutil.Factory, arg string) (cmdutil.Repo, error) {
	if arg == "" {
		return f.BaseRepo()
	}
	r, err := gitctx.ParseRepoArg(arg, cmdutil.DefaultWorkspace(f))
	if err != nil {
		return cmdutil.Repo{}, err
	}
	return cmdutil.Repo{Workspace: r.Workspace, Slug: r.Slug}, nil
}

const repoListFields = "values.name,values.slug,values.full_name,values.description,values.is_private,values.language,values.updated_on,values.created_on,values.size,values.fork_policy,values.uuid,values.mainbranch.name,values.project.key,values.project.name,values.workspace.slug,values.links.html.href,values.links.clone,values.parent.full_name,next"

func fetchRepo(ctx context.Context, client *api.Client, repo cmdutil.Repo) (*bitbucket.Repository, error) {
	var r bitbucket.Repository
	if _, err := client.Do(ctx, api.Request{Path: fmt.Sprintf("/repositories/%s/%s", repo.Workspace, repo.Slug)}, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// cloneURLFor picks the clone URL for the configured protocol. Server
// supplied links are validated to point at Bitbucket and normalized
// (embedded usernames dropped, SSH moved to ssh.bitbucket.org); the
// repository's full_name is the fallback.
func cloneURLFor(r *bitbucket.Repository, protocol string) (string, error) {
	if href := r.CloneURL(protocol); href != "" {
		if u, ok := gitctx.NormalizeCloneURL(href, protocol); ok {
			return u, nil
		}
	}
	if u, ok := gitctx.NormalizeCloneURL("https://bitbucket.org/"+r.FullName, protocol); ok {
		return u, nil
	}
	return "", fmt.Errorf("repository %q has no usable clone URL", r.FullName)
}

func visibility(r *bitbucket.Repository) string {
	if r.IsPrivate {
		return "private"
	}
	return "public"
}
