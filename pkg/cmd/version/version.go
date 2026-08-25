// Package version implements `bb version`.
package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/uehatsu/bb/internal/build"
	"github.com/uehatsu/bb/internal/cmdutil"
)

// NewCmdVersion returns the version command.
func NewCmdVersion(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show bb version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(f.IOStreams.Out, Format(build.Version, build.Date))
			return nil
		},
	}
}

// Format renders the version line.
func Format(version, date string) string {
	if date != "" {
		return fmt.Sprintf("bb version %s (%s)\n", version, date)
	}
	return fmt.Sprintf("bb version %s\n", version)
}
