package stages

import (
	"context"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/cacheprovider"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
)

type CacheSource struct {
	Name       string                   `json:"name,omitempty"`
	Key        string                   `json:"key,omitempty"`
	Descriptor cacheprovider.Descriptor `json:"descriptor,omitempty"`
	Warnings   []string                 `json:"warnings,omitempty"`
}

type CacheExtract struct {
	Sources              []CacheSource `json:"sources,omitempty"`
	Timeout              int           `json:"timeout,omitempty"`
	Concurrency          int           `json:"concurrency,omitempty"`
	MaxAttempts          int           `json:"max_attempts,omitempty"`
	Paths                []string      `json:"paths,omitempty"`
	CleanupFailedExtract bool          `json:"cleanup_failed_extract,omitempty"`
	Warnings             []string      `json:"warnings,omitempty"`
}

func (s CacheExtract) Run(ctx context.Context, e *env.Env) error {
	// todo: impl
	return nil
}
