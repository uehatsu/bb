package pr

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/testutil"
)

func mergedJSON() string { return strings.Replace(prJSON, `"state":"OPEN"`, `"state":"MERGED"`, 1) }

func TestMergeSync(t *testing.T) {
	h := testutil.NewHarness(t)
	var fetched int32
	h.Handle("/repositories/acme/widgets/pullrequests/42", func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&fetched, 1) == 1 {
			w.Write([]byte(prJSON))
			return
		}
		w.Write([]byte(mergedJSON()))
	})
	var body string
	var query string
	h.Handle("/repositories/acme/widgets/pullrequests/42/merge", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body, query = string(b), r.URL.RawQuery
		w.Write([]byte(mergedJSON()))
	})
	_ = h.Config.Set("merge_strategy", "squash")
	cmd := NewCmdMerge(h.Factory)
	cmd.SetArgs([]string{"42", "--delete-branch", "-b", "Squashed"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `"merge_strategy":"squash"`) || !strings.Contains(body, `"close_source_branch":true`) || !strings.Contains(body, `"message":"Squashed"`) || query != "async=true" {
		t.Errorf("merge request: %s ?%s", body, query)
	}
	if !strings.Contains(h.ErrOut.String(), "Merged pull request #42") || !strings.Contains(h.ErrOut.String(), "Deleted branch feat/login") {
		t.Errorf("stderr: %s", h.ErrOut.String())
	}

	// explicit strategy flags
	cmd = NewCmdMerge(h.Factory)
	cmd.SetArgs([]string{"42", "--rebase"})
	fetched = 0
	_ = cmd.Execute()
	if !strings.Contains(body, `"merge_strategy":"rebase_merge"`) {
		t.Errorf("--rebase: %s", body)
	}
	cmd = NewCmdMerge(h.Factory)
	cmd.SetArgs([]string{"42", "--squash", "--merge"})
	if err := cmd.Execute(); err == nil {
		t.Error("expected conflict error")
	}
}

func TestMergeAsyncPolling(t *testing.T) {
	h := testutil.NewHarness(t)
	var fetched int32
	h.Handle("/repositories/acme/widgets/pullrequests/42", func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&fetched, 1) == 1 {
			w.Write([]byte(prJSON))
			return
		}
		w.Write([]byte(mergedJSON()))
	})
	h.Handle("/repositories/acme/widgets/pullrequests/42/merge", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", h.Server.URL+"/2.0/repositories/acme/widgets/pullrequests/42/merge/task-status/task1")
		w.WriteHeader(202)
	})
	var polls int32
	h.Handle("/repositories/acme/widgets/pullrequests/42/merge/task-status/task1", func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&polls, 1) < 2 {
			w.Write([]byte(`{"task_status":"PENDING"}`))
			return
		}
		w.Write([]byte(`{"task_status":"SUCCESS"}`))
	})
	cmd := NewCmdMerge(h.Factory)
	cmd.SetArgs([]string{"42", "--merge", "--timeout", "10s"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if polls < 2 || !strings.Contains(h.ErrOut.String(), "Merged pull request #42") {
		t.Errorf("polls=%d err=%s", polls, h.ErrOut.String())
	}
}

func TestMergeAsyncFailureAndNotOpen(t *testing.T) {
	h := testutil.NewHarness(t)
	h.JSON("GET", "/repositories/acme/widgets/pullrequests/42", 200, prJSON)
	h.Handle("/repositories/acme/widgets/pullrequests/42/merge", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", h.Server.URL+"/2.0/task")
		w.WriteHeader(202)
	})
	h.JSON("GET", "/task", 200, `{"task_status":"FAILED","error":{"message":"conflicts"}}`)
	cmd := NewCmdMerge(h.Factory)
	cmd.SetArgs([]string{"42", "--merge"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Errorf("expected failure, got %v", err)
	}

	h.JSON("GET", "/repositories/acme/widgets/pullrequests/43", 200, mergedJSON())
	cmd = NewCmdMerge(h.Factory)
	cmd.SetArgs([]string{"43", "--merge"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "MERGED") {
		t.Errorf("expected not-open error, got %v", err)
	}
}

func TestDeclineApproveUnapproveReady(t *testing.T) {
	h := testutil.NewHarness(t)
	h.JSON("GET", "/repositories/acme/widgets/pullrequests/42", 200, prJSON)
	calls := map[string]string{}
	for _, p := range []string{"decline", "approve", "request-changes"} {
		p := p
		h.Handle("/repositories/acme/widgets/pullrequests/42/"+p, func(w http.ResponseWriter, r *http.Request) {
			calls[p] = r.Method
			w.Write([]byte(`{}`))
		})
	}
	var putBody string
	h.Mux.HandleFunc("/2.0/repositories/acme/widgets/pullrequests/42/", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) })
	h.Handle("/repositories/acme/widgets/refs/branches/feat%2Flogin", func(w http.ResponseWriter, r *http.Request) {
		calls["branch-delete"] = r.Method
		w.WriteHeader(204)
	})

	run := func(c interface{ Execute() error }) {
		t.Helper()
		if err := c.Execute(); err != nil {
			t.Fatal(err)
		}
	}
	d := NewCmdDecline(h.Factory)
	d.SetArgs([]string{"42", "--delete-branch"})
	run(d)
	a := NewCmdApprove(h.Factory)
	a.SetArgs([]string{"42"})
	run(a)
	u := NewCmdUnapprove(h.Factory)
	u.SetArgs([]string{"42"})
	run(u)
	if calls["decline"] != "POST" || calls["branch-delete"] != "DELETE" || calls["approve"] != "DELETE" {
		t.Errorf("calls: %v", calls)
	}
	_ = putBody

	h2 := testutil.NewHarness(t)
	h2.Handle("/repositories/acme/widgets/pullrequests/42", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			b, _ := io.ReadAll(r.Body)
			putBody = string(b)
		}
		w.Write([]byte(prJSON))
	})
	rd := NewCmdReady(h2.Factory)
	rd.SetArgs([]string{"42"})
	run(rd)
	if putBody != `{"draft":false,"title":"Fix login"}` {
		t.Errorf("ready body: %s", putBody)
	}
	rd = NewCmdReady(h2.Factory)
	rd.SetArgs([]string{"42", "--undo"})
	run(rd)
	if putBody != `{"draft":true,"title":"Fix login"}` {
		t.Errorf("undo body: %s", putBody)
	}

	if err := NewCmdReopen(h2.Factory).Execute(); err == nil || !strings.Contains(err.Error(), "cannot be reopened") {
		t.Errorf("reopen: %v", err)
	}
}

func TestReviewAndComment(t *testing.T) {
	h := testutil.NewHarness(t)
	h.JSON("GET", "/repositories/acme/widgets/pullrequests/42", 200, prJSON)
	var comments []string
	h.Handle("/repositories/acme/widgets/pullrequests/42/comments", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		comments = append(comments, string(b))
		w.WriteHeader(201)
		w.Write([]byte(`{"id":9,"links":{"html":{"href":"https://bitbucket.org/acme/widgets/pull-requests/42#comment-9"}}}`))
	})
	actions := map[string]int{}
	for _, p := range []string{"approve", "request-changes"} {
		p := p
		h.Handle("/repositories/acme/widgets/pullrequests/42/"+p, func(w http.ResponseWriter, r *http.Request) {
			actions[p]++
			w.Write([]byte(`{}`))
		})
	}
	rv := NewCmdReview(h.Factory)
	rv.SetArgs([]string{"42", "--request-changes", "-b", "Add tests"})
	if err := rv.Execute(); err != nil {
		t.Fatal(err)
	}
	rv = NewCmdReview(h.Factory)
	rv.SetArgs([]string{"42", "--approve"})
	if err := rv.Execute(); err != nil {
		t.Fatal(err)
	}
	if actions["request-changes"] != 1 || actions["approve"] != 1 || len(comments) != 1 || !strings.Contains(comments[0], `"raw":"Add tests"`) {
		t.Errorf("actions=%v comments=%v", actions, comments)
	}
	rv = NewCmdReview(h.Factory)
	rv.SetArgs([]string{"42", "--comment"})
	if err := rv.Execute(); err == nil {
		t.Error("--comment without body should fail")
	}
	rv = NewCmdReview(h.Factory)
	rv.SetArgs([]string{"42"})
	if err := rv.Execute(); err == nil {
		t.Error("no action should fail")
	}

	cm := NewCmdComment(h.Factory)
	cm.SetArgs([]string{"42", "--path", "src/main.go", "--line", "10", "-b", "typo"})
	if err := cm.Execute(); err != nil {
		t.Fatal(err)
	}
	last := comments[len(comments)-1]
	if !strings.Contains(last, `"inline":{"path":"src/main.go","to":10}`) {
		t.Errorf("inline: %s", last)
	}
	if !strings.HasSuffix(h.Out.String(), "#comment-9\n") {
		t.Errorf("out: %s", h.Out.String())
	}
	h.In.WriteString("from stdin")
	cm = NewCmdComment(h.Factory)
	cm.SetArgs([]string{"42", "-F", "-"})
	if err := cm.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(comments[len(comments)-1], "from stdin") {
		t.Error("body-file stdin")
	}
	cm = NewCmdComment(h.Factory)
	cm.SetArgs([]string{"42"})
	if err := cm.Execute(); err == nil {
		t.Error("non-interactive without body should fail")
	}
}

func TestDiffAndChecks(t *testing.T) {
	h := testutil.NewHarness(t)
	h.JSON("GET", "/repositories/acme/widgets/pullrequests/42", 200, prJSON)
	h.Handle("/repositories/acme/widgets/pullrequests/42/diff", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1 +1 @@\n-old\n+new\n"))
	})
	h.JSON("GET", "/repositories/acme/widgets/pullrequests/42/diffstat", 200, `{"values":[{"status":"modified","lines_added":1,"lines_removed":1,"new":{"path":"x"}},{"status":"added","lines_added":5,"lines_removed":0,"new":{"path":"y/z.go"}}]}`)

	d := NewCmdDiff(h.Factory)
	d.SetArgs([]string{"42"})
	if err := d.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Out.String(), "+new\n") || strings.Contains(h.Out.String(), "\x1b[") {
		t.Errorf("diff: %q", h.Out.String())
	}
	h.Out.Reset()
	d = NewCmdDiff(h.Factory)
	d.SetArgs([]string{"42", "--name-only"})
	_ = d.Execute()
	if h.Out.String() != "x\ny/z.go\n" {
		t.Errorf("name-only: %q", h.Out.String())
	}

	h.JSON("GET", "/repositories/acme/widgets/pullrequests/42/statuses", 200, `{"values":[{"key":"build","name":"Pipeline","state":"SUCCESSFUL","url":"https://x"},{"key":"lint","state":"INPROGRESS"}]}`)
	h.Out.Reset()
	c := NewCmdChecks(h.Factory)
	c.SetArgs([]string{"42"})
	if err := c.Execute(); !errors.Is(err, cmdutil.ErrPending) {
		t.Errorf("expected ErrPending, got %v", err)
	}
	if !strings.Contains(h.Out.String(), "SUCCESSFUL\tPipeline") || !strings.Contains(h.Out.String(), "INPROGRESS\tlint") {
		t.Errorf("checks: %q", h.Out.String())
	}
	h.JSON("GET", "/repositories/acme/widgets/pullrequests/43", 200, strings.Replace(prJSON, `"id":42`, `"id":43`, 1))
	h.JSON("GET", "/repositories/acme/widgets/pullrequests/43/statuses", 200, `{"values":[{"key":"build","state":"FAILED"}]}`)
	c = NewCmdChecks(h.Factory)
	c.SetArgs([]string{"43"})
	if err := c.Execute(); !errors.Is(err, cmdutil.ErrSilent) {
		t.Errorf("expected ErrSilent for failed checks, got %v", err)
	}
}

func TestEditReviewers(t *testing.T) {
	h := testutil.NewHarness(t)
	var putBody string
	h.Handle("/repositories/acme/widgets/pullrequests/42", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			b, _ := io.ReadAll(r.Body)
			putBody = string(b)
		}
		w.Write([]byte(strings.Replace(prJSON, `"reviewers":[{"nickname":"bob"}]`, `"reviewers":[{"nickname":"bob","uuid":"{bob}"}]`, 1)))
	})
	h.JSON("GET", "/workspaces/acme/members", 200, `{"values":[{"user":{"uuid":"{alice}","nickname":"alice"}},{"user":{"uuid":"{bob}","nickname":"bob"}}]}`)
	e := NewCmdEdit(h.Factory)
	e.SetArgs([]string{"42", "--title", "New", "--add-reviewer", "alice", "--remove-reviewer", "bob", "--base", "develop"})
	if err := e.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(putBody, `"title":"New"`) || !strings.Contains(putBody, `"reviewers":[{"uuid":"{alice}"}]`) || !strings.Contains(putBody, `"destination":{"branch":{"name":"develop"}}`) {
		t.Errorf("put body: %s", putBody)
	}
	e = NewCmdEdit(h.Factory)
	e.SetArgs([]string{"42"})
	if err := e.Execute(); err == nil {
		t.Error("expected error with nothing to edit")
	}
}

func TestStatus(t *testing.T) {
	h := testutil.NewHarness(t)
	h.JSON("GET", "/user", 200, `{"uuid":"{me}","nickname":"me"}`)
	h.Handle("/repositories/acme/widgets/pullrequests", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		switch {
		case strings.Contains(q, "author.uuid"):
			w.Write([]byte(`{"values":[` + prJSON + `]}`))
		case strings.Contains(q, "reviewers.uuid"):
			w.Write([]byte(`{"values":[]}`))
		default:
			w.Write([]byte(`{"values":[]}`))
		}
	})
	s := NewCmdStatus(h.Factory)
	s.SetArgs([]string{})
	if err := s.Execute(); err != nil {
		t.Fatal(err)
	}
	out := h.Out.String()
	if !strings.Contains(out, "Created by you") || !strings.Contains(out, "#42") || !strings.Contains(out, "no pull requests to review") {
		t.Errorf("status:\n%s", out)
	}
}

func TestDiffColorFlag(t *testing.T) {
	h := testutil.NewHarness(t)
	h.JSON("GET", "/repositories/acme/widgets/pullrequests/42", 200, prJSON)
	h.Handle("/repositories/acme/widgets/pullrequests/42/diff", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("+new\n"))
	})
	d := NewCmdDiff(h.Factory)
	d.SetArgs([]string{"42", "--color", "always"})
	if err := d.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Out.String(), "\x1b[32m+new") {
		t.Errorf("expected colored output: %q", h.Out.String())
	}
	d = NewCmdDiff(h.Factory)
	d.SetArgs([]string{"42", "--color", "sometimes"})
	if err := d.Execute(); err == nil {
		t.Error("expected invalid --color error")
	}
}

func TestDeclineDeleteBranchRefusesFork(t *testing.T) {
	h := testutil.NewHarness(t)
	fork := strings.Replace(prJSON, `"repository":{"full_name":"acme/widgets","workspace":{"slug":"acme"}}`, `"repository":{"full_name":"alice/widgets-fork"}`, 1)
	h.JSON("GET", "/repositories/acme/widgets/pullrequests/42", 200, fork)
	h.JSON("POST", "/repositories/acme/widgets/pullrequests/42/decline", 200, `{}`)
	deleted := false
	h.Handle("/repositories/acme/widgets/refs/branches/feat%2Flogin", func(w http.ResponseWriter, r *http.Request) { deleted = true })
	d := NewCmdDecline(h.Factory)
	d.SetArgs([]string{"42", "--delete-branch"})
	err := d.Execute()
	if err != nil || deleted || !strings.Contains(h.ErrOut.String(), "alice/widgets-fork") {
		t.Errorf("fork branch must not be deleted from base repo, decline still succeeds: err=%v deleted=%v stderr=%s", err, deleted, h.ErrOut.String())
	}
}

func TestReviewActionBeforeComment(t *testing.T) {
	h := testutil.NewHarness(t)
	h.JSON("GET", "/repositories/acme/widgets/pullrequests/42", 200, prJSON)
	h.JSON("POST", "/repositories/acme/widgets/pullrequests/42/approve", 403, `{"type":"error","error":{"message":"forbidden"}}`)
	commented := false
	h.Handle("/repositories/acme/widgets/pullrequests/42/comments", func(w http.ResponseWriter, r *http.Request) { commented = true; w.Write([]byte(`{}`)) })
	rv := NewCmdReview(h.Factory)
	rv.SetArgs([]string{"42", "--approve", "-b", "LGTM"})
	if err := rv.Execute(); err == nil || commented {
		t.Errorf("failed approve must not leave a comment: err=%v commented=%v", err, commented)
	}
}

func TestEditRemoveReviewerWhoLeftWorkspace(t *testing.T) {
	h := testutil.NewHarness(t)
	var putBody string
	h.Handle("/repositories/acme/widgets/pullrequests/42", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			b, _ := io.ReadAll(r.Body)
			putBody = string(b)
		}
		w.Write([]byte(strings.Replace(prJSON, `"reviewers":[{"nickname":"bob"}]`, `"reviewers":[{"nickname":"bob","uuid":"{bob}"},{"nickname":"gone","uuid":"{gone}"}]`, 1)))
	})
	// members endpoint no longer lists "gone"; it must not be consulted for them
	h.JSON("GET", "/workspaces/acme/members", 200, `{"values":[{"user":{"uuid":"{bob}","nickname":"bob"}}]}`)
	e := NewCmdEdit(h.Factory)
	e.SetArgs([]string{"42", "--remove-reviewer", "gone"})
	if err := e.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(putBody, `"reviewers":[{"uuid":"{bob}"}]`) {
		t.Errorf("put body: %s", putBody)
	}
}
