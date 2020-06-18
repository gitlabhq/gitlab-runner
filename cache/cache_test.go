package cache

import (
	"errors"
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
	key                    string
	configExists           bool
	adapterExists          bool
	errorOnAdapterCreation bool
	adapterURL             *url.URL
	expectedURL            *url.URL
	expectedOutput         []string
}

func prepareFakeCreateAdapter(t *testing.T, operationName string, tc cacheOperationTest) func() {
	assertAdapterExpectations := func(t mock.TestingT) bool { return true }

	var cacheAdapter Adapter
	if tc.adapterExists {
		a := new(MockAdapter)

		if tc.adapterURL != nil {
			a.On(operationName).Return(tc.adapterURL)
		}

		assertAdapterExpectations = a.AssertExpectations
		cacheAdapter = a
	}

	var cacheAdapterCreationError error
	if tc.errorOnAdapterCreation {
		cacheAdapterCreationError = errors.New("test error")
	}

	oldCreateAdapter := createAdapter
	createAdapter = func(cacheConfig *common.CacheConfig, timeout time.Duration, objectName string) (Adapter, error) {
		return cacheAdapter, cacheAdapterCreationError
	}

	return func() {
		createAdapter = oldCreateAdapter
		assertAdapterExpectations(t)
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
	}

	return build
}

func testCacheOperation(
	t *testing.T,
	operationName string,
	operation func(build *common.Build, key string) *url.URL,
	tc cacheOperationTest,
) {
	t.Run(operationName, func(t *testing.T) {
		hook := test.NewGlobal()

		cleanupCreateAdapter := prepareFakeCreateAdapter(t, operationName, tc)
		defer cleanupCreateAdapter()

		build := prepareFakeBuild(tc)
		generatedURL := operation(build, tc.key)
		assert.Equal(t, tc.expectedURL, generatedURL)

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
			adapterURL:     nil,
			expectedURL:    nil,
			expectedOutput: []string{"Cache config not defined. Skipping cache operation."},
		},
		"key-not-specified": {
			configExists:   true,
			adapterExists:  true,
			adapterURL:     nil,
			expectedURL:    nil,
			expectedOutput: []string{"Empty cache key. Skipping adapter selection."},
		},
		"adapter-doesnt-exists": {
			key:           "key",
			configExists:  true,
			adapterExists: false,
			adapterURL:    exampleURL,
			expectedURL:   nil,
		},
		"adapter-error-on-factorization": {
			key:                    "key",
			configExists:           true,
			errorOnAdapterCreation: true,
			adapterURL:             exampleURL,
			expectedURL:            nil,
			expectedOutput: []string{
				"Could not create cache adapter",
				"test error",
			},
		},
		"adapter-exists": {
			key:           "key",
			configExists:  true,
			adapterExists: true,
			adapterURL:    exampleURL,
			expectedURL:   exampleURL,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			testCacheOperation(t, "GetDownloadURL", GetCacheDownloadURL, tc)
			testCacheOperation(t, "GetUploadURL", GetCacheUploadURL, tc)
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
			expectedObjectName: "runner/longtoke/project/10/key",
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
			expectedObjectName: "whatever/runner/longtoke/project/10/key",
		},
		"multiple segment path is set": {
			cache:              cache,
			build:              cacheBuild,
			key:                "key",
			path:               "some/other/path/goes/here",
			expectedObjectName: "some/other/path/goes/here/runner/longtoke/project/10/key",
		},
		"path is empty": {
			cache:              cache,
			build:              cacheBuild,
			key:                "key",
			path:               "",
			expectedObjectName: "runner/longtoke/project/10/key",
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
			expectedObjectName: "runner/longtoke/project/10/key",
		},
		"key escapes project namespace": {
			cache:              cache,
			build:              cacheBuild,
			key:                "../9/key",
			expectedObjectName: "",
			expectedError:      "computed cache path outside of project bucket. Please remove `../` from cache key",
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
