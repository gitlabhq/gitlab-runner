// Package tokensanitizer implements a masking Writer, where specified prefixes are
// used to replace the alphabet of any word matching the pattern {prefix}{alphabet}
// with the word "[MASKED]".
//
// The allowed characters in the alphabet part of the token are:
// * Alphanumeric characters: 0-9, a-z, A-Z
// * Specia characters: -, ., _, =
//
// To achieve masking over Write() boundaries, each prefix has its own writer.
// These writers are stacked, with each one calling the next, in length order,
// starting with the longest. This allows each writer to scan for their prefix
// in-turn, filtering data down to the next writer as required.
//
// Each tokensanitizer writer tracks when its prefix is being found, and scan until
// an unauthorized character is found. It then replaces it the matching characters.
// If a full match isn't found, sends the matched bytes to the next writer unmodified.
//
// The masking write for the `glpat-` prefix is created by default
package tokensanitizer

import (
	"bytes"
	"io"
	"sort"
	"strings"
)

const (
	defaultPATPrefix = "glpat-"
)

var (
	// alphabet is the character set we expect a token to comform to, not all
	// tokens will necessarily support all characters here, but the alphabet
	// should support all tokens.
	alphabet = [256]bool{
		'-': true, '.': true,

		'0': true, '1': true, '2': true, '3': true, '4': true, '5': true, '6': true,
		'7': true, '8': true, '9': true,

		'A': true, 'B': true, 'C': true, 'D': true, 'E': true, 'F': true, 'G': true,
		'H': true, 'I': true, 'J': true, 'K': true, 'L': true, 'M': true, 'N': true,
		'O': true, 'P': true, 'Q': true, 'R': true, 'S': true, 'T': true, 'U': true,
		'V': true, 'W': true, 'X': true, 'Y': true, 'Z': true,

		'_': true,

		'a': true, 'b': true, 'c': true, 'd': true, 'e': true, 'f': true, 'g': true,
		'h': true, 'i': true, 'j': true, 'k': true, 'l': true, 'm': true, 'n': true,
		'o': true, 'p': true, 'q': true, 'r': true, 's': true, 't': true, 'u': true,
		'v': true, 'w': true, 'x': true, 'y': true, 'z': true,

		'=': true,
	}
	mask = []byte("[MASKED]")
)

type TokenSanitizer struct {
	next io.WriteCloser
}

// New returns a new TokenSanitizer.
// We only allow 10 token prefixes at the moment. Everything else is being silently ignored
func New(w io.WriteCloser, prefixes []string) *TokenSanitizer {
	m := &TokenSanitizer{}

	prefixes = append(prefixes, defaultPATPrefix)
	sort.Slice(prefixes, func(i, j int) bool {
		return len(prefixes[i]) < len(prefixes[j])
	})

	m.next = w

	// create token sanitizer for each unique prefix
	unique := map[string]struct{}{}
	for i := 0; i < len(prefixes) && len(unique) < 10; i++ {
		trimmed := strings.TrimSpace(prefixes[i])
		if _, ok := unique[trimmed]; ok || trimmed == "" {
			continue
		}
		unique[trimmed] = struct{}{}
		m.next = &tokenSanitizer{next: m.next, prefix: []byte(trimmed)}
	}

	return m
}

func (m *TokenSanitizer) Write(p []byte) (n int, err error) {
	return m.next.Write(p)
}

// Close flushes any remaining data and closes the underlying writer.
func (m *TokenSanitizer) Close() error {
	return m.next.Close()
}

type tokenSanitizer struct {
	prefix   []byte
	matching int
	masked   bool
	next     io.WriteCloser
}

//nolint:funlen,gocognit
func (m *tokenSanitizer) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return
	}

	// fast path: if the write is "[MASKED]" from an upper-level, don't bother
	// processing it, send it to the next writer.
	if bytes.Equal(p, mask) {
		return m.next.Write(p)
	}

	var last int
	for n < len(p) {
		if m.matching == len(m.prefix) {
			if alphabet[p[n]] {
				m.masked = true
				n++
				last = n
				continue
			}

			if m.masked {
				m.masked = false
				_, err := m.next.Write(mask)
				if err != nil {
					return n, err
				}
			}
			m.matching = 0
		}

		// optimization: use the faster IndexByte to jump to the start of a
		// potential prefix and if not found, advance the whole buffer.
		if m.matching == 0 {
			off := bytes.IndexByte(p[n:], m.prefix[0])
			if off < 0 {
				n += len(p[n:])
				break
			}
			if off > -1 {
				n += off
			}
		}

		// find out how much data we can match: the minimum of len(p) and the
		// remainder of the prefix.
		min := len(m.prefix[m.matching:])
		if len(p[n:]) < min {
			min = len(p[n:])
		}

		// try to match the next part of the prefix
		if bytes.HasPrefix(p[n:], m.prefix[m.matching:m.matching+min]) {
			// send any data that we've not sent prior to our match to the
			// next writer.
			_, err = m.next.Write(p[last:n])
			if err != nil {
				return n, err
			}

			m.matching += min
			n += min
			last = n

			if m.matching == len(m.prefix) {
				_, err := m.next.Write(m.prefix[:m.matching])
				if err != nil {
					return n, err
				}
			}

			continue
		}

		// if we didn't complete a prefix match, send the tracked bytes of
		// the prefix to the next writer unmodified.
		if m.matching > 0 {
			_, err = m.next.Write(m.prefix[:m.matching])
			if err != nil {
				return n, err
			}

			// if the end of this prefix matches the start of it, try again
			if m.prefix[0] == p[n] {
				m.matching = 1
				last++
				n++
				continue
			}
		}
		m.matching = 0

		n++
	}

	// any unmatched data is sent to the next writer
	_, err = m.next.Write(p[last:n])

	return n, err
}

// Close flushes any remaining data and closes the underlying writer.
func (m *tokenSanitizer) Close() error {
	var werr error

	if m.masked {
		// When a valid is located at the end of the whole packet,
		// we leave the Write function without actually writing the mask byte
		// not revealing any part of the token but not accurately masking it either.
		// This condition places in the Close function allows us to catch this scenario
		_, werr = m.next.Write(mask)
	} else {
		_, werr = m.next.Write(m.prefix[:m.matching])
	}

	err := m.next.Close()
	if err == nil {
		return werr
	}

	return err
}
