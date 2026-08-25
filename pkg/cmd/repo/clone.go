package repo

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/gitctx"
)

// NewCmdClone returns `repo clone`.
func NewCmdClone(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clone <repository> [<directory>] [-- <gitflags>...]",
		Short: "Clone a repository locally",
		Long: `Clone a Bitbucket repository locally. Pass additional git clone flags
after "--".

The protocol is taken from the git_protocol config (https by default). SSH
clones use ssh.bitbucket.org. When cloning a fork, an "upstream" remote
pointing at the parent repository is added.`,
		Example: `  $ bb repo clone acme/widgets
  $ bb repo clone widgets            # uses the default workspace
  $ bb repo clone acme/widgets mydir -- --depth 1`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repo, err := gitctx.ParseRepoArg(args[0], cmdutil.DefaultWorkspace(f))
			if err != nil {
				return err
			}
			dir := ""
			var gitArgs []string
			rest := args[1:]
			if len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
				dir, rest = rest[0], rest[1:]
			}
			gitArgs = rest
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			r, err := fetchRepo(cmd.Context(), client, cmdutil.Repo{Workspace: repo.Workspace, Slug: repo.Slug})
			if err != nil {
				return err
			}
			return cloneRepo(cmd.Context(), f, r, dir, gitArgs)
		},
	}
	cmd.Flags().SetInterspersed(false)
	return cmd
}

func cloneRepo(ctx context.Context, f *cmdutil.Factory, r *bitbucket.Repository, dir string, gitArgs []string) error {
	protocol := "https"
	if cfg, err := f.Config(); err == nil {
		protocol, _ = cfg.Get("git_protocol")
	}
	u := cloneURLFor(r, protocol)
	g, err := f.GitClient()
	if err != nil {
		return err
	}
	args := append([]string{"clone"}, gitArgs...)
	args = append(args, "--", u)
	if dir != "" {
		args = append(args, dir)
	}
	if err := g.Run(ctx, args...); err != nil {
		return err
	}
	if r.Parent != nil {
		target := dir
		if target == "" {
			target = path.Base(strings.TrimSuffix(u, ".git"))
		}
		parentURL := "https://bitbucket.org/" + r.Parent.FullName + ".git"
		if protocol == "ssh" {
			parentURL = fmt.Sprintf("git@%s:%s.git", gitctx.SSHHost, r.Parent.FullName)
		}
		sub := *g
		sub.Dir = target
		if _, err := sub.Output(ctx, "remote", "add", "upstream", parentURL); err != nil {
			fmt.Fprintf(f.IOStreams.ErrOut, "warning: could not add upstream remote: %v\n", err)
		}
	}
	return nil
}
