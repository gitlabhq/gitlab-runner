//go:build !integration
// +build !integration

package trace

import (
	"bytes"
	"encoding/base64"
	"io"
	"math"
	"math/rand"
	"strings"
	"testing"
	"unicode/utf8"

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

		// data written at certain boundaries could cause short writes
		// due to https://github.com/golang/go/issues/46892
		//
		// issue: https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28001
		//
		// These tests are scenarios that would cause this to occur:
		//nolint:lll
		{
			input:    "buffered writes due to large secret: " + strings.Repeat("_", 2500) + "|" + strings.Repeat("+", 3000),
			values:   []string{strings.Repeat("+", 3000)},
			expected: "buffered writes due to large secret: " + strings.Repeat("_", 2500) + "[MASKED]",
		},
		{
			// a previous bug in safecopy always used the last index of a "safe token"
			// even when a better option was a available to safely copy more data.
			input:    "head slice\n" + strings.Repeat(".", 4095) + "|tail slice",
			values:   []string{"zzz"},
			expected: "head slice\n" + strings.Repeat(".", 4095) + "tail slice",
		},

		// large secrets / flushing on certain boundaries that exceed the internal text/transform
		// buffer results in short writes.
		// https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27964
		{
			input:    "large secret, but no match: " + strings.Repeat("_", 3000) + "|" + strings.Repeat("+", 3000),
			values:   []string{strings.Repeat("_", 8000)},
			expected: "large secret, but no match: " + strings.Repeat("_", 3000) + strings.Repeat("+", 3000),
		},
		//nolint:lll
		{
			input:    "large secret, full sized/full match: " + strings.Repeat("_", maxPhraseSize/2) + "|" + strings.Repeat("_", maxPhraseSize-(maxPhraseSize/2)),
			values:   []string{strings.Repeat("_", maxPhraseSize)},
			expected: "large secret, full sized/full match: [MASKED]",
		},
		//nolint:lll
		{
			input:    "large secret, over sized/partial match/tailing reveal: " + strings.Repeat("_", maxPhraseSize/2) + "|" + strings.Repeat("_", maxPhraseSize-(maxPhraseSize/2)) + "endsuffix",
			values:   []string{strings.Repeat("_", maxPhraseSize) + "endsuffix"},
			expected: "large secret, over sized/partial match/tailing reveal: [MASKED]endsuffix",
		},
		{
			input:    "large secret, 2x mask: " + strings.Repeat("_", 3000) + "|" + strings.Repeat("_", 3000),
			values:   []string{strings.Repeat("_", 6000)},
			expected: "large secret, 2x mask: [MASKED]" + strings.Repeat("_", 6000-maxPhraseSize),
		},
		{
			input:    "large secret mask in single write: " + strings.Repeat("_", 6000),
			values:   []string{strings.Repeat("_", 6000)},
			expected: "large secret mask in single write: [MASKED]",
		},
		//nolint:lll
		{
			input:    "undersized partial matches should have no masks: __nomatch __|small __" + strings.Repeat("0", 3000) + "|" + strings.Repeat("0", 3000),
			values:   []string{"__x"},
			expected: "undersized partial matches should have no masks: __nomatch __small __" + strings.Repeat("0", 3000) + strings.Repeat("0", 3000),
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
				n, err := buffer.Write(part)
				require.NoError(t, err)

				assert.Equal(t, len(part), n)
			}

			buffer.Finish()

			content, err := buffer.Bytes(0, math.MaxInt64)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, string(content))
		})
	}
}

func TestPhraseFind(t *testing.T) {
	tests := []struct {
		input  string
		phrase string
		match  matchType
		index  int
	}{
		{
			input: "this text [secret] contains the secret",
			match: fullMatch,
			index: 10,
		},
		{
			input: "this text has no secret",
			match: noPossibleMatch,
			index: 23,
		},
		{
			input: "within this text [secret] there's two [secret]s",
			match: fullMatch,
			index: 17,
		},
		{
			input: "within this text there's a partial [secre",
			match: partialMatch,
			index: 35,
		},
		{
			input: "within this text there's almost a [secret followed by a full [secret]",
			match: fullMatch,
			index: 61,
		},
		{
			input: "within this text there's almost a [secret followed by a partial [se",
			match: partialMatch,
			index: 64,
		},
		{
			input: "within this text there's a [[secret]",
			match: fullMatch,
			index: 28,
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			match, index := find([]byte(tc.input), []byte("[secret]"))
			assert.Equal(t, tc.match, match)
			assert.Equal(t, tc.index, index)
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
			expected: "cannot safely flush: [MASKED] ",
		},
		{
			input:    "cannot safely flush: secret secre!",
			expected: "cannot safely flush: [MASKED] secre!",
		},
		{
			input:    "cannot safely flush: secret secre\t",
			expected: "cannot safely flush: [MASKED] secre\t",
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
		//nolint:lll
		{
			input:    "can safely flush: \n: but doesn't use the last safe token if the input is much greater than the token being indexed sec",
			expected: "can safely flush: \n: but doesn't use the last safe token if the input is much greater than the token being indexed ",
		},
		{
			input:    "can safely flush: \n: and always uses the last safe token even on long inputs\n",
			expected: "can safely flush: \n: and always uses the last safe token even on long inputs\n",
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

func TestRandomCopyReadback(t *testing.T) {
	input := make([]byte, 1*1024*1024)
	_, err := rand.Read(input)
	require.NoError(t, err)

	input = bytes.ToValidUTF8(input, []byte(string(utf8.RuneError)))

	buffer, err := New()
	require.NoError(t, err)
	defer buffer.Close()

	buffer.SetLimit(math.MaxInt64)
	buffer.SetMasked([]string{"a"})

	n, err := io.Copy(buffer, bytes.NewReader(input))
	require.NoError(t, err)
	require.Equal(t, n, int64(len(input)))

	buffer.Finish()

	content, err := buffer.Bytes(0, math.MaxInt64)
	require.NoError(t, err)

	expected := strings.ReplaceAll(string(input), "a", "[MASKED]")

	assert.Equal(t, []byte(expected), content)
}

func TestMaskLargePartialMasks(t *testing.T) {
	count := 20
	chunkSize := maxPhraseSize / 2

	generateRandom := func(size int) string {
		buf := make([]byte, size)
		_, err := rand.Read(buf)
		require.NoError(t, err)

		return base64.StdEncoding.EncodeToString(buf)
	}

	var secrets []string
	for i := 1; i <= count; i++ {
		secrets = append(secrets, generateRandom(i*chunkSize))
	}

	var unmasked []string
	for i := 1; i <= count; i++ {
		unmasked = append(unmasked, generateRandom(i*chunkSize))
	}

	buffer, err := New()
	require.NoError(t, err)
	defer buffer.Close()

	buffer.SetMasked(secrets)

	for i := range secrets {
		n, err := buffer.Write([]byte(secrets[i]))
		require.NoError(t, err)
		assert.Equal(t, len(secrets[i]), n)

		n, err = buffer.Write([]byte(unmasked[i]))
		require.NoError(t, err)
		assert.Equal(t, len(unmasked[i]), n)
	}

	buffer.Finish()

	content, err := buffer.Bytes(0, math.MaxInt64)
	require.NoError(t, err)

	for i := range secrets {
		assert.NotContains(t, string(content), secrets[i], "contains secret %d", i)
		assert.Contains(t, string(content), unmasked[i], "doesn't contain safe unmasked %d", i)
	}
}
