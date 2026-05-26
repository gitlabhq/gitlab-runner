package common

import (
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

// ClassifyImagePullFailure inspects an error or status message from an image
// pull (Docker daemon, Kubernetes container waiting reason, etc.) and returns
// the most specific failure reason possible.
//
//   - Network-level failures (DNS, TCP, TLS, I/O timeout) → RunnerExternalDependencyFailure
//   - Image or tag does not exist → ConfigurationError
//   - Everything else (auth, access denied, …) → ImagePullFailure
func ClassifyImagePullFailure(msg string) spec.JobFailureReason {
	lower := strings.ToLower(msg)

	switch {
	case strings.Contains(lower, "dial tcp"),
		strings.Contains(lower, "connection refused"),
		strings.Contains(lower, "no such host"),
		strings.Contains(lower, "i/o timeout"),
		strings.Contains(lower, "tls handshake"):
		return RunnerExternalDependencyFailure

	case strings.Contains(lower, "not found"),
		strings.Contains(lower, "manifest unknown"):
		return ConfigurationError

	default:
		return ImagePullFailure
	}
}
