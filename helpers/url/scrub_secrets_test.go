//go:build !integration

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
		{
			input:  "Get http://localhost/?id=123&X-Amz-Signature=abcd1234&private_token=abcd1234",
			output: "Get http://localhost/?id=123&X-Amz-Signature=[FILTERED]&private_token=[FILTERED]",
		},
		{
			input:  "Get http://localhost/?id=123&X-Amz-Security-Token=abcd1234&private_token=abcd1234",
			output: "Get http://localhost/?id=123&X-Amz-Security-Token=[FILTERED]&private_token=[FILTERED]",
		},
		{
			input: "Get http://localhost/?id=123&" +
				"X-Amz-Credential=ABCDEF123456%2F20180920%2Fus-east-1%2Fs3%2Faws4_request",
			output: "Get http://localhost/?id=123&X-Amz-Credential=[FILTERED]",
		},
		{
			input:  "Get http://localhost/?private_token=abcd1234 test",
			output: "Get http://localhost/?private_token=[FILTERED] test",
		},
	}

	for _, example := range examples {
		assert.Equal(t, example.output, ScrubSecrets(example.input))
	}
}
