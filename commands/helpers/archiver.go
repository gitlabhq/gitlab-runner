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

	"github.com/sirupsen/logrus"
)

func init() {
	// enable fastzip archiver/extractor
	logger := logrus.WithField("name", featureflags.UseFastzip)
	if on := featureflags.IsOn(logger, os.Getenv(featureflags.UseFastzip)); on {
		archive.Register(archive.Zip, fastzip.NewArchiver, fastzip.NewExtractor)
	}
}

// GetCompressionLevel converts the compression level name to compression level type
// https://docs.gitlab.com/ee/ci/runners/README.html#artifact-and-cache-settings
func GetCompressionLevel(name string) archive.CompressionLevel {
	switch name {
	case "fastest":
		return archive.FastestCompression
	case "fast":
		return archive.FastCompression
	case "slow":
		return archive.SlowCompression
	case "slowest":
		return archive.SlowestCompression
	case "default", "":
		return archive.DefaultCompression
	}

	logrus.Warningf("compression level %q is invalid, falling back to default", name)

	return archive.DefaultCompression
}
