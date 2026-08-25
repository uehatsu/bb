package cmdutil

import (
	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/config"
	"github.com/uehatsu/bb/internal/git"
	"github.com/uehatsu/bb/internal/iostreams"
	"github.com/uehatsu/bb/internal/prompt"
)

// Repo identifies a Bitbucket repository by workspace and slug.
type Repo struct {
	Workspace string
	Slug      string
}

// FullName returns "workspace/slug".
func (r Repo) FullName() string { return r.Workspace + "/" + r.Slug }

// Factory wires dependencies into commands lazily so that tests can stub
// any of them. It mirrors GitHub CLI's cmdutil.Factory.
type Factory struct {
	IOStreams  *iostreams.IOStreams
	Executable string

	// Config returns the loaded configuration (cached).
	Config func() (*config.Config, error)
	// APIClient returns an authenticated Bitbucket API client. It returns an
	// *AuthError when no credential is available.
	APIClient func() (*api.Client, error)
	// BaseRepo resolves the target repository from -R, BB_REPO, or git remotes.
	BaseRepo func() (Repo, error)
	// GitClient runs git commands.
	GitClient func() (*git.Client, error)
	// Prompter asks interactive questions.
	Prompter prompt.Prompter
}
