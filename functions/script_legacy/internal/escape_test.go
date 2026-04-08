//go:build !integration

package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeForAnsiC(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no escaping needed",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "backslash",
			input:    "path\\to\\file",
			expected: "path\\\\to\\\\file",
		},
		{
			name:     "single quote",
			input:    "it's working",
			expected: "it\\'s working",
		},
		{
			name:     "newline",
			input:    "line1\nline2",
			expected: "line1\\nline2",
		},
		{
			name:     "tab",
			input:    "col1\tcol2",
			expected: "col1\\tcol2",
		},
		{
			name:     "carriage return",
			input:    "text\rmore",
			expected: "text\\rmore",
		},
		{
			name:     "mixed special chars",
			input:    "echo 'hello'\nworld\\test",
			expected: "echo \\'hello\\'\\nworld\\\\test",
		},
		{
			name:     "command with quotes",
			input:    `echo "hello" 'world'`,
			expected: "echo \"hello\" \\'world\\'",
		},
		{
			name:     "ANSI escape sequence - ESC character",
			input:    "\x1b[1;32mGreen\x1b[0m",
			expected: "\\x1b[1;32mGreen\\x1b[0m",
		},
		{
			name:     "terminal clear screen",
			input:    "\x1b[2J\x1b[H",
			expected: "\\x1b[2J\\x1b[H",
		},
		{
			name:     "null byte",
			input:    "text\x00more",
			expected: "text\\x00more",
		},
		{
			name:     "DEL character",
			input:    "text\x7fmore",
			expected: "text\\x7fmore",
		},
		{
			name:     "non-ASCII characters",
			input:    "café",
			expected: "caf\\xc3\\xa9",
		},
		{
			name:     "mixed control and printable",
			input:    "Hello\x1b[31mRed\x1b[0mWorld",
			expected: "Hello\\x1b[31mRed\\x1b[0mWorld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EscapeForAnsiC(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEscapeForAnsiC_SecurityFeatures(t *testing.T) {
	// Test that terminal manipulation sequences are properly escaped
	tests := []struct {
		name        string
		input       string
		description string
	}{
		{
			name:        "prevent screen clear",
			input:       "\x1b[2J",
			description: "ESC[2J clears entire screen",
		},
		{
			name:        "prevent cursor positioning",
			input:       "\x1b[10;20H",
			description: "ESC[10;20H positions cursor at row 10, col 20",
		},
		{
			name:        "prevent color manipulation",
			input:       "\x1b[31m\x1b[42m",
			description: "ESC[31m red foreground, ESC[42m green background",
		},
		{
			name:        "prevent line deletion",
			input:       "\x1b[2K",
			description: "ESC[2K deletes entire line",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EscapeForAnsiC(tt.input)
			assert.True(t, containsHexEscape(result),
				"Expected %s to be hex-escaped for security, got: %q", tt.description, result)
		})
	}
}

func containsHexEscape(s string) bool {
	return len(s) >= 4 && s[0] == '\\' && s[1] == 'x'
}

func TestEscapeForPosix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "''",
		},
		{
			name:     "simple text",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "text with space",
			input:    "hello world",
			expected: `"hello world"`,
		},
		{
			name:     "double quote",
			input:    `echo "hello"`,
			expected: `"echo \"hello\""`,
		},
		{
			name:     "backtick",
			input:    "echo `date`",
			expected: "\"echo \\`date\\`\"",
		},
		{
			name:     "backslash",
			input:    `path\to\file`,
			expected: `"path\\to\\file"`,
		},
		{
			name:     "dollar sign",
			input:    "echo $VAR",
			expected: `"echo \$VAR"`,
		},
		{
			name:     "special chars needing quotes",
			input:    "test!value",
			expected: `"test!value"`,
		},
		{
			name:     "command with multiple special chars",
			input:    `echo "test" $VAR | grep foo`,
			expected: `"echo \"test\" \$VAR | grep foo"`,
		},
		{
			name:     "parentheses",
			input:    "cmd (arg1 arg2)",
			expected: `"cmd (arg1 arg2)"`,
		},
		{
			name:     "glob patterns",
			input:    "ls *.txt",
			expected: `"ls *.txt"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EscapeForPosix(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
