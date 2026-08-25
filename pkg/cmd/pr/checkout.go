package pr

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/git"
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
			return runCheckout(cmd.Context(), f, args[0], branchName, force, detach)
		},
	}
	cmd.Flags().StringVarP(&branchName, "branch", "b", "", "Local branch name to use (default: the source branch name)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Reset the existing local branch to the latest state of the pull request")
	cmd.Flags().BoolVar(&detach, "detach", false, "Checkout PR with a detached HEAD")
	return cmd
}

func runCheckout(ctx context.Context, f *cmdutil.Factory, selector, localBranch string, force, detach bool) error {
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
	if srcBranch == "" || strings.HasPrefix(srcBranch, "-") {
		return fmt.Errorf("invalid source branch %q", srcBranch)
	}
	remote := "origin"
	isFork := pr.Source.Repository != nil && pr.Source.Repository.FullName != "" && pr.Source.Repository.FullName != repo.FullName()
	if isFork {
		// The PR's source.repository is a short reference without a
		// workspace object; derive the workspace from full_name.
		forkRepo, ok := gitctx.ParseRemoteURL("https://bitbucket.org/" + pr.Source.Repository.FullName)
		if !ok {
			return fmt.Errorf("unexpected fork repository name %q", pr.Source.Repository.FullName)
		}
		remote = forkRepo.Workspace
		if existing, _ := g.ConfigGet(ctx, "", "remote."+remote+".url"); existing == "" {
			if _, err := g.Output(ctx, "remote", "add", remote, gitctx.CloneURL(forkRepo, protocol)); err != nil {
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
	if strings.HasPrefix(localBranch, "-") {
		return fmt.Errorf("invalid branch name %q", localBranch)
	}
	// Note: these subcommands take a commit-ish, not a pathspec, so "--" must
	// NOT precede the ref (it would turn the ref into a path argument).
	trackingRef := remote + "/" + srcBranch
	if _, err := g.Output(ctx, "fetch", remote, "+refs/heads/"+srcBranch+":refs/remotes/"+trackingRef); err != nil {
		return err
	}
	if detach {
		return g.Run(ctx, "checkout", "--detach", trackingRef)
	}
	if branchExists(ctx, g, localBranch) {
		if err := g.Run(ctx, "checkout", localBranch); err != nil {
			return err
		}
		if force {
			return g.Run(ctx, "reset", "--hard", trackingRef)
		}
		return g.Run(ctx, "merge", "--ff-only", trackingRef)
	}
	return g.Run(ctx, "checkout", "-b", localBranch, "--track", trackingRef)
}

func branchExists(ctx context.Context, g git.Runner, name string) bool {
	_, err := g.Output(ctx, "rev-parse", "--verify", "--quiet", "refs/heads/"+name)
	return err == nil
}
