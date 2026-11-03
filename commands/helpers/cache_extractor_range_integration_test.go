//go:build integration

package helpers

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

// These tests run the parallel cache download against real object stores and real proxies.
// Each endpoint comes from an environment variable, and a test skips when its variable is unset:
//
//	RANGE_IT_S3_ENDPOINT       S3-compatible store, for example SeaweedFS or LocalStack
//	RANGE_IT_S3_ACCESS_KEY     access key for RANGE_IT_S3_ENDPOINT
//	RANGE_IT_S3_SECRET_KEY     secret key for RANGE_IT_S3_ENDPOINT
//	RANGE_IT_DROP_RANGE_LB     round-robin proxy to the same store where every second request has Range removed
//	RANGE_IT_STATIC_LB         round-robin proxy where every second request goes to a static server that ignores Range
//	RANGE_IT_STATIC_DIR        document root of that static server
//	RANGE_IT_CACHING_PROXY     caching proxy to the same store whose cache key leaves out Range
//	RANGE_IT_NO_RANGE_SERVER   static server that never honors Range, serving RANGE_IT_STATIC_DIR
//
// The helpers and the tests in this file hold for every fix of the parallel cache download. The
// tests of one fix are in a file of their own.

const (
	rangeITBucket    = "runner-range-it"
	rangeITChunkSize = 1 << 20
	rangeITSize      = 8<<20 + 12345
)

type rangeITStore struct {
	endpoint string
	client   *s3.Client
	creds    aws.CredentialsProvider
}

func rangeITEnv(t *testing.T, name string) string {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		t.Skipf("%s is not set", name)
	}
	return v
}

func newRangeITStore(t *testing.T) *rangeITStore {
	t.Helper()
	endpoint := rangeITEnv(t, "RANGE_IT_S3_ENDPOINT")
	creds := credentials.NewStaticCredentialsProvider(
		rangeITEnv(t, "RANGE_IT_S3_ACCESS_KEY"), rangeITEnv(t, "RANGE_IT_S3_SECRET_KEY"), "")

	store := &rangeITStore{endpoint: endpoint, creds: creds, client: rangeITClient(endpoint, creds)}
	_, err := store.client.CreateBucket(t.Context(), &s3.CreateBucketInput{Bucket: aws.String(rangeITBucket)})
	var owned *types.BucketAlreadyOwnedByYou
	var exists *types.BucketAlreadyExists
	if err != nil && !errors.As(err, &owned) && !errors.As(err, &exists) {
		require.NoError(t, err)
	}
	return store
}

func rangeITClient(endpoint string, creds aws.CredentialsProvider) *s3.Client {
	return s3.New(s3.Options{
		BaseEndpoint: aws.String(endpoint),
		Region:       "us-east-1",
		Credentials:  creds,
		UsePathStyle: true,
	})
}

func (s *rangeITStore) put(t *testing.T, key string, body []byte) {
	t.Helper()
	_, err := s.client.PutObject(t.Context(), &s3.PutObjectInput{
		Bucket: aws.String(rangeITBucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body),
	})
	require.NoError(t, err)
}

// presign signs a GET for key against endpoint. Presigned URLs sign only the host, so a proxy
// in front of the store can serve them as long as it keeps the Host header.
func (s *rangeITStore) presign(t *testing.T, endpoint, key string) string {
	t.Helper()
	req, err := s3.NewPresignClient(rangeITClient(endpoint, s.creds)).PresignGetObject(t.Context(),
		&s3.GetObjectInput{Bucket: aws.String(rangeITBucket), Key: aws.String(key)},
		s3.WithPresignExpires(15*time.Minute))
	require.NoError(t, err)
	return req.URL
}

func rangeITPayload(t *testing.T) []byte {
	t.Helper()
	b := make([]byte, rangeITSize)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return b
}

func rangeITKey(t *testing.T) string {
	return fmt.Sprintf("%s-%d.zip", strings.ReplaceAll(t.Name(), "/", "-"), time.Now().UnixNano())
}

func rangeITCommand(rawURL string) *CacheExtractorCommand {
	return &CacheExtractorCommand{
		File:               rangeITArchive,
		URL:                rawURL,
		ChunkSize:          rangeITChunkSize,
		Concurrency:        4,
		TransferBufferSize: defaultCacheTransferBufferSize,
	}
}

// requireNotCorrupt fails when the archive exists and holds anything other than one of the
// wanted payloads. A missing archive is a cache miss, which is acceptable, and returns false.
func requireNotCorrupt(t *testing.T, wants ...[]byte) bool {
	t.Helper()
	got, err := os.ReadFile(rangeITArchive)
	if os.IsNotExist(err) {
		return false
	}
	require.NoError(t, err)
	for _, want := range wants {
		if bytes.Equal(got, want) {
			return true
		}
	}
	require.Failf(t, "corrupt archive on disk", "archive has %d bytes and matches no expected object", len(got))
	return false
}

func TestPresignedParallelDownloadRealStore(t *testing.T) {
	t.Setenv(featureflags.UseParallelCacheTransfer, "true")
	store := newRangeITStore(t)
	rangeITChdir(t)

	key := rangeITKey(t)
	payload := rangeITPayload(t)
	store.put(t, key, payload)

	done, err := rangeITCommand(store.presign(t, store.endpoint, key)).tryPresignedParallelDownload()
	require.NoError(t, err)
	require.True(t, done)
	require.True(t, requireNotCorrupt(t, payload), "archive should be downloaded")
}

// A load balancer where one node ignores Range answers some chunk requests with 200 and the
// whole object.
func TestPresignedParallelDownloadRangeDroppingLoadBalancer(t *testing.T) {
	t.Setenv(featureflags.UseParallelCacheTransfer, "true")
	store := newRangeITStore(t)
	lb := rangeITEnv(t, "RANGE_IT_DROP_RANGE_LB")
	rangeITChdir(t)

	key := rangeITKey(t)
	payload := rangeITPayload(t)
	store.put(t, key, payload)

	err := rangeITCommand(store.presign(t, lb, key)).handlePresignedURL()
	t.Logf("handlePresignedURL: err=%v", err)
	requireNotCorrupt(t, payload)
}

// A caching proxy whose cache key leaves out Range serves one stored response for every range.
func TestPresignedParallelDownloadRangeBlindCachingProxy(t *testing.T) {
	t.Setenv(featureflags.UseParallelCacheTransfer, "true")
	store := newRangeITStore(t)
	proxy := rangeITEnv(t, "RANGE_IT_CACHING_PROXY")
	rangeITChdir(t)

	key := rangeITKey(t)
	payload := rangeITPayload(t)
	store.put(t, key, payload)

	err := rangeITCommand(store.presign(t, proxy, key)).handlePresignedURL()
	t.Logf("handlePresignedURL: err=%v", err)
	requireNotCorrupt(t, payload)
}

// A server that never honors Range must take the sequential path and produce the object.
func TestPresignedDownloadServerWithoutRangeSupport(t *testing.T) {
	t.Setenv(featureflags.UseParallelCacheTransfer, "true")
	server := rangeITEnv(t, "RANGE_IT_NO_RANGE_SERVER")
	dir := rangeITEnv(t, "RANGE_IT_STATIC_DIR")
	rangeITChdir(t)

	key := rangeITKey(t)
	payload := rangeITPayload(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, key), payload, 0o644))
	t.Cleanup(func() { _ = os.Remove(filepath.Join(dir, key)) })

	err := rangeITCommand(server + "/" + key).handlePresignedURL()
	require.NoError(t, err)
	require.True(t, requireNotCorrupt(t, payload), "archive should be downloaded")
}

func TestGoCloudParallelDownloadRealStore(t *testing.T) {
	store := newRangeITStore(t)
	require.True(t, testGoCloudParallelDownload(t, store, store.endpoint, func(string, []byte) {}), "archive should be downloaded")
}

// testGoCloudParallelDownload uploads an object, hands it to mirror, downloads it with the
// GoCloud URL of endpoint and reports whether the archive is on disk.
func testGoCloudParallelDownload(t *testing.T, store *rangeITStore, endpoint string, mirror func(string, []byte)) bool {
	t.Helper()
	t.Setenv(featureflags.UseParallelCacheTransfer, "true")
	creds, err := store.creds.Retrieve(t.Context())
	require.NoError(t, err)
	t.Setenv("AWS_ACCESS_KEY_ID", creds.AccessKeyID)
	t.Setenv("AWS_SECRET_ACCESS_KEY", creds.SecretAccessKey)
	rangeITChdir(t)

	key := rangeITKey(t)
	payload := rangeITPayload(t)
	store.put(t, key, payload)
	mirror(key, payload)

	q := url.Values{}
	q.Set("endpoint", endpoint)
	q.Set("region", "us-east-1")
	q.Set("use_path_style", "true")
	q.Set("disable_https", "true")

	cmd := rangeITCommand("")
	cmd.GoCloudURL = fmt.Sprintf("s3://%s/%s?%s", rangeITBucket, key, q.Encode())

	err = cmd.handleGoCloudURL()
	t.Logf("handleGoCloudURL: err=%v", err)
	return requireNotCorrupt(t, payload)
}

const rangeITArchive = "archive.zip"

// rangeITChdir runs the test in an empty directory, where the extractor writes rangeITArchive.
func rangeITChdir(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
}
