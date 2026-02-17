package stages

import (
	"context"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/cacheprovider"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
)

type CacheArchive struct {
	Name                   string                   `json:"name,omitempty"`
	Key                    string                   `json:"key,omitempty"`
	Untracked              bool                     `json:"untracked,omitempty"`
	Paths                  []string                 `json:"paths,omitempty"`
	ArchiverFormat         string                   `json:"archiver_format,omitempty"`
	CompressionLevel       string                   `json:"compression_level,omitempty"`
	Timeout                int                      `json:"timeout,omitempty"`
	Descriptor             cacheprovider.Descriptor `json:"descriptor,omitempty"`
	MaxUploadedArchiveSize int64                    `json:"max_uploaded_archive_size,omitempty"`
	OnSuccess              bool                     `json:"on_success,omitempty"`
	OnFailure              bool                     `json:"on_failure,omitempty"`
	Warnings               []string                 `json:"warnings,omitempty"`
}

func (s CacheArchive) Run(ctx context.Context, e *env.Env) error {
	// todo: impl
	return nil
}
