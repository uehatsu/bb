// Command bb is a GitHub-CLI-like command line interface for Bitbucket Cloud.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return execute(ctx, factory.New(), os.Args[1:])
}

// execute builds the command tree for f, runs it with args, and returns the
// exit status. It is separated from run so tests can drive it with a stub
// Factory and a cancellable context.
func execute(ctx context.Context, f *cmdutil.Factory, args []string) int {
	rootCmd := root.NewCmdRoot(f)
	rootCmd.SetIn(f.IOStreams.In)
	rootCmd.SetOut(f.IOStreams.Out)
	rootCmd.SetErr(f.IOStreams.ErrOut)
	rootCmd.SetArgs(args)
	rootCmd.SetContext(ctx)
	// Unknown / malformed flags are usage errors like unknown commands:
	// error + usage on stderr, exit 1 (gh parity).
	rootCmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return cmdutil.FlagErrorWrap(err) })
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
	// Usage errors: our FlagError, and cobra's report of an unknown root
	// command (raised before flag parsing, so it is a plain error). Like gh,
	// print the usage to stderr so stdout stays clean for scripts.
	var flagErr *cmdutil.FlagError
	if errors.As(err, &flagErr) || strings.HasPrefix(err.Error(), "unknown command ") {
		fmt.Fprintln(errOut, strings.TrimRight(err.Error(), "\n"))
		fmt.Fprintln(errOut)
		if cmd != nil {
			fmt.Fprint(errOut, cmd.UsageString())
		}
		return cmdutil.ExitError
	}
	fmt.Fprintln(errOut, err)
	return cmdutil.ExitError
}
