//go:build !integration

package helpers

import (
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

var UploaderCredentials = common.JobCredentials{
	ID:    1000,
	Token: "test",
	URL:   "test",
}

// Create a function that returns a Network interface with injected test behavior
func createTestNewNetwork(testNet *testNetwork) func() common.Network {
	return func() common.Network {
		return testNet
	}
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
	testNet := &testNetwork{
		uploadState: common.UploadTooLarge,
	}
	cmd := ArtifactsUploaderCommand{
		JobCredentials: UploaderCredentials,
		newNetwork:     createTestNewNetwork(testNet),
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

	assert.Equal(t, 1, testNet.uploadCalled)
}

func TestArtifactsUploaderForbidden(t *testing.T) {
	testNet := &testNetwork{
		uploadState: common.UploadForbidden,
	}
	cmd := ArtifactsUploaderCommand{
		JobCredentials: UploaderCredentials,
		newNetwork:     createTestNewNetwork(testNet),
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

	assert.Equal(t, 1, testNet.uploadCalled)
}

func TestArtifactsUploaderRetry(t *testing.T) {
	OnEachZipArchiver(t, func(t *testing.T) {
		testNet := &testNetwork{
			uploadState: common.UploadFailed,
		}
		cmd := ArtifactsUploaderCommand{
			JobCredentials: UploaderCredentials,
			newNetwork:     createTestNewNetwork(testNet),
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

		assert.Equal(t, defaultTries, testNet.uploadCalled)
	})
}

func TestArtifactsUploaderDefaultSucceeded(t *testing.T) {
	OnEachZipArchiver(t, func(t *testing.T) {
		testNet := &testNetwork{
			uploadState: common.UploadSucceeded,
		}
		cmd := ArtifactsUploaderCommand{
			JobCredentials: UploaderCredentials,
			newNetwork:     createTestNewNetwork(testNet),
			fileArchiver: fileArchiver{
				Paths: []string{artifactsTestArchivedFile},
			},
		}

		writeTestFile(t, artifactsTestArchivedFile)
		defer os.Remove(artifactsTestArchivedFile)

		cmd.Execute(nil)
		assert.Equal(t, 1, testNet.uploadCalled)
		assert.Equal(t, spec.ArtifactFormatZip, testNet.uploadFormat)
		assert.Equal(t, DefaultUploadName+".zip", testNet.uploadName)
		assert.Empty(t, testNet.uploadType)
	})
}

func TestArtifactsUploaderZipSucceeded(t *testing.T) {
	OnEachZipArchiver(t, func(t *testing.T) {
		testNet := &testNetwork{
			uploadState: common.UploadSucceeded,
		}
		cmd := ArtifactsUploaderCommand{
			JobCredentials: UploaderCredentials,
			Format:         spec.ArtifactFormatZip,
			Name:           "my-release",
			Type:           "my-type",
			newNetwork:     createTestNewNetwork(testNet),
			fileArchiver: fileArchiver{
				Paths: []string{artifactsTestArchivedFile},
			},
		}

		writeTestFile(t, artifactsTestArchivedFile)
		defer os.Remove(artifactsTestArchivedFile)

		cmd.Execute(nil)
		assert.Equal(t, 1, testNet.uploadCalled)
		assert.Equal(t, spec.ArtifactFormatZip, testNet.uploadFormat)
		assert.Equal(t, "my-release.zip", testNet.uploadName)
		assert.Equal(t, "my-type", testNet.uploadType)
		assert.Contains(t, testNet.uploadedFiles, artifactsTestArchivedFile)
	})
}

func TestArtifactsUploaderGzipSendsMultipleFiles(t *testing.T) {
	testNet := &testNetwork{
		uploadState: common.UploadSucceeded,
	}
	cmd := ArtifactsUploaderCommand{
		JobCredentials: UploaderCredentials,
		Format:         spec.ArtifactFormatGzip,
		Name:           "junit.xml",
		Type:           "junit",
		newNetwork:     createTestNewNetwork(testNet),
		fileArchiver: fileArchiver{
			Paths: []string{artifactsTestArchivedFile, artifactsTestArchivedFile2},
		},
	}

	writeTestFile(t, artifactsTestArchivedFile)
	defer os.Remove(artifactsTestArchivedFile)

	writeTestFile(t, artifactsTestArchivedFile2)
	defer os.Remove(artifactsTestArchivedFile)

	cmd.Execute(nil)
	assert.Equal(t, 1, testNet.uploadCalled)
	assert.Equal(t, "junit.xml.gz", testNet.uploadName)
	assert.Equal(t, spec.ArtifactFormatGzip, testNet.uploadFormat)
	assert.Equal(t, "junit", testNet.uploadType)
	assert.Contains(t, testNet.uploadedFiles, artifactsTestArchivedFile)
	assert.Contains(t, testNet.uploadedFiles, artifactsTestArchivedFile2)
}

func TestArtifactsUploaderRawSucceeded(t *testing.T) {
	testNet := &testNetwork{
		uploadState: common.UploadSucceeded,
	}
	cmd := ArtifactsUploaderCommand{
		JobCredentials: UploaderCredentials,
		Format:         spec.ArtifactFormatRaw,
		Name:           "my-release",
		Type:           "my-type",
		newNetwork:     createTestNewNetwork(testNet),
		fileArchiver: fileArchiver{
			Paths: []string{artifactsTestArchivedFile},
		},
	}

	writeTestFile(t, artifactsTestArchivedFile)
	defer os.Remove(artifactsTestArchivedFile)

	cmd.Execute(nil)
	assert.Equal(t, 1, testNet.uploadCalled)
	assert.Equal(t, spec.ArtifactFormatRaw, testNet.uploadFormat)
	assert.Equal(t, "my-release", testNet.uploadName)
	assert.Equal(t, "my-type", testNet.uploadType)
	assert.Contains(t, testNet.uploadedFiles, "raw")
}

func TestArtifactsUploaderRawDoesNotSendMultipleFiles(t *testing.T) {
	testNet := &testNetwork{
		uploadState: common.UploadSucceeded,
	}
	cmd := ArtifactsUploaderCommand{
		JobCredentials: UploaderCredentials,
		Format:         spec.ArtifactFormatRaw,
		Name:           "junit.xml",
		Type:           "junit",
		newNetwork:     createTestNewNetwork(testNet),
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
	testNet := &testNetwork{
		uploadState: common.UploadSucceeded,
	}
	cmd := ArtifactsUploaderCommand{
		JobCredentials: UploaderCredentials,
		newNetwork:     createTestNewNetwork(testNet),
		fileArchiver:   fileArchiver{},
	}

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	assert.NotPanics(t, func() {
		cmd.Execute(nil)
	})
}

func TestArtifactsUploaderServiceUnavailable(t *testing.T) {
	testNet := &testNetwork{
		uploadState: common.UploadServiceUnavailable,
	}
	cmd := ArtifactsUploaderCommand{
		JobCredentials: UploaderCredentials,
		newNetwork:     createTestNewNetwork(testNet),
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

	assert.Equal(t, serviceUnavailableTries, testNet.uploadCalled)
}

func TestArtifactsExcludedPaths(t *testing.T) {
	testNet := &testNetwork{
		uploadState: common.UploadSucceeded,
	}

	cmd := ArtifactsUploaderCommand{
		JobCredentials: UploaderCredentials,
		newNetwork:     createTestNewNetwork(testNet),
		Format:         spec.ArtifactFormatRaw,
		fileArchiver: fileArchiver{
			Paths:   []string{artifactsTestArchivedFile},
			Exclude: []string{"something/**"},
		},
	}

	writeTestFile(t, artifactsTestArchivedFile)
	defer os.Remove(artifactsTestArchivedFile)

	cmd.Execute(nil)

	assert.Equal(t, 1, testNet.uploadCalled)
}

func TestFileArchiverCompressionLevel(t *testing.T) {
	writeTestFile(t, artifactsTestArchivedFile)
	defer os.Remove(artifactsTestArchivedFile)

	testNet := &testNetwork{
		uploadState: common.UploadSucceeded,
	}

	for _, expectedLevel := range []string{"fastest", "fast", "default", "slow", "slowest"} {
		t.Run(expectedLevel, func(t *testing.T) {
			mockArchiver := archive.NewMockArchiver(t)

			// Save previous archiver and restore it after test to prevent
			// goroutine assertions from affecting subsequent tests
			prevArchiver, _ := archive.Register(
				"zip",
				func(w io.Writer, dir string, level archive.CompressionLevel) (archive.Archiver, error) {
					assert.Equal(t, GetCompressionLevel(expectedLevel), level)
					return mockArchiver, nil
				},
				nil,
			)
			defer func() {
				archive.Register("zip", prevArchiver, nil)
			}()

			mockArchiver.On("Archive", mock.Anything, mock.Anything).Return(nil)

			cmd := ArtifactsUploaderCommand{
				JobCredentials: UploaderCredentials,
				newNetwork:     createTestNewNetwork(testNet),
				Format:         spec.ArtifactFormatZip,
				fileArchiver: fileArchiver{
					Paths: []string{artifactsTestArchivedFile},
				},
				CompressionLevel: expectedLevel,
			}
			assert.NoError(t, cmd.enumerate())
			_, bodyProvider := cmd.createBodyProvider()
			r, err := bodyProvider.GetReader()
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
			assert.Equal(t, tt.expectedShouldRetry, r.shouldRetry(tt.tries, tt.err))
		})
	}
}

type timeoutTestFixture struct {
	timeout               time.Duration
	responseHeaderTimeout time.Duration
	mockNetwork           *testNetwork
	executeCommand        bool
	expectedError         bool
}

func (f *timeoutTestFixture) setupCommand() *ArtifactsUploaderCommand {
	cmd := &ArtifactsUploaderCommand{
		JobCredentials:        UploaderCredentials,
		Timeout:               f.timeout,
		ResponseHeaderTimeout: f.responseHeaderTimeout,
		fileArchiver: fileArchiver{
			Paths: []string{artifactsTestArchivedFile},
		},
	}

	if f.mockNetwork != nil {
		cmd.newNetwork = createTestNewNetwork(f.mockNetwork)
	} else {
		// Use real network client creation to test timeout value propagation
		cmd.newNetwork = func() common.Network {
			return network.NewGitLabClient(
				network.WithHttpClientOptions(network.HttpClientOptions{
					Timeout:               &cmd.Timeout,
					ResponseHeaderTimeout: &cmd.ResponseHeaderTimeout,
				}),
			)
		}
	}

	return cmd
}

func TestArtifactsUploaderCommandTimeouts(t *testing.T) {
	tests := map[string]struct {
		fixture                       *timeoutTestFixture
		expectedTimeout               time.Duration
		expectedResponseHeaderTimeout time.Duration
		expectedUploadCalled          int
	}{
		"uses timeout values when creating network client": {
			fixture: &timeoutTestFixture{
				timeout:               time.Hour,
				responseHeaderTimeout: 10 * time.Minute,
				executeCommand:        false,
			},
			expectedTimeout:               time.Hour,
			expectedResponseHeaderTimeout: 10 * time.Minute,
		},
		"zero timeout values work": {
			fixture: &timeoutTestFixture{
				timeout:               0,
				responseHeaderTimeout: 0,
				executeCommand:        false,
			},
			expectedTimeout:               0,
			expectedResponseHeaderTimeout: 0,
		},
		"timeout values passed to network client when no injected network": {
			fixture: &timeoutTestFixture{
				timeout:               time.Minute,
				responseHeaderTimeout: 30 * time.Second,
				executeCommand:        true,
				expectedError:         true,
			},
			expectedTimeout:               time.Minute,
			expectedResponseHeaderTimeout: 30 * time.Second,
		},
		"injected network takes precedence over timeout values": {
			fixture: &timeoutTestFixture{
				timeout:               time.Hour,
				responseHeaderTimeout: 10 * time.Minute,
				mockNetwork: &testNetwork{
					uploadState: common.UploadSucceeded,
				},
				executeCommand: true,
				expectedError:  false,
			},
			expectedTimeout:               time.Hour,
			expectedResponseHeaderTimeout: 10 * time.Minute,
			expectedUploadCalled:          1,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			writeTestFile(t, artifactsTestArchivedFile)
			defer os.Remove(artifactsTestArchivedFile)

			cmd := tt.fixture.setupCommand()

			// Verify timeout values are set correctly
			assert.Equal(t, tt.expectedTimeout, cmd.Timeout)
			assert.Equal(t, tt.expectedResponseHeaderTimeout, cmd.ResponseHeaderTimeout)

			// Execute command if required by the test case
			if tt.fixture.executeCommand {
				err := cmd.enumerate()
				require.NoError(t, err)

				err = cmd.Run()

				if tt.fixture.expectedError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}

				if tt.fixture.mockNetwork != nil {
					assert.Equal(t, tt.expectedUploadCalled, tt.fixture.mockNetwork.uploadCalled)
				}
			}
		})
	}
}
