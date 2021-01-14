package trace

import (
	"bytes"

	"golang.org/x/text/transform"
)

const (
	// mask is the string that replaces any found sensitive information
	mask = "[MASKED]"

	// safeTokens are tokens that cannot appear in secret phrases or URL tokens
	// and allows calling writers to set a safe boundary at which data written
	// isn't buffered before being flushed to the underlying writer.
	safeTokens = "\r\n"

	// sensitiveURLMaxTokenSize is the max token size we consider for sensitive
	// URL param values. This prevents missing tokens that appear on a boundary
	// and ensures we always have sufficient data. Param values can get quite
	// long, but this should be long enough to cover all cases defined in
	// `sensitiveURLTokens`.
	sensitiveURLMaxTokenSize = 255
)

var (
	// sensitiveURLTokens are the param tokens we search for and replace the
	// values of to [MASKED].
	sensitiveURLTokens = [][]byte{
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
	}
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

// NewSensitiveURLParamTransform returns a transform.Transformer that replaces common
// sensitive param values with [MASKED]
func NewSensitiveURLParamTransform() transform.Transformer {
	return sensitiveURLParamTransform(sensitiveURLTokens)
}

type sensitiveURLParamTransform [][]byte

func (sensitiveURLParamTransform) Reset() {}

func (t sensitiveURLParamTransform) hasSensitiveParam(query []byte) bool {
	for _, param := range t {
		if bytes.EqualFold(query, param) {
			return true
		}
	}
	return false
}

//nolint:gocognit
func (t sensitiveURLParamTransform) Transform(dst, src []byte, atEOF bool) (nDst, nSrc int, err error) {
	for {
		// copy up until ? or &, the start of a url parameter
		idx := bytes.IndexAny(src[nSrc:], "?&")
		if idx == -1 {
			break
		}

		// index of key end
		var key []byte
		keyIndex := bytes.IndexAny(src[nSrc+idx:], "=")
		if keyIndex == -1 {
			break
		}

		// extract key
		key = src[nSrc+idx : nSrc+idx+keyIndex]
		idx += keyIndex

		// index of value end
		end := t.indexOfParamEnd(src[nSrc+idx:], atEOF)
		if end == -1 {
			break
		}

		// copy everything up until the value
		err := copyn(dst, src, &nDst, &nSrc, idx)
		if err != nil {
			return nDst, nSrc, err
		}

		// rewrite value if key is one we want to rewrite
		value := src[nSrc : nSrc+end]
		if t.hasSensitiveParam(key[1:]) {
			value = []byte("=" + mask)
		}

		// replace value
		err = replace(dst, &nDst, &nSrc, value, end)
		if err != nil {
			return nDst, nSrc, err
		}
	}

	return safecopy(dst, src, atEOF, nDst, nSrc, sensitiveURLMaxTokenSize)
}

func (t sensitiveURLParamTransform) indexOfParamEnd(src []byte, atEOF bool) int {
	end := bytes.IndexAny(src, "& ")
	if end == -1 {
		if !atEOF {
			return -1
		}

		// if we're atEOF, everything is the value
		end = len(src)
	}

	return end
}

// replace copies a replacement into the dst buffer and advances nDst and nSrc.
func replace(dst []byte, nDst, nSrc *int, replacement []byte, advance int) error {
	n := copy(dst[*nDst:], replacement)
	*nDst += n
	if n < len(replacement) {
		return transform.ErrShortDst
	}
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
		// copy up to the last safe token if any, otherwise don't copy
		// until we have a buffer of at least tokenSize.
		idx := bytes.LastIndexAny(src[nSrc:], safeTokens)
		if idx < 0 {
			remaining -= tokenSize + 1
		} else {
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
