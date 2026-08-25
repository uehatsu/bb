package pr

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/gitctx"
)

// NewCmdCheckout returns `pr checkout`.
func NewCmdCheckout(f *cmdutil.Factory) *cobra.Command {
	var branchName string
	var force, detach bool
	cmd := &cobra.Command{
		Use:   "checkout {<number> | <branch> | <url>}",
		Short: "Check out a pull request in git",
		Long: `Fetch the pull request's source branch and check it out locally.

For pull requests from forks, a remote named after the fork's workspace is
added and the branch is fetched from it.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCheckout(f, args[0], branchName, force, detach)
		},
	}
	cmd.Flags().StringVarP(&branchName, "branch", "b", "", "Local branch name to use (default: the source branch name)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Reset the existing local branch to the latest state of the pull request")
	cmd.Flags().BoolVar(&detach, "detach", false, "Checkout PR with a detached HEAD")
	return cmd
}

func runCheckout(f *cmdutil.Factory, selector, localBranch string, force, detach bool) error {
	ctx := context.Background()
	repo, err := f.BaseRepo()
	if err != nil {
		return err
	}
	client, err := f.APIClient()
	if err != nil {
		return err
	}
	pr, err := resolvePR(ctx, f, client, repo, selector)
	if err != nil {
		return err
	}
	g, err := f.GitClient()
	if err != nil {
		return err
	}
	protocol := "https"
	if cfg, err := f.Config(); err == nil {
		protocol, _ = cfg.Get("git_protocol")
	}

	srcBranch := pr.Source.Branch.Name
	remote := "origin"
	isFork := pr.Source.Repository != nil && pr.Destination.Repository != nil && pr.Source.Repository.FullName != pr.Destination.Repository.FullName
	if isFork {
		remote = pr.Source.Repository.Workspace.Slug
		if remote == "" {
			remote = "fork-" + fmt.Sprint(pr.ID)
		}
		if existing, _ := g.ConfigGet(ctx, "", "remote."+remote+".url"); existing == "" {
			if _, err := g.Output(ctx, "remote", "add", "--", remote, cloneURL(pr.Source.Repository, protocol)); err != nil {
				return err
			}
		}
	} else {
		remotes, err := g.Remotes(ctx)
		if err == nil {
			for _, r := range remotes {
				if parsed, ok := gitctx.ParseRemoteURL(r.URL); ok && parsed.Workspace == repo.Workspace && parsed.Slug == repo.Slug {
					remote = r.Name
					break
				}
			}
		}
	}

	if localBranch == "" {
		localBranch = srcBranch
	}
	if _, err := g.Output(ctx, "fetch", "--", remote, "+refs/heads/"+srcBranch+":refs/remotes/"+remote+"/"+srcBranch); err != nil {
		return err
	}
	if detach {
		return g.Run(ctx, "checkout", "--detach", "--", remote+"/"+srcBranch)
	}
	if exists, _ := g.ConfigGet(ctx, "", "branch."+localBranch+".merge"); exists != "" || branchExists(ctx, g, localBranch) {
		if err := g.Run(ctx, "checkout", "--", localBranch); err != nil {
			return err
		}
		if force {
			return g.Run(ctx, "reset", "--hard", "--", remote+"/"+srcBranch)
		}
		return g.Run(ctx, "merge", "--ff-only", "--", remote+"/"+srcBranch)
	}
	if err := g.Run(ctx, "checkout", "-b", localBranch, "--track", "--", remote+"/"+srcBranch); err != nil {
		return err
	}
	return nil
}

func branchExists(ctx context.Context, g interface {
	Output(context.Context, ...string) (string, error)
}, name string) bool {
	_, err := g.Output(ctx, "rev-parse", "--verify", "--quiet", "refs/heads/"+name)
	return err == nil
}

func cloneURL(r *bitbucket.Repository, protocol string) string {
	if protocol == "ssh" {
		return fmt.Sprintf("git@%s:%s.git", gitctx.SSHHost, r.FullName)
	}
	return "https://bitbucket.org/" + r.FullName + ".git"
}
