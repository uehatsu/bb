package cmdutil

import (
	"net/http"

	"github.com/uehatsu/bb/internal/iostreams"
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

	// HTTPClient returns an authenticated http.Client for api.bitbucket.org.
	HTTPClient func() (*http.Client, error)
	// BaseRepo resolves the target repository from -R, BB_REPO, or git remotes.
	BaseRepo func() (Repo, error)
	// Config returns the loaded configuration.
	Config func() (Config, error)
}

// Config is the minimal configuration surface commands depend on. The
// concrete implementation lives in internal/config.
type Config interface {
	Get(key string) (string, error)
	Set(key, value string) error
	Keys() []string
	Write() error
}
