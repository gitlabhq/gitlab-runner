package dns

import (
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
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

const emptyRFC1123SubdomainErrorMessage = "validating rfc1123 subdomain"

type RFC1123SubdomainError struct {
	errs []string
}

func (d *RFC1123SubdomainError) Error() string {
	if len(d.errs) == 0 {
		return emptyRFC1123SubdomainErrorMessage
	}

	return strings.Join(d.errs, ", ")
}

func (d *RFC1123SubdomainError) Is(err error) bool {
	_, ok := err.(*RFC1123SubdomainError)
	return ok
}

func ValidateDNS1123Subdomain(name string) error {
	errs := validation.IsDNS1123Subdomain(name)
	if len(errs) == 0 {
		return nil
	}

	return &RFC1123SubdomainError{errs: errs}
}
