package cmdutil

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestAddJSONFlags(t *testing.T) {
	validate := func(fs []string) error {
		for _, f := range fs {
			if f != "id" && f != "title" {
				return FlagErrorf("unknown %s", f)
			}
		}
		return nil
	}
	newCmd := func() (*cobra.Command, func() (interface{ Active() bool }, error)) {
		cmd := &cobra.Command{Use: "x", RunE: func(*cobra.Command, []string) error { return nil }}
		build := AddJSONFlags(cmd, validate, []string{"id", "title"})
		return cmd, func() (interface{ Active() bool }, error) {
			e, err := build()
			if e == nil {
				return nil, err
			}
			return e, err
		}
	}

	cmd, build := newCmd()
	cmd.SetArgs([]string{"--json", "id,title", "--jq", ".[]"})
	_ = cmd.Execute()
	if e, err := build(); err != nil || e == nil || !e.Active() {
		t.Fatalf("expected exporter, got %v %v", e, err)
	}

	cmd, build = newCmd()
	cmd.SetArgs([]string{"--json", "nope"})
	_ = cmd.Execute()
	if _, err := build(); err == nil {
		t.Fatal("expected validation error")
	}

	cmd, build = newCmd()
	cmd.SetArgs([]string{"--jq", "."})
	_ = cmd.Execute()
	if _, err := build(); err == nil {
		t.Fatal("expected error: jq without json")
	}

	cmd, build = newCmd()
	cmd.SetArgs([]string{})
	_ = cmd.Execute()
	if e, err := build(); err != nil || e != nil {
		t.Fatalf("expected nil exporter, got %v %v", e, err)
	}
}
