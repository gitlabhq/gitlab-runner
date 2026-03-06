package stages

import (
	"context"
	"fmt"
	"strconv"
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

func (m ArtifactMetadata) args() []string {
	args := []string{
		"--generate-artifacts-metadata",
		"--runner-id", m.RunnerID,
		"--repo-url", m.RepoURL,
		"--repo-digest", m.RepoDigest,
		"--job-name", m.JobName,
		"--executor-name", m.ExecutorName,
		"--runner-name", m.RunnerName,
		"--started-at", m.StartedAt,
		"--ended-at", time.Now().Format(time.RFC3339),
		"--schema-version", m.SchemaVersion,
	}

	for _, p := range m.Parameters {
		args = append(args, "--metadata-parameter", p)
	}

	return args
}

func (s ArtifactUpload) Run(ctx context.Context, e *env.Env) error {
	if !s.shouldRun(e) {
		e.Debugf("Skipping artifact upload %q: not applicable for current job status", s.ArtifactName)
		return nil
	}

	archiverArgs := s.archiverArgs()
	if len(archiverArgs) == 0 {
		e.Debugf("Skipping artifact upload %q: no paths to archive", s.ArtifactName)
		return nil
	}

	e.Noticef("Uploading artifacts...")

	args := []string{
		"artifacts-uploader",
		"--url", e.BaseURL,
		"--token", e.Token,
		"--id", strconv.FormatInt(e.ID, 10),
	}

	if s.Timeout != 0 {
		args = append(args, "--timeout", fmt.Sprintf("%v", s.Timeout))
	}

	if s.ResponseHeaderTimeout != 0 {
		args = append(args, "--response-header-timeout", fmt.Sprintf("%v", s.ResponseHeaderTimeout))
	}

	if s.Metadata != nil {
		args = append(args, s.Metadata.args()...)
	}

	args = append(args, archiverArgs...)

	if s.ArtifactName != "" {
		args = append(args, "--name", s.ArtifactName)
	}

	if s.ExpireIn != "" {
		args = append(args, "--expire-in", s.ExpireIn)
	}

	if s.Format != "" {
		args = append(args, "--artifact-format", s.Format)
	}

	if s.Type != "" {
		args = append(args, "--artifact-type", s.Type)
	}

	if s.CompressionLevel != "" {
		args = append(args, "--compression-level", s.CompressionLevel)
	}

	if err := e.RunnerCommand(ctx, e.HelperEnvs(nil), args...); err != nil {
		return fmt.Errorf("uploading artifacts %q: %w", s.ArtifactName, err)
	}

	return nil
}

func (s ArtifactUpload) shouldRun(e *env.Env) bool {
	if e.IsSuccessful() {
		return s.OnSuccess
	}
	return s.OnFailure
}

func (s ArtifactUpload) archiverArgs() []string {
	var args []string

	for _, p := range s.Paths {
		args = append(args, "--path", p)
	}

	for _, p := range s.Exclude {
		args = append(args, "--exclude", p)
	}

	if s.Untracked {
		args = append(args, "--untracked")
	}

	return args
}
