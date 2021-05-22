package helpers

// https://github.com/zimbatm/direnv/blob/master/shell.go

import (
	"bytes"
	"encoding/hex"
)

/*
 * Escaping
 */

const (
	ACK           = 6
	TAB           = 9
	LF            = 10
	CR            = 13
	US            = 31
	SPACE         = 32
	AMPERSTAND    = 38
	SINGLE_QUOTE  = 39
	PLUS          = 43
	NINE          = 57
	QUESTION      = 63
	LOWERCASE_Z   = 90
	OPEN_BRACKET  = 91
	BACKSLASH     = 92
	UNDERSCORE    = 95
	CLOSE_BRACKET = 93
	BACKTICK      = 96
	TILDA         = 126
	DEL           = 127
)

type shellEscaper struct {
}

//nolint:lll
// ShellEscape is taken from
// https://github.com/solidsnack/shell-escape/blob/056c7b308be32ffeafec815907699f6c27536b1e/Data/ByteString/ShellEscape/Bash.hs
/*
A Bash escaped string. The strings are wrapped in @$\'...\'@ if any
bytes within them must be escaped; otherwise, they are left as is.
Newlines and other control characters are represented as ANSI escape
sequences. High bytes are represented as hex codes. Thus Bash escaped
strings will always fit on one line and never contain non-ASCII bytes.
*/
func ShellEscapeLegacy(str string) string {
	e := newShellEscaper()
	outStr := e.getEscapedString(str)

	return outStr
}

func newShellEscaper() *shellEscaper {
	e := &shellEscaper{}

	return e
}

func (e *shellEscaper) hex(char byte, out *bytes.Buffer) bool {
	data := []byte{BACKSLASH, 'x', 0, 0}
	hex.Encode(data[2:], []byte{char})
	out.Write(data)
	return true
}

func (e *shellEscaper) backslash(char byte, out *bytes.Buffer) bool {
	out.Write([]byte{BACKSLASH, char})
	return true
}

func (e *shellEscaper) escaped(str string, out *bytes.Buffer) bool {
	out.WriteString(str)
	return true
}

func (e *shellEscaper) quoted(char byte, out *bytes.Buffer) bool {
	out.WriteByte(char)
	return true
}

func (e *shellEscaper) literal(char byte, out *bytes.Buffer) bool {
	out.WriteByte(char)
	return false
}

func (e *shellEscaper) getEscapedString(str string) string {
	if str == "" {
		return "''"
	}

	escape := false
	in := []byte(str)
	out := bytes.NewBuffer(make([]byte, 0, len(str)*2))

	for _, c := range in {
		if e.processChar(c, out) {
			escape = true
		}
	}

	outStr := out.String()
	if escape {
		outStr = "$'" + outStr + "'"
	}

	return outStr
}

func (e *shellEscaper) processChar(char byte, out *bytes.Buffer) bool {
	switch {
	case char == TAB:
		return e.escaped(`\t`, out)
	case char == LF:
		return e.escaped(`\n`, out)
	case char == CR:
		return e.escaped(`\r`, out)
	case char <= US:
		return e.hex(char, out)
	case char <= AMPERSTAND:
		return e.quoted(char, out)
	case char == SINGLE_QUOTE:
		return e.backslash(char, out)
	case char <= PLUS:
		return e.quoted(char, out)
	case char <= NINE:
		return e.literal(char, out)
	case char <= QUESTION:
		return e.quoted(char, out)
	case char <= LOWERCASE_Z:
		return e.literal(char, out)
	case char == OPEN_BRACKET:
		return e.quoted(char, out)
	case char == BACKSLASH:
		return e.backslash(char, out)
	case char <= CLOSE_BRACKET:
		return e.quoted(char, out)
	case char == UNDERSCORE:
		return e.literal(char, out)
	case char <= BACKTICK:
		return e.quoted(char, out)
	case char <= TILDA:
		return e.quoted(char, out)
	case char == DEL:
		return e.hex(char, out)
	default:
		return e.hex(char, out)
	}
}
