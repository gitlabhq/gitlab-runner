// Helper functions that are shared between unit tests and integration tests

package helpers

import (
	"testing"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/fastzip"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/ziplegacy"
)

func OnEachZipArchiver(t *testing.T, f func(t *testing.T), include ...string) {
	archivers := map[string]archive.NewArchiverFunc{
		"legacy":  ziplegacy.NewArchiver,
		"fastzip": fastzip.NewArchiver,
	}

	for name, archiver := range archivers {
		if !hasArchiver(name, include) {
			continue
		}
		archive.Register(archive.Zip, archiver, ziplegacy.NewExtractor)
		t.Run(name, f)
	}
}

func OnEachZipExtractor(t *testing.T, f func(t *testing.T), include ...string) {
	extractors := map[string]archive.NewExtractorFunc{
		"legacy":  ziplegacy.NewExtractor,
		"fastzip": fastzip.NewExtractor,
	}

	for name, extractor := range extractors {
		if !hasArchiver(name, include) {
			continue
		}
		archive.Register(archive.Zip, ziplegacy.NewArchiver, extractor)
		t.Run(name, f)
	}
}

func hasArchiver(name string, include []string) bool {
	if len(include) == 0 {
		return true
	}

	for _, inc := range include {
		if inc == name {
			return true
		}
	}
	return false
}
