package stages

import (
	"context"
	"maps"
	"strconv"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/cacheprovider"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/env"
	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run/stages/internal/retry"
)

type CacheSource struct {
	Name       string                   `json:"name,omitempty"`
	Key        string                   `json:"key,omitempty"`
	Descriptor cacheprovider.Descriptor `json:"descriptor,omitempty"`
	// AlternateKey / AlternateDescriptor carry the FF_HASH_CACHE_KEYS-opposite form so
	// cache-extractor can pick whichever URL has the newer Last-Modified timestamp.
	AlternateKey        string                   `json:"alternate_key,omitempty"`
	AlternateDescriptor cacheprovider.Descriptor `json:"alternate_descriptor,omitempty"`
	Warnings            []string                 `json:"warnings,omitempty"`
}

type CacheExtract struct {
	Sources     []CacheSource `json:"sources,omitempty"`
	Timeout     int           `json:"timeout,omitempty"`
	Concurrency int           `json:"concurrency,omitempty"`
	MaxAttempts int           `json:"max_attempts,omitempty"`
	Paths       []string      `json:"paths,omitempty"`
	Warnings    []string      `json:"warnings,omitempty"`
	// UseExponentialBackoffStageRetry gates exponential sleep between retry attempts; when false retries run back-to-back.
	UseExponentialBackoffStageRetry bool `json:"use_exponential_backoff_stage_retry,omitempty"`
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
	backoff := retry.NewBackoff()

	for attempt := 1; attempt <= attempts; attempt++ {
		if attempt > 1 {
			if s.UseExponentialBackoffStageRetry {
				retry.SleepWithNotice(e, backoff.Duration())
			}
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
		}
	}

	return nil
}

func (s CacheExtract) extract(ctx context.Context, e *env.Env, src CacheSource) error {
	archiveFile := s.archivePath(e, src.Key)

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
	if desc.HeadURL != "" {
		args = append(args, "--head-url", desc.HeadURL)
	}

	alt := src.AlternateDescriptor
	if alt.URL != "" {
		if alt.GoCloudURL {
			args = append(args, "--alternate-gocloud-url", alt.URL)
		} else {
			args = append(args, "--alternate-url", alt.URL)
		}
		if alt.HeadURL != "" {
			args = append(args, "--alternate-head-url", alt.HeadURL)
		}
	}

	// cache-extractor doesn't accept --header (only cache-archiver does),
	// so drop them. Matches abstract shell, which also doesn't forward
	// headers on the download path.
	_ = desc.Headers

	// Primary wins on key collision: cache-extractor sees a single env
	// per invocation, so the URL it attempts first must keep its own
	// credentials. alt's env only fills keys the primary descriptor
	// didn't set.
	envOverlay := make(map[string]string, len(desc.Env)+len(alt.Env))
	maps.Copy(envOverlay, alt.Env)
	maps.Copy(envOverlay, desc.Env)
	return e.RunnerCommand(ctx, e.HelperEnvs(envOverlay), args...)
}

func (s CacheExtract) archivePath(e *env.Env, key string) string {
	return cacheArchivePath(e, key)
}
