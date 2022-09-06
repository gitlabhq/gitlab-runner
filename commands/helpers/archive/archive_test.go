//go:build !integration

package archive_test

import (
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/gziplegacy"
	_ "gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/raw"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive/ziplegacy"
)

func TestDefaultRegistration(t *testing.T) {
	tests := map[archive.Format]struct {
		hasArchiver, hasExtractor bool
	}{
		archive.Raw:  {hasArchiver: true, hasExtractor: false},
		archive.Gzip: {hasArchiver: true, hasExtractor: false},
		archive.Zip:  {hasArchiver: true, hasExtractor: true},
	}

	for tn, tc := range tests {
		t.Run(string(tn), func(t *testing.T) {
			_, err := archive.NewArchiver(tn, nil, "", archive.DefaultCompression)

			if tc.hasArchiver {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, archive.ErrUnsupportedArchiveFormat)
			}

			_, err = archive.NewExtractor(tn, nil, 0, "")

			if tc.hasExtractor {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, archive.ErrUnsupportedArchiveFormat)
			}
		})
	}
}

func TestRegister(t *testing.T) {
	format := archive.Format("new-format")

	archive.Register(format, ziplegacy.NewArchiver, ziplegacy.NewExtractor)

	_, err := archive.NewArchiver(format, nil, "", archive.DefaultCompression)
	assert.NoError(t, err)

	_, err = archive.NewExtractor(format, nil, 0, "")
	assert.NoError(t, err)
}

func TestRegisterOverride(t *testing.T) {
	existingGzipArchiver, err := gziplegacy.NewArchiver(io.Discard, "", archive.DefaultCompression)
	assert.NoError(t, err)

	existingZipArchiver, err := ziplegacy.NewArchiver(io.Discard, "", archive.DefaultCompression)
	assert.NoError(t, err)

	existingZipExtractor, err := ziplegacy.NewExtractor(nil, 0, "")
	assert.NoError(t, err)

	// assert existing archiver
	archiver, err := archive.NewArchiver(archive.Gzip, nil, "", archive.DefaultCompression)
	assert.NoError(t, err)
	assert.IsType(t, existingGzipArchiver, archiver)

	_, err = archive.NewExtractor(archive.Gzip, nil, 0, "")
	assert.Error(t, err)

	// override
	archive.Register(archive.Gzip, ziplegacy.NewArchiver, ziplegacy.NewExtractor)

	archiver, err = archive.NewArchiver(archive.Gzip, nil, "", archive.DefaultCompression)
	assert.NoError(t, err)
	assert.IsType(t, existingZipArchiver, archiver)

	extractor, err := archive.NewExtractor(archive.Gzip, nil, 0, "")
	assert.NoError(t, err)
	assert.IsType(t, existingZipExtractor, extractor)
}
