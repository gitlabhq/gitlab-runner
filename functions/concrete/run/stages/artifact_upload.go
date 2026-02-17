package stages

import (
	"context"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
)

type ArtifactUpload struct {
	Untracked             bool              `json:"untracked,omitempty"`
	Paths                 []string          `json:"paths,omitempty"`
	Exclude               []string          `json:"exclude,omitempty"`
	ArtifactName          string            `json:"artifact_name,omitempty"`
	ExpireIn              string            `json:"expire_in,omitempty"`
	Format                string            `json:"format,omitempty"`
	Type                  string            `json:"type,omitempty"`
	CompressionLevel      string            `json:"compression_level,omitempty"`
	Timeout               time.Duration     `json:"timeout,omitempty"`
	ResponseHeaderTimeout time.Duration     `json:"response_header_timeout,omitempty"`
	OnSuccess             bool              `json:"on_success,omitempty"`
	OnFailure             bool              `json:"on_failure,omitempty"`
	Metadata              *ArtifactMetadata `json:"metadata,omitempty"`
}

type ArtifactMetadata struct {
	RunnerID      string   `json:"runner_id,omitempty"`
	RepoURL       string   `json:"repo_url,omitempty"`
	RepoDigest    string   `json:"repo_digest,omitempty"`
	JobName       string   `json:"job_name,omitempty"`
	ExecutorName  string   `json:"executor_name,omitempty"`
	RunnerName    string   `json:"runner_name,omitempty"`
	StartedAt     string   `json:"started_at,omitempty"`
	SchemaVersion string   `json:"schema_version,omitempty"`
	Parameters    []string `json:"parameters,omitempty"`
}

func (s ArtifactUpload) Run(ctx context.Context, e *env.Env) error {
	// todo: impl
	return nil
}
