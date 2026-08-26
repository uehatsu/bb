package root

import (
	"strings"
	"testing"

	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/iostreams"
)

func TestRootHelp(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: io}
	cmd := NewCmdRoot(f)
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Bitbucket Cloud") {
		t.Errorf("help missing description: %s", out.String())
	}
}

func TestVersionCommand(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	cmd := NewCmdRoot(&cmdutil.Factory{IOStreams: io})
	cmd.SetArgs([]string{"version"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.String(), "bb version ") {
		t.Errorf("unexpected: %q", out.String())
	}
}

func TestUnknownSubcommandFails(t *testing.T) {
	for _, args := range [][]string{{"bogus"}, {"pipeline", "nonsense"}, {"pr", "nonsense"}, {"repo", "nonsense"}, {"auth", "nonsense"}, {"branch", "x"}, {"workspace", "x"}, {"project", "x"}, {"config", "x"}} {
		io, _, _, _ := iostreams.Test()
		cmd := NewCmdRoot(&cmdutil.Factory{IOStreams: io})
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Errorf("%v: expected unknown command error, got %v", args, err)
		}
	}
}

func TestUnknownSubcommandSuggests(t *testing.T) {
	io, _, _, _ := iostreams.Test()
	cmd := NewCmdRoot(&cmdutil.Factory{IOStreams: io})
	cmd.SetArgs([]string{"pr", "merg"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "Did you mean") || !strings.Contains(err.Error(), "merge") {
		t.Errorf("expected a suggestion for 'merg', got %v", err)
	}
}

func TestGroupsWithoutArgsShowHelp(t *testing.T) {
	groups := [][]string{{}, {"auth"}, {"repo"}, {"pr"}, {"pipeline"}, {"branch"}, {"workspace"}, {"project"}, {"config"}}
	for _, args := range groups {
		io, _, out, _ := iostreams.Test()
		cmd := NewCmdRoot(&cmdutil.Factory{IOStreams: io})
		cmd.SetOut(out)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Errorf("%v: expected help (exit 0), got %v", args, err)
			continue
		}
		if !strings.Contains(out.String(), "Available Commands") {
			t.Errorf("%v: help not printed: %q", args, out.String())
		}
		// --help must keep working too
		io2, _, out2, _ := iostreams.Test()
		cmd = NewCmdRoot(&cmdutil.Factory{IOStreams: io2})
		cmd.SetOut(out2)
		cmd.SetArgs(append(append([]string{}, args...), "--help"))
		if err := cmd.Execute(); err != nil || !strings.Contains(out2.String(), "Usage") {
			t.Errorf("%v --help: err=%v out=%q", args, err, out2.String())
		}
	}
}

func TestRootVersionFlag(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	cmd := NewCmdRoot(&cmdutil.Factory{IOStreams: io})
	cmd.SetOut(out)
	cmd.SetArgs([]string{"--version"})
	if err := cmd.Execute(); err != nil || !strings.HasPrefix(out.String(), "bb version ") {
		t.Errorf("--version: err=%v out=%q", err, out.String())
	}
}
