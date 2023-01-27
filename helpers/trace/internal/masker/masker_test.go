//go:build !integration

package masker

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error {
	return nil
}

func TestMasking(t *testing.T) {
	tests := []struct {
		input    string
		values   []string
		expected string
	}{
		{
			input:    "empty secrets have no affect",
			values:   []string{""},
			expected: "empty secrets have no affect",
		},
		{
			input:    "no escaping at all",
			expected: "no escaping at all",
		},
		{
			input:    "secrets",
			values:   []string{"secrets"},
			expected: "[MASKED]",
		},
		{
			input:    "secret|s",
			values:   []string{"secrets"},
			expected: "[MASKED]",
		},
		{
			input:    "s|ecrets",
			values:   []string{"secrets"},
			expected: "[MASKED]",
		},
		{
			input:    "secretssecrets",
			values:   []string{"secrets"},
			expected: "[MASKED][MASKED]",
		},
		{
			input:    "ssecrets",
			values:   []string{"secrets"},
			expected: "s[MASKED]",
		},
		{
			input:    "s|secrets",
			values:   []string{"secrets"},
			expected: "s[MASKED]",
		},
		{
			input:    "at the start of the buffer",
			values:   []string{"at"},
			expected: "[MASKED] the start of the buffer",
		},
		{
			input:    "in the middle of the buffer",
			values:   []string{"middle"},
			expected: "in the [MASKED] of the buffer",
		},
		{
			input:    "at the end of the buffer",
			values:   []string{"buffer"},
			expected: "at the end of the [MASKED]",
		},
		{
			input:    "all values are masked",
			values:   []string{"all", "values", "are", "masked"},
			expected: "[MASKED] [MASKED] [MASKED] [MASKED]",
		},
		{
			input:    "prefixed and suffixed: xfoox ybary ffoo barr ffooo bbarr",
			values:   []string{"foo", "bar"},
			expected: "prefixed and suffixed: x[MASKED]x y[MASKED]y f[MASKED] [MASKED]r f[MASKED]o b[MASKED]r",
		},
		{
			input:    "prefix|ed, su|ffi|xed |and split|:| xfo|ox y|bary ffo|o ba|rr ffooo b|barr",
			values:   []string{"foo", "bar"},
			expected: "prefixed, suffixed and split: x[MASKED]x y[MASKED]y f[MASKED] [MASKED]r f[MASKED]o b[MASKED]r",
		},
		{
			input:    "sp|lit al|l val|ues ar|e |mask|ed",
			values:   []string{"split", "all", "values", "are", "masked"},
			expected: "[MASKED] [MASKED] [MASKED] [MASKED] [MASKED]",
		},
		{
			input:    "prefix_mask mask prefix_|mask prefix_ma|sk mas|k",
			values:   []string{"mask", "prefix_mask"},
			expected: "[MASKED] [MASKED] [MASKED] [MASKED] [MASKED]",
		},
		{
			input:    "large secret: " + strings.Repeat("_", 8000) + "|" + strings.Repeat("_", 8000),
			values:   []string{strings.Repeat("_", 8000*2)},
			expected: "large secret: [MASKED]",
		},
		{
			input:    "overlap: this is the en| foobar",
			values:   []string{"this is the end", "en foobar", "en"},
			expected: "overlap: this is the [MASKED]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			buf := new(bytes.Buffer)

			m := New(nopCloser{buf}, tc.values)

			parts := bytes.Split([]byte(tc.input), []byte{'|'})
			for _, part := range parts {
				n, err := m.Write(part)
				require.NoError(t, err)

				assert.Equal(t, len(part), n)
			}

			require.NoError(t, m.Close())
			assert.Equal(t, tc.expected, buf.String())
		})
	}
}
