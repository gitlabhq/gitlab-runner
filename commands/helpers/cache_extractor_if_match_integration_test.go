//go:build integration

package helpers

import (
	"bytes"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/transfer"
)

// overwriteAfterFirstChunk replaces the object once, after the first chunk response arrives.
// write runs on the goroutine of that chunk request, not on the test's, so it reports a failure
// with assert. require would call t.FailNow there, which ends only that goroutine: the download
// would lose the chunk without seeing an error, and the test would go on.
type overwriteAfterFirstChunk struct {
	next  http.RoundTripper
	once  sync.Once
	write func()
}

func (o *overwriteAfterFirstChunk) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := o.next.RoundTrip(req)
	if err == nil && req.Header.Get("Range") != "bytes=0-0" {
		o.once.Do(o.write)
	}
	return resp, err
}

// A job that uploads the same cache key during the download replaces the object between chunk
// requests. The chunks requested after that are answered 412 to the If-Match that holds the
// probe's ETag, and the download falls back to a single GET of the object as it is then.
func TestPresignedParallelDownloadObjectReplacedDuringDownload(t *testing.T) {
	t.Setenv(featureflags.UseParallelCacheTransfer, "true")
	store := newRangeITStore(t)
	rangeITChdir(t)

	key := rangeITKey(t)
	v1 := rangeITPayload(t)
	v2 := rangeITPayload(t)
	store.put(t, key, v1)

	cmd := rangeITCommand(store.presign(t, store.endpoint, key))
	client := NewCacheClient(0)
	replace := func() {
		_, err := store.client.PutObject(t.Context(), &s3.PutObjectInput{
			Bucket: aws.String(rangeITBucket),
			Key:    aws.String(key),
			Body:   bytes.NewReader(v2),
		})
		assert.NoError(t, err, "replacing the object during the download")
	}
	client.Transport = &overwriteAfterFirstChunk{next: client.Transport, write: replace}
	cmd.client = client

	err := cmd.handlePresignedURL()
	require.NoError(t, err)
	require.True(t, requireNotCorrupt(t, v1, v2), "archive should be downloaded")
}

// The parallel download sends If-Match with the probe's ETag on each chunk. This checks that a
// presigned URL accepts that unsigned header and that the store answers 412 on a mismatch.
func TestPresignedRangeRequestHonorsIfMatch(t *testing.T) {
	store := newRangeITStore(t)

	key := rangeITKey(t)
	store.put(t, key, rangeITPayload(t))
	rawURL := store.presign(t, store.endpoint, key)

	get := func(ifMatch string) *http.Response {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, rawURL, nil)
		require.NoError(t, err)
		req.Header.Set("Range", "bytes=1048576-2097151")
		if ifMatch != "" {
			req.Header.Set("If-Match", ifMatch)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		return resp
	}

	probe := get("")
	require.Equal(t, http.StatusPartialContent, probe.StatusCode)
	etag := probe.Header.Get("ETag")
	require.NotEmpty(t, etag)
	require.True(t, transfer.StrongETag(etag),
		"the store's ETag %q must be strong for the chunk requests to send it in If-Match", etag)

	assert.Equal(t, http.StatusPartialContent, get(etag).StatusCode, "matching If-Match")
	assert.Equal(t, http.StatusPreconditionFailed, get(`"0123456789abcdef0123456789abcdef"`).StatusCode, "stale If-Match")
}
