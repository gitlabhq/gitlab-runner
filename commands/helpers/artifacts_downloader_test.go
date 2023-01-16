//go:build !integration

package helpers

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"

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
		"download unauthorized": {
			downloadState:          common.DownloadUnauthorized,
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

// Some version of urfave have a bug that causes it to balk when the value of an
// argument starts with a `-`. This test is here to ensure we don't up/down
// grade to version of urfave with this bug.
// See https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29448 and
// https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29193
func Test_URFavArgParsing(t *testing.T) {
	app := cli.NewApp()
	app.Name = "gitlab-runner-helper"
	app.Usage = "a GitLab Runner Helper"
	app.Version = common.AppVersion.ShortLine()
	app.Commands = common.GetCommands()

	jobToken := "-Abajdbajdbajb"

	defer os.Remove("foo.txt")

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, jobToken, r.Header.Get("Job-Token"))

		w.WriteHeader(http.StatusOK)
		zw := zip.NewWriter(w)
		defer zw.Close()
		w1, err := zw.Create("foo.txt")
		require.NoError(t, err)
		_, err = w1.Write(bytes.Repeat([]byte("198273qhnjbqwdjbqwe2109u3abcdef3"), 1024*1024))
		require.NoError(t, err)
	}))
	defer s.Close()

	args := []string{
		"gitlab-runner-helper",
		"artifacts-downloader",
		"--url", s.URL,
		"--token", jobToken,
		"--id", "12345",
	}

	err := app.Run(args)
	assert.NoError(t, err)

	if err != nil {
		assert.NotContains(t, err.Error(), "WARNING: Missing build ID (--id)")
		assert.NotContains(t, err.Error(), "FATAL: Incomplete arguments ")
	}
}
