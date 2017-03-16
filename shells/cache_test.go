package shells

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
)

func defaultS3CacheFactory() *common.CacheConfig {
	return &common.CacheConfig{
		Type:           "s3",
		ServerAddress:  "server.com",
		AccessKey:      "access",
		SecretKey:      "key",
		BucketName:     "test",
		BucketLocation: "location",
	}
}

var s3CacheBuild = &common.Build{
	JobResponse: common.JobResponse{
		JobInfo: common.JRJobInfo{
			ProjectID: 10,
		},
		RunnerInfo: common.JRRunnerInfo{
			Timeout: 3600,
		},
	},
	Runner: &common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "longtoken",
		},
		RunnerSettings: common.RunnerSettings{
			Cache: defaultS3CacheFactory(),
		},
	},
}

func TestS3CacheUploadURL(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	url := getCacheUploadURL(s3CacheBuild, "key")
	require.NotNil(t, url)
	assert.Equal(t, s3Cache.ServerAddress, url.Host)
}

func TestS3CacheDownloadURL(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	url := getCacheDownloadURL(s3CacheBuild, "key")
	require.NotNil(t, url)
	assert.Equal(t, s3Cache.ServerAddress, url.Host)
}

func TestGetCacheObjectNameWhenKeyIsEmptyResultIsAlsoEmpty(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	url := getCacheObjectName(s3CacheBuild, s3Cache, "")
	require.Empty(t, url)
}

func TestGetCacheObjectName(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	url := getCacheObjectName(s3CacheBuild, s3Cache, "key")
	require.Equal(t, "runner/longtoke/project/10/key", url)
}

func TestGetCacheObjectNameWhenPathIsSetThenUrlContainsIt(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	s3Cache.Path = "whatever"
	url := getCacheObjectName(s3CacheBuild, s3Cache, "key")
	require.Equal(t, "whatever/runner/longtoke/project/10/key", url)
}

func TestGetCacheObjectNameWhenPathHasMultipleSegmentIsSetThenUrlContainsIt(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	s3Cache.Path = "some/other/path/goes/here"
	url := getCacheObjectName(s3CacheBuild, s3Cache, "key")
	require.Equal(t, "some/other/path/goes/here/runner/longtoke/project/10/key", url)
}

func TestGetCacheObjectNameWhenPathIsNotSetThenUrlDoesNotContainIt(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	s3Cache.Path = ""
	url := getCacheObjectName(s3CacheBuild, s3Cache, "key")
	require.Equal(t, "runner/longtoke/project/10/key", url)
}

func TestGetCacheObjectNameWhenSharedFlagIsFalseThenRunnerSegmentExistsInTheUrl(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	s3Cache.Shared = false
	url := getCacheObjectName(s3CacheBuild, s3Cache, "key")
	require.Equal(t, "runner/longtoke/project/10/key", url)
}

func TestGetCacheObjectNameWhenSharedFlagIsFalseThenRunnerSegmentShouldNotBePresent(t *testing.T) {
	s3Cache := defaultS3CacheFactory()
	s3Cache.Shared = true
	url := getCacheObjectName(s3CacheBuild, s3Cache, "key")
	require.Equal(t, "project/10/key", url)
}
