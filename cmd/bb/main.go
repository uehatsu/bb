// Command bb is a GitHub-CLI-like command line interface for Bitbucket Cloud.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	rootCmd.SetContext(ctx)
	cmd, err := rootCmd.ExecuteC()
	return exitCode(f.IOStreams.ErrOut, cmd, err, ctx.Err() != nil)
}

// exitCode maps a command error to the process exit status (gh conventions:
// 0 ok, 1 error, 2 cancelled, 4 auth required, 8 pending).
func exitCode(errOut io.Writer, cmd *cobra.Command, err error, interrupted bool) int {
	if err == nil {
		return cmdutil.ExitOK
	}
	if errors.Is(err, context.Canceled) || interrupted {
		fmt.Fprintln(errOut, "\nbb: interrupted")
		return cmdutil.ExitCancel
	}

	if errors.Is(err, cmdutil.ErrSilent) {
		return cmdutil.ExitError
	}
	if cmdutil.IsUserCancellation(err) {
		fmt.Fprintln(errOut)
		return cmdutil.ExitCancel
	}
	if errors.Is(err, cmdutil.ErrPending) {
		return cmdutil.ExitPending
	}
	var authErr *cmdutil.AuthError
	if errors.As(err, &authErr) {
		fmt.Fprintln(errOut, authErr.Error())
		return cmdutil.ExitAuth
	}
	var flagErr *cmdutil.FlagError
	if errors.As(err, &flagErr) {
		fmt.Fprintln(errOut, err)
		fmt.Fprintln(errOut)
		if cmd != nil {
			_ = cmd.Usage()
		}
		return cmdutil.ExitError
	}
	fmt.Fprintln(errOut, err)
	return cmdutil.ExitError
}
