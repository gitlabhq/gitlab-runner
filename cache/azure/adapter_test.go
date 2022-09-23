//go:build !integration

package azure

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var (
	accountName    = "azuretest"
	accountKey     = base64.StdEncoding.EncodeToString([]byte("12345"))
	containerName  = "test"
	objectName     = "key"
	storageDomain  = "example.com"
	defaultTimeout = 1 * time.Hour
)

func defaultAzureCache() *common.CacheConfig {
	return &common.CacheConfig{
		Type: "azure",
		Azure: &common.CacheAzureConfig{
			CacheAzureCredentials: common.CacheAzureCredentials{
				AccountName: accountName,
				AccountKey:  accountKey,
			},
			ContainerName: containerName,
			StorageDomain: storageDomain,
		},
	}
}

type adapterOperationInvalidConfigTestCase struct {
	provideAzureConfig bool

	errorOnCredentialsResolverInitialization bool
	credentialsResolverResolveError          bool

	accountName        string
	accountKey         string
	containerName      string
	expectedErrorMsg   string
	expectedGoCloudURL string
}

func prepareMockedCredentialsResolverInitializer(tc adapterOperationInvalidConfigTestCase) func() {
	oldCredentialsResolverInitializer := credentialsResolverInitializer
	credentialsResolverInitializer = func(config *common.CacheAzureConfig) (*defaultCredentialsResolver, error) {
		if tc.errorOnCredentialsResolverInitialization {
			return nil, errors.New("test error")
		}

		return newDefaultCredentialsResolver(config)
	}

	return func() {
		credentialsResolverInitializer = oldCredentialsResolverInitializer
	}
}

func prepareMockedCredentialsResolverForInvalidConfig(adapter *azureAdapter, tc adapterOperationInvalidConfigTestCase) {
	cr := &mockCredentialsResolver{}

	resolveCall := cr.On("Resolve")
	if tc.credentialsResolverResolveError {
		resolveCall.Return(fmt.Errorf("test error"))
	} else {
		resolveCall.Return(nil)
	}

	cr.On("Credentials").Return(&common.CacheAzureCredentials{
		AccountName: tc.accountName,
		AccountKey:  tc.accountKey,
	})

	adapter.credentialsResolver = cr
}

func testAdapterOperationWithInvalidConfig(
	t *testing.T,
	name string,
	tc adapterOperationInvalidConfigTestCase,
	adapter *azureAdapter,
	operation func() *url.URL,
) {
	t.Run(name, func(t *testing.T) {
		prepareMockedCredentialsResolverForInvalidConfig(adapter, tc)
		hook := test.NewGlobal()

		u := operation()
		assert.Nil(t, u)

		message, err := hook.LastEntry().String()
		require.NoError(t, err)
		assert.Contains(t, message, tc.expectedErrorMsg)
	})
}

func testGoCloudURLWithInvalidConfig(
	t *testing.T,
	name string,
	tc adapterOperationInvalidConfigTestCase,
	adapter *azureAdapter,
	operation func() *url.URL,
) {
	t.Run(name, func(t *testing.T) {
		prepareMockedCredentialsResolverForInvalidConfig(adapter, tc)

		u := operation()

		if u != nil {
			assert.Equal(t, tc.expectedGoCloudURL, u.String())
		} else {
			assert.Empty(t, tc.expectedGoCloudURL)
		}
	})
}

func testUploadEnvWithInvalidConfig(
	t *testing.T,
	name string,
	tc adapterOperationInvalidConfigTestCase,
	adapter *azureAdapter,
	operation func() map[string]string,
) {
	t.Run(name, func(t *testing.T) {
		prepareMockedCredentialsResolverForInvalidConfig(adapter, tc)

		u := operation()
		assert.Empty(t, u)
	})
}

func TestAdapterOperation_InvalidConfig(t *testing.T) {
	tests := map[string]adapterOperationInvalidConfigTestCase{
		"no-azure-config": {
			containerName:    containerName,
			expectedErrorMsg: "Missing Azure configuration",
		},
		"error-on-credentials-resolver-initialization": {
			provideAzureConfig:                       true,
			errorOnCredentialsResolverInitialization: true,
		},
		"credentials-resolver-resolve-error": {
			provideAzureConfig:              true,
			credentialsResolverResolveError: true,
			containerName:                   containerName,
			expectedErrorMsg:                `error resolving Azure credentials" error="test error"`,
			expectedGoCloudURL:              "azblob://test/key",
		},
		"no-credentials": {
			provideAzureConfig: true,
			containerName:      containerName,
			expectedErrorMsg:   "error generating Azure pre-signed URL\" error=\"missing Azure storage account name\"",
			expectedGoCloudURL: "azblob://test/key",
		},
		"no-account-name": {
			provideAzureConfig: true,
			accountKey:         accountKey,
			containerName:      containerName,
			expectedErrorMsg:   "error generating Azure pre-signed URL\" error=\"missing Azure storage account name\"",
			expectedGoCloudURL: "azblob://test/key",
		},
		"no-account-key": {
			provideAzureConfig: true,
			accountName:        accountName,
			containerName:      containerName,
			expectedErrorMsg:   "error generating Azure pre-signed URL\" error=\"missing Azure storage account key\"",
			expectedGoCloudURL: "azblob://test/key",
		},
		"invalid-container-name-and-no-account-key": {
			provideAzureConfig: true,
			accountName:        accountName,
			containerName:      "\x00",
			expectedErrorMsg:   "error generating Azure pre-signed URL\" error=\"missing Azure storage account key\"",
		},
		"container-not-specified": {
			provideAzureConfig: true,
			accountName:        "access-id",
			accountKey:         accountKey,
			expectedErrorMsg:   "ContainerName can't be empty",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cleanupCredentialsResolverInitializerMock := prepareMockedCredentialsResolverInitializer(tc)
			defer cleanupCredentialsResolverInitializerMock()

			config := defaultAzureCache()
			config.Azure.ContainerName = tc.containerName
			if !tc.provideAzureConfig {
				config.Azure = nil
			}

			a, err := New(config, defaultTimeout, objectName)
			if !tc.provideAzureConfig {
				assert.Nil(t, a)
				assert.EqualError(t, err, "missing Azure configuration")
				return
			}

			if tc.errorOnCredentialsResolverInitialization {
				assert.Nil(t, a)
				assert.EqualError(t, err, "error while initializing Azure credentials resolver: test error")
				return
			}

			require.NotNil(t, a)
			assert.NoError(t, err)

			adapter, ok := a.(*azureAdapter)
			require.True(t, ok, "Adapter should be properly casted to *adapter type")

			testAdapterOperationWithInvalidConfig(t, "GetDownloadURL", tc, adapter, a.GetDownloadURL)
			testAdapterOperationWithInvalidConfig(t, "GetUploadURL", tc, adapter, a.GetUploadURL)
			testGoCloudURLWithInvalidConfig(t, "GetGoCloudURL", tc, adapter, a.GetGoCloudURL)
			testUploadEnvWithInvalidConfig(t, "GetUploadEnv", tc, adapter, a.GetUploadEnv)
		})
	}
}

type adapterOperationTestCase struct {
	objectName    string
	returnedURL   string
	returnedError error
	expectedError string
}

func prepareMockedCredentialsResolver(adapter *azureAdapter) func(t *testing.T) {
	cr := &mockCredentialsResolver{}
	cr.On("Resolve").Return(nil)
	cr.On("Credentials").Return(&common.CacheAzureCredentials{
		AccountName: accountName,
		AccountKey:  accountKey,
	})

	adapter.credentialsResolver = cr

	return func(t *testing.T) {
		cr.AssertExpectations(t)
	}
}

func prepareMockedSignedURLGenerator(
	t *testing.T,
	tc adapterOperationTestCase,
	expectedMethod string,
	adapter *azureAdapter,
) {
	adapter.generateSignedURL = func(name string, opts *signedURLOptions) (*url.URL, error) {
		assert.Equal(t, containerName, opts.ContainerName)
		assert.Equal(t, accountName, opts.Credentials.AccountName)
		assert.Equal(t, accountKey, opts.Credentials.AccountKey)
		assert.Equal(t, expectedMethod, opts.Method)

		u, err := url.Parse(tc.returnedURL)
		if err != nil {
			return nil, err
		}

		return u, tc.returnedError
	}
}

func testAdapterOperation(
	t *testing.T,
	tc adapterOperationTestCase,
	name string,
	expectedMethod string,
	adapter *azureAdapter,
	operation func() *url.URL,
) {
	t.Run(name, func(t *testing.T) {
		cleanupCredentialsResolverMock := prepareMockedCredentialsResolver(adapter)
		defer cleanupCredentialsResolverMock(t)

		prepareMockedSignedURLGenerator(t, tc, expectedMethod, adapter)
		hook := test.NewGlobal()

		u := operation()

		if tc.expectedError != "" {
			message, err := hook.LastEntry().String()
			require.NoError(t, err)
			assert.Contains(t, message, tc.expectedError)
			return
		}

		assert.Empty(t, hook.AllEntries())

		assert.Equal(t, tc.returnedURL, u.String())
	})
}

func TestAdapterOperation(t *testing.T) {
	tests := map[string]adapterOperationTestCase{
		"error-on-URL-signing": {
			objectName:    objectName,
			returnedURL:   "",
			returnedError: fmt.Errorf("test error"),
			expectedError: "error generating Azure pre-signed URL\" error=\"test error\"",
		},
		"invalid-URL-returned": {
			objectName:    objectName,
			returnedURL:   "://test",
			returnedError: nil,
			expectedError: "error generating Azure pre-signed URL\" error=\"parse",
		},
		"valid-configuration": {
			objectName:    objectName,
			returnedURL:   "https://myaccount.blob.core.windows.net/mycontainer/mydirectory/myfile.txt?sig=XYZ&sp=r",
			returnedError: nil,
			expectedError: "",
		},
		"valid-configuration-with-leading-slash": {
			objectName:    "/" + objectName,
			returnedURL:   "https://myaccount.blob.core.windows.net/mycontainer/mydirectory/myfile.txt?sig=XYZ&sp=r",
			returnedError: nil,
			expectedError: "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			config := defaultAzureCache()

			a, err := New(config, defaultTimeout, tc.objectName)
			require.NoError(t, err)

			adapter, ok := a.(*azureAdapter)
			require.True(t, ok, "Adapter should be properly casted to *adapter type")

			testAdapterOperation(
				t,
				tc,
				"GetDownloadURL",
				http.MethodGet,
				adapter,
				a.GetDownloadURL,
			)
			testAdapterOperation(
				t,
				tc,
				"GetUploadURL",
				http.MethodPut,
				adapter,
				a.GetUploadURL,
			)

			headers := adapter.GetUploadHeaders()
			require.NotNil(t, headers)
			assert.Len(t, headers, 2)
			assert.Equal(t, "application/octet-stream", headers.Get("Content-Type"))
			assert.Equal(t, "BlockBlob", headers.Get("x-ms-blob-type"))

			u := adapter.GetGoCloudURL()
			assert.Equal(t, "azblob://test/key", u.String())

			env := adapter.GetUploadEnv()
			assert.Len(t, env, 3)
			assert.Equal(t, accountName, env["AZURE_STORAGE_ACCOUNT"])
			assert.NotEmpty(t, env["AZURE_STORAGE_SAS_TOKEN"])
			assert.Empty(t, env["AZURE_STORAGE_KEY"])
			assert.Equal(t, storageDomain, env["AZURE_STORAGE_DOMAIN"])
		})
	}
}
