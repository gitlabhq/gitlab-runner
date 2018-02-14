package shells

import (
	"testing"

	"github.com/minio/minio-go/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func defaultS3CacheFactory() *common.CacheConfig {
	return &common.CacheConfig{
		Type:           "s3",
		BucketName:     "test",
		BucketLocation: "location",
		ServerAddress:  "server.com",
		AccessKey:      "access",
		SecretKey:      "key",
	}
}

func iamS3CacheFactory(t *testing.T) (*common.CacheConfig, *mockFakeIAMCredentialsProvider) {
	cacheConfig := defaultS3CacheFactory()
	cacheConfig.ServerAddress = ""
	cacheConfig.AccessKey = ""
	cacheConfig.SecretKey = ""

	iamProvider := &mockFakeIAMCredentialsProvider{}
	iamFactory = func() *credentials.Credentials {
		return credentials.New(iamProvider)
	}

	return cacheConfig, iamProvider
}

func defaults3CacheBuild(cacheConfig *common.CacheConfig) *common.Build {
	return &common.Build{
		JobResponse: common.JobResponse{
			JobInfo: common.JobInfo{
				ProjectID: 10,
			},
			RunnerInfo: common.RunnerInfo{
				Timeout: 3600,
			},
		},
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{
				Token: "longtoken",
			},
			RunnerSettings: common.RunnerSettings{
				Cache: cacheConfig,
			},
		},
	}
}

func TestS3CacheUploadURL(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	s3Cache.Insecure = false
	s3CacheBuild := defaults3CacheBuild(s3Cache)
	url := getCacheUploadURL(s3CacheBuild, "key")
	require.NotNil(t, url)
	assert.Equal(t, s3Cache.ServerAddress, url.Host)
	assert.Regexp(t, "^https://", url)
	assert.Contains(t, url.String(), "X-Amz-Credential=access%2F")
}

func TestS3CacheUploadURLForIamCredentials(t *testing.T) {
	s3Cache, iamProvider := iamS3CacheFactory(t)

	fakeValue := credentials.Value{
		AccessKeyID:     "access-from-iam",
		SecretAccessKey: "secret-from-iam",
	}
	iamProvider.On("Retrieve").Return(fakeValue, nil).Once()
	iamProvider.On("IsExpired").Return(false)
	defer iamProvider.AssertExpectations(t)

	s3Cache.Insecure = false
	s3CacheBuild := defaults3CacheBuild(s3Cache)
	url := getCacheUploadURL(s3CacheBuild, "key")
	require.NotNil(t, url)
	assert.Equal(t, "test.s3.amazonaws.com", url.Host)
	assert.Regexp(t, "^https://", url)
	assert.Contains(t, url.String(), "X-Amz-Credential=access-from-iam%2F")
}

func TestS3CacheUploadInsecureURL(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	s3Cache.Insecure = true
	s3CacheBuild := defaults3CacheBuild(s3Cache)
	url := getCacheUploadURL(s3CacheBuild, "key")
	require.NotNil(t, url)
	assert.Equal(t, s3Cache.ServerAddress, url.Host)
	assert.Regexp(t, "^http://", url)
}

func TestS3CacheDownloadURL(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	s3Cache.Insecure = false
	s3CacheBuild := defaults3CacheBuild(s3Cache)
	url := getCacheDownloadURL(s3CacheBuild, "key")
	require.NotNil(t, url)
	assert.Equal(t, s3Cache.ServerAddress, url.Host)
	assert.Regexp(t, "^https://", url)
}

func TestS3CacheDownloadInsecureURL(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	s3Cache.Insecure = true
	s3CacheBuild := defaults3CacheBuild(s3Cache)
	url := getCacheDownloadURL(s3CacheBuild, "key")
	require.NotNil(t, url)
	assert.Equal(t, s3Cache.ServerAddress, url.Host)
	assert.Regexp(t, "^http://", url)
}

func TestGetCacheObjectNameWhenKeyIsEmptyResultIsAlsoEmpty(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	s3CacheBuild := defaults3CacheBuild(s3Cache)
	url := getCacheObjectName(s3CacheBuild, s3Cache, "")
	require.Empty(t, url)
}

func TestGetCacheObjectName(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	s3CacheBuild := defaults3CacheBuild(s3Cache)
	url := getCacheObjectName(s3CacheBuild, s3Cache, "key")
	require.Equal(t, "runner/longtoke/project/10/key", url)
}

func TestGetCacheObjectNameWhenPathIsSetThenUrlContainsIt(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	s3Cache.Path = "whatever"
	s3CacheBuild := defaults3CacheBuild(s3Cache)
	url := getCacheObjectName(s3CacheBuild, s3Cache, "key")
	require.Equal(t, "whatever/runner/longtoke/project/10/key", url)
}

func TestGetCacheObjectNameWhenPathHasMultipleSegmentIsSetThenUrlContainsIt(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	s3Cache.Path = "some/other/path/goes/here"
	s3CacheBuild := defaults3CacheBuild(s3Cache)
	url := getCacheObjectName(s3CacheBuild, s3Cache, "key")
	require.Equal(t, "some/other/path/goes/here/runner/longtoke/project/10/key", url)
}

func TestGetCacheObjectNameWhenPathIsNotSetThenUrlDoesNotContainIt(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	s3Cache.Path = ""
	s3CacheBuild := defaults3CacheBuild(s3Cache)
	url := getCacheObjectName(s3CacheBuild, s3Cache, "key")
	require.Equal(t, "runner/longtoke/project/10/key", url)
}

func TestGetCacheObjectNameWhenSharedFlagIsFalseThenRunnerSegmentExistsInTheUrl(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	s3Cache.Shared = false
	s3CacheBuild := defaults3CacheBuild(s3Cache)
	url := getCacheObjectName(s3CacheBuild, s3Cache, "key")
	require.Equal(t, "runner/longtoke/project/10/key", url)
}

func TestGetCacheObjectNameWhenSharedFlagIsFalseThenRunnerSegmentShouldNotBePresent(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	s3Cache.Shared = true
	s3CacheBuild := defaults3CacheBuild(s3Cache)
	url := getCacheObjectName(s3CacheBuild, s3Cache, "key")
	require.Equal(t, "project/10/key", url)
}
