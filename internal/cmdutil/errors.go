// Package cmdutil provides shared helpers for cobra commands: the Factory,
// error/exit-code conventions, and common flag handling.
package cmdutil

import (
	"errors"
	"fmt"
)

// Exit codes, aligned with GitHub CLI.
const (
	ExitOK      = 0
	ExitError   = 1
	ExitCancel  = 2
	ExitAuth    = 4
	ExitPending = 8
)

// ErrSilent indicates the error has already been reported to the user.
var ErrSilent = errors.New("silent error")

// ErrCancel is returned when the user aborts an interactive prompt.
var ErrCancel = errors.New("cancelled")

// ErrPending is used by watch-style commands whose target is still running.
var ErrPending = errors.New("pending")

// AuthError signals that the user must authenticate first.
type AuthError struct{ Msg string }

func (e *AuthError) Error() string { return e.Msg }

// NewAuthError returns an AuthError with a login hint.
func NewAuthError(msg string) error {
	if msg == "" {
		msg = "authentication required"
	}
	return &AuthError{Msg: msg + ". Run `bb auth login` to authenticate."}
}

// FlagError wraps usage errors so the caller can print help.
type FlagError struct{ err error }

func (e *FlagError) Error() string { return e.err.Error() }
func (e *FlagError) Unwrap() error { return e.err }

// FlagErrorf creates a FlagError.
func FlagErrorf(format string, a ...any) error {
	return &FlagError{err: fmt.Errorf(format, a...)}
}

// FlagErrorWrap wraps an existing error as a FlagError.
func FlagErrorWrap(err error) error { return &FlagError{err: err} }

// IsUserCancellation reports whether err represents a user cancel (Ctrl-C / prompt abort).
func IsUserCancellation(err error) bool {
	return errors.Is(err, ErrCancel)
}
