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
//   - Access denied / authentication required (missing credentials, expired
//     tokens, no access to a private image, wrong image path, etc.) →
//     ConfigurationError
//   - Everything else → ImagePullFailure
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

	// Authentication / access-denied errors are job/config problems: the job
	// provided no credentials, provided ones that are invalid/expired, or
	// referenced a private image it has no access to. A registry-side
	// authentication hiccup that surfaces as 401 here is misattributed to
	// config in the rare case it occurs, but those incidents are detected
	// separately by the registry's own monitoring.
	case strings.Contains(lower, "pull access denied"),
		strings.Contains(lower, "access to the resource is denied"),
		strings.Contains(lower, "no basic auth credentials"),
		strings.Contains(lower, "repository does not exist or may require"),
		strings.Contains(lower, "unauthorized"),
		strings.Contains(lower, "authentication required"):
		return ConfigurationError

	default:
		return ImagePullFailure
	}
}
