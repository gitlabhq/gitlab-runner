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

func TestDotEnvEscape(t *testing.T) {
	var tests = []struct {
		name      string
		variables map[string]string
		expected  string
	}{
		{
			name:      "Simple key-value pair",
			variables: map[string]string{"KEY": "value"},
			expected:  "KEY=\"value\"\n",
		},
		{
			name:      "Value with spaces",
			variables: map[string]string{"KEY": "value with spaces"},
			expected:  "KEY=\"value with spaces\"\n",
		},
		{
			name:      "Value with special characters",
			variables: map[string]string{"KEY": "value\\with\\special\\characters"},
			expected:  "KEY=\"value\\\\with\\\\special\\\\characters\"\n",
		},
		{
			name:      "Value with quotes",
			variables: map[string]string{"KEY": "value \"with\" quotes"},
			expected:  "KEY=\"value \\\"with\\\" quotes\"\n",
		},
		{
			name:      "Value with newlines",
			variables: map[string]string{"KEY": "value\nwith\nnewlines"},
			expected:  "KEY=\"value\\nwith\\nnewlines\"\n",
		},
		{
			name:      "Value with tabs",
			variables: map[string]string{"KEY": "value\twith\ttabs"},
			expected:  "KEY=\"value\twith\ttabs\"\n",
		},
		{
			name:      "Empty value",
			variables: map[string]string{"KEY": ""},
			expected:  "KEY=\"\"\n",
		},
		{
			name:      "Multiple valid key-value pairs",
			variables: map[string]string{"KEY1": "value1", "KEY2": "value2"},
			expected:  "KEY1=\"value1\"\nKEY2=\"value2\"\n",
		},
		{
			name:      "Invalid key is skipped",
			variables: map[string]string{"INVALID-KEY": "value", "VALID_KEY": "valid_value"},
			expected:  "VALID_KEY=\"valid_value\"\n",
		},
		{
			name:      "Unicode characters",
			variables: map[string]string{"UNICODE": "こんにちは世界🌍"},
			expected:  "UNICODE=\"こんにちは世界🌍\"\n",
		},
		{
			name:      "Value with equals sign",
			variables: map[string]string{"KEY": "value=with=equals"},
			expected:  "KEY=\"value=with=equals\"\n",
		},
		{
			name:      "Value starting and ending with spaces",
			variables: map[string]string{"KEY": " value with spaces "},
			expected:  "KEY=\" value with spaces \"\n",
		},
		{
			name:      "Value with dollar signs",
			variables: map[string]string{"KEY": "value with $dollar signs"},
			expected:  "KEY=\"value with $dollar signs\"\n",
		},
		{
			name:      "Empty map",
			variables: map[string]string{},
			expected:  "",
		},
		{
			name:      "Nil map",
			variables: nil,
			expected:  "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output := DotEnvEscape(test.variables)
			assert.Equal(t, test.expected, output, "variables=%v", test.variables)
		})
	}
}
