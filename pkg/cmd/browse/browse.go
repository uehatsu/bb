// Package browse implements `bb browse`.
package browse

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/browser"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/gitctx"
)

// Options for browse.
type Options struct {
	Selector     string
	Branch       string
	Settings     bool
	Pipelines    bool
	PullRequests bool
	Commits      bool
	NoBrowser    bool
	Commit       string
}

// NewCmdBrowse returns the browse command.
func NewCmdBrowse(f *cmdutil.Factory) *cobra.Command {
	opts := &Options{}
	cmd := &cobra.Command{
		Use:   "browse [<number> | <path>]",
		Short: "Open the repository in the browser",
		Long: `Open the current repository on bitbucket.org in the web browser.

A numeric argument opens that pull request; a path opens the file or
directory on the selected branch.`,
		Example: `  $ bb browse
  $ bb browse 42
  $ bb browse src/main.go --branch develop
  $ bb browse --pull-requests
  $ bb browse -n   # print the URL only`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.Selector = args[0]
			}
			n := 0
			for _, b := range []bool{opts.Settings, opts.Pipelines, opts.PullRequests, opts.Commits} {
				if b {
					n++
				}
			}
			if n > 1 || (n == 1 && opts.Selector != "") {
				return cmdutil.FlagErrorf("specify only one of --settings, --pipelines, --pull-requests, --commits, or a selector")
			}
			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			u, err := BuildURL(repo, opts)
			if err != nil {
				return err
			}
			if opts.NoBrowser {
				fmt.Fprintln(f.IOStreams.Out, u)
				return nil
			}
			if f.IOStreams.IsStdoutTTY() {
				fmt.Fprintf(f.IOStreams.ErrOut, "Opening %s in your browser.\n", u)
			}
			cfg, _ := f.Config()
			configured := ""
			if cfg != nil {
				configured, _ = cfg.Get("browser")
			}
			return browser.New(configured).Browse(u)
		},
	}
	cmd.Flags().StringVarP(&opts.Branch, "branch", "b", "", "Select another branch by passing in the branch name")
	cmd.Flags().StringVarP(&opts.Commit, "commit", "c", "", "Open the commit page for the given hash")
	cmd.Flags().BoolVarP(&opts.Settings, "settings", "s", false, "Open repository settings")
	cmd.Flags().BoolVar(&opts.Pipelines, "pipelines", false, "Open the pipelines page")
	cmd.Flags().BoolVarP(&opts.PullRequests, "pull-requests", "p", false, "Open the pull requests page")
	cmd.Flags().BoolVar(&opts.Commits, "commits", false, "Open the commits page")
	cmd.Flags().BoolVarP(&opts.NoBrowser, "no-browser", "n", false, "Print destination URL instead of opening the browser")
	cmdutil.EnableRepoOverride(cmd, f)
	return cmd
}

// BuildURL computes the bitbucket.org URL for the options.
func BuildURL(repo cmdutil.Repo, opts *Options) (string, error) {
	if !gitctx.ValidName(repo.Workspace) || !gitctx.ValidName(repo.Slug) {
		return "", fmt.Errorf("invalid repository %q", repo.FullName())
	}
	base := "https://bitbucket.org/" + url.PathEscape(repo.Workspace) + "/" + url.PathEscape(repo.Slug)
	switch {
	case opts.Settings:
		return base + "/admin", nil
	case opts.Pipelines:
		return base + "/pipelines", nil
	case opts.PullRequests:
		return base + "/pull-requests/", nil
	case opts.Commits:
		return base + "/commits/", nil
	case opts.Commit != "":
		if !isHex(opts.Commit) {
			return "", fmt.Errorf("invalid commit hash %q", opts.Commit)
		}
		return base + "/commits/" + opts.Commit, nil
	}
	if opts.Selector == "" {
		if opts.Branch != "" {
			return base + "/branch/" + escapePath(opts.Branch), nil
		}
		return base, nil
	}
	if n, err := strconv.Atoi(opts.Selector); err == nil && n > 0 {
		return fmt.Sprintf("%s/pull-requests/%d", base, n), nil
	}
	p, line := splitLine(opts.Selector)
	if strings.HasPrefix(p, "/") || strings.Contains(p, "..") {
		return "", fmt.Errorf("invalid path %q", opts.Selector)
	}
	ref := opts.Branch
	if ref == "" {
		ref = "HEAD"
	}
	u := base + "/src/" + escapePath(ref) + "/" + escapePath(p)
	if line != "" {
		u += "#lines-" + line
	}
	return u, nil
}

func escapePath(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

// splitLine separates "path:12" or "path:12-20" into path and line spec.
func splitLine(s string) (string, string) {
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return s, ""
	}
	spec := s[i+1:]
	for _, r := range spec {
		if (r < '0' || r > '9') && r != '-' {
			return s, ""
		}
	}
	if spec == "" {
		return s, ""
	}
	return s[:i], strings.ReplaceAll(spec, "-", ":")
}

func isHex(s string) bool {
	if len(s) < 4 || len(s) > 40 {
		return false
	}
	for _, r := range s {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}
