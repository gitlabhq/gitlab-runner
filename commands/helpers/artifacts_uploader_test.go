package helpers

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"io/ioutil"

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

	ioutil.WriteFile(artifactsTestArchivedFile, nil, 0600)
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

	ioutil.WriteFile(artifactsTestArchivedFile, nil, 0600)
	defer os.Remove(artifactsTestArchivedFile)

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})

	assert.Equal(t, 1, network.uploadCalled)
}

func TestArtifactsUploaderRetry(t *testing.T) {
	network := &testNetwork{
		uploadState: common.UploadFailed,
	}
	cmd := ArtifactsUploaderCommand{
		JobCredentials: UploaderCredentials,
		network:        network,
		retryHelper: retryHelper{
			Retry: 2,
		},
		fileArchiver: fileArchiver{
			Paths: []string{artifactsTestArchivedFile},
		},
	}

	ioutil.WriteFile(artifactsTestArchivedFile, nil, 0600)
	defer os.Remove(artifactsTestArchivedFile)

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})

	assert.Equal(t, 3, network.uploadCalled)
}

func TestArtifactsUploaderDefaultSucceeded(t *testing.T) {
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

	ioutil.WriteFile(artifactsTestArchivedFile, nil, 0600)
	defer os.Remove(artifactsTestArchivedFile)

	cmd.Execute(nil)
	assert.Equal(t, 1, network.uploadCalled)
	assert.Equal(t, common.ArtifactFormatZip, network.uploadFormat)
	assert.Equal(t, DefaultUploadName+".zip", network.uploadName)
	assert.Empty(t, network.uploadType)
}

func TestArtifactsUploaderZipSucceeded(t *testing.T) {
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

	ioutil.WriteFile(artifactsTestArchivedFile, nil, 0600)
	defer os.Remove(artifactsTestArchivedFile)

	cmd.Execute(nil)
	assert.Equal(t, 1, network.uploadCalled)
	assert.Equal(t, common.ArtifactFormatZip, network.uploadFormat)
	assert.Equal(t, "my-release.zip", network.uploadName)
	assert.Equal(t, "my-type", network.uploadType)
	assert.Contains(t, network.uploadedFiles, artifactsTestArchivedFile)
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

	ioutil.WriteFile(artifactsTestArchivedFile, nil, 0600)
	defer os.Remove(artifactsTestArchivedFile)

	ioutil.WriteFile(artifactsTestArchivedFile2, nil, 0600)
	defer os.Remove(artifactsTestArchivedFile)

	cmd.Execute(nil)
	assert.Equal(t, 1, network.uploadCalled)
	assert.Equal(t, "junit.xml.gz", network.uploadName)
	assert.Equal(t, common.ArtifactFormatGzip, network.uploadFormat)
	assert.Equal(t, "junit", network.uploadType)
	assert.Contains(t, network.uploadedFiles, artifactsTestArchivedFile)
	assert.Contains(t, network.uploadedFiles, artifactsTestArchivedFile2)
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
