package root

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/iostreams"
)

// run executes bb with args against a fresh command tree and returns what was
// written to stdout and the error. The args slice passed to cobra is never
// nil: cobra reads os.Args when SetArgs(nil) is used.
func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	io, _, out, _ := iostreams.Test()
	cmd := NewCmdRoot(&cmdutil.Factory{IOStreams: io})
	cmd.SetOut(out)
	cmd.SetArgs(append([]string{}, args...))
	err := cmd.Execute()
	return out.String(), err
}

// groups returns the argument path of every command that has subcommands,
// the root included as the empty path, so that new groups are covered
// automatically.
func groups(t *testing.T) [][]string {
	t.Helper()
	io, _, _, _ := iostreams.Test()
	var paths [][]string
	var walk func(c *cobra.Command, path []string)
	walk = func(c *cobra.Command, path []string) {
		if c.HasAvailableSubCommands() {
			paths = append(paths, path)
		}
		for _, sub := range c.Commands() {
			if sub.IsAvailableCommand() {
				walk(sub, append(append([]string{}, path...), sub.Name()))
			}
		}
	}
	walk(NewCmdRoot(&cmdutil.Factory{IOStreams: io}), nil)
	if len(paths) < 9 {
		t.Fatalf("expected the root and at least 8 command groups, got %v", paths)
	}
	return paths
}

func TestRootHelp(t *testing.T) {
	out, err := run(t, "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Bitbucket Cloud") {
		t.Errorf("help missing description: %s", out)
	}
}

func TestVersionCommand(t *testing.T) {
	out, err := run(t, "version")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, "bb version ") {
		t.Errorf("unexpected: %q", out)
	}
}

func TestRootVersionFlag(t *testing.T) {
	if out, err := run(t, "--version"); err != nil || !strings.HasPrefix(out, "bb version ") {
		t.Errorf("--version: err=%v out=%q", err, out)
	}
}

func TestUnknownSubcommandFails(t *testing.T) {
	for _, path := range groups(t) {
		args := append(append([]string{}, path...), "bogus")
		if _, err := run(t, args...); err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Errorf("%v: expected unknown command error, got %v", args, err)
		}
	}
}

func TestUnknownSubcommandSuggests(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"pr", "merg"}, "merge"},   // prefix
		{[]string{"pr", "mrege"}, "merge"},  // transposition (Levenshtein distance 2)
		{[]string{"auth", "logn"}, "login"}, // omission
		{[]string{"repoo"}, "repo"},         // root level
	}
	for _, tc := range cases {
		_, err := run(t, tc.args...)
		if err == nil || !strings.Contains(err.Error(), "Did you mean") || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%v: expected a suggestion of %q, got %v", tc.args, tc.want, err)
		}
	}
	// An empty argument is a prefix of every name; it must not list them all.
	if _, err := run(t, "pr", ""); err == nil || strings.Contains(err.Error(), "Did you mean") {
		t.Errorf(`pr "": expected an error without suggestions, got %v`, err)
	}
}

func TestUnknownRootCommandIsCheckedBeforeFlags(t *testing.T) {
	// cobra validates the root's first argument before parsing flags, so a
	// typo is reported even when --help/--version or a subcommand's flags follow.
	for _, args := range [][]string{{"bogus", "--help"}, {"bogus", "--version"}, {"rpeo", "list", "-R", "ws/repo"}, {"bogus", "--json", "x"}} {
		if _, err := run(t, args...); err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Errorf("%v: expected unknown command error, got %v", args, err)
		}
	}
	// An empty first argument is skipped, as before: help, exit 0.
	if out, err := run(t, ""); err != nil || !strings.Contains(out, "Available Commands") {
		t.Errorf(`bb "": expected help, got err=%v out=%q`, err, out)
	}
}

func TestHelpCommand(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"help", "pr"}, {"help", "pr", "merge"}} {
		out, err := run(t, args...)
		if err != nil || !strings.Contains(out, "Usage") {
			t.Errorf("%v: err=%v out=%q", args, err, out)
		}
	}
	// help is usable but not advertised in the command list.
	if out, _ := run(t, "--help"); strings.Contains(out, "Help about any command") {
		t.Errorf("help must stay hidden from the command list: %q", out)
	}
	// unknown help topics are usage errors (exit 1), like unknown commands.
	for _, args := range [][]string{{"help", "bogus"}, {"help", "pr", "bogus"}} {
		_, err := run(t, args...)
		var flagErr *cmdutil.FlagError
		if !errors.As(err, &flagErr) || !strings.Contains(err.Error(), "unknown help topic") {
			t.Errorf("%v: expected usage error, got %v", args, err)
		}
	}
}

func TestGroupsWithoutArgsShowHelp(t *testing.T) {
	for _, path := range groups(t) {
		out, err := run(t, path...)
		if err != nil {
			t.Errorf("%v: expected help (exit 0), got %v", path, err)
			continue
		}
		if !strings.Contains(out, "Available Commands") {
			t.Errorf("%v: help not printed: %q", path, out)
		}
	}
}

func TestGroupsHelpFlag(t *testing.T) {
	for _, path := range groups(t) {
		args := append(append([]string{}, path...), "--help")
		if out, err := run(t, args...); err != nil || !strings.Contains(out, "Usage") {
			t.Errorf("%v: err=%v out=%q", args, err, out)
		}
	}
}
