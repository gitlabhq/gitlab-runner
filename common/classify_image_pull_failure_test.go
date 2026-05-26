//go:build !integration

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

func TestClassifyImagePullFailure(t *testing.T) {
	tests := map[string]struct {
		msg            string
		expectedReason spec.JobFailureReason
	}{
		"dial tcp - no such host": {
			msg:            `Get "https://registry-1.docker.io/v2/": dial tcp: lookup registry-1.docker.io: no such host`,
			expectedReason: RunnerExternalDependencyFailure,
		},
		"dial tcp - connection refused": {
			msg:            `Get "https://registry.example.com/v2/": dial tcp 1.2.3.4:443: connect: connection refused`,
			expectedReason: RunnerExternalDependencyFailure,
		},
		"no such host": {
			msg:            `dial tcp: lookup private.registry.local: no such host`,
			expectedReason: RunnerExternalDependencyFailure,
		},
		"i/o timeout": {
			msg:            `Get "https://registry.example.com/v2/": i/o timeout`,
			expectedReason: RunnerExternalDependencyFailure,
		},
		"tls handshake timeout": {
			msg:            `Get "https://registry.example.com/v2/": tls handshake timeout`,
			expectedReason: RunnerExternalDependencyFailure,
		},
		"manifest not found": {
			msg:            `manifest for nginx:nonexistent not found: manifest unknown: manifest unknown`,
			expectedReason: ConfigurationError,
		},
		"manifest unknown": {
			msg:            `manifest unknown: manifest unknown`,
			expectedReason: ConfigurationError,
		},
		"auth failure": {
			msg:            `pull access denied for private/image, repository does not exist or may require 'docker login'`,
			expectedReason: ImagePullFailure,
		},
		"generic error": {
			msg:            `this is a test`,
			expectedReason: ImagePullFailure,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.expectedReason, ClassifyImagePullFailure(tt.msg))
		})
	}
}
