package dns

import (
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
		{name: "very-long-token-----------------------------------------------end", expected: "very-long-token-----------------------------------------------e"},
	}

	for _, example := range examples {
		t.Run(example.name, func(t *testing.T) {
			name := MakeRFC1123Compatible(example.name)

			assert.Equal(t, example.expected, name)
			test.AssertRFC1123Compatibility(t, name)
		})
	}
}
