//go:build !integration

package service

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"
)

// Helper function to create a string pointer
func stringPtr(s string) *string {
	return &s
}

func TestAWSSecretsManager_GetSecretString(t *testing.T) {
	tests := map[string]struct {
		mockResponse  *secretsmanager.GetSecretValueOutput
		mockError     error
		expectedValue string
		expectError   bool
		errorContains string
	}{
		"Success": {
			mockResponse: &secretsmanager.GetSecretValueOutput{
				SecretString: stringPtr("my-secret"),
			},
			mockError:     nil,
			expectedValue: "my-secret",
			expectError:   false,
		},
		"Error": {
			mockResponse:  nil,
			mockError:     errors.New("aws error"),
			expectedValue: "",
			expectError:   true,
		},
		"BinarySuccess": {
			mockResponse: &secretsmanager.GetSecretValueOutput{
				SecretString: nil,
				SecretBinary: []byte("hello world"),
			},
			mockError:     nil,
			expectedValue: "aGVsbG8gd29ybGQ=",
			expectError:   false,
		},
		"RealBinarySuccess": {
			mockResponse: &secretsmanager.GetSecretValueOutput{
				SecretString: nil,
				SecretBinary: []byte{0x00, 0xff, 0x10, 0x20, 0x7f},
			},
			mockError:     nil,
			expectedValue: "AP8QIH8=",
			expectError:   false,
		},
		"EmptySecret": {
			mockResponse: &secretsmanager.GetSecretValueOutput{
				SecretString: nil,
				SecretBinary: nil,
			},
			mockError:     nil,
			expectedValue: "",
			expectError:   true,
			errorContains: "secret contains no value",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			mockClient := NewMockSecretsManagerAPI(t)
			mockClient.
				On("GetSecretValue", mock.Anything, mock.Anything, mock.Anything).
				Return(tt.mockResponse, tt.mockError)

			manager := &awsSecretsManager{client: mockClient}
			val, err := manager.GetSecretString(t.Context(), "id", nil, nil)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
				assert.Empty(t, val)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedValue, val)
			}
		})
	}
}

func TestNewAWSSecretsManager(t *testing.T) {
	ctx := t.Context()
	manager, err := NewAWSSecretsManager(ctx, "some-region", nil)
	assert.NoError(t, err)
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.client)
	assert.NotEmpty(t, manager.client.(*secretsmanager.Client).Options().AppID)
}

func TestNewWebIdentityRoleProvider(t *testing.T) {
	provider := NewWebIdentityRoleProvider("some-region", "arn:aws:iam::123456789012:role/test", "token", "sessionName")
	assert.NotNil(t, provider)
}
