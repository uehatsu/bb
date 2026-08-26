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
	for _, args := range [][]string{{"pipeline", "nonsense"}, {"pr", "nonsense"}, {"repo", "nonsense"}, {"auth", "nonsense"}, {"branch", "x"}, {"workspace", "x"}, {"project", "x"}, {"config", "x"}} {
		io, _, _, _ := iostreams.Test()
		cmd := NewCmdRoot(&cmdutil.Factory{IOStreams: io})
		cmd.SetArgs(args)
		if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Errorf("%v: expected unknown command error, got %v", args, err)
		}
	}
}
