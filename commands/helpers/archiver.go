package helpers

import (
	// auto-register default archivers/extractors
	_ "gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/gziplegacy"
	_ "gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/raw"
	_ "gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/ziplegacy"
)

func init() {
	// Register archivers/extractors based on feature flags/environment:
	// https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2210
}
