package cmdutil

import (
	"runtime"
	"testing"

	"github.com/uehatsu/bb/internal/config"
	"github.com/uehatsu/bb/internal/iostreams"
)

func TestOpenBrowserAllowlist(t *testing.T) {
	noop := "true"
	if runtime.GOOS == "windows" {
		noop = "cmd /c exit 0"
	}
	t.Setenv("BROWSER", noop) // never launch a real browser from tests // /usr/bin/true: succeeds without opening anything
	ios, _, _, errOut := iostreams.Test()
	ios.SetStdoutTTY(true)
	cfg, _ := config.LoadFrom(t.TempDir())
	f := &Factory{IOStreams: ios, Config: func() (*config.Config, error) { return cfg, nil }}
	if err := OpenBrowser(f, "https://bitbucket.org/acme/widgets"); err != nil {
		t.Errorf("bitbucket url: %v", err)
	}
	if err := OpenBrowser(f, "https://API.Bitbucket.org/x"); err != nil {
		t.Errorf("subdomain, mixed case: %v", err)
	}
	for _, bad := range []string{"https://evil.example.com/", "http://bitbucket.org/", "https://bitbucket.org.evil.com/", "file:///etc/passwd"} {
		if err := OpenBrowser(f, bad); err == nil {
			t.Errorf("%s must be refused", bad)
		}
	}
	if errOut.Len() == 0 {
		t.Error("TTY should print the Opening… hint")
	}
}
