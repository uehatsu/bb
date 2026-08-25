// Package repo implements `bb repo` subcommands.
package repo

import (
	"context"
	"fmt"
	"strings"

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

// sshCloneURL rewrites a Bitbucket SSH clone link to the ssh.bitbucket.org host.
func sshCloneURL(href string) string {
	href = strings.Replace(href, "git@bitbucket.org:", "git@"+gitctx.SSHHost+":", 1)
	href = strings.Replace(href, "ssh://git@bitbucket.org/", "ssh://git@"+gitctx.SSHHost+"/", 1)
	return href
}

// cloneURLFor picks the clone URL for the configured protocol.
func cloneURLFor(r *bitbucket.Repository, protocol string) string {
	if protocol == "ssh" {
		if u := r.CloneURL("ssh"); u != "" {
			return sshCloneURL(u)
		}
		return fmt.Sprintf("git@%s:%s.git", gitctx.SSHHost, r.FullName)
	}
	if u := r.CloneURL("https"); u != "" {
		// strip any embedded username so the credential helper is used
		if i := strings.Index(u, "@"); i > 0 && strings.HasPrefix(u, "https://") {
			return "https://" + u[i+1:]
		}
		return u
	}
	return "https://bitbucket.org/" + r.FullName + ".git"
}

func visibility(r *bitbucket.Repository) string {
	if r.IsPrivate {
		return "private"
	}
	return "public"
}
