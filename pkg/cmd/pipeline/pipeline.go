// Package pipeline implements `bb pipeline` (Bitbucket Pipelines).
package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/bitbucket"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/output"
)

// NewCmdPipeline returns the `pipeline` command group.
func NewCmdPipeline(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "pipeline <command>",
		Aliases: []string{"run", "pipelines"},
		Short:   "Run and monitor Bitbucket Pipelines",
		Example: `  $ bb pipeline list
  $ bb pipeline run --branch main
  $ bb pipeline watch 128
  $ bb pipeline log 128`,
	}
	cmdutil.EnableRepoOverride(cmd, f)
	cmd.AddCommand(
		NewCmdList(f),
		NewCmdView(f),
		NewCmdRun(f),
		NewCmdStop(f),
		NewCmdWatch(f),
		NewCmdLog(f),
	)
	return cmd
}

const pipelineFields = "values.uuid,values.build_number,values.state,values.target,values.creator.nickname,values.creator.display_name,values.trigger.name,values.created_on,values.completed_on,values.build_seconds_used,values.links.html.href,next"

func basePath(repo cmdutil.Repo) string {
	return fmt.Sprintf("/repositories/%s/%s/pipelines", repo.Workspace, repo.Slug)
}

// resolvePipeline accepts a build number or a {uuid}.
func resolvePipeline(ctx context.Context, c *api.Client, repo cmdutil.Repo, selector string) (*bitbucket.Pipeline, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, errors.New("pipeline build number or UUID required")
	}
	if strings.HasPrefix(selector, "{") || strings.Contains(selector, "-") {
		if !strings.HasPrefix(selector, "{") {
			selector = "{" + selector + "}"
		}
		var p bitbucket.Pipeline
		if _, err := c.Do(ctx, api.Request{Path: basePath(repo) + "/" + selector}, &p); err != nil {
			return nil, err
		}
		return &p, nil
	}
	n, err := strconv.Atoi(strings.TrimPrefix(selector, "#"))
	if err != nil || n <= 0 {
		return nil, fmt.Errorf("invalid pipeline %q: expected a build number or UUID", selector)
	}
	var found *bitbucket.Pipeline
	err = api.Paginate(ctx, c, basePath(repo), api.ListOptions{Limit: 1, Query: fmt.Sprintf("build_number=%d", n), Fields: pipelineFields}, func(p bitbucket.Pipeline) error {
		found = &p
		return api.ErrStop
	})
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, fmt.Errorf("pipeline #%d not found in %s", n, repo.FullName())
	}
	return found, nil
}

func statusText(p *bitbucket.Pipeline) string {
	if r := p.State.ResultName(); r != "" {
		return r
	}
	if p.State.Stage != nil && p.State.Stage.Name != "" {
		return p.State.Stage.Name
	}
	return p.State.Name
}

func statusColor(cs interface {
	Green(string) string
	Red(string) string
	Yellow(string) string
	Gray(string) string
}, p *bitbucket.Pipeline) func(string) string {
	switch statusText(p) {
	case "SUCCESSFUL":
		return cs.Green
	case "FAILED", "ERROR":
		return cs.Red
	case "STOPPED", "EXPIRED":
		return cs.Gray
	}
	return cs.Yellow
}

func isDone(p *bitbucket.Pipeline) bool { return p.State.Name == "COMPLETED" }

func printPipelineRow(tp *output.TablePrinter, cs interface {
	Green(string) string
	Red(string) string
	Yellow(string) string
	Gray(string) string
	Cyan(string) string
	Bold(string) string
}, p *bitbucket.Pipeline, now time.Time) {
	tp.AddField(fmt.Sprintf("#%d", p.BuildNumber), cs.Bold)
	tp.AddField(statusText(p), statusColor(cs, p))
	tp.AddField(p.Target.RefName, cs.Cyan)
	commit := ""
	if p.Target.Commit != nil {
		commit = p.Target.Commit.ShortHash()
	}
	tp.AddField(commit, cs.Gray)
	trigger := ""
	if p.Trigger != nil {
		trigger = strings.ToLower(p.Trigger.Name)
	}
	tp.AddField(trigger, nil)
	if tp.IsTTY() {
		tp.AddField(output.TimeAgo(now, p.CreatedOn), cs.Gray)
	} else {
		tp.AddField(p.CreatedOn.Format(time.RFC3339), nil)
	}
	tp.EndRow()
}

// NewCmdList returns `pipeline list`.
func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	var limit int
	var branch, status string
	var buildExporter func() (*output.Exporter, error)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent pipeline runs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			exporter, err := buildExporter()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			var clauses []string
			if branch != "" {
				clauses = append(clauses, "target.branch="+api.BBQLQuote(branch))
			}
			if status != "" {
				s := strings.ToUpper(status)
				switch s {
				case "PENDING", "IN_PROGRESS", "COMPLETED":
					clauses = append(clauses, "state.name="+api.BBQLQuote(s))
				default:
					clauses = append(clauses, "state.result.name="+api.BBQLQuote(s))
				}
			}
			var runs []bitbucket.Pipeline
			if err := api.Paginate(ctx, client, basePath(repo), api.ListOptions{Limit: limit, Sort: "-created_on", Fields: pipelineFields, Query: api.BBQLAnd(clauses...)}, func(p bitbucket.Pipeline) error {
				runs = append(runs, p)
				return nil
			}); err != nil {
				return err
			}
			ios := f.IOStreams
			if exporter != nil {
				return exporter.Write(ios, bitbucket.PipelineFields.ExportAll(runs, exporter.Fields))
			}
			if len(runs) == 0 {
				fmt.Fprintf(ios.ErrOut, "No pipelines found in %s\n", repo.FullName())
				return nil
			}
			tp := output.NewTablePrinter(ios)
			cs := ios.ColorScheme()
			now := time.Now()
			for i := range runs {
				printPipelineRow(tp, cs, &runs[i], now)
			}
			return tp.Render()
		},
	}
	cmd.Flags().IntVarP(&limit, "limit", "L", 20, "Maximum number of pipelines to list")
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Filter by branch")
	cmd.Flags().StringVarP(&status, "status", "s", "", "Filter by status: pending|in_progress|completed|successful|failed|error|stopped")
	buildExporter = cmdutil.AddJSONFlags(cmd, bitbucket.PipelineFields.Validate, bitbucket.PipelineFields.Names())
	return cmd
}

// NewCmdView returns `pipeline view`.
func NewCmdView(f *cmdutil.Factory) *cobra.Command {
	var web bool
	var buildExporter func() (*output.Exporter, error)
	cmd := &cobra.Command{
		Use:   "view <number|uuid>",
		Short: "View a pipeline run and its steps",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			exporter, err := buildExporter()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			p, err := resolvePipeline(ctx, client, repo, args[0])
			if err != nil {
				return err
			}
			ios := f.IOStreams
			if web {
				u := p.Links.HTML()
				if u == "" {
					u = fmt.Sprintf("https://bitbucket.org/%s/pipelines/results/%d", repo.FullName(), p.BuildNumber)
				}
				return cmdutil.OpenBrowser(f, u)
			}
			if exporter != nil {
				return exporter.Write(ios, bitbucket.PipelineFields.Export(*p, exporter.Fields))
			}
			steps, err := fetchSteps(ctx, client, repo, p.UUID)
			if err != nil {
				return err
			}
			printPipeline(f, repo, p, steps)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open the pipeline in the browser")
	buildExporter = cmdutil.AddJSONFlags(cmd, bitbucket.PipelineFields.Validate, bitbucket.PipelineFields.Names())
	return cmd
}

func fetchSteps(ctx context.Context, c *api.Client, repo cmdutil.Repo, uuid string) ([]bitbucket.PipelineStep, error) {
	var steps []bitbucket.PipelineStep
	err := api.Paginate(ctx, c, basePath(repo)+"/"+uuid+"/steps", api.ListOptions{Fields: "values.uuid,values.name,values.state,values.started_on,values.completed_on,values.duration_in_seconds,next"}, func(s bitbucket.PipelineStep) error {
		steps = append(steps, s)
		return nil
	})
	return steps, err
}

func printPipeline(f *cmdutil.Factory, repo cmdutil.Repo, p *bitbucket.Pipeline, steps []bitbucket.PipelineStep) {
	ios := f.IOStreams
	cs := ios.ColorScheme()
	fmt.Fprintf(ios.Out, "%s %s\n", cs.Bold(fmt.Sprintf("Pipeline #%d", p.BuildNumber)), statusColor(cs, p)(statusText(p)))
	fmt.Fprintf(ios.Out, "%s %s", cs.Gray("Target:"), p.Target.RefName)
	if p.Target.Commit != nil {
		fmt.Fprintf(ios.Out, " (%s)", p.Target.Commit.ShortHash())
	}
	fmt.Fprintln(ios.Out)
	if p.Target.Selector != nil {
		fmt.Fprintf(ios.Out, "%s %s: %s\n", cs.Gray("Selector:"), p.Target.Selector.Type, p.Target.Selector.Pattern)
	}
	if p.Creator != nil {
		fmt.Fprintf(ios.Out, "%s %s\n", cs.Gray("Started by:"), p.Creator.Name())
	}
	if !p.CreatedOn.IsZero() {
		fmt.Fprintf(ios.Out, "%s %s\n", cs.Gray("Created:"), p.CreatedOn.Format("2006-01-02 15:04:05"))
	}
	if p.BuildSecondsUsed > 0 {
		fmt.Fprintf(ios.Out, "%s %s\n", cs.Gray("Duration:"), (time.Duration(p.BuildSecondsUsed) * time.Second).String())
	}
	if len(steps) > 0 {
		fmt.Fprintln(ios.Out)
		tp := output.NewTablePrinter(ios)
		for i, s := range steps {
			result := s.State.ResultName()
			if result == "" {
				result = s.State.Name
			}
			color := cs.Yellow
			switch result {
			case "SUCCESSFUL":
				color = cs.Green
			case "FAILED", "ERROR":
				color = cs.Red
			case "STOPPED", "EXPIRED", "NOT_RUN":
				color = cs.Gray
			}
			name := s.Name
			if name == "" {
				name = fmt.Sprintf("step %d", i+1)
			}
			tp.AddField(fmt.Sprintf("%d", i+1), cs.Gray)
			tp.AddField(name, nil)
			tp.AddField(result, color)
			if s.DurationSec > 0 {
				tp.AddField((time.Duration(s.DurationSec) * time.Second).String(), cs.Gray)
			} else {
				tp.AddField("", nil)
			}
			tp.AddField(s.UUID, cs.Gray)
			tp.EndRow()
		}
		_ = tp.Render()
	}
	fmt.Fprintln(ios.Out)
	u := p.Links.HTML()
	if u == "" {
		u = fmt.Sprintf("https://bitbucket.org/%s/pipelines/results/%d", repo.FullName(), p.BuildNumber)
	}
	fmt.Fprintln(ios.Out, cs.Gray("View this pipeline on Bitbucket: "+u))
}

// NewCmdRun returns `pipeline run`.
func NewCmdRun(f *cmdutil.Factory) *cobra.Command {
	var branch, tag, commit, custom string
	var vars []string
	var watch bool
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Trigger a pipeline",
		Long: `Trigger a pipeline for a branch (default: the current branch), tag, or
commit. Use --custom to run a custom pipeline defined under "custom:" in
bitbucket-pipelines.yml, and --var KEY=VALUE to pass pipeline variables.`,
		Example: `  $ bb pipeline run
  $ bb pipeline run --branch main --custom deploy --var ENV=prod
  $ bb pipeline run --tag v1.2.0 --watch`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			n := 0
			for _, s := range []string{branch, tag} {
				if s != "" {
					n++
				}
			}
			if n > 1 {
				return cmdutil.FlagErrorf("specify only one of --branch or --tag")
			}
			ctx := cmd.Context()
			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			if branch == "" && tag == "" && commit == "" {
				if f.GitClient == nil {
					return cmdutil.FlagErrorf("--branch or --tag is required")
				}
				g, err := f.GitClient()
				if err != nil {
					return cmdutil.FlagErrorf("--branch or --tag is required outside a git repository")
				}
				branch, err = g.CurrentBranch(ctx)
				if err != nil {
					return fmt.Errorf("could not determine current branch: %w", err)
				}
			}
			// Bitbucket target shapes: pipeline_ref_target {ref_type, ref_name[, commit]}
			// or pipeline_commit_target {commit} when only a commit is given.
			var target map[string]any
			switch {
			case commit != "" && !cmd.Flags().Changed("branch") && !cmd.Flags().Changed("tag"):
				target = map[string]any{"type": "pipeline_commit_target", "commit": map[string]string{"type": "commit", "hash": commit}}
			case tag != "":
				target = map[string]any{"type": "pipeline_ref_target", "ref_type": "tag", "ref_name": tag}
			default:
				target = map[string]any{"type": "pipeline_ref_target", "ref_type": "branch", "ref_name": branch}
			}
			if commit != "" && target["type"] == "pipeline_ref_target" {
				target["commit"] = map[string]string{"type": "commit", "hash": commit}
			}
			if custom != "" {
				target["selector"] = map[string]string{"type": "custom", "pattern": custom}
			}
			payload := map[string]any{"target": target}
			if len(vars) > 0 {
				var vs []map[string]any
				for _, v := range vars {
					k, val, ok := strings.Cut(v, "=")
					if !ok {
						return cmdutil.FlagErrorf("--var must be KEY=VALUE, got %q", v)
					}
					vs = append(vs, map[string]any{"key": k, "value": val})
				}
				payload["variables"] = vs
			}
			var created bitbucket.Pipeline
			if _, err := client.Do(ctx, api.Request{Method: "POST", Path: basePath(repo), Body: payload}, &created); err != nil {
				return err
			}
			cs := f.IOStreams.ColorScheme()
			fmt.Fprintf(f.IOStreams.ErrOut, "%s Started pipeline #%d for %s\n", cs.SuccessIcon(), created.BuildNumber, created.Target.RefName)
			u := created.Links.HTML()
			if u == "" {
				u = fmt.Sprintf("https://bitbucket.org/%s/pipelines/results/%d", repo.FullName(), created.BuildNumber)
			}
			fmt.Fprintln(f.IOStreams.Out, u)
			if watch {
				return watchPipeline(ctx, f, client, repo, created.UUID, 0, 0, true)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Branch to run the pipeline for (default: current branch)")
	cmd.Flags().StringVarP(&tag, "tag", "t", "", "Tag to run the pipeline for")
	cmd.Flags().StringVarP(&commit, "commit", "c", "", "Specific commit hash to build")
	cmd.Flags().StringVar(&custom, "custom", "", "Name of a custom pipeline to run")
	cmd.Flags().StringArrayVar(&vars, "var", nil, "Pipeline variable KEY=VALUE (repeatable)")
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Watch the pipeline until it completes")
	return cmd
}

// NewCmdStop returns `pipeline stop`.
func NewCmdStop(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:     "stop <number|uuid>",
		Aliases: []string{"cancel"},
		Short:   "Stop a running pipeline",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			p, err := resolvePipeline(ctx, client, repo, args[0])
			if err != nil {
				return err
			}
			if isDone(p) {
				return fmt.Errorf("pipeline #%d has already completed (%s)", p.BuildNumber, statusText(p))
			}
			if _, err := client.Do(ctx, api.Request{Method: "POST", Path: basePath(repo) + "/" + p.UUID + "/stopPipeline"}, nil); err != nil {
				return err
			}
			fmt.Fprintf(f.IOStreams.ErrOut, "%s Requested stop of pipeline #%d\n", f.IOStreams.ColorScheme().SuccessIcon(), p.BuildNumber)
			return nil
		},
	}
}

// NewCmdWatch returns `pipeline watch`.
func NewCmdWatch(f *cmdutil.Factory) *cobra.Command {
	var interval, timeout time.Duration
	var exitStatus bool
	cmd := &cobra.Command{
		Use:   "watch <number|uuid>",
		Short: "Watch a pipeline until it completes",
		Long:  "Poll a pipeline until it completes. The exit code is 1 when the pipeline did not succeed.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			p, err := resolvePipeline(ctx, client, repo, args[0])
			if err != nil {
				return err
			}
			return watchPipeline(ctx, f, client, repo, p.UUID, interval, timeout, exitStatus)
		},
	}
	cmd.Flags().DurationVarP(&interval, "interval", "i", 0, "Initial polling interval (default 3s, grows to 30s)")
	cmd.Flags().DurationVar(&timeout, "timeout", 2*time.Hour, "Give up after this long")
	cmd.Flags().BoolVar(&exitStatus, "exit-status", true, "Exit with non-zero status if the pipeline fails (use --exit-status=false to disable)")
	return cmd
}

// watchPipeline polls until completion. When exitStatus is true a
// non-successful result yields a non-zero exit code.
func watchPipeline(ctx context.Context, f *cmdutil.Factory, client *api.Client, repo cmdutil.Repo, uuid string, interval, timeout time.Duration, exitStatus bool) error {
	ios := f.IOStreams
	cs := ios.ColorScheme()
	if interval <= 0 {
		interval = 3 * time.Second
	}
	var last *bitbucket.Pipeline
	lastStatus := ""
	err := api.Poll(ctx, api.PollOptions{Initial: interval, Max: 30 * time.Second, Timeout: timeout}, func(ctx context.Context) (bool, error) {
		var p bitbucket.Pipeline
		if _, err := client.Do(ctx, api.Request{Path: basePath(repo) + "/" + uuid}, &p); err != nil {
			return false, err
		}
		last = &p
		if s := statusText(&p); s != lastStatus {
			fmt.Fprintf(ios.ErrOut, "%s Pipeline #%d: %s\n", cs.Gray(time.Now().Format("15:04:05")), p.BuildNumber, statusColor(cs, &p)(s))
			lastStatus = s
		}
		return isDone(&p), nil
	})
	if err != nil {
		return err
	}
	result := last.State.ResultName()
	switch result {
	case "SUCCESSFUL":
		fmt.Fprintf(ios.ErrOut, "%s Pipeline #%d succeeded\n", cs.SuccessIcon(), last.BuildNumber)
		return nil
	default:
		fmt.Fprintf(ios.ErrOut, "%s Pipeline #%d finished with %s\n", cs.FailureIcon(), last.BuildNumber, result)
		if !exitStatus {
			return nil
		}
		return cmdutil.ErrSilent
	}
}

// NewCmdLog returns `pipeline log`.
func NewCmdLog(f *cmdutil.Factory) *cobra.Command {
	var step int
	var follow bool
	cmd := &cobra.Command{
		Use:   "log <number|uuid>",
		Short: "Print the log of a pipeline step",
		Long: `Print the log output of a pipeline's steps. By default all steps are
printed in order; use --step N to select one. With --follow the log is
tailed (using HTTP Range requests) until the step completes.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			repo, err := f.BaseRepo()
			if err != nil {
				return err
			}
			client, err := f.APIClient()
			if err != nil {
				return err
			}
			p, err := resolvePipeline(ctx, client, repo, args[0])
			if err != nil {
				return err
			}
			steps, err := fetchSteps(ctx, client, repo, p.UUID)
			if err != nil {
				return err
			}
			if len(steps) == 0 {
				return fmt.Errorf("pipeline #%d has no steps yet", p.BuildNumber)
			}
			if step > 0 {
				if step > len(steps) {
					return fmt.Errorf("pipeline #%d has only %d steps", p.BuildNumber, len(steps))
				}
				steps = steps[step-1 : step]
			}
			ios := f.IOStreams
			cs := ios.ColorScheme()
			if !follow {
				if err := ios.StartPager(); err == nil {
					defer ios.StopPager()
				}
			}
			for i, s := range steps {
				if len(steps) > 1 || step == 0 {
					name := s.Name
					if name == "" {
						name = fmt.Sprintf("step %d", i+1)
					}
					fmt.Fprintf(ios.Out, "%s\n", cs.Bold("==> "+name))
				}
				if err := streamStepLog(ctx, client, repo, p.UUID, s.UUID, follow, ios.Out); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&step, "step", "s", 0, "Step number to show (1-based)")
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Keep fetching new log output until the step finishes")
	return cmd
}

// streamStepLog fetches a step log, using Range requests to append when following.
func streamStepLog(ctx context.Context, client *api.Client, repo cmdutil.Repo, pipelineUUID, stepUUID string, follow bool, out interface{ Write([]byte) (int, error) }) error {
	path := fmt.Sprintf("%s/%s/steps/%s/log", basePath(repo), pipelineUUID, stepUUID)
	offset := 0
	fetch := func() (int, bool, error) {
		req := api.Request{Path: path, Accept: "*/*"}
		if offset > 0 {
			req.Headers = map[string]string{"Range": fmt.Sprintf("bytes=%d-", offset)}
		}
		resp, err := client.DoRaw(ctx, req)
		if err != nil {
			var herr *api.HTTPError
			if errors.As(err, &herr) && (herr.StatusCode == 416 || herr.IsNotFound()) {
				return 0, false, nil // nothing new yet
			}
			return 0, false, err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK && offset > 0 {
			// Server ignored the Range header and returned the whole log:
			// skip what was already printed.
			if _, err := io.CopyN(io.Discard, resp.Body, int64(offset)); err != nil && err != io.EOF {
				return 0, false, err
			}
		}
		n, err := io.Copy(out, resp.Body)
		if err != nil {
			return int(n), false, fmt.Errorf("reading log: %w", err)
		}
		return int(n), true, nil
	}
	n, _, err := fetch()
	if err != nil {
		return err
	}
	offset += n
	if !follow {
		return nil
	}
	return api.Poll(ctx, api.PollOptions{Initial: 3 * time.Second, Max: 10 * time.Second}, func(ctx context.Context) (bool, error) {
		n, _, err := fetch()
		if err != nil {
			return false, err
		}
		offset += n
		var s bitbucket.PipelineStep
		if _, err := client.Do(ctx, api.Request{Path: fmt.Sprintf("%s/%s/steps/%s", basePath(repo), pipelineUUID, stepUUID)}, &s); err != nil {
			return false, err
		}
		return s.State.Name == "COMPLETED", nil
	})
}
