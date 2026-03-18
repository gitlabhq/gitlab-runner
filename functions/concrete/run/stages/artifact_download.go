package stages

import (
	"context"
	"fmt"
	"strconv"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
)

type ArtifactDownload struct {
	ID               int64  `json:"id,omitempty"`
	Token            string `json:"token,omitempty"`
	ArtifactName     string `json:"artifact_name,omitempty"`
	Filename         string `json:"filename,omitempty"`
	DownloadAttempts int    `json:"download_attempts,omitempty"`
	Concurrency      int    `json:"concurrency,omitempty"` // unused for now, because artifacts-download uses env vars directly
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

	attempts := s.DownloadAttempts
	if attempts < 1 {
		attempts = 1
	}

	var err error
	for i := 1; i <= attempts; i++ {
		if i > 1 {
			e.Warningf("Retrying artifact download for %s (attempt %d/%d)...", s.ArtifactName, i, attempts)
		} else {
			e.Noticef("Downloading artifacts for %s (%d)...", s.ArtifactName, s.ID)
		}

		err = e.RunnerCommand(ctx, nil, args...)
		if err == nil {
			return nil
		}
	}

	return fmt.Errorf("downloading artifacts for %s (%d): %w", s.ArtifactName, s.ID, err)
}
