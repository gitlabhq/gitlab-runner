//go:build integration

package network

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

// These tests download artifacts in parallel from a real object store. A stub coordinator answers
// the artifact request with a 302 to a presigned URL, as GitLab does for direct downloads.
// Endpoints come from the same RANGE_IT_* variables as the cache extractor integration tests.
// Every chunk request carries If-Match with the probe's ETag, so a store that refused that
// unsigned header on a presigned URL would fail the parallel path of every download here.

const rangeITArtifactBucket = "runner-range-it"

func rangeITArtifactEnv(t *testing.T, name string) string {
	t.Helper()
	v := os.Getenv(name)
	if v == "" {
		t.Skipf("%s is not set", name)
	}
	return v
}

func rangeITArtifactS3(endpoint string, creds aws.CredentialsProvider) *s3.Client {
	return s3.New(s3.Options{
		BaseEndpoint: aws.String(endpoint),
		Region:       "us-east-1",
		Credentials:  creds,
		UsePathStyle: true,
	})
}

// rangeITArtifactObject uploads a random object larger than two parallel chunks and returns it
// with a presigned GET URL for it on urlEndpoint.
func rangeITArtifactObject(t *testing.T, urlEndpoint string) ([]byte, string) {
	t.Helper()
	endpoint := rangeITArtifactEnv(t, "RANGE_IT_S3_ENDPOINT")
	creds := credentials.NewStaticCredentialsProvider(
		rangeITArtifactEnv(t, "RANGE_IT_S3_ACCESS_KEY"), rangeITArtifactEnv(t, "RANGE_IT_S3_SECRET_KEY"), "")
	if urlEndpoint == "" {
		urlEndpoint = endpoint
	}

	client := rangeITArtifactS3(endpoint, creds)
	_, err := client.CreateBucket(t.Context(), &s3.CreateBucketInput{Bucket: aws.String(rangeITArtifactBucket)})
	if err != nil && !strings.Contains(err.Error(), "BucketAlready") {
		require.NoError(t, err)
	}

	payload := make([]byte, 2*artifactParallelChunkSize+12345)
	_, err = rand.Read(payload)
	require.NoError(t, err)

	key := fmt.Sprintf("%s-%d.zip", t.Name(), time.Now().UnixNano())
	_, err = client.PutObject(t.Context(), &s3.PutObjectInput{
		Bucket: aws.String(rangeITArtifactBucket), Key: aws.String(key), Body: bytes.NewReader(payload),
	})
	require.NoError(t, err)

	req, err := s3.NewPresignClient(rangeITArtifactS3(urlEndpoint, creds)).PresignGetObject(t.Context(),
		&s3.GetObjectInput{Bucket: aws.String(rangeITArtifactBucket), Key: aws.String(key)},
		s3.WithPresignExpires(15*time.Minute))
	require.NoError(t, err)
	return payload, req.URL
}

// rangeITDownloadArtifacts runs DownloadArtifacts with direct download and parallel transfer
// enabled, and returns the state, the file content and whether the parallel path finished.
func rangeITDownloadArtifacts(t *testing.T, objectURL string) (common.DownloadState, []byte, bool) {
	t.Helper()
	t.Setenv(featureflags.UseParallelArtifactTransfer, "true")

	coordinator := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v4/jobs/10/artifacts" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, objectURL, http.StatusFound)
	}))
	t.Cleanup(coordinator.Close)

	hook := test.NewGlobal()
	t.Cleanup(hook.Reset)

	path := filepath.Join(t.TempDir(), "artifacts.zip")
	dest, err := os.Create(path)
	require.NoError(t, err)

	directDownload := true
	state := NewGitLabClient().DownloadArtifacts(
		common.JobCredentials{ID: 10, URL: coordinator.URL, Token: "token"}, dest, &directDownload)
	require.NoError(t, dest.Close())

	got, err := os.ReadFile(path)
	require.NoError(t, err)

	started, failed := false, false
	for _, e := range hook.AllEntries() {
		switch {
		case strings.HasPrefix(e.Message, "Artifact download: parallel failed"):
			failed = true
			t.Logf("parallel download failed: %v", e.Data[logrus.ErrorKey])
		case strings.HasPrefix(e.Message, "Artifact download: parallel,"):
			started = true
		}
	}
	return state, got, started && !failed
}

func TestDownloadArtifactsParallelRealStore(t *testing.T) {
	payload, objectURL := rangeITArtifactObject(t, "")

	state, got, parallel := rangeITDownloadArtifacts(t, objectURL)
	require.Equal(t, common.DownloadSucceeded, state)
	require.True(t, parallel, "the parallel path should finish against a store that honors Range")
	require.True(t, bytes.Equal(payload, got), "artifact content differs from the object (%d bytes, want %d)", len(got), len(payload))
}

// A load balancer where one node ignores Range answers some chunk requests with 200 and the
// whole object. The download must still produce the object.
func TestDownloadArtifactsParallelRangeDroppingLoadBalancer(t *testing.T) {
	payload, objectURL := rangeITArtifactObject(t, rangeITArtifactEnv(t, "RANGE_IT_DROP_RANGE_LB"))

	state, got, parallel := rangeITDownloadArtifacts(t, objectURL)
	t.Logf("state=%v parallel=%v", state, parallel)
	require.Equal(t, common.DownloadSucceeded, state)
	require.True(t, bytes.Equal(payload, got), "artifact content differs from the object (%d bytes, want %d)", len(got), len(payload))
}
