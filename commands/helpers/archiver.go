package helpers

import (
	"os"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/fastzip"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"

	// auto-register default archivers/extractors
	_ "gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/gziplegacy"
	_ "gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/raw"
	_ "gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/tarzstd"
	_ "gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/ziplegacy"

	"github.com/sirupsen/logrus"
)

func init() {
	// enable fastzip archiver/extractor
	logger := logrus.WithField("name", featureflags.UseFastzip)
	if on := featureflags.IsOn(logger, os.Getenv(featureflags.UseFastzip)); on {
		archive.Register(archive.Zip, fastzip.NewArchiver, fastzip.NewExtractor)

		// The default zstd compressor is fastzip, this is registered via the
		// fastzip implementation (helpers/archive/fastzip).
		//
		// The default zstd decompressor is the legacy zip implementation (helpers/archive/ziplegacy).
		// This intended to allow the default zip implementation to still be able to decompress zstd,
		// even if it is unable to compress it (only fastzip can compress). This also allows the older
		// extraction behaviour to be enabled.
		//
		// Here we're registering the decompress only if FF_USE_FASTZIP is enabled. This overrides
		// the ziplegacy zstd support.
		archive.Register(archive.ZipZstd, nil, fastzip.NewExtractor)
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
