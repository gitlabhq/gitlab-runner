//go:build !integration

package azure

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
)

var (
	accountName    = "azuretest"
	accountKey     = base64.StdEncoding.EncodeToString([]byte("12345"))
	containerName  = "test"
	objectName     = "key"
	storageDomain  = "example.com"
	defaultTimeout = 1 * time.Hour
)

func defaultAzureCache() *cacheconfig.Config {
	return &cacheconfig.Config{
		Type: "azure",
		Azure: &cacheconfig.CacheAzureConfig{
			CacheAzureCredentials: cacheconfig.CacheAzureCredentials{
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
	credentialsResolverInitializer = func(config *cacheconfig.CacheAzureConfig) (*defaultCredentialsResolver, error) {
		if tc.errorOnCredentialsResolverInitialization {
			return nil, errors.New("test error")
		}

		return newDefaultCredentialsResolver(config)
	}

	return func() {
		credentialsResolverInitializer = oldCredentialsResolverInitializer
	}
}

func prepareMockedCredentialsResolverForInvalidConfig(t *testing.T, adapter *azureAdapter, tc adapterOperationInvalidConfigTestCase) {
	cr := newMockCredentialsResolver(t)

	resolveCall := cr.On("Resolve").Maybe()
	if tc.credentialsResolverResolveError {
		resolveCall.Return(fmt.Errorf("test error"))
	} else {
		resolveCall.Return(nil)
	}

	config := defaultAzureCache()
	config.Azure.CacheAzureCredentials.AccountName = tc.accountName
	config.Azure.CacheAzureCredentials.AccountKey = tc.accountKey
	config.Azure.ContainerName = tc.containerName

	// Always return an account key signer to avoid metadata lookups
	signer, err := newAccountKeySigner(config.Azure)
	cr.On("Signer").Return(signer, err).Maybe()

	adapter.credentialsResolver = cr
}

func testGoCloudURLWithInvalidConfig(
	t *testing.T,
	name string,
	tc adapterOperationInvalidConfigTestCase,
	adapter *azureAdapter,
	operation func(ctx context.Context, upload bool) (cache.GoCloudURL, error),
	expectedErrorMessage string,
) {
	t.Run(name, func(t *testing.T) {
		prepareMockedCredentialsResolverForInvalidConfig(t, adapter, tc)

		u, err := operation(t.Context(), true)

		if expectedErrorMessage != "" {
			assert.ErrorContains(t, err, expectedErrorMessage)
		} else {
			assert.NoError(t, err)
		}

		if tc.expectedGoCloudURL != "" {
			assert.Equal(t, tc.expectedGoCloudURL, u.URL.String())
		} else {
			assert.Nil(t, u.URL)
		}
	})
}

func testUploadEnvWithInvalidConfig(
	t *testing.T,
	name string,
	tc adapterOperationInvalidConfigTestCase,
	adapter *azureAdapter,
	operation func(context.Context) (map[string]string, error),
) {
	t.Run(name, func(t *testing.T) {
		prepareMockedCredentialsResolverForInvalidConfig(t, adapter, tc)

		u, err := operation(t.Context())
		assert.NoError(t, err)
		assert.Equal(t, accountName, u["AZURE_STORAGE_ACCOUNT"])
		assert.Equal(t, storageDomain, u["AZURE_STORAGE_DOMAIN"])
		assert.NotContains(t, u, "AZURE_SAS_TOKEN")
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
			expectedGoCloudURL:              "azblob://test/key",
		},
		"no-credentials": {
			provideAzureConfig: true,
			containerName:      containerName,
			expectedGoCloudURL: "azblob://test/key",
		},
		"no-account-key": {
			provideAzureConfig: true,
			accountName:        accountName,
			containerName:      containerName,
			expectedGoCloudURL: "azblob://test/key",
		},
		"invalid-container-name": {
			provideAzureConfig: true,
			accountName:        accountName,
			containerName:      "\x00",
			expectedErrorMsg:   "error parsing blob URL",
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

			ctx := t.Context()
			assert.Nil(t, adapter.GetDownloadURL(ctx).URL)
			assert.Nil(t, adapter.GetUploadURL(ctx).URL)

			testGoCloudURLWithInvalidConfig(t, "GetGoCloudURL", tc, adapter, a.GetGoCloudURL, tc.expectedErrorMsg)
		})
	}
}

type adapterOperationTestCase struct {
	objectName    string
	returnedURL   string
	returnedError error
	expectedError string
}

func prepareMockedSignedURLGenerator(
	t *testing.T,
	tc adapterOperationTestCase,
	expectedMethod string,
	adapter *azureAdapter,
) {
	adapter.generateSignedURL = func(ctx context.Context, name string, opts *signedURLOptions) (*url.URL, error) {
		assert.Equal(t, containerName, opts.ContainerName)
		assert.Equal(t, expectedMethod, opts.Method)

		u, err := url.Parse(tc.returnedURL)
		if err != nil {
			return nil, err
		}

		return u, tc.returnedError
	}
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

			u, err := adapter.GetGoCloudURL(t.Context(), true)
			assert.NoError(t, err)
			assert.Equal(t, "azblob://test/key", u.URL.String())

			assert.Len(t, u.Environment, 3)
			assert.Equal(t, accountName, u.Environment["AZURE_STORAGE_ACCOUNT"])
			assert.NotEmpty(t, u.Environment["AZURE_STORAGE_SAS_TOKEN"])
			assert.Empty(t, u.Environment["AZURE_STORAGE_KEY"])
			assert.Equal(t, storageDomain, u.Environment["AZURE_STORAGE_DOMAIN"])

			du, err := adapter.GetGoCloudURL(t.Context(), false)
			assert.NoError(t, err)
			assert.Equal(t, "azblob://test/key", du.URL.String())

			assert.Len(t, du.Environment, 3)
			assert.Equal(t, accountName, du.Environment["AZURE_STORAGE_ACCOUNT"])
			assert.NotEmpty(t, du.Environment["AZURE_STORAGE_SAS_TOKEN"])
			assert.Empty(t, du.Environment["AZURE_STORAGE_KEY"])
			assert.Equal(t, storageDomain, du.Environment["AZURE_STORAGE_DOMAIN"])

			ctx := t.Context()
			assert.Nil(t, adapter.GetDownloadURL(ctx).URL)
			assert.Nil(t, adapter.GetUploadURL(ctx).URL)
		})
	}
}
