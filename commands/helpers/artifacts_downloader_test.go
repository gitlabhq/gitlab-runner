package helpers

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers"
	"os"
)

var downloaderCredentials = common.JobCredentials{
	ID:    1000,
	Token: "test",
	URL:   "test",
}

func TestArtifactsDownloaderRequirements(t *testing.T) {
	helpers.MakeFatalToPanic()

	cmd := ArtifactsDownloaderCommand{}
	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
}

func TestArtifactsDownloaderNotFound(t *testing.T) {
	network := &testNetwork{
		downloadState: common.DownloadNotFound,
	}
	cmd := ArtifactsDownloaderCommand{
		JobCredentials: downloaderCredentials,
		network:        network,
	}

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})

	assert.Equal(t, 1, network.downloadCalled)
}

func TestArtifactsDownloaderForbidden(t *testing.T) {
	network := &testNetwork{
		downloadState: common.DownloadForbidden,
	}
	cmd := ArtifactsDownloaderCommand{
		JobCredentials: downloaderCredentials,
		network:        network,
	}

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})

	assert.Equal(t, 1, network.downloadCalled)
}

func TestArtifactsDownloaderRetry(t *testing.T) {
	network := &testNetwork{
		downloadState: common.DownloadFailed,
	}
	cmd := ArtifactsDownloaderCommand{
		JobCredentials: downloaderCredentials,
		network:        network,
		retryHelper: retryHelper{
			Retry: 2,
		},
	}

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})

	assert.Equal(t, 3, network.downloadCalled)
}

func TestArtifactsDownloaderSucceeded(t *testing.T) {
	network := &testNetwork{
		downloadState: common.DownloadSucceeded,
	}
	cmd := ArtifactsDownloaderCommand{
		JobCredentials: downloaderCredentials,
		network:        network,
	}

	os.Remove(artifactsTestArchivedFile)
	fi, _ := os.Stat(artifactsTestArchivedFile)
	assert.Nil(t, fi)
	cmd.Execute(nil)
	assert.Equal(t, 1, network.downloadCalled)
	fi, _ = os.Stat(artifactsTestArchivedFile)
	assert.NotNil(t, fi)
}
