package stages

import (
	"context"
	"fmt"
	"strconv"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/stages/internal/retry"
)

type ArtifactDownload struct {
	ID               int64  `json:"id,omitempty"`
	Token            string `json:"token,omitempty"`
	ArtifactName     string `json:"artifact_name,omitempty"`
	Filename         string `json:"filename,omitempty"`
	DownloadAttempts int    `json:"download_attempts,omitempty"`
	Concurrency      int    `json:"concurrency,omitempty"` // unused for now, because artifacts-download uses env vars directly
	// UseExponentialBackoffStageRetry mirrors
	// FF_USE_EXPONENTIAL_BACKOFF_STAGE_RETRY. See GetSources for
	// detail; the same schedule applies to artifact download retries.
	UseExponentialBackoffStageRetry bool `json:"use_exponential_backoff_stage_retry,omitempty"`
}

func (s ArtifactDownload) Run(ctx context.Context, e *env.Env) error {
	if s.Filename == "" {
		e.Debugf("Skipping artifact download for %s (%d): no filename", s.ArtifactName, s.ID)
		return nil
	}

	args := []string{
		"artifacts-downloader",
		"--url", e.BaseURL,
		"--token", s.Token,
		"--id", strconv.FormatInt(s.ID, 10),
	}

	attempts := max(s.DownloadAttempts, 1)

	backoff := retry.NewBackoff()
	var err error
	for i := 1; i <= attempts; i++ {
		if i > 1 {
			if s.UseExponentialBackoffStageRetry {
				retry.SleepWithNotice(e, backoff.Duration())
			}
			e.Warningf("Retrying artifact download for %s (attempt %d/%d)...", s.ArtifactName, i, attempts)
		} else {
			e.Noticef("Downloading artifacts for %s (%d)...", s.ArtifactName, s.ID)
		}

		err = e.RunnerCommand(ctx, e.HelperEnvs(nil), args...)
		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("downloading artifacts for %s (%d): %w", s.ArtifactName, s.ID, err)
}
