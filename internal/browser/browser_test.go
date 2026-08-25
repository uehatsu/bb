package browser

import (
	"runtime"
	"strings"
	"testing"
)

func TestBrowse(t *testing.T) {
	t.Setenv("BROWSER", "")
	var got []string
	rec := func(name string, args ...string) error { got = append([]string{name}, args...); return nil }

	b := New("")
	b.Run = rec
	if err := b.Browse("https://bitbucket.org/x"); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"darwin": "open", "windows": "rundll32"}[runtime.GOOS]
	if want == "" {
		want = "xdg-open"
	}
	if got[0] != want || got[len(got)-1] != "https://bitbucket.org/x" {
		t.Errorf("default launcher: %v", got)
	}

	b = New("firefox --new-tab")
	b.Run = rec
	_ = b.Browse("https://bitbucket.org/y")
	if strings.Join(got, " ") != "firefox --new-tab https://bitbucket.org/y" {
		t.Errorf("configured command: %v", got)
	}

	t.Setenv("BROWSER", "chromium")
	b = New("firefox")
	b.Run = rec
	_ = b.Browse("https://bitbucket.org/z")
	if got[0] != "chromium" {
		t.Errorf("BROWSER env must win: %v", got)
	}

	if err := b.Browse("file:///etc/passwd"); err == nil {
		t.Error("non-http scheme must be refused")
	}
	if err := b.Browse("::bad"); err == nil {
		t.Error("unparsable URL must be refused")
	}
}
