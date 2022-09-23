//go:build !integration

package urlsanitizer

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

//nolint:lll
func TestMasking(t *testing.T) {
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
			input:    "multiple: &private_token=hello &?x-amz-security-token=hello &?x-amz-security-token=hello ?x-amz-security?x-amz-security-token=hello",
			expected: "multiple: &private_token=[MASKED] &?x-amz-security-token=[MASKED] &?x-amz-security-token=[MASKED] ?x-amz-security?x-amz-security-token=[MASKED]",
		},
		{
			input:    "above known key size: http://example.org/?this-is-a-really-really-long-key-name=foobar",
			expected: "above known key size: http://example.org/?this-is-a-really-really-long-key-name=foobar",
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
		{
			input:    "a long sensitive URL http://example.com/?x-amz-credential=abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789",
			expected: "a long sensitive URL http://example.com/?x-amz-credential=[MASKED]",
		},
		{
			input:    "a really long sensitive URL http://example.com/?x-amz-credential=" + strings.Repeat("0", 8*1024) + " that is still scrubbed",
			expected: "a really long sensitive URL http://example.com/?x-amz-credential=[MASKED] that is still scrubbed",
		},
		{
			input:    "spl|it sensit|ive UR|L http://example.com/?x-amz-cred|ential=abcdefghij|klmnopqrstuvwxyz01234567",
			expected: "split sensitive URL http://example.com/?x-amz-credential=[MASKED]",
		},
		{
			input:    "newline: http://example.com/?x-amz-credential=abc\nhttp://example.com/?x-amz-credential=abc",
			expected: "newline: http://example.com/?x-amz-credential=[MASKED]\nhttp://example.com/?x-amz-credential=[MASKED]",
		},
		{
			input:    "control character: http://example.com/?x-amz-credential=abc\bhttp://example.com/?x-amz-credential=abc",
			expected: "control character: http://example.com/?x-amz-credential=[MASKED]\bhttp://example.com/?x-amz-credential=[MASKED]",
		},
		{
			input:    "rss_token=notmasked http://example.com/?rss_token=!@#$A&x-amz-credential=abc&test=test",
			expected: "rss_token=notmasked http://example.com/?rss_token=[MASKED]&x-amz-credential=[MASKED]&test=test",
		},
		{
			input:    "query string with no value: http://example.com/?x-amz-credential=&private_token=gitlab",
			expected: "query string with no value: http://example.com/?x-amz-credential=[MASKED]&private_token=[MASKED]",
		},
		{
			input:    "invalid URL with double &: http://example.com/?x-amz-credential=abc&&private_token=gitlab",
			expected: "invalid URL with double &: http://example.com/?x-amz-credential=[MASKED]&&private_token=[MASKED]",
		},
		{
			input:    "invalid URL with double ?: http://example.com/?x-amz-credential=abc??private_token=gitlab",
			expected: "invalid URL with double ?: http://example.com/?x-amz-credential=[MASKED]??private_token=[MASKED]",
		},
		{
			input:    "split on &: http://example.com/|&|x-amz-cre|dential=abc",
			expected: "split on &: http://example.com/&x-amz-credential=[MASKED]",
		},
		{
			input:    "split on ?: http://example.com/|?|x-amz-cre|dential=abc",
			expected: "split on ?: http://example.com/?x-amz-credential=[MASKED]",
		},
		{
			input:    "split after ?: http://example.com/|?||x-amz-cre|dential=abc",
			expected: "split after ?: http://example.com/?x-amz-credential=[MASKED]",
		},
		{
			input:    "interweaved tokens: ?|one ?x-amz-credential=abc two=three ?|one=two &token &x-amz-credential=abc =token ?=",
			expected: "interweaved tokens: ?one ?x-amz-credential=[MASKED] two=three ?one=two &token &x-amz-credential=[MASKED] =token ?=",
		},
		{
			input:    "terminated before mask: ?x",
			expected: "terminated before mask: ?x",
		},
		{
			input:    "terminated before mask: ?x|-",
			expected: "terminated before mask: ?x-",
		},
		{
			input:    "terminated before mask: ?x-|",
			expected: "terminated before mask: ?x-",
		},
		{
			input:    "terminated before mask: ?x-amz-credential=",
			expected: "terminated before mask: ?x-amz-credential=[MASKED]",
		},
		{
			input:    "terminated before mask: ?x-amz-credential=|",
			expected: "terminated before mask: ?x-amz-credential=[MASKED]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			buf := new(bytes.Buffer)

			m := New(nopCloser{buf})

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
