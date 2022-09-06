//go:build !integration

package log

import (
	"bytes"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func TestSecretsCleanupHook(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		expected string
	}{
		{
			name:     "With Secrets",
			message:  "Get http://localhost/?id=123&X-Amz-Signature=abcd1234&private_token=abcd1234",
			expected: "Get http://localhost/?id=123&X-Amz-Signature=[FILTERED]&private_token=[FILTERED]",
		},
		{
			name:     "No Secrets",
			message:  "Fatal: Get http://localhost/?id=123",
			expected: "Fatal: Get http://localhost/?id=123",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buffer := &bytes.Buffer{}

			logger := logrus.New()
			logger.Out = buffer
			AddSecretsCleanupLogHook(logger)

			logger.Errorln(test.message)

			assert.Contains(t, buffer.String(), test.expected)
		})
	}
}
