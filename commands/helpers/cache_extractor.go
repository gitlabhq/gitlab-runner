package helpers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/meter"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"
	"gitlab.com/gitlab-org/gitlab-runner/log"

	"gocloud.dev/blob"
	_ "gocloud.dev/blob/azureblob" // Needed to register the Azure driver
)

type CacheExtractorCommand struct {
	retryHelper
	meter.TransferMeterCommand

	File       string `long:"file" description:"The file containing your cache artifacts"`
	URL        string `long:"url" description:"URL of remote cache resource"`
	GoCloudURL string `long:"gocloud-url" description:"Go Cloud URL of remote cache resource (requires credentials)"`
	Timeout    int    `long:"timeout" description:"Overall timeout for cache downloading request (in minutes)"`

	client *CacheClient
	mux    *blob.URLMux
}

func (c *CacheExtractorCommand) getClient() *CacheClient {
	if c.client == nil {
		c.client = NewCacheClient(c.Timeout)
	}

	return c.client
}

func checkIfUpToDate(path string, resp *http.Response) (bool, time.Time) {
	fi, _ := os.Lstat(path)
	date, _ := time.Parse(http.TimeFormat, resp.Header.Get("Last-Modified"))
	return fi != nil && !date.After(fi.ModTime()), date
}

func getRemoteCacheSize(resp *http.Response) int64 {
	length, _ := strconv.Atoi(resp.Header.Get("Content-Length"))
	if length <= 0 {
		return meter.UnknownTotalSize
	}

	return int64(length)
}

func (c *CacheExtractorCommand) download(_ int) error {
	err := os.MkdirAll(filepath.Dir(c.File), 0o700)
	if err != nil {
		return err
	}

	if c.GoCloudURL != "" {
		return c.handleGoCloudURL()
	}

	return c.handlePresignedURL()
}

func (c *CacheExtractorCommand) getCache() (*http.Response, error) {
	resp, err := c.getClient().Get(c.URL)
	if err != nil {
		return nil, retryableErr{err: err}
	}

	if resp.StatusCode == http.StatusNotFound {
		_ = resp.Body.Close()
		return nil, os.ErrNotExist
	}

	return resp, retryOnServerError(resp)
}

func (c *CacheExtractorCommand) handlePresignedURL() error {
	resp, err := c.getCache()
	if err != nil {
		return err
	}

	// Close() is checked properly below, where the file handling is being finalized
	defer func() { _ = resp.Body.Close() }()

	upToDate, date := checkIfUpToDate(c.File, resp)
	if upToDate {
		logrus.Infoln(filepath.Base(c.File), "is up to date")
		return nil
	}

	etag := resp.Header.Get("ETag")
	cleanedURL := url_helpers.CleanURL(c.URL)
	contentLength := getRemoteCacheSize(resp)

	return c.downloadAndSaveCache(resp.Body, date, etag, cleanedURL, contentLength)
}

func (c *CacheExtractorCommand) handleGoCloudURL() error {
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

	attrs, err := b.Attributes(ctx, objectName)
	if err != nil {
		return err
	}

	reader, err := b.NewReader(ctx, objectName, nil)
	if err != nil {
		return err
	}
	defer reader.Close()

	cleanedURL := url_helpers.CleanURL(c.GoCloudURL)

	return c.downloadAndSaveCache(reader, attrs.ModTime, attrs.ETag, cleanedURL, attrs.Size)
}

func (c *CacheExtractorCommand) downloadAndSaveCache(reader io.Reader, date time.Time, etag, cleanedURL string, contentLength int64) error {
	file, err := os.CreateTemp(filepath.Dir(c.File), "cache")
	if err != nil {
		return err
	}

	defer func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}()

	// For legacy purposes, caches written to disk use the extension `.zip`
	// even when a different compression format is used. To avoid confusion,
	// we avoid the extension name in logs.
	name := strings.TrimSuffix(filepath.Base(c.File), filepath.Ext(c.File))

	if etag != "" {
		logrus.WithField("ETag", etag).Infoln("Downloading", name, "from", cleanedURL)
	} else {
		logrus.Infoln("Downloading", name, "from", cleanedURL)
	}

	writer := meter.NewWriter(
		file,
		c.TransferMeterFrequency,
		meter.LabelledRateFormat(os.Stdout, "Downloading cache", contentLength),
	)

	defer func() { _ = writer.Close() }()

	_, err = io.Copy(writer, reader)
	if err != nil {
		return retryableErr{err: err}
	}

	err = os.Chtimes(file.Name(), time.Now(), date)
	if err != nil {
		return err
	}

	err = writer.Close()
	if err != nil {
		return err
	}

	return os.Rename(file.Name(), c.File)
}

func (c *CacheExtractorCommand) Execute(cliContext *cli.Context) {
	log.SetRunnerFormatter()

	wd, err := os.Getwd()
	if err != nil {
		logrus.Fatalln("Unable to get working directory")
	}

	if c.File == "" {
		warningln("Missing cache file")
	}

	if c.URL != "" || c.GoCloudURL != "" {
		err := c.doRetry(c.download)
		if err != nil {
			warningln(err)
		}
	} else {
		logrus.Infoln(
			"No URL provided, cache will not be downloaded from shared cache server. " +
				"Instead a local version of cache will be extracted.")
	}

	f, size, format, err := openArchive(c.File)
	if os.IsNotExist(err) {
		warningln("Cache file does not exist")
	}
	if err != nil {
		logrus.Fatalln(err)
	}
	defer f.Close()

	extractor, err := archive.NewExtractor(format, f, size, wd)
	if err != nil {
		logrus.Fatalln(err)
	}

	err = extractor.Extract(context.Background())
	if err != nil {
		logrus.Fatalln(err)
	}
}

func warningln(args interface{}) {
	logrus.Warningln(args)
	logrus.Exit(1)
}

func init() {
	common.RegisterCommand2(
		"cache-extractor",
		"download and extract cache artifacts (internal)",
		&CacheExtractorCommand{
			retryHelper: retryHelper{
				Retry:     2,
				RetryTime: time.Second,
			},
		},
	)
}
