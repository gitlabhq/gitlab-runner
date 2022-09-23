package helpers

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/meter"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"
	"gitlab.com/gitlab-org/gitlab-runner/log"
)

type CacheExtractorCommand struct {
	retryHelper
	meter.TransferMeterCommand

	File    string `long:"file" description:"The file containing your cache artifacts"`
	URL     string `long:"url" description:"URL of remote cache resource"`
	Timeout int    `long:"timeout" description:"Overall timeout for cache downloading request (in minutes)"`

	client *CacheClient
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

	resp, err := c.getCache()
	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()

	upToDate, date := checkIfUpToDate(c.File, resp)
	if upToDate {
		logrus.Infoln(filepath.Base(c.File), "is up to date")
		return nil
	}

	file, err := os.CreateTemp(filepath.Dir(c.File), "cache")
	if err != nil {
		return err
	}

	defer func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}()

	logrus.Infoln("Downloading", filepath.Base(c.File), "from", url_helpers.CleanURL(c.URL))

	writer := meter.NewWriter(
		file,
		c.TransferMeterFrequency,
		meter.LabelledRateFormat(os.Stdout, "Downloading cache", getRemoteCacheSize(resp)),
	)

	// Close() is checked properly bellow, where the file handling is being finalized
	defer func() { _ = writer.Close() }()

	_, err = io.Copy(writer, resp.Body)
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

	err = os.Rename(file.Name(), c.File)
	if err != nil {
		return err
	}

	return nil
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

func (c *CacheExtractorCommand) Execute(cliContext *cli.Context) {
	log.SetRunnerFormatter()

	wd, err := os.Getwd()
	if err != nil {
		logrus.Fatalln("Unable to get working directory")
	}

	if c.File == "" {
		warningln("Missing cache file")
	}

	if c.URL != "" {
		err := c.doRetry(c.download)
		if err != nil {
			warningln(err)
		}
	} else {
		logrus.Infoln(
			"No URL provided, cache will not be downloaded from shared cache server. " +
				"Instead a local version of cache will be extracted.")
	}

	f, size, err := openZip(c.File)
	if os.IsNotExist(err) {
		return
	}
	if err != nil {
		logrus.Fatalln(err)
	}
	defer f.Close()

	extractor, err := archive.NewExtractor(archive.Zip, f, size, wd)
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
