package store

import (
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type jobFilter func(*common.Job) bool

func newCanResumeJobFilter(cfg *common.StoreConfig) jobFilter {
	return func(job *common.Job) bool {
		return (job.State.GetBuildState() == common.BuildRunRuntimeRunning || job.State.GetBuildState() == common.BuildRunStatePending) &&
			time.Since(job.State.GetHealthCheckAt()) > cfg.GetHealthTimeout() &&
			job.State.GetRetries() < cfg.GetMaxRetries()
	}
}

func newCanDeleteJobFilter(cfg *common.StoreConfig) jobFilter {
	return func(job *common.Job) bool {
		buildState := job.State.GetBuildState()

		isFailedState := buildState == common.BuildRunRuntimeCanceled ||
			buildState == common.BuildRunRuntimeFailed ||
			buildState == common.BuildRunRuntimeTerminated ||
			buildState == common.BuildRunRuntimeTimedout
		isStale := time.Since(job.State.GetHealthCheckAt()) > cfg.GetStaleTimeout()
		isOverRetryLimit := time.Since(job.State.GetHealthCheckAt()) > cfg.GetHealthTimeout() &&
			job.State.GetRetries() >= cfg.GetMaxRetries()

		return isFailedState || isStale || isOverRetryLimit
	}
}
