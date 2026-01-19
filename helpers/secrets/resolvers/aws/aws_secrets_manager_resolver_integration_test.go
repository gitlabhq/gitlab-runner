//go:build integration

package aws

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	sm "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

type mockResponse struct {
	statusCode int
	body       string
	assertions func(*testing.T, *http.Request)
}

type testCase struct {
	name          string
	secret        spec.Secret
	response      mockResponse
	expectedVal   string
	expectError   bool
	errorContains string
	envOverrides  map[string]string
	customFactory func(*testing.T, string) AWSSecretsManager
}

// Common test data
var (
	defaultEnv = map[string]string{
		"AWS_REGION":                "us-west-2",
		"AWS_ACCESS_KEY_ID":         "test",
		"AWS_SECRET_ACCESS_KEY":     "test",
		"AWS_SESSION_TOKEN":         "test",
		"AWS_EC2_METADATA_DISABLED": "true",
	}

	basicSecret = spec.Secret{
		AWSSecretsManager: &spec.AWSSecret{
			SecretId: "test-secret",
			Field:    "Date",
			Region:   "us-west-2",
		},
	}

	jsonResponse = mockResponse{
		statusCode: 200,
		body:       `{"SecretString":"{\"Date\":\"2020-08-24\"}"}`,
	}

	missingFieldResponse = mockResponse{
		statusCode: 200,
		body:       `{"SecretString":"{\"Other\":\"value\"}"}`,
	}
)

func TestAWSSecretsManagerIntegration(t *testing.T) {
	tests := map[string]testCase{
		"basic secret retrieval": {
			secret:      basicSecret,
			response:    jsonResponse,
			expectedVal: "2020-08-24",
		},
		"field not found": {
			secret:        basicSecret,
			response:      missingFieldResponse,
			expectError:   true,
			errorContains: "key 'Date' not found",
		},
		"version stage AWSCURRENT": {
			secret: spec.Secret{
				AWSSecretsManager: &spec.AWSSecret{
					SecretId:     "prod-app-secrets/database",
					Field:        "password",
					Region:       "us-east-1",
					VersionStage: "AWSCURRENT",
				},
			},
			response: mockResponse{
				statusCode: 200,
				body:       `{"SecretString":"{\"password\":\"s3cr3t\"}"}`,
				assertions: func(t *testing.T, r *http.Request) {
					body, _ := io.ReadAll(r.Body)
					assert.Contains(t, string(body), `"VersionStage":"AWSCURRENT"`)
				},
			},
			expectedVal: "s3cr3t",
			envOverrides: map[string]string{
				"AWS_REGION": "us-east-1",
			},
		},
		"version ID": {
			secret: spec.Secret{
				AWSSecretsManager: &spec.AWSSecret{
					SecretId:  "prod-app-secrets/database",
					Field:     "password",
					Region:    "us-east-1",
					VersionId: "01234567-89ab-cdef-0123-456789abcdef",
				},
			},
			response: mockResponse{
				statusCode: 200,
				body:       `{"SecretString":"{\"password\":\"old\"}"}`,
				assertions: func(t *testing.T, r *http.Request) {
					body, _ := io.ReadAll(r.Body)
					assert.Contains(t, string(body), `"VersionId":"01234567-89ab-cdef-0123-456789abcdef"`)
				},
			},
			expectedVal: "old",
			envOverrides: map[string]string{
				"AWS_REGION": "us-east-1",
			},
		},
		"version ID and stage conflict": {
			secret: spec.Secret{
				AWSSecretsManager: &spec.AWSSecret{
					SecretId:     "prod-app-secrets/database",
					Field:        "password",
					Region:       "us-east-1",
					VersionId:    "01234567-89ab-cdef-0123-456789abcdef",
					VersionStage: "AWSCURRENT",
				},
			},
			response: mockResponse{
				statusCode: 400,
				body:       `{"__type":"ValidationException","message":"Cannot specify both VersionId and VersionStage."}`,
				assertions: func(t *testing.T, r *http.Request) {
					body, _ := io.ReadAll(r.Body)
					assert.Contains(t, string(body), `"VersionId":`)
					assert.Contains(t, string(body), `"VersionStage":`)
				},
			},
			expectError:   true,
			errorContains: "Cannot specify both VersionId and VersionStage",
			envOverrides: map[string]string{
				"AWS_REGION": "us-east-1",
			},
		},
		"cross-account ARN": {
			secret: spec.Secret{
				AWSSecretsManager: &spec.AWSSecret{
					SecretId: "arn:aws:secretsmanager:us-east-1:987654321098:secret:shared-api-keys-AbCdEf",
					Field:    "production_key",
					Region:   "us-east-1",
				},
			},
			response: mockResponse{
				statusCode: 200,
				body:       `{"SecretString":"{\"production_key\":\"k123\"}"}`,
				assertions: func(t *testing.T, r *http.Request) {
					body, _ := io.ReadAll(r.Body)
					assert.Contains(t, string(body), "arn:aws:secretsmanager:us-east-1:987654321098:secret:shared-api-keys-AbCdEf")
				},
			},
			expectedVal: "k123",
			envOverrides: map[string]string{
				"AWS_REGION": "us-east-1",
			},
		},
		"per-secret region override": {
			secret: spec.Secret{
				AWSSecretsManager: &spec.AWSSecret{
					SecretId: "eu-app-secrets/database",
					Region:   "eu-west-1", // override
				},
			},
			response: mockResponse{
				statusCode: 200,
				body:       `{"SecretString":"\"ok\""}`,
				assertions: func(t *testing.T, r *http.Request) {
					region := extractRegionFromAuth(r.Header.Get("Authorization"))
					assert.Equal(t, "eu-west-1", region, "expected SigV4 region scope to use per-secret region")
				},
			},
			expectedVal: "\"ok\"",
			envOverrides: map[string]string{
				"AWS_REGION": "us-east-1", // global differs from per-secret
			},
		},
		"secret binary base64": {
			secret: spec.Secret{
				AWSSecretsManager: &spec.AWSSecret{
					SecretId: "bin",
					Region:   "us-west-2",
				},
			},
			response: mockResponse{
				statusCode: 200,
				body:       `{"SecretBinary":"AP8QIH8="}`,
			},
			expectedVal: "AP8QIH8=",
		},
		"field with number": {
			secret: spec.Secret{
				AWSSecretsManager: &spec.AWSSecret{
					SecretId: "cfg",
					Field:    "retries",
					Region:   "us-west-2",
				},
			},
			response: mockResponse{
				statusCode: 200,
				body:       `{"SecretString":"{\"retries\":3}"}`,
			},
			expectedVal: "3",
		},
		"retry on 5xx": {
			secret: spec.Secret{
				AWSSecretsManager: &spec.AWSSecret{
					SecretId: "r",
					Region:   "us-west-2",
				},
			},
			response: mockResponse{
				statusCode: 200,
				body:       `{"SecretString":"\"ok\""}`,
				assertions: func(t *testing.T, r *http.Request) {
					// This will be called for each request
				},
			},
			expectedVal: "\"ok\"",
			envOverrides: map[string]string{
				"AWS_MAX_ATTEMPTS": "2", // 1 initial + 1 retry
			},
		},
		"OIDC web identity role": {
			secret: spec.Secret{
				AWSSecretsManager: &spec.AWSSecret{
					SecretId: "app-secrets/database",
					Field:    "password",
					Region:   "us-east-1",
					Server: spec.AWSServer{
						Region:          "us-east-1",
						JWT:             "dummy-oidc-id-token",
						RoleArn:         "arn:aws:iam::123456789012:role/gitlab-secrets-role",
						RoleSessionName: "12345-67890-gitlab.example.com",
					},
				},
			},
			response: mockResponse{
				statusCode: 200,
				body:       `{"SecretString":"{\"password\":\"s3cr3t\"}"}`,
			},
			expectedVal: "s3cr3t",
			customFactory: func(t *testing.T, serverURL string) AWSSecretsManager {
				return createOIDCFactory(t, serverURL)
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			srv := createMockServer(t, tc)
			defer srv.Close()

			setupEnvironment(t, tc, srv.URL)

			if tc.customFactory != nil {
				setupCustomFactory(t, tc.customFactory, srv.URL)
			}

			val, err := newResolver(tc.secret).Resolve()

			if tc.expectError {
				require.Error(t, err)
				assert.Empty(t, val)
				if tc.errorContains != "" {
					assert.Contains(t, err.Error(), tc.errorContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedVal, val)
			}
		})
	}
}

// Helper functions for test setup

func createMockServer(t *testing.T, tc testCase) *httptest.Server {
	if tc.name == "retry on 5xx" {
		return createRetryMockServer(t, tc)
	}

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if tc.response.assertions != nil {
			// Read body for assertions but reset it
			body, _ := io.ReadAll(r.Body)
			r.Body.Close()
			r.Body = io.NopCloser(strings.NewReader(string(body)))
			tc.response.assertions(t, r)
		} else {
			io.ReadAll(r.Body)
			r.Body.Close()
		}

		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		if tc.response.statusCode != 200 {
			w.WriteHeader(tc.response.statusCode)
		}
		w.Write([]byte(tc.response.body))
	}))
}

func createRetryMockServer(t *testing.T, tc testCase) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-amz-json-1.1")
		w.Write([]byte(tc.response.body))
	}))
}

func setupEnvironment(t *testing.T, tc testCase, serverURL string) {
	// Set default environment
	env := make(map[string]string)
	for k, v := range defaultEnv {
		env[k] = v
	}

	// Apply overrides
	for k, v := range tc.envOverrides {
		env[k] = v
	}

	// Set endpoint URL
	env["AWS_ENDPOINT_URL_SECRETS_MANAGER"] = serverURL

	// Apply all environment variables
	for k, v := range env {
		t.Setenv(k, v)
	}
}

func setupCustomFactory(t *testing.T, factoryFunc func(*testing.T, string) AWSSecretsManager, serverURL string) {
	oldFactory := newAWSSecretsManagerService
	t.Cleanup(func() { newAWSSecretsManagerService = oldFactory })

	newAWSSecretsManagerService = func(ctx context.Context, region string, webIdentityProvider *stscreds.WebIdentityRoleProvider) (AWSSecretsManager, error) {
		return factoryFunc(t, serverURL), nil
	}
}

func createOIDCFactory(t *testing.T, serverURL string) AWSSecretsManager {
	// Real client that points to our mock server
	resolver := aws.EndpointResolverWithOptionsFunc(func(service, _ string, _ ...interface{}) (aws.Endpoint, error) {
		if service == sm.ServiceID {
			return aws.Endpoint{URL: serverURL, PartitionID: "aws", SigningRegion: "us-east-1"}, nil
		}
		return aws.Endpoint{}, &aws.EndpointNotFoundError{}
	})

	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion("us-east-1"),
		config.WithEndpointResolverWithOptions(resolver),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("AKID", "SECRET", "TOKEN")),
	)
	require.NoError(t, err)

	return &realClient{c: sm.NewFromConfig(cfg)}
}

func extractRegionFromAuth(auth string) string {
	i := strings.Index(auth, "Credential=")
	if i < 0 {
		return ""
	}
	scope := auth[i+len("Credential="):]
	if j := strings.Index(scope, ","); j >= 0 {
		scope = scope[:j]
	}
	parts := strings.Split(scope, "/")
	if len(parts) >= 4 {
		return parts[2] // date, region, service
	}
	return ""
}

// realClient implementation remains the same
type realClient struct{ c *sm.Client }

func (r *realClient) GetSecretString(
	ctx context.Context,
	secretId string,
	versionId *string,
	versionStage *string,
) (string, error) {
	out, err := r.c.GetSecretValue(ctx, &sm.GetSecretValueInput{
		SecretId:     aws.String(secretId),
		VersionId:    versionId,
		VersionStage: versionStage,
	})
	if err != nil {
		return "", err
	}
	if out.SecretString != nil {
		return *out.SecretString, nil
	}
	if out.SecretBinary != nil {
		return string(out.SecretBinary), nil
	}
	return "", nil
}
