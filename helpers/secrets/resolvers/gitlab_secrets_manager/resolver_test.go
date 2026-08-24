//go:build !integration

package gitlab_secrets_manager

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openbao/openbao/api/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/secrets"
	_ "gitlab.com/gitlab-org/gitlab-runner/helpers/vault/secret_engines/kv_v2"
)

func TestResolver_Name(t *testing.T) {
	r := newResolver(spec.Secret{})
	assert.Equal(t, resolverName, r.Name())
}

func TestResolver_IsSupported(t *testing.T) {
	tests := map[string]struct {
		secret                   spec.Secret
		expectedGitLabSecretsMgr bool
	}{
		"supported secret": {
			secret: spec.Secret{
				GitLabSecretsManager: &spec.GitLabSecretsManagerSecret{},
			},
			expectedGitLabSecretsMgr: true,
		},
		"unsupported secret": {
			secret:                   spec.Secret{},
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
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"data": map[string]any{
					"data": map[string]any{
						"test_field": "test_value",
					},
					"metadata": map[string]any{
						"version": 1,
					},
				},
			}))
		case "/v1/forbidden_path/data/forbidden_path":
			w.WriteHeader(http.StatusForbidden)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"errors": []string{"permission denied"},
			}))
		case "/v1/unavailable_path/data/unavailable_path":
			w.WriteHeader(http.StatusServiceUnavailable)
			require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
				"errors": []string{"service unavailable"},
			}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	testCases := []struct {
		name                          string
		secret                        spec.Secret
		expectedErr                   string
		expectedValue                 string
		expectConfigurationError      bool
		expectExternalDependencyError bool
	}{
		{
			name:        "unsupported",
			expectedErr: "trying to resolve unsupported secret",
		},
		{
			name: "failure creating vault client",
			secret: spec.Secret{
				GitLabSecretsManager: &spec.GitLabSecretsManagerSecret{},
			},
			expectedErr: "creating vault client",
		},
		{
			name: "failure get secret",
			secret: spec.Secret{
				GitLabSecretsManager: &spec.GitLabSecretsManagerSecret{
					Server: spec.GitLabSecretsManagerServer{
						URL: server.URL,
						InlineAuth: spec.GitLabSecretsManagerServerInlineAuth{
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
			name: "failure get secret with path",
			secret: spec.Secret{
				GitLabSecretsManager: &spec.GitLabSecretsManagerSecret{
					Server: spec.GitLabSecretsManagerServer{
						URL: server.URL,
						InlineAuth: spec.GitLabSecretsManagerServerInlineAuth{
							Path: "auth/jwt/login",
							JWT:  "test-jwt",
							Role: "test-role",
						},
					},
				},
			},
			expectedErr: "getting secret",
		},
		{
			name: "success",
			secret: spec.Secret{
				GitLabSecretsManager: &spec.GitLabSecretsManagerSecret{
					Server: spec.GitLabSecretsManagerServer{
						URL: server.URL,
						InlineAuth: spec.GitLabSecretsManagerServerInlineAuth{
							AuthMount: "jwt",
							JWT:       "test-jwt",
							Role:      "test-role",
						},
					},
					Engine: spec.GitLabSecretsManagerEngine{
						Name: "kv-v2",
						Path: "test_path",
					},
					Path:  "test_path",
					Field: "test_field",
				},
			},
			expectedValue: "test_value",
		},
		{
			name: "success with path",
			secret: spec.Secret{
				GitLabSecretsManager: &spec.GitLabSecretsManagerSecret{
					Server: spec.GitLabSecretsManagerServer{
						URL: server.URL,
						InlineAuth: spec.GitLabSecretsManagerServerInlineAuth{
							Path: "auth/jwt/login",
							JWT:  "test-jwt",
							Role: "test-role",
						},
					},
					Engine: spec.GitLabSecretsManagerEngine{
						Name: "kv-v2",
						Path: "test_path",
					},
					Path:  "test_path",
					Field: "test_field",
				},
			},
			expectedValue: "test_value",
		},
		{
			name: "permission denied is a configuration error",
			secret: spec.Secret{
				GitLabSecretsManager: &spec.GitLabSecretsManagerSecret{
					Server: spec.GitLabSecretsManagerServer{
						URL: server.URL,
						InlineAuth: spec.GitLabSecretsManagerServerInlineAuth{
							AuthMount: "jwt",
							JWT:       "test-jwt",
							Role:      "test-role",
						},
					},
					Engine: spec.GitLabSecretsManagerEngine{
						Name: "kv-v2",
						Path: "forbidden_path",
					},
					Path:  "forbidden_path",
					Field: "test_field",
				},
			},
			expectedErr:              "getting secret",
			expectConfigurationError: true,
		},
		{
			name: "service unavailable is an external dependency error",
			secret: spec.Secret{
				GitLabSecretsManager: &spec.GitLabSecretsManagerSecret{
					Server: spec.GitLabSecretsManagerServer{
						URL: server.URL,
						InlineAuth: spec.GitLabSecretsManagerServerInlineAuth{
							AuthMount: "jwt",
							JWT:       "test-jwt",
							Role:      "test-role",
						},
					},
					Engine: spec.GitLabSecretsManagerEngine{
						Name: "kv-v2",
						Path: "unavailable_path",
					},
					Path:  "unavailable_path",
					Field: "test_field",
				},
			},
			expectedErr:                   "getting secret",
			expectExternalDependencyError: true,
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

				var configurationErr *secrets.ResolvingConfigurationError
				var externalDependencyErr *secrets.ResolvingExternalDependencyError
				assert.Equal(t, tc.expectConfigurationError, errors.As(err, &configurationErr), "configuration error classification")
				assert.Equal(t, tc.expectExternalDependencyError, errors.As(err, &externalDependencyErr), "external dependency classification")
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedValue, value)
			}
		})
	}
}
