//go:build !integration

package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	mock "github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/vault/secret_engines"
)

func TestService_GetSecret(t *testing.T) {
	secret_engines.MustRegisterFactory("test_engine", func(client vault.Client, path string) vault.SecretEngine {
		mse := vault.NewMockSecretEngine(t)
		mse.On("Get", mock.MatchedBy(func(path string) bool {
			return true
		})).Return(func(path string) (map[string]interface{}, error) {
			switch path {
			case "error":
				return nil, errors.New("computer said no")
			case "missing_data":
				return nil, nil
			default:
				return map[string]interface{}{
					"test_field":    "test_value",
					"empty_field":   "",
					"numeric_field": 1234,
				}, nil
			}
		})
		return mse
	})

	testCases := []struct {
		name          string
		secret        *spec.GitLabSecretsManagerSecret
		expectedErr   string
		expectedValue string
	}{
		{
			name: "failed to get secret engine",
			secret: &spec.GitLabSecretsManagerSecret{
				Engine: spec.GitLabSecretsManagerEngine{
					Name: "invalid",
				},
			},
			expectedErr: "getting secret engine",
		},
		{
			name: "failed to get secret data",
			secret: &spec.GitLabSecretsManagerSecret{
				Engine: spec.GitLabSecretsManagerEngine{
					Name: "test_engine",
				},
				Path: "error",
			},
			expectedErr: "get secret data",
		},
		{
			name: "secret not found",
			secret: &spec.GitLabSecretsManagerSecret{
				Engine: spec.GitLabSecretsManagerEngine{
					Name: "test_engine",
				},
				Path: "missing_data",
			},
			expectedErr: "secret not found",
		},
		{
			name: "field not found",
			secret: &spec.GitLabSecretsManagerSecret{
				Engine: spec.GitLabSecretsManagerEngine{
					Name: "test_engine",
				},
				Field: "missing_field",
			},
			expectedErr: `field "missing_field" not found in secret`,
		},
		{
			name: "field exists but empty string",
			secret: &spec.GitLabSecretsManagerSecret{
				Engine: spec.GitLabSecretsManagerEngine{
					Name: "test_engine",
				},
				Path:  "test_path",
				Field: "empty_field",
			},
			expectedValue: "",
		},
		{
			name: "field exists but not string",
			secret: &spec.GitLabSecretsManagerSecret{
				Engine: spec.GitLabSecretsManagerEngine{
					Name: "test_engine",
				},
				Path:  "test_path",
				Field: "numeric_field",
			},
			expectedErr: `field "numeric_field" has invalid type int (expected string)`,
		},
		{
			name: "success",
			secret: &spec.GitLabSecretsManagerSecret{
				Engine: spec.GitLabSecretsManagerEngine{
					Name: "test_engine",
				},
				Field: "test_field",
			},
			expectedValue: "test_value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			gsm := NewGitlabSecretsManager(nil)

			val, err := gsm.GetSecret(tc.secret)

			if tc.expectedErr != "" {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tc.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, val, tc.expectedValue)
			}
		})
	}
}
