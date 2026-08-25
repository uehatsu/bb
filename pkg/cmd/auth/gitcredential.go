package auth

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/config"
)

// NewCmdGitCredential implements the git credential helper protocol.
func NewCmdGitCredential(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:    "git-credential <get|store|erase>",
		Short:  "Git credential helper (internal)",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := f.Config()
			if err != nil {
				return err
			}
			return gitCredential(args[0], f.IOStreams.In, f.IOStreams.Out, func() (config.Credential, error) {
				return config.ResolveFreshCredential(cmd.Context(), cfg.Credentials(), config.DefaultHost, os.Getenv, time.Now())
			})
		},
	}
}

// gitCredential handles one helper invocation. Only "get" produces output,
// and only for https://bitbucket.org; "store" and "erase" are no-ops so git
// can never modify or delete bb's stored token.
func gitCredential(op string, in io.Reader, out io.Writer, resolve func() (config.Credential, error)) error {
	attrs, err := parseCredentialInput(in)
	if err != nil {
		return err
	}
	if op != "get" {
		return nil
	}
	if attrs["protocol"] != "https" || !isBitbucketHost(attrs["host"]) {
		return nil // not ours; let git try the next helper
	}
	cred, err := resolve()
	if errors.Is(err, config.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	// If git already asked for a specific username other than ours, stay silent.
	if u := attrs["username"]; u != "" && u != cred.GitUsername() {
		return nil
	}
	fmt.Fprintf(out, "protocol=https\nhost=bitbucket.org\nusername=%s\npassword=%s\n", cred.GitUsername(), cred.Token)
	return nil
}

func isBitbucketHost(h string) bool {
	h = strings.ToLower(h)
	return h == "bitbucket.org" || h == "bitbucket.org:443"
}

func parseCredentialInput(in io.Reader) (map[string]string, error) {
	attrs := map[string]string{}
	sc := bufio.NewScanner(in)
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			break
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		attrs[k] = v
	}
	return attrs, sc.Err()
}
