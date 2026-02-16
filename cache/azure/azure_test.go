//go:build !integration

package azure

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
)

type azureSigningTest struct {
	accountName   string
	accountKey    string
	storageDomain string
	containerName string
	method        string
	endpoint      string

	expectedErrorOnGeneration bool
	expectedServiceURL        string
}

const (
	mockClientInfo = "my-client"
	mockIDToken    = "my-idt"
)

type mockSTS struct{}

var accessTokenRespSuccess = []byte(fmt.Sprintf(`{"access_token": "%s", "expires_in": 3600}`, "tokenValue"))

func (m *mockSTS) Do(req *http.Request) (*http.Response, error) {
	res := &http.Response{StatusCode: http.StatusNotFound}
	s := strings.Split(req.URL.Path, "/")
	if s[len(s)-1] != "token" {
		return res, nil
	}

	if err := req.ParseForm(); err != nil {
		return nil, fmt.Errorf("mockSTS failed to parse a request body: %w", err)
	}
	if grant := req.FormValue("grant_type"); grant == "device_code" || grant == "password" {
		// include account info because we're authenticating a user
		res.Body = io.NopCloser(bytes.NewReader(
			[]byte(fmt.Sprintf(`{"access_token":"at","expires_in": 3600,"refresh_token":"rt","client_info":%q,"id_token":%q}`, mockClientInfo, mockIDToken)),
		))
	} else {
		res.Body = io.NopCloser(bytes.NewReader(accessTokenRespSuccess))
	}

	res.StatusCode = http.StatusOK
	return res, nil
}

func TestAccountKeySigning(t *testing.T) {
	tests := map[string]azureSigningTest{
		"missing account name": {
			accountKey:                accountKey,
			containerName:             "test-container",
			method:                    http.MethodGet,
			expectedErrorOnGeneration: true,
		},
		"missing account key": {
			accountName:               accountName,
			containerName:             "test-container",
			method:                    http.MethodGet,
			expectedErrorOnGeneration: true,
		},
		"GET request": {
			accountName:        accountName,
			accountKey:         accountKey,
			containerName:      "test-container",
			method:             http.MethodGet,
			expectedServiceURL: "https://azuretest.blob.core.windows.net",
		},
		"GET request in custom storage domain": {
			accountName:        accountName,
			accountKey:         accountKey,
			storageDomain:      "blob.core.chinacloudapi.cn",
			containerName:      "test-container",
			method:             http.MethodGet,
			expectedServiceURL: "https://azuretest.blob.core.chinacloudapi.cn",
		},
		"PUT request": {
			accountName:        accountName,
			accountKey:         accountKey,
			containerName:      "test-container",
			method:             http.MethodPut,
			expectedServiceURL: "https://azuretest.blob.core.windows.net",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			credentials := &cacheconfig.CacheAzureCredentials{
				AccountName: tt.accountName,
				AccountKey:  tt.accountKey,
			}
			config := &cacheconfig.CacheAzureConfig{
				CacheAzureCredentials: *credentials,
				ContainerName:         tt.containerName,
				StorageDomain:         tt.storageDomain,
			}
			opts := &signedURLOptions{
				ContainerName: containerName,
				Method:        tt.method,
				Timeout:       1 * time.Hour,
			}

			signer, err := newAccountKeySigner(config)

			if tt.expectedErrorOnGeneration {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedServiceURL, signer.ServiceURL())

			opts.Signer = signer
			token, err := getSASToken(t.Context(), objectName, opts)
			require.NoError(t, err)

			q, err := url.ParseQuery(token)
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

func TestUserDelegationSigning(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate Azure API response
		w.Header().Set("Content-Type", "application/xml")
		responseBody := `
    <UserDelegationKey>
        <SignedOid>f81d4fae-7dec-11d0-a765-00a0c91e6bf6</SignedOid>
        <SignedTid>72f988bf-86f1-41af-91ab-2d7cd011db47</SignedTid>
        <SignedStart>2024-09-19T00:00:00Z</SignedStart>
        <SignedExpiry>2024-09-26T00:00:00Z</SignedExpiry>
        <SignedService>b</SignedService>
        <SignedVersion>2020-02-10</SignedVersion>
        <Value>UDELEGATIONKEYXYZ....</Value>
        <SignedKey>rL7...ABC</SignedKey>
    </UserDelegationKey>`
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Date", time.Now().UTC().Format(http.TimeFormat))
		_, _ = w.Write([]byte(responseBody))
	})

	server := httptest.NewTLSServer(handler)
	defer server.Close()

	// Azure requires HTTPS to be used. Since we are setting up our own
	// fake API server, skip TLS verification.
	customTransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	tests := map[string]azureSigningTest{
		"missing account name": {
			accountKey:                accountKey,
			containerName:             "test-container",
			method:                    http.MethodGet,
			expectedErrorOnGeneration: true,
		},
		"GET request": {
			accountName: accountName,
			accountKey:  accountKey,
			method:      http.MethodGet,
			endpoint:    server.URL,
		},
		"PUT request": {
			accountName: accountName,
			accountKey:  accountKey,
			method:      http.MethodPut,
			endpoint:    server.URL,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			credentials := &cacheconfig.CacheAzureCredentials{
				AccountName: tt.accountName,
				AccountKey:  tt.accountKey,
			}
			config := &cacheconfig.CacheAzureConfig{
				CacheAzureCredentials: *credentials,
			}
			opts := &signedURLOptions{
				ContainerName: containerName,
				Method:        tt.method,
				Timeout:       1 * time.Hour,
			}

			signer, err := newUserDelegationKeySigner(config,
				withDefaultCredentialTransporter(&mockSTS{}),
				withBlobServiceEndpoint(tt.endpoint),
				withBlobServiceTransport(customTransport))
			if tt.expectedErrorOnGeneration {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, server.URL, signer.ServiceURL())

			opts.Signer = signer
			token, err := getSASToken(t.Context(), objectName, opts)
			require.NoError(t, err)

			q, err := url.ParseQuery(token)
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
