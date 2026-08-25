// Package iostreams abstracts stdin/stdout/stderr, TTY detection, and color
// so commands can be tested without touching the real terminal.
package iostreams

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/mattn/go-isatty"
	"golang.org/x/term"
)

// IOStreams bundles the three standard streams plus terminal metadata.
type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer

	stdinTTY  bool
	stdoutTTY bool
	stderrTTY bool
	colorOn   bool
	termWidth int

	pagerCmd string
	pager    *exec.Cmd
	pagerW   io.WriteCloser
	origOut  io.Writer
}

// System returns IOStreams wired to the real process streams.
func System() *IOStreams {
	s := &IOStreams{
		In:        os.Stdin,
		Out:       os.Stdout,
		ErrOut:    os.Stderr,
		stdinTTY:  isTerminal(os.Stdin),
		stdoutTTY: isTerminal(os.Stdout),
		stderrTTY: isTerminal(os.Stderr),
	}
	s.colorOn = s.stdoutTTY && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb"
	if s.stdoutTTY {
		if w, _, err := term.GetSize(int(os.Stdout.Fd())); err == nil {
			s.termWidth = w
		}
	}
	return s
}

// Test returns IOStreams backed by buffers for use in tests.
func Test() (*IOStreams, *bytes.Buffer, *bytes.Buffer, *bytes.Buffer) {
	in, out, errOut := &bytes.Buffer{}, &bytes.Buffer{}, &bytes.Buffer{}
	return &IOStreams{In: in, Out: out, ErrOut: errOut, termWidth: 80}, in, out, errOut
}

func isTerminal(f *os.File) bool {
	return isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())
}

// IsStdinTTY reports whether stdin is a terminal.
func (s *IOStreams) IsStdinTTY() bool { return s.stdinTTY }

// IsStdoutTTY reports whether stdout is a terminal.
func (s *IOStreams) IsStdoutTTY() bool { return s.stdoutTTY }

// IsStderrTTY reports whether stderr is a terminal.
func (s *IOStreams) IsStderrTTY() bool { return s.stderrTTY }

// CanPrompt reports whether interactive prompts are possible.
func (s *IOStreams) CanPrompt() bool { return s.stdinTTY && s.stdoutTTY }

// ColorEnabled reports whether ANSI colors should be emitted.
func (s *IOStreams) ColorEnabled() bool { return s.colorOn }

// SetColorEnabled overrides color detection (tests).
func (s *IOStreams) SetColorEnabled(b bool) { s.colorOn = b }

// SetStdoutTTY overrides TTY detection (tests).
func (s *IOStreams) SetStdoutTTY(b bool) { s.stdoutTTY = b }

// SetStdinTTY overrides TTY detection (tests).
func (s *IOStreams) SetStdinTTY(b bool) { s.stdinTTY = b }

// TerminalWidth returns the terminal width, or 80 if unknown.
func (s *IOStreams) TerminalWidth() int {
	if s.termWidth <= 0 {
		return 80
	}
	return s.termWidth
}

// SetPager sets the pager command (e.g. "less -R"). Empty disables paging.
func (s *IOStreams) SetPager(cmd string) { s.pagerCmd = cmd }

// StartPager pipes Out through the configured pager when stdout is a TTY.
func (s *IOStreams) StartPager() error {
	if s.pagerCmd == "" || !s.stdoutTTY {
		return nil
	}
	parts := strings.Fields(s.pagerCmd)
	if len(parts) == 0 {
		return nil
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Env = os.Environ()
	if os.Getenv("LESS") == "" {
		cmd.Env = append(cmd.Env, "LESS=FRX")
	}
	if os.Getenv("LV") == "" {
		cmd.Env = append(cmd.Env, "LV=-c")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	w, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	s.pager, s.pagerW, s.origOut = cmd, w, s.Out
	s.Out = w
	return nil
}

// StopPager closes the pager pipe and waits for the pager to exit.
func (s *IOStreams) StopPager() {
	if s.pager == nil {
		return
	}
	_ = s.pagerW.Close()
	_ = s.pager.Wait()
	s.Out = s.origOut
	s.pager, s.pagerW = nil, nil
}
