package helpers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

const cacheArchiverArchive = "archive.zip"
const cacheArchiverTestArchivedFile = "archive_file"

func TestCacheArchiverIsUpToDate(t *testing.T) {
	writeTestFile(t, cacheArchiverTestArchivedFile)
	defer os.Remove(cacheArchiverTestArchivedFile)

	defer os.Remove(cacheArchiverArchive)
	cmd := CacheArchiverCommand{
		File: cacheArchiverArchive,
		fileArchiver: fileArchiver{
			Paths: []string{
				cacheArchiverTestArchivedFile,
			},
		},
	}
	cmd.Execute(nil)
	fi, _ := os.Stat(cacheArchiverArchive)
	cmd.Execute(nil)
	fi2, _ := os.Stat(cacheArchiverArchive)
	assert.Equal(t, fi.ModTime(), fi2.ModTime(), "archive is up to date")

	// We need to wait one second, since the FS doesn't save milliseconds
	time.Sleep(time.Second)

	err := os.Chtimes(cacheArchiverTestArchivedFile, time.Now(), time.Now())
	assert.NoError(t, err)

	cmd.Execute(nil)
	fi3, _ := os.Stat(cacheArchiverArchive)
	assert.NotEqual(t, fi.ModTime(), fi3.ModTime(), "archive should get updated")
}

func TestCacheArchiverForIfNoFileDefined(t *testing.T) {
	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()
	cmd := CacheArchiverCommand{}
	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
}

func testCacheBaseUploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "405 Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.URL.Path != "/cache.zip" {
		if r.URL.Path == "/timeout" {
			time.Sleep(50 * time.Millisecond)
		}
		http.NotFound(w, r)
		return
	}
}

func testCacheUploadHandler(w http.ResponseWriter, r *http.Request) {
	testCacheBaseUploadHandler(w, r)

	if r.Header.Get("Content-Type") != "application/octet-stream" {
		http.Error(w, "500 Wrong Content-Type header", http.StatusInternalServerError)
		return
	}
	if r.Header.Get("Last-Modified") == "" {
		http.Error(w, "500 Missing Last-Modified header", http.StatusInternalServerError)
		return
	}
}

func testCacheUploadWithCustomHeaders(w http.ResponseWriter, r *http.Request) {
	testCacheBaseUploadHandler(w, r)

	if r.Header.Get("Content-Type") != "application/zip" {
		http.Error(w, "500 Wrong Content-Type header", http.StatusInternalServerError)
	}

	if r.Header.Get("x-ms-blob-type") != "BlockBlob" {
		http.Error(w, "500 Wrong x-ms-blob-type header", http.StatusInternalServerError)
	}

	if r.Header.Get("Last-Modified") != "" {
		http.Error(w, "500 Unexpected Last-Modified header included", http.StatusInternalServerError)
	}
}

func TestCacheArchiverRemoteServerNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(testCacheUploadHandler))
	defer ts.Close()

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()
	os.Remove(cacheExtractorArchive)
	cmd := CacheArchiverCommand{
		File:    cacheExtractorArchive,
		URL:     ts.URL + "/invalid-file.zip",
		Timeout: 0,
	}
	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
}

func TestCacheArchiverRemoteServer(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(testCacheUploadHandler))
	defer ts.Close()

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()
	os.Remove(cacheExtractorArchive)
	cmd := CacheArchiverCommand{
		File:    cacheExtractorArchive,
		URL:     ts.URL + "/cache.zip",
		Timeout: 0,
	}
	assert.NotPanics(t, func() {
		cmd.Execute(nil)
	})
}

func TestCacheArchiverRemoteServerWithHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(testCacheUploadWithCustomHeaders))
	defer ts.Close()

	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()
	os.Remove(cacheExtractorArchive)
	cmd := CacheArchiverCommand{
		File:    cacheExtractorArchive,
		URL:     ts.URL + "/cache.zip",
		Headers: []string{"Content-Type: application/zip", "x-ms-blob-type:   BlockBlob "},
		Timeout: 0,
	}
	assert.NotPanics(t, func() {
		cmd.Execute(nil)
	})
}

func TestCacheArchiverRemoteServerTimedOut(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(testCacheUploadHandler))
	defer ts.Close()

	output := logrus.StandardLogger().Out
	var buf bytes.Buffer
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(output)
	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	os.Remove(cacheExtractorArchive)
	cmd := CacheArchiverCommand{
		File: cacheExtractorArchive,
		URL:  ts.URL + "/timeout",
	}
	cmd.getClient().Timeout = 1 * time.Millisecond

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
	assert.Contains(t, buf.String(), "Client.Timeout")
}

func TestCacheArchiverRemoteServerFailOnInvalidServer(t *testing.T) {
	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()
	os.Remove(cacheExtractorArchive)
	cmd := CacheArchiverCommand{
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
