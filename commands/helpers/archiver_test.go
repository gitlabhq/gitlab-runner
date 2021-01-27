package helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/fastzip"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/ziplegacy"
)

func OnEachZipArchiver(t *testing.T, f func(t *testing.T)) {
	archivers := map[string]archive.NewArchiverFunc{
		"legacy":  ziplegacy.NewArchiver,
		"fastzip": fastzip.NewArchiver,
	}

	for name, archiver := range archivers {
		archive.Register(archive.Zip, archiver, ziplegacy.NewExtractor)
		t.Run(name, f)
	}
}

func OnEachZipExtractor(t *testing.T, f func(t *testing.T)) {
	extractors := map[string]archive.NewExtractorFunc{
		"legacy":  ziplegacy.NewExtractor,
		"fastzip": fastzip.NewExtractor,
	}

	for name, extractor := range extractors {
		archive.Register(archive.Zip, ziplegacy.NewArchiver, extractor)
		t.Run(name, f)
	}
}

func TestCompressionLevel(t *testing.T) {
	tests := map[string]archive.CompressionLevel{
		"fastest": archive.FastestCompression,
		"fast":    archive.FastCompression,
		"slow":    archive.SlowCompression,
		"slowest": archive.SlowestCompression,
		"default": archive.DefaultCompression,
		"":        archive.DefaultCompression,
		"invalid": archive.DefaultCompression,
	}

	for name, level := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, level, getCompressionLevel(name))
		})
	}
}
