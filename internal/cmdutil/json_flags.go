package cmdutil

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/output"
)

// AddJSONFlags registers --json, --jq and --template on cmd and returns a
// function that builds the Exporter after flags are parsed. validate is
// called with the requested field names.
func AddJSONFlags(cmd *cobra.Command, validate func([]string) error, available []string) func() (*output.Exporter, error) {
	var (
		jsonFields []string
		jqExpr     string
		tmpl       string
	)
	f := cmd.Flags()
	f.StringSliceVar(&jsonFields, "json", nil, "Output JSON with the specified fields (comma-separated)")
	f.StringVarP(&jqExpr, "jq", "q", "", "Filter JSON output using a jq expression")
	f.StringVarP(&tmpl, "template", "t", "", "Format JSON output using a Go template")
	if len(available) > 0 {
		_ = cmd.RegisterFlagCompletionFunc("json", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return available, cobra.ShellCompDirectiveNoFileComp
		})
	}
	return func() (*output.Exporter, error) {
		if len(jsonFields) == 0 && jqExpr == "" && tmpl == "" {
			return nil, nil
		}
		if len(jsonFields) == 0 {
			return nil, FlagErrorf("cannot use `--jq` or `--template` without specifying `--json`")
		}
		var fields []string
		for _, fl := range jsonFields {
			if fl = strings.TrimSpace(fl); fl != "" {
				fields = append(fields, fl)
			}
		}
		if len(fields) == 0 {
			return nil, FlagErrorf("--json requires at least one field name\nAvailable fields:\n  %s", strings.Join(available, "\n  "))
		}
		if validate != nil {
			if err := validate(fields); err != nil {
				return nil, FlagErrorWrap(err)
			}
		}
		return &output.Exporter{Fields: fields, JQ: jqExpr, Template: tmpl}, nil
	}
}
