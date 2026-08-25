package cmdutil

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/gitctx"
)

// EnableRepoOverride adds the -R/--repo flag to cmd (persistent) and makes
// f.BaseRepo honor it. Resolution order: --repo, BB_REPO, git remotes.
func EnableRepoOverride(cmd *cobra.Command, f *Factory) {
	cmd.PersistentFlags().StringP("repo", "R", "", "Select another repository using the WORKSPACE/REPO format")
	_ = cmd.RegisterFlagCompletionFunc("repo", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveNoFileComp
	})
	base := f.BaseRepo
	f.BaseRepo = func() (Repo, error) {
		if v, _ := cmd.PersistentFlags().GetString("repo"); v != "" {
			return repoFromArg(f, v)
		}
		if v := os.Getenv("BB_REPO"); v != "" {
			return repoFromArg(f, v)
		}
		if base != nil {
			return base()
		}
		return RepoFromRemotes(f)
	}
}

func repoFromArg(f *Factory, arg string) (Repo, error) {
	ws := ""
	if cfg, err := f.Config(); err == nil {
		ws, _ = cfg.Get("workspace")
	}
	r, err := gitctx.ParseRepoArg(arg, ws)
	if err != nil {
		return Repo{}, err
	}
	return Repo{Workspace: r.Workspace, Slug: r.Slug}, nil
}

// RepoFromRemotes resolves the repository from the current git remotes.
func RepoFromRemotes(f *Factory) (Repo, error) {
	if f.GitClient == nil {
		return Repo{}, gitctx.ErrNoRemote
	}
	g, err := f.GitClient()
	if err != nil {
		return Repo{}, fmt.Errorf("%w (%v)", gitctx.ErrNoRemote, err)
	}
	remotes, err := g.Remotes(context.Background())
	if err != nil {
		return Repo{}, fmt.Errorf("%w (%v)", gitctx.ErrNoRemote, err)
	}
	var parsed []gitctx.Remote
	for _, r := range remotes {
		if repo, ok := gitctx.ParseRemoteURL(r.URL); ok {
			parsed = append(parsed, gitctx.Remote{Name: r.Name, Repo: repo})
		}
	}
	best, err := gitctx.PickRemote(parsed)
	if err != nil {
		return Repo{}, err
	}
	return Repo{Workspace: best.Repo.Workspace, Slug: best.Repo.Slug}, nil
}

// DefaultWorkspace returns the configured default workspace ("" if unset).
func DefaultWorkspace(f *Factory) string {
	cfg, err := f.Config()
	if err != nil {
		return ""
	}
	ws, _ := cfg.Get("workspace")
	return ws
}
