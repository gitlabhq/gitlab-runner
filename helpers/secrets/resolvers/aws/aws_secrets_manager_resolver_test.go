//go:build !integration

package aws

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/secrets"
)

func TestResolver_Name(t *testing.T) {
	r := newResolver(common.Secret{})
	assert.Equal(t, resolverName, r.Name())
}

func TestResolver_IsSupported(t *testing.T) {
	tests := map[string]struct {
		secret      common.Secret
		isSupported bool
	}{
		"supported secret": {
			secret: common.Secret{
				AWSSecretsManager: &common.AWSSecret{},
			},
			isSupported: true,
		},
		"unsupported secret": {
			secret:      common.Secret{},
			isSupported: false,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			r := newResolver(tt.secret)
			assert.Equal(t, tt.isSupported, r.IsSupported())
		})
	}
}

func TestResolver_Resolve(t *testing.T) {
	secret := common.Secret{
		AWSSecretsManager: &common.AWSSecret{
			SecretId:     "test",
			VersionId:    "version",
			VersionStage: "version_stage",
			Field:        "Date",
		},
	}

	tests := map[string]struct {
		secret                    common.Secret
		vaultServiceCreationError error
		setupMock                 func(*MockAWSSecretsManager)
		expectedValue             string
		expectedError             error
	}{
		"error on support detection": {
			expectedError: &secrets.ResolvingUnsupportedSecretError{},
		},
		"error on vault service creation": {
			secret:                    secret,
			vaultServiceCreationError: assert.AnError,
			expectedError:             assert.AnError,
		},
		"error on field resolving": {
			secret: secret,
			setupMock: func(m *MockAWSSecretsManager) {
				m.EXPECT().
					GetSecretString(mock.Anything, "test", mock.Anything, mock.Anything).
					Return("", assert.AnError).
					Once()
			},
			expectedError: assert.AnError,
		},
		"field resolved properly": {
			secret: secret,
			setupMock: func(m *MockAWSSecretsManager) {
				m.EXPECT().
					GetSecretString(mock.Anything, "test", mock.Anything, mock.Anything).
					Return(`{"Date":"2020-08-24"}`, nil).
					Once()
			},
			expectedValue: "2020-08-24",
			expectedError: nil,
		},
		"field not found in JSON": {
			secret: secret,
			setupMock: func(m *MockAWSSecretsManager) {
				m.EXPECT().
					GetSecretString(mock.Anything, "test", mock.Anything, mock.Anything).
					Return(`{"Other":"value"}`, nil).
					Once()
			},
			expectedError: fmt.Errorf("key 'Date' not found in AWS Secrets Manager response for secret 'test'"),
		},
		"invalid JSON returned": {
			secret: secret,
			setupMock: func(m *MockAWSSecretsManager) {
				m.EXPECT().
					GetSecretString(mock.Anything, "test", mock.Anything, mock.Anything).
					Return(`not-a-json`, nil).
					Once()
			},
			expectedError: fmt.Errorf("failed to parse JSON for secret 'test'"),
		},
		"error when JWT is provided without RoleArn": {
			secret: common.Secret{
				AWSSecretsManager: &common.AWSSecret{
					SecretId: "test-secret",
					Server: common.AWSServer{
						JWT:    "dummy-jwt-token",
						Region: "us-east-1",
					},
				},
			},
			expectedError: fmt.Errorf("Role ARN is required when using JWT for AWS authentication"),
		},
		"uses server region when secret region is empty": {
			secret: common.Secret{
				AWSSecretsManager: &common.AWSSecret{
					SecretId: "test-secret",
					Server: common.AWSServer{
						Region: "us-west-2",
					},
				},
			},
			setupMock: func(m *MockAWSSecretsManager) {
				m.EXPECT().
					GetSecretString(mock.Anything, "test-secret", mock.Anything, mock.Anything).
					Return("secret-value", nil).
					Once()
			},
			expectedValue: "secret-value",
		},
		"plain text secret with no field specified": {
			secret: common.Secret{
				AWSSecretsManager: &common.AWSSecret{
					SecretId: "test-secret",
					Region:   "us-east-1",
				},
			},
			setupMock: func(m *MockAWSSecretsManager) {
				m.EXPECT().
					GetSecretString(mock.Anything, "test-secret", mock.Anything, mock.Anything).
					Return("plain-text-secret", nil).
					Once()
			},
			expectedValue: "plain-text-secret",
		},
		"number value": {
			secret: common.Secret{
				AWSSecretsManager: &common.AWSSecret{
					SecretId: "test",
					Field:    "foo",
				},
			},
			setupMock: func(m *MockAWSSecretsManager) {
				m.EXPECT().
					GetSecretString(mock.Anything, "test", mock.Anything, mock.Anything).
					Return(`{"foo":42}`, nil).
					Once()
			},
			expectedValue: "42",
		},
		"boolean value": {
			secret: common.Secret{
				AWSSecretsManager: &common.AWSSecret{
					SecretId: "test",
					Field:    "active",
				},
			},
			setupMock: func(m *MockAWSSecretsManager) {
				m.EXPECT().
					GetSecretString(mock.Anything, "test", mock.Anything, mock.Anything).
					Return(`{"active":false}`, nil).
					Once()
			},
			expectedValue: "false",
		},
		"object as value": {
			secret: common.Secret{
				AWSSecretsManager: &common.AWSSecret{
					SecretId: "test",
					Field:    "field",
				},
			},
			setupMock: func(m *MockAWSSecretsManager) {
				m.EXPECT().
					GetSecretString(mock.Anything, "test", mock.Anything, mock.Anything).
					Return(`{"field":{"bar":123}}`, nil).
					Once()
			},
			expectedError: fmt.Errorf("key 'field' in aws secrets manager response for secret 'test' is not a string, number or boolean"),
		},
		"array as value": {
			secret: common.Secret{
				AWSSecretsManager: &common.AWSSecret{
					SecretId: "test",
					Field:    "field",
				},
			},
			setupMock: func(m *MockAWSSecretsManager) {
				m.EXPECT().
					GetSecretString(mock.Anything, "test", mock.Anything, mock.Anything).
					Return(`{"field":[1,2,3]}`, nil).
					Once()
			},
			expectedError: fmt.Errorf("key 'field' in aws secrets manager response for secret 'test' is not a string, number or boolean"),
		},
		"uses default credentials when roleArn is empty and no JWT": {
			secret: common.Secret{
				AWSSecretsManager: &common.AWSSecret{
					SecretId: "test-secret",
					Server: common.AWSServer{
						Region: "us-east-1",
						// No JWT and no RoleArn - should use default credentials
					},
				},
			},
			setupMock: func(m *MockAWSSecretsManager) {
				m.EXPECT().
					GetSecretString(mock.Anything, "test-secret", mock.Anything, mock.Anything).
					Return("secret-value-with-default-creds", nil).
					Once()
			},
			expectedValue: "secret-value-with-default-creds",
			expectedError: nil,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			oldAWSSecretsManagerService := newAWSSecretsManagerService
			defer func() { newAWSSecretsManagerService = oldAWSSecretsManagerService }()

			var mockSvc *MockAWSSecretsManager
			if tt.setupMock != nil {
				mockSvc = NewMockAWSSecretsManager(t)
				tt.setupMock(mockSvc)
			}

			newAWSSecretsManagerService = func(ctx context.Context, region string, webIdentityProvider *stscreds.WebIdentityRoleProvider) (AWSSecretsManager, error) {
				if tt.vaultServiceCreationError != nil {
					return nil, tt.vaultServiceCreationError
				}
				if mockSvc != nil {
					return mockSvc, nil
				}
				return NewMockAWSSecretsManager(t), nil
			}

			r := newResolver(tt.secret)
			value, err := r.Resolve()

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedValue, value)
		})
	}
}
