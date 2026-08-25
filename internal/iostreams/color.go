package iostreams

import "fmt"

// ColorScheme produces ANSI-colored strings when colors are enabled and
// plain strings otherwise.
type ColorScheme struct{ enabled bool }

// ColorScheme returns a scheme bound to this stream's color setting.
func (s *IOStreams) ColorScheme() *ColorScheme { return &ColorScheme{enabled: s.colorOn} }

func (c *ColorScheme) wrap(code string, s string) string {
	if !c.enabled {
		return s
	}
	return fmt.Sprintf("\x1b[%sm%s\x1b[0m", code, s)
}

func (c *ColorScheme) Bold(s string) string    { return c.wrap("1", s) }
func (c *ColorScheme) Red(s string) string     { return c.wrap("31", s) }
func (c *ColorScheme) Green(s string) string   { return c.wrap("32", s) }
func (c *ColorScheme) Yellow(s string) string  { return c.wrap("33", s) }
func (c *ColorScheme) Blue(s string) string    { return c.wrap("34", s) }
func (c *ColorScheme) Magenta(s string) string { return c.wrap("35", s) }
func (c *ColorScheme) Cyan(s string) string    { return c.wrap("36", s) }
func (c *ColorScheme) Gray(s string) string    { return c.wrap("90", s) }

// Colorize applies a named color ("red", "green", ...). Unknown names are no-ops.
func (c *ColorScheme) Colorize(name, s string) string {
	switch name {
	case "bold":
		return c.Bold(s)
	case "red":
		return c.Red(s)
	case "green":
		return c.Green(s)
	case "yellow":
		return c.Yellow(s)
	case "blue":
		return c.Blue(s)
	case "magenta":
		return c.Magenta(s)
	case "cyan":
		return c.Cyan(s)
	case "gray", "grey":
		return c.Gray(s)
	}
	return s
}

// SuccessIcon returns a green check mark (or "✓" plain).
func (c *ColorScheme) SuccessIcon() string { return c.Green("✓") }

// FailureIcon returns a red cross.
func (c *ColorScheme) FailureIcon() string { return c.Red("✗") }

// WarningIcon returns a yellow exclamation.
func (c *ColorScheme) WarningIcon() string { return c.Yellow("!") }
