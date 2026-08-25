// Package prompt abstracts interactive input so commands can be tested.
package prompt

import (
	"errors"
	"io"

	"github.com/charmbracelet/huh"
)

// Prompter asks questions on the terminal.
type Prompter interface {
	Input(title, placeholder string) (string, error)
	Password(title string) (string, error)
	Confirm(title string, def bool) (bool, error)
	Select(title string, options []string) (string, error)
	MultiSelect(title string, options []string) ([]string, error)
	Editor(title, initial string) (string, error)
}

// ErrCancelled is returned when the user aborts a prompt.
var ErrCancelled = errors.New("prompt cancelled")

// Huh is a Prompter backed by charmbracelet/huh.
type Huh struct {
	In  io.Reader
	Out io.Writer
}

func (h *Huh) run(f *huh.Form) error {
	err := f.Run()
	if errors.Is(err, huh.ErrUserAborted) {
		return ErrCancelled
	}
	return err
}

// Input implements Prompter.
func (h *Huh) Input(title, placeholder string) (string, error) {
	var v string
	err := h.run(huh.NewForm(huh.NewGroup(huh.NewInput().Title(title).Placeholder(placeholder).Value(&v))))
	return v, err
}

// Password implements Prompter.
func (h *Huh) Password(title string) (string, error) {
	var v string
	err := h.run(huh.NewForm(huh.NewGroup(huh.NewInput().Title(title).EchoMode(huh.EchoModePassword).Value(&v))))
	return v, err
}

// Confirm implements Prompter.
func (h *Huh) Confirm(title string, def bool) (bool, error) {
	v := def
	err := h.run(huh.NewForm(huh.NewGroup(huh.NewConfirm().Title(title).Value(&v))))
	return v, err
}

// Select implements Prompter.
func (h *Huh) Select(title string, options []string) (string, error) {
	var v string
	opts := make([]huh.Option[string], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, o)
	}
	err := h.run(huh.NewForm(huh.NewGroup(huh.NewSelect[string]().Title(title).Options(opts...).Value(&v))))
	return v, err
}

// MultiSelect implements Prompter.
func (h *Huh) MultiSelect(title string, options []string) ([]string, error) {
	var v []string
	opts := make([]huh.Option[string], len(options))
	for i, o := range options {
		opts[i] = huh.NewOption(o, o)
	}
	err := h.run(huh.NewForm(huh.NewGroup(huh.NewMultiSelect[string]().Title(title).Options(opts...).Value(&v))))
	return v, err
}

// Editor implements Prompter with a multi-line text field.
func (h *Huh) Editor(title, initial string) (string, error) {
	v := initial
	err := h.run(huh.NewForm(huh.NewGroup(huh.NewText().Title(title).Value(&v).Lines(8))))
	return v, err
}

// Stub is a scripted Prompter for tests.
type Stub struct {
	Inputs       []string
	Passwords    []string
	Confirms     []bool
	Selects      []string
	MultiSelects [][]string
	Editors      []string
}

// pop returns the next scripted answer; running out of answers is treated
// as the user aborting the prompt.
func pop[T any](s *[]T) (T, error) {
	var zero T
	if len(*s) == 0 {
		return zero, ErrCancelled
	}
	v := (*s)[0]
	*s = (*s)[1:]
	return v, nil
}

func (s *Stub) Input(string, string) (string, error)           { return pop(&s.Inputs) }
func (s *Stub) Password(string) (string, error)                { return pop(&s.Passwords) }
func (s *Stub) Confirm(string, bool) (bool, error)             { return pop(&s.Confirms) }
func (s *Stub) Select(string, []string) (string, error)        { return pop(&s.Selects) }
func (s *Stub) MultiSelect(string, []string) ([]string, error) { return pop(&s.MultiSelects) }
func (s *Stub) Editor(string, string) (string, error)          { return pop(&s.Editors) }
