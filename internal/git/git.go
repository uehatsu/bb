// Package git is a thin wrapper around the git executable. Arguments are
// always passed as an argv slice (never through a shell), and user-supplied
// positional values are separated with "--" where git supports it.
package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// Client runs git in a working directory.
type Client struct {
	GitPath string
	Dir     string
	Stdout  *os.File // for interactive commands (clone/checkout); nil = capture
	Stderr  *os.File
}

// New locates git on PATH.
func New(dir string) (*Client, error) {
	p, err := exec.LookPath("git")
	if err != nil {
		return nil, errors.New("git executable not found in PATH")
	}
	return &Client{GitPath: p, Dir: dir, Stderr: os.Stderr}, nil
}

// Output runs git with args and returns trimmed stdout.
func (c *Client) Output(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.GitPath, args...)
	cmd.Dir = c.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &Error{Args: args, Stderr: strings.TrimSpace(stderr.String()), Err: err}
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Run runs git with args, streaming output to the terminal.
func (c *Client) Run(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, c.GitPath, args...)
	cmd.Dir = c.Dir
	cmd.Stdin = os.Stdin
	if c.Stdout != nil {
		cmd.Stdout = c.Stdout
	} else {
		cmd.Stdout = os.Stdout
	}
	if c.Stderr != nil {
		cmd.Stderr = c.Stderr
	} else {
		cmd.Stderr = os.Stderr
	}
	if err := cmd.Run(); err != nil {
		return &Error{Args: args, Err: err}
	}
	return nil
}

// Error wraps a failed git invocation.
type Error struct {
	Args   []string
	Stderr string
	Err    error
}

func (e *Error) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("git %s: %s", strings.Join(e.Args, " "), e.Stderr)
	}
	return fmt.Sprintf("git %s: %v", strings.Join(e.Args, " "), e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

// Remote is a parsed `git remote -v` entry (fetch URL).
type Remote struct {
	Name string
	URL  string
}

// Remotes lists fetch remotes.
func (c *Client) Remotes(ctx context.Context) ([]Remote, error) {
	out, err := c.Output(ctx, "remote", "-v")
	if err != nil {
		return nil, err
	}
	var remotes []Remote
	seen := map[string]bool{}
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || seen[fields[0]] {
			continue
		}
		if len(fields) >= 3 && fields[2] != "(fetch)" {
			continue
		}
		seen[fields[0]] = true
		remotes = append(remotes, Remote{Name: fields[0], URL: fields[1]})
	}
	return remotes, nil
}

// CurrentBranch returns the checked-out branch name.
func (c *Client) CurrentBranch(ctx context.Context) (string, error) {
	out, err := c.Output(ctx, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return "", errors.New("not on any branch (detached HEAD)")
	}
	return out, nil
}

// ConfigGet reads a git config value ("" when unset).
func (c *Client) ConfigGet(ctx context.Context, scopeFlag, key string) (string, error) {
	args := []string{"config"}
	if scopeFlag != "" {
		args = append(args, scopeFlag)
	}
	args = append(args, "--get", key)
	out, err := c.Output(ctx, args...)
	var gerr *Error
	if errors.As(err, &gerr) {
		var exitErr *exec.ExitError
		if errors.As(gerr.Err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", nil // unset
		}
	}
	return out, err
}

// ConfigSet writes a git config value.
func (c *Client) ConfigSet(ctx context.Context, scopeFlag, key, value string) error {
	args := []string{"config"}
	if scopeFlag != "" {
		args = append(args, scopeFlag)
	}
	args = append(args, "--replace-all", key, value)
	_, err := c.Output(ctx, args...)
	return err
}

// ConfigAdd appends a multi-valued git config entry.
func (c *Client) ConfigAdd(ctx context.Context, scopeFlag, key, value string) error {
	args := []string{"config"}
	if scopeFlag != "" {
		args = append(args, scopeFlag)
	}
	args = append(args, "--add", key, value)
	_, err := c.Output(ctx, args...)
	return err
}

// ConfigUnsetAll removes all values for key (no error when absent).
func (c *Client) ConfigUnsetAll(ctx context.Context, scopeFlag, key string) error {
	args := []string{"config"}
	if scopeFlag != "" {
		args = append(args, scopeFlag)
	}
	args = append(args, "--unset-all", key)
	_, err := c.Output(ctx, args...)
	var gerr *Error
	if errors.As(err, &gerr) {
		var exitErr *exec.ExitError
		if errors.As(gerr.Err, &exitErr) && exitErr.ExitCode() == 5 {
			return nil // key absent
		}
	}
	return err
}
