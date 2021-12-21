//go:build !integration
// +build !integration

package gcs

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var accessID2 = "test-access-id-2@X.iam.gserviceaccount.com"

type credentialsResolverTestCase struct {
	config                         *common.CacheGCSConfig
	credentialsFileContent         *credentialsFile
	credentialsFileDoesNotExist    bool
	credentialsFileWithInvalidJSON bool
	errorExpectedOnInitialization  bool
	errorExpectedOnResolve         bool
	expectedCredentials            *common.CacheGCSCredentials
}

func prepareStubbedCredentialsFile(t *testing.T, testCase credentialsResolverTestCase) func() {
	cleanup := func() {}

	if testCase.credentialsFileContent != nil {
		file, err := ioutil.TempFile("", "gcp-credentials-file")
		require.NoError(t, err)

		cleanup = func() {
			os.Remove(file.Name())
		}

		testCase.config.CredentialsFile = file.Name()

		switch {
		case testCase.credentialsFileDoesNotExist:
			os.Remove(file.Name())
		case testCase.credentialsFileWithInvalidJSON:
			_, err = file.Write([]byte("a"))
			require.NoError(t, err)

			err = file.Close()
			require.NoError(t, err)
		default:
			data, err := json.Marshal(testCase.credentialsFileContent)
			require.NoError(t, err)

			_, err = file.Write(data)
			require.NoError(t, err)

			err = file.Close()
			require.NoError(t, err)
		}
	}

	return cleanup
}

func getCredentialsConfig(accessID string, privateKey string) *common.CacheGCSConfig {
	return &common.CacheGCSConfig{
		CacheGCSCredentials: common.CacheGCSCredentials{
			AccessID:   accessID,
			PrivateKey: privateKey,
		},
	}
}

func getCredentialsFileContent(fileType string, clientEmail string, privateKey string) *credentialsFile {
	return &credentialsFile{
		Type:        fileType,
		ClientEmail: clientEmail,
		PrivateKey:  privateKey,
	}
}

func getExpectedCredentials(accessID string, privateKey string) *common.CacheGCSCredentials {
	return &common.CacheGCSCredentials{
		AccessID:   accessID,
		PrivateKey: privateKey,
	}
}

func TestDefaultCredentialsResolver(t *testing.T) {
	cases := map[string]credentialsResolverTestCase{
		"config is nil": {
			config:                        nil,
			credentialsFileContent:        nil,
			errorExpectedOnInitialization: true,
		},
		"credentials not set": {
			config:                 &common.CacheGCSConfig{},
			errorExpectedOnResolve: true,
		},
		"credentials direct in config": {
			config:                 getCredentialsConfig(accessID, privateKey),
			errorExpectedOnResolve: false,
			expectedCredentials:    getExpectedCredentials(accessID, privateKey),
		},
		"credentials in credentials file - service account file": {
			config:                 &common.CacheGCSConfig{},
			credentialsFileContent: getCredentialsFileContent(TypeServiceAccount, accessID, privateKey),
			errorExpectedOnResolve: false,
			expectedCredentials:    getExpectedCredentials(accessID, privateKey),
		},
		"credentials in credentials file - unsupported type credentials file": {
			config:                 &common.CacheGCSConfig{},
			credentialsFileContent: getCredentialsFileContent("unknown_type", "", ""),
			errorExpectedOnResolve: true,
		},
		"credentials in both places - credentials file takes precedence": {
			config:                 getCredentialsConfig(accessID, privateKey),
			credentialsFileContent: getCredentialsFileContent(TypeServiceAccount, accessID2, privateKey),
			errorExpectedOnResolve: false,
			expectedCredentials:    getExpectedCredentials(accessID2, privateKey),
		},
		"credentials in non-existing credentials file": {
			config:                      &common.CacheGCSConfig{},
			credentialsFileContent:      getCredentialsFileContent(TypeServiceAccount, accessID, privateKey),
			credentialsFileDoesNotExist: true,
			errorExpectedOnResolve:      true,
		},
		"credentials in credentials file - invalid JSON": {
			config:                         &common.CacheGCSConfig{},
			credentialsFileContent:         getCredentialsFileContent(TypeServiceAccount, accessID, privateKey),
			credentialsFileWithInvalidJSON: true,
			errorExpectedOnResolve:         true,
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			cleanupCredentialsFileMock := prepareStubbedCredentialsFile(t, testCase)
			defer cleanupCredentialsFileMock()

			cr, err := newDefaultCredentialsResolver(testCase.config)

			if testCase.errorExpectedOnInitialization {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err, "Error on resolver initialization is not expected")

			err = cr.Resolve()

			if testCase.errorExpectedOnResolve {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err, "Error on credentials resolving is not expected")
			assert.Equal(t, testCase.expectedCredentials, cr.Credentials())
		})
	}
}
