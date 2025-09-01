package helpers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"mvdan.cc/sh/v3/shell"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/meter"
	"gitlab.com/gitlab-org/gitlab-runner/common"
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
	meter.TransferMeterCommand
	artifactStatementGenerator

	network common.Network

	Name             string                `long:"name" description:"The name of the archive"`
	ExpireIn         string                `long:"expire-in" description:"When to expire artifacts"`
	Format           common.ArtifactFormat `long:"artifact-format" description:"Format of generated artifacts"`
	Type             string                `long:"artifact-type" description:"Type of generated artifacts"`
	CompressionLevel string                `long:"compression-level" env:"ARTIFACT_COMPRESSION_LEVEL" description:"Compression level (fastest, fast, default, slow, slowest)"`
	CiDebugTrace     bool                  `long:"ci-debug-trace" env:"CI_DEBUG_TRACE" description:"enable debug trace logging"`
}

func (c *ArtifactsUploaderCommand) artifactFilename(name string, format common.ArtifactFormat) string {
	name = filepath.Base(name)
	if name == "" || name == "." {
		name = DefaultUploadName
	}

	switch format {
	case common.ArtifactFormatZip, common.ArtifactFormatZipZstd:
		return name + ".zip"

	case common.ArtifactFormatGzip:
		return name + ".gz"

	case common.ArtifactFormatTarZstd:
		return name + ".tar.zst"
	}
	return name
}

// createBodyProvider returns the artifact name and the stream provider for the request body.
func (c *ArtifactsUploaderCommand) createBodyProvider() (string, common.ContentProvider) {
	if len(c.files) == 0 {
		return "", nil
	}

	format := c.Format
	if format == common.ArtifactFormatDefault {
		format = common.ArtifactFormatZip
	}

	filename := c.artifactFilename(c.Name, format)

	// Create a StreamProvider that doesn't know its content length in advance
	streamProvider := common.StreamProvider{
		ReaderFactory: func() (io.ReadCloser, error) {
			pr, pw := io.Pipe()

			archiver, archiveErr := archive.NewArchiver(archive.Format(format), pw, c.wd, GetCompressionLevel(c.CompressionLevel))
			if archiveErr != nil {
				pr.CloseWithError(archiveErr)
				return nil, archiveErr
			}

			// Start a new Goroutine to create the archive for this attempt
			go func() {
				archiveErr := archiver.Archive(context.Background(), c.files)
				pw.CloseWithError(archiveErr)
			}()

			meteredReader := meter.NewReader(
				pr,
				c.TransferMeterFrequency,
				meter.LabelledRateFormat(os.Stdout, "Uploading artifacts", meter.UnknownTotalSize),
			)

			return meteredReader, nil
		},
	}

	return filename, streamProvider
}

func (c *ArtifactsUploaderCommand) Run() error {
	artifactsName, bodyProvider := c.createBodyProvider()
	if bodyProvider == nil {
		logrus.Errorln("No files to upload")
		return nil
	}

	// Create the archive
	options := common.ArtifactsOptions{
		BaseName:           artifactsName,
		ExpireIn:           c.ExpireIn,
		Format:             c.Format,
		Type:               c.Type,
		LogResponseDetails: c.CiDebugTrace,
	}

	// Upload the data
	resp, location := c.network.UploadRawArtifacts(c.JobCredentials, bodyProvider, options)
	switch resp {
	case common.UploadSucceeded:
		return nil
	case common.UploadRedirected:
		return c.handleRedirect(location)
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

func (c *ArtifactsUploaderCommand) handleRedirect(location string) error {
	newURL, err := url.Parse(location)
	if err != nil {
		return retryableErr{err: fmt.Errorf("parsing new location URL: %w", err)}
	}

	newURL.RawQuery = ""
	newURL.Path = ""

	c.JobCredentials.URL = newURL.String()

	logrus.WithField("location", location).
		WithField("new-url", c.JobCredentials.URL).
		Info("Upload request redirected")

	return retryableErr{err: fmt.Errorf("request redirected")}
}

func (c *ArtifactsUploaderCommand) shouldRetry(tries int, err error) bool {
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

	c.normalizeArgs()

	// Enumerate files
	err := c.enumerate()
	if err != nil {
		logrus.Fatalln(err)
	}

	if c.GenerateArtifactsMetadata {
		logrus.Infof("Generating artifacts statement")

		metadataFile, err := c.generateStatementToFile(generateStatementOptions{
			artifactName: c.Name,
			files:        c.files,
			artifactsWd:  c.wd,
			jobID:        c.ID,
		})
		if err != nil {
			logrus.Fatalln(err)
		}
		c.process(metadataFile)
	}

	// If the upload fails, exit with a non-zero exit code to indicate an issue?
	if err := retry.WithFn(c, c.Run).Run(); err != nil {
		logrus.Fatalln(err)
	}
}

func (c *ArtifactsUploaderCommand) NewRetry() *retry.Retry {
	return retry.
		New().
		WithCheck(c.shouldRetry).
		WithLogrus(logrus.WithField("context", "artifacts-uploader"))
}

func (c *ArtifactsUploaderCommand) normalizeArgs() {
	if c.URL == "" || c.Token == "" {
		logrus.Fatalln("Missing runner credentials")
	}
	if c.ID <= 0 {
		logrus.Fatalln("Missing build ID")
	}

	if name, err := shell.Expand(c.Name, nil); err != nil {
		logrus.Warnf("invalid artifact name: %v", err)
	} else {
		c.Name = name
	}

	for idx := range c.Paths {
		if path, err := shell.Expand(c.Paths[idx], nil); err != nil {
			logrus.Warnf("invalid path %q: %v", path, err)
		} else {
			c.Paths[idx] = path
		}
	}

	for idx := range c.Exclude {
		if path, err := shell.Expand(c.Exclude[idx], nil); err != nil {
			logrus.Warnf("invalid path %q: %v", path, err)
		} else {
			c.Exclude[idx] = path
		}
	}
}

func init() {
	common.RegisterCommand(
		"artifacts-uploader",
		"create and upload build artifacts (internal)",
		&ArtifactsUploaderCommand{
			network: network.NewGitLabClient(),
			Name:    "artifacts",
		},
	)
}
