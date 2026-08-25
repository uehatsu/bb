// Package factory constructs the production cmdutil.Factory.
package factory

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/config"
	"github.com/uehatsu/bb/internal/git"
	"github.com/uehatsu/bb/internal/iostreams"
	"github.com/uehatsu/bb/internal/prompt"
)

// New returns a Factory wired to the real environment.
func New() *cmdutil.Factory {
	io := iostreams.System()
	exe, _ := os.Executable()
	f := &cmdutil.Factory{
		IOStreams:  io,
		Executable: exe,
		Prompter:   &prompt.Huh{In: os.Stdin, Out: os.Stderr},
	}
	f.Config = configFunc()
	f.APIClient = apiClientFunc(f)
	f.GitClient = func() (git.Runner, error) { return git.New("") }
	f.BaseRepo = func() (cmdutil.Repo, error) { return cmdutil.RepoFromRemotes(f) }

	applyPager(f, io)
	return f
}

func configFunc() func() (*config.Config, error) {
	var once sync.Once
	var cfg *config.Config
	var err error
	return func() (*config.Config, error) {
		once.Do(func() { cfg, err = config.Load() })
		return cfg, err
	}
}

func apiClientFunc(f *cmdutil.Factory) func() (*api.Client, error) {
	var once sync.Once
	var client *api.Client
	var err error
	return func() (*api.Client, error) {
		once.Do(func() {
			cfg, cerr := f.Config()
			if cerr != nil {
				err = cerr
				return
			}
			cred, rerr := config.ResolveFreshCredential(context.Background(), cfg.Credentials(), config.DefaultHost, os.Getenv, time.Now())
			var refreshErr *config.ErrRefreshFailed
			switch {
			case errors.Is(rerr, config.ErrNotFound):
				err = cmdutil.NewAuthError("not logged in to bitbucket.org")
				return
			case errors.As(rerr, &refreshErr):
				err = &cmdutil.AuthError{Msg: rerr.Error()}
				return
			case rerr != nil:
				// token refreshed but not persisted: usable for this run
				fmt.Fprintf(f.IOStreams.ErrOut, "warning: %v\n", rerr)
			}
			var opts []api.Option
			switch os.Getenv("BB_DEBUG") {
			case "1", "true":
				opts = append(opts, api.WithLogger(f.IOStreams.ErrOut, false))
			case "2":
				opts = append(opts, api.WithLogger(f.IOStreams.ErrOut, true))
			}
			client = api.NewClient(api.NewAuthenticator(cred), opts...)
		})
		return client, err
	}
}

func applyPager(f *cmdutil.Factory, io *iostreams.IOStreams) {
	if p := os.Getenv("BB_PAGER"); p != "" {
		io.SetPager(p)
		return
	}
	if cfg, err := f.Config(); err == nil {
		if p, _ := cfg.Get("pager"); p != "" {
			io.SetPager(p)
			return
		}
	}
	if p := os.Getenv("PAGER"); p != "" {
		io.SetPager(p)
	}
}
