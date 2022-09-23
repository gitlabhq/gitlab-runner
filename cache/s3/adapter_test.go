//go:build !integration

package s3

import (
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var defaultTimeout = 1 * time.Hour

const (
	bucketName     = "test"
	objectName     = "key"
	bucketLocation = "location"
)

func defaultCacheFactory() *common.CacheConfig {
	return &common.CacheConfig{
		Type: "s3",
		S3: &common.CacheS3Config{
			ServerAddress:  "server.com",
			AccessKey:      "access",
			SecretKey:      "key",
			BucketName:     bucketName,
			BucketLocation: bucketLocation},
	}
}

func defaultCacheFactoryEncryptionAES() *common.CacheConfig {
	cacheConfig := defaultCacheFactory()
	cacheConfig.S3.ServerSideEncryption = "S3"
	return cacheConfig
}

func defaultCacheFactoryEncryptionKMS() *common.CacheConfig {
	cacheConfig := defaultCacheFactory()
	cacheConfig.S3.ServerSideEncryption = "KMS"
	cacheConfig.S3.ServerSideEncryptionKeyID = "alias/my-key"
	return cacheConfig
}

type cacheOperationTest struct {
	errorOnMinioClientInitialization bool
	errorOnURLPresigning             bool

	presignedURL          *url.URL
	expectedURL           *url.URL
	expectedUploadHeaders http.Header
}

func onFakeMinioURLGenerator(tc cacheOperationTest) func() {
	client := new(mockMinioClient)

	var err error
	if tc.errorOnURLPresigning {
		err = errors.New("test error")
	}

	client.
		On(
			"PresignHeader", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).
		Return(tc.presignedURL, err)

	oldNewMinioURLGenerator := newMinioClient
	newMinioClient = func(s3 *common.CacheS3Config) (minioClient, error) {
		if tc.errorOnMinioClientInitialization {
			return nil, errors.New("test error")
		}
		return client, nil
	}

	return func() {
		newMinioClient = oldNewMinioURLGenerator
	}
}

func testCacheOperation(
	t *testing.T,
	operationName string,
	operation func(adapter cache.Adapter) *url.URL,
	tc cacheOperationTest,
	cacheConfig *common.CacheConfig,
) {
	t.Run(operationName, func(t *testing.T) {
		cleanupMinioURLGeneratorMock := onFakeMinioURLGenerator(tc)
		defer cleanupMinioURLGeneratorMock()

		adapter, err := New(cacheConfig, defaultTimeout, objectName)

		if tc.errorOnMinioClientInitialization {
			assert.EqualError(t, err, "error while creating S3 cache storage client: test error")

			return
		}
		require.NoError(t, err)

		URL := operation(adapter)
		assert.Equal(t, tc.expectedURL, URL)

		uploadHeaders := adapter.GetUploadHeaders()
		if tc.expectedUploadHeaders != nil {
			require.NotNil(t, uploadHeaders)
			expectedUploadHeaders := tc.expectedUploadHeaders
			assert.Len(t, uploadHeaders, len(expectedUploadHeaders))
			assert.True(t, reflect.DeepEqual(expectedUploadHeaders, uploadHeaders))
		} else {
			assert.Nil(t, uploadHeaders)
		}

		assert.Nil(t, adapter.GetGoCloudURL())
		assert.Empty(t, adapter.GetUploadEnv())
	})
}

func TestCacheOperation(t *testing.T) {
	URL, err := url.Parse("https://s3.example.com")
	require.NoError(t, err)

	tests := map[string]cacheOperationTest{
		"error-on-minio-client-initialization": {
			errorOnMinioClientInitialization: true,
		},
		"error-on-presigning-url": {
			errorOnURLPresigning: true,
			presignedURL:         URL,
			expectedURL:          nil,
		},
		"presigned-url": {
			presignedURL: URL,
			expectedURL:  URL,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			testCacheOperation(
				t,
				"GetDownloadURL",
				func(adapter cache.Adapter) *url.URL { return adapter.GetDownloadURL() },
				test,
				defaultCacheFactory(),
			)
			testCacheOperation(
				t,
				"GetUploadURL",
				func(adapter cache.Adapter) *url.URL { return adapter.GetUploadURL() },
				test,
				defaultCacheFactory(),
			)
		})
	}
}

func TestCacheOperationEncryptionAES(t *testing.T) {
	URL, err := url.Parse("https://s3.example.com")
	require.NoError(t, err)
	headers := http.Header{}
	headers.Add("X-Amz-Server-Side-Encryption", "AES256")

	tests := map[string]cacheOperationTest{
		"error-on-minio-client-initialization": {
			errorOnMinioClientInitialization: true,
			expectedUploadHeaders:            headers,
		},
		"error-on-presigning-url": {
			errorOnURLPresigning:  true,
			presignedURL:          URL,
			expectedURL:           nil,
			expectedUploadHeaders: headers,
		},
		"presigned-url-aes": {
			presignedURL:          URL,
			expectedURL:           URL,
			expectedUploadHeaders: headers,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			testCacheOperation(
				t,
				"GetDownloadURL",
				func(adapter cache.Adapter) *url.URL { return adapter.GetDownloadURL() },
				test,
				defaultCacheFactoryEncryptionAES(),
			)
			testCacheOperation(
				t,
				"GetUploadURL",
				func(adapter cache.Adapter) *url.URL { return adapter.GetUploadURL() },
				test,
				defaultCacheFactoryEncryptionAES(),
			)
		})
	}
}

func TestCacheOperationEncryptionKMS(t *testing.T) {
	URL, err := url.Parse("https://s3.example.com")
	require.NoError(t, err)
	headers := http.Header{}
	headers.Add("X-Amz-Server-Side-Encryption", "aws:kms")
	headers.Add("X-Amz-Server-Side-Encryption-Aws-Kms-Key-Id", "alias/my-key")

	tests := map[string]cacheOperationTest{
		"error-on-minio-client-initialization": {
			errorOnMinioClientInitialization: true,
			expectedUploadHeaders:            headers,
		},
		"error-on-presigning-url": {
			errorOnURLPresigning:  true,
			presignedURL:          URL,
			expectedURL:           nil,
			expectedUploadHeaders: headers,
		},
		"presigned-url-kms": {
			presignedURL:          URL,
			expectedURL:           URL,
			expectedUploadHeaders: headers,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			testCacheOperation(
				t,
				"GetDownloadURL",
				func(adapter cache.Adapter) *url.URL { return adapter.GetDownloadURL() },
				test,
				defaultCacheFactoryEncryptionKMS(),
			)
			testCacheOperation(
				t,
				"GetUploadURL",
				func(adapter cache.Adapter) *url.URL { return adapter.GetUploadURL() },
				test,
				defaultCacheFactoryEncryptionKMS(),
			)
		})
	}
}

func TestNoConfiguration(t *testing.T) {
	s3Cache := defaultCacheFactory()
	s3Cache.S3 = nil

	adapter, err := New(s3Cache, defaultTimeout, objectName)
	assert.Nil(t, adapter)

	assert.EqualError(t, err, "missing S3 configuration")
}
