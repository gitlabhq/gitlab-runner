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
		"http client timeout awaiting headers": {
			msg:            `Get "https://gitlab.com/jwt/auth?...": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`,
			expectedReason: RunnerExternalDependencyFailure,
		},
		"bare context deadline exceeded is not transient": {
			msg:            `Head "https://registry.gitlab.com/v2/...": context deadline exceeded`,
			expectedReason: ImagePullFailure,
		},
		"request canceled while waiting for connection": {
			msg:            `Get "https://registry.example.com/v2/": net/http: request canceled while waiting for connection`,
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
		"pull access denied": {
			msg:            `pull access denied for private/image, repository does not exist or may require 'docker login'`,
			expectedReason: ConfigurationError,
		},
		"daemon pull access denied with denied suffix": {
			msg:            `Error response from daemon: pull access denied for appvance/nike, repository does not exist or may require 'docker login': denied: requested access to the resource is denied`,
			expectedReason: ConfigurationError,
		},
		"gitlab registry pull access denied": {
			msg:            `failed to pull image "registry.gitlab.com/group/project/image:latest" with specified policies [always]: Error response from daemon: pull access denied for registry.gitlab.com/group/project/image, repository does not exist or may require 'docker login': denied: requested access to the resource is denied`,
			expectedReason: ConfigurationError,
		},
		"no basic auth credentials": {
			msg:            `Get "https://registry.example.com/v2/": no basic auth credentials`,
			expectedReason: ConfigurationError,
		},
		"pull access denied only": {
			msg:            `pull access denied for some/image`,
			expectedReason: ConfigurationError,
		},
		"access to the resource is denied only": {
			msg:            `denied: requested access to the resource is denied`,
			expectedReason: ConfigurationError,
		},
		"repository does not exist or may require only": {
			msg:            `repository does not exist or may require authorization`,
			expectedReason: ConfigurationError,
		},
		"unauthorized authentication required": {
			msg:            `Error response from daemon: unauthorized: authentication required`,
			expectedReason: ConfigurationError,
		},
		"unauthorized only": {
			msg:            `Get "https://registry.example.com/v2/": 401 unauthorized`,
			expectedReason: ConfigurationError,
		},
		"authentication required only": {
			msg:            `registry returned: authentication required to access this resource`,
			expectedReason: ConfigurationError,
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
