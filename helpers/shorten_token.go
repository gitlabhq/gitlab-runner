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

// instanceTokenPrefixPattern matches the optional instance token prefix GitLab can prepend to glrt- and glcbt-
// tokens, e.g. "acme-" in "acme-glrt-...". GitLab only allows alphanumeric instance prefixes.
const instanceTokenPrefixPattern = `([A-Za-z0-9]+-)?`

const createdRunnerTokenPrefix = "glrt-"

var prefixRes = []*regexp.Regexp{
	regexp.MustCompile(`^` + instanceTokenPrefixPattern + regexp.QuoteMeta(createdRunnerTokenPrefix) + `(t[123]_)?|^t[123]_|^glrtr-`), // runner authentication token
	regexp.MustCompile(`^` + instanceTokenPrefixPattern + `glcbt-`),                                                                   // job token
	regexp.MustCompile(`^GR[0-9A-Fa-f]{7}`),                                                                                           // runner registration token. These should no longer appear, but just in case...
}

var createdRunnerTokenRe = regexp.MustCompile(`^` + instanceTokenPrefixPattern + regexp.QuoteMeta(createdRunnerTokenPrefix))

func IsCreatedRunnerToken(token string) bool {
	return createdRunnerTokenRe.MatchString(token)
}

const shortTokenLen = 9

func ShortenToken(in string) string {
	// Strip known prefixes
	for _, re := range prefixRes {
		in = re.ReplaceAllString(in, "")
	}

	// take the first 9 characters
	end := math.Min(shortTokenLen, float64(len(in)))
	result := in[:int(end)]

	// Kubernetes label values must start and end with [A-Za-z0-9].
	// Fix the leading character: prepend "r" and drop the last character so
	// the length is preserved and the original leading character is retained
	// at position 1 (avoiding collisions between tokens differing only in
	// the first character).
	if result != "" && !isAlphaNum(result[0]) {
		result = "r" + result[:len(result)-1]
	}

	// Fix the trailing character: replace a non-alphanumeric last character
	// with "r" to satisfy the Kubernetes label value constraint.
	if result != "" && !isAlphaNum(result[len(result)-1]) {
		result = result[:len(result)-1] + "r"
	}

	return result
}

// isAlphaNum reports whether c is an ASCII letter or digit.
func isAlphaNum(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}
