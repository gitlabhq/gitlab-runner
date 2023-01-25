// Package masker implements a masking Writer, where specified phrases are
// replaced with the word "[MASKED]".
//
// To achieve masking over Write() boundaries, each phrase has its own writer.
// These writers are stacked, with each one calling the next, in length order,
// starting with the longest. This allows each writer to scan for their phrase
// in-turn, filtering data down to the next writer as required.
//
// Each mask writer tracks when its phrase is being written, and counts until
// either it's matched all bytes of the phrase, and then replaces it, or if a
// full match isn't found, sends the matched bytes to the next writer
// unmodified.
package masker

import (
	"bytes"
	"io"
	"sort"
)

var mask = []byte("[MASKED]")

type Masker struct {
	next io.WriteCloser
}

// New returns a new Masker.
func New(w io.WriteCloser, phrases []string) *Masker {
	m := &Masker{}

	sort.Slice(phrases, func(i, j int) bool {
		return len(phrases[i]) < len(phrases[j])
	})

	m.next = w

	// Create a masker for each unique phrase
	unique := map[string]struct{}{}
	for i := 0; i < len(phrases); i++ {
		if phrases[i] == "" {
			continue
		}
		if _, ok := unique[phrases[i]]; ok {
			continue
		}
		unique[phrases[i]] = struct{}{}
		m.next = &masker{next: m.next, phrase: []byte(phrases[i])}
	}

	return m
}

func (m *Masker) Write(p []byte) (n int, err error) {
	return m.next.Write(p)
}

// Close flushes any remaining data and closes the underlying writer.
func (m *Masker) Close() error {
	return m.next.Close()
}

type masker struct {
	phrase   []byte
	matching int
	next     io.WriteCloser
}

//nolint:funlen,gocognit
func (m *masker) Write(p []byte) (n int, err error) {
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
		// optimization: use the faster IndexByte to jump to the start of a
		// potential phrase and if not found, advance the whole buffer.
		if m.matching == 0 {
			off := bytes.IndexByte(p[n:], m.phrase[0])
			if off < 0 {
				n += len(p[n:])
				break
			}
			if off > -1 {
				n += off
			}
		}

		// find out how much data we can match: the minimum of len(p) and the
		// remainder of the phrase.
		min := len(m.phrase[m.matching:])
		if len(p[n:]) < min {
			min = len(p[n:])
		}

		// try to match the next part of the phrase
		if bytes.HasPrefix(p[n:], m.phrase[m.matching:m.matching+min]) {
			// send any data that we've not sent prior to our match to the
			// next writer.
			_, err = m.next.Write(p[last:n])
			if err != nil {
				return n, err
			}

			m.matching += min
			n += min
			last = n

			// if we've tracked each byte of our phrase, we can replace it
			if m.matching == len(m.phrase) {
				_, err := m.Write(mask)
				if err != nil {
					return n, err
				}
				m.matching = 0
			}

			continue
		}

		// if we didn't complete a phrase match, send the tracked bytes of
		// the phrase to the next writer unmodified.
		if m.matching > 0 {
			_, err = m.next.Write(m.phrase[:m.matching])
			if err != nil {
				return n, err
			}

			// if the end of this phrase matches the start of it, try again
			if m.phrase[0] == p[n] {
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
func (m *masker) Close() error {
	var werr error

	if m.matching == len(m.phrase) {
		// this mask is added to avoid a potential undiscovered edge-case:
		// this should be unreachable as we replace full matches immediately in
		// Write().
		_, werr = m.next.Write(mask)
	} else {
		_, werr = m.next.Write(m.phrase[:m.matching])
	}

	err := m.next.Close()
	if err == nil {
		return werr
	}

	return err
}
