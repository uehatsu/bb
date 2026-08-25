package repo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/browser"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/output"
)

// NewCmdView returns `repo view`.
func NewCmdView(f *cmdutil.Factory) *cobra.Command {
	var web bool
	var branch string
	var buildExporter func() (*output.Exporter, error)
	cmd := &cobra.Command{
		Use:   "view [<repository>]",
		Short: "View a repository",
		Long:  "Display the description and README of a repository.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			arg := ""
			if len(args) > 0 {
				arg = args[0]
			}
			repo, err := resolveRepoArg(f, arg)
			if err != nil {
				return err
			}
			exporter, err := buildExporter()
			if err != nil {
				return err
			}
			return runView(f, repo, branch, web, exporter)
		},
	}
	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open the repository in the browser")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "View a specific branch of the repository")
	buildExporter = cmdutil.AddJSONFlags(cmd, bitbucket.RepositoryFields.Validate, bitbucket.RepositoryFields.Names())
	cmdutil.EnableRepoOverride(cmd, f)
	return cmd
}

func runView(f *cmdutil.Factory, repo cmdutil.Repo, branch string, web bool, exporter *output.Exporter) error {
	ctx := context.Background()
	ios := f.IOStreams
	if web {
		u := "https://bitbucket.org/" + repo.FullName()
		if branch != "" {
			u += "/branch/" + branch
		}
		if ios.IsStdoutTTY() {
			fmt.Fprintf(ios.ErrOut, "Opening %s in your browser.\n", u)
		}
		cfg, _ := f.Config()
		configured := ""
		if cfg != nil {
			configured, _ = cfg.Get("browser")
		}
		return browser.New(configured).Browse(u)
	}
	client, err := f.APIClient()
	if err != nil {
		return err
	}
	r, err := fetchRepo(ctx, client, repo)
	if err != nil {
		return err
	}
	if exporter != nil {
		return exporter.Write(ios, bitbucket.RepositoryFields.Export(*r, exporter.Fields))
	}

	ref := branch
	if ref == "" && r.MainBranch != nil {
		ref = r.MainBranch.Name
	}
	readme, _ := fetchReadme(ctx, client, repo, ref)

	cs := ios.ColorScheme()
	if err := ios.StartPager(); err == nil {
		defer ios.StopPager()
	}
	fmt.Fprintf(ios.Out, "%s\n", cs.Bold(r.FullName))
	if r.Description != "" {
		fmt.Fprintf(ios.Out, "%s\n", r.Description)
	}
	fmt.Fprintln(ios.Out)
	if r.Project != nil {
		fmt.Fprintf(ios.Out, "%s %s (%s)\n", cs.Gray("Project:"), r.Project.Name, r.Project.Key)
	}
	fmt.Fprintf(ios.Out, "%s %s\n", cs.Gray("Visibility:"), visibility(r))
	if r.MainBranch != nil {
		fmt.Fprintf(ios.Out, "%s %s\n", cs.Gray("Main branch:"), r.MainBranch.Name)
	}
	if r.Language != "" {
		fmt.Fprintf(ios.Out, "%s %s\n", cs.Gray("Language:"), r.Language)
	}
	if r.Parent != nil {
		fmt.Fprintf(ios.Out, "%s %s\n", cs.Gray("Forked from:"), r.Parent.FullName)
	}
	fmt.Fprintf(ios.Out, "%s %s\n", cs.Gray("Fork policy:"), r.ForkPolicy)
	if !r.UpdatedOn.IsZero() {
		fmt.Fprintf(ios.Out, "%s %s\n", cs.Gray("Updated:"), r.UpdatedOn.Format("2006-01-02 15:04"))
	}
	fmt.Fprintln(ios.Out)
	if readme != "" {
		fmt.Fprintln(ios.Out, strings.TrimRight(readme, "\n"))
		fmt.Fprintln(ios.Out)
	} else {
		fmt.Fprintln(ios.Out, cs.Gray("This repository does not have a README"))
		fmt.Fprintln(ios.Out)
	}
	fmt.Fprintf(ios.Out, "%s\n", cs.Gray("View this repository on Bitbucket: "+r.Links.HTML()))
	return nil
}

var readmeNames = []string{"README.md", "README.MD", "readme.md", "README", "README.rst", "README.txt"}

func fetchReadme(ctx context.Context, client *api.Client, repo cmdutil.Repo, ref string) (string, error) {
	if ref == "" {
		return "", errors.New("no ref")
	}
	for _, name := range readmeNames {
		resp, err := client.DoRaw(ctx, api.Request{Path: fmt.Sprintf("/repositories/%s/%s/src/%s/%s", repo.Workspace, repo.Slug, ref, name), Accept: "*/*"})
		if err != nil {
			var herr *api.HTTPError
			if errors.As(err, &herr) && herr.IsNotFound() {
				continue
			}
			return "", err
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		resp.Body.Close()
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	return "", nil
}
