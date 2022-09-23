//go:build !integration

package helpers

import (
	"errors"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

var UploaderCredentials = common.JobCredentials{
	ID:    1000,
	Token: "test",
	URL:   "test",
}

func TestArtifactsUploaderRequirements(t *testing.T) {
	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	cmd := ArtifactsUploaderCommand{}
	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
}

func TestArtifactsUploaderTooLarge(t *testing.T) {
	network := &testNetwork{
		uploadState: common.UploadTooLarge,
	}
	cmd := ArtifactsUploaderCommand{
		JobCredentials: UploaderCredentials,
		network:        network,
		fileArchiver: fileArchiver{
			Paths: []string{artifactsTestArchivedFile},
		},
	}

	writeTestFile(t, artifactsTestArchivedFile)
	defer os.Remove(artifactsTestArchivedFile)

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})

	assert.Equal(t, 1, network.uploadCalled)
}

func TestArtifactsUploaderForbidden(t *testing.T) {
	network := &testNetwork{
		uploadState: common.UploadForbidden,
	}
	cmd := ArtifactsUploaderCommand{
		JobCredentials: UploaderCredentials,
		network:        network,
		fileArchiver: fileArchiver{
			Paths: []string{artifactsTestArchivedFile},
		},
	}

	writeTestFile(t, artifactsTestArchivedFile)
	defer os.Remove(artifactsTestArchivedFile)

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})

	assert.Equal(t, 1, network.uploadCalled)
}

func TestArtifactsUploaderRetry(t *testing.T) {
	OnEachZipArchiver(t, func(t *testing.T) {
		network := &testNetwork{
			uploadState: common.UploadFailed,
		}
		cmd := ArtifactsUploaderCommand{
			JobCredentials: UploaderCredentials,
			network:        network,
			fileArchiver: fileArchiver{
				Paths: []string{artifactsTestArchivedFile},
			},
		}

		writeTestFile(t, artifactsTestArchivedFile)
		defer os.Remove(artifactsTestArchivedFile)

		removeHook := helpers.MakeFatalToPanic()
		defer removeHook()

		assert.Panics(t, func() {
			cmd.Execute(nil)
		})

		assert.Equal(t, defaultTries, network.uploadCalled)
	})
}

func TestArtifactsUploaderDefaultSucceeded(t *testing.T) {
	OnEachZipArchiver(t, func(t *testing.T) {
		network := &testNetwork{
			uploadState: common.UploadSucceeded,
		}
		cmd := ArtifactsUploaderCommand{
			JobCredentials: UploaderCredentials,
			network:        network,
			fileArchiver: fileArchiver{
				Paths: []string{artifactsTestArchivedFile},
			},
		}

		writeTestFile(t, artifactsTestArchivedFile)
		defer os.Remove(artifactsTestArchivedFile)

		cmd.Execute(nil)
		assert.Equal(t, 1, network.uploadCalled)
		assert.Equal(t, common.ArtifactFormatZip, network.uploadFormat)
		assert.Equal(t, DefaultUploadName+".zip", network.uploadName)
		assert.Empty(t, network.uploadType)
	})
}

func TestArtifactsUploaderZipSucceeded(t *testing.T) {
	OnEachZipArchiver(t, func(t *testing.T) {
		network := &testNetwork{
			uploadState: common.UploadSucceeded,
		}
		cmd := ArtifactsUploaderCommand{
			JobCredentials: UploaderCredentials,
			Format:         common.ArtifactFormatZip,
			Name:           "my-release",
			Type:           "my-type",
			network:        network,
			fileArchiver: fileArchiver{
				Paths: []string{artifactsTestArchivedFile},
			},
		}

		writeTestFile(t, artifactsTestArchivedFile)
		defer os.Remove(artifactsTestArchivedFile)

		cmd.Execute(nil)
		assert.Equal(t, 1, network.uploadCalled)
		assert.Equal(t, common.ArtifactFormatZip, network.uploadFormat)
		assert.Equal(t, "my-release.zip", network.uploadName)
		assert.Equal(t, "my-type", network.uploadType)
		assert.Contains(t, network.uploadedFiles, artifactsTestArchivedFile)
	})
}

func TestArtifactsUploaderGzipSendsMultipleFiles(t *testing.T) {
	network := &testNetwork{
		uploadState: common.UploadSucceeded,
	}
	cmd := ArtifactsUploaderCommand{
		JobCredentials: UploaderCredentials,
		Format:         common.ArtifactFormatGzip,
		Name:           "junit.xml",
		Type:           "junit",
		network:        network,
		fileArchiver: fileArchiver{
			Paths: []string{artifactsTestArchivedFile, artifactsTestArchivedFile2},
		},
	}

	writeTestFile(t, artifactsTestArchivedFile)
	defer os.Remove(artifactsTestArchivedFile)

	writeTestFile(t, artifactsTestArchivedFile2)
	defer os.Remove(artifactsTestArchivedFile)

	cmd.Execute(nil)
	assert.Equal(t, 1, network.uploadCalled)
	assert.Equal(t, "junit.xml.gz", network.uploadName)
	assert.Equal(t, common.ArtifactFormatGzip, network.uploadFormat)
	assert.Equal(t, "junit", network.uploadType)
	assert.Contains(t, network.uploadedFiles, artifactsTestArchivedFile)
	assert.Contains(t, network.uploadedFiles, artifactsTestArchivedFile2)
}

func TestArtifactsUploaderRawSucceeded(t *testing.T) {
	network := &testNetwork{
		uploadState: common.UploadSucceeded,
	}
	cmd := ArtifactsUploaderCommand{
		JobCredentials: UploaderCredentials,
		Format:         common.ArtifactFormatRaw,
		Name:           "my-release",
		Type:           "my-type",
		network:        network,
		fileArchiver: fileArchiver{
			Paths: []string{artifactsTestArchivedFile},
		},
	}

	writeTestFile(t, artifactsTestArchivedFile)
	defer os.Remove(artifactsTestArchivedFile)

	cmd.Execute(nil)
	assert.Equal(t, 1, network.uploadCalled)
	assert.Equal(t, common.ArtifactFormatRaw, network.uploadFormat)
	assert.Equal(t, "my-release", network.uploadName)
	assert.Equal(t, "my-type", network.uploadType)
	assert.Contains(t, network.uploadedFiles, "raw")
}

func TestArtifactsUploaderRawDoesNotSendMultipleFiles(t *testing.T) {
	network := &testNetwork{
		uploadState: common.UploadSucceeded,
	}
	cmd := ArtifactsUploaderCommand{
		JobCredentials: UploaderCredentials,
		Format:         common.ArtifactFormatRaw,
		Name:           "junit.xml",
		Type:           "junit",
		network:        network,
		fileArchiver: fileArchiver{
			Paths: []string{artifactsTestArchivedFile, artifactsTestArchivedFile2},
		},
	}

	writeTestFile(t, artifactsTestArchivedFile)
	defer os.Remove(artifactsTestArchivedFile)

	writeTestFile(t, artifactsTestArchivedFile2)
	defer os.Remove(artifactsTestArchivedFile2)

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
}

func TestArtifactsUploaderNoFilesDoNotGenerateError(t *testing.T) {
	network := &testNetwork{
		uploadState: common.UploadSucceeded,
	}
	cmd := ArtifactsUploaderCommand{
		JobCredentials: UploaderCredentials,
		network:        network,
		fileArchiver:   fileArchiver{},
	}

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	assert.NotPanics(t, func() {
		cmd.Execute(nil)
	})
}

func TestArtifactsUploaderServiceUnavailable(t *testing.T) {
	network := &testNetwork{
		uploadState: common.UploadServiceUnavailable,
	}
	cmd := ArtifactsUploaderCommand{
		JobCredentials: UploaderCredentials,
		network:        network,
		fileArchiver: fileArchiver{
			Paths: []string{artifactsTestArchivedFile},
		},
	}

	writeTestFile(t, artifactsTestArchivedFile)
	defer os.Remove(artifactsTestArchivedFile)

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})

	assert.Equal(t, serviceUnavailableTries, network.uploadCalled)
}

func TestArtifactsExcludedPaths(t *testing.T) {
	network := &testNetwork{
		uploadState: common.UploadSucceeded,
	}

	cmd := ArtifactsUploaderCommand{
		JobCredentials: UploaderCredentials,
		network:        network,
		Format:         common.ArtifactFormatRaw,
		fileArchiver: fileArchiver{
			Paths:   []string{artifactsTestArchivedFile},
			Exclude: []string{"something/**"},
		},
	}

	writeTestFile(t, artifactsTestArchivedFile)
	defer os.Remove(artifactsTestArchivedFile)

	cmd.Execute(nil)

	assert.Equal(t, 1, network.uploadCalled)
}

func TestFileArchiverCompressionLevel(t *testing.T) {
	writeTestFile(t, artifactsTestArchivedFile)
	defer os.Remove(artifactsTestArchivedFile)

	network := &testNetwork{
		uploadState: common.UploadSucceeded,
	}

	for _, expectedLevel := range []string{"fastest", "fast", "default", "slow", "slowest"} {
		t.Run(expectedLevel, func(t *testing.T) {
			mockArchiver := new(archive.MockArchiver)
			defer mockArchiver.AssertExpectations(t)

			archive.Register(
				"zip",
				func(w io.Writer, dir string, level archive.CompressionLevel) (archive.Archiver, error) {
					assert.Equal(t, GetCompressionLevel(expectedLevel), level)
					return mockArchiver, nil
				},
				nil,
			)

			mockArchiver.On("Archive", mock.Anything, mock.Anything).Return(nil)

			cmd := ArtifactsUploaderCommand{
				JobCredentials: UploaderCredentials,
				network:        network,
				Format:         common.ArtifactFormatZip,
				fileArchiver: fileArchiver{
					Paths: []string{artifactsTestArchivedFile},
				},
				CompressionLevel: expectedLevel,
			}
			assert.NoError(t, cmd.enumerate())
			_, r, err := cmd.createReadStream()
			require.NoError(t, err)
			defer r.Close()
			_, _ = io.Copy(io.Discard, r)
		})
	}
}

func TestArtifactUploaderCommandShouldRetry(t *testing.T) {
	tests := map[string]struct {
		err   error
		tries int

		expectedShouldRetry bool
	}{
		"no error, first try": {
			err:   nil,
			tries: 1,

			expectedShouldRetry: false,
		},
		"random error, first try": {
			err:   errors.New("err"),
			tries: 1,

			expectedShouldRetry: false,
		},
		"retryable error, first try": {
			err:   retryableErr{},
			tries: 1,

			expectedShouldRetry: true,
		},
		"retryable error, max tries": {
			err:   retryableErr{},
			tries: defaultTries,

			expectedShouldRetry: false,
		},
		"retryable error, over max tries limit": {
			err:   retryableErr{},
			tries: defaultTries + 10,

			expectedShouldRetry: false,
		},
		"retryable error, before reaching service unavailable tries": {
			err:   retryableErr{err: errServiceUnavailable},
			tries: serviceUnavailableTries - 1,

			expectedShouldRetry: true,
		},
		"retryable error service unavailable, max tries": {
			err:   retryableErr{err: errServiceUnavailable},
			tries: serviceUnavailableTries,

			expectedShouldRetry: false,
		},
		"retryable error service unavailable, over max errors limit": {
			err:   retryableErr{err: errServiceUnavailable},
			tries: serviceUnavailableTries + 10,

			expectedShouldRetry: false,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			r := ArtifactsUploaderCommand{}
			assert.Equal(t, tt.expectedShouldRetry, r.ShouldRetry(tt.tries, tt.err))
		})
	}
}
