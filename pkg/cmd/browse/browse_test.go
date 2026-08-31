package browse

import (
	"testing"

	"github.com/uehatsu/bb/internal/cmdutil"
)

func TestBuildURL(t *testing.T) {
	repo := cmdutil.Repo{Workspace: "acme", Slug: "widgets"}
	cases := []struct {
		opts Options
		want string
	}{
		{Options{}, "https://bitbucket.org/acme/widgets"},
		{Options{Selector: "42"}, "https://bitbucket.org/acme/widgets/pull-requests/42"},
		{Options{Selector: "src/main.go"}, "https://bitbucket.org/acme/widgets/src/HEAD/src/main.go"},
		{Options{Selector: "src/main.go:10-20", Branch: "feat/x"}, "https://bitbucket.org/acme/widgets/src/feat/x/src/main.go#lines-10:20"},
		{Options{Branch: "develop"}, "https://bitbucket.org/acme/widgets/branch/develop"},
		{Options{Settings: true}, "https://bitbucket.org/acme/widgets/admin"},
		{Options{PullRequests: true}, "https://bitbucket.org/acme/widgets/pull-requests/"},
		{Options{Pipelines: true}, "https://bitbucket.org/acme/widgets/pipelines"},
		{Options{Commit: "abc1234"}, "https://bitbucket.org/acme/widgets/commits/abc1234"},
		// CJK (multi-byte) path: must be percent-encoded.
		{Options{Selector: "日本語/ファイル.txt"}, "https://bitbucket.org/acme/widgets/src/HEAD/%E6%97%A5%E6%9C%AC%E8%AA%9E/%E3%83%95%E3%82%A1%E3%82%A4%E3%83%AB.txt"},
	}
	for _, c := range cases {
		got, err := BuildURL(repo, &c.opts)
		if err != nil || got != c.want {
			t.Errorf("%+v: got %q err=%v want %q", c.opts, got, err, c.want)
		}
	}
	for _, bad := range []Options{{Selector: "../x"}, {Selector: "/etc"}, {Commit: "zz"}} {
		if _, err := BuildURL(repo, &bad); err == nil {
			t.Errorf("%+v should error", bad)
		}
	}
	if _, err := BuildURL(cmdutil.Repo{Workspace: "a b", Slug: "x"}, &Options{}); err == nil {
		t.Error("invalid workspace should error")
	}
}
