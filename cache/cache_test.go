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

	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
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
	var adapter Adapter

	oldCreateAdapter := createAdapter
	createAdapter = func(_ *cacheconfig.Config, _ time.Duration, _ string) (Adapter, error) {
		return adapter, tc.adapterCreateError
	}
	t.Cleanup(func() {
		createAdapter = oldCreateAdapter
	})

	if !tc.adapterExists {
		return
	}

	madapter := NewMockAdapter(t)
	adapter = madapter

	if tc.adapterURL.URL == nil {
		return
	}

	if operationName == "GetGoCloudURL" {
		madapter.On(operationName, mock.Anything, true).Return(GoCloudURL{URL: tc.adapterURL.URL}, nil).Once()
	} else {
		madapter.On(operationName, mock.Anything).Return(tc.adapterURL).Once()
	}

	if operationName == "GetUploadURL" {
		madapter.On("WithMetadata", tc.metadata).Once()
	}
}

func prepareFakeConfig(tc cacheOperationTest) *cacheconfig.Config {
	if !tc.configExists {
		return nil
	}

	config := &cacheconfig.Config{}
	if tc.adapterExists {
		config.Type = "test"
	}

	return config
}

func testCacheOperation(
	t *testing.T,
	operationName string,
	operation func(ctx context.Context, adaptor Adapter) PresignedURL,
	tc cacheOperationTest,
) {
	t.Run(operationName, func(t *testing.T) {
		ctx := t.Context()
		hook := test.NewGlobal()

		prepareFakeCreateAdapter(t, operationName, tc)

		config := prepareFakeConfig(tc)
		adaptor := GetAdapter(config, 3600*time.Second, "shorttoken", "10", tc.key)
		generatedURL := operation(ctx, adaptor)
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
			testCacheOperation(t, "GetDownloadURL", func(ctx context.Context, adaptor Adapter) PresignedURL {
				return adaptor.GetDownloadURL(ctx)
			}, tc)
			testCacheOperation(t, "GetUploadURL", func(ctx context.Context, adaptor Adapter) PresignedURL {
				adaptor.WithMetadata(tc.metadata)
				return adaptor.GetUploadURL(ctx)
			}, tc)
			testCacheOperation(t, "GetGoCloudURL", func(ctx context.Context, adaptor Adapter) PresignedURL {
				u, _ := adaptor.GetGoCloudURL(ctx, true)
				return PresignedURL{URL: u.URL}
			}, tc)
		})
	}
}

func defaultCacheConfig() *cacheconfig.Config {
	return &cacheconfig.Config{
		Type: "test",
	}
}

type generateObjectNameTestCase struct {
	key    string
	path   string
	shared bool

	expectedObjectName string
	expectedError      string
}

func TestGenerateObjectName(t *testing.T) {
	tests := map[string]generateObjectNameTestCase{
		"default usage": {
			key:                "key",
			expectedObjectName: "runner/longtoken/project/10/key",
		},
		"empty key": {
			key:                "",
			expectedObjectName: "",
			expectedError:      "Empty cache key",
		},
		"short path is set": {
			key:                "key",
			path:               "whatever",
			expectedObjectName: "whatever/runner/longtoken/project/10/key",
		},
		"multiple segment path is set": {
			key:                "key",
			path:               "some/other/path/goes/here",
			expectedObjectName: "some/other/path/goes/here/runner/longtoken/project/10/key",
		},
		"path is empty": {
			key:                "key",
			path:               "",
			expectedObjectName: "runner/longtoken/project/10/key",
		},
		"shared flag is set to true": {
			key:                "key",
			shared:             true,
			expectedObjectName: "project/10/key",
		},
		"shared flag is set to false": {
			key:                "key",
			shared:             false,
			expectedObjectName: "runner/longtoken/project/10/key",
		},
		"path traversal but within base path": {
			key:                "../10/key",
			expectedObjectName: "runner/longtoken/project/10/key",
		},
		"path traversal resolves to empty key": {
			key:           "../10",
			expectedError: "computed cache path outside of project bucket",
		},
		"path traversal escapes project namespace": {
			key:           "../10-outside",
			expectedError: "computed cache path outside of project bucket",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			hook := test.NewGlobal()

			cache := defaultCacheConfig()
			cache.Path = tc.path
			cache.Shared = tc.shared

			var capturedObjectName string
			oldCreateAdapter := createAdapter
			createAdapter = func(_ *cacheconfig.Config, _ time.Duration, objectName string) (Adapter, error) {
				capturedObjectName = objectName
				return NewMockAdapter(t), nil
			}
			t.Cleanup(func() {
				createAdapter = oldCreateAdapter
			})

			adapter := GetAdapter(cache, 3600*time.Second, "longtoken", "10", tc.key)

			if tc.expectedError != "" {
				// The error/warning cases return a nopAdaptor and log instead of returning an error
				assert.IsType(t, nopAdapter{}, adapter)
				require.NotEmpty(t, hook.AllEntries())
				message, err := hook.LastEntry().String()
				require.NoError(t, err)
				assert.Contains(t, message, tc.expectedError)
			} else {
				assert.Equal(t, tc.expectedObjectName, capturedObjectName)
				assert.NotEqual(t, nopAdapter{}, adapter)
			}
		})
	}
}
