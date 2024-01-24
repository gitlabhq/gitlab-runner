// Package urlsanitizer replaces sensitive parameter values with [MASKED].
//
// This is achieved by extracting keys in the format of ?key= or &key= and if
// the key is deemed sensitive, consumes the value that follows it.
package urlsanitizer

import (
	"bytes"
	"io"
	"strings"
	"unicode"
)

// tokenParamKeys are the param keys for sensitive tokens we sanitize (replace
// with [MASKED]).
var tokenParamKeys = map[string]struct{}{
	// 20 characters, used for authenticating to GitLab
	"private_token": {},
	// ~88 characters, a base64 encoded string of random 64 bytes
	"authenticity_token": {},
	// 20 characters. RSS feed token. Unlikely to appear in a build log, but here for backwards compatibility.
	"rss_token": {},
	// 64 characters, Amazon presigned signature hex encoded sha256 hmac
	"x-amz-signature": {},
	// Amazon presigned URL credential is always in the format of
	// <access-key>/<date>/<region>/<service>/aws4_request.
	"x-amz-credential": {},
	// Amazon temporary security token from STS.
	"x-amz-security-token": {},
}

var mask = []byte("[MASKED]")

type URLSanitizer struct {
	w       io.WriteCloser
	match   []byte
	masking bool
}

// New returns a new URL Sanitizer.
func New(w io.WriteCloser) *URLSanitizer {
	var max int
	for token := range tokenParamKeys {
		if len(token) > max {
			max = len(token) + 1
		}
	}

	return &URLSanitizer{w: w, match: make([]byte, 0, max)}
}

//nolint:funlen,gocognit
func (s *URLSanitizer) Write(p []byte) (n int, err error) {
	var last int

	for n < len(p) {
		// if we're in masking mode, we throw away all bytes until we find
		// the end of the parameter we're masking.
		if s.masking {
			off := bytes.IndexFunc(p[n:], isParamEnd)
			if off == -1 {
				// no end found, so skip these bytes
				n += len(p[n:])
				last = n
				break
			} else {
				// end found, so skip the bytes up until the match and write
				// [MASKED] in their place.
				n += off
				last += off
				s.masking = false

				_, err = s.w.Write(mask)
				if err != nil {
					return n, err
				}
			}
		}

		// if our match is at capacity (maximum token size), reset it and
		// continue looking for the next token.
		if len(s.match) == cap(s.match) {
			s.match = s.match[:0]
		}

		// fast path: if we're not matching any parameters, skip towards ? or &
		// if none found, we can bail early
		if len(s.match) == 0 {
			off := bytes.IndexAny(p[n:], "?&")
			if off == -1 {
				n += len(p[n:])
				break
			} else {
				s.match = append(s.match, p[n+off])
				n += off + 1
			}
		}

		// all of p consumed, so break
		if n >= len(p) {
			break
		}

		// find any of key name
		off := bytes.IndexAny(p[n:], "=?&")

		// if not found, continue adding to key match
		if off == -1 {
			s.match = append(s.match, p[n])
			n++
			continue
		}

		// bail early if the key contains another param separator
		if p[n+off] == '?' || p[n+off] == '&' {
			s.match = s.match[:0]
			n += off
			continue
		}

		// bail early if key would exceed our known key sizes
		if off+len(s.match) > cap(s.match) {
			s.match = s.match[:0]
			n++
			continue
		}

		key := append(s.match, p[n:n+off]...)
		n += off + 1

		// check if the key is one supported, and if so, write data until this
		// point and move to masking mode
		if _, ok := tokenParamKeys[strings.ToLower(string(key[1:]))]; ok {
			_, err = s.w.Write(p[last:n])
			if err != nil {
				return n, err
			}

			last = n
			s.masking = true
		}

		// reset match
		s.match = s.match[:0]
	}

	if len(p[last:n]) > 0 {
		_, err = s.w.Write(p[last:n])
	}

	return n, err
}

// Close flushes any remaining data and closes the underlying writer.
func (s *URLSanitizer) Close() error {
	var werr error
	if s.masking {
		_, werr = s.w.Write(mask)
	}

	err := s.w.Close()
	if err == nil {
		return werr
	}
	return err
}

func isParamEnd(r rune) bool {
	// URL parameters cannot include certain characters without percent encoding them
	// but it's pointless following the actual spec, because nobody else does.
	//
	// Using the most common reserved and special characters we know wouldn't
	// be present in a URL param value is good enough:
	return r == '?' || r == '&' || unicode.IsSpace(r) || unicode.IsControl(r)
}
