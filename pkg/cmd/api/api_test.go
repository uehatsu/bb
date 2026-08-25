package api

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	bbapi "github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/config"
	"github.com/uehatsu/bb/internal/iostreams"
)

func newFactory(t *testing.T, h http.Handler) (*cmdutil.Factory, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	io, in, out, errOut := iostreams.Test()
	cfg, _ := config.LoadFrom(t.TempDir())
	f := &cmdutil.Factory{
		IOStreams: io,
		Config:    func() (*config.Config, error) { return cfg, nil },
		APIClient: func() (*bbapi.Client, error) {
			return bbapi.NewClient(bbapi.NewAuthenticator(config.Credential{Method: config.AuthBearer, Token: "t"}), bbapi.WithBaseURL(srv.URL+"/2.0")), nil
		},
		BaseRepo: func() (cmdutil.Repo, error) { return cmdutil.Repo{Workspace: "acme", Slug: "widgets"}, nil },
	}
	return f, in, out, errOut, srv
}

func TestAPIGetPlaceholdersAndQuery(t *testing.T) {
	var got *http.Request
	f, _, out, _, _ := newFactory(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.Write([]byte(`{"slug":"widgets"}`))
	}))
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"repositories/{workspace}/{repo_slug}", "-X", "GET", "-f", "fields=slug"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got.Method != "GET" || got.URL.Path != "/2.0/repositories/acme/widgets" || got.URL.Query().Get("fields") != "slug" {
		t.Errorf("request: %s %s", got.Method, got.URL)
	}
	if out.String() != `{"slug":"widgets"}`+"\n" {
		t.Errorf("out: %q", out.String())
	}
}

func TestAPIPostTypedFields(t *testing.T) {
	var body []byte
	var method string
	f, _, _, _, _ := newFactory(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		body, _ = io.ReadAll(r.Body)
		w.WriteHeader(201)
		w.Write([]byte(`{}`))
	}))
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"repositories/acme/widgets/refs/branches", "-f", "name=feat", "-F", "target[hash]=abc", "-F", "draft=true", "-F", "n=3", "-F", "tags[]=a", "-F", "tags[]=b"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if method != "POST" {
		t.Errorf("method %s", method)
	}
	s := string(body)
	for _, want := range []string{`"name":"feat"`, `"target":{"hash":"abc"}`, `"draft":true`, `"n":3`, `"tags":["a","b"]`} {
		if !strings.Contains(s, want) {
			t.Errorf("body missing %s: %s", want, s)
		}
	}
}

func TestAPIPaginateAndJQ(t *testing.T) {
	f, _, out, _, srv := newFactory(t, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("/2.0/repositories/acme/widgets/pullrequests", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "2" {
			w.Write([]byte(`{"values":[{"id":3,"title":"C"}]}`))
			return
		}
		w.Write([]byte(`{"values":[{"id":1,"title":"A"},{"id":2,"title":"B"}],"next":"` + srv.URL + `/2.0/repositories/acme/widgets/pullrequests?page=2"}`))
	})
	srv.Config.Handler = mux
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"repositories/{workspace}/{repo}/pullrequests", "--paginate", "--jq", ".[].title"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "A\nB\nC\n" {
		t.Errorf("out: %q", out.String())
	}
}

func TestAPIErrorOutput(t *testing.T) {
	f, _, out, errOut, _ := newFactory(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"type":"error","error":{"message":"Not found"}}`))
	}))
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"/nope", "-i"})
	err := cmd.Execute()
	if err != cmdutil.ErrSilent {
		t.Fatalf("expected ErrSilent, got %v", err)
	}
	if !strings.Contains(out.String(), "404") || !strings.Contains(out.String(), "Not found") || !strings.Contains(errOut.String(), "HTTP 404") {
		t.Errorf("out=%q err=%q", out.String(), errOut.String())
	}
}

func TestAPIInputStdin(t *testing.T) {
	var body []byte
	f, in, _, _, _ := newFactory(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		w.Write([]byte(`{}`))
	}))
	in.WriteString(`{"title":"from stdin"}`)
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"-X", "PUT", "--input", "-", "repositories/acme/widgets/pullrequests/1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"title":"from stdin"}` {
		t.Errorf("body: %s", body)
	}
}

func TestAPIFlagValidation(t *testing.T) {
	f, _, _, _, _ := newFactory(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"/x", "--paginate", "-X", "POST"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error for paginate with POST")
	}
}

func TestRepoOverrideFlag(t *testing.T) {
	var path string
	f, _, _, _, _ := newFactory(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Write([]byte(`{}`))
	}))
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"repositories/{workspace}/{repo_slug}", "-R", "other/repo"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if path != "/2.0/repositories/other/repo" {
		t.Errorf("-R not honored: %s", path)
	}
	t.Setenv("BB_REPO", "env/repo")
	cmd = NewCmdAPI(f)
	cmd.SetArgs([]string{"repositories/{workspace}/{repo_slug}"})
	_ = cmd.Execute()
	if path != "/2.0/repositories/env/repo" {
		t.Errorf("BB_REPO not honored: %s", path)
	}
}

func TestAPIPaginateRespectsPathQuery(t *testing.T) {
	var got *http.Request
	f, _, _, _, _ := newFactory(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.Write([]byte(`{"values":[]}`))
	}))
	cmd := NewCmdAPI(f)
	cmd.SetArgs([]string{"repositories/acme/widgets/pullrequests?pagelen=100&state=MERGED", "--paginate", "-H", "X-Test: 1"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	q := got.URL.Query()
	if q.Get("pagelen") != "100" || q.Get("state") != "MERGED" || len(q["pagelen"]) != 1 || got.Header.Get("X-Test") != "1" {
		t.Errorf("query=%v header=%q", q, got.Header.Get("X-Test"))
	}
}
