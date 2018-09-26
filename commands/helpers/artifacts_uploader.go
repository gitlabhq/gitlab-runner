package helpers

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/archives"
	"gitlab.com/gitlab-org/gitlab-runner/log"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

const DefaultUploadName = "default"

type ArtifactsUploaderCommand struct {
	common.JobCredentials
	fileArchiver
	retryHelper
	network common.Network

	Name     string                `long:"name" description:"The name of the archive"`
	ExpireIn string                `long:"expire-in" description:"When to expire artifacts"`
	Format   common.ArtifactFormat `long:"artifact-format" description:"Format of generated artifacts"`
	Type     string                `long:"artifact-type" description:"Type of generated artifacts"`
}

func (c *ArtifactsUploaderCommand) generateZipArchive(w *io.PipeWriter) {
	err := archives.CreateZipArchive(w, c.sortedFiles())
	w.CloseWithError(err)
}

func (c *ArtifactsUploaderCommand) generateGzipStream(w *io.PipeWriter) {
	err := archives.CreateGzipArchive(w, c.sortedFiles())
	w.CloseWithError(err)
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

	default:
		return "", nil, fmt.Errorf("unsupported archive format: %s", c.Format)
	}
}

func (c *ArtifactsUploaderCommand) createAndUpload() (bool, error) {
	artifactsName, stream, err := c.createReadStream()
	if err != nil {
		return false, err
	}
	if stream == nil {
		logrus.Errorln("No files to upload")
		return false, nil
	}
	defer stream.Close()

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
		return false, nil
	case common.UploadForbidden:
		return false, os.ErrPermission
	case common.UploadTooLarge:
		return false, errors.New("Too large")
	case common.UploadFailed:
		return true, os.ErrInvalid
	default:
		return false, os.ErrInvalid
	}
}

func (c *ArtifactsUploaderCommand) Execute(*cli.Context) {
	log.SetRunnerFormatter()

	if len(c.URL) == 0 || len(c.Token) == 0 {
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
	err = c.doRetry(c.createAndUpload)
	if err != nil {
		logrus.Fatalln(err)
	}
}

func init() {
	common.RegisterCommand2("artifacts-uploader", "create and upload build artifacts (internal)", &ArtifactsUploaderCommand{
		network: network.NewGitLabClient(),
		retryHelper: retryHelper{
			Retry:     2,
			RetryTime: time.Second,
		},
		Name: "artifacts",
	})
}
