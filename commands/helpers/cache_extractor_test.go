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
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gocloud.dev/blob"
	"gocloud.dev/blob/azureblob"
	"gocloud.dev/blob/fileblob"
	"gocloud.dev/blob/s3blob"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/log/test"
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
		cdTempDir(t)

		file, err := os.Create(cacheExtractorArchive)
		require.NoError(t, err)

		archive := zip.NewWriter(file)
		_, err = archive.Create(cacheExtractorTestArchivedFile)
		require.NoError(t, err)

		w, err := archive.Create(cacheExtractorTestFile)
		require.NoError(t, err)
		_, err = w.Write(expectedContents)
		require.NoError(t, err)

		archive.Close()
		file.Close()

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
		cdTempDir(t)

		removeHook := helpers.MakeFatalToPanic()
		defer removeHook()
		writeTestFile(t, cacheExtractorArchive)

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
		http.Error(w, "408 Method not allowed", http.StatusRequestTimeout)
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

// TestCacheExtractorRemoteServerRejectsPartialContent covers the sequential download of a presigned
// URL, a GET without Range, answered with 206 Partial Content. A caching proxy whose cache key leaves
// out Range answers it that way once it stored a 206 for an earlier Range request of the same object,
// such as the bytes=0-0 probe of a runner with FF_USE_PARALLEL_CACHE_TRANSFER. The body is a part of
// the object, so the download fails and writes no archive, instead of keeping that part as an archive
// with the object's Last-Modified that every later run finds up to date. An archive already on disk
// and up to date is still used, as it is for any answer, since nothing is downloaded for it.
func TestCacheExtractorRemoteServerRejectsPartialContent(t *testing.T) {
	const earlierArchive = "the archive of an earlier run"
	lastModified := time.Date(2020, 5, 1, 12, 0, 0, 0, time.UTC)

	tests := map[string]struct {
		archiveOnDisk bool
		wantErr       bool
		wantArchive   string
	}{
		"no archive on disk": {
			wantErr: true,
		},
		"archive on disk and up to date": {
			archiveOnDisk: true,
			wantArchive:   earlierArchive,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cdTempDir(t)

			if tc.archiveOnDisk {
				require.NoError(t, os.WriteFile(cacheExtractorArchive, []byte(earlierArchive), 0o600))
				require.NoError(t, os.Chtimes(cacheExtractorArchive, time.Now(), lastModified.Add(time.Hour)))
			}

			payload := parallelTestZipBytes(t)
			var requests atomic.Int64
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.Header().Set("Last-Modified", lastModified.Format(http.TimeFormat))
				w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-0/%d", len(payload)))
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write(payload[:1])
			}))
			t.Cleanup(ts.Close)

			cmd := CacheExtractorCommand{
				File:               cacheExtractorArchive,
				URL:                ts.URL + "/cache.zip",
				TransferBufferSize: defaultCacheTransferBufferSize,
			}

			err := cmd.handlePresignedURL()
			assert.Equal(t, int64(1), requests.Load(), "the sequential download makes one GET")
			if tc.wantErr {
				require.Error(t, err, "a 206 to a GET without Range should fail the download")
				assert.ErrorContains(t, err, "206 Partial Content")
				assert.NoFileExists(t, cacheExtractorArchive)
				return
			}
			require.NoError(t, err)
			got, err := os.ReadFile(cacheExtractorArchive)
			require.NoError(t, err)
			assert.Equal(t, tc.wantArchive, string(got))
		})
	}
}

func TestSelectPresignedURL(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	older := now.Add(-1 * time.Hour)

	makeServer := func(t *testing.T, lastModified time.Time, statusCode int) *httptest.Server {
		t.Helper()
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if statusCode != http.StatusOK {
				http.Error(w, "not found", statusCode)
				return
			}
			w.Header().Set("Last-Modified", lastModified.UTC().Format(http.TimeFormat))
		}))
	}

	t.Run("no alternate URL, uses primary", func(t *testing.T) {
		primary := makeServer(t, now, http.StatusOK)
		defer primary.Close()

		cmd := &CacheExtractorCommand{URL: primary.URL + "/"}
		assert.Equal(t, primary.URL+"/", cmd.selectPresignedURL())
	})

	t.Run("primary newer, uses primary", func(t *testing.T) {
		primary := makeServer(t, now, http.StatusOK)
		alternate := makeServer(t, older, http.StatusOK)
		defer primary.Close()
		defer alternate.Close()

		cmd := &CacheExtractorCommand{URL: primary.URL + "/", AlternateURL: alternate.URL + "/"}
		assert.Equal(t, primary.URL+"/", cmd.selectPresignedURL())
	})

	t.Run("alternate newer, uses alternate", func(t *testing.T) {
		primary := makeServer(t, older, http.StatusOK)
		alternate := makeServer(t, now, http.StatusOK)
		defer primary.Close()
		defer alternate.Close()

		cmd := &CacheExtractorCommand{URL: primary.URL + "/", AlternateURL: alternate.URL + "/"}
		assert.Equal(t, alternate.URL+"/", cmd.selectPresignedURL())
	})

	t.Run("primary not found, uses alternate", func(t *testing.T) {
		primary := makeServer(t, now, http.StatusNotFound)
		alternate := makeServer(t, now, http.StatusOK)
		defer primary.Close()
		defer alternate.Close()

		cmd := &CacheExtractorCommand{URL: primary.URL + "/", AlternateURL: alternate.URL + "/"}
		assert.Equal(t, alternate.URL+"/", cmd.selectPresignedURL())
	})

	t.Run("alternate not found, uses primary", func(t *testing.T) {
		primary := makeServer(t, now, http.StatusOK)
		alternate := makeServer(t, now, http.StatusNotFound)
		defer primary.Close()
		defer alternate.Close()

		cmd := &CacheExtractorCommand{URL: primary.URL + "/", AlternateURL: alternate.URL + "/"}
		assert.Equal(t, primary.URL+"/", cmd.selectPresignedURL())
	})

	t.Run("both not found, uses primary", func(t *testing.T) {
		primary := makeServer(t, now, http.StatusNotFound)
		alternate := makeServer(t, now, http.StatusNotFound)
		defer primary.Close()
		defer alternate.Close()

		cmd := &CacheExtractorCommand{URL: primary.URL + "/", AlternateURL: alternate.URL + "/"}
		assert.Equal(t, primary.URL+"/", cmd.selectPresignedURL())
	})

	t.Run("primary returns unexpected status, uses alternate", func(t *testing.T) {
		primary := makeServer(t, now, http.StatusForbidden)
		alternate := makeServer(t, now, http.StatusOK)
		defer primary.Close()
		defer alternate.Close()

		cmd := &CacheExtractorCommand{URL: primary.URL + "/", AlternateURL: alternate.URL + "/"}
		assert.Equal(t, alternate.URL+"/", cmd.selectPresignedURL())
	})

	t.Run("both return unexpected status, uses primary", func(t *testing.T) {
		primary := makeServer(t, now, http.StatusForbidden)
		alternate := makeServer(t, now, http.StatusForbidden)
		defer primary.Close()
		defer alternate.Close()

		cmd := &CacheExtractorCommand{URL: primary.URL + "/", AlternateURL: alternate.URL + "/"}
		assert.Equal(t, primary.URL+"/", cmd.selectPresignedURL())
	})

	// Verifies that HeadURL is used for HEAD requests instead of URL.
	// Mirrors S3 behavior where GET presigned URLs reject HEAD requests with 403.
	t.Run("uses HeadURL when set, even if URL would fail HEAD", func(t *testing.T) {
		primaryGet := makeServer(t, now, http.StatusForbidden)
		primaryHead := makeServer(t, now, http.StatusOK)
		alternate := makeServer(t, older, http.StatusOK)
		defer primaryGet.Close()
		defer primaryHead.Close()
		defer alternate.Close()

		cmd := &CacheExtractorCommand{
			URL:          primaryGet.URL + "/",
			HeadURL:      primaryHead.URL + "/",
			AlternateURL: alternate.URL + "/",
		}
		// If HeadURL is used, primary HEAD succeeds and is newer than alternate, so primary is selected.
		// If URL were (incorrectly) used for HEAD, primary would 403 and alternate would be selected.
		assert.Equal(t, primaryGet.URL+"/", cmd.selectPresignedURL())
	})

	t.Run("uses AlternateHeadURL when set, even if AlternateURL would fail HEAD", func(t *testing.T) {
		primary := makeServer(t, older, http.StatusOK)
		alternateGet := makeServer(t, now, http.StatusForbidden)
		alternateHead := makeServer(t, now, http.StatusOK)
		defer primary.Close()
		defer alternateGet.Close()
		defer alternateHead.Close()

		cmd := &CacheExtractorCommand{
			URL:              primary.URL + "/",
			AlternateURL:     alternateGet.URL + "/",
			AlternateHeadURL: alternateHead.URL + "/",
		}
		// If AlternateHeadURL is used, alternate HEAD succeeds with a newer timestamp, so alternate is selected.
		// If AlternateURL were (incorrectly) used for HEAD, alternate would 403 and primary would be selected.
		assert.Equal(t, alternateGet.URL+"/", cmd.selectPresignedURL())
	})
}

func TestSelectGoCloudSource(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	older := now.Add(-1 * time.Hour)

	const scheme = "testblob2"

	setupBucket := func(t *testing.T) (*blob.URLMux, string) {
		t.Helper()
		mux, dir := setupGoCloudFileBucket(t, scheme)
		return mux, dir
	}

	writeBlob := func(t *testing.T, dir, name string, modTime time.Time) {
		t.Helper()
		writeZipFile(t, path.Join(dir, name))
		require.NoError(t, os.Chtimes(path.Join(dir, name), modTime, modTime))
		// write empty attrs sidecar so fileblob returns modTime correctly
		attrFile := path.Join(dir, name+".attrs")
		require.NoError(t, os.WriteFile(attrFile, []byte("{}"), 0o640))
	}

	t.Run("primary newer, uses primary", func(t *testing.T) {
		cdTempDir(t)
		mux, dir := setupBucket(t)

		writeBlob(t, dir, "primary.zip", now)
		writeBlob(t, dir, "alternate.zip", older)

		ctx := t.Context()
		primaryB, err := mux.OpenBucket(ctx, scheme+"://bucket/primary.zip")
		require.NoError(t, err)
		defer primaryB.Close()

		altB, err := mux.OpenBucket(ctx, scheme+"://bucket/alternate.zip")
		require.NoError(t, err)
		defer altB.Close()

		cmd := &CacheExtractorCommand{}
		gotBucket, gotObject := cmd.selectGoCloudSource(ctx, primaryB, "primary.zip", altB, "alternate.zip")
		assert.Equal(t, primaryB, gotBucket)
		assert.Equal(t, "primary.zip", gotObject)
	})

	t.Run("alternate newer, uses alternate", func(t *testing.T) {
		cdTempDir(t)
		mux, dir := setupBucket(t)

		writeBlob(t, dir, "primary.zip", older)
		writeBlob(t, dir, "alternate.zip", now)

		ctx := t.Context()
		primaryB, err := mux.OpenBucket(ctx, scheme+"://bucket/primary.zip")
		require.NoError(t, err)
		defer primaryB.Close()

		altB, err := mux.OpenBucket(ctx, scheme+"://bucket/alternate.zip")
		require.NoError(t, err)
		defer altB.Close()

		cmd := &CacheExtractorCommand{}
		gotBucket, gotObject := cmd.selectGoCloudSource(ctx, primaryB, "primary.zip", altB, "alternate.zip")
		assert.Equal(t, altB, gotBucket)
		assert.Equal(t, "alternate.zip", gotObject)
	})

	t.Run("primary not found, uses alternate", func(t *testing.T) {
		cdTempDir(t)
		mux, dir := setupBucket(t)

		writeBlob(t, dir, "alternate.zip", now)
		// primary.zip does not exist

		ctx := t.Context()
		primaryB, err := mux.OpenBucket(ctx, scheme+"://bucket/primary.zip")
		require.NoError(t, err)
		defer primaryB.Close()

		altB, err := mux.OpenBucket(ctx, scheme+"://bucket/alternate.zip")
		require.NoError(t, err)
		defer altB.Close()

		cmd := &CacheExtractorCommand{}
		gotBucket, gotObject := cmd.selectGoCloudSource(ctx, primaryB, "primary.zip", altB, "alternate.zip")
		assert.Equal(t, altB, gotBucket)
		assert.Equal(t, "alternate.zip", gotObject)
	})

	t.Run("alternate not found, uses primary", func(t *testing.T) {
		cdTempDir(t)
		mux, dir := setupBucket(t)

		writeBlob(t, dir, "primary.zip", now)
		// alternate.zip does not exist

		ctx := t.Context()
		primaryB, err := mux.OpenBucket(ctx, scheme+"://bucket/primary.zip")
		require.NoError(t, err)
		defer primaryB.Close()

		altB, err := mux.OpenBucket(ctx, scheme+"://bucket/alternate.zip")
		require.NoError(t, err)
		defer altB.Close()

		cmd := &CacheExtractorCommand{}
		gotBucket, gotObject := cmd.selectGoCloudSource(ctx, primaryB, "primary.zip", altB, "alternate.zip")
		assert.Equal(t, primaryB, gotBucket)
		assert.Equal(t, "primary.zip", gotObject)
	})
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
func cdTempDir(t *testing.T) {
	t.Helper()

	pwd, err := os.Getwd()
	require.NoError(t, err, "getting current PWD")

	d := t.TempDir()
	require.NoError(t, os.Chdir(d), "changing into temp dir")

	t.Cleanup(func() {
		require.NoError(t, os.Chdir(pwd), "changing back into previous PWD")
	})
}

// parallelTestZipBytes returns the same archive bytes as writeZipFile (small zip > default parallel chunk for tests that set a tiny ChunkSize).
func parallelTestZipBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zipWriter := zip.NewWriter(&buf)
	f, err := zipWriter.Create(cacheExtractorTestArchivedFile)
	require.NoError(t, err)
	_, err = io.WriteString(f, "This is a test.")
	require.NoError(t, err)
	require.NoError(t, zipWriter.Close())
	return buf.Bytes()
}

func parseBytesRangeHeader(h string) (start, end int64, ok bool) {
	const prefix = "bytes="
	if !strings.HasPrefix(h, prefix) {
		return 0, 0, false
	}
	h = h[len(prefix):]
	startStr, endStr, found := strings.Cut(h, "-")
	if !found {
		return 0, 0, false
	}
	start, startErr := strconv.ParseInt(startStr, 10, 64)
	end, endErr := strconv.ParseInt(endStr, 10, 64)
	if startErr != nil || endErr != nil {
		return 0, 0, false
	}
	return start, end, true
}

// testParallelPresignedCacheHandler serves a fixed payload with 206 + Content-Range for every Range GET (probe + chunk fetches).
func testParallelPresignedCacheHandler(t *testing.T, payload []byte, lastModified string) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/cache.zip" {
			http.NotFound(w, r)
			return
		}
		rangeHdr := r.Header.Get("Range")
		start, end, ok := parseBytesRangeHeader(rangeHdr)
		if !ok {
			http.Error(w, "missing or invalid Range", http.StatusBadRequest)
			return
		}
		if start < 0 || end < start || start >= int64(len(payload)) {
			http.Error(w, "range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if end >= int64(len(payload)) {
			end = int64(len(payload)) - 1
		}
		seg := payload[start : end+1]
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Last-Modified", lastModified)
		w.Header().Set("x-fakeCloud-meta-foo", "some foo")
		w.Header().Set("x-fakeCloud-meta-blank", "")
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(seg)
	}
}

// testParallelPresignedCacheHandlerWithHEAD is like testParallelPresignedCacheHandler but also handles HEAD requests
// (returning Last-Modified) and counts the number of Range GETs via the supplied atomic counter.
func testParallelPresignedCacheHandlerWithHEAD(t *testing.T, payload []byte, lastModified string, getCount *atomic.Int64) http.HandlerFunc {
	t.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cache.zip" {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodHead {
			w.Header().Set("Last-Modified", lastModified)
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		getCount.Add(1)
		rangeHdr := r.Header.Get("Range")
		start, end, ok := parseBytesRangeHeader(rangeHdr)
		if !ok {
			http.Error(w, "missing or invalid Range", http.StatusBadRequest)
			return
		}
		if start < 0 || end < start || start >= int64(len(payload)) {
			http.Error(w, "range not satisfiable", http.StatusRequestedRangeNotSatisfiable)
			return
		}
		if end >= int64(len(payload)) {
			end = int64(len(payload)) - 1
		}
		seg := payload[start : end+1]
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Last-Modified", lastModified)
		w.Header().Set("x-fakeCloud-meta-foo", "some foo")
		w.Header().Set("x-fakeCloud-meta-blank", "")
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write(seg)
	}
}

// TestCacheExtractorPresignedParallelTransferAlternate verifies that when an alternate URL is configured
// and selectPresignedURL picks it (alternate is newer), the parallel download path fetches chunks from
// the alternate URL and not from the primary.
func TestCacheExtractorPresignedParallelTransferAlternate(t *testing.T) {
	t.Setenv(featureflags.UseParallelCacheTransfer, "true")

	cdTempDir(t)
	removeHook := helpers.MakeWarningToPanic()
	t.Cleanup(removeHook)

	payload := parallelTestZipBytes(t)
	require.Greater(t, len(payload), 32, "payload should span multiple parallel chunks")

	primaryLM := time.Date(2020, 5, 1, 12, 0, 0, 0, time.UTC).Format(http.TimeFormat)
	alternateLM := time.Date(2021, 5, 1, 12, 0, 0, 0, time.UTC).Format(http.TimeFormat)

	var primaryGetCount, alternateGetCount atomic.Int64

	primaryTS := httptest.NewServer(testParallelPresignedCacheHandlerWithHEAD(t, payload, primaryLM, &primaryGetCount))
	t.Cleanup(primaryTS.Close)

	alternateTS := httptest.NewServer(testParallelPresignedCacheHandlerWithHEAD(t, payload, alternateLM, &alternateGetCount))
	t.Cleanup(alternateTS.Close)

	cmd := CacheExtractorCommand{
		File:         cacheExtractorArchive,
		URL:          primaryTS.URL + "/cache.zip",
		AlternateURL: alternateTS.URL + "/cache.zip",
		Timeout:      0,
		ChunkSize:    7,
		Concurrency:  4,
	}
	assert.NotPanics(t, func() { cmd.Execute(nil) })

	assert.FileExists(t, cacheExtractorTestArchivedFile)

	// selectPresignedURL picks alternate (newer Last-Modified). All Range GETs (probe + chunks) must hit alternate.
	// Before the fix, parallel download used c.URL directly and would have hit primary.
	assert.Zero(t, primaryGetCount.Load(), "primary Range GETs should not happen when alternate is newer")
	assert.Greater(t, alternateGetCount.Load(), int64(0), "alternate Range GETs should happen")
}

// TestCacheExtractorPresignedParallelTransfer exercises tryPresignedParallelDownload: Range probe, parallel chunk GETs, and WriteAt via the meter writer.
func TestCacheExtractorPresignedParallelTransfer(t *testing.T) {
	t.Setenv(featureflags.UseParallelCacheTransfer, "true")

	cdTempDir(t)
	removeHook := helpers.MakeWarningToPanic()
	t.Cleanup(removeHook)

	payload := parallelTestZipBytes(t)
	require.Greater(t, len(payload), 32, "payload should span multiple parallel chunks")

	lm := time.Date(2020, 5, 1, 12, 0, 0, 0, time.UTC).Format(http.TimeFormat)
	ts := httptest.NewServer(testParallelPresignedCacheHandler(t, payload, lm))
	t.Cleanup(ts.Close)

	cmd := CacheExtractorCommand{
		File:        cacheExtractorArchive,
		URL:         ts.URL + "/cache.zip",
		Timeout:     0,
		ChunkSize:   7,
		Concurrency: 4,
		// TransferMeterFrequency left at 0 so meter.NewWriter returns *os.File (io.WriterAt) for parallel download.
	}
	assert.NotPanics(t, func() { cmd.Execute(nil) })

	assert.FileExists(t, cacheExtractorTestArchivedFile)
	assert.FileExists(t, cacheExtractorMetadata)
	data, err := os.ReadFile(cacheExtractorMetadata)
	require.NoError(t, err)
	assert.JSONEq(t, `{"blank":"","foo":"some foo"}`, string(data))

	got, err := os.ReadFile(cacheExtractorArchive)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

// TestCacheExtractorPresignedParallelTransferFallsBackOnFullBodyChunk covers a server that honors
// the bytes=0-0 probe and the first chunk, but answers every chunk after the first with 200 OK and
// the whole object, as a load balancer does when one of its nodes ignores Range. Written at the
// chunk offset, that body would corrupt the archive, so the parallel download must stop and the
// extractor must make one sequential GET instead.
func TestCacheExtractorPresignedParallelTransferFallsBackOnFullBodyChunk(t *testing.T) {
	t.Setenv(featureflags.UseParallelCacheTransfer, "true")
	cdTempDir(t)

	payload := parallelTestZipBytes(t)
	require.Greater(t, len(payload), 32, "payload should span multiple parallel chunks")

	lm := time.Date(2020, 5, 1, 12, 0, 0, 0, time.UTC).Format(http.TimeFormat)
	rangeHandler := testParallelPresignedCacheHandler(t, payload, lm)
	var fullBody, sequential atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Last-Modified", lm)
		start, _, ok := parseBytesRangeHeader(r.Header.Get("Range"))
		switch {
		case r.Header.Get("Range") == "":
			sequential.Add(1)
			_, _ = w.Write(payload)
		case ok && start > 0:
			fullBody.Add(1)
			_, _ = w.Write(payload)
		default:
			rangeHandler(w, r)
		}
	}))
	t.Cleanup(ts.Close)

	cmd := CacheExtractorCommand{
		File:               cacheExtractorArchive,
		URL:                ts.URL + "/cache.zip",
		ChunkSize:          7,
		Concurrency:        4,
		TransferBufferSize: defaultCacheTransferBufferSize,
	}

	require.NoError(t, cmd.handlePresignedURL())
	assert.Positive(t, fullBody.Load(), "the server should have answered chunks with 200 and the whole object")
	assert.Equal(t, int64(1), sequential.Load(), "the extractor should fall back to one sequential GET")

	got, err := os.ReadFile(cacheExtractorArchive)
	require.NoError(t, err)
	assert.Equal(t, payload, got)
}

// TestCacheExtractorPresignedParallelTransferRejectsChunkOutsideRange covers chunks answered with a
// 206 that is not for the requested range. A sequential GET would not help: a caching proxy whose
// cache key leaves out Range answers every request, one without Range included, with the partial
// 206 it stored first, and the sequential download would keep that as the archive. So the parallel
// download fails with a retryable error, no GET without Range is made and no archive is written.
func TestCacheExtractorPresignedParallelTransferRejectsChunkOutsideRange(t *testing.T) {
	tests := map[string]struct {
		// replayToAll answers every request with respond, the probe and a GET without Range
		// included, as the caching proxy does once it has stored the probe's 206. Otherwise
		// respond answers every chunk after the first.
		replayToAll bool
		respond     func(w http.ResponseWriter, payload []byte, start, end int64)
	}{
		"206 for another range": {
			respond: func(w http.ResponseWriter, payload []byte, start, end int64) {
				w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-%d/%d", end-start, len(payload)))
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write(payload[:end-start+1])
			},
		},
		"206 without Content-Range": {
			respond: func(w http.ResponseWriter, payload []byte, start, end int64) {
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write(payload[:end-start+1])
			},
		},
		"206 a caching proxy replays to every request": {
			replayToAll: true,
			respond: func(w http.ResponseWriter, payload []byte, _, _ int64) {
				w.Header().Set("Content-Range", fmt.Sprintf("bytes 0-0/%d", len(payload)))
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write(payload[:1])
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv(featureflags.UseParallelCacheTransfer, "true")
			cdTempDir(t)

			payload := parallelTestZipBytes(t)
			require.Greater(t, len(payload), 32, "payload should span multiple parallel chunks")

			lm := time.Date(2020, 5, 1, 12, 0, 0, 0, time.UTC).Format(http.TimeFormat)
			rangeHandler := testParallelPresignedCacheHandler(t, payload, lm)
			var outsideRange, sequential atomic.Int64
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Last-Modified", lm)
				if r.Header.Get("Range") == "" {
					sequential.Add(1)
				}
				start, end, ok := parseBytesRangeHeader(r.Header.Get("Range"))
				switch {
				case tc.replayToAll || (ok && start > 0):
					outsideRange.Add(1)
					tc.respond(w, payload, start, end)
				case !ok:
					_, _ = w.Write(payload)
				default:
					rangeHandler(w, r)
				}
			}))
			t.Cleanup(ts.Close)

			cmd := CacheExtractorCommand{
				File:               cacheExtractorArchive,
				URL:                ts.URL + "/cache.zip",
				ChunkSize:          7,
				Concurrency:        4,
				TransferBufferSize: defaultCacheTransferBufferSize,
			}

			err := cmd.handlePresignedURL()
			require.ErrorAs(t, err, new(retryableError), "a chunk outside its range should fail with a retryable error")
			assert.ErrorContains(t, err, "range request failed: 206 Partial Content")
			assert.Positive(t, outsideRange.Load(), "the server should have answered chunks outside their range")
			assert.Zero(t, sequential.Load(), "a 206 outside the range should not switch to the sequential GET")
			assert.NoFileExists(t, cacheExtractorArchive)
		})
	}
}

// TestCacheExtractorPresignedParallelTransferFailsOnChunkError covers a chunk answered with a
// server error. That says nothing about Range support, so the parallel download fails with a
// retryable error and the extractor does not switch to the sequential GET.
func TestCacheExtractorPresignedParallelTransferFailsOnChunkError(t *testing.T) {
	t.Setenv(featureflags.UseParallelCacheTransfer, "true")
	cdTempDir(t)

	payload := parallelTestZipBytes(t)
	lm := time.Date(2020, 5, 1, 12, 0, 0, 0, time.UTC).Format(http.TimeFormat)
	rangeHandler := testParallelPresignedCacheHandler(t, payload, lm)
	var sequential atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, _, ok := parseBytesRangeHeader(r.Header.Get("Range"))
		switch {
		case !ok:
			sequential.Add(1)
			_, _ = w.Write(payload)
		case start > 0:
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
		default:
			rangeHandler(w, r)
		}
	}))
	t.Cleanup(ts.Close)

	cmd := CacheExtractorCommand{
		File:        cacheExtractorArchive,
		URL:         ts.URL + "/cache.zip",
		ChunkSize:   7,
		Concurrency: 4,
	}

	err := cmd.handlePresignedURL()
	require.ErrorAs(t, err, new(retryableError), "a server error on a chunk should stay retryable")
	assert.ErrorContains(t, err, "range request failed: 503 Service Unavailable")
	assert.Zero(t, sequential.Load(), "a server error on a chunk should not switch to the sequential GET")
	assert.NoFileExists(t, cacheExtractorArchive)
}

// TestCacheExtractorGoCloudParallelTransfer exercises handleGoCloudURL with FF_USE_PARALLEL_CACHE_TRANSFER, range probe, and parallel NewRangeReader + WriteAt.
func TestCacheExtractorGoCloudParallelTransfer(t *testing.T) {
	t.Setenv(featureflags.UseParallelCacheTransfer, "true")

	cdTempDir(t)
	removeHook := helpers.MakeWarningToPanic()
	t.Cleanup(removeHook)

	mux, tmpDir := setupGoCloudFileBucket(t, "testblob")
	testFile := path.Join(tmpDir, cacheExtractorArchive)
	writeZipFileAndMetadata(t, testFile)

	info, err := os.Stat(testFile)
	require.NoError(t, err)
	require.Greater(t, info.Size(), int64(32), "object should be larger than test chunk size")

	cmd := CacheExtractorCommand{
		File:        cacheExtractorArchive,
		GoCloudURL:  fmt.Sprintf("testblob://bucket/%s", cacheExtractorArchive),
		Timeout:     0,
		mux:         mux,
		ChunkSize:   32,
		Concurrency: 4,
	}
	assert.NotPanics(t, func() { cmd.Execute(nil) })

	assert.FileExists(t, cacheExtractorTestArchivedFile)
	assert.FileExists(t, cacheExtractorMetadata)
	data, err := os.ReadFile(cacheExtractorMetadata)
	require.NoError(t, err)
	assert.JSONEq(t, `{"blank":"","foo":"some foo"}`, string(data))
}

// goCloudRangeDrivers points a GoCloud cache URL of each driver whose chunk responses the parallel
// download checks at a stand-in for its endpoint, and returns the URL mux to open it with.
var goCloudRangeDrivers = map[string]func(t *testing.T, endpoint string) (string, *blob.URLMux){
	"s3": func(t *testing.T, endpoint string) (string, *blob.URLMux) {
		t.Setenv("AWS_ACCESS_KEY_ID", "test")
		t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
		q := url.Values{}
		q.Set("endpoint", endpoint)
		q.Set("region", "us-east-1")
		q.Set("use_path_style", "true")
		q.Set("disable_https", "true")
		mux := new(blob.URLMux)
		mux.RegisterBucket(s3blob.Scheme, &s3blob.URLOpener{})
		return "s3://bucket/" + cacheExtractorArchive + "?" + q.Encode(), mux
	},
	"azblob": func(t *testing.T, endpoint string) (string, *blob.URLMux) {
		u, err := url.Parse(endpoint)
		require.NoError(t, err)
		mux := new(blob.URLMux)
		mux.RegisterBucket(azureblob.Scheme, &azureblob.URLOpener{
			MakeClient: func(svcURL azureblob.ServiceURL, containerName azureblob.ContainerName) (*container.Client, error) {
				containerURL, err := url.JoinPath(string(svcURL), string(containerName))
				if err != nil {
					return nil, err
				}
				return container.NewClientWithNoCredential(containerURL, nil)
			},
			ServiceURLOptions: azureblob.ServiceURLOptions{AccountName: "account", StorageDomain: u.Host, Protocol: "http"},
		})
		return "azblob://container/" + cacheExtractorArchive, mux
	},
}

// writeGoCloudRange answers with a 206 that holds bytes start-end of payload.
func writeGoCloudRange(w http.ResponseWriter, payload []byte, start, end int64) {
	w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
	w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
	w.WriteHeader(http.StatusPartialContent)
	_, _ = w.Write(payload[start : end+1])
}

// TestCacheExtractorGoCloudParallelTransferChecksChunkRange covers the parallel GoCloud download from
// a stand-in for an S3 and an Azure endpoint. The s3blob and azureblob drivers return the body of any
// 2xx answer, so the extractor reads each chunk's HTTP response. The first chunk is answered as
// requested and every chunk after it by the case. A chunk answered with 200 and the whole object makes
// the extractor download the object with a single reader. A 206 for another range, or one without
// Content-Range, fails the download with a retryable error, with no GET without Range and no archive.
func TestCacheExtractorGoCloudParallelTransferChecksChunkRange(t *testing.T) {
	responses := map[string]struct {
		respond        func(w http.ResponseWriter, payload []byte, start, end int64)
		wantErr        bool
		wantSequential int64
	}{
		"206 as requested": {
			respond: writeGoCloudRange,
		},
		"200 with the whole object": {
			respond: func(w http.ResponseWriter, payload []byte, _, _ int64) {
				w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
				_, _ = w.Write(payload)
			},
			wantSequential: 1,
		},
		"206 for another range": {
			respond: func(w http.ResponseWriter, payload []byte, start, end int64) {
				writeGoCloudRange(w, payload, 0, end-start)
			},
			wantErr: true,
		},
		"206 without Content-Range": {
			respond: func(w http.ResponseWriter, payload []byte, start, end int64) {
				w.Header().Set("Content-Length", strconv.FormatInt(end-start+1, 10))
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write(payload[:end-start+1])
			},
			wantErr: true,
		},
	}

	for driver, open := range goCloudRangeDrivers {
		for name, tc := range responses {
			t.Run(driver+"/"+name, func(t *testing.T) {
				t.Setenv(featureflags.UseParallelCacheTransfer, "true")
				cdTempDir(t)

				payload := parallelTestZipBytes(t)
				require.Greater(t, len(payload), 32, "payload should span multiple parallel chunks")

				lm := time.Date(2020, 5, 1, 12, 0, 0, 0, time.UTC).Format(http.TimeFormat)
				var sequential atomic.Int64
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Last-Modified", lm)
					w.Header().Set("x-ms-creation-time", lm)
					w.Header().Set("ETag", `"0123456789abcdef0123456789abcdef"`)
					w.Header().Set("Content-Type", "application/zip")
					// The Azure SDK sends the range in x-ms-range.
					rangeHeader := r.Header.Get("Range")
					if rangeHeader == "" {
						rangeHeader = r.Header.Get("x-ms-range")
					}
					start, end, ok := parseBytesRangeHeader(rangeHeader)
					switch {
					case r.Method == http.MethodHead:
						w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
					case !ok:
						sequential.Add(1)
						w.Header().Set("Content-Length", strconv.Itoa(len(payload)))
						_, _ = w.Write(payload)
					case start > 0:
						tc.respond(w, payload, start, end)
					default:
						writeGoCloudRange(w, payload, start, end)
					}
				}))
				t.Cleanup(ts.Close)

				gocloudURL, mux := open(t, ts.URL)
				cmd := CacheExtractorCommand{
					File:               cacheExtractorArchive,
					GoCloudURL:         gocloudURL,
					mux:                mux,
					ChunkSize:          7,
					Concurrency:        4,
					TransferBufferSize: defaultCacheTransferBufferSize,
				}

				err := cmd.handleGoCloudURL()
				assert.Equal(t, tc.wantSequential, sequential.Load(), "GETs without Range")
				if tc.wantErr {
					require.ErrorAs(t, err, new(retryableError), "a chunk outside its range should fail with a retryable error")
					assert.ErrorContains(t, err, "range request failed: 206 Partial Content")
					assert.NoFileExists(t, cacheExtractorArchive)
					return
				}
				require.NoError(t, err)
				got, err := os.ReadFile(cacheExtractorArchive)
				require.NoError(t, err)
				assert.Equal(t, payload, got)
			})
		}
	}
}

// TestCheckChunkRange covers the rule both parallel chunk fetchers apply to the answer for a chunk:
// only a 206 whose Content-Range is the requested range is accepted, and only a 200 is marked with
// errRangeNotHonored, which makes the caller download the object with a single request.
func TestCheckChunkRange(t *testing.T) {
	const offset, length = 7, 7 // bytes 7-13

	tests := map[string]struct {
		code         int
		contentRange string
		wantErr      string
		wantFallback bool
	}{
		"206 as requested": {
			code:         http.StatusPartialContent,
			contentRange: "bytes 7-13/50",
		},
		"206 as requested with an unknown total": {
			code:         http.StatusPartialContent,
			contentRange: "bytes 7-13/*",
		},
		"200 with the whole object": {
			code:         http.StatusOK,
			wantErr:      "range request failed: 200 OK: range not honored",
			wantFallback: true,
		},
		"200 with the requested Content-Range": {
			code:         http.StatusOK,
			contentRange: "bytes 7-13/50",
			wantErr:      "range request failed: 200 OK: range not honored",
			wantFallback: true,
		},
		"206 for another range": {
			code:         http.StatusPartialContent,
			contentRange: "bytes 0-6/50",
			wantErr:      "range request failed: 206 Partial Content",
		},
		"206 for a longer range from the same offset": {
			code:         http.StatusPartialContent,
			contentRange: "bytes 7-130/500",
			wantErr:      "range request failed: 206 Partial Content",
		},
		"206 without Content-Range": {
			code:    http.StatusPartialContent,
			wantErr: "range request failed: 206 Partial Content",
		},
		"416": {
			code:         http.StatusRequestedRangeNotSatisfiable,
			contentRange: "bytes */50",
			wantErr:      "range request failed: 416 Requested Range Not Satisfiable",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			status := fmt.Sprintf("%d %s", tc.code, http.StatusText(tc.code))

			err := checkChunkRange(tc.code, status, tc.contentRange, offset, length)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			require.EqualError(t, err, tc.wantErr)
			if tc.wantFallback {
				assert.ErrorIs(t, err, errRangeNotHonored)
			} else {
				assert.NotErrorIs(t, err, errRangeNotHonored)
			}
		})
	}
}

// TestGoCloudChunkStatusLogsUnreadableResponse covers the debug line of goCloudChunkStatus. An Azure
// chunk read without the context that captures its response stands for a transport that does not
// provide it: ok is false, so the chunk would be written without the range check, and the line says
// so. A chunk of fileblob, a driver that is not checked, gives ok false without the line.
func TestGoCloudChunkStatusLogsUnreadableResponse(t *testing.T) {
	const notChecked = "GoCloud chunk response could not be read, chunk range not checked"

	level := logrus.GetLevel()
	logrus.SetLevel(logrus.DebugLevel)
	t.Cleanup(func() { logrus.SetLevel(level) })

	payload := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	// notCheckedEntries calls goCloudChunkStatus on r with no captured response, and returns ok
	// and the entries it logged with the line of an unchecked chunk.
	notCheckedEntries := func(t *testing.T, r *blob.Reader) (bool, []*logrus.Entry) {
		hook, removeHook := test.NewHook()
		t.Cleanup(removeHook)

		_, _, _, ok := goCloudChunkStatus(r, nil)

		var entries []*logrus.Entry
		for _, entry := range hook.AllEntries() {
			if entry.Message == notChecked {
				entries = append(entries, entry)
			}
		}
		return ok, entries
	}

	t.Run("azblob without the captured response", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Last-Modified", time.Date(2020, 5, 1, 12, 0, 0, 0, time.UTC).Format(http.TimeFormat))
			writeGoCloudRange(w, payload, 7, 13)
		}))
		t.Cleanup(ts.Close)

		gocloudURL, mux := goCloudRangeDrivers["azblob"](t, ts.URL)
		b, err := mux.OpenBucket(t.Context(), gocloudURL)
		require.NoError(t, err)
		t.Cleanup(func() { _ = b.Close() })
		// goCloudRangeFetchChunk reads with policy.WithCaptureResponse in the context; this read does not.
		r, err := b.NewRangeReader(t.Context(), cacheExtractorArchive, 7, 7, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = r.Close() })

		ok, entries := notCheckedEntries(t, r)
		assert.False(t, ok)
		require.Len(t, entries, 1)
		assert.Equal(t, logrus.DebugLevel, entries[0].Level)
	})

	t.Run("fileblob", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(path.Join(dir, cacheExtractorArchive), payload, 0o600))
		b, err := fileblob.OpenBucket(dir, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = b.Close() })
		r, err := b.NewRangeReader(t.Context(), cacheExtractorArchive, 7, 7, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = r.Close() })

		ok, entries := notCheckedEntries(t, r)
		assert.False(t, ok)
		assert.Empty(t, entries)
	})
}

func TestGoCloudURLSchemeAssumesRangeSupport(t *testing.T) {
	t.Parallel()

	assert.True(t, goCloudURLSchemeAssumesRangeSupport("s3"))
	assert.True(t, goCloudURLSchemeAssumesRangeSupport("S3"))
	assert.True(t, goCloudURLSchemeAssumesRangeSupport("gs"))
	assert.True(t, goCloudURLSchemeAssumesRangeSupport("azblob"))
	assert.False(t, goCloudURLSchemeAssumesRangeSupport("testblob"))
	assert.False(t, goCloudURLSchemeAssumesRangeSupport("file"))
}

// TestUseParallelCacheTransferEnv checks env parsing for the feature flag. Parallel download wiring is covered by
// TestCacheExtractorPresignedParallelTransfer and TestCacheExtractorGoCloudParallelTransfer.
func TestUseParallelCacheTransferEnv(t *testing.T) {
	logger := logrus.WithField("name", featureflags.UseParallelCacheTransfer)
	t.Run("unset", func(t *testing.T) {
		t.Setenv(featureflags.UseParallelCacheTransfer, "")
		assert.False(t, featureflags.IsOn(logger, os.Getenv(featureflags.UseParallelCacheTransfer)))
	})
	t.Run("false", func(t *testing.T) {
		t.Setenv(featureflags.UseParallelCacheTransfer, "false")
		assert.False(t, featureflags.IsOn(logger, os.Getenv(featureflags.UseParallelCacheTransfer)))
	})
	t.Run("true", func(t *testing.T) {
		t.Setenv(featureflags.UseParallelCacheTransfer, "true")
		assert.True(t, featureflags.IsOn(logger, os.Getenv(featureflags.UseParallelCacheTransfer)))
	})
}

func makeTestZipArchive(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("file.txt")
	require.NoError(t, err)
	_, err = w.Write([]byte("data"))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestCacheExtractor_RedactURL_SuppressesURLInLog(t *testing.T) {
	cdTempDir(t)
	archive := makeTestZipArchive(t)

	future := time.Now().Add(time.Hour)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "cache.zip", future, bytes.NewReader(archive))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	orig := logrus.StandardLogger().Out
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(orig)

	cmd := &CacheExtractorCommand{
		File:               cacheExtractorArchive,
		URL:                srv.URL,
		RedactURL:          true,
		TransferBufferSize: defaultCacheTransferBufferSize,
	}
	err := cmd.downloadPresignedSequential()
	require.NoError(t, err)

	logged := buf.String()
	assert.NotContains(t, logged, srv.URL, "URL must not appear in logs when RedactURL is true")
	assert.Contains(t, logged, "remote storage", "generic placeholder must appear when RedactURL is true")
}

func TestCacheExtractor_NoRedact_ShowsURLInLog(t *testing.T) {
	cdTempDir(t)
	archive := makeTestZipArchive(t)

	future := time.Now().Add(time.Hour)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "cache.zip", future, bytes.NewReader(archive))
	}))
	defer srv.Close()

	var buf bytes.Buffer
	orig := logrus.StandardLogger().Out
	logrus.SetOutput(&buf)
	defer logrus.SetOutput(orig)

	cmd := &CacheExtractorCommand{
		File:               cacheExtractorArchive,
		URL:                srv.URL,
		RedactURL:          false,
		TransferBufferSize: defaultCacheTransferBufferSize,
	}
	err := cmd.downloadPresignedSequential()
	require.NoError(t, err)

	assert.Contains(t, buf.String(), srv.URL, "URL must appear in logs when RedactURL is false")
}
