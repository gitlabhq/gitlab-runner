//go:build !integration

package omitwriter

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_OmitWriter(t *testing.T) {
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
			writer := New()

			for _, input := range tc.input {
				n, err := writer.Write([]byte(input))
				require.NoError(t, err)
				require.Equal(t, len(input), n)
			}

			require.Equal(t, tc.expected, writer.Error().Error())
		})
	}
}

// Test_OmitWriter_IOCopy holds the io.Writer contract: io.Copy, like
// stdcopy.StdCopy, fails with io.ErrShortWrite when a write reports fewer
// bytes than it was given.
func Test_OmitWriter_IOCopy(t *testing.T) {
	writer := New()

	n, err := io.Copy(writer, strings.NewReader("some stderr output"))
	require.NoError(t, err)
	require.Equal(t, int64(len("some stderr output")), n)
	require.EqualError(t, writer.Error(), "some stderr output")
}

func Test_OmitWriter_NothingWritten(t *testing.T) {
	writer := New()
	require.NoError(t, writer.Error())

	_, err := writer.Write(nil)
	require.NoError(t, err)
	require.NoError(t, writer.Error())
}
