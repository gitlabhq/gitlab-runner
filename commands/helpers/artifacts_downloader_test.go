//go:build !integration
// +build !integration

package helpers

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

var downloaderCredentials = common.JobCredentials{
	ID:    1000,
	Token: "test",
	URL:   "test",
}

func TestArtifactsDownloaderRequirements(t *testing.T) {
	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	cmd := ArtifactsDownloaderCommand{}
	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
}

func TestArtifactsDownloader(t *testing.T) {
	testCases := map[string]struct {
		downloadState                common.DownloadState
		directDownload               bool
		stagingDir                   string
		expectedSuccess              bool
		expectedDownloadCalled       int
		expectedDirectDownloadCalled int
	}{
		"download not found": {
			downloadState:          common.DownloadNotFound,
			expectedSuccess:        false,
			expectedDownloadCalled: 1,
		},
		"download forbidden": {
			downloadState:          common.DownloadForbidden,
			expectedSuccess:        false,
			expectedDownloadCalled: 1,
		},
		"retries are called": {
			downloadState:          common.DownloadFailed,
			expectedSuccess:        false,
			expectedDownloadCalled: 3,
		},
		"first try is always direct download": {
			downloadState:                common.DownloadFailed,
			directDownload:               true,
			expectedSuccess:              false,
			expectedDirectDownloadCalled: 1,
			expectedDownloadCalled:       3,
		},
		"downloads artifact without direct download if requested": {
			downloadState:                common.DownloadSucceeded,
			directDownload:               false,
			expectedSuccess:              true,
			expectedDirectDownloadCalled: 0,
			expectedDownloadCalled:       1,
		},
		"downloads artifact with direct download if requested": {
			downloadState:                common.DownloadSucceeded,
			directDownload:               true,
			expectedSuccess:              true,
			expectedDirectDownloadCalled: 1,
			expectedDownloadCalled:       1,
		},
		"setting invalid staging directory": {
			downloadState: common.DownloadSucceeded,
			stagingDir:    "/dev/null",
		},
	}

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	// ensure clean state
	os.Remove(artifactsTestArchivedFile)

	for testName, testCase := range testCases {
		OnEachZipArchiver(t, func(t *testing.T) {
			t.Run(testName, func(t *testing.T) {
				network := &testNetwork{
					downloadState: testCase.downloadState,
				}
				cmd := ArtifactsDownloaderCommand{
					JobCredentials: downloaderCredentials,
					DirectDownload: testCase.directDownload,
					network:        network,
					retryHelper: retryHelper{
						Retry: 2,
					},
					StagingDir: testCase.stagingDir,
				}

				// file is cleaned after running test
				defer os.Remove(artifactsTestArchivedFile)

				if testCase.expectedSuccess {
					require.NotPanics(t, func() {
						cmd.Execute(nil)
					})

					assert.FileExists(t, artifactsTestArchivedFile)
				} else {
					require.Panics(t, func() {
						cmd.Execute(nil)
					})
				}

				assert.Equal(t, testCase.expectedDirectDownloadCalled, network.directDownloadCalled)
				assert.Equal(t, testCase.expectedDownloadCalled, network.downloadCalled)
			})
		})
	}
}
