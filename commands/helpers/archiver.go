package helpers

import (
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
	// IsOnFromEnv labels its own log entries with the feature flag name.
	registerArchivers(logrus.StandardLogger())
}

// registerArchivers registers the fastzip archiver and extractor when
// FF_USE_FASTZIP resolves to on. It is kept separate from init() so that tests
// can exercise the registration with a controlled environment. It reports
// whether fastzip was registered.
func registerArchivers(logger logrus.FieldLogger) bool {
	if !featureflags.IsOnFromEnv(logger, featureflags.UseFastzip) {
		return false
	}

	archive.Register(archive.Zip, fastzip.NewArchiver, fastzip.NewExtractor)

	// The default zstd compressor is fastzip, registered by the fastzip
	// implementation itself (commands/helpers/archive/fastzip).
	//
	// The legacy zip implementation (commands/helpers/archive/ziplegacy)
	// registers a zstd decompressor, so that the legacy implementation can
	// still decompress zstd even though it cannot compress it (only fastzip
	// can). Registering fastzip's extractor here overrides that, making
	// fastzip the zstd decompressor whenever the flag is on, and leaving the
	// older extraction behaviour available by turning the flag off.
	archive.Register(archive.ZipZstd, nil, fastzip.NewExtractor)

	return true
}

// GetCompressionLevel converts the compression level name to compression level type
// https://docs.gitlab.com/ci/runners/configure_runners/#artifact-and-cache-settings
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
