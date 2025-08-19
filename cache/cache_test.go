//go:build !integration

package cache

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type cacheOperationTest struct {
	key                string
	configExists       bool
	adapterExists      bool
	adapterCreateError error
	adapterURL         PresignedURL
	metadata           map[string]string
	expectedURL        *url.URL
	expectedOutput     []string
}

func prepareFakeCreateAdapter(t *testing.T, operationName string, tc cacheOperationTest) {
	var adapter *MockAdapter = nil

	// override the adapter creator, reset after test run
	oldCreateAdapter := createAdapter
	createAdapter = func(_ *common.CacheConfig, _ time.Duration, _ string) (Adapter, error) {
		return adapter, tc.adapterCreateError
	}
	t.Cleanup(func() {
		createAdapter = oldCreateAdapter
	})

	// for tests where we don't want the adapter to exist, we can return
	if !tc.adapterExists {
		return
	}

	// for all other tests, we set up a "real" mock
	adapter = NewMockAdapter(t)

	// for tests that are not supposed to produce a URL, we can leave the adapter mock without any assertions and return
	if tc.adapterURL.URL == nil {
		return
	}

	// for any other tests, we set up the assertions based on the test case at hand
	if operationName == "GetGoCloudURL" {
		adapter.On(operationName, mock.Anything, true).Return(GoCloudURL{URL: tc.adapterURL.URL}, nil).Once()
	} else {
		adapter.On(operationName, mock.Anything).Return(tc.adapterURL).Once()
	}

	if operationName == "GetUploadURL" {
		adapter.On("WithMetadata", tc.metadata).Once()
	}
}

func prepareFakeBuild(tc cacheOperationTest) *common.Build {
	build := &common.Build{
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{},
		},
	}

	if tc.configExists {
		build.Runner.Cache = &common.CacheConfig{}

		if tc.adapterExists {
			build.Runner.Cache.Type = "test"
		}
	}

	return build
}

func getCacheGoCloudURLAdapter(ctx context.Context, build *common.Build, key string) PresignedURL {
	u, _ := GetCacheGoCloudURL(ctx, build, key, true)
	return PresignedURL{URL: u.URL}
}

func getCachUploadURLWithMetadata(metadata map[string]string) func(ctx context.Context, build *common.Build, key string) PresignedURL {
	return func(ctx context.Context, build *common.Build, key string) PresignedURL {
		return GetCacheUploadURL(ctx, build, key, metadata)
	}
}

func testCacheOperation(
	t *testing.T,
	operationName string,
	operation func(ctx context.Context, build *common.Build, key string) PresignedURL,
	tc cacheOperationTest,
) {
	t.Run(operationName, func(t *testing.T) {
		ctx := context.Background()
		hook := test.NewGlobal()

		prepareFakeCreateAdapter(t, operationName, tc)

		build := prepareFakeBuild(tc)
		generatedURL := operation(ctx, build, tc.key)
		assert.Equal(t, tc.expectedURL, generatedURL.URL)

		if len(tc.expectedOutput) == 0 {
			assert.Len(t, hook.AllEntries(), 0)
		} else {
			for _, expectedOutput := range tc.expectedOutput {
				message, err := hook.LastEntry().String()
				require.NoError(t, err)
				assert.Contains(t, message, expectedOutput)
			}
		}
	})
}

func TestCacheOperations(t *testing.T) {
	exampleURL, err := url.Parse("example.com")
	require.NoError(t, err)

	tests := map[string]cacheOperationTest{
		"no-config": {
			key:            "key",
			adapterExists:  true,
			adapterURL:     PresignedURL{},
			expectedURL:    nil,
			expectedOutput: nil,
		},
		"key-not-specified": {
			configExists:   true,
			adapterExists:  true,
			adapterURL:     PresignedURL{},
			expectedURL:    nil,
			expectedOutput: []string{"Empty cache key. Skipping adapter selection."},
		},
		"adapter-doesnt-exists": {
			key:           "key",
			configExists:  true,
			adapterExists: false,
			adapterURL:    PresignedURL{URL: exampleURL},
			expectedURL:   nil,
		},
		"adapter-error-on-factorization": {
			key:                "key",
			configExists:       true,
			adapterExists:      true,
			adapterCreateError: fmt.Errorf("some creation error"),
			adapterURL:         PresignedURL{URL: exampleURL},
			expectedURL:        exampleURL,
			expectedOutput:     []string{`error="some creation error"`},
		},
		"adapter-exists": {
			key:           "key",
			configExists:  true,
			adapterExists: true,
			adapterURL:    PresignedURL{URL: exampleURL},
			expectedURL:   exampleURL,
		},
		"adapter-exists-with-metadata": {
			key:           "key",
			configExists:  true,
			adapterExists: true,
			metadata:      map[string]string{"foo": "some foo"},
			adapterURL:    PresignedURL{URL: exampleURL},
			expectedURL:   exampleURL,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			testCacheOperation(t, "GetDownloadURL", GetCacheDownloadURL, tc)
			testCacheOperation(t, "GetUploadURL", getCachUploadURLWithMetadata(tc.metadata), tc)
			testCacheOperation(t, "GetGoCloudURL", getCacheGoCloudURLAdapter, tc)
		})
	}
}

func defaultCacheConfig() *common.CacheConfig {
	return &common.CacheConfig{
		Type: "test",
	}
}

func defaultBuild(cacheConfig *common.CacheConfig) *common.Build {
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

type generateObjectNameTestCase struct {
	cache *common.CacheConfig
	build *common.Build

	key    string
	path   string
	shared bool

	expectedObjectName string
	expectedError      string
}

func TestGenerateObjectName(t *testing.T) {
	cache := defaultCacheConfig()
	cacheBuild := defaultBuild(cache)

	tests := map[string]generateObjectNameTestCase{
		"default usage": {
			cache:              cache,
			build:              cacheBuild,
			key:                "key",
			expectedObjectName: "runner/longtoken/project/10/key",
		},
		"empty key": {
			cache:              cache,
			build:              cacheBuild,
			key:                "",
			expectedObjectName: "",
		},
		"short path is set": {
			cache:              cache,
			build:              cacheBuild,
			key:                "key",
			path:               "whatever",
			expectedObjectName: "whatever/runner/longtoken/project/10/key",
		},
		"multiple segment path is set": {
			cache:              cache,
			build:              cacheBuild,
			key:                "key",
			path:               "some/other/path/goes/here",
			expectedObjectName: "some/other/path/goes/here/runner/longtoken/project/10/key",
		},
		"path is empty": {
			cache:              cache,
			build:              cacheBuild,
			key:                "key",
			path:               "",
			expectedObjectName: "runner/longtoken/project/10/key",
		},
		"shared flag is set to true": {
			cache:              cache,
			build:              cacheBuild,
			key:                "key",
			shared:             true,
			expectedObjectName: "project/10/key",
		},
		"shared flag is set to false": {
			cache:              cache,
			build:              cacheBuild,
			key:                "key",
			shared:             false,
			expectedObjectName: "runner/longtoken/project/10/key",
		},
		"path traversal but within base path": {
			cache:              cache,
			build:              cacheBuild,
			key:                "../10/key",
			expectedObjectName: "runner/longtoken/project/10/key",
		},
		"path traversal resolves to empty key": {
			cache:         cache,
			build:         cacheBuild,
			key:           "../10",
			expectedError: "computed cache path outside of project bucket. Please remove `../` from cache key",
		},
		"path traversal escapes project namespace": {
			cache:         cache,
			build:         cacheBuild,
			key:           "../10-outside",
			expectedError: "computed cache path outside of project bucket. Please remove `../` from cache key",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cache.Path = test.path
			cache.Shared = test.shared

			objectName, err := generateObjectName(test.build, test.cache, test.key)

			assert.Equal(t, test.expectedObjectName, objectName)
			if test.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.expectedError)
			}
		})
	}
}
