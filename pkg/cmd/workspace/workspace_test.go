package workspace

import (
	"strings"
	"testing"

	"github.com/uehatsu/bb/internal/testutil"
)

func TestWorkspaceCommands(t *testing.T) {
	h := testutil.NewHarness(t)
	h.JSON("GET", "/user/workspaces", 200, `{"values":[{"permission":"owner","workspace":{"slug":"acme","name":"ACME Inc","uuid":"{w}"}}]}`)
	h.JSON("GET", "/workspaces/acme", 200, `{"slug":"acme","name":"ACME Inc","uuid":"{w}","is_private":true,"links":{"html":{"href":"https://bitbucket.org/acme/"}}}`)
	h.JSON("GET", "/workspaces/acme/members", 200, `{"values":[{"user":{"nickname":"alice","display_name":"Alice","uuid":"{a}"}}]}`)

	l := NewCmdList(h.Factory)
	l.SetArgs([]string{})
	if err := l.Execute(); err != nil {
		t.Fatal(err)
	}
	if h.Out.String() != "acme\tACME Inc\towner\n" {
		t.Errorf("list: %q", h.Out.String())
	}
	h.Out.Reset()
	l = NewCmdList(h.Factory)
	l.SetArgs([]string{"--json", "slug"})
	_ = l.Execute()
	if h.Out.String() != `[{"slug":"acme"}]`+"\n" {
		t.Errorf("json: %q", h.Out.String())
	}

	h.Out.Reset()
	v := NewCmdView(h.Factory)
	v.SetArgs([]string{}) // falls back to base repo workspace
	if err := v.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Out.String(), "ACME Inc (acme)") || !strings.Contains(h.Out.String(), "team, private") {
		t.Errorf("view: %s", h.Out.String())
	}

	h.Out.Reset()
	m := NewCmdMembers(h.Factory)
	m.SetArgs([]string{"acme"})
	if err := m.Execute(); err != nil {
		t.Fatal(err)
	}
	if h.Out.String() != "alice\tAlice\t{a}\n" {
		t.Errorf("members: %q", h.Out.String())
	}
}
