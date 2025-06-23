package helpers

import (
	"math"
	"regexp"
)

// Known prefixes to strip from tokens:
// - glrt- and glrtr- are registration tokens
// - glcbt- is a ci job token
// - GR* is an old runner registration token
// - t[123]_ is a partition prefix which can appear with a glrt- registration token, or by itself.
//
// Any token prefixed added here should probably also be added to allTokenPrefixes in tokensanitizer package.

var prefixRes = []*regexp.Regexp{
	regexp.MustCompile(`^glrt-(t[123]_)?|^t[123]_|^glrtr-`), // runner authentication token
	regexp.MustCompile(`^glcbt-`),                           // job token
	regexp.MustCompile(`^GR[0-9A-Fa-f]{7}`),                 // runner registration token. These should no longer appear, but just in case...
}

const shortTokenLen = 9

func ShortenToken(in string) string {
	// Strip known prefixes
	for _, re := range prefixRes {
		in = re.ReplaceAllString(in, "")
	}

	// take the first 9 characters
	end := math.Min(shortTokenLen, float64(len(in)))
	return in[:int(end)]
}
