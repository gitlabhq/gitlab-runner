package helpers

import (
	"os"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/fastzip"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"

	// auto-register default archivers/extractors
	_ "gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/gziplegacy"
	_ "gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/raw"
	_ "gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/ziplegacy"
)

func init() {
	// enable fastzip archiver/extractor
	if on, _ := featureflags.IsOn(os.Getenv(featureflags.UseFastzip)); on {
		archive.Register(archive.Zip, fastzip.NewArchiver, fastzip.NewExtractor)
	}
}
