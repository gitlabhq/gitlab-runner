package trace

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/transform"
)

func TestVariablesMaskingBoundary(t *testing.T) {
	tests := map[string]struct {
		values   []string
		expected string
	}{
		"no escaping at all http://example.org/?test=foobar": {
			expected: "no escaping at all http://example.org/?test=foobar",
		},
		"at the start of the buffer": {
			values:   []string{"at"},
			expected: "[MASKED] the start of the buffer",
		},
		"in the middle of the buffer": {
			values:   []string{"middle"},
			expected: "in the [MASKED] of the buffer",
		},
		"at the end of the buffer": {
			values:   []string{"buffer"},
			expected: "at the end of the [MASKED]",
		},
		"all values are masked": {
			values:   []string{"all", "values", "are", "masked"},
			expected: "[MASKED] [MASKED] [MASKED] [MASKED]",
		},
		"prefixed and suffixed: xfoox ybary ffoo barr ffooo bbarr": {
			values:   []string{"foo", "bar"},
			expected: "prefixed and suffixed: x[MASKED]x y[MASKED]y f[MASKED] [MASKED]r f[MASKED]o b[MASKED]r",
		},
		"prefix|ed, su|ffi|xed |and split|:| xfo|ox y|bary ffo|o ba|rr ffooo b|barr": {
			values:   []string{"foo", "bar"},
			expected: "prefixed, suffixed and split: x[MASKED]x y[MASKED]y f[MASKED] [MASKED]r f[MASKED]o b[MASKED]r",
		},
		"sp|lit al|l val|ues ar|e |mask|ed": {
			values:   []string{"split", "all", "values", "are", "masked"},
			expected: "[MASKED] [MASKED] [MASKED] [MASKED] [MASKED]",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			buffer, err := New()
			require.NoError(t, err)
			defer buffer.Close()

			buffer.SetMasked(tc.values)

			parts := bytes.Split([]byte(tn), []byte{'|'})
			for _, part := range parts {
				_, err = buffer.Write(part)
				require.NoError(t, err)
			}

			buffer.Finish()

			content, err := buffer.Bytes(0, 1000)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, string(content))
		})
	}
}

func TestMaskNonEOFSafeBoundary(t *testing.T) {
	// The truncated output from unflushed results depends on the max token
	// size we're trying to find.
	// If this test fails, it's likely it needs to be adjusted because the
	// max token size has changed.
	tests := []struct {
		input    string
		expected string
	}{
		{
			input:    "cannot safely flush: secret secre",
			expected: "cannot safely flush: [MASKED]",
		},
		{
			input:    "cannot safely flush: secret secre!",
			expected: "cannot safely flush: [MASKED]",
		},
		{
			input:    "can safely flush: secret secre\n",
			expected: "can safely flush: [MASKED] secre\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			buffer, err := New()
			require.NoError(t, err)
			defer buffer.Close()

			buffer.SetMasked([]string{"secret"})

			_, err = buffer.Write([]byte(tc.input))
			require.NoError(t, err)

			content, err := buffer.Bytes(0, 1000)
			require.NoError(t, err)

			assert.Equal(t, tc.expected, string(content))
		})
	}
}

func TestMaskShortWrites(t *testing.T) {
	tests := []string{
		"the source is too long to copy to the destination",
		"a phrase is replaced but the source is too long to copy to the destination",
		"the source is too long to copy to the destination and but contains a phrase",
		"included phrase is replaced but replacement text is too for destination",
	}

	for _, tn := range tests {
		t.Run(tn, func(t *testing.T) {
			var dst [10]byte

			transformer := NewPhraseTransform("phrase")

			_, _, err := transformer.Transform(dst[:], []byte(tn), true)
			assert.ErrorIs(t, err, transform.ErrShortDst)

			_, _, err = transformer.Transform(dst[:], []byte(tn), false)
			assert.ErrorIs(t, err, transform.ErrShortDst)
		})
	}
}
