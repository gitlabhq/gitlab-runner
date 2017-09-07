package url_helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScrubSecrets(t *testing.T) {
	examples := []struct {
		input  string
		output string
	}{
		{input: "Get http://localhost/?id=123", output: "Get http://localhost/?id=123"},
		{input: "Get http://localhost/?id=123&X-Amz-Signature=abcd1234&private_token=abcd1234", output: "Get http://localhost/?id=123&X-Amz-Signature=[FILTERED]&private_token=[FILTERED]"},
	}

	for _, example := range examples {
		assert.Equal(t, example.output, ScrubSecrets(example.input))
	}
}
