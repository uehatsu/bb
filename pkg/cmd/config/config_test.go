package config

import (
	"testing"

	"github.com/uehatsu/bb/internal/testutil"
)

func TestConfigCommands(t *testing.T) {
	h := testutil.NewHarness(t)
	c := NewCmdConfig(h.Factory)
	c.SetArgs([]string{"set", "workspace", "acme"})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	c = NewCmdConfig(h.Factory)
	c.SetArgs([]string{"get", "workspace"})
	if err := c.Execute(); err != nil || h.Out.String() != "acme\n" {
		t.Errorf("get: %v %q", err, h.Out.String())
	}
	c = NewCmdConfig(h.Factory)
	c.SetArgs([]string{"set", "git_protocol", "ftp"})
	if err := c.Execute(); err == nil {
		t.Error("expected validation error")
	}
	c = NewCmdConfig(h.Factory)
	c.SetArgs([]string{"get", "nope"})
	if err := c.Execute(); err == nil {
		t.Error("expected unknown key error")
	}
	h.Out.Reset()
	c = NewCmdConfig(h.Factory)
	c.SetArgs([]string{"list"})
	_ = c.Execute()
	if got := h.Out.String(); !contains(got, "git_protocol=https\n") || !contains(got, "workspace=acme\n") || !contains(got, "merge_strategy=merge_commit\n") {
		t.Errorf("list: %q", got)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
