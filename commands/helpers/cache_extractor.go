package helpers

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/archives"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/url"
	"gitlab.com/gitlab-org/gitlab-runner/log"
)

type CacheExtractorCommand struct {
	retryHelper
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

func (c *CacheExtractorCommand) download() (bool, error) {
	os.MkdirAll(filepath.Dir(c.File), 0700)

	file, err := ioutil.TempFile(filepath.Dir(c.File), "cache")
	if err != nil {
		return false, err
	}
	defer file.Close()
	defer os.Remove(file.Name())

	resp, err := c.getClient().Get(c.URL)
	if err != nil {
		return true, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return false, os.ErrNotExist
	} else if resp.StatusCode/100 != 2 {
		// Retry on server errors
		retry := resp.StatusCode/100 == 5
		return retry, fmt.Errorf("Received: %s", resp.Status)
	}

	fi, _ := os.Lstat(c.File)
	date, _ := time.Parse(http.TimeFormat, resp.Header.Get("Last-Modified"))
	if fi != nil && !date.After(fi.ModTime()) {
		logrus.Infoln(filepath.Base(c.File), "is up to date")
		return false, nil
	}

	logrus.Infoln("Downloading", filepath.Base(c.File), "from", url_helpers.CleanURL(c.URL))
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return true, err
	}
	os.Chtimes(file.Name(), time.Now(), date)

	err = os.Rename(file.Name(), c.File)
	if err != nil {
		return false, err
	}
	return false, nil
}

func (c *CacheExtractorCommand) Execute(context *cli.Context) {
	log.SetRunnerFormatter()

	if len(c.File) == 0 {
		logrus.Fatalln("Missing cache file")
	}

	if c.URL != "" {
		err := c.doRetry(c.download)
		if err != nil {
			logrus.Fatalln(err)
		}
	} else {
		logrus.Infoln("No URL provided, cache will not be downloaded from shared cache server. Instead a local version of cache will be extracted.")
	}

	err := archives.ExtractZipFile(c.File)
	if err != nil && !os.IsNotExist(err) {
		logrus.Fatalln(err)
	}
}

func init() {
	common.RegisterCommand2("cache-extractor", "download and extract cache artifacts (internal)", &CacheExtractorCommand{
		retryHelper: retryHelper{
			Retry:     2,
			RetryTime: time.Second,
		},
	})
}
