//go:build !integration

package helpers

import (
	"crypto/rand"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func BenchmarkEscaping(b *testing.B) {
	data := make([]byte, 1024*1024)
	if _, err := rand.Read(data); err != nil {
		panic(err)
	}

	input := string(data)

	b.Run("bash-ansi-c-shellescape", func(b *testing.B) {
		b.SetBytes(int64(len(input)))
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			ShellEscape(input)
		}
	})

	b.Run("posix-shellescape", func(b *testing.B) {
		b.SetBytes(int64(len(input)))
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			PosixShellEscape(input)
		}
	})

	b.Run("strconv.quote", func(b *testing.B) {
		b.SetBytes(int64(len(input)))
		b.ReportAllocs()

		for i := 0; i < b.N; i++ {
			strconv.Quote(input)
		}
	})
}

func TestShellEscape(t *testing.T) {
	var tests = []struct {
		in  string
		out string
	}{
		{"unquoted", "unquoted"},
		{"standard string", "$'standard string'"},
		{"+\t\n\r&", "$'+\\t\\n\\r&'"},
		{"", "''"},
		{"hello, 世界", "$'hello, \\xe4\\xb8\\x96\\xe7\\x95\\x8c'"},
		{"blackslash \\n", "$'blackslash \\\\n'"},
		{"f", "f"},
		{"\f", "$'\\f'"},
		{"export variable='test' && echo $variable", "$'export variable=\\'test\\' && echo $variable'"},
		{"$HOME", `$'$HOME'`},
		{"'$HOME'", `$'\'$HOME\''`},
	}

	for _, test := range tests {
		actual := ShellEscape(test.in)
		assert.Equal(t, test.out, actual, "src=%v", test.in)
	}
}

func TestPosixShellEscape(t *testing.T) {
	var tests = []struct {
		in  string
		out string
	}{
		{"unquoted", "unquoted"},
		{"standard string", `"standard string"`},
		{"+\t\n\r&", "\"+\t\n\r&\""},
		{"", "''"},
		{"hello, 世界", `"hello, 世界"`},
		{"blackslash \\n", "\"blackslash \\\\n\""},
		{"f", "f"},
		{"\f", "\f"},
		{"export variable='test' && echo $variable", `"export variable='test' && echo \$variable"`},
	}

	for _, test := range tests {
		actual := PosixShellEscape(test.in)
		assert.Equal(t, test.out, actual, "src=%v", test.in)
	}
}
