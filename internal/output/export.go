package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/template"
	"time"

	"github.com/itchyny/gojq"

	"github.com/uehatsu/bb/internal/iostreams"
)

// Exporter renders already-projected data (maps/slices of maps) as JSON,
// filtered through jq, or through a Go template — mirroring gh's
// --json/--jq/--template trio.
type Exporter struct {
	Fields   []string
	JQ       string
	Template string
}

// Active reports whether any export mode was requested.
func (e *Exporter) Active() bool {
	return e != nil && (len(e.Fields) > 0 || e.JQ != "" || e.Template != "")
}

// Write renders data to io.Out.
func (e *Exporter) Write(io *iostreams.IOStreams, data any) error {
	switch {
	case e.JQ != "":
		return writeJQ(io.Out, data, e.JQ)
	case e.Template != "":
		return writeTemplate(io, data, e.Template)
	default:
		return writeJSON(io.Out, data, io.IsStdoutTTY())
	}
}

func writeJSON(w io.Writer, data any, pretty bool) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(data)
}

// toPlain converts data to generic JSON values for gojq/templates.
func toPlain(data any) (any, error) {
	b, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var v any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return normalizeNumbers(v), nil
}

func normalizeNumbers(v any) any {
	switch x := v.(type) {
	case json.Number:
		if i, err := x.Int64(); err == nil {
			return int(i)
		}
		f, _ := x.Float64()
		return f
	case map[string]any:
		for k, val := range x {
			x[k] = normalizeNumbers(val)
		}
		return x
	case []any:
		for i, val := range x {
			x[i] = normalizeNumbers(val)
		}
		return x
	}
	return v
}

func writeJQ(w io.Writer, data any, expr string) error {
	q, err := gojq.Parse(expr)
	if err != nil {
		return fmt.Errorf("invalid jq expression: %w", err)
	}
	code, err := gojq.Compile(q)
	if err != nil {
		return fmt.Errorf("invalid jq expression: %w", err)
	}
	plain, err := toPlain(data)
	if err != nil {
		return err
	}
	iter := code.Run(plain)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, isErr := v.(error); isErr {
			return err
		}
		switch x := v.(type) {
		case string:
			fmt.Fprintln(w, x)
		default:
			b, err := gojq.Marshal(x)
			if err != nil {
				return err
			}
			fmt.Fprintln(w, string(b))
		}
	}
	return nil
}

func writeTemplate(io *iostreams.IOStreams, data any, tmpl string) error {
	plain, err := toPlain(data)
	if err != nil {
		return err
	}
	t, err := template.New("").Funcs(TemplateFuncs(io)).Parse(tmpl)
	if err != nil {
		return fmt.Errorf("invalid template: %w", err)
	}
	return t.Execute(io.Out, plain)
}

// TemplateFuncs returns gh-compatible template helpers.
func TemplateFuncs(io *iostreams.IOStreams) template.FuncMap {
	cs := io.ColorScheme()
	return template.FuncMap{
		"color": func(name string, v any) string {
			return cs.Colorize(name, fmt.Sprint(v))
		},
		"timeago": func(v any) (string, error) {
			t, err := parseTime(v)
			if err != nil {
				return "", err
			}
			return TimeAgo(time.Now(), t), nil
		},
		"timefmt": func(layout string, v any) (string, error) {
			t, err := parseTime(v)
			if err != nil {
				return "", err
			}
			return t.Format(layout), nil
		},
		"truncate": func(n int, v any) string { return Truncate(fmt.Sprint(v), n) },
		"join": func(sep string, v any) (string, error) {
			items, ok := v.([]any)
			if !ok {
				return "", fmt.Errorf("join: expected array, got %T", v)
			}
			parts := make([]string, len(items))
			for i, it := range items {
				parts[i] = fmt.Sprint(it)
			}
			return strings.Join(parts, sep), nil
		},
		"pluck": func(field string, v any) ([]any, error) {
			items, ok := v.([]any)
			if !ok {
				return nil, fmt.Errorf("pluck: expected array, got %T", v)
			}
			out := make([]any, 0, len(items))
			for _, it := range items {
				if m, ok := it.(map[string]any); ok {
					out = append(out, m[field])
				}
			}
			return out, nil
		},
		"tablerow": func(fields ...any) string {
			parts := make([]string, len(fields))
			for i, f := range fields {
				parts[i] = fmt.Sprint(f)
			}
			return strings.Join(parts, "\t") + "\n"
		},
	}
}

func parseTime(v any) (time.Time, error) {
	switch x := v.(type) {
	case time.Time:
		return x, nil
	case string:
		return time.Parse(time.RFC3339Nano, x)
	}
	return time.Time{}, fmt.Errorf("cannot parse %T as time", v)
}

// TimeAgo renders a relative duration like gh ("about 2 hours ago").
func TimeAgo(now, t time.Time) string {
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "less than a minute ago"
	case d < time.Hour:
		return plural(int(d.Minutes()), "minute") + " ago"
	case d < 24*time.Hour:
		return "about " + plural(int(d.Hours()), "hour") + " ago"
	case d < 30*24*time.Hour:
		return "about " + plural(int(d.Hours()/24), "day") + " ago"
	case d < 365*24*time.Hour:
		return "about " + plural(int(d.Hours()/24/30), "month") + " ago"
	}
	return "about " + plural(int(d.Hours()/24/365), "year") + " ago"
}

func plural(n int, unit string) string {
	if n == 1 {
		return "1 " + unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
