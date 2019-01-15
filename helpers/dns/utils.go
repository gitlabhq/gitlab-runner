package dns

import (
	"regexp"
	"strings"
)

const (
	RFC1123NameMaximumLength         = 63
	RFC1123NotAllowedCharacters      = "[^-a-z0-9]"
	RFC1123NotAllowedStartCharacters = "^[^a-z0-9]+"
)

func MakeRFC1123Compatible(name string) string {
	name = strings.ToLower(name)

	nameNotAllowedChars := regexp.MustCompile(RFC1123NotAllowedCharacters)
	name = nameNotAllowedChars.ReplaceAllString(name, "")

	nameNotAllowedStartChars := regexp.MustCompile(RFC1123NotAllowedStartCharacters)
	name = nameNotAllowedStartChars.ReplaceAllString(name, "")

	if len(name) > RFC1123NameMaximumLength {
		name = name[0:RFC1123NameMaximumLength]
	}

	return name
}
