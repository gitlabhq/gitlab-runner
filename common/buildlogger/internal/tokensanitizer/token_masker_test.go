//go:build !integration

package tokensanitizer

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger/internal"
)

var words = []string{"Lorem", "ipsum", "odor", "amet", "consectetuer", "adipiscing", "elit",
	"Ad", "sagittis", "volutpat", "aptent", "augue", "dis", "dui", "primis", "laoreet",
	"taciti", "fusce", "sapien", "ullamcorper", "ex", "venenatis"}

func TestTokenMasking(t *testing.T) {
	tests := map[string]struct {
		prefixes []string
		input    string
		expected string
	}{
		"simple prefix masking": {
			input:    "Lorem ipsum dolor sit amet, ex ea commodo glpat-imperdiet in voluptate velit esse",
			expected: "Lorem ipsum dolor sit amet, ex ea commodo glpat-[MASKED] in voluptate velit esse",
		},
		"prefix at the end of the line": {
			input:    "Lorem ipsum dolor sit amet, ex ea commodo in voluptate velit esseglpat-imperdiet",
			expected: "Lorem ipsum dolor sit amet, ex ea commodo in voluptate velit esseglpat-[MASKED]",
		},
		"prefix at the beginning of the line": {
			input:    "glpat-imperdiet Lorem ipsum dolor sit amet, ex ea commodo in voluptate velit esse",
			expected: "glpat-[MASKED] Lorem ipsum dolor sit amet, ex ea commodo in voluptate velit esse",
		},
		"prefix inside of the line": {
			input:    "esseglpat-imperdiet=_-. end Lorem ipsum dolor sit amet, ex ea commodo  in voluptate velit",
			expected: "esseglpat-[MASKED] end Lorem ipsum dolor sit amet, ex ea commodo  in voluptate velit",
		},
		"two prefix concatenate": {
			input:    "glpat-impglpat-erdiet Lorem ipsum dolor sit amet, ex ea commodo  in voluptate velit esse",
			expected: "glpat-[MASKED] Lorem ipsum dolor sit amet, ex ea commodo  in voluptate velit esse",
		},
		"multiple packets pat masking": {
			input:    "glpat|-imperdiet Lorem ipsum dolor sit amet, ex ea commodo gl|pat-imperdiet in voluptate velit esse",
			expected: "glpat-[MASKED] Lorem ipsum dolor sit amet, ex ea commodo glpat-[MASKED] in voluptate velit esse",
		},
		"second multiple packets pat masking": {
			input:    "glpat| -imperdiet Lorem ipsum dolor sit amet",
			expected: "glpat -imperdiet Lorem ipsum dolor sit amet",
		},
		"long input": {
			input:    "Lorglpat-ipsu dolor sit amglpat-t, consglpat-ctglpat-tur adipiscing glpat-lit, sglpat-d do glpat-iusmod tglpat-mpor incididunt ut laborglpat-=_ glpat-t dolorglpat-=_ magna aliqua.",
			expected: "Lorglpat-[MASKED] dolor sit amglpat-[MASKED], consglpat-[MASKED] adipiscing glpat-[MASKED], sglpat-[MASKED] do glpat-[MASKED] tglpat-[MASKED] incididunt ut laborglpat-[MASKED] glpat-[MASKED] dolorglpat-[MASKED] magna aliqua.",
		},
		"multiple packets long input": {
			input:    "Lorglpat-ipsu dolor sit amglp|at-t, consglpat-ctg|lpat-tur adipiscing glpat-lit, sglpat-|d do glpat-iusmod t|glpat-mpor incididunt ut |laborglpat-=_ glpat-t dolorglpat-=_ magna aliqua.",
			expected: "Lorglpat-[MASKED] dolor sit amglpat-[MASKED], consglpat-[MASKED] adipiscing glpat-[MASKED], sglpat-[MASKED] do glpat-[MASKED] tglpat-[MASKED] incididunt ut laborglpat-[MASKED] glpat-[MASKED] dolorglpat-[MASKED] magna aliqua.",
		},
		"second long input": {
			input:    "Lorglpat- ipsu dolor sit amglpat-t, consglpat-ctglpat-tur adipiscing glpat-lit, sglpat-d do glpat-iusmod tglpat-mpor incididunt ut laborglpat-=_ glpat-t dolorglpat-=_ magna aliqua.",
			expected: "Lorglpat- ipsu dolor sit amglpat-[MASKED], consglpat-[MASKED] adipiscing glpat-[MASKED], sglpat-[MASKED] do glpat-[MASKED] tglpat-[MASKED] incididunt ut laborglpat-[MASKED] glpat-[MASKED] dolorglpat-[MASKED] magna aliqua.",
		},
		"custom prefix with default one at the beginning of the line": {
			prefixes: []string{"token-"},
			input:    "token-imperdiet Lorem ipsum dolor sit amet, ex ea commodo in voluptate velit esse",
			expected: "token-[MASKED] Lorem ipsum dolor sit amet, ex ea commodo in voluptate velit esse",
		},
		"custom prefix with default one multiple packets long input": {
			prefixes: []string{"tok-"},
			input:    "Lortok-ipsu dolor sit amt|ok-t, cons-ctg|lpat-tur adipiscing tok-lit, stok-|d gltok-test do tok-iusmod t|tok-mpor incididunt ut |labortok-=_ tok-t dolortok-=_ magna aliqua. Tglpat-llus orci ac auctor auguglpat-eee mauris auguglpat-wEr_ lorem",
			expected: "Lortok-[MASKED] dolor sit amtok-[MASKED], cons-ctglpat-[MASKED] adipiscing tok-[MASKED], stok-[MASKED] gltok-[MASKED] do tok-[MASKED] ttok-[MASKED] incididunt ut labortok-[MASKED] tok-[MASKED] dolortok-[MASKED] magna aliqua. Tglpat-[MASKED] orci ac auctor auguglpat-[MASKED] mauris auguglpat-[MASKED] lorem",
		},
		"ignored sixteenth prefix and more": {
			prefixes: []string{"mask1-", "mask2-", "mask3-", "mask4-", "mask5-", "mask6-", "mask7-", "mask8-", "mask9-", "mask10-", "mask11-"},
			input:    "Lormask1-ipsu dolor sit amm|ask2-t, cons-ctg|lpat-tur adipiscing mask5-lit, smask11-|d do mask7-iusmod t|glpat-mpor incididunt ut |labormask10-=_ mask9-t",
			expected: "Lormask1-[MASKED] dolor sit ammask2-[MASKED], cons-ctglpat-[MASKED] adipiscing mask5-[MASKED], smask11-d do mask7-iusmod tglpat-[MASKED] incididunt ut labormask10-=_ mask9-t",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			buf := new(bytes.Buffer)

			m := New(internal.NewNopCloser(buf), internal.Unique(append(tc.prefixes, DefaultTokenPrefixes...)))

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

func BenchmarkTokenMaskingPerformance(b *testing.B) {
	paragraphs := map[string]struct {
		input string
	}{
		"100K words": {
			input: generateParagraph(100000, DefaultTokenPrefixes, words),
		},
		"300K words": {
			input: generateParagraph(300000, DefaultTokenPrefixes, words),
		},
		"800K words": {
			input: generateParagraph(800000, DefaultTokenPrefixes, words),
		},
		"1.5M words": {
			input: generateParagraph(1500000, DefaultTokenPrefixes, words),
		},
		"5M words": {
			input: generateParagraph(5000000, DefaultTokenPrefixes, words),
		},
	}

	tests := map[string]struct {
		defaultToken []string
		// expected     string
	}{
		"one default token": {
			defaultToken: DefaultTokenPrefixes[:1],
		},
		"two default tokens": {
			defaultToken: DefaultTokenPrefixes[:2],
		},
		"four default tokens": {
			defaultToken: DefaultTokenPrefixes[:4],
		},
		"all but one default tokens": {
			defaultToken: DefaultTokenPrefixes[:len(DefaultTokenPrefixes)-1],
		},
		"all default tokens": {
			defaultToken: DefaultTokenPrefixes,
		},
	}

	for pn, pc := range paragraphs {
		for tn, tc := range tests {
			b.Run(fmt.Sprintf("%s_%s", pn, tn), func(b *testing.B) {
				b.ResetTimer()
				b.ReportAllocs()

				for n := 0; n < b.N; n++ {
					m := New(internal.NewNopCloser(io.Discard), internal.Unique(tc.defaultToken))

					n, err := m.Write([]byte(pc.input))
					b.SetBytes(int64(n))
					require.NoError(b, err)
					require.NoError(b, m.Close())
					assert.Equal(b, len([]byte(pc.input)), n)
				}
			})
		}
	}
}

func BenchmarkTokenMaskingDuration(b *testing.B) {
	input := generateParagraph(5000000, DefaultTokenPrefixes, words)
	b.ResetTimer()
	b.ReportAllocs()

	for n := 0; n < b.N; n++ {
		m := New(internal.NewNopCloser(io.Discard), internal.Unique(DefaultTokenPrefixes))

		n, err := m.Write([]byte(input))
		b.SetBytes(int64(n))
		require.NoError(b, err)
		require.NoError(b, m.Close())
		assert.Equal(b, len([]byte(input)), n)
	}
}

func generateParagraph(numberOfWords int, token, wordPool []string) string {
	words := append([]string{}, wordPool...)
	sb := strings.Builder{}

	for _, tok := range token {
		words = append(words, fmt.Sprintf("%slorem", tok))
	}

	for i := 0; i < numberOfWords; i++ {
		if i > 0 {
			sb.WriteString(" ")
		}

		sb.WriteString(words[rand.Intn(len(words))])
	}

	return sb.String()
}
