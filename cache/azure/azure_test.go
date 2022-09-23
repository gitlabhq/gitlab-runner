//go:build !integration

package azure

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type azureURLGenerationTest struct {
	accountName   string
	accountKey    string
	storageDomain string
	method        string

	expectedErrorOnGeneration bool
}

func TestAzureClientURLGeneration(t *testing.T) {
	tests := map[string]azureURLGenerationTest{
		"missing account name": {
			accountKey:                accountKey,
			method:                    http.MethodGet,
			expectedErrorOnGeneration: true,
		},
		"missing account key": {
			accountName:               accountName,
			method:                    http.MethodGet,
			expectedErrorOnGeneration: true,
		},
		"GET request": {
			accountName: accountName,
			accountKey:  accountKey,
			method:      http.MethodGet,
		},
		"GET request in custom storage domain": {
			accountName:   accountName,
			accountKey:    accountKey,
			storageDomain: "blob.core.chinacloudapi.cn",
			method:        http.MethodGet,
		},

		"PUT request": {
			accountName: accountName,
			accountKey:  accountKey,
			method:      http.MethodPut,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			opts := &signedURLOptions{
				ContainerName: containerName,
				StorageDomain: tt.storageDomain,
				Credentials: &common.CacheAzureCredentials{
					AccountName: tt.accountName,
					AccountKey:  tt.accountKey,
				},
				Method:  tt.method,
				Timeout: 1 * time.Hour,
			}

			url, err := presignedURL(objectName, opts)

			if tt.expectedErrorOnGeneration {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, "https", url.Scheme)

			domain := DefaultAzureServer
			if tt.storageDomain != "" {
				domain = tt.storageDomain
			}
			assert.Equal(t, fmt.Sprintf("%s.%s", tt.accountName, domain), url.Host)
			assert.Equal(t, fmt.Sprintf("/%s/%s", containerName, objectName), url.Path)

			require.NotNil(t, url)

			q := url.Query()
			token, err := getSASToken(objectName, opts)
			require.NoError(t, err)
			assert.Equal(t, q.Encode(), token)

			// Sanity check query parameters from
			// https://docs.microsoft.com/en-us/rest/api/storageservices/create-service-sas
			assert.NotNil(t, q["sv"])                    // SignedVersion
			assert.Equal(t, []string{"b"}, q["sr"])      // SignedResource (blob)
			assert.NotNil(t, q["st"])                    // SignedStart
			assert.NotNil(t, q["se"])                    // SignedExpiry
			assert.NotNil(t, q["sig"])                   // Signature
			assert.Equal(t, []string{"https"}, q["spr"]) // SignedProtocol

			// SignedPermission
			expectedPermissionValue := "w"
			if tt.method == http.MethodGet {
				expectedPermissionValue = "r"
			}
			assert.Equal(t, []string{expectedPermissionValue}, q["sp"])
		})
	}
}
