// Package factory constructs the production cmdutil.Factory.
package factory

import (
	"os"

	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/iostreams"
)

// New returns a Factory wired to the real environment. Dependencies that
// need configuration (HTTP client, base repo) are attached in later steps.
func New() *cmdutil.Factory {
	io := iostreams.System()
	if p := os.Getenv("BB_PAGER"); p != "" {
		io.SetPager(p)
	} else if p := os.Getenv("PAGER"); p != "" {
		io.SetPager(p)
	}
	exe, _ := os.Executable()
	return &cmdutil.Factory{
		IOStreams:  io,
		Executable: exe,
	}
}
