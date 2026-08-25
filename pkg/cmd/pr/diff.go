package pr

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/iostreams"
)

// NewCmdDiff returns `pr diff`.
func NewCmdDiff(f *cmdutil.Factory) *cobra.Command {
	var nameOnly, stat bool
	var color string
	cmd := &cobra.Command{
		Use:   "diff [<number> | <branch> | <url>]",
		Short: "View changes in a pull request",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch color {
			case "always", "never", "auto":
			default:
				return cmdutil.FlagErrorf("invalid --color %q (always|never|auto)", color)
			}
			sel := ""
			if len(args) > 0 {
				sel = args[0]
			}
			return withPR(cmd.Context(), f, sel, func(ctx context.Context, c *api.Client, repo cmdutil.Repo, pr *bitbucket.PullRequest) error {
				ios := f.IOStreams
				prID := pr.ID
				useColor := color == "always" || (color == "auto" && ios.ColorEnabled())
				if nameOnly || stat {
					var entries []bitbucket.DiffStat
					if err := api.Paginate(ctx, c, prPath(repo, prID, "diffstat"), api.ListOptions{Fields: "values.status,values.lines_added,values.lines_removed,values.old.path,values.new.path,next"}, func(d bitbucket.DiffStat) error {
						entries = append(entries, d)
						return nil
					}); err != nil {
						return err
					}
					cs := ios.ColorScheme()
					for _, e := range entries {
						if nameOnly {
							fmt.Fprintln(ios.Out, e.Path())
						} else {
							fmt.Fprintf(ios.Out, "%-10s %s %s\n", e.Status, e.Path(), cs.Green(fmt.Sprintf("+%d", e.LinesAdded))+" "+cs.Red(fmt.Sprintf("-%d", e.LinesRemoved)))
						}
					}
					return nil
				}
				resp, err := c.DoRaw(ctx, api.Request{Path: prPath(repo, prID, "diff"), Accept: "text/plain"})
				if err != nil {
					return err
				}
				defer resp.Body.Close()
				if err := ios.StartPager(); err == nil {
					defer ios.StopPager()
				}
				if useColor {
					return colorizeDiff(resp.Body, ios.Out, iostreams.NewColorScheme(true))
				}
				_, err = io.Copy(ios.Out, resp.Body)
				return err
			})
		},
	}
	cmd.Flags().BoolVar(&nameOnly, "name-only", false, "Output only names of changed files")
	cmd.Flags().BoolVar(&stat, "stat", false, "Output a diffstat summary")
	cmd.Flags().StringVar(&color, "color", "auto", "Use color in diff output: {always|never|auto}")
	return cmd
}

func colorizeDiff(r io.Reader, w io.Writer, cs *iostreams.ColorScheme) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	for _, line := range strings.SplitAfter(string(b), "\n") {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"), strings.HasPrefix(line, "diff "):
			line = cs.Bold(line)
		case strings.HasPrefix(line, "@@"):
			line = cs.Cyan(line)
		case strings.HasPrefix(line, "+"):
			line = cs.Green(line)
		case strings.HasPrefix(line, "-"):
			line = cs.Red(line)
		}
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
	}
	return nil
}
