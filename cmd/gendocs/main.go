// Command gendocs generates Markdown reference docs for every bb command.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra/doc"

	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/iostreams"
	"github.com/uehatsu/bb/pkg/cmd/root"
)

func main() {
	dir := "docs/reference"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	f := &cmdutil.Factory{IOStreams: iostreams.System()}
	cmd := root.NewCmdRoot(f)
	cmd.DisableAutoGenTag = true
	if err := doc.GenMarkdownTree(cmd, dir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
