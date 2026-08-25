package cmdutil

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/uehatsu/bb/internal/browser"
)

// OpenBrowser opens u in the user's browser, honoring the configured command.
// Only bitbucket.org (and subdomains) URLs are opened, so that a tampered API
// response can never launch the browser at an arbitrary site.
func OpenBrowser(f *Factory, u string) error {
	parsed, err := url.Parse(u)
	if err != nil || parsed.Scheme != "https" || !isBitbucketWebHost(parsed.Hostname()) {
		return fmt.Errorf("refusing to open non-Bitbucket URL %q", u)
	}
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

func isBitbucketWebHost(h string) bool {
	return h == "bitbucket.org" || strings.HasSuffix(h, ".bitbucket.org")
}
