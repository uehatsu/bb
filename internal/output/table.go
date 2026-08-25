// Package output renders command results as aligned tables (TTY), TSV
// (non-TTY), or JSON/jq/template exports.
package output

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/uehatsu/bb/internal/iostreams"
)

// TablePrinter accumulates rows and renders them aligned for terminals or
// tab-separated for pipes.
type TablePrinter struct {
	io       *iostreams.IOStreams
	isTTY    bool
	maxWidth int
	rows     [][]cell
	cur      []cell
}

type cell struct {
	text  string
	color func(string) string
}

// NewTablePrinter creates a printer bound to the streams.
func NewTablePrinter(io *iostreams.IOStreams) *TablePrinter {
	return &TablePrinter{io: io, isTTY: io.IsStdoutTTY(), maxWidth: io.TerminalWidth()}
}

// IsTTY reports whether output is formatted for a terminal.
func (t *TablePrinter) IsTTY() bool { return t.isTTY }

// AddField appends a cell to the current row. color is applied only on TTY.
func (t *TablePrinter) AddField(s string, color func(string) string) {
	t.cur = append(t.cur, cell{text: s, color: color})
}

// EndRow finishes the current row.
func (t *TablePrinter) EndRow() {
	t.rows = append(t.rows, t.cur)
	t.cur = nil
}

// Render writes the table.
func (t *TablePrinter) Render() error {
	if t.cur != nil {
		t.EndRow()
	}
	if !t.isTTY {
		for _, row := range t.rows {
			parts := make([]string, len(row))
			for i, c := range row {
				parts[i] = c.text
			}
			if _, err := fmt.Fprintln(t.io.Out, strings.Join(parts, "\t")); err != nil {
				return err
			}
		}
		return nil
	}
	ncols := 0
	for _, r := range t.rows {
		if len(r) > ncols {
			ncols = len(r)
		}
	}
	widths := make([]int, ncols)
	for _, r := range t.rows {
		for i, c := range r {
			if w := displayWidth(c.text); w > widths[i] {
				widths[i] = w
			}
		}
	}
	// Shrink the widest column if the table would overflow.
	const sep = 2
	total := sep * (ncols - 1)
	for _, w := range widths {
		total += w
	}
	if total > t.maxWidth && ncols > 0 {
		widest := 0
		for i, w := range widths {
			if w > widths[widest] {
				widest = i
			}
		}
		excess := total - t.maxWidth
		if widths[widest]-excess >= 8 {
			widths[widest] -= excess
		} else {
			widths[widest] = 8
		}
	}
	for _, r := range t.rows {
		var b strings.Builder
		for i, c := range r {
			text := Truncate(c.text, widths[i])
			if i < len(r)-1 {
				text = pad(text, widths[i])
			}
			if c.color != nil {
				text = c.color(text)
			}
			b.WriteString(text)
			if i < len(r)-1 {
				b.WriteString(strings.Repeat(" ", sep))
			}
		}
		if _, err := fmt.Fprintln(t.io.Out, strings.TrimRight(b.String(), " ")); err != nil {
			return err
		}
	}
	return nil
}

func displayWidth(s string) int {
	w := 0
	for _, r := range s {
		if isWide(r) {
			w += 2
		} else {
			w++
		}
	}
	return w
}

func isWide(r rune) bool {
	// Rough East Asian Wide/Fullwidth detection sufficient for alignment.
	return (r >= 0x1100 && r <= 0x115F) || (r >= 0x2E80 && r <= 0xA4CF) ||
		(r >= 0xAC00 && r <= 0xD7A3) || (r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0xFE30 && r <= 0xFE4F) || (r >= 0xFF00 && r <= 0xFF60) ||
		(r >= 0xFFE0 && r <= 0xFFE6) || (r >= 0x1F300 && r <= 0x1FAFF) ||
		(r >= 0x20000 && r <= 0x3FFFD)
}

func pad(s string, width int) string {
	if w := displayWidth(s); w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

// Truncate shortens s to fit width display cells, appending "..." if cut.
func Truncate(s string, width int) string {
	if width <= 0 || displayWidth(s) <= width {
		return s
	}
	if width <= 3 {
		return string([]rune(s)[:width])
	}
	w := 0
	var b strings.Builder
	for _, r := range s {
		rw := 1
		if isWide(r) {
			rw = 2
		}
		if w+rw > width-3 {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + "..."
}

// WriteLine is a small helper for consistent line output.
func WriteLine(w io.Writer, s string) {
	if !strings.HasSuffix(s, "\n") {
		s += "\n"
	}
	_, _ = io.WriteString(w, s)
}

// RuneLen is exported for tests.
func RuneLen(s string) int { return utf8.RuneCountInString(s) }
