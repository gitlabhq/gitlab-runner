package helpers

import (
	"strings"
)

type mode string

const (
	lit mode = "literal"
	quo mode = "quote"

	hextable = "0123456789abcdef"
)

// modeTable is a mapping of ascii characters to an escape mode:
//   - escape character: where the mode is also the escaped string
//   - literal: a string full of only literals does not require quoting
//   - quote: a character that will need string quoting
//   - "": a missing mapping indicates that the character will need hex quoting
//
// https://www.gnu.org/software/bash/manual/html_node/ANSI_002dC-Quoting.html
var modeTable = [256]mode{
	'\a': `\a`, '\b': `\b`, '\t': `\t`, '\n': `\n`, '\v': `\v`, '\f': `\f`,
	'\r': `\r`, '\'': `\'`, '\\': `\\`,

	',': lit, '-': lit, '.': lit, '/': lit,
	'0': lit, '1': lit, '2': lit, '3': lit, '4': lit, '5': lit, '6': lit,
	'7': lit, '8': lit, '9': lit,

	'@': lit, 'A': lit, 'B': lit, 'C': lit, 'D': lit, 'E': lit, 'F': lit,
	'G': lit, 'H': lit, 'I': lit, 'J': lit, 'K': lit, 'L': lit, 'M': lit,
	'N': lit, 'O': lit, 'P': lit, 'Q': lit, 'R': lit, 'S': lit, 'T': lit,
	'U': lit, 'V': lit, 'W': lit, 'X': lit, 'Y': lit, 'Z': lit,

	'_': lit, 'a': lit, 'b': lit, 'c': lit, 'd': lit, 'e': lit, 'f': lit,
	'g': lit, 'h': lit, 'i': lit, 'j': lit, 'k': lit, 'l': lit, 'm': lit,
	'n': lit, 'o': lit, 'p': lit, 'q': lit, 'r': lit, 's': lit, 't': lit,
	'u': lit, 'v': lit, 'w': lit, 'x': lit, 'y': lit, 'z': lit,

	' ': quo, '!': quo, '"': quo, '#': quo, '$': quo, '%': quo, '&': quo,
	'(': quo, ')': quo, '*': quo, '+': quo, ':': quo, ';': quo, '<': quo,
	'=': quo, '>': quo, '?': quo, '[': quo, ']': quo, '^': quo, '`': quo,
	'{': quo, '|': quo, '}': quo, '~': quo,
}

// ShellEscape returns either a string identical to the input, or an escaped
// string if certain characters are present. ANSI-C Quoting is used for
// control characters and hexcodes are used for non-ascii characters.
func ShellEscape(input string) string {
	if input == "" {
		return "''"
	}

	var sb strings.Builder
	sb.Grow(len(input) * 2)

	escape := false
	for _, c := range []byte(input) {
		mode := modeTable[c]
		switch mode {
		case lit:
			sb.WriteByte(c)
		case quo:
			sb.WriteByte(c)
			escape = true
		case "":
			sb.Write([]byte{'\\', 'x', hextable[c>>4], hextable[c&0x0f]})
			escape = true
		default:
			sb.WriteString(string(mode))
			escape = true
		}
	}

	if escape {
		return "$'" + sb.String() + "'"
	}

	return sb.String()
}

// posixModeTable defines what characters need quoting, and which need to be
// backslash escaped:
//
// https://pubs.opengroup.org/onlinepubs/9699919799/utilities/V3_chap02.html#tag_18_02
var posixModeTable = [256]mode{
	'`': "\\`", '"': `\"`, '\\': `\\`, '$': `\$`,

	' ': quo, '!': quo, '#': quo, '%': quo, '&': quo, '(': quo, ')': quo,
	'*': quo, '<': quo, '=': quo, '>': quo, '?': quo, '[': quo, '|': quo,
}

// PosixShellEscape double quotes strings and escapes a string where necessary.
func PosixShellEscape(input string) string {
	if input == "" {
		return "''"
	}

	var sb strings.Builder
	sb.Grow(len(input) * 2)

	escape := false
	for _, c := range []byte(input) {
		mode := posixModeTable[c]
		switch mode {
		case quo:
			sb.WriteByte(c)
			escape = true
		case "":
			sb.WriteByte(c)
		default:
			sb.WriteString(string(mode))
			escape = true
		}
	}

	if escape {
		return `"` + sb.String() + `"`
	}

	return sb.String()
}
