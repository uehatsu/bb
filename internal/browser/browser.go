// Package browser opens URLs in the user's web browser.
package browser

import (
	"errors"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Browser launches URLs.
type Browser struct {
	// Command overrides the launcher (BROWSER env / config "browser").
	Command string
	// Run executes the command; replaced in tests.
	Run func(name string, args ...string) error
}

// New returns a Browser honoring the BROWSER environment variable and the
// optional configured command.
func New(configured string) *Browser {
	cmd := os.Getenv("BROWSER")
	if cmd == "" {
		cmd = configured
	}
	return &Browser{Command: cmd, Run: func(name string, args ...string) error {
		c := exec.Command(name, args...)
		c.Stdout, c.Stderr = os.Stdout, os.Stderr
		return c.Run()
	}}
}

// Browse opens rawURL after validating it is an http(s) URL.
func (b *Browser) Browse(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
		return errors.New("refusing to open non-http(s) URL")
	}
	if b.Command != "" {
		parts := strings.Fields(b.Command)
		return b.Run(parts[0], append(parts[1:], rawURL)...)
	}
	switch runtime.GOOS {
	case "darwin":
		return b.Run("open", rawURL)
	case "windows":
		return b.Run("rundll32", "url.dll,FileProtocolHandler", rawURL)
	default:
		return b.Run("xdg-open", rawURL)
	}
}
