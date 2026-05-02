package stages

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strconv"

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
	if !s.shouldRun(e) {
		e.Debugf("Skipping cache archiving for %s: not applicable for current job status", s.Key)
		return nil
	}

	archiverArgs := s.archiverArgs()
	if len(archiverArgs) == 0 {
		e.Debugf("Skipping cache archiving for %s: no paths to archive", s.Key)
		return nil
	}

	e.Noticef("Creating cache %s...", s.Key)

	archiveFile := path.Join(e.CacheDir, s.Key, "cache.zip")

	args := []string{
		"cache-archiver",
		"--file", archiveFile,
		"--timeout", strconv.Itoa(s.Timeout),
	}

	if s.MaxUploadedArchiveSize > 0 {
		args = append(args, "--max-uploaded-archive-size", strconv.FormatInt(s.MaxUploadedArchiveSize, 10))
	}

	args = append(args, archiverArgs...)

	desc := s.Descriptor
	if desc.URL != "" {
		if desc.GoCloudURL {
			args = append(args, "--gocloud-url", desc.URL)
		} else {
			args = append(args, "--url", desc.URL)
		}
	}

	if desc.HeadURL != "" {
		args = append(args, "--check-url", desc.HeadURL)
	}

	for k, values := range desc.Headers {
		for _, v := range values {
			args = append(args, "--header", fmt.Sprintf("%s: %s", k, v))
		}
	}

	if desc.Env == nil {
		desc.Env = make(map[string]string)
	}

	metaJSON, _ := json.Marshal(map[string]string{"cachekey": s.Name})
	desc.Env["CACHE_METADATA"] = string(metaJSON)

	if err := e.RunnerCommand(ctx, e.HelperEnvs(desc.Env), args...); err != nil {
		e.Warningf("Failed to create cache")
		return fmt.Errorf("archiving cache %s: %w", s.Key, err)
	}

	e.Noticef("Created cache")
	return nil
}

func (s CacheArchive) shouldRun(e *env.Env) bool {
	if e.IsSuccessful() {
		return s.OnSuccess
	}
	return s.OnFailure
}

func (s CacheArchive) archiverArgs() []string {
	var args []string

	for _, p := range s.Paths {
		args = append(args, "--path", p)
	}

	if s.Untracked {
		args = append(args, "--untracked")
	}

	return args
}
