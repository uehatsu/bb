package output

import (
	"strings"
	"testing"
	"time"

	"github.com/uehatsu/bb/internal/iostreams"
)

func TestTableTSV(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	tp := NewTablePrinter(io)
	tp.AddField("1", nil)
	tp.AddField("Fix bug", nil)
	tp.EndRow()
	tp.AddField("22", nil)
	// CJK (multi-byte) sample: TSV output must pass it through untouched.
	tp.AddField("日本語タイトル", nil)
	tp.EndRow()
	if err := tp.Render(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\tFix bug\n22\t日本語タイトル\n" {
		t.Errorf("tsv: %q", out.String())
	}
}

func TestTableTTYAligned(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	io.SetStdoutTTY(true)
	tp := NewTablePrinter(io)
	tp.AddField("1", nil)
	tp.AddField("a", nil)
	tp.EndRow()
	tp.AddField("22", nil)
	// CJK sample: two runes, but four columns wide on a terminal.
	tp.AddField("日本", nil)
	tp.EndRow()
	_ = tp.Render()
	want := "1   a\n22  日本\n"
	if out.String() != want {
		t.Errorf("tty:\n%q\nwant\n%q", out.String(), want)
	}
}

func TestTruncate(t *testing.T) {
	if Truncate("hello world", 8) != "hello..." {
		t.Error(Truncate("hello world", 8))
	}
	if Truncate("short", 10) != "short" {
		t.Error("no truncation expected")
	}
	// CJK sample: truncation counts display width, not bytes.
	if Truncate("日本語の長い文字列", 9) != "日本語..." {
		t.Error(Truncate("日本語の長い文字列", 9))
	}
}

func TestExporterJSONAndJQ(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	data := []map[string]any{{"id": 1, "title": "A"}, {"id": 2, "title": "B"}}
	if err := (&Exporter{Fields: []string{"id"}}).Write(io, data); err != nil {
		t.Fatal(err)
	}
	if out.String() != `[{"id":1,"title":"A"},{"id":2,"title":"B"}]`+"\n" {
		t.Errorf("json: %s", out.String())
	}
	out.Reset()
	if err := (&Exporter{JQ: ".[].title"}).Write(io, data); err != nil {
		t.Fatal(err)
	}
	if out.String() != "A\nB\n" {
		t.Errorf("jq: %q", out.String())
	}
	out.Reset()
	if err := (&Exporter{JQ: "map(.id) | add"}).Write(io, data); err != nil {
		t.Fatal(err)
	}
	if out.String() != "3\n" {
		t.Errorf("jq number: %q", out.String())
	}
	if err := (&Exporter{JQ: ".[[["}).Write(io, data); err == nil {
		t.Error("expected jq parse error")
	}
}

func TestExporterTemplate(t *testing.T) {
	io, _, out, _ := iostreams.Test()
	data := []map[string]any{{"id": 1, "title": "A", "at": "2026-08-25T00:00:00Z"}}
	tmpl := `{{range .}}{{tablerow .id (truncate 3 .title) (timefmt "2006" .at)}}{{end}}`
	if err := (&Exporter{Template: tmpl}).Write(io, data); err != nil {
		t.Fatal(err)
	}
	if out.String() != "1\tA\t2026\n" {
		t.Errorf("template: %q", out.String())
	}
}

func TestTimeAgo(t *testing.T) {
	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	cases := map[time.Duration]string{
		30 * time.Second:     "less than a minute ago",
		5 * time.Minute:      "5 minutes ago",
		time.Hour:            "about 1 hour ago",
		3 * 24 * time.Hour:   "about 3 days ago",
		45 * 24 * time.Hour:  "about 1 month ago",
		800 * 24 * time.Hour: "about 2 years ago",
	}
	for d, want := range cases {
		if got := TimeAgo(now, now.Add(-d)); got != want {
			t.Errorf("%v: got %q want %q", d, got, want)
		}
	}
	if !strings.Contains(TimeAgo(now, now.Add(-90*time.Second)), "1 minute") {
		t.Error("singular minute")
	}
}

func TestWriteLine(t *testing.T) {
	var b strings.Builder
	WriteLine(&b, "a")
	WriteLine(&b, "b\n")
	if b.String() != "a\nb\n" {
		t.Errorf("%q", b.String())
	}
	// CJK sample: two runes.
	if RuneLen("日本") != 2 {
		t.Error("RuneLen")
	}
}
