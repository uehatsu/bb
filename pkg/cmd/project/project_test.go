package project

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/uehatsu/bb/internal/testutil"
)

func TestProjectCommands(t *testing.T) {
	h := testutil.NewHarness(t)
	var body string
	h.Handle("/workspaces/acme/projects", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.WriteHeader(201)
			w.Write([]byte(`{"key":"PROJ","name":"My Project","links":{"html":{"href":"https://bitbucket.org/acme/workspace/projects/PROJ"}}}`))
			return
		}
		w.Write([]byte(`{"values":[{"key":"PROJ","name":"My Project","description":"d","is_private":true}]}`))
	})
	h.JSON("GET", "/workspaces/acme/projects/PROJ", 200, `{"key":"PROJ","name":"My Project","is_private":false,"links":{"html":{"href":"https://x"}}}`)

	root := NewCmdProject(h.Factory)
	root.SetArgs([]string{"list"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if h.Out.String() != "PROJ\tMy Project\td\tprivate\n" {
		t.Errorf("list: %q", h.Out.String())
	}

	h.Out.Reset()
	root = NewCmdProject(h.Factory)
	root.SetArgs([]string{"view", "proj", "-w", "acme"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Out.String(), "My Project (PROJ)") || !strings.Contains(h.Out.String(), "public") {
		t.Errorf("view: %s", h.Out.String())
	}

	h.Out.Reset()
	root = NewCmdProject(h.Factory)
	root.SetArgs([]string{"create", "proj", "--name", "My Project", "--public", "-d", "desc"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `"key":"PROJ"`) || !strings.Contains(body, `"is_private":false`) || !strings.Contains(body, `"description":"desc"`) {
		t.Errorf("create body: %s", body)
	}
	root = NewCmdProject(h.Factory)
	root.SetArgs([]string{"create", "X"})
	if err := root.Execute(); err == nil {
		t.Error("expected --name required error")
	}
}
