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
)

// newPhraseTransform returns a transform.Transformer that replaces the `phrase`
// with [MASKED]
func newPhraseTransform(phrase string) transform.Transformer {
	return phraseTransform(phrase)
}

type phraseTransform []byte

func (phraseTransform) Reset() {}

func (t phraseTransform) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	for {
		// copy up until phrase
		i := bytes.Index(src[nSrc:], t)
		if i == -1 {
			break
		}

		err = copyn(dst, src, &nDst, &nSrc, i)
		if err != nil {
			return nDst, nSrc, err
		}

		// replace phrase
		err = replace(dst, &nDst, &nSrc, []byte(mask), len(t))
		if err != nil {
			return nDst, nSrc, err
		}
	}

	return safecopy(dst, src, atEOF, nDst, nSrc, len(t))
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
