//go:build !integration

package s3v2

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/johannesboyne/gofakes3"
	"github.com/johannesboyne/gofakes3/backend/s3mem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func setupMockS3Server(t *testing.T) *common.CacheS3Config {
	backend := s3mem.New()
	server := gofakes3.New(backend)
	ts := httptest.NewServer(server.Server())
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	url, err := url.Parse(ts.URL)
	require.NoError(t, err)

	s3Config := &common.CacheS3Config{
		ServerAddress:  url.Host,
		Insecure:       true,
		BucketLocation: "us-west-1",
		BucketName:     "test-bucket",
		AccessKey:      "test-access-key",
		SecretKey:      "test-secret-key",
	}

	t.Cleanup(func() {
		ts.Close()
	})

	client, err := newRawS3Client(s3Config)
	require.NoError(t, err)

	_, err = client.CreateBucket(ctx, &s3.CreateBucketInput{
		Bucket: aws.String(s3Config.BucketName),
	})
	require.NoError(t, err)

	return s3Config
}

func TestNewS3ClientOptions(t *testing.T) {
	tests := map[string]struct {
		s3Config         common.CacheS3Config
		expectedRegion   string
		expectedScheme   string
		usePathStyle     bool
		useAccelerate    bool
		expectedEndpoint string
	}{
		"s3-standard": {
			s3Config: common.CacheS3Config{
				AccessKey:      "test-access-key",
				SecretKey:      "test-secret-key",
				BucketName:     "test-bucket",
				BucketLocation: "us-west-2",
				Insecure:       false,
			},
			expectedRegion:   "us-west-2",
			expectedScheme:   "https",
			usePathStyle:     false,
			useAccelerate:    false,
			expectedEndpoint: "",
		},
		"s3-iam-profile": {
			s3Config: common.CacheS3Config{
				BucketName:     "test-bucket",
				BucketLocation: "us-west-2",
				Insecure:       false,
			},
			expectedRegion:   "us-west-2",
			expectedScheme:   "https",
			usePathStyle:     false,
			useAccelerate:    false,
			expectedEndpoint: "",
		},
		"s3-accelerate": {
			s3Config: common.CacheS3Config{
				ServerAddress:  "s3-accelerate.amazonaws.com",
				AccessKey:      "test-access-key",
				SecretKey:      "test-secret-key",
				BucketName:     "test-bucket",
				BucketLocation: "us-east-1",
				Insecure:       false,
			},
			expectedRegion:   "us-east-1",
			expectedScheme:   "https",
			usePathStyle:     false,
			useAccelerate:    true,
			expectedEndpoint: "https://s3-accelerate.amazonaws.com",
		},
		"s3-custom-endpoint": {
			s3Config: common.CacheS3Config{
				ServerAddress:  "localhost:9000",
				BucketName:     "test-bucket",
				BucketLocation: "us-west-2",
				Insecure:       true,
			},
			expectedRegion:   "us-west-2",
			expectedScheme:   "http",
			usePathStyle:     true, // Not virtual-host compatible
			useAccelerate:    false,
			expectedEndpoint: "http://localhost:9000",
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			client, err := newS3Client(&tt.s3Config)
			require.NoError(t, err)

			clientOptions := client.(*s3Client).client.Options()

			require.Equal(t, tt.expectedRegion, clientOptions.Region)
			require.Equal(t, tt.useAccelerate, clientOptions.UseAccelerate)
			require.Equal(t, tt.usePathStyle, clientOptions.UsePathStyle)

			if tt.expectedEndpoint == "" {
				require.Nil(t, clientOptions.BaseEndpoint)
			} else {
				require.Equal(t, tt.expectedEndpoint, *clientOptions.BaseEndpoint)
			}
		})
	}
}

func TestS3Client_PresignURL(t *testing.T) {
	s3Config := setupMockS3Server(t)

	tests := map[string]struct {
		encryptionType     string
		encryptionKeyID    string
		accessKey          string
		secretKey          string
		expectedEncryption string
		expectedKMSKeyID   string
	}{
		"no-encryption-with-credentials": {
			encryptionType:     "",
			accessKey:          "test-access-key",
			secretKey:          "test-secret-key",
			expectedEncryption: "",
			expectedKMSKeyID:   "",
		},
		"s3-encryption-with-credentials": {
			encryptionType:     "S3",
			accessKey:          "test-access-key",
			secretKey:          "test-secret-key",
			expectedEncryption: "AES256",
			expectedKMSKeyID:   "",
		},
		"kms-encryption-with-credentials": {
			encryptionType:     "KMS",
			encryptionKeyID:    "alias/my-key",
			accessKey:          "test-access-key",
			secretKey:          "test-secret-key",
			expectedEncryption: "aws:kms",
			expectedKMSKeyID:   "alias/my-key",
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			s3Config.ServerSideEncryption = tt.encryptionType
			s3Config.ServerSideEncryptionKeyID = tt.encryptionKeyID
			s3Config.AccessKey = tt.accessKey
			s3Config.SecretKey = tt.secretKey

			s3Client, err := newS3Client(s3Config)
			require.NoError(t, err)

			// Presign a PUT request to upload an object
			objectName := "test-object"
			url, err := s3Client.PresignURL(context.Background(), http.MethodPut, s3Config.BucketName, objectName, 5*time.Minute)
			require.NoError(t, err)

			// Verify encryption headers
			if tt.expectedEncryption != "" {
				assert.Equal(t, tt.expectedEncryption, url.Headers.Get("x-amz-server-side-encryption"))
			}

			if tt.expectedKMSKeyID != "" {
				assert.Equal(t, tt.expectedKMSKeyID, url.Headers.Get("x-amz-server-side-encryption-aws-kms-key-id"))
			}

			// Use the presigned URL to upload an object
			content := []byte("Hello, world!")
			req, err := http.NewRequest(http.MethodPut, url.URL.String(), bytes.NewReader(content))
			require.NoError(t, err)

			client := &http.Client{}
			resp, err := client.Do(req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			resp.Body.Close()

			// Presign a GET request to download the object
			url, err = s3Client.PresignURL(context.Background(), http.MethodGet, s3Config.BucketName, objectName, 5*time.Minute)
			require.NoError(t, err)

			req, err = http.NewRequest(http.MethodGet, url.URL.String(), bytes.NewReader(content))
			require.NoError(t, err)

			resp, err = client.Do(req)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			resp.Body.Close()

			assert.Equal(t, content, body)
		})
	}
}
