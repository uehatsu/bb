// Command bb is a GitHub-CLI-like command line interface for Bitbucket Cloud.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/uehatsu/bb/internal/cmdutil"
	"github.com/uehatsu/bb/internal/factory"
	"github.com/uehatsu/bb/pkg/cmd/root"
)

func main() {
	os.Exit(run())
}

func run() int {
	f := factory.New()
	rootCmd := root.NewCmdRoot(f)
	rootCmd.SetIn(f.IOStreams.In)
	rootCmd.SetOut(f.IOStreams.Out)
	rootCmd.SetErr(f.IOStreams.ErrOut)

	cmd, err := rootCmd.ExecuteC()
	if err == nil {
		return cmdutil.ExitOK
	}

	if errors.Is(err, cmdutil.SilentError) {
		return cmdutil.ExitError
	}
	if cmdutil.IsUserCancellation(err) {
		fmt.Fprintln(f.IOStreams.ErrOut)
		return cmdutil.ExitCancel
	}
	if errors.Is(err, cmdutil.PendingError) {
		return cmdutil.ExitPending
	}
	var authErr *cmdutil.AuthError
	if errors.As(err, &authErr) {
		fmt.Fprintln(f.IOStreams.ErrOut, authErr.Error())
		return cmdutil.ExitAuth
	}
	var flagErr *cmdutil.FlagError
	if errors.As(err, &flagErr) {
		fmt.Fprintln(f.IOStreams.ErrOut, err)
		fmt.Fprintln(f.IOStreams.ErrOut)
		_ = cmd.Usage()
		return cmdutil.ExitError
	}
	fmt.Fprintln(f.IOStreams.ErrOut, err)
	return cmdutil.ExitError
}
