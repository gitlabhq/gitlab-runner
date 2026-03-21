package stages

import (
	"context"
	"fmt"
	"path"
	"strconv"

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

//nolint:gocognit
func (s CacheExtract) Run(ctx context.Context, e *env.Env) error {
	if len(s.Sources) == 0 {
		e.Debugf("Skipping cache extraction: no sources configured")
		return nil
	}

	for _, w := range s.Warnings {
		e.Warningf("%s", w)
	}

	attempts := max(1, s.MaxAttempts)

	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			e.Warningf("Retrying cache extraction (attempt %d/%d)...", attempt, attempts)
		}

		for i, src := range s.Sources {
			for _, w := range src.Warnings {
				e.Warningf("%s", w)
			}

			if i == 0 {
				e.Noticef("Checking cache for %s...", src.Name)
			} else {
				e.Noticef("Checking cache for %s (fallback)...", src.Name)
			}

			err := s.extract(ctx, e, src)
			if err == nil {
				e.Noticef("Successfully extracted cache")
				return nil
			}

			e.Warningf("Failed to extract cache %s: %v", src.Name, err)

			// todo: Cleanup failed extraction... this functionality is likely broken in the abstract
			// shell. So if we want to keep this, we likely need a new implementation.
			// if s.CleanupFailedExtract {
			// }
		}
	}

	return nil
}

func (s CacheExtract) extract(ctx context.Context, e *env.Env, src CacheSource) error {
	archiveFile := path.Join(e.CacheDir, src.Key, "cache.zip")

	args := []string{
		"cache-extractor",
		"--file", archiveFile,
		"--timeout", strconv.Itoa(s.Timeout),
	}

	desc := src.Descriptor
	if desc.URL != "" {
		if desc.GoCloudURL {
			args = append(args, "--gocloud-url", desc.URL)
		} else {
			args = append(args, "--url", desc.URL)
		}
	}

	for k, values := range desc.Headers {
		for _, v := range values {
			args = append(args, "--header", fmt.Sprintf("%s: %s", k, v))
		}
	}

	return e.RunnerCommand(ctx, e.HelperEnvs(desc.Env), args...)
}
