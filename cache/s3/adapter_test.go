package s3

import (
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var defaultTimeout = 1 * time.Hour

func defaultCacheFactory() *common.CacheConfig {
	return &common.CacheConfig{
		Type: "s3",
		S3: &common.CacheS3Config{
			ServerAddress:  "server.com",
			AccessKey:      "access",
			SecretKey:      "key",
			BucketName:     "test",
			BucketLocation: "location"},
	}
}

type cacheOperationTest struct {
	errorOnMinioClientInitialization bool
	errorOnURLPresigning             bool

	presignedURL *url.URL
	expectedURL  *url.URL
}

func onFakeMinioURLGenerator(tc cacheOperationTest) func() {
	client := new(mockMinioClient)

	var err error
	if tc.errorOnURLPresigning {
		err = errors.New("test error")
	}

	client.On("PresignedGetObject", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(tc.presignedURL, err)
	client.On("PresignedPutObject", mock.Anything, mock.Anything, mock.Anything).Return(tc.presignedURL, err)

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

func testCacheOperation(t *testing.T, operationName string, operation func(adapter cache.Adapter) *url.URL, tc cacheOperationTest) {
	t.Run(operationName, func(t *testing.T) {
		cleanupMinioURLGeneratorMock := onFakeMinioURLGenerator(tc)
		defer cleanupMinioURLGeneratorMock()

		cacheConfig := defaultCacheFactory()

		adapter, err := New(cacheConfig, defaultTimeout, "key")

		if tc.errorOnMinioClientInitialization {
			assert.EqualError(t, err, "error while creating S3 cache storage client: test error")

			return
		}
		require.NoError(t, err)

		URL := operation(adapter)
		assert.Equal(t, tc.expectedURL, URL)
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
			testCacheOperation(t, "GetDownloadURL", func(adapter cache.Adapter) *url.URL { return adapter.GetDownloadURL() }, test)
			testCacheOperation(t, "GetUploadURL", func(adapter cache.Adapter) *url.URL { return adapter.GetUploadURL() }, test)
		})
	}
}

// TODO: Remove in 12.0
func deprecatedCacheFactory() *common.CacheConfig {
	return &common.CacheConfig{
		Type:           "s3",
		ServerAddress:  "server.com",
		AccessKey:      "access",
		SecretKey:      "key",
		BucketName:     "test",
		BucketLocation: "location",
	}
}

// TODO: Remove in 12.0
func TestS3DeprecatedConfigFormatDetection(t *testing.T) {
	hook := test.NewGlobal()
	s3Cache := deprecatedCacheFactory()

	adapter, err := New(s3Cache, defaultTimeout, "key")
	require.NoError(t, err)

	url := adapter.GetDownloadURL()
	assert.NotNil(t, url)

	entries := hook.AllEntries()
	message, err := entries[0].String()
	require.NoError(t, err)
	assert.Contains(t, message, "Runner uses S3 caching with deprecated configuration format")
}

func TestNoConfiguration(t *testing.T) {
	oldDeprecatedConfigHandler := deprecatedConfigHandler
	defer func() {
		deprecatedConfigHandler = oldDeprecatedConfigHandler
	}()
	deprecatedConfigHandler = func(config *common.CacheConfig) *common.CacheConfig {
		return config
	}

	s3Cache := defaultCacheFactory()
	s3Cache.S3 = nil

	adapter, err := New(s3Cache, defaultTimeout, "key")
	assert.Nil(t, adapter)

	assert.EqualError(t, err, "missing S3 configuration")
}
