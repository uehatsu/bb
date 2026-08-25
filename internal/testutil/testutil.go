// Package testutil provides a stubbed Factory backed by an httptest server.
package testutil

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/config"
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
}

// NewHarness returns a Harness whose API client points at an httptest
// server. Register handlers on h.Mux (paths are prefixed with /2.0).
func NewHarness(t *testing.T) *Harness {
	t.Helper()
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
		BaseRepo: func() (cmdutil.Repo, error) { return cmdutil.Repo{Workspace: "acme", Slug: "widgets"}, nil },
		Prompter: stub,
	}
	return &Harness{Factory: f, Config: cfg, In: in, Out: out, ErrOut: errOut, Server: srv, Mux: mux, Prompt: stub}
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

// SetTTY marks stdout/stdin as a terminal.
func (h *Harness) SetTTY(tty bool) {
	h.Factory.IOStreams.SetStdoutTTY(tty)
	h.Factory.IOStreams.SetStdinTTY(tty)
}
