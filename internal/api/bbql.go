package api

import "strings"

// BBQLQuote returns s as a double-quoted BBQL string literal, escaping
// backslashes and double quotes. Always use this when embedding user or
// server supplied values in a q= filter.
func BBQLQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\', '"':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}

// BBQLAnd joins non-empty clauses with AND, parenthesizing each so that
// user-supplied expressions cannot change precedence.
func BBQLAnd(clauses ...string) string {
	var parts []string
	for _, c := range clauses {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		parts = append(parts, "("+c+")")
	}
	return strings.Join(parts, " AND ")
}
