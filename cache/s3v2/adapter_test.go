//go:build !integration

package s3v2

import (
	"context"
	"errors"
	"net/url"
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
		Type: "s3v2",
		S3: &common.CacheS3Config{
			ServerAddress:  "server.com",
			AccessKey:      "access",
			SecretKey:      "key",
			BucketName:     bucketName,
			BucketLocation: bucketLocation},
	}
}

type cacheOperationTest struct {
	errorOnS3ClientInitialization bool
	errorOnURLPresigning          bool

	presignedURL *url.URL
	expectedURL  *url.URL
}

func onFakeS3URLGenerator(tc cacheOperationTest) func() {
	client := new(mockS3Presigner)

	var err error
	if tc.errorOnURLPresigning {
		err = errors.New("test error")
	}

	client.
		On(
			"PresignURL", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything,
		).
		Return(cache.PresignedURL{URL: tc.presignedURL}, err)

	oldS3URLGenerator := newS3Client
	newS3Client = func(s3 *common.CacheS3Config) (s3Presigner, error) {
		if tc.errorOnS3ClientInitialization {
			return nil, errors.New("test error")
		}
		return client, nil
	}

	return func() {
		newS3Client = oldS3URLGenerator
	}
}

func testCacheOperation(
	t *testing.T,
	operationName string,
	operation func(adapter cache.Adapter) cache.PresignedURL,
	tc cacheOperationTest,
	cacheConfig *common.CacheConfig,
) {
	t.Run(operationName, func(t *testing.T) {
		cleanupS3URLGeneratorMock := onFakeS3URLGenerator(tc)
		defer cleanupS3URLGeneratorMock()

		adapter, err := New(cacheConfig, defaultTimeout, objectName)

		if tc.errorOnS3ClientInitialization {
			assert.EqualError(t, err, "error while creating S3 cache storage client: test error")

			return
		}
		require.NoError(t, err)

		URL := operation(adapter)
		assert.Equal(t, tc.expectedURL, URL.URL)

		ctx := context.Background()
		assert.Nil(t, adapter.GetGoCloudURL(ctx))
		assert.Empty(t, adapter.GetUploadEnv(ctx))
	})
}

func TestCacheOperation(t *testing.T) {
	URL, err := url.Parse("https://s3.example.com")
	require.NoError(t, err)

	tests := map[string]cacheOperationTest{
		"error-on-s3-client-initialization": {
			errorOnS3ClientInitialization: true,
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
				func(adapter cache.Adapter) cache.PresignedURL { return adapter.GetDownloadURL(context.Background()) },
				test,
				defaultCacheFactory(),
			)
			testCacheOperation(
				t,
				"GetUploadURL",
				func(adapter cache.Adapter) cache.PresignedURL { return adapter.GetUploadURL(context.Background()) },
				test,
				defaultCacheFactory(),
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
