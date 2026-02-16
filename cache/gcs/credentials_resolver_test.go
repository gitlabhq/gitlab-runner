//go:build !integration

package gcs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"cloud.google.com/go/iam/credentials/apiv1/credentialspb"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
)

var accessID2 = "test-access-id-2@X.iam.gserviceaccount.com"

type credentialsResolverTestCase struct {
	config                         *cacheconfig.CacheGCSConfig
	credentialsFileContent         *credentialsFile
	credentialsFileDoesNotExist    bool
	credentialsFileWithInvalidJSON bool
	metadataServerError            bool
	errorExpectedOnInitialization  bool
	errorExpectedOnResolve         bool
	expectedCredentials            *cacheconfig.CacheGCSCredentials
}

func getCredentialsConfig(accessID string, privateKey string) *cacheconfig.CacheGCSConfig {
	return &cacheconfig.CacheGCSConfig{
		CacheGCSCredentials: cacheconfig.CacheGCSCredentials{
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

func getExpectedCredentials(accessID string, privateKey string) *cacheconfig.CacheGCSCredentials {
	return &cacheconfig.CacheGCSCredentials{
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
			config:                 &cacheconfig.CacheGCSConfig{},
			errorExpectedOnResolve: false,
			expectedCredentials:    getExpectedCredentials(accessID, ""),
		},
		"credentials not set - metadata server error": {
			config:                 &cacheconfig.CacheGCSConfig{},
			metadataServerError:    true,
			errorExpectedOnResolve: true,
		},
		"credentials direct in config": {
			config:                 getCredentialsConfig(accessID, privateKey),
			errorExpectedOnResolve: false,
			expectedCredentials:    getExpectedCredentials(accessID, privateKey),
		},
		"credentials direct in config - only accessID": {
			config:                 getCredentialsConfig(accessID, ""),
			errorExpectedOnResolve: true,
		},
		"credentials direct in config - only privatekey": {
			config:                 getCredentialsConfig("", privateKey),
			errorExpectedOnResolve: true,
		},
		"credentials in credentials file - service account file": {
			config:                 &cacheconfig.CacheGCSConfig{},
			credentialsFileContent: getCredentialsFileContent(TypeServiceAccount, accessID, privateKey),
			errorExpectedOnResolve: false,
			expectedCredentials:    getExpectedCredentials(accessID, privateKey),
		},
		"credentials in credentials file - unsupported type credentials file": {
			config:                 &cacheconfig.CacheGCSConfig{},
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
			config:                      &cacheconfig.CacheGCSConfig{},
			credentialsFileContent:      getCredentialsFileContent(TypeServiceAccount, accessID, privateKey),
			credentialsFileDoesNotExist: true,
			errorExpectedOnResolve:      true,
		},
		"credentials in credentials file - invalid JSON": {
			config:                         &cacheconfig.CacheGCSConfig{},
			credentialsFileContent:         getCredentialsFileContent(TypeServiceAccount, accessID, privateKey),
			credentialsFileWithInvalidJSON: true,
			errorExpectedOnResolve:         true,
		},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			if testCase.credentialsFileContent != nil {
				pathname := filepath.Join(t.TempDir(), "gcp-credentials-file")

				testCase.config.CredentialsFile = pathname

				switch {
				case testCase.credentialsFileDoesNotExist:
					// no-op
				case testCase.credentialsFileWithInvalidJSON:
					require.NoError(t, os.WriteFile(pathname, []byte("a"), 0o600))
				default:
					data, err := json.Marshal(testCase.credentialsFileContent)
					require.NoError(t, err)
					require.NoError(t, os.WriteFile(pathname, data, 0o600))
				}
			}

			mc := NewMockMetadataClient(t)
			metadataCall := mc.On("Email", mock.Anything).Maybe()
			if testCase.metadataServerError {
				metadataCall.Return("", fmt.Errorf("test error"))
			} else {
				metadataCall.Return(accessID, nil)
			}
			cr, err := newDefaultCredentialsResolver(testCase.config)

			if testCase.errorExpectedOnInitialization {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err, "Error on resolver initialization is not expected")
			cr.metadataClient = mc

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

type signBytesOperationTestCase struct {
	returnError error
	output      []byte
}

func TestSignBytesOperation(t *testing.T) {
	tests := map[string]signBytesOperationTestCase{
		"valid-sign": {
			returnError: nil,
			output:      []byte("output"),
		},
		"error": {
			returnError: errors.New("error"),
			output:      nil,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			config := getCredentialsConfig(accessID, "")

			sbr := credentialspb.SignBlobResponse{SignedBlob: tc.output}

			icc := NewMockIamCredentialsClient(t)
			signBlobCall := icc.On("SignBlob", mock.Anything, mock.Anything).Maybe()
			cr, _ := newDefaultCredentialsResolver(config)
			if tc.returnError == nil {
				cr.credentialsClient = icc
				signBlobCall.Return(&sbr, nil)
			} else {
				signBlobCall.Return(nil, tc.returnError)
			}

			signed, err := cr.SignBytesFunc(t.Context())([]byte("input"))

			if tc.returnError == nil {
				assert.Nil(t, err)
				assert.Equal(t, signed, tc.output)
			} else {
				assert.ErrorAs(t, err, &tc.returnError)
				assert.Nil(t, signed)
			}
		})
	}
}
