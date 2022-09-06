//go:build !integration

package helpers

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

const cacheExtractorArchive = "archive.zip"
const cacheExtractorTestArchivedFile = "archive_file"
const cacheExtractorTestFile = "test_file"

func TestCacheExtractorValidArchive(t *testing.T) {
	expectedContents := bytes.Repeat([]byte("198273qhnjbqwdjbqwe2109u3abcdef3"), 1024*1024)
	OnEachZipExtractor(t, func(t *testing.T) {
		file, err := os.Create(cacheExtractorArchive)
		assert.NoError(t, err)
		defer file.Close()
		defer os.Remove(file.Name())
		defer os.Remove(cacheExtractorTestArchivedFile)
		defer os.Remove(cacheExtractorTestFile)

		archive := zip.NewWriter(file)
		_, err = archive.Create(cacheExtractorTestArchivedFile)
		require.NoError(t, err)

		w, err := archive.Create(cacheExtractorTestFile)
		require.NoError(t, err)
		_, err = w.Write(expectedContents)
		require.NoError(t, err)

		archive.Close()

		_, err = os.Stat(cacheExtractorTestArchivedFile)
		require.Error(t, err)

		cmd := CacheExtractorCommand{
			File: cacheExtractorArchive,
		}
		assert.NotPanics(t, func() {
			cmd.Execute(nil)
		})

		_, err = os.Stat(cacheExtractorTestArchivedFile)
		assert.NoError(t, err)

		contents, err := os.ReadFile(cacheExtractorTestFile)
		assert.NoError(t, err)
		assert.Equal(t, expectedContents, contents)
	})
}

func TestCacheExtractorForInvalidArchive(t *testing.T) {
	OnEachZipExtractor(t, func(t *testing.T) {
		removeHook := helpers.MakeFatalToPanic()
		defer removeHook()
		writeTestFile(t, cacheExtractorArchive)
		defer os.Remove(cacheExtractorArchive)

		cmd := CacheExtractorCommand{
			File: cacheExtractorArchive,
		}
		assert.Panics(t, func() {
			cmd.Execute(nil)
		})
	})
}

func TestCacheExtractorForIfNoFileDefined(t *testing.T) {
	removeHook := helpers.MakeWarningToPanic()
	defer removeHook()
	cmd := CacheExtractorCommand{}
	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
}

func TestCacheExtractorForNotExistingFile(t *testing.T) {
	removeHook := helpers.MakeWarningToPanic()
	defer removeHook()
	cmd := CacheExtractorCommand{
		File: "/../../../test.zip",
	}
	assert.NotPanics(t, func() {
		cmd.Execute(nil)
	})
}

func testServeCache(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "408 Method not allowed", 408)
		return
	}
	if r.URL.Path != "/cache.zip" {
		if r.URL.Path == "/timeout" {
			time.Sleep(50 * time.Millisecond)
		}
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Last-Modified", time.Now().Format(http.TimeFormat))
	archive := zip.NewWriter(w)
	_, _ = archive.Create(cacheExtractorTestArchivedFile)
	archive.Close()
}

func TestCacheExtractorRemoteServerNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(testServeCache))
	defer ts.Close()

	removeHook := helpers.MakeWarningToPanic()
	defer removeHook()
	cmd := CacheExtractorCommand{
		File:    "non-existing-test.zip",
		URL:     ts.URL + "/invalid-file.zip",
		Timeout: 0,
	}
	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
	_, err := os.Stat(cacheExtractorTestArchivedFile)
	assert.Error(t, err)
}

func TestCacheExtractorRemoteServerTimedOut(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(testServeCache))
	defer ts.Close()

	output := logrus.StandardLogger().Out
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(output)
	removeHook := helpers.MakeWarningToPanic()
	defer removeHook()

	cmd := CacheExtractorCommand{
		File: "non-existing-test.zip",
		URL:  ts.URL + "/timeout",
	}
	cmd.getClient().Timeout = 1 * time.Millisecond

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
	assert.Contains(t, buf.String(), "Client.Timeout")

	_, err := os.Stat(cacheExtractorTestArchivedFile)
	assert.Error(t, err)
}

func TestCacheExtractorRemoteServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(testServeCache))
	defer ts.Close()

	defer os.Remove(cacheExtractorArchive)
	defer os.Remove(cacheExtractorTestArchivedFile)
	os.Remove(cacheExtractorArchive)
	os.Remove(cacheExtractorTestArchivedFile)

	removeHook := helpers.MakeWarningToPanic()
	defer removeHook()
	cmd := CacheExtractorCommand{
		File:    cacheExtractorArchive,
		URL:     ts.URL + "/cache.zip",
		Timeout: 0,
	}
	assert.NotPanics(t, func() {
		cmd.Execute(nil)
	})

	_, err := os.Stat(cacheExtractorTestArchivedFile)
	assert.NoError(t, err)

	err = os.Chtimes(cacheExtractorArchive, time.Now().Add(time.Hour), time.Now().Add(time.Hour))
	assert.NoError(t, err)

	assert.NotPanics(t, func() { cmd.Execute(nil) }, "archive is up to date")
}

func TestCacheExtractorRemoteServerFailOnInvalidServer(t *testing.T) {
	removeHook := helpers.MakeWarningToPanic()
	defer removeHook()
	os.Remove(cacheExtractorArchive)
	cmd := CacheExtractorCommand{
		File:    cacheExtractorArchive,
		URL:     "http://localhost:65333/cache.zip",
		Timeout: 0,
	}
	assert.Panics(t, func() {
		cmd.Execute(nil)
	})

	_, err := os.Stat(cacheExtractorTestArchivedFile)
	assert.Error(t, err)
}
