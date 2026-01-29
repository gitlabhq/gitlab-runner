//go:build !integration

package common

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

func TestCacheCheckPolicy(t *testing.T) {
	for num, tc := range []struct {
		object      spec.CachePolicy
		subject     spec.CachePolicy
		expected    bool
		expectErr   bool
		description string
	}{
		{spec.CachePolicyPullPush, spec.CachePolicyPull, true, false, "pull-push allows pull"},
		{spec.CachePolicyPullPush, spec.CachePolicyPush, true, false, "pull-push allows push"},
		{spec.CachePolicyUndefined, spec.CachePolicyPull, true, false, "undefined allows pull"},
		{spec.CachePolicyUndefined, spec.CachePolicyPush, true, false, "undefined allows push"},
		{spec.CachePolicyPull, spec.CachePolicyPull, true, false, "pull allows pull"},
		{spec.CachePolicyPull, spec.CachePolicyPush, false, false, "pull forbids push"},
		{spec.CachePolicyPush, spec.CachePolicyPull, false, false, "push forbids pull"},
		{spec.CachePolicyPush, spec.CachePolicyPush, true, false, "push allows push"},
		{"unknown", spec.CachePolicyPull, false, true, "unknown raises error on pull"},
		{"unknown", spec.CachePolicyPush, false, true, "unknown raises error on push"},
	} {
		cache := spec.Cache{Policy: tc.object}

		result, err := cache.CheckPolicy(tc.subject)
		if tc.expectErr {
			assert.Errorf(t, err, "case %d: %s", num, tc.description)
		} else {
			assert.NoErrorf(t, err, "case %d: %s", num, tc.description)
		}

		assert.Equal(t, tc.expected, result, "case %d: %s", num, tc.description)
	}
}

func TestShouldCache(t *testing.T) {
	for _, params := range []struct {
		jobSuccess          bool
		when                spec.CacheWhen
		expectedShouldCache bool
	}{
		{true, spec.CacheWhenOnSuccess, true},
		{true, spec.CacheWhenAlways, true},
		{true, spec.CacheWhenOnFailure, false},
		{false, spec.CacheWhenOnSuccess, false},
		{false, spec.CacheWhenAlways, true},
		{false, spec.CacheWhenOnFailure, true},
	} {
		tn := "jobSuccess=" + strconv.FormatBool(params.jobSuccess) + ",when=" + string(params.when)

		t.Run(tn, func(t *testing.T) {
			expected := params.expectedShouldCache

			actual := params.when.ShouldCache(params.jobSuccess)

			assert.Equal(
				t,
				actual,
				expected,
				"Value returned from ShouldCache was not as expected",
			)
		})
	}
}

func TestSecrets_expandVariables(t *testing.T) {
	testServerURL := "server-url"
	testNamespace := "custom-namespace"
	testAuthName := "auth-name"
	testAuthPath := "auth-path"
	testAuthJWT := "auth-jwt"
	testAuthRole := "auth-role"
	testAuthUnknown := "auth-unknown"
	testEngineName := "engine-name"
	testEnginePath := "engine-path"
	testPath := "secret-path"
	testField := "secret-field"

	variables := spec.Variables{
		{Key: "CI_VAULT_SERVER_URL", Value: testServerURL},
		{Key: "CI_VAULT_NAMESPACE", Value: testNamespace},
		{Key: "CI_VAULT_AUTH_NAME", Value: testAuthName},
		{Key: "CI_VAULT_AUTH_PATH", Value: testAuthPath},
		{Key: "CI_VAULT_AUTH_JWT", Value: testAuthJWT},
		{Key: "CI_VAULT_AUTH_ROLE", Value: testAuthRole},
		{Key: "CI_VAULT_AUTH_UNKNOWN_DATA", Value: testAuthUnknown},
		{Key: "CI_VAULT_ENGINE_NAME", Value: testEngineName},
		{Key: "CI_VAULT_ENGINE_PATH", Value: testEnginePath},
		{Key: "CI_VAULT_PATH", Value: testPath},
		{Key: "CI_VAULT_FIELD", Value: testField},
	}

	assertValue := func(t *testing.T, prefix string, variableValue string, testedValue interface{}) {
		assert.Equal(
			t,
			fmt.Sprintf("%s %s", prefix, variableValue),
			testedValue,
		)
	}

	tests := map[string]struct {
		secrets       spec.Secrets
		assertSecrets func(t *testing.T, secrets spec.Secrets)
	}{
		"no secrets defined": {
			secrets: nil,
			assertSecrets: func(t *testing.T, secrets spec.Secrets) {
				assert.Nil(t, secrets)
			},
		},
		"nil vault secret": {
			secrets: spec.Secrets{
				"VAULT": spec.Secret{
					Vault: nil,
				},
			},
			assertSecrets: func(t *testing.T, secrets spec.Secrets) {
				assert.Nil(t, secrets["VAULT"].Vault)
			},
		},
		"vault missing data": {
			secrets: spec.Secrets{
				"VAULT": spec.Secret{
					Vault: &spec.VaultSecret{},
				},
			},
			assertSecrets: func(t *testing.T, secrets spec.Secrets) {
				assert.NotNil(t, secrets["VAULT"].Vault)
			},
		},
		"vault missing jwt data": {
			secrets: spec.Secrets{
				"VAULT": spec.Secret{
					Vault: &spec.VaultSecret{
						Server: spec.VaultServer{
							Auth: spec.VaultAuth{
								Data: map[string]interface{}{
									"role": testAuthRole,
								},
							},
						},
					},
				},
			},
			assertSecrets: func(t *testing.T, secrets spec.Secrets) {
				require.NotNil(t, secrets["VAULT"].Vault)
				assert.Equal(t, testAuthRole, secrets["VAULT"].Vault.Server.Auth.Data["role"])
			},
		},
		"vault secret defined": {
			secrets: spec.Secrets{
				"VAULT": spec.Secret{
					Vault: &spec.VaultSecret{
						Server: spec.VaultServer{
							URL:       "url ${CI_VAULT_SERVER_URL}",
							Namespace: "namespace ${CI_VAULT_NAMESPACE}",
							Auth: spec.VaultAuth{
								Name: "name ${CI_VAULT_AUTH_NAME}",
								Path: "path ${CI_VAULT_AUTH_PATH}",
								Data: map[string]interface{}{
									"jwt":     "jwt ${CI_VAULT_AUTH_JWT}",
									"role":    "role ${CI_VAULT_AUTH_ROLE}",
									"unknown": "unknown ${CI_VAULT_AUTH_UNKNOWN_DATA}",
								},
							},
						},
						Engine: spec.VaultEngine{
							Name: "name ${CI_VAULT_ENGINE_NAME}",
							Path: "path ${CI_VAULT_ENGINE_PATH}",
						},
						Path:  "path ${CI_VAULT_PATH}",
						Field: "field ${CI_VAULT_FIELD}",
					},
				},
			},
			assertSecrets: func(t *testing.T, secrets spec.Secrets) {
				require.NotNil(t, secrets["VAULT"].Vault)
				assertValue(t, "url", testServerURL, secrets["VAULT"].Vault.Server.URL)
				assertValue(t, "namespace", testNamespace, secrets["VAULT"].Vault.Server.Namespace)
				assertValue(t, "name", testAuthName, secrets["VAULT"].Vault.Server.Auth.Name)
				assertValue(t, "path", testAuthPath, secrets["VAULT"].Vault.Server.Auth.Path)
				require.NotNil(t, secrets["VAULT"].Vault.Server.Auth.Data["jwt"])
				assertValue(t, "jwt", testAuthJWT, secrets["VAULT"].Vault.Server.Auth.Data["jwt"])
				require.NotNil(t, secrets["VAULT"].Vault.Server.Auth.Data["role"])
				assertValue(t, "role", testAuthRole, secrets["VAULT"].Vault.Server.Auth.Data["role"])
				require.NotNil(t, secrets["VAULT"].Vault.Server.Auth.Data["unknown"])
				assertValue(t, "unknown", testAuthUnknown, secrets["VAULT"].Vault.Server.Auth.Data["unknown"])
				assertValue(t, "name", testEngineName, secrets["VAULT"].Vault.Engine.Name)
				assertValue(t, "path", testEnginePath, secrets["VAULT"].Vault.Engine.Path)
				assertValue(t, "path", testPath, secrets["VAULT"].Vault.Path)
				assertValue(t, "field", testField, secrets["VAULT"].Vault.Field)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.NotPanics(t, func() {
				tt.secrets.ExpandVariables(variables)
				tt.assertSecrets(t, tt.secrets)
			})
		})
	}
}

func TestGCPSecretManagerSecrets_expandVariables(t *testing.T) {
	secretName := "my-secret-1234"
	secretVersion := "version-999"
	projectNumber := "8888"
	poolId := "my-pool-123"
	providerId := "my-provider-123"
	jwt := "my-jwt"

	variables := spec.Variables{
		{Key: "NAME", Value: secretName},
		{Key: "VERSION", Value: secretVersion},
		{Key: "PROJECT_NUMBER", Value: projectNumber},
		{Key: "POOL_ID", Value: poolId},
		{Key: "PROVIDER_ID", Value: providerId},
		{Key: "JWT", Value: jwt},
	}

	tests := map[string]struct {
		secrets       spec.Secrets
		assertSecrets func(t *testing.T, secrets spec.Secrets)
	}{
		"no secrets defined": {
			secrets: nil,
			assertSecrets: func(t *testing.T, secrets spec.Secrets) {
				assert.Nil(t, secrets)
			},
		},
		"empty data": {
			secrets: spec.Secrets{
				"VAULT": spec.Secret{
					GCPSecretManager: &spec.GCPSecretManagerSecret{},
				},
			},
			assertSecrets: func(t *testing.T, secrets spec.Secrets) {
				assert.Equal(t, &spec.GCPSecretManagerSecret{}, secrets["VAULT"].GCPSecretManager)
			},
		},
		"without expansion": {
			secrets: spec.Secrets{
				"VAULT": spec.Secret{
					GCPSecretManager: &spec.GCPSecretManagerSecret{
						Name:    "my-secret",
						Version: "latest",
						Server: spec.GCPSecretManagerServer{
							ProjectNumber:                        "1234",
							WorkloadIdentityFederationPoolId:     "pool-id",
							WorkloadIdentityFederationProviderID: "provider-id",
							JWT:                                  "jwt",
						},
					},
				},
			},
			assertSecrets: func(t *testing.T, secrets spec.Secrets) {
				assert.Equal(t, "my-secret", secrets["VAULT"].GCPSecretManager.Name)
				assert.Equal(t, "latest", secrets["VAULT"].GCPSecretManager.Version)
				assert.Equal(t, "1234", secrets["VAULT"].GCPSecretManager.Server.ProjectNumber)
				assert.Equal(t, "pool-id", secrets["VAULT"].GCPSecretManager.Server.WorkloadIdentityFederationPoolId)
				assert.Equal(t, "provider-id", secrets["VAULT"].GCPSecretManager.Server.WorkloadIdentityFederationProviderID)
				assert.Equal(t, "jwt", secrets["VAULT"].GCPSecretManager.Server.JWT)
			},
		},
		"with expansion": {
			secrets: spec.Secrets{
				"VAULT": spec.Secret{
					GCPSecretManager: &spec.GCPSecretManagerSecret{
						Name:    "$NAME",
						Version: "$VERSION",
						Server: spec.GCPSecretManagerServer{
							ProjectNumber:                        "$PROJECT_NUMBER",
							WorkloadIdentityFederationPoolId:     "$POOL_ID",
							WorkloadIdentityFederationProviderID: "$PROVIDER_ID",
							JWT:                                  "$JWT",
						},
					},
				},
			},
			assertSecrets: func(t *testing.T, secrets spec.Secrets) {
				assert.Equal(t, secretName, secrets["VAULT"].GCPSecretManager.Name)
				assert.Equal(t, secretVersion, secrets["VAULT"].GCPSecretManager.Version)
				assert.Equal(t, projectNumber, secrets["VAULT"].GCPSecretManager.Server.ProjectNumber)
				assert.Equal(t, poolId, secrets["VAULT"].GCPSecretManager.Server.WorkloadIdentityFederationPoolId)
				assert.Equal(t, providerId, secrets["VAULT"].GCPSecretManager.Server.WorkloadIdentityFederationProviderID)
				assert.Equal(t, jwt, secrets["VAULT"].GCPSecretManager.Server.JWT)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.NotPanics(t, func() {
				tt.secrets.ExpandVariables(variables)
				tt.assertSecrets(t, tt.secrets)
			})
		})
	}
}

func TestAzureKeyVaultSecrets_expandVariables(t *testing.T) {
	testName := "key-name"
	testVersion := "key-version"
	testAuthJWT := "auth-jwt"

	variables := spec.Variables{
		{Key: "CI_AZURE_KEY_VAULT_KEY_NAME", Value: testName},
		{Key: "CI_AZURE_KEY_VAULT_KEY_VERSION", Value: testVersion},
		{Key: "CI_AZURE_KEY_VAULT_AUTH_JWT", Value: testAuthJWT},
	}

	assertValue := func(t *testing.T, prefix string, variableValue string, testedValue interface{}) {
		assert.Equal(
			t,
			fmt.Sprintf("%s %s", prefix, variableValue),
			testedValue,
		)
	}

	tests := map[string]struct {
		secrets       spec.Secrets
		assertSecrets func(t *testing.T, secrets spec.Secrets)
	}{
		"no secrets defined": {
			secrets: nil,
			assertSecrets: func(t *testing.T, secrets spec.Secrets) {
				assert.Nil(t, secrets)
			},
		},
		"nil vault secret": {
			secrets: spec.Secrets{
				"VAULT": spec.Secret{
					AzureKeyVault: nil,
				},
			},
			assertSecrets: func(t *testing.T, secrets spec.Secrets) {
				assert.Nil(t, secrets["VAULT"].Vault)
			},
		},
		"vault missing data": {
			secrets: spec.Secrets{
				"VAULT": spec.Secret{
					AzureKeyVault: &spec.AzureKeyVaultSecret{},
				},
			},
			assertSecrets: func(t *testing.T, secrets spec.Secrets) {
				assert.NotNil(t, secrets["VAULT"].AzureKeyVault)
			},
		},
		"vault missing jwt data": {
			secrets: spec.Secrets{
				"VAULT": spec.Secret{
					AzureKeyVault: &spec.AzureKeyVaultSecret{
						Name:    testName,
						Version: testVersion,
						Server: spec.AzureKeyVaultServer{
							ClientID: "test_client_id",
							TenantID: "test_tenant_id",
							URL:      "test_url",
						},
					},
				},
			},
			assertSecrets: func(t *testing.T, secrets spec.Secrets) {
				require.NotNil(t, secrets["VAULT"].AzureKeyVault)
				assert.Equal(t, testName, secrets["VAULT"].AzureKeyVault.Name)
				assert.Equal(t, testVersion, secrets["VAULT"].AzureKeyVault.Version)
			},
		},
		"vault secret defined": {
			secrets: spec.Secrets{
				"VAULT": spec.Secret{
					AzureKeyVault: &spec.AzureKeyVaultSecret{
						Name:    "name ${CI_AZURE_KEY_VAULT_KEY_NAME}",
						Version: "version ${CI_AZURE_KEY_VAULT_KEY_VERSION}",
						Server: spec.AzureKeyVaultServer{
							ClientID: "client_id",
							TenantID: "tenant_id",
							JWT:      "jwt ${CI_AZURE_KEY_VAULT_AUTH_JWT}",
							URL:      "url",
						},
					},
				},
			},
			assertSecrets: func(t *testing.T, secrets spec.Secrets) {
				require.NotNil(t, secrets["VAULT"].AzureKeyVault)
				assertValue(t, "name", testName, secrets["VAULT"].AzureKeyVault.Name)
				assertValue(t, "version", testVersion, secrets["VAULT"].AzureKeyVault.Version)
				assertValue(t, "jwt", testAuthJWT, secrets["VAULT"].AzureKeyVault.Server.JWT)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.NotPanics(t, func() {
				tt.secrets.ExpandVariables(variables)
				tt.assertSecrets(t, tt.secrets)
			})
		})
	}
}

func TestJobResponse_JobURL(t *testing.T) {
	jobID := int64(1)

	testCases := map[string]string{
		"http://user:pass@gitlab.example.com/my-namespace/my-project.git":     "http://gitlab.example.com/my-namespace/my-project/-/jobs/1",
		"http://user:pass@gitlab.example.com/my-namespace/my-project":         "http://gitlab.example.com/my-namespace/my-project/-/jobs/1",
		"http://user:pass@gitlab.example.com/my-namespace/my.git.project.git": "http://gitlab.example.com/my-namespace/my.git.project/-/jobs/1",
		"http://user:pass@gitlab.example.com/my-namespace/my.git.project":     "http://gitlab.example.com/my-namespace/my.git.project/-/jobs/1",
	}

	for repoURL, expectedURL := range testCases {
		job := spec.Job{
			ID: jobID,
			GitInfo: spec.GitInfo{
				RepoURL: repoURL,
			},
		}

		assert.Equal(t, expectedURL, job.JobURL())
	}
}

func Test_Image_ExecutorOptions_UnmarshalJSON(t *testing.T) {
	emptyUser := spec.StringOrInt64("")
	uid1000 := spec.StringOrInt64("1000")
	ubuntuUser := spec.StringOrInt64("ubuntu")

	tests := map[string]struct {
		json           string
		expected       func(*testing.T, spec.Image)
		expectedErrMsg []string
	}{
		"no executor_opts": {
			json: `{"executor_opts":{}}`,
			expected: func(t *testing.T, i spec.Image) {
				assert.Equal(t, "", i.ExecutorOptions.Docker.Platform)
				assert.Equal(t, emptyUser, i.ExecutorOptions.Docker.User)
			},
		},
		"docker, empty": {
			json: `{"executor_opts":{"docker": {}}}`,
			expected: func(t *testing.T, i spec.Image) {
				assert.Equal(t, "", i.ExecutorOptions.Docker.Platform)
				assert.Equal(t, emptyUser, i.ExecutorOptions.Docker.User)
			},
		},
		"docker, only user": {
			json: `{"executor_opts":{"docker": {"user": "ubuntu"}}}`,
			expected: func(t *testing.T, i spec.Image) {
				assert.Equal(t, "", i.ExecutorOptions.Docker.Platform)
				assert.Equal(t, ubuntuUser, i.ExecutorOptions.Docker.User)
			},
		},
		"docker, only platform": {
			json: `{"executor_opts":{"docker": {"platform": "amd64"}}}`,
			expected: func(t *testing.T, i spec.Image) {
				assert.Equal(t, "amd64", i.ExecutorOptions.Docker.Platform)
				assert.Equal(t, emptyUser, i.ExecutorOptions.Docker.User)
			},
		},
		"docker, all options": {
			json: `{"executor_opts":{"docker": {"platform": "arm64", "user": "ubuntu"}}}`,
			expected: func(t *testing.T, i spec.Image) {
				assert.Equal(t, "arm64", i.ExecutorOptions.Docker.Platform)
				assert.Equal(t, ubuntuUser, i.ExecutorOptions.Docker.User)
			},
		},
		"docker, invalid options": {
			json:           `{"executor_opts":{"docker": {"foobar": 1234}}}`,
			expectedErrMsg: []string{`Unsupported "image" options [foobar] for "docker executor"; supported options are [platform user]`},
			expected: func(t *testing.T, i spec.Image) {
				assert.Equal(t, "", i.ExecutorOptions.Docker.Platform)
				assert.Equal(t, emptyUser, i.ExecutorOptions.Docker.User)
			},
		},
		"kubernetes, empty": {
			json: `{"executor_opts":{"kubernetes": {}}}`,
			expected: func(t *testing.T, i spec.Image) {
				assert.Equal(t, emptyUser, i.ExecutorOptions.Kubernetes.User)
			},
		},
		"kubernetes, all options": {
			json: `{"executor_opts":{"kubernetes": {"user": "1000"}}}`,
			expected: func(t *testing.T, i spec.Image) {
				assert.Equal(t, uid1000, i.ExecutorOptions.Kubernetes.User)
			},
		},
		"kubernetes, user as int64": {
			json: `{"executor_opts":{"kubernetes": {"user": 1000}}}`,
			expected: func(t *testing.T, i spec.Image) {
				assert.Equal(t, uid1000, i.ExecutorOptions.Kubernetes.User)
			},
		},
		"kubernetes, invalid options": {
			json:           `{"executor_opts":{"kubernetes": {"foobar": 1234}}}`,
			expectedErrMsg: []string{`Unsupported "image" options [foobar] for "kubernetes executor"; supported options are [user]`},
			expected: func(t *testing.T, i spec.Image) {
				assert.Equal(t, "", i.ExecutorOptions.Docker.Platform)
				assert.Equal(t, emptyUser, i.ExecutorOptions.Docker.User)
				assert.Equal(t, emptyUser, i.ExecutorOptions.Kubernetes.User)
			},
		},
		"invalid executor": {
			json:           `{"executor_opts":{"k8s": {}}}`,
			expectedErrMsg: []string{`Unsupported "image" options [k8s] for "executor_opts"; supported options are [docker kubernetes]`},
			expected: func(t *testing.T, i spec.Image) {
				assert.Equal(t, "", i.ExecutorOptions.Docker.Platform)
				assert.Equal(t, emptyUser, i.ExecutorOptions.Docker.User)
				assert.Equal(t, emptyUser, i.ExecutorOptions.Kubernetes.User)
			},
		},
		"docker, invalid executor, valid executor, invalid option": {
			json: `{"executor_opts":{"k8s": {}, "docker": {"platform": "amd64", "foobar": 1234}}}`,
			expectedErrMsg: []string{
				`Unsupported "image" options [k8s] for "executor_opts"; supported options are [docker kubernetes]`,
				`Unsupported "image" options [foobar] for "docker executor"; supported options are [platform user]`,
			},
			expected: func(t *testing.T, i spec.Image) {
				assert.Equal(t, "amd64", i.ExecutorOptions.Docker.Platform)
				assert.Equal(t, emptyUser, i.ExecutorOptions.Docker.User)
			},
		},
		"kubernetes, invalid executor, valid executor, invalid option": {
			json: `{"executor_opts":{"dockers": {}, "kubernetes": {"user": "1000", "foobar": 1234}}}`,
			expectedErrMsg: []string{
				`Unsupported "image" options [dockers] for "executor_opts"; supported options are [docker kubernetes]`,
				`Unsupported "image" options [foobar] for "kubernetes executor"; supported options are [user]`,
			},
			expected: func(t *testing.T, i spec.Image) {
				assert.Equal(t, uid1000, i.ExecutorOptions.Kubernetes.User)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := spec.Image{}
			err := json.Unmarshal([]byte(tt.json), &got)
			assert.NoError(t, err)
			tt.expected(t, got)

			if len(tt.expectedErrMsg) == 0 {
				assert.Nil(t, got.UnsupportedOptions())
			} else {
				for i := range tt.expectedErrMsg {
					assert.Contains(t, got.UnsupportedOptions().Error(), tt.expectedErrMsg[i])
				}
			}
		})
	}
}

func TestJobResponse_Run(t *testing.T) {
	tests := map[string]struct {
		json            string
		wantJSON        string
		wantErr         bool
		execNativeSteps bool
	}{
		"steps not requested": {
			json:     `{}`,
			wantJSON: `{}`,
		},
		"steps not requested, image is unmodified": {
			json:     `{"Image":{"Name":"registry.gitlab.com/project/image:v1"}}`,
			wantJSON: `{"Image":{"Name":"registry.gitlab.com/project/image:v1"}}`,
		},
		"steps are requested via shim, default image set": {
			json: `{"Run":"[{\"Name:\":\"hello\",\"Script\":\"echo hello world\"}]"}`,
			wantJSON: `
{
  "Run":"[{\"Name:\":\"hello\",\"Script\":\"echo hello world\"}]",
  "Variables":[
    {
      "Key":"STEPS",
      "Value":"[{\"Name:\":\"hello\",\"Script\":\"echo hello world\"}]",
      "Raw":true
    }
  ],
  "Steps":[
    {
      "Name":"script",
      "Script":["step-runner ci"],
      "Timeout":3600,
      "When":"on_success"
    }
  ]
}`,
		},
		"steps are requested via shim, image unmodified": {
			json: `
{
  "Run":"[{\"Name:\":\"hello\",\"Script\":\"echo hello world\"}]",
  "Image":{"Name":"registry.gitlab.com/project/image:v1"}
}`,
			wantJSON: `
{
  "Run":"[{\"Name:\":\"hello\",\"Script\":\"echo hello world\"}]",
  "Variables":[
    {
      "Key":"STEPS",
      "Value":"[{\"Name:\":\"hello\",\"Script\":\"echo hello world\"}]",
      "Raw":true
    }
  ],
  "Steps":[
    {
      "Name":"script",
      "Script":["step-runner ci"],
      "Timeout":3600,
      "When":"on_success"
    }
  ],
  "Image":{"Name":"registry.gitlab.com/project/image:v1"}
}`,
		},
		"steps and script are requested": {
			json: `
{
  "Run":"[{\"Name:\":\"hello\",\"Script\":\"echo hello world\"}]",
  "Steps":[
    {
      "Name":"script",
      "Script":["echo hello job"],
      "Timeout":3600,
      "When":"on_success"
    }
  ]
}`,
			wantErr: true,
		},
		"steps requested and STEP variable used": {
			json: `
{
  "Run":"[{\"Name:\":\"hello\",\"Script\":\"echo hello world\"}]",
  "Variables":[
    {
      "Key":"STEPS",
      "Value":"not steps",
      "Raw":true
    }
  ]
}`,
			wantErr: true,
		},

		"steps request via native exec, executor supports native exec": {
			execNativeSteps: true,
			json: `
{
  "Run":"[{\"Name:\":\"hello\",\"Script\":\"echo hello world\"}]",
  "Variables":[
    {
      "Key":"FF_USE_NATIVE_STEPS",
      "Value":"true"
    }
  ]
}`,
			wantJSON: `
{
  "Run":"[{\"Name:\":\"hello\",\"Script\":\"echo hello world\"}]",
  "Variables":[
    {
      "Key":"FF_USE_NATIVE_STEPS",
      "Value":"true"
    }
  ],
  "Steps":[
    {
      "Name":"run"
    }
  ]
}`,
		},
		"steps request via native exec, executor does not support native exec": {
			execNativeSteps: false,
			json: `
{
  "Run":"[{\"Name:\":\"hello\",\"Script\":\"echo hello world\"}]",
  "Variables":[
    {
      "Key":"FF_USE_NATIVE_STEPS",
      "Value":"true"
    }
  ]
}`,
			wantJSON: `
{
  "Run":"[{\"Name:\":\"hello\",\"Script\":\"echo hello world\"}]",
  "Variables":[
    {
      "Key":"FF_USE_NATIVE_STEPS",
      "Value":"true"
    },
    {
      "Key":"STEPS",
      "Value":"[{\"Name:\":\"hello\",\"Script\":\"echo hello world\"}]",
      "Raw":true
    }
  ],
  "Steps":[
    {
      "Name":"script",
      "Script":["step-runner ci"],
      "Timeout":3600,
      "When":"on_success"
    }
  ]
}`,
		},
		"steps are requested via shim, executor supports native exec": {
			execNativeSteps: true,
			json: `
{
  "Run":"[{\"Name:\":\"hello\",\"Script\":\"echo hello world\"}]"
}`,
			wantJSON: `
{
  "Run":"[{\"Name:\":\"hello\",\"Script\":\"echo hello world\"}]",
  "Steps":[
    {
      "Name":"run"
    }
  ]
}`,
		},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			jobResponse := &spec.Job{}
			require.NoError(t, json.Unmarshal([]byte(tt.json), &jobResponse))

			err := jobResponse.ValidateStepsJobRequest(tt.execNativeSteps)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			want := &spec.Job{}
			require.NoError(t, json.Unmarshal([]byte(tt.wantJSON), &want))
			require.Equal(t, want, jobResponse)
		})
	}
}

func TestFeaturesInfo_JSONMarshaling(t *testing.T) {
	tests := []struct {
		name     string
		features FeaturesInfo
		expected string
	}{
		{
			name:     "all default (disabled)",
			features: FeaturesInfo{},
			expected: `{"variables":false,"image":false,"services":false,"artifacts":false,"cache":false,"fallback_cache_keys":false,"shared":false,"upload_multiple_artifacts":false,"upload_raw_artifacts":false,"session":false,"terminal":false,"refspecs":false,"masking":false,"proxy":false,"raw_variables":false,"artifacts_exclude":false,"multi_build_steps":false,"trace_reset":false,"trace_checksum":false,"trace_size":false,"vault_secrets":false,"cancelable":false,"return_exit_code":false,"service_variables":false,"service_multiple_aliases":false,"image_executor_opts":false,"service_executor_opts":false,"cancel_gracefully":false,"native_steps_integration":false,"two_phase_job_commit":false,"job_inputs":false}`,
		},
		{
			name: "some enabled",
			features: FeaturesInfo{
				Variables:         true,
				Image:             true,
				TwoPhaseJobCommit: true,
				JobInputs:         true,
			},
			expected: `{"variables":true,"image":true,"services":false,"artifacts":false,"cache":false,"fallback_cache_keys":false,"shared":false,"upload_multiple_artifacts":false,"upload_raw_artifacts":false,"session":false,"terminal":false,"refspecs":false,"masking":false,"proxy":false,"raw_variables":false,"artifacts_exclude":false,"multi_build_steps":false,"trace_reset":false,"trace_checksum":false,"trace_size":false,"vault_secrets":false,"cancelable":false,"return_exit_code":false,"service_variables":false,"service_multiple_aliases":false,"image_executor_opts":false,"service_executor_opts":false,"cancel_gracefully":false,"native_steps_integration":false,"two_phase_job_commit":true,"job_inputs":true}`,
		},
		{
			name: "all enabled",
			features: FeaturesInfo{
				Variables:               true,
				Image:                   true,
				Services:                true,
				Artifacts:               true,
				Cache:                   true,
				FallbackCacheKeys:       true,
				Shared:                  true,
				UploadMultipleArtifacts: true,
				UploadRawArtifacts:      true,
				Session:                 true,
				Terminal:                true,
				Refspecs:                true,
				Masking:                 true,
				Proxy:                   true,
				RawVariables:            true,
				ArtifactsExclude:        true,
				MultiBuildSteps:         true,
				TraceReset:              true,
				TraceChecksum:           true,
				TraceSize:               true,
				VaultSecrets:            true,
				Cancelable:              true,
				ReturnExitCode:          true,
				ServiceVariables:        true,
				ServiceMultipleAliases:  true,
				ImageExecutorOpts:       true,
				ServiceExecutorOpts:     true,
				CancelGracefully:        true,
				NativeStepsIntegration:  true,
				TwoPhaseJobCommit:       true,
				JobInputs:               true,
			},
			expected: `{"variables":true,"image":true,"services":true,"artifacts":true,"cache":true,"fallback_cache_keys":true,"shared":true,"upload_multiple_artifacts":true,"upload_raw_artifacts":true,"session":true,"terminal":true,"refspecs":true,"masking":true,"proxy":true,"raw_variables":true,"artifacts_exclude":true,"multi_build_steps":true,"trace_reset":true,"trace_checksum":true,"trace_size":true,"vault_secrets":true,"cancelable":true,"return_exit_code":true,"service_variables":true,"service_multiple_aliases":true,"image_executor_opts":true,"service_executor_opts":true,"cancel_gracefully":true,"native_steps_integration":true,"two_phase_job_commit":true,"job_inputs":true}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			jsonData, err := json.Marshal(tt.features)
			require.NoError(t, err)
			assert.JSONEq(t, tt.expected, string(jsonData))

			// Test unmarshaling
			var features FeaturesInfo
			err = json.Unmarshal(jsonData, &features)
			require.NoError(t, err)
			assert.Equal(t, tt.features, features)
		})
	}
}
