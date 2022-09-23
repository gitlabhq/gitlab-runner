package helpers

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/meter"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/log"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

//nolint:lll
type ArtifactsDownloaderCommand struct {
	common.JobCredentials
	retryHelper
	network common.Network
	meter.TransferMeterCommand

	DirectDownload bool   `long:"direct-download" env:"FF_USE_DIRECT_DOWNLOAD" description:"Support direct download for data stored externally to GitLab"`
	StagingDir     string `long:"archiver-staging-dir" env:"ARCHIVER_STAGING_DIR" description:"Directory to stage artifact archives"`
}

func (c *ArtifactsDownloaderCommand) directDownloadFlag(retry int) *bool {
	// We want to send `?direct_download=true`
	// Use direct download only on a first attempt
	if c.DirectDownload && retry == 0 {
		return &c.DirectDownload
	}

	// We don't want to send `?direct_download=false`
	return nil
}

func (c *ArtifactsDownloaderCommand) download(file string, retry int) error {
	artifactsFile, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("creating target file: %w", err)
	}

	// Close() is checked properly inside of DownloadArtifacts() call
	defer func() { _ = artifactsFile.Close() }()

	writer := meter.NewWriter(
		artifactsFile,
		c.TransferMeterFrequency,
		meter.LabelledRateFormat(os.Stdout, "Downloading artifacts", meter.UnknownTotalSize),
	)

	// Close() is checked properly inside of DownloadArtifacts() call
	defer func() { _ = writer.Close() }()

	switch c.network.DownloadArtifacts(c.JobCredentials, writer, c.directDownloadFlag(retry)) {
	case common.DownloadSucceeded:
		return nil
	case common.DownloadNotFound:
		return os.ErrNotExist
	case common.DownloadForbidden, common.DownloadUnauthorized:
		return os.ErrPermission
	case common.DownloadFailed:
		return retryableErr{err: os.ErrInvalid}
	default:
		return os.ErrInvalid
	}
}

func (c *ArtifactsDownloaderCommand) Execute(cliContext *cli.Context) {
	log.SetRunnerFormatter()

	wd, err := os.Getwd()
	if err != nil {
		logrus.Fatalln("Unable to get working directory")
	}

	if c.URL == "" {
		logrus.Warningln("Missing URL (--url)")
	}
	if c.Token == "" {
		logrus.Warningln("Missing runner credentials (--token)")
	}
	if c.ID <= 0 {
		logrus.Warningln("Missing build ID (--id)")
	}
	if c.ID <= 0 || c.Token == "" || c.URL == "" {
		logrus.Fatalln("Incomplete arguments")
	}

	// Create temporary file
	file, err := os.CreateTemp(c.StagingDir, "artifacts")
	if err != nil {
		logrus.Fatalln(err)
	}
	_ = file.Close()
	defer func() { _ = os.Remove(file.Name()) }()

	// Download artifacts file
	err = c.doRetry(func(retry int) error {
		return c.download(file.Name(), retry)
	})
	if err != nil {
		logrus.Fatalln(err)
	}

	f, size, err := openZip(file.Name())
	if err != nil {
		logrus.Fatalln(err)
	}
	defer f.Close()

	extractor, err := archive.NewExtractor(archive.Zip, f, size, wd)
	if err != nil {
		logrus.Fatalln(err)
	}

	// Extract artifacts file
	err = extractor.Extract(context.Background())
	if err != nil {
		logrus.Fatalln(err)
	}
}

func openZip(filename string) (*os.File, int64, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, 0, err
	}

	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, 0, err
	}

	return f, fi.Size(), nil
}

func init() {
	common.RegisterCommand2(
		"artifacts-downloader",
		"download and extract build artifacts (internal)",
		&ArtifactsDownloaderCommand{
			network: network.NewGitLabClient(),
			retryHelper: retryHelper{
				Retry:     2,
				RetryTime: time.Second,
			},
		},
	)
}
