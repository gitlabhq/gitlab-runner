//go:build !integration

package tokensanitizer

import (
	"bytes"
	"io"
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
			input:    "Lortok-ipsu dolor sit amt|ok-t, cons-ctg|lpat-tur adipiscing tok-lit, stok-|d do tok-iusmod t|tok-mpor incididunt ut |labortok-=_ tok-t dolortok-=_ magna aliqua. Tglpat-llus orci ac auctor auguglpat-eee mauris auguglpat-wEr_ lorem",
			expected: "Lortok-[MASKED] dolor sit amtok-[MASKED], cons-ctglpat-[MASKED] adipiscing tok-[MASKED], stok-[MASKED] do tok-[MASKED] ttok-[MASKED] incididunt ut labortok-[MASKED] tok-[MASKED] dolortok-[MASKED] magna aliqua. Tglpat-[MASKED] orci ac auctor auguglpat-[MASKED] mauris auguglpat-[MASKED] lorem",
		},
		"ignored eleventh prefix and more": {
			prefixes: []string{"mask1-", "mask2-", "mask3-", "mask4-", "mask5-", "mask6-", "mask7-", "mask8-", "mask9-", "mask10-", "mask11-"},
			input:    "Lormask1-ipsu dolor sit amm|ask2-t, cons-ctg|lpat-tur adipiscing mask5-lit, smask11-|d do mask7-iusmod t|glpat-mpor incididunt ut |labormask10-=_ mask9-t",
			expected: "Lormask1-[MASKED] dolor sit ammask2-[MASKED], cons-ctglpat-[MASKED] adipiscing mask5-[MASKED], smask11-d do mask7-[MASKED] tglpat-[MASKED] incididunt ut labormask10-=_ mask9-[MASKED]",
		},
		"whitespaced prefixes": {
			prefixes: []string{" mask1- ", "	mask2-", "mask3-	", "mask4-", "mask5-", "mask6-", "mask7-", "mask8-", "mask9-"},
			input:    "Lormask1-ipsu dolor sit amm|ask2-t, cons-ctg|lpat-tur adipiscing mask5-lit, smask11-|d do mask7-iusmod t|glpat-mpor incididunt ut |labormask10-=_ mask9-t",
			expected: "Lormask1-[MASKED] dolor sit ammask2-[MASKED], cons-ctglpat-[MASKED] adipiscing mask5-[MASKED], smask11-d do mask7-[MASKED] tglpat-[MASKED] incididunt ut labormask10-=_ mask9-[MASKED]",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			buf := new(bytes.Buffer)

			m := New(nopCloser{buf}, tc.prefixes)

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
