package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/iostreams"
)

func TestExitCode(t *testing.T) {
	cmd := &cobra.Command{Use: "x"}
	cases := []struct {
		name        string
		err         error
		interrupted bool
		want        int
		stderr      string
	}{
		{"ok", nil, false, cmdutil.ExitOK, ""},
		{"generic", errors.New("boom"), false, cmdutil.ExitError, "boom"},
		{"silent", cmdutil.ErrSilent, false, cmdutil.ExitError, ""},
		{"cancel", cmdutil.ErrCancel, false, cmdutil.ExitCancel, ""},
		{"interrupted", errors.New("x"), true, cmdutil.ExitCancel, "interrupted"},
		{"ctx canceled", context.Canceled, false, cmdutil.ExitCancel, "interrupted"},
		{"pending", cmdutil.ErrPending, false, cmdutil.ExitPending, ""},
		{"auth", cmdutil.NewAuthError("nope"), false, cmdutil.ExitAuth, "bb auth login"},
		{"flag", cmdutil.FlagErrorf("bad flag"), false, cmdutil.ExitError, "bad flag"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			got := exitCode(&buf, cmd, tc.err, tc.interrupted)
			if got != tc.want {
				t.Errorf("exit=%d want %d", got, tc.want)
			}
			if tc.stderr != "" && !strings.Contains(buf.String(), tc.stderr) {
				t.Errorf("stderr %q missing %q", buf.String(), tc.stderr)
			}
			if tc.stderr == "" && tc.err != nil && strings.TrimSpace(buf.String()) != "" && tc.name == "silent" {
				t.Errorf("silent error must print nothing: %q", buf.String())
			}
		})
	}
}

func TestExecute(t *testing.T) {
	ios, _, out, errOut := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: ios}
	if code := execute(context.Background(), f, []string{"version"}); code != 0 || !strings.HasPrefix(out.String(), "bb version") {
		t.Errorf("version: code=%d out=%q", code, out.String())
	}
	out.Reset()
	if code := execute(context.Background(), f, []string{"no-such-command"}); code != cmdutil.ExitError || !strings.Contains(errOut.String(), "unknown command") {
		t.Errorf("unknown: code=%d err=%q", code, errOut.String())
	}
	if out.Len() != 0 || !strings.Contains(errOut.String(), "Usage:") {
		t.Errorf("unknown: usage must go to stderr only: out=%q err=%q", out.String(), errOut.String())
	}
	errOut.Reset()
	if code := execute(context.Background(), f, []string{"pr", "bogus"}); code != cmdutil.ExitError || !strings.Contains(errOut.String(), "unknown command") {
		t.Errorf("group unknown: code=%d err=%q", code, errOut.String())
	}
	out.Reset()
	if code := execute(context.Background(), f, []string{"pr"}); code != 0 || !strings.Contains(out.String(), "Available Commands") {
		t.Errorf("group help: code=%d out=%q", code, out.String())
	}
	errOut.Reset()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code := execute(ctx, f, []string{"no-such-command"}); code != cmdutil.ExitCancel || !strings.Contains(errOut.String(), "interrupted") {
		t.Errorf("cancelled ctx: code=%d err=%q", code, errOut.String())
	}
}

func TestUnknownFlagIsUsageError(t *testing.T) {
	ios, _, out, errOut := iostreams.Test()
	f := &cmdutil.Factory{IOStreams: ios}
	for _, args := range [][]string{{"--bogusflag"}, {"pr", "list", "--bogusflag"}, {"pr", "view", "-Z"}} {
		out.Reset()
		errOut.Reset()
		code := execute(context.Background(), f, args)
		if code != cmdutil.ExitError || !strings.Contains(errOut.String(), "unknown") || !strings.Contains(errOut.String(), "Usage:") || out.Len() != 0 {
			t.Errorf("%v: code=%d stdout=%q stderr=%q", args, code, out.String(), errOut.String())
		}
	}
}
