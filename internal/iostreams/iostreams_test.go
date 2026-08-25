package iostreams

import (
	"strings"
	"testing"
)

func TestColorSchemeAndFlags(t *testing.T) {
	on := NewColorScheme(true)
	off := NewColorScheme(false)
	if off.Red("x") != "x" || !strings.Contains(on.Red("x"), "\x1b[31m") || !strings.Contains(on.Blue("x"), "\x1b[34m") {
		t.Error("basic colors")
	}
	for _, name := range []string{"bold", "red", "green", "yellow", "blue", "magenta", "cyan", "gray", "grey"} {
		if on.Colorize(name, "x") == "x" {
			t.Errorf("Colorize(%s) should color", name)
		}
	}
	if on.Colorize("unknown", "x") != "x" {
		t.Error("unknown color name is a no-op")
	}
	if on.SuccessIcon() == "" || on.FailureIcon() == "" || on.WarningIcon() == "" {
		t.Error("icons")
	}

	ios, _, _, _ := Test()
	if ios.IsStdinTTY() || ios.IsStdoutTTY() || ios.IsStderrTTY() || ios.CanPrompt() || ios.ColorEnabled() {
		t.Error("Test streams are non-TTY by default")
	}
	ios.SetStdinTTY(true)
	ios.SetStdoutTTY(true)
	ios.SetColorEnabled(true)
	if !ios.CanPrompt() || !ios.ColorEnabled() || ios.TerminalWidth() != 80 {
		t.Error("setters")
	}
	ios.SetPager("cat")
	if ios.Pager() != "cat" {
		t.Error("Pager")
	}
	// StartPager is a no-op when stdout is not a real terminal file; ensure it
	// does not panic and StopPager is safe to call.
	ios.SetStdoutTTY(false)
	if err := ios.StartPager(); err != nil {
		t.Error(err)
	}
	ios.StopPager()
	if System() == nil {
		t.Error("System")
	}
}
