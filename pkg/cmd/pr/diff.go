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
)

// NewCmdDiff returns `pr diff`.
func NewCmdDiff(f *cmdutil.Factory) *cobra.Command {
	var nameOnly, stat, color bool
	cmd := &cobra.Command{
		Use:   "diff [<number> | <branch> | <url>]",
		Short: "View changes in a pull request",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sel := ""
			if len(args) > 0 {
				sel = args[0]
			}
			return withPR(f, sel, func(ctx context.Context, c *api.Client, repo cmdutil.Repo, prID int, title, state string) error {
				ios := f.IOStreams
				if nameOnly || stat {
					var entries []bitbucket.DiffStat
					if err := api.Paginate(ctx, c, prPath(repo, prID, "diffstat"), api.ListOptions{}, func(d bitbucket.DiffStat) error {
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
				if color || (ios.ColorEnabled() && cmd.Flags().Lookup("color").Value.String() != "false") {
					return colorizeDiff(resp.Body, ios.Out, ios.ColorScheme())
				}
				_, err = io.Copy(ios.Out, resp.Body)
				return err
			})
		},
	}
	cmd.Flags().BoolVar(&nameOnly, "name-only", false, "Output only names of changed files")
	cmd.Flags().BoolVar(&stat, "stat", false, "Output a diffstat summary")
	cmd.Flags().BoolVar(&color, "color", false, "Force colored diff output")
	return cmd
}

func colorizeDiff(r io.Reader, w io.Writer, cs interface {
	Green(string) string
	Red(string) string
	Cyan(string) string
	Bold(string) string
}) error {
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
