//go:build !integration

package helpers

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob"
	"gocloud.dev/blob/fileblob"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

const (
	cacheExtractorArchive          = "archive.zip"
	cacheExtractorMetadata         = "metadata.json"
	cacheExtractorTestArchivedFile = "archive_file"
	cacheExtractorTestFile         = "test_file"
)

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

func writeZipFile(t *testing.T, filename string) {
	var buf bytes.Buffer

	zipWriter := zip.NewWriter(&buf)
	f, err := zipWriter.Create(cacheExtractorTestArchivedFile)
	require.NoError(t, err)

	_, err = io.WriteString(f, "This is a test.")
	require.NoError(t, err)

	err = zipWriter.Close()
	require.NoError(t, err)

	outFile, err := os.Create(filename)
	require.NoError(t, err)
	defer outFile.Close()

	_, err = buf.WriteTo(outFile)
	if err != nil {
		require.NoError(t, err)
	}
}

func writeZipFileAndMetadata(t *testing.T, filename string) {
	writeZipFile(t, filename)

	attrFile := filename + ".attrs"

	json, err := json.Marshal(map[string]any{
		"user.metadata": map[string]string{
			"foo":   "some foo",
			"blank": "",
		},
	})
	require.NoError(t, err, "marshaling blob attributes")

	err = os.WriteFile(attrFile, json, 0640)
	require.NoError(t, err, "writing blob attributes sidecar file")
}

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
	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
}

func testServeCacheWithETag(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("ETag", "some-etag")
	testServeCache(w, r)
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

	w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
	w.Header().Set("x-fakeCloud-meta-foo", "some foo")
	w.Header().Set("x-random", "ignored")
	w.Header().Set("x-fakeClound-meta-blank", "")
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
	testCases := map[string]struct {
		handler    http.Handler
		goCloudURL bool
	}{
		"no ETag": {
			handler: http.HandlerFunc(testServeCache),
		},
		"ETag": {
			handler: http.HandlerFunc(testServeCacheWithETag),
		},
		"GoCloud URL": {
			goCloudURL: true,
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			cdTempDir(t)

			removeHook := helpers.MakeWarningToPanic()
			t.Cleanup(removeHook)

			cmd := CacheExtractorCommand{
				File:    cacheExtractorArchive,
				Timeout: 0,
			}

			if tc.goCloudURL {
				mux, tmpDir := setupGoCloudFileBucket(t, "testblob")
				cmd.mux = mux
				cmd.GoCloudURL = fmt.Sprintf("testblob://bucket/%s", cacheExtractorArchive)

				testFile := path.Join(tmpDir, cacheExtractorArchive)
				writeZipFileAndMetadata(t, testFile)
			} else {
				ts := httptest.NewServer(tc.handler)
				t.Cleanup(ts.Close)
				cmd.URL = ts.URL + "/cache.zip"
			}

			assert.NotPanics(t, func() {
				cmd.Execute(nil)
			})

			assert.FileExists(t, cacheExtractorTestArchivedFile, "cache file does not exist")
			err := os.Chtimes(cacheExtractorArchive, time.Now().Add(time.Hour), time.Now().Add(time.Hour))
			assert.NoError(t, err)

			assert.FileExists(t, cacheExtractorMetadata, "cache metadata does not exist")
			data, err := os.ReadFile(cacheExtractorMetadata)
			assert.NoError(t, err, "reading cache metadata content")
			assert.Equal(t, `{"blank":"","foo":"some foo"}`, string(data), "unexpected cache metadata content")

			assert.NotPanics(t, func() { cmd.Execute(nil) }, "archive is up to date")
		})
	}
}

func TestCacheExtractorRemoteServerFailOnInvalidServer(t *testing.T) {
	removeHook := helpers.MakeWarningToPanic()
	t.Cleanup(removeHook)

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

func TestIsLocalCacheFileUpToDate(t *testing.T) {
	tmpDir := t.TempDir()
	cacheFile := path.Join(tmpDir, "cache-file")

	// Create cache file
	err := os.WriteFile(cacheFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Set a specific modification time
	modTime := time.Now()
	err = os.Chtimes(cacheFile, modTime, modTime)
	require.NoError(t, err)

	// Test when remote file is older (cache is up to date)
	result := isLocalCacheFileUpToDate(cacheFile, modTime.Add(-1*time.Hour))
	require.True(t, result, "Cache should be up to date when remote file is older")

	// Test when remote file is newer (cache is outdated)
	result = isLocalCacheFileUpToDate(cacheFile, modTime.Add(1*time.Hour))
	require.False(t, result, "Cache should be outdated when remote file is newer")
}

// cdTempDir creates a temp dir and changes into it; after the test this directory is cleaned up automatically.
func cdTempDir(t *testing.T) string {
	t.Helper()

	pwd, err := os.Getwd()
	require.NoError(t, err, "getting current PWD")

	d := t.TempDir()
	require.NoError(t, os.Chdir(d), "changing into temp dir")

	t.Cleanup(func() {
		require.NoError(t, os.Chdir(pwd), "changing back into previous PWD")
	})

	return d
}
