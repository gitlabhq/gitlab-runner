package trace

import (
	"bytes"
	"unicode"

	"golang.org/x/text/transform"
)

const (
	// mask is the string that replaces any found sensitive information
	mask = "[MASKED]"

	// safeTokens are tokens that cannot appear in secret phrases or URL tokens
	// and allows calling writers to set a safe boundary at which data written
	// isn't buffered before being flushed to the underlying writer.
	safeTokens = "\r\n"

	// maxPhraseSize is the maximum sized phrase supported. This should be equal
	// to text/transform's internal buffer.
	// https://cs.opensource.google/go/x/text/+/refs/tags/v0.3.6:transform/transform.go;l=130
	maxPhraseSize = 4096
)

// sensitiveURLTokens are the param tokens we search for and replace the
// values of to [MASKED].
var sensitiveURLTokens = [][]byte{
	// 20 characters, used for authenticating to GitLab
	[]byte("private_token"),
	// ~88 characters, a base64 encoded string of random 64 bytes
	[]byte("authenticity_token"),
	// 20 characters. RSS feed token. Unlikely to appear in a build log, but here for backwards compatibility.
	[]byte("rss_token"),
	// 64 characters, Amazon presigned signature hex encoded sha256 hmac
	[]byte("x-amz-signature"),
	// Amazon presigned URL credential is always in the format of
	// <access-key>/<date>/<region>/<service>/aws4_request.
	[]byte("x-amz-credential"),
	// Amazon temporary security token from STS.
	[]byte("x-amz-security-token"),
}

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

// newSensitiveURLParamTransform returns a transform.Transformer that replaces common
// sensitive param values with [MASKED]
func newSensitiveURLParamTransform() transform.Transformer {
	t := &sensitiveURLParamTransform{
		keys: sensitiveURLTokens,
	}

	for _, key := range t.keys {
		if len(key) > t.maxKeySize {
			t.maxKeySize = len(key)
		}
	}

	return t
}

type sensitiveURLParamTransform struct {
	keys       [][]byte
	maxKeySize int
	mask       bool
}

func (t *sensitiveURLParamTransform) Reset() {
	t.mask = false
}

func (t *sensitiveURLParamTransform) hasSensitiveParam(query []byte) bool {
	for _, param := range t.keys {
		if bytes.EqualFold(query, param) {
			return true
		}
	}
	return false
}

//nolint:gocognit
func (t *sensitiveURLParamTransform) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	for {
		// if masking, consume everything until we find the end
		if t.mask {
			end := t.indexOfParamEnd(src[nSrc:], atEOF)
			if end == -1 {
				return nDst, len(src), err
			}

			nSrc += end
			t.mask = false

			continue
		}

		// if not masking, find the start of a url parameter (? or &)
		idx := bytes.IndexAny(src[nSrc:], "?&")
		if idx == -1 {
			// if nothing found, copy until end of data
			err := copyn(dst, src, &nDst, &nSrc, len(src[nSrc:]))
			return nDst, nSrc, err
		}
		idx++

		// index of key end
		keyIndex := bytes.IndexAny(src[nSrc+idx:], "=")
		if keyIndex == -1 {
			break
		}

		hasKey := t.hasSensitiveParam(src[nSrc+idx : nSrc+idx+keyIndex])
		if hasKey {
			idx += keyIndex + 1
		}

		// depending on whether a sensitive key has been found, progress
		// by copying either to the start or end of the key.
		err = copyn(dst, src, &nDst, &nSrc, idx)
		if err != nil {
			return nDst, nSrc, err
		}

		if hasKey {
			// insert masked text
			err = replace(dst, &nDst, &nSrc, []byte(mask), 0)
			if err != nil {
				return nDst, nSrc, err
			}

			t.mask = true
		}
	}

	return safecopy(dst, src, atEOF, nDst, nSrc, t.maxKeySize)
}

func (t sensitiveURLParamTransform) indexOfParamEnd(src []byte, atEOF bool) int {
	isParamEnd := func(r rune) bool {
		// URL parameters cannot include certain characters without percent encoding them
		// but it's pointless following the actual spec, because nobody else does.
		//
		// Using the most common reserved and special characters we know wouldn't
		// be present in a URL param value is good enough:
		return r == '?' || r == '&' || unicode.IsSpace(r) || unicode.IsControl(r)
	}

	end := bytes.IndexFunc(src, isParamEnd)
	if end == -1 {
		if !atEOF {
			return -1
		}

		// if we're atEOF, everything is the value
		end = len(src)
	}

	return end
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
