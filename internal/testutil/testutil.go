// Package testutil provides a stubbed Factory backed by an httptest server.
package testutil

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/config"
	"github.com/uehatsu/bb/internal/git"
	"github.com/uehatsu/bb/internal/iostreams"
	"github.com/uehatsu/bb/internal/prompt"
)

// Harness bundles everything a command test needs.
type Harness struct {
	Factory *cmdutil.Factory
	Config  *config.Config
	In      *bytes.Buffer
	Out     *bytes.Buffer
	ErrOut  *bytes.Buffer
	Server  *httptest.Server
	Mux     *http.ServeMux
	Prompt  *prompt.Stub
	Git     *git.Stub
}

// NewHarness returns a Harness whose API client points at an httptest
// server. Register handlers on h.Mux (paths are prefixed with /2.0).
func NewHarness(t *testing.T) *Harness {
	t.Helper()
	IsolateEnv(t)
	InstantPolling(t)
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	ios, in, out, errOut := iostreams.Test()
	cfg, err := config.LoadFrom(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	stub := &prompt.Stub{}
	f := &cmdutil.Factory{
		IOStreams: ios,
		Config:    func() (*config.Config, error) { return cfg, nil },
		APIClient: func() (*api.Client, error) {
			return api.NewClient(api.NewAuthenticator(config.Credential{Method: config.AuthBearer, Token: "t"}), api.WithBaseURL(srv.URL+"/2.0"), api.WithNoRetry(true)), nil
		},
		BaseRepo:  func() (cmdutil.Repo, error) { return cmdutil.Repo{Workspace: "acme", Slug: "widgets"}, nil },
		GitClient: func() (git.Runner, error) { return nil, errors.New("git not available in tests") },
		Prompter:  stub,
	}
	return &Harness{Factory: f, Config: cfg, In: in, Out: out, ErrOut: errOut, Server: srv, Mux: mux, Prompt: stub}
}

// InstantPolling makes api.Poll skip its sleeps for the duration of the test
// (ctx cancellation is still honored).
func InstantPolling(t *testing.T) {
	t.Helper()
	orig := api.PollSleep
	api.PollSleep = func(ctx context.Context, _ time.Duration) error { return ctx.Err() }
	t.Cleanup(func() { api.PollSleep = orig })
}

// NoopBrowser returns a BROWSER command that accepts a URL and does nothing.
func NoopBrowser() string {
	if runtime.GOOS == "windows" {
		return "cmd /c exit 0"
	}
	return "true"
}

// IsolateEnv clears every bb-related environment variable so tests never
// touch the developer's real credentials, keyring, or browser.
func IsolateEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{"BB_TOKEN", "BB_EMAIL", "BB_AUTH_METHOD", "BB_CREDENTIAL_STORE", "BB_REPO", "BB_WORKSPACE", "BB_CONFIG_DIR", "BB_OAUTH_CLIENT_ID", "BB_OAUTH_CLIENT_SECRET", "BB_DEBUG", "BB_NO_RETRY", "BB_PAGER", "PAGER"} {
		t.Setenv(k, "")
	}
	t.Setenv("BB_CONFIG_DIR", t.TempDir())
	// Never launch a real browser from tests: point BROWSER at a command that
	// succeeds without doing anything.
	t.Setenv("BROWSER", NoopBrowser())
}

// JSON registers a handler returning body for method+path.
func (h *Harness) JSON(method, path string, status int, body string) {
	h.Mux.HandleFunc("/2.0"+path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.WriteHeader(405)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
}

// Handle registers an arbitrary handler under /2.0+path.
func (h *Harness) Handle(path string, fn http.HandlerFunc) {
	h.Mux.HandleFunc("/2.0"+path, fn)
}

// UseGit installs a git stub and returns it.
func (h *Harness) UseGit() *git.Stub {
	h.Git = git.NewStub()
	h.Factory.GitClient = func() (git.Runner, error) { return h.Git, nil }
	return h.Git
}

// SetTTY marks stdout/stdin as a terminal.
func (h *Harness) SetTTY(tty bool) {
	h.Factory.IOStreams.SetStdoutTTY(tty)
	h.Factory.IOStreams.SetStdinTTY(tty)
}
