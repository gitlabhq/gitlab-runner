package stages

import (
	"context"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
)

type ArtifactDownload struct {
	ID               int64  `json:"id,omitempty"`
	Token            string `json:"token,omitempty"`
	ArtifactName     string `json:"artifact_name,omitempty"`
	Filename         string `json:"filename,omitempty"`
	DownloadAttempts int    `json:"download_attempts,omitempty"`
	Concurrency      int    `json:"concurrency,omitempty"`
}

func (s ArtifactDownload) Run(ctx context.Context, e *env.Env) error {
	// todo: impl
	return nil
}
