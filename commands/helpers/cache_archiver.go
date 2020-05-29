package helpers

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/archives"
	url_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/url"
	"gitlab.com/gitlab-org/gitlab-runner/log"
)

type CacheArchiverCommand struct {
	fileArchiver
	retryHelper
	File    string `long:"file" description:"The path to file"`
	URL     string `long:"url" description:"URL of remote cache resource"`
	Timeout int    `long:"timeout" description:"Overall timeout for cache uploading request (in minutes)"`

	client *CacheClient
}

func (c *CacheArchiverCommand) getClient() *CacheClient {
	if c.client == nil {
		c.client = NewCacheClient(c.Timeout)
	}

	return c.client
}

func (c *CacheArchiverCommand) upload(_ int) error {
	logrus.Infoln("Uploading", filepath.Base(c.File), "to", url_helpers.CleanURL(c.URL))

	file, err := os.Open(c.File)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	fi, err := file.Stat()
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, c.URL, file)
	if err != nil {
		return retryableErr{err: err}
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Last-Modified", fi.ModTime().Format(http.TimeFormat))
	req.ContentLength = fi.Size()

	resp, err := c.getClient().Do(req)
	if err != nil {
		return retryableErr{err: err}
	}
	defer func() { _ = resp.Body.Close() }()

	return retryOnServerError(resp)
}

func (c *CacheArchiverCommand) Execute(*cli.Context) {
	log.SetRunnerFormatter()

	if c.File == "" {
		logrus.Fatalln("Missing --file")
	}

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
	err = archives.CreateZipFile(c.File, c.sortedFiles())
	if err != nil {
		logrus.Fatalln(err)
	}

	// Upload archive if needed
	if c.URL != "" {
		err := c.doRetry(c.upload)
		if err != nil {
			logrus.Fatalln(err)
		}
	} else {
		logrus.Infoln(
			"No URL provided, cache will be not uploaded to shared cache server. " +
				"Cache will be stored only locally.")
	}
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
