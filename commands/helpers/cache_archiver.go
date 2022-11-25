package helpers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"mvdan.cc/sh/v3/shell"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/meter"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"
	"gitlab.com/gitlab-org/gitlab-runner/log"

	"gocloud.dev/blob"
	_ "gocloud.dev/blob/azureblob" // Needed to register the Azure driver
)

//nolint:lll
type CacheArchiverCommand struct {
	fileArchiver
	retryHelper
	meter.TransferMeterCommand

	File                   string   `long:"file" description:"The path to file"`
	URL                    string   `long:"url" description:"URL of remote cache resource (pre-signed URL)"`
	GoCloudURL             string   `long:"gocloud-url" description:"Go Cloud URL of remote cache resource (requires credentials)"`
	Timeout                int      `long:"timeout" description:"Overall timeout for cache uploading request (in minutes)"`
	Headers                []string `long:"header" description:"HTTP headers to send with PUT request (in form of 'key:value')"`
	CompressionLevel       string   `long:"compression-level" env:"CACHE_COMPRESSION_LEVEL" description:"Compression level (fastest, fast, default, slow, slowest)"`
	MaxUploadedArchiveSize int64    `long:"max-uploaded-archive-size" env:"CACHE_MAX_UPLOADED_ARCHIVE_SIZE" description:"Limit the size of the cache archive being uploaded to cloud storage, in bytes."`

	client *CacheClient
	mux    *blob.URLMux
}

func (c *CacheArchiverCommand) getClient() *CacheClient {
	if c.client == nil {
		c.client = NewCacheClient(c.Timeout)
	}

	return c.client
}

func (c *CacheArchiverCommand) upload(_ int) error {
	file, err := os.Open(c.File)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	fi, err := file.Stat()
	if err != nil {
		return err
	}

	rc := meter.NewReader(
		file,
		c.TransferMeterFrequency,
		meter.LabelledRateFormat(os.Stdout, "Uploading cache", fi.Size()),
	)
	defer rc.Close()

	if c.GoCloudURL != "" {
		return c.handleGoCloudURL(rc)
	}

	return c.handlePresignedURL(fi, rc)
}

func (c *CacheArchiverCommand) handlePresignedURL(fi os.FileInfo, file io.Reader) error {
	logrus.Infoln("Uploading", filepath.Base(c.File), "to", url_helpers.CleanURL(c.URL))

	req, err := http.NewRequest(http.MethodPut, c.URL, file)
	if err != nil {
		return retryableErr{err: err}
	}

	c.setHeaders(req, fi)
	req.ContentLength = fi.Size()

	resp, err := c.getClient().Do(req)
	if err != nil {
		return retryableErr{err: err}
	}
	defer func() { _ = resp.Body.Close() }()

	return retryOnServerError(resp)
}

func (c *CacheArchiverCommand) handleGoCloudURL(file io.Reader) error {
	logrus.Infoln("Uploading", filepath.Base(c.File), "to", url_helpers.CleanURL(c.GoCloudURL))

	if c.mux == nil {
		c.mux = blob.DefaultURLMux()
	}

	ctx, cancelWrite := context.WithCancel(context.Background())
	defer cancelWrite()

	u, err := url.Parse(c.GoCloudURL)
	if err != nil {
		return err
	}

	objectName := strings.TrimLeft(u.Path, "/")
	if objectName == "" {
		return fmt.Errorf("no object name provided")
	}

	b, err := c.mux.OpenBucket(ctx, c.GoCloudURL)
	if err != nil {
		return err
	}
	defer b.Close()

	writer, err := b.NewWriter(ctx, objectName, nil)
	if err != nil {
		return err
	}

	if _, err = io.Copy(writer, file); err != nil {
		cancelWrite()
		if writerErr := writer.Close(); writerErr != nil {
			logrus.WithError(writerErr).Error("error closing Go cloud upload after copy failure")
		}
		return err
	}

	if err := writer.Close(); err != nil {
		return err
	}

	return nil
}

func (c *CacheArchiverCommand) createZipFile(filename string) (int64, error) {
	err := os.MkdirAll(filepath.Dir(filename), 0o700)
	if err != nil {
		return 0, err
	}

	f, err := os.CreateTemp(filepath.Dir(filename), "archive_")
	if err != nil {
		return 0, err
	}
	defer os.Remove(f.Name())
	defer f.Close()

	logrus.Debugln("Temporary file:", f.Name())

	archiver, err := archive.NewArchiver(archive.Zip, f, c.wd, GetCompressionLevel(c.CompressionLevel))
	if err != nil {
		return 0, err
	}

	// Create archive
	err = archiver.Archive(context.Background(), c.files)
	if err != nil {
		return 0, err
	}

	info, err := f.Stat()
	if err != nil {
		return 0, err
	}

	err = f.Close()
	if err != nil {
		return 0, err
	}

	return info.Size(), os.Rename(f.Name(), filename)
}

func (c *CacheArchiverCommand) Execute(*cli.Context) {
	log.SetRunnerFormatter()

	c.normalizeArgs()

	// Enumerate files
	err := c.enumerate()
	if err != nil {
		logrus.Fatalln(err)
	}

	// Check if list of files changed
	if !c.isFileChanged(c.File) {
		logrus.Infoln("Archive is up to date!")

		return
	}

	// Create archive
	size, err := c.createZipFile(c.File)
	if err != nil {
		logrus.Fatalln(err)
	}

	c.uploadArchiveIfNeeded(size)
}

func (c *CacheArchiverCommand) normalizeArgs() {
	if c.File == "" {
		logrus.Fatalln("Missing --file")
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

func (c *CacheArchiverCommand) uploadArchiveIfNeeded(size int64) {
	if c.URL == "" && c.GoCloudURL == "" {
		logrus.Infoln(
			"No URL provided, cache will not be uploaded to shared cache server. " +
				"Cache will be stored only locally.")
		return
	}

	if c.MaxUploadedArchiveSize != 0 && size > c.MaxUploadedArchiveSize {
		logrus.Infoln(fmt.Sprintf("Cache archive size (%d) is too big (Limit is set to %d). "+
			"Cache will be stored only locally.", size, c.MaxUploadedArchiveSize))
		return
	}

	err := c.doRetry(c.upload)
	if err != nil {
		logrus.Fatalln(err)
	}
}

func (c *CacheArchiverCommand) setHeaders(req *http.Request, fi os.FileInfo) {
	if len(c.Headers) > 0 {
		for _, header := range c.Headers {
			parsed := strings.SplitN(header, ":", 2)

			if len(parsed) != 2 {
				continue
			}

			req.Header.Set(strings.TrimSpace(parsed[0]), strings.TrimSpace(parsed[1]))
		}
	}

	// Set default headers. But don't override custom Content-Type.
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/octet-stream")
	}
	req.Header.Set("Last-Modified", fi.ModTime().Format(http.TimeFormat))
}

func init() {
	common.RegisterCommand2(
		"cache-archiver",
		"create and upload cache artifacts (internal)",
		&CacheArchiverCommand{
			retryHelper: retryHelper{
				Retry:     2,
				RetryTime: time.Second,
			},
		},
	)
}
