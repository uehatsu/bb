// Package api implements `bb api`, a low-level escape hatch for any
// Bitbucket REST endpoint.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	bbapi "github.com/uehatsu/bb/internal/api"
	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/output"
)

// Options for `bb api`.
type Options struct {
	Path      string
	Method    string
	RawFields []string
	Fields    []string
	Headers   []string
	InputFile string
	Paginate  bool
	Include   bool
	Silent    bool
	JQ        string
	Template  string
}

// NewCmdAPI returns the api command.
func NewCmdAPI(f *cmdutil.Factory) *cobra.Command {
	opts := &Options{}
	cmd := &cobra.Command{
		Use:   "api <endpoint>",
		Short: "Make an authenticated Bitbucket API request",
		Long: `Makes an authenticated HTTP request to the Bitbucket Cloud REST API 2.0 and
prints the response.

The endpoint is a path relative to https://api.bitbucket.org/2.0, e.g.
"/user" or "repositories/{workspace}/{repo_slug}". The placeholders
{workspace} and {repo_slug} (or {repo}) are replaced with the current
repository (from --repo, BB_REPO, or git remotes).

The default method is GET, or POST when any fields are given. Use -f/--raw-field
for string values and -F/--field for typed values: "true", "false", "null",
integers are converted; "@file" reads the value from a file ("@-" for stdin).
For GET requests fields become query parameters; otherwise they are sent as a
JSON body.

With --paginate, all pages are fetched by following "next" links and the
"values" arrays are concatenated into a single JSON array.`,
		Example: `  $ bb api /user
  $ bb api repositories/{workspace}/{repo_slug}/pullrequests --paginate --jq '.[].title'
  $ bb api repositories/{workspace}/{repo_slug}/refs/branches -f name=feat -F 'target[hash]=abc123'
  $ bb api -X PUT repositories/acme/widgets -f description="New description"
  $ bb api --input body.json -X POST repositories/acme/widgets/pullrequests`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Path = args[0]
			if opts.Paginate && opts.Method != "" && !strings.EqualFold(opts.Method, "GET") {
				return cmdutil.FlagErrorf("--paginate only works with GET requests")
			}
			if opts.Paginate && opts.Include {
				return cmdutil.FlagErrorf("--include cannot be combined with --paginate")
			}
			if opts.InputFile != "" && (len(opts.Fields) > 0 || len(opts.RawFields) > 0) {
				return cmdutil.FlagErrorf("--input cannot be combined with --field/--raw-field")
			}
			return run(cmd.Context(), f, opts)
		},
	}
	cmd.Flags().StringVarP(&opts.Method, "method", "X", "", "The HTTP method for the request (default GET, or POST with fields)")
	cmd.Flags().StringArrayVarP(&opts.RawFields, "raw-field", "f", nil, "Add a string parameter in key=value format")
	cmd.Flags().StringArrayVarP(&opts.Fields, "field", "F", nil, "Add a typed parameter in key=value format")
	cmd.Flags().StringArrayVarP(&opts.Headers, "header", "H", nil, "Add a HTTP request header in key:value format")
	cmd.Flags().StringVar(&opts.InputFile, "input", "", "The file to use as body for the HTTP request (use \"-\" for stdin)")
	cmd.Flags().BoolVar(&opts.Paginate, "paginate", false, "Fetch all pages of results, following next links")
	cmd.Flags().BoolVarP(&opts.Include, "include", "i", false, "Include HTTP response status line and headers in the output")
	cmd.Flags().BoolVar(&opts.Silent, "silent", false, "Do not print the response body")
	cmd.Flags().StringVarP(&opts.JQ, "jq", "q", "", "Query to select values from the response using jq syntax")
	cmd.Flags().StringVarP(&opts.Template, "template", "t", "", "Format JSON output using a Go template")
	cmdutil.EnableRepoOverride(cmd, f)
	return cmd
}

func run(ctx context.Context, f *cmdutil.Factory, opts *Options) error {
	ios := f.IOStreams
	client, err := f.APIClient()
	if err != nil {
		return err
	}
	path, err := fillPlaceholders(f, opts.Path)
	if err != nil {
		return err
	}

	params, err := parseFields(opts.RawFields, opts.Fields, ios.In)
	if err != nil {
		return err
	}
	method := strings.ToUpper(opts.Method)
	if method == "" {
		method = http.MethodGet
		if len(params) > 0 || opts.InputFile != "" {
			method = http.MethodPost
		}
	}
	headers := map[string]string{}
	for _, h := range opts.Headers {
		k, v, ok := strings.Cut(h, ":")
		if !ok {
			return cmdutil.FlagErrorf("invalid header %q (expected key:value)", h)
		}
		headers[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}

	req := bbapi.Request{Method: method, Path: path, Headers: headers}
	if opts.InputFile != "" {
		var body []byte
		if opts.InputFile == "-" {
			body, err = io.ReadAll(ios.In)
		} else {
			body, err = os.ReadFile(opts.InputFile)
		}
		if err != nil {
			return err
		}
		req.Body = body
	} else if len(params) > 0 {
		if method == http.MethodGet {
			req.Query = url.Values{}
			for k, v := range params {
				req.Query.Set(k, fmt.Sprint(v))
			}
		} else {
			req.Body = params
		}
	}

	if opts.Paginate {
		// Query parameters embedded in the path (e.g. ?pagelen=100) take
		// precedence over the defaults Paginate would add.
		extra := url.Values{}
		if u, err := client.Resolve(path); err == nil {
			extra = u.Query()
			path = strings.TrimSuffix(path[:len(path)-len(u.RawQuery)], "?")
		}
		for k, vs := range req.Query {
			extra[k] = vs
		}
		var all []json.RawMessage
		err := bbapi.Paginate(ctx, client, path, bbapi.ListOptions{Extra: extra, Headers: headers}, func(v json.RawMessage) error {
			all = append(all, v)
			return nil
		})
		if err != nil {
			return err
		}
		if opts.Silent {
			return nil
		}
		return writeBody(f, all, opts)
	}

	resp, err := client.DoRaw(ctx, req)
	if err != nil {
		var herr *bbapi.HTTPError
		if errors.As(err, &herr) {
			if opts.Include {
				fmt.Fprintf(ios.Out, "HTTP/1.1 %d %s\n\n", herr.StatusCode, http.StatusText(herr.StatusCode))
			}
			if !opts.Silent && herr.Body != "" {
				printRaw(ios.Out, []byte(herr.Body), ios.IsStdoutTTY())
			}
			fmt.Fprintf(ios.ErrOut, "bb: %s\n", herr.Error())
			return cmdutil.ErrSilent
		}
		return err
	}
	defer resp.Body.Close()

	if opts.Include {
		fmt.Fprintf(ios.Out, "%s %s\n", resp.Proto, resp.Status)
		for k, vs := range resp.Header {
			for _, v := range vs {
				fmt.Fprintf(ios.Out, "%s: %s\n", k, bbapi.MaskHeader(k, v))
			}
		}
		fmt.Fprintln(ios.Out)
	}
	if opts.Silent {
		return nil
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if opts.JQ != "" || opts.Template != "" {
		var v any
		if err := json.Unmarshal(body, &v); err != nil {
			return fmt.Errorf("response is not JSON; cannot apply --jq/--template: %w", err)
		}
		return writeBody(f, v, opts)
	}
	printRaw(ios.Out, body, ios.IsStdoutTTY())
	return nil
}

func writeBody(f *cmdutil.Factory, v any, opts *Options) error {
	ex := &output.Exporter{JQ: opts.JQ, Template: opts.Template}
	return ex.Write(f.IOStreams, v)
}

// printRaw pretty-prints JSON on a TTY; otherwise writes the body verbatim.
func printRaw(w io.Writer, body []byte, pretty bool) {
	if pretty && json.Valid(body) {
		var buf bytes.Buffer
		if json.Indent(&buf, body, "", "  ") == nil {
			buf.WriteByte('\n')
			_, _ = w.Write(buf.Bytes())
			return
		}
	}
	_, _ = w.Write(body)
	if len(body) > 0 && body[len(body)-1] != '\n' {
		_, _ = w.Write([]byte("\n"))
	}
}

// fillPlaceholders replaces {workspace}, {repo_slug}, {repo}, {owner} with the
// resolved base repository, only when they appear.
func fillPlaceholders(f *cmdutil.Factory, path string) (string, error) {
	if !strings.Contains(path, "{") {
		return path, nil
	}
	needs := strings.Contains(path, "{workspace}") || strings.Contains(path, "{owner}") ||
		strings.Contains(path, "{repo_slug}") || strings.Contains(path, "{repo}")
	if !needs {
		return path, nil
	}
	repo, err := f.BaseRepo()
	if err != nil {
		return "", err
	}
	r := strings.NewReplacer("{workspace}", repo.Workspace, "{owner}", repo.Workspace, "{repo_slug}", repo.Slug, "{repo}", repo.Slug)
	return r.Replace(path), nil
}

// parseFields turns -f/-F values into a (possibly nested) map.
// Keys like "target[hash]" or "source[branch][name]" create nested objects;
// a trailing "[]" appends to an array.
func parseFields(raw, typed []string, stdin io.Reader) (map[string]any, error) {
	out := map[string]any{}
	add := func(spec string, convert bool) error {
		key, value, ok := strings.Cut(spec, "=")
		if !ok {
			return cmdutil.FlagErrorf("field %q must be in key=value format", spec)
		}
		var v any = value
		if convert {
			cv, err := typedValue(value, stdin)
			if err != nil {
				return err
			}
			v = cv
		}
		return setNested(out, key, v)
	}
	for _, r := range raw {
		if err := add(r, false); err != nil {
			return nil, err
		}
	}
	for _, t := range typed {
		if err := add(t, true); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func typedValue(s string, stdin io.Reader) (any, error) {
	switch s {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		return nil, nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}
	if strings.HasPrefix(s, "@") {
		var b []byte
		var err error
		if s == "@-" {
			b, err = io.ReadAll(stdin)
		} else {
			b, err = os.ReadFile(s[1:])
		}
		if err != nil {
			return nil, err
		}
		return string(b), nil
	}
	return s, nil
}

func setNested(m map[string]any, key string, v any) error {
	parts := splitKey(key)
	cur := m
	for i, p := range parts {
		last := i == len(parts)-1
		if p == "" { // array append
			return cmdutil.FlagErrorf("invalid field key %q", key)
		}
		if last {
			if strings.HasSuffix(key, "[]") {
				arr, _ := cur[p].([]any)
				cur[p] = append(arr, v)
			} else {
				cur[p] = v
			}
			return nil
		}
		next, ok := cur[p].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[p] = next
		}
		cur = next
	}
	return nil
}

func splitKey(key string) []string {
	key = strings.TrimSuffix(key, "[]")
	var parts []string
	cur := strings.Builder{}
	for _, r := range key {
		switch r {
		case '[':
			parts = append(parts, cur.String())
			cur.Reset()
		case ']':
		default:
			cur.WriteRune(r)
		}
	}
	parts = append(parts, cur.String())
	return parts
}
