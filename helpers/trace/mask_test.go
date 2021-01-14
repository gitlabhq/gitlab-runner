// +build !integration

package trace

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/transform"
)

func TestVariablesMaskingBoundary(t *testing.T) {
	tests := []struct {
		input    string
		values   []string
		expected string
	}{
		{
			input:    "no escaping at all http://example.org/?test=foobar",
			expected: "no escaping at all http://example.org/?test=foobar",
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
			input:    "http://example.com/?private_token=deadbeef sensitive URL at the start",
			expected: "http://example.com/?private_token=[MASKED] sensitive URL at the start",
		},
		{
			input:    "a sensitive URL at the end http://example.com/?authenticity_token=deadbeef",
			expected: "a sensitive URL at the end http://example.com/?authenticity_token=[MASKED]",
		},
		{
			input:    "a sensitive URL http://example.com/?rss_token=deadbeef in the middle",
			expected: "a sensitive URL http://example.com/?rss_token=[MASKED] in the middle",
		},
		{
			input:    "a sensitive URL http://example.com/?X-AMZ-sigNATure=deadbeef with mixed case",
			expected: "a sensitive URL http://example.com/?X-AMZ-sigNATure=[MASKED] with mixed case",
		},
		{
			input:    "a sensitive URL http://example.com/?param=second&x-amz-credential=deadbeef second param",
			expected: "a sensitive URL http://example.com/?param=second&x-amz-credential=[MASKED] second param",
		},
		{
			input:    "a sensitive URL http://example.com/?rss_token=hide&x-amz-credential=deadbeef both params",
			expected: "a sensitive URL http://example.com/?rss_token=[MASKED]&x-amz-credential=[MASKED] both params",
		},
		//nolint:lll
		{
			input:    "a long sensitive URL http://example.com/?x-amz-credential=abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789",
			expected: "a long sensitive URL http://example.com/?x-amz-credential=[MASKED]",
		},
		//nolint:lll
		{
			input:    "a really long sensitive URL http://example.com/?x-amz-credential=" + strings.Repeat("0", sensitiveURLMaxTokenSize-len("x-amz-credential=")),
			expected: "a really long sensitive URL http://example.com/?x-amz-credential=[MASKED]",
		},
		//nolint:lll
		{
			input:    "a sensitive URL containing a token too long to scrub http://example.com/?x-amz-credential=" + strings.Repeat("0", sensitiveURLMaxTokenSize-len("x-amz-credential=")+1),
			expected: "a sensitive URL containing a token too long to scrub http://example.com/?x-amz-credential=" + strings.Repeat("0", sensitiveURLMaxTokenSize-len("x-amz-credential=")+1),
		},
		{
			input:    "spl|it sensit|ive UR|L http://example.com/?x-amz-cred|ential=abcdefghij|klmnopqrstuvwxyz01234567",
			expected: "split sensitive URL http://example.com/?x-amz-credential=[MASKED]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			buffer, err := New()
			require.NoError(t, err)
			defer buffer.Close()

			buffer.SetMasked(tc.values)

			parts := bytes.Split([]byte(tc.input), []byte{'|'})
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
			input:    "cannot safely flush: secret secre\t",
			expected: "cannot safely flush: [MASKED]",
		},
		{
			input:    "can safely flush: secret secre\r",
			expected: "can safely flush: [MASKED] secre\r",
		},
		{
			input:    "can safely flush: secret secre\n",
			expected: "can safely flush: [MASKED] secre\n",
		},
		{
			input:    "can safely flush: secret secre\r\n",
			expected: "can safely flush: [MASKED] secre\r\n",
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

			transformer := newPhraseTransform("phrase")

			_, _, err := transformer.Transform(dst[:], []byte(tn), true)
			assert.ErrorIs(t, err, transform.ErrShortDst)

			_, _, err = transformer.Transform(dst[:], []byte(tn), false)
			assert.ErrorIs(t, err, transform.ErrShortDst)
		})
	}
}
