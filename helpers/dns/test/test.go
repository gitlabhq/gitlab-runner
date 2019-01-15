package test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func AssertRFC1123Compatibility(t *testing.T, name string) {
	dns1123MaxLength := 63
	dns1123FormatRegexp := regexp.MustCompile("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$")

	assert.True(t, len(name) <= dns1123MaxLength, "Name length needs to be shorter than %d", dns1123MaxLength)
	assert.Regexp(t, dns1123FormatRegexp, name, "Name needs to be in RFC-1123 allowed format")
}
