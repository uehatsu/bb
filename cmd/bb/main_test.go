package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/cmdutil"
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
