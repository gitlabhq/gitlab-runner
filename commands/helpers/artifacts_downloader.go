package helpers

import (
	"io/ioutil"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/archives"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/formatter"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

type ArtifactsDownloaderCommand struct {
	common.JobCredentials
	retryHelper
	network common.Network
}

func (c *ArtifactsDownloaderCommand) download(file string) (bool, error) {
	switch c.network.DownloadArtifacts(c.JobCredentials, file) {
	case common.DownloadSucceeded:
		return false, nil
	case common.DownloadNotFound:
		return false, os.ErrNotExist
	case common.DownloadForbidden:
		return false, os.ErrPermission
	case common.DownloadFailed:
		return true, os.ErrInvalid
	default:
		return false, os.ErrInvalid
	}
}

func (c *ArtifactsDownloaderCommand) Execute(context *cli.Context) {
	formatter.SetRunnerFormatter()

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
	err = c.doRetry(func() (bool, error) {
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
