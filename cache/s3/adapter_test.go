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
	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
)

var defaultTimeout = 1 * time.Hour

const (
	bucketName     = "test"
	objectName     = "key"
	bucketLocation = "location"
)

func defaultCacheFactory() *cacheconfig.Config {
	return &cacheconfig.Config{
		Type: "s3",
		S3: &cacheconfig.CacheS3Config{
			ServerAddress:  "server.com",
			AccessKey:      "access",
			SecretKey:      "key",
			BucketName:     bucketName,
			BucketLocation: bucketLocation},
	}
}

func defaultCacheFactoryEncryptionAES() *cacheconfig.Config {
	cacheConfig := defaultCacheFactory()
	cacheConfig.S3.ServerSideEncryption = "S3"
	return cacheConfig
}

func defaultCacheFactoryEncryptionKMS() *cacheconfig.Config {
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
	metadata              map[string]string
}

func onFakeMinioURLGenerator(t *testing.T, tc cacheOperationTest) {
	client := newMockMinioClient(t)

	var err error
	if tc.errorOnURLPresigning {
		err = errors.New("test error")
	}

	client.
		On(
			"PresignHeader", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		).
		Return(tc.presignedURL, err).Maybe()

	oldNewMinioURLGenerator := newMinioClient
	newMinioClient = func(s3 *cacheconfig.CacheS3Config) (minioClient, error) {
		if tc.errorOnMinioClientInitialization {
			return nil, errors.New("test error")
		}
		return client, nil
	}

	t.Cleanup(func() {
		newMinioClient = oldNewMinioURLGenerator
	})
}

func testCacheOperation(
	t *testing.T,
	operationName string,
	operation func(adapter cache.Adapter) cache.PresignedURL,
	tc cacheOperationTest,
	cacheConfig *cacheconfig.Config,
) {
	t.Run(operationName, func(t *testing.T) {
		onFakeMinioURLGenerator(t, tc)

		adapter, err := New(cacheConfig, defaultTimeout, objectName)

		if tc.errorOnMinioClientInitialization {
			assert.EqualError(t, err, "error while creating S3 cache storage client: test error")

			return
		}
		require.NoError(t, err)

		adapter.WithMetadata(tc.metadata)

		u := operation(adapter)
		assert.Equal(t, tc.expectedURL, u.URL)

		uploadHeaders := u.Headers
		if operationName == "GetDownloadURL" {
			assert.Empty(t, uploadHeaders)
		} else {
			if tc.expectedUploadHeaders != nil {
				expectedUploadHeaders := tc.expectedUploadHeaders
				assert.Len(t, uploadHeaders, len(expectedUploadHeaders))
				assert.True(
					t, reflect.DeepEqual(expectedUploadHeaders, uploadHeaders),
					"headers are not equal:\nexpected %q\nactual: %q", expectedUploadHeaders, uploadHeaders,
				)
			} else {
				assert.Empty(t, uploadHeaders)
			}
		}

		goCloudURL, err := adapter.GetGoCloudURL(t.Context(), true)
		assert.NoError(t, err)
		assert.Nil(t, goCloudURL.URL)
		assert.Empty(t, goCloudURL.Environment)

		goCloudURL, err = adapter.GetGoCloudURL(t.Context(), false)
		assert.NoError(t, err)
		assert.Nil(t, goCloudURL.URL)
		assert.Empty(t, goCloudURL.Environment)
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
		"presigned-url-with-metadata": {
			presignedURL: URL,
			expectedURL:  URL,
			metadata:     map[string]string{"foo": "some foo"},
			expectedUploadHeaders: http.Header{
				"X-Amz-Meta-Foo": []string{"some foo"},
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			testCacheOperation(
				t,
				"GetDownloadURL",
				func(adapter cache.Adapter) cache.PresignedURL { return adapter.GetDownloadURL(t.Context()) },
				test,
				defaultCacheFactory(),
			)
			testCacheOperation(
				t,
				"GetUploadURL",
				func(adapter cache.Adapter) cache.PresignedURL { return adapter.GetUploadURL(t.Context()) },
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
			expectedUploadHeaders: nil,
		},
		"presigned-url-aes": {
			presignedURL:          URL,
			expectedURL:           URL,
			expectedUploadHeaders: headers,
		},
		"presigned-url-aes-with-metdata": {
			presignedURL: URL,
			expectedURL:  URL,
			metadata:     map[string]string{"foo": "some foo"},
			expectedUploadHeaders: func() http.Header {
				h := headers.Clone()
				h["X-Amz-Meta-Foo"] = []string{"some foo"}
				return h
			}(),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			testCacheOperation(
				t,
				"GetDownloadURL",
				func(adapter cache.Adapter) cache.PresignedURL { return adapter.GetDownloadURL(t.Context()) },
				test,
				defaultCacheFactoryEncryptionAES(),
			)
			testCacheOperation(
				t,
				"GetUploadURL",
				func(adapter cache.Adapter) cache.PresignedURL { return adapter.GetUploadURL(t.Context()) },
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
			expectedUploadHeaders:            nil,
		},
		"error-on-presigning-url": {
			errorOnURLPresigning:  true,
			presignedURL:          URL,
			expectedURL:           nil,
			expectedUploadHeaders: nil,
		},
		"presigned-url-kms": {
			presignedURL:          URL,
			expectedURL:           URL,
			expectedUploadHeaders: headers,
		},
		"presigned-url-kms-with-metadata": {
			presignedURL: URL,
			expectedURL:  URL,
			metadata:     map[string]string{"foo": "some foo"},
			expectedUploadHeaders: func() http.Header {
				h := headers.Clone()
				h["X-Amz-Meta-Foo"] = []string{"some foo"}
				return h
			}(),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			testCacheOperation(
				t,
				"GetDownloadURL",
				func(adapter cache.Adapter) cache.PresignedURL { return adapter.GetDownloadURL(t.Context()) },
				test,
				defaultCacheFactoryEncryptionKMS(),
			)
			testCacheOperation(
				t,
				"GetUploadURL",
				func(adapter cache.Adapter) cache.PresignedURL { return adapter.GetUploadURL(t.Context()) },
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
