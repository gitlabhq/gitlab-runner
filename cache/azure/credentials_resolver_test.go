//go:build !integration

package azure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
)

type credentialsResolverTestCase struct {
	config                        *cacheconfig.CacheAzureConfig
	errorExpectedOnInitialization bool
	errorExpectedOnResolve        bool
	expectedCredentials           *cacheconfig.CacheAzureCredentials
}

type signerTestCase struct {
	config                *cacheconfig.CacheAzureConfig
	errorExpectedOnSigner bool
	expectedSignerType    string
}

func getCredentialsConfig(accountName string, accountKey string) *cacheconfig.CacheAzureConfig {
	return &cacheconfig.CacheAzureConfig{
		CacheAzureCredentials: cacheconfig.CacheAzureCredentials{
			AccountName: accountName,
			AccountKey:  accountKey,
		},
		ContainerName: "test-container",
	}
}

func getExpectedCredentials(accountName string, accountKey string) *cacheconfig.CacheAzureCredentials {
	return &cacheconfig.CacheAzureCredentials{
		AccountName: accountName,
		AccountKey:  accountKey,
	}
}

func TestDefaultCredentialsResolver(t *testing.T) {
	cases := map[string]credentialsResolverTestCase{
		"config is nil": {
			config:                        nil,
			errorExpectedOnInitialization: true,
		},
		"credentials not set": {
			config:                 &cacheconfig.CacheAzureConfig{},
			errorExpectedOnResolve: true,
		},
		"credentials direct in config": {
			config:                 getCredentialsConfig(accountName, accountKey),
			errorExpectedOnResolve: false,
			expectedCredentials:    getExpectedCredentials(accountName, accountKey),
		},
	}

	for tn, tt := range cases {
		t.Run(tn, func(t *testing.T) {
			cr, err := newDefaultCredentialsResolver(tt.config)

			if tt.errorExpectedOnInitialization {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err, "Error on resolver initialization is not expected")

			err = cr.Resolve()

			if tt.errorExpectedOnResolve {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err, "Error on credentials resolving is not expected")
			assert.Equal(t, tt.expectedCredentials, cr.Credentials())
		})
	}
}

func TestSigner(t *testing.T) {
	cases := map[string]signerTestCase{
		"account name not set": {
			config:                getCredentialsConfig("", accountKey),
			errorExpectedOnSigner: true,
		},
		"account key not set": {
			config:                getCredentialsConfig(accountName, ""),
			errorExpectedOnSigner: false,
			expectedSignerType:    "userDelegationKeySigner",
		},
		"account name and key set": {
			config:                getCredentialsConfig(accountName, accountKey),
			errorExpectedOnSigner: false,
			expectedSignerType:    "accountKeySigner",
		},
	}

	for tn, tt := range cases {
		t.Run(tn, func(t *testing.T) {
			cr, err := newDefaultCredentialsResolver(tt.config)
			require.NoError(t, err, "Error on resolver initialization is not expected")

			signer, err := cr.Signer()
			if tt.errorExpectedOnSigner {
				assert.Error(t, err)
				assert.Nil(t, signer)
				return
			}

			require.NoError(t, err, "Error on signer is not expected")

			if tt.expectedSignerType == "accountKeySigner" {
				_, ok := signer.(*accountKeySigner)
				assert.True(t, ok, "Signer is expected to be of accountKeySigner type")
			} else {
				_, ok := signer.(*userDelegationKeySigner)
				assert.True(t, ok, "Signer is expected to be of userDelegationKeySigner type")
			}
		})
	}
}
