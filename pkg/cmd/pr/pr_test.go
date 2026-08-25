package pr

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/testutil"
)

const prJSON = `{"id":42,"title":"Fix login","description":"Closes JIRA-1","state":"OPEN","draft":false,"author":{"nickname":"alice","uuid":"{a}"},"source":{"branch":{"name":"feat/login"},"repository":{"full_name":"acme/widgets","workspace":{"slug":"acme"}}},"destination":{"branch":{"name":"main"},"repository":{"full_name":"acme/widgets"}},"comment_count":2,"task_count":0,"updated_on":"2026-08-20T00:00:00Z","reviewers":[{"nickname":"bob"}],"participants":[{"user":{"nickname":"bob"},"role":"REVIEWER","approved":true,"state":"approved"}],"links":{"html":{"href":"https://bitbucket.org/acme/widgets/pull-requests/42"}}}`

func TestParsePRNumber(t *testing.T) {
	for in, want := range map[string]int{"42": 42, "#42": 42, "https://bitbucket.org/acme/widgets/pull-requests/42/diff": 42, "https://bitbucket.org/acme/widgets/pull-requests/7": 7} {
		if got, err := parsePRNumber(in); err != nil || got != want {
			t.Errorf("%s: %d %v", in, got, err)
		}
	}
	for _, bad := range []string{"feat/x", "0", "-1", ""} {
		if _, err := parsePRNumber(bad); err == nil {
			t.Errorf("%q should fail", bad)
		}
	}
}

func TestListFiltersAndOutput(t *testing.T) {
	h := testutil.NewHarness(t)
	var got *http.Request
	h.JSON("GET", "/user", 200, `{"uuid":"{me-uuid}","nickname":"me"}`)
	h.Handle("/repositories/acme/widgets/pullrequests", func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.Write([]byte(`{"values":[` + prJSON + `]}`))
	})
	cmd := NewCmdList(h.Factory)
	cmd.SetArgs([]string{"--state", "all", "--author", "@me", "--base", "main", "-L", "10"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	q := got.URL.Query()
	if states := q["state"]; len(states) != 4 {
		t.Errorf("state params: %v", states)
	}
	if !strings.Contains(q.Get("q"), `author.uuid="{me-uuid}"`) || !strings.Contains(q.Get("q"), `destination.branch.name="main"`) {
		t.Errorf("q: %s", q.Get("q"))
	}
	if !strings.Contains(q.Get("fields"), "values.id") {
		t.Errorf("fields must narrow response: %s", q.Get("fields"))
	}
	if h.Out.String() != "#42\tFix login\tfeat/login\tOPEN\t2026-08-20T00:00:00Z\n" {
		t.Errorf("out: %q", h.Out.String())
	}
	cmd = NewCmdList(h.Factory)
	cmd.SetArgs([]string{"--state", "bogus"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected invalid state error")
	}
}

func TestViewByNumberAndBranch(t *testing.T) {
	h := testutil.NewHarness(t)
	h.JSON("GET", "/repositories/acme/widgets/pullrequests/42", 200, prJSON)
	var listReq *http.Request
	h.Handle("/repositories/acme/widgets/pullrequests", func(w http.ResponseWriter, r *http.Request) {
		listReq = r
		w.Write([]byte(`{"values":[` + prJSON + `]}`))
	})
	h.JSON("GET", "/repositories/acme/widgets/pullrequests/42/comments", 200, `{"values":[{"id":1,"content":{"raw":"LGTM"},"user":{"nickname":"bob"},"created_on":"2026-08-20T00:00:00Z"},{"id":2,"deleted":true,"content":{"raw":"gone"}}]}`)

	cmd := NewCmdView(h.Factory)
	cmd.SetArgs([]string{"42", "--comments"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	s := h.Out.String()
	for _, want := range []string{"title:\tFix login", "state:\tOPEN", "head:\tfeat/login", "Closes JIRA-1", "comment by bob", "LGTM"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "gone") {
		t.Error("deleted comment should be hidden")
	}

	h.Out.Reset()
	cmd = NewCmdView(h.Factory)
	cmd.SetArgs([]string{"feat/login", "--json", "id,headRefName"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if listReq == nil || !strings.Contains(listReq.URL.Query().Get("q"), `source.branch.name="feat/login"`) {
		t.Errorf("branch lookup query: %v", listReq)
	}
	if h.Out.String() != `{"headRefName":"feat/login","id":42}`+"\n" {
		t.Errorf("json: %s", h.Out.String())
	}
}

func TestCreateWithReviewersAndDefaults(t *testing.T) {
	h := testutil.NewHarness(t)
	h.JSON("GET", "/repositories/acme/widgets", 200, `{"mainbranch":{"name":"main"}}`)
	h.JSON("GET", "/repositories/acme/widgets/effective-default-reviewers", 200, `{"values":[{"user":{"uuid":"{default}"}}]}`)
	h.JSON("GET", "/workspaces/acme/members", 200, `{"values":[{"user":{"uuid":"{alice}","nickname":"alice"}},{"user":{"uuid":"{bob}","nickname":"bob","display_name":"Bob B"}}]}`)
	var body string
	h.Handle("/repositories/acme/widgets/pullrequests", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(201)
		w.Write([]byte(prJSON))
	})
	cmd := NewCmdCreate(h.Factory)
	cmd.SetArgs([]string{"--title", "Fix login", "--body", "Closes JIRA-1", "--head", "feat/login", "--reviewer", "alice,@bob", "--draft", "--close-source-branch"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"title":"Fix login"`, `"description":"Closes JIRA-1"`, `"source":{"branch":{"name":"feat/login"}}`, `"destination":{"branch":{"name":"main"}}`, `"draft":true`, `"close_source_branch":true`, `{"uuid":"{default}"}`, `{"uuid":"{alice}"}`, `{"uuid":"{bob}"}`} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %s:\n%s", want, body)
		}
	}
	if h.Out.String() != "https://bitbucket.org/acme/widgets/pull-requests/42\n" {
		t.Errorf("out: %q", h.Out.String())
	}

	cmd = NewCmdCreate(h.Factory)
	cmd.SetArgs([]string{"--title", "x", "--head", "feat/login", "--reviewer", "nobody"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "nobody") {
		t.Errorf("expected missing reviewer error, got %v", err)
	}
	cmd = NewCmdCreate(h.Factory)
	cmd.SetArgs([]string{"--head", "feat/login"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error: no title non-interactive")
	}
	cmd = NewCmdCreate(h.Factory)
	cmd.SetArgs([]string{"--title", "x", "--head", "main"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "both") {
		t.Errorf("expected same-branch error, got %v", err)
	}
}

func TestCreateFill(t *testing.T) {
	h := testutil.NewHarness(t)
	h.JSON("GET", "/repositories/acme/widgets", 200, `{"mainbranch":{"name":"main"}}`)
	h.JSON("GET", "/repositories/acme/widgets/effective-default-reviewers", 200, `{"values":[]}`)
	var commitsReq *http.Request
	h.Handle("/repositories/acme/widgets/commits", func(w http.ResponseWriter, r *http.Request) {
		commitsReq = r
		w.Write([]byte(`{"values":[{"hash":"b","message":"second commit\n\ndetails"},{"hash":"a","message":"first commit"}]}`))
	})
	var body string
	h.Handle("/repositories/acme/widgets/pullrequests", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.Write([]byte(prJSON))
	})
	cmd := NewCmdCreate(h.Factory)
	cmd.SetArgs([]string{"--fill", "--head", "feat/login"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	q := commitsReq.URL.Query()
	if q.Get("include") != "feat/login" || q.Get("exclude") != "main" {
		t.Errorf("commits query: %v", q)
	}
	if !strings.Contains(body, `"title":"feat/login"`) || !strings.Contains(body, `"description":"- first commit\n- second commit"`) {
		t.Errorf("fill body: %s", body)
	}
}

func TestCreateWebURL(t *testing.T) {
	h := testutil.NewHarness(t)
	t.Setenv("BROWSER", testutil.NoopBrowser())
	cmd := NewCmdCreate(h.Factory)
	cmd.SetArgs([]string{"--web", "--head", "feat/x", "--base", "develop", "--title", "Hi there"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
}

func TestResolveReviewersUsesFilterThenFallback(t *testing.T) {
	h := testutil.NewHarness(t)
	var queries []string
	h.Handle("/workspaces/acme/members", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		queries = append(queries, q)
		switch {
		case strings.Contains(q, `"alice"`):
			w.Write([]byte(`{"values":[{"user":{"uuid":"{alice}","nickname":"alice"}}]}`))
		case q != "":
			w.Write([]byte(`{"values":[]}`))
		default:
			w.Write([]byte(`{"values":[{"user":{"uuid":"{alice}","nickname":"alice"}},{"user":{"uuid":"{bob}","nickname":"bobby","display_name":"Bob B"}}]}`))
		}
	})
	client, _ := h.Factory.APIClient()
	repo := cmdutil.Repo{Workspace: "acme", Slug: "widgets"}
	got, err := resolveReviewers(t.Context(), client, repo, []string{"alice", "{direct-uuid}"}, false)
	if err != nil || len(got) != 2 {
		t.Fatalf("filter path: %v %v", got, err)
	}
	if len(queries) != 1 || !strings.Contains(queries[0], "alice") {
		t.Errorf("expected one filtered query, got %v", queries)
	}
	// display-name match forces the scan fallback
	queries = nil
	got, err = resolveReviewers(t.Context(), client, repo, []string{"Bob B"}, false)
	if err != nil || len(got) != 1 || got[0] != "{bob}" {
		t.Fatalf("scan fallback: %v %v", got, err)
	}
	if len(queries) != 2 || queries[1] != "" {
		t.Errorf("expected filter then full scan, got %v", queries)
	}
	// 400 on q= → scan
	h2 := testutil.NewHarness(t)
	h2.Handle("/workspaces/acme/members", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "" {
			w.WriteHeader(400)
			w.Write([]byte(`{"type":"error","error":{"message":"unsupported filter"}}`))
			return
		}
		w.Write([]byte(`{"values":[{"user":{"uuid":"{alice}","nickname":"alice"}}]}`))
	})
	client2, _ := h2.Factory.APIClient()
	if got, err := resolveReviewers(t.Context(), client2, repo, []string{"alice"}, false); err != nil || len(got) != 1 {
		t.Errorf("400 fallback: %v %v", got, err)
	}
}
