package helpers

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/archives"
	"gitlab.com/gitlab-org/gitlab-runner/log"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

type ArtifactsDownloaderCommand struct {
	common.JobCredentials
	retryHelper
	network common.Network

	DirectDownload bool `long:"direct-download" env:"FF_USE_DIRECT_DOWNLOAD" description:"Support direct download for data stored externally to GitLab"`
}

func (c *ArtifactsDownloaderCommand) directDownloadFlag() *bool {
	// We want to send `?direct_download=true`
	if c.DirectDownload {
		return &c.DirectDownload
	}

	// We don't want to send `?direct_download=false`
	return nil
}

func (c *ArtifactsDownloaderCommand) download(file string) error {
	switch c.network.DownloadArtifacts(c.JobCredentials, file, c.directDownloadFlag()) {
	case common.DownloadSucceeded:
		return nil
	case common.DownloadNotFound:
		return os.ErrNotExist
	case common.DownloadForbidden:
		return os.ErrPermission
	case common.DownloadFailed:
		return retryableErr{err: os.ErrInvalid}
	default:
		return os.ErrInvalid
	}
}

func (c *ArtifactsDownloaderCommand) Execute(context *cli.Context) {
	log.SetRunnerFormatter()

	if len(c.URL) == 0 || len(c.Token) == 0 {
		logrus.Fatalln("Missing runner credentials")
	}
	if c.ID <= 0 {
		logrus.Fatalln("Missing build ID")
	}

	// Create temporary file
	file, err := ioutil.TempFile("", "artifacts")
	if err != nil {
		logrus.Fatalln(err)
	}
	file.Close()
	defer os.Remove(file.Name())

	// Download artifacts file
	err = c.doRetry(func() error {
		return c.download(file.Name())
	})
	if err != nil {
		logrus.Fatalln(err)
	}

	// Extract artifacts file
	err = archives.ExtractZipFile(file.Name())
	if err != nil {
		logrus.Fatalln(err)
	}
}

func init() {
	common.RegisterCommand2("artifacts-downloader", "download and extract build artifacts (internal)", &ArtifactsDownloaderCommand{
		network: network.NewGitLabClient(),
		retryHelper: retryHelper{
			Retry:     2,
			RetryTime: time.Second,
		},
	})
}
