//go:build !integration

package gitlab_secrets_manager

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openbao/openbao/api/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	_ "gitlab.com/gitlab-org/gitlab-runner/helpers/vault/secret_engines/kv_v2"
)

func TestResolver_Name(t *testing.T) {
	r := newResolver(common.Secret{})
	assert.Equal(t, resolverName, r.Name())
}

func TestResolver_IsSupported(t *testing.T) {
	tests := map[string]struct {
		secret                   common.Secret
		expectedGitLabSecretsMgr bool
	}{
		"supported secret": {
			secret: common.Secret{
				GitLabSecretsManager: &common.GitLabSecretsManagerSecret{},
			},
			expectedGitLabSecretsMgr: true,
		},
		"unsupported secret": {
			secret:                   common.Secret{},
			expectedGitLabSecretsMgr: false,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			r := newResolver(tt.secret)
			assert.Equal(t, tt.expectedGitLabSecretsMgr, r.IsSupported())
		})
	}
}

func TestResolver_Resolve(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/sys/health":
			require.NoError(t, json.NewEncoder(w).Encode(api.HealthResponse{
				Initialized: true,
				Sealed:      false,
			}))
		case "/v1/test_path/data/test_path":
			require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{
					"data": map[string]interface{}{
						"test_field": "test_value",
					},
					"metadata": map[string]interface{}{
						"version": 1,
					},
				},
			}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	testCases := []struct {
		name          string
		secret        common.Secret
		expectedErr   string
		expectedValue string
	}{
		{
			name:        "unsupported",
			expectedErr: "trying to resolve unsupported secret",
		},
		{
			name: "failure creating vault client",
			secret: common.Secret{
				GitLabSecretsManager: &common.GitLabSecretsManagerSecret{},
			},
			expectedErr: "creating vault client",
		},
		{
			name: "failure get secret",
			secret: common.Secret{
				GitLabSecretsManager: &common.GitLabSecretsManagerSecret{
					Server: common.GitLabSecretsManagerServer{
						URL: server.URL,
						InlineAuth: common.GitLabSecretsManagerServerInlineAuth{
							AuthMount: "jwt",
							JWT:       "test-jwt",
							Role:      "test-role",
						},
					},
				},
			},
			expectedErr: "getting secret",
		},
		{
			name: "success",
			secret: common.Secret{
				GitLabSecretsManager: &common.GitLabSecretsManagerSecret{
					Server: common.GitLabSecretsManagerServer{
						URL: server.URL,
						InlineAuth: common.GitLabSecretsManagerServerInlineAuth{
							AuthMount: "jwt",
							JWT:       "test-jwt",
							Role:      "test-role",
						},
					},
					Engine: common.GitLabSecretsManagerEngine{
						Name: "kv-v2",
						Path: "test_path",
					},
					Path:  "test_path",
					Field: "test_field",
				},
			},
			expectedValue: "test_value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resolver := newResolver(tc.secret)
			value, err := resolver.Resolve()

			if tc.expectedErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectedErr)
				assert.Empty(t, value)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedValue, value)
			}
		})
	}
}
