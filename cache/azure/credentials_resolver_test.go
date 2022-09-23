//go:build !integration

package azure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type credentialsResolverTestCase struct {
	config                        *common.CacheAzureConfig
	errorExpectedOnInitialization bool
	errorExpectedOnResolve        bool
	expectedCredentials           *common.CacheAzureCredentials
}

func getCredentialsConfig(accountName string, accountKey string) *common.CacheAzureConfig {
	return &common.CacheAzureConfig{
		CacheAzureCredentials: common.CacheAzureCredentials{
			AccountName: accountName,
			AccountKey:  accountKey,
		},
	}
}

func getExpectedCredentials(accountName string, accountKey string) *common.CacheAzureCredentials {
	return &common.CacheAzureCredentials{
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
			config:                 &common.CacheAzureConfig{},
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
