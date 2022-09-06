//go:build !integration

package dns

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/dns/test"
)

func TestMakeRFC1123Compatible(t *testing.T) {
	examples := []struct {
		name     string
		expected string
	}{
		{name: "tOk3_?ofTHE-Runner", expected: "tok3ofthe-runner"},
		{name: "----tOk3_?ofTHE-Runner", expected: "tok3ofthe-runner"},
		{
			name:     "very-long-token-----------------------------------------------end",
			expected: "very-long-token-----------------------------------------------e",
		},
	}

	for _, example := range examples {
		t.Run(example.name, func(t *testing.T) {
			name := MakeRFC1123Compatible(example.name)

			assert.Equal(t, example.expected, name)
			test.AssertRFC1123Compatibility(t, name)
		})
	}
}

func TestValidateDNS1123Subdomain(t *testing.T) {
	examples := []struct {
		name  string
		valid bool
	}{
		{name: "valid-dns", valid: true},
		{name: "1.1.1.1", valid: true},
		{name: "a.b.c", valid: true},
		{name: "c-1.p", valid: true},
		{name: "a---b", valid: true},

		{name: "__invalid", valid: false},
		{name: "long-" + strings.Repeat("a", 300), valid: false},
		{name: "A.B", valid: false},
		{name: "A.2---C", valid: false},
		{name: "A_B--C", valid: false},
	}

	for _, example := range examples {
		t.Run(example.name, func(t *testing.T) {
			err := ValidateDNS1123Subdomain(example.name)

			if example.valid {
				assert.NoError(t, err)
				return
			}

			assert.NotNil(t, err)
		})
	}

	// A separate test for empty subdomain value since otherwise it's rendered as
	// TestValidateDNS1123Subdomain/#00 which is less clear
	t.Run("empty", func(t *testing.T) {
		assert.NotNil(t, ValidateDNS1123Subdomain(""))
	})
}

func TestRFC1123SubdomainError(t *testing.T) {
	tests := map[string]struct {
		err *RFC1123SubdomainError

		expected string
	}{
		"one inner message": {
			err: &RFC1123SubdomainError{errs: []string{"one"}},

			expected: "one",
		},
		"two inner messages": {
			err: &RFC1123SubdomainError{errs: []string{"one", "two"}},

			expected: "one, two",
		},
		"empty inner err": {
			err:      &RFC1123SubdomainError{},
			expected: emptyRFC1123SubdomainErrorMessage,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestRFC1123SubdomainErrorIs(t *testing.T) {
	tests := map[string]struct {
		is error

		expected bool
	}{
		"is": {
			is: &RFC1123SubdomainError{},

			expected: true,
		},
		"is not": {
			is: errors.New("is not"),

			expected: false,
		},
		"is not - nil": {
			is: nil,

			expected: false,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			err := &RFC1123SubdomainError{}
			assert.Equal(t, tt.expected, err.Is(tt.is))
		})
	}
}
