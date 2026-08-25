package branch

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/uehatsu/bb/internal/testutil"
)

func TestBranchCommands(t *testing.T) {
	h := testutil.NewHarness(t)
	var got *http.Request
	var body string
	h.Handle("/repositories/acme/widgets/refs/branches", func(w http.ResponseWriter, r *http.Request) {
		got = r
		if r.Method == "POST" {
			b, _ := io.ReadAll(r.Body)
			body = string(b)
			w.WriteHeader(201)
			w.Write([]byte(`{"name":"feat/x","target":{"hash":"abcdef123456"}}`))
			return
		}
		w.Write([]byte(`{"values":[{"name":"main","target":{"hash":"1234567890","message":"Initial\n\nbody","date":"2026-08-01T00:00:00Z"}}]}`))
	})
	h.JSON("GET", "/repositories/acme/widgets", 200, `{"mainbranch":{"name":"main"}}`)
	deleted := ""
	h.Handle("/repositories/acme/widgets/refs/branches/feat%2Fx", func(w http.ResponseWriter, r *http.Request) {
		deleted = r.Method
		w.WriteHeader(204)
	})

	l := NewCmdList(h.Factory)
	l.SetArgs([]string{"--search", "ma"})
	if err := l.Execute(); err != nil {
		t.Fatal(err)
	}
	if got.URL.Query().Get("q") != `name ~ "ma"` || h.Out.String() != "main\t1234567\tInitial\t2026-08-01T00:00:00Z\n" {
		t.Errorf("list: q=%s out=%q", got.URL.Query().Get("q"), h.Out.String())
	}

	c := NewCmdCreate(h.Factory)
	c.SetArgs([]string{"feat/x"})
	if err := c.Execute(); err != nil {
		t.Fatal(err)
	}
	if body != `{"name":"feat/x","target":{"hash":"main"}}` {
		t.Errorf("create body: %s", body)
	}
	c = NewCmdCreate(h.Factory)
	c.SetArgs([]string{"feat/y", "--from", "abc123"})
	_ = c.Execute()
	if !strings.Contains(body, `"hash":"abc123"`) {
		t.Errorf("from: %s", body)
	}

	d := NewCmdDelete(h.Factory)
	d.SetArgs([]string{"feat/x", "--yes"})
	if err := d.Execute(); err != nil || deleted != "DELETE" {
		t.Errorf("delete: %v %s", err, deleted)
	}
}
