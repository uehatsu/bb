package pipeline

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

const runJSON = `{"uuid":"{p1}","build_number":128,"state":{"name":"IN_PROGRESS","stage":{"name":"RUNNING"}},"target":{"type":"pipeline_ref_target","ref_type":"branch","ref_name":"main","commit":{"hash":"abcdef1234"}},"trigger":{"name":"PUSH"},"created_on":"2026-08-25T00:00:00Z","links":{"html":{"href":"https://bitbucket.org/acme/widgets/pipelines/results/128"}}}`

func doneJSON(result string) string {
	return strings.Replace(runJSON, `"state":{"name":"IN_PROGRESS","stage":{"name":"RUNNING"}}`, `"state":{"name":"COMPLETED","result":{"name":"`+result+`"}}`, 1)
}

func TestListAndResolve(t *testing.T) {
	h := testutil.NewHarness(t)
	var got *http.Request
	h.Handle("/repositories/acme/widgets/pipelines", func(w http.ResponseWriter, r *http.Request) {
		got = r
		w.Write([]byte(`{"values":[` + doneJSON("SUCCESSFUL") + `]}`))
	})
	cmd := NewCmdList(h.Factory)
	cmd.SetArgs([]string{"--branch", "main", "--status", "failed", "-L", "5"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	q := got.URL.Query()
	if q.Get("sort") != "-created_on" || q.Get("pagelen") != "5" || !strings.Contains(q.Get("q"), `target.branch="main"`) || !strings.Contains(q.Get("q"), `state.result.name="FAILED"`) {
		t.Errorf("query: %v", q)
	}
	if h.Out.String() != "#128\tSUCCESSFUL\tmain\tabcdef1\tpush\t2026-08-25T00:00:00Z\n" {
		t.Errorf("out: %q", h.Out.String())
	}

	h.JSON("GET", "/repositories/acme/widgets/pipelines/{p1}/steps", 200, `{"values":[{"uuid":"{s1}","name":"Build","state":{"name":"COMPLETED","result":{"name":"SUCCESSFUL"}},"duration_in_seconds":42}]}`)
	h.Out.Reset()
	v := NewCmdView(h.Factory)
	v.SetArgs([]string{"128"})
	if err := v.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.URL.Query().Get("q"), "build_number=128") {
		t.Errorf("resolve by number query: %s", got.URL.RawQuery)
	}
	if !strings.Contains(h.Out.String(), "Pipeline #128 SUCCESSFUL") || !strings.Contains(h.Out.String(), "Build\tSUCCESSFUL\t42s") {
		t.Errorf("view: %s", h.Out.String())
	}
	v = NewCmdView(h.Factory)
	v.SetArgs([]string{"128", "--json", "buildNumber,result"})
	h.Out.Reset()
	_ = v.Execute()
	if h.Out.String() != `{"buildNumber":128,"result":"SUCCESSFUL"}`+"\n" {
		t.Errorf("json: %s", h.Out.String())
	}
}

func TestRunAndWatch(t *testing.T) {
	h := testutil.NewHarness(t)
	var body string
	var polls int32
	h.Handle("/repositories/acme/widgets/pipelines", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(201)
		w.Write([]byte(runJSON))
	})
	h.Handle("/repositories/acme/widgets/pipelines/{p1}", func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&polls, 1) < 2 {
			w.Write([]byte(runJSON))
			return
		}
		w.Write([]byte(doneJSON("FAILED")))
	})
	cmd := NewCmdRun(h.Factory)
	cmd.SetArgs([]string{"--branch", "main", "--custom", "deploy", "--var", "ENV=prod", "--watch", "--"})
	err := cmd.Execute()
	if !errors.Is(err, cmdutil.ErrSilent) {
		t.Fatalf("expected ErrSilent for failed pipeline, got %v", err)
	}
	for _, want := range []string{`"ref_type":"branch"`, `"ref_name":"main"`, `"type":"pipeline_ref_target"`, `"selector":{"pattern":"deploy","type":"custom"}`, `"variables":[{"key":"ENV","value":"prod"}]`} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %s: %s", want, body)
		}
	}
	if polls < 2 || !strings.Contains(h.ErrOut.String(), "finished with FAILED") {
		t.Errorf("polls=%d err=%s", polls, h.ErrOut.String())
	}
	if !strings.Contains(h.Out.String(), "pipelines/results/128") {
		t.Errorf("out: %s", h.Out.String())
	}
	cmd = NewCmdRun(h.Factory)
	cmd.SetArgs([]string{"--branch", "x", "--tag", "y"})
	if err := cmd.Execute(); err == nil {
		t.Error("branch+tag should conflict")
	}
}

func TestStopAndLog(t *testing.T) {
	h := testutil.NewHarness(t)
	h.JSON("GET", "/repositories/acme/widgets/pipelines/{p1}", 200, runJSON)
	stopped := false
	h.Handle("/repositories/acme/widgets/pipelines/{p1}/stopPipeline", func(w http.ResponseWriter, r *http.Request) {
		stopped = r.Method == "POST"
		w.WriteHeader(204)
	})
	s := NewCmdStop(h.Factory)
	s.SetArgs([]string{"{p1}"})
	if err := s.Execute(); err != nil || !stopped {
		t.Fatalf("stop: %v %v", err, stopped)
	}

	h.JSON("GET", "/repositories/acme/widgets/pipelines/{p1}/steps", 200, `{"values":[{"uuid":"{s1}","name":"Build","state":{"name":"COMPLETED"}},{"uuid":"{s2}","name":"Test","state":{"name":"COMPLETED"}}]}`)
	var ranges []string
	h.Handle("/repositories/acme/widgets/pipelines/{p1}/steps/{step}/log", func(w http.ResponseWriter, r *http.Request) {
		switch r.PathValue("step") {
		case "{s1}":
			ranges = append(ranges, r.Header.Get("Range"))
			w.Write([]byte("line1\nline2\n"))
		case "{s2}":
			w.Write([]byte("tests ok\n"))
		default:
			w.WriteHeader(404)
		}
	})
	l := NewCmdLog(h.Factory)
	l.SetArgs([]string{"{p1}"})
	if err := l.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(h.Out.String(), "==> Build\nline1\nline2\n==> Test\ntests ok\n") {
		t.Errorf("log: %q", h.Out.String())
	}
	h.Out.Reset()
	l = NewCmdLog(h.Factory)
	l.SetArgs([]string{"{p1}", "--step", "2"})
	_ = l.Execute()
	if h.Out.String() != "tests ok\n" {
		t.Errorf("step log: %q", h.Out.String())
	}
	l = NewCmdLog(h.Factory)
	l.SetArgs([]string{"{p1}", "--step", "5"})
	if err := l.Execute(); err == nil {
		t.Error("expected out-of-range error")
	}
	if len(ranges) == 0 || ranges[0] != "" {
		t.Errorf("first fetch must not send Range: %v", ranges)
	}
}
