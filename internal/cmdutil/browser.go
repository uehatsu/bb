package cmdutil

import (
	"fmt"

	"github.com/uehatsu/bb/internal/browser"
)

// OpenBrowser opens u in the user's browser, honoring the configured command.
func OpenBrowser(f *Factory, u string) error {
	if f.IOStreams.IsStdoutTTY() {
		fmt.Fprintf(f.IOStreams.ErrOut, "Opening %s in your browser.\n", u)
	}
	configured := ""
	if f.Config != nil {
		if cfg, err := f.Config(); err == nil {
			configured, _ = cfg.Get("browser")
		}
	}
	return browser.New(configured).Browse(u)
}
