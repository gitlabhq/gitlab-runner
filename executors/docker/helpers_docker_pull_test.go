package docker

import (
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/pull"
)

// SetPullRetryConfigForTesting overrides the image-pull retry attempt count
// and jittered backoff window used by every pull manager created for the
// rest of the test, restoring the production defaults via the returned
// func. It exists purely as a test seam for real-daemon integration tests
// (see TestDockerCommandPullRetriesTransientRegistryFailure) that need the
// retry loop to run on a much shorter, more forgiving timing budget than
// production uses, instead of racing the real 2-10s jittered backoff
// window.
//
// It mutates the package-level createPullManager var without
// synchronization, so it must not be called from a test running in
// parallel with another that reads or writes createPullManager.
func SetPullRetryConfigForTesting(maxAttempts int, minBackoff, maxBackoff time.Duration) (restore func()) {
	orig := createPullManager
	createPullManager = func(e *executor) (pull.Manager, error) {
		pullManager := pull.NewManager(e.Context, &e.BuildLogger, newPullManagerConfig(e), e.dockerConn, func() {
			e.SetCurrentStage(ExecutorStagePullingImage)
		}, pull.WithPullRetryConfig(maxAttempts, minBackoff, maxBackoff))

		return pullManager, nil
	}

	return func() { createPullManager = orig }
}
