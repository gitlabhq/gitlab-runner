package helpers

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/archives"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/retry"
	"gitlab.com/gitlab-org/gitlab-runner/log"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

const (
	DefaultUploadName       = "default"
	defaultTries            = 3
	serviceUnavailableTries = 6
)

var (
	errServiceUnavailable = errors.New("service unavailable")
	errTooLarge           = errors.New("too large")
)

type ArtifactsUploaderCommand struct {
	common.JobCredentials
	fileArchiver
	network common.Network

	Name     string                `long:"name" description:"The name of the archive"`
	ExpireIn string                `long:"expire-in" description:"When to expire artifacts"`
	Format   common.ArtifactFormat `long:"artifact-format" description:"Format of generated artifacts"`
	Type     string                `long:"artifact-type" description:"Type of generated artifacts"`
}

func (c *ArtifactsUploaderCommand) generateZipArchive(w *io.PipeWriter) {
	err := archives.CreateZipArchive(w, c.sortedFiles())
	_ = w.CloseWithError(err)
}

func (c *ArtifactsUploaderCommand) generateGzipStream(w *io.PipeWriter) {
	err := archives.CreateGzipArchive(w, c.sortedFiles())
	_ = w.CloseWithError(err)
}

func (c *ArtifactsUploaderCommand) openRawStream() (io.ReadCloser, error) {
	fileNames := c.sortedFiles()
	if len(fileNames) > 1 {
		return nil, errors.New("only one file can be send as raw")
	}

	return os.Open(fileNames[0])
}

func (c *ArtifactsUploaderCommand) createReadStream() (string, io.ReadCloser, error) {
	if len(c.files) == 0 {
		return "", nil, nil
	}

	name := filepath.Base(c.Name)
	if name == "" || name == "." {
		name = DefaultUploadName
	}

	switch c.Format {
	case common.ArtifactFormatZip, common.ArtifactFormatDefault:
		pr, pw := io.Pipe()
		go c.generateZipArchive(pw)

		return name + ".zip", pr, nil

	case common.ArtifactFormatGzip:
		pr, pw := io.Pipe()
		go c.generateGzipStream(pw)

		return name + ".gz", pr, nil

	case common.ArtifactFormatRaw:
		file, err := c.openRawStream()

		return name, file, err

	default:
		return "", nil, fmt.Errorf("unsupported archive format: %s", c.Format)
	}
}

func (c *ArtifactsUploaderCommand) Run() error {
	artifactsName, stream, err := c.createReadStream()
	if err != nil {
		return err
	}
	if stream == nil {
		logrus.Errorln("No files to upload")

		return nil
	}
	defer func() { _ = stream.Close() }()

	// Create the archive
	options := common.ArtifactsOptions{
		BaseName: artifactsName,
		ExpireIn: c.ExpireIn,
		Format:   c.Format,
		Type:     c.Type,
	}

	// Upload the data
	switch c.network.UploadRawArtifacts(c.JobCredentials, stream, options) {
	case common.UploadSucceeded:
		return nil
	case common.UploadForbidden:
		return os.ErrPermission
	case common.UploadTooLarge:
		return errTooLarge
	case common.UploadFailed:
		return retryableErr{err: os.ErrInvalid}
	case common.UploadServiceUnavailable:
		return retryableErr{err: errServiceUnavailable}
	default:
		return os.ErrInvalid
	}
}

func (c *ArtifactsUploaderCommand) ShouldRetry(tries int, err error) bool {
	var errAs retryableErr
	if !errors.As(err, &errAs) {
		return false
	}

	maxTries := defaultTries
	if errors.Is(errAs, errServiceUnavailable) {
		maxTries = serviceUnavailableTries
	}

	if tries >= maxTries {
		return false
	}

	return true
}

func (c *ArtifactsUploaderCommand) Execute(*cli.Context) {
	log.SetRunnerFormatter()

	if c.URL == "" || c.Token == "" {
		logrus.Fatalln("Missing runner credentials")
	}
	if c.ID <= 0 {
		logrus.Fatalln("Missing build ID")
	}

	// Enumerate files
	err := c.enumerate()
	if err != nil {
		logrus.Fatalln(err)
	}

	// If the upload fails, exit with a non-zero exit code to indicate an issue?
	logger := logrus.WithField("context", "artifacts-uploader")
	retryable := retry.New(retry.WithLogrus(c, logger))
	err = retryable.Run()
	if err != nil {
		logrus.Fatalln(err)
	}
}

func init() {
	common.RegisterCommand2(
		"artifacts-uploader",
		"create and upload build artifacts (internal)",
		&ArtifactsUploaderCommand{
			network: network.NewGitLabClient(),
			Name:    "artifacts",
		},
	)
}
