package internal

import (
	"fmt"
	"strings"
)

const hextable = "0123456789abcdef"

// EscapeForAnsiC escapes a string for use in ANSI-C quoting ($'...').
// Control characters and non-ASCII bytes are hex-escaped to prevent terminal manipulation.
// This prevents jobs from clearing the screen or rewriting logs using ANSI escape sequences.
// Matches GitLab Runner's ShellEscape behavior.
func EscapeForAnsiC(s string) string {
	var buf strings.Builder

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '\\':
			buf.WriteString("\\\\")
		case '\'':
			buf.WriteString("\\'")
		case '\n':
			buf.WriteString("\\n")
		case '\r':
			buf.WriteString("\\r")
		case '\t':
			buf.WriteString("\\t")
		case '\a':
			buf.WriteString("\\a")
		case '\b':
			buf.WriteString("\\b")
		case '\f':
			buf.WriteString("\\f")
		case '\v':
			buf.WriteString("\\v")
		default:
			// Hex-escape control characters (0x00-0x1F, 0x7F) and non-ASCII (>0x7F)
			// This prevents ANSI escape sequences (ESC = 0x1B) from manipulating terminal
			if c < 0x20 || c == 0x7F || c > 0x7F {
				buf.WriteString(fmt.Sprintf("\\x%c%c", hextable[c>>4], hextable[c&0x0f]))
			} else {
				buf.WriteByte(c)
			}
		}
	}

	return buf.String()
}

// EscapeForPosix escapes a string for use in POSIX double-quoted strings.
// Matches GitLab Runner's PosixShellEscape behavior.
// Escapes: `, ", \, $
func EscapeForPosix(s string) string {
	if s == "" {
		return "''"
	}

	var buf strings.Builder
	needsQuoting := false

	for _, r := range s {
		switch r {
		case '`', '"', '\\', '$':
			buf.WriteRune('\\')
			buf.WriteRune(r)
			needsQuoting = true
		case ' ', '!', '#', '%', '&', '(', ')', '*', '<', '=', '>', '?', '[', '|':
			buf.WriteRune(r)
			needsQuoting = true
		default:
			buf.WriteRune(r)
		}
	}

	if needsQuoting {
		return `"` + buf.String() + `"`
	}

	return buf.String()
}
