//go:build integration

package helpers_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
	"github.com/urfave/cli"
	"gocloud.dev/blob"
	"gocloud.dev/blob/fileblob"

	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/commands/helpers/archive"
	testHelpers "gitlab.com/gitlab-org/gitlab-runner/helpers"
)

const cacheArchiverArchive = "archive.zip"
const cacheArchiverTestArchivedFile = "archive_file"
const cacheExtractorTestArchivedFile = "archive_file"

func TestCacheArchiverUploadExpandArgs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(testCacheUploadHandler))
	defer srv.Close()

	os.Setenv("expand", "expanded")
	defer os.Unsetenv("expand")

	cmd := helpers.CacheArchiverCommand{
		File:    cacheArchiverArchive,
		URL:     srv.URL + "/cache.zip",
		Timeout: 0,
	}
	cmd.Paths = []string{"unexpanded", "path/${expand}/${expand:1:3}"}
	cmd.Exclude = []string{"unexpanded", "path/$expand/${foo:-bar}"}

	cmd.Execute(&cli.Context{})

	assert.Equal(t, []string{"unexpanded", "path/expanded/xpa"}, cmd.Paths)
	assert.Equal(t, []string{"unexpanded", "path/expanded/bar"}, cmd.Exclude)
}

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
	mux, bucketDir := setupGoCloudFileBucket(t, "testblob")

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

			prevArchiver, _ := archive.Register(
				"zip",
				func(w io.Writer, dir string, level archive.CompressionLevel) (archive.Archiver, error) {
					assert.Equal(t, helpers.GetCompressionLevel(expectedLevel), level)
					return mockArchiver, nil
				},
				nil,
			)
			defer archive.Register(
				"zip",
				prevArchiver,
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

func setupGoCloudFileBucket(t *testing.T, scheme string) (m *blob.URLMux, bucketDir string) {
	tmpDir := t.TempDir()

	mux := new(blob.URLMux)
	fake := &dirOpener{tmpDir: tmpDir}
	mux.RegisterBucket(scheme, fake)

	return mux, tmpDir
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

	if r.Header.Get("Last-Modified") == "" {
		http.Error(w, "500 Expected Last-Modified header included", http.StatusInternalServerError)
	}
}

func writeTestFile(t *testing.T, fileName string) {
	err := os.WriteFile(fileName, nil, 0600)
	require.NoError(t, err, "Writing file:", fileName)
}

func TestCacheArchiverUploadedSize(t *testing.T) {
	tests := map[string]struct {
		limit    int
		exceeded bool
	}{
		"no-limit":    {limit: 0, exceeded: false},
		"above-limit": {limit: 10, exceeded: true},
		"equal-limit": {limit: 22, exceeded: false},
		"below-limit": {limit: 25, exceeded: false},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			defer logrus.SetOutput(logrus.StandardLogger().Out)
			defer testHelpers.MakeFatalToPanic()()

			var buf bytes.Buffer
			logrus.SetOutput(&buf)

			ts := httptest.NewServer(http.HandlerFunc(testCacheBaseUploadHandler))
			defer ts.Close()

			defer os.Remove(cacheArchiverArchive)
			cmd := helpers.CacheArchiverCommand{
				File:                   cacheArchiverArchive,
				MaxUploadedArchiveSize: int64(tc.limit),
				URL:                    ts.URL + "/cache.zip",
				Timeout:                0,
			}
			assert.NotPanics(t, func() {
				cmd.Execute(nil)
			})

			if tc.exceeded {
				require.Contains(t, buf.String(), "too big")
			} else {
				require.NotContains(t, buf.String(), "too big")
			}
		})
	}
}
