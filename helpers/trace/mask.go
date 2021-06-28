package trace

import (
	"bytes"

	"golang.org/x/text/transform"
)

const (
	// mask is the string that replaces any found sensitive information
	mask = "[MASKED]"

	// safeTokens are tokens that cannot appear in secret phrases
	// and allows calling writers to set a safe boundary at which data written
	// isn't buffered before being flushed to the underlying writer.
	safeTokens = "\r\n"

	// maxPhraseSize is the maximum sized phrase supported. This should be equal
	// to text/transform's internal buffer.
	// https://cs.opensource.google/go/x/text/+/refs/tags/v0.3.6:transform/transform.go;l=130
	maxPhraseSize = 4096
)

// newPhraseTransform returns a transform.Transformer that replaces the `phrase`
// with [MASKED]
func newPhraseTransform(phrase string) transform.Transformer {
	return phraseTransform(phrase)
}

type phraseTransform []byte

func (phraseTransform) Reset() {}

type matchType int

const (
	noPossibleMatch matchType = iota
	partialMatch
	fullMatch
)

//nolint:gocognit
func (t phraseTransform) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	for {
		match, index := find(src[nSrc:], t)
		// copy up to the index where we either found or skipped data
		err = copyn(dst, src, &nDst, &nSrc, index)
		if err != nil {
			return nDst, nSrc, err
		}

		if match == fullMatch {
			// replace phrase
			err = replace(dst, &nDst, &nSrc, []byte(mask), len(t))
			if err != nil {
				return nDst, nSrc, err
			}
			continue
		}

		// If we matched at the beginning of the src buffer and the src and token are larger
		// or equal to maxPhraseSize it means our buffer is entirely full of t[:maxPhraseSize].
		//
		// At this point, we mask data we have, but future writes will reveal the tail of any secret.
		// This is done because we cannot match beyond maxPhraseSize without filling the internal
		// text/transform buffer and returning an error.
		if match == partialMatch && index == 0 && nSrc == 0 && len(src) >= maxPhraseSize && len(t) >= maxPhraseSize {
			err = replace(dst, &nDst, &nSrc, []byte(mask), maxPhraseSize)
			if err != nil {
				return nDst, nSrc, err
			}
		}

		break
	}

	return safecopy(dst, src, atEOF, nDst, nSrc, len(t))
}

func find(src []byte, phrase []byte) (matchType, int) {
	n := 0
	for {
		i := bytes.IndexByte(src[n:], phrase[0])
		if i == -1 {
			return noPossibleMatch, len(src)
		}

		remaining := len(src[n+i:])
		if remaining > len(phrase) {
			remaining = len(phrase)
		}

		if bytes.Equal(src[n+i:n+i+remaining], phrase[:remaining]) {
			if remaining == len(phrase) {
				return fullMatch, n + i
			}
			return partialMatch, n + i
		}

		n += i + 1
	}
}

// replace copies a replacement into the dst buffer and advances nDst and nSrc.
func replace(dst []byte, nDst, nSrc *int, replacement []byte, advance int) error {
	if len(dst[*nDst:]) < len(replacement) {
		return transform.ErrShortDst
	}

	n := copy(dst[*nDst:], replacement)
	*nDst += n
	*nSrc += advance

	return nil
}

// copy copies data from src to dst for length n and advances nDst and nSrc.
func copyn(dst, src []byte, nDst, nSrc *int, n int) error {
	copied := copy(dst[*nDst:], src[*nSrc:*nSrc+n])
	*nDst += copied
	*nSrc += copied
	if copied < n {
		return transform.ErrShortDst
	}

	return nil
}

// safecopy copies the remaining data minus that of the token size, preventing
// the accidental copy of the beginning of a token that should be replaced. If
// atEOF is true, the full remaining data is copied.
func safecopy(dst, src []byte, atEOF bool, nDst, nSrc int, tokenSize int) (int, int, error) {
	var err error

	remaining := len(src[nSrc:])
	if !atEOF {
		// copy either:
		// - up until the last safe token if any, or
		// - up to our data length minus tokenSize,
		// whichever has the higher index position.
		idx := bytes.LastIndexAny(src[nSrc:], safeTokens)
		remaining -= tokenSize
		if idx+1 > remaining {
			remaining = idx + 1
		}

		err = transform.ErrShortSrc
	}

	if remaining > 0 {
		err := copyn(dst, src, &nDst, &nSrc, remaining)
		if err != nil {
			return nDst, nSrc, err
		}
	}

	return nDst, nSrc, err
}
