//go:build !integration

package exec

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStderrOmitWriter(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "simple",
			input:    []string{"hello"},
			expected: "hello",
		},
		{
			name:     "multiple writes",
			input:    []string{"first", "second"},
			expected: "firstsecond",
		},
		{
			name:     "full buffer",
			input:    []string{strings.Repeat("abcdefgh", (32*1024/8)-1) + "1234567"},
			expected: strings.Repeat("abcdefgh", (32*1024/8)-1) + "1234567",
		},
		{
			name:     "wrap around",
			input:    []string{strings.Repeat("abcdefgh", (32*1024/8)-1), "1234567wrapped_"},
			expected: "omitted 8... " + strings.Repeat("abcdefgh", (32*1024/8)-2) + "1234567wrapped_",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			writer := newStderrOmitWriter()

			for _, input := range tc.input {
				_, err := writer.Write([]byte(input))
				require.NoError(t, err)
			}

			require.Equal(t, tc.expected, writer.Error().Error())
		})
	}
}
