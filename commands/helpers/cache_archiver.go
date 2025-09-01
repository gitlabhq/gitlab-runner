package helpers

import (
	"context"
	"encoding/json"
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
	_ "gocloud.dev/blob/s3blob"    // Needed to register the AWS S3 driver
)

type CacheArchiverCommand struct {
	fileArchiver
	retryHelper
	meter.TransferMeterCommand

	File                   string   `long:"file" description:"The path to file"`
	URL                    string   `long:"url" description:"URL of remote cache resource (pre-signed URL)"`
	GoCloudURL             string   `long:"gocloud-url" description:"Go Cloud URL of remote cache resource (requires credentials)"`
	Timeout                int      `long:"timeout" description:"Overall timeout for cache uploading request (in minutes)"`
	Headers                []string `long:"header" description:"HTTP headers to send with PUT request (in form of 'key:value')"`
	Metadata               metadata `long:"metadata" env:"CACHE_METADATA" description:"Metadata for the cache artifact (JSON encoded key-value-pairs, e.g. '{\"foo\":\"bar\",\"blerp\":\"blip\"}')"`
	CompressionLevel       string   `long:"compression-level" env:"CACHE_COMPRESSION_LEVEL" description:"Compression level (fastest, fast, default, slow, slowest)"`
	CompressionFormat      string   `long:"compression-format" env:"CACHE_COMPRESSION_FORMAT" description:"Compression format (zip, tarzstd)"`
	MaxUploadedArchiveSize int64    `long:"max-uploaded-archive-size" env:"CACHE_MAX_UPLOADED_ARCHIVE_SIZE" description:"Limit the size of the cache archive being uploaded to cloud storage, in bytes."`
	EnvFile                string   `long:"env-file" description:"Filename containing environment variables to read"`

	client *CacheClient
	mux    *blob.URLMux
}

type metadata map[string]string

func (m *metadata) UnmarshalFlag(raw string) error {
	return json.Unmarshal([]byte(raw), m)
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

	err = loadEnvFile(c.EnvFile)
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

	opts := &blob.WriterOptions{
		Metadata: c.Metadata,
	}

	writer, err := b.NewWriter(ctx, objectName, opts)
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

	switch strings.ToLower(c.CompressionFormat) {
	case string(common.ArtifactFormatTarZstd):
		c.CompressionFormat = string(common.ArtifactFormatTarZstd)
	default:
		c.CompressionFormat = string(common.ArtifactFormatZip)
	}

	archiver, err := archive.NewArchiver(archive.Format(c.CompressionFormat), f, c.wd, GetCompressionLevel(c.CompressionLevel))
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

	err = writeCacheMetadataFile(c.File, c.Metadata)
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
	for k, v := range split(c.Headers) {
		req.Header.Set(strings.TrimSpace(k), strings.TrimSpace(v))
	}

	// Set default headers. But don't override custom Content-Type.
	if req.Header.Get(common.ContentType) == "" {
		req.Header.Set(common.ContentType, "application/octet-stream")
	}
	req.Header.Set("Last-Modified", fi.ModTime().UTC().Format(http.TimeFormat))
}

func split(raw []string) map[string]string {
	const sep = ":"

	data := make(map[string]string, len(raw))

	for _, s := range raw {
		k, v, ok := strings.Cut(s, sep)
		if !ok {
			continue
		}
		data[k] = v
	}

	return data
}

func init() {
	common.RegisterCommand(
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
