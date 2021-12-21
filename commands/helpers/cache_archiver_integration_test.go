//go:build integration
// +build integration

package helpers_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob"
	"gocloud.dev/blob/fileblob"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
	testHelpers "gitlab.com/gitlab-org/gitlab-runner/helpers"
)

const cacheArchiverArchive = "archive.zip"
const cacheArchiverTestArchivedFile = "archive_file"
const cacheExtractorTestArchivedFile = "archive_file"

func TestCacheArchiverIsUpToDate(t *testing.T) {
	helpers.OnEachZipArchiver(t, func(t *testing.T) {
		writeTestFile(t, cacheArchiverTestArchivedFile)
		defer os.Remove(cacheArchiverTestArchivedFile)

		defer os.Remove(cacheArchiverArchive)
		cmd := helpers.NewCacheArchiverCommandForTest(cacheArchiverArchive, []string{cacheArchiverTestArchivedFile})
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
	})
}

func TestCacheArchiverForIfNoFileDefined(t *testing.T) {
	removeHook := testHelpers.MakeFatalToPanic()
	defer removeHook()
	cmd := helpers.CacheArchiverCommand{}
	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
}

func TestCacheArchiverRemoteServerNotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(testCacheUploadHandler))
	defer ts.Close()

	removeHook := testHelpers.MakeFatalToPanic()
	defer removeHook()
	defer os.Remove(cacheArchiverArchive)
	cmd := helpers.CacheArchiverCommand{
		File:    cacheArchiverArchive,
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

	removeHook := testHelpers.MakeFatalToPanic()
	defer removeHook()
	defer os.Remove(cacheArchiverArchive)
	cmd := helpers.CacheArchiverCommand{
		File:    cacheArchiverArchive,
		URL:     ts.URL + "/cache.zip",
		Timeout: 0,
	}
	assert.NotPanics(t, func() {
		cmd.Execute(nil)
	})
}

func TestCacheArchiverGoCloudRemoteServer(t *testing.T) {
	mux, bucketDir, cleanup := setupGoCloudFileBucket(t, "testblob")
	defer cleanup()

	objectName := "path/to/cache.zip"

	removeHook := testHelpers.MakeFatalToPanic()
	defer removeHook()
	defer os.Remove(cacheArchiverArchive)
	cmd := helpers.CacheArchiverCommand{
		File:       cacheArchiverArchive,
		GoCloudURL: fmt.Sprintf("testblob://bucket/" + objectName),
		Timeout:    0,
	}
	helpers.SetCacheArchiverCommandMux(&cmd, mux)
	assert.NotPanics(t, func() {
		cmd.Execute(nil)
	})

	goCloudObjectExists(t, bucketDir, objectName)
}

func TestCacheArchiverRemoteServerWithHeaders(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(testCacheUploadWithCustomHeaders))
	defer ts.Close()

	removeHook := testHelpers.MakeFatalToPanic()
	defer removeHook()
	defer os.Remove(cacheArchiverArchive)
	cmd := helpers.CacheArchiverCommand{
		File:    cacheArchiverArchive,
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
	removeHook := testHelpers.MakeFatalToPanic()
	defer removeHook()

	defer os.Remove(cacheArchiverArchive)
	cmd := helpers.CacheArchiverCommand{
		File: cacheArchiverArchive,
		URL:  ts.URL + "/timeout",
	}
	helpers.SetCacheArchiverCommandClientTimeout(&cmd, 1*time.Millisecond)

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
	assert.Contains(t, buf.String(), "Client.Timeout")
}

func TestCacheArchiverRemoteServerFailOnInvalidServer(t *testing.T) {
	removeHook := testHelpers.MakeFatalToPanic()
	defer removeHook()
	defer os.Remove(cacheArchiverArchive)
	cmd := helpers.CacheArchiverCommand{
		File:    cacheArchiverArchive,
		URL:     "http://localhost:65333/cache.zip",
		Timeout: 0,
	}
	assert.Panics(t, func() {
		cmd.Execute(nil)
	})

	_, err := os.Stat(cacheExtractorTestArchivedFile)
	assert.Error(t, err)
}

func TestCacheArchiverCompressionLevel(t *testing.T) {
	writeTestFile(t, cacheArchiverTestArchivedFile)
	defer os.Remove(cacheArchiverTestArchivedFile)

	for _, expectedLevel := range []string{"fastest", "fast", "default", "slow", "slowest"} {
		t.Run(expectedLevel, func(t *testing.T) {
			mockArchiver := new(archive.MockArchiver)
			defer mockArchiver.AssertExpectations(t)

			archive.Register(
				"zip",
				func(w io.Writer, dir string, level archive.CompressionLevel) (archive.Archiver, error) {
					assert.Equal(t, helpers.GetCompressionLevel(expectedLevel), level)
					return mockArchiver, nil
				},
				nil,
			)

			mockArchiver.On("Archive", mock.Anything, mock.Anything).Return(nil)

			defer os.Remove(cacheArchiverArchive)
			cmd := helpers.NewCacheArchiverCommandForTest(cacheArchiverArchive, []string{cacheArchiverTestArchivedFile})
			cmd.CompressionLevel = expectedLevel
			cmd.Execute(nil)
		})
	}
}

type dirOpener struct {
	tmpDir string
}

func (o *dirOpener) OpenBucketURL(_ context.Context, u *url.URL) (*blob.Bucket, error) {
	return fileblob.OpenBucket(o.tmpDir, nil)
}

func setupGoCloudFileBucket(t *testing.T, scheme string) (m *blob.URLMux, bucketDir string, cleanup func()) {
	tmpDir, err := ioutil.TempDir("", "test-bucket")
	require.NoError(t, err)

	mux := new(blob.URLMux)
	fake := &dirOpener{tmpDir: tmpDir}
	mux.RegisterBucket(scheme, fake)
	cleanup = func() {
		os.RemoveAll(tmpDir)
	}

	return mux, tmpDir, cleanup
}

func goCloudObjectExists(t *testing.T, bucketDir string, objectName string) {
	bucket, err := fileblob.OpenBucket(bucketDir, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exists, err := bucket.Exists(ctx, objectName)
	require.NoError(t, err)
	assert.True(t, exists)
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

func writeTestFile(t *testing.T, fileName string) {
	err := ioutil.WriteFile(fileName, nil, 0600)
	require.NoError(t, err, "Writing file:", fileName)
}
