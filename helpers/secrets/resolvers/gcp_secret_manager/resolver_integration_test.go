//go:build integration

package gcp_secret_manager

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type realClient struct {
	baseURL string // e.g. http://127.0.0.1:XXXXX
}

func newRealClientFromEnv() *realClient {
	ep := os.Getenv("GCP_SECRET_MANAGER_ENDPOINT")
	if ep == "" {
		panic("GCP_SECRET_MANAGER_ENDPOINT must be set for integration tests")
	}
	return &realClient{baseURL: strings.TrimRight(ep, "/")}
}

// Expects s to carry at least Secret name and optional Version.
// Project number comes from env (defaults), mirroring the resolver behavior.
func (c *realClient) GetSecret(ctx context.Context, s *common.GCPSecretManagerSecret) (string, error) {
	if s == nil {
		return "", errors.New("nil secret")
	}

	project := os.Getenv("GCP_PROJECT_NUMBER")
	if project == "" {
		return "", errors.New("GCP_PROJECT_NUMBER not set")
	}

	secretName := s.Name
	if secretName == "" {
		return "", errors.New("secret name is empty")
	}
	version := s.Version
	if version == "" {
		version = "latest"
	}

	url := fmt.Sprintf("%s/v1/projects/%s/secrets/%s/versions/%s:access", c.baseURL, project, secretName, version)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, http.NoBody)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	type payloadT struct {
		Payload struct {
			Data string `json:"data"`
		} `json:"payload"`
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		if e.Error != "" {
			return "", errors.New(e.Error)
		}
		return "", fmt.Errorf("http %d", resp.StatusCode)
	}

	var out payloadT
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}

	// GCP returns base64-encoded payload bytes.
	if out.Payload.Data == "" {
		return "", nil
	}
	decoded, err := base64.StdEncoding.DecodeString(out.Payload.Data)
	if err != nil {
		return "", err
	}
	return string(decoded), nil
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func setEnvMap(t *testing.T, kv map[string]string) {
	t.Helper()
	for k, v := range kv {
		t.Setenv(k, v)
	}
}

func TestGCPSecretManagerResolver_Integration(t *testing.T) {
	defaultEnv := map[string]string{
		"GCP_PROJECT_NUMBER":                           "1234567890",
		"GCP_WORKLOAD_IDENTITY_FEDERATION_POOL_ID":     "gitlab-pool",
		"GCP_WORKLOAD_IDENTITY_FEDERATION_PROVIDER_ID": "gitlab-provider",
	}

	type serverCase struct {
		secretPath    string
		status        int
		body          string
		assertRequest func(*testing.T, *http.Request)
	}

	// Mock GCP SM server
	newServer := func(t *testing.T, sc serverCase) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			assert.Equal(t, sc.secretPath, r.URL.Path)
			if sc.assertRequest != nil {
				sc.assertRequest(t, r)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(sc.status)
			_, _ = w.Write([]byte(sc.body))
		}))
	}

	smAccess := func(project, name, version string) string {
		if version == "" {
			version = "latest"
		}
		return fmt.Sprintf("/v1/projects/%s/secrets/%s/versions/%s:access", project, name, version)
	}

	tests := map[string]struct {
		secret        common.Secret
		setupEnv      map[string]string
		server        serverCase
		expectedValue string
		expectErrSub  string
	}{
		"unsupported when nil": {
			secret:       common.Secret{}, // GCPSecretManager: nil
			expectErrSub: "unsupported",
		},
		"basic success (latest)": {
			secret: common.Secret{
				GCPSecretManager: &common.GCPSecretManagerSecret{
					Name: "api-key",
				},
			},
			setupEnv: defaultEnv,
			server: serverCase{
				secretPath: smAccess("1234567890", "api-key", "latest"),
				status:     200,
				body:       `{"payload":{"data":"` + b64("secret-value") + `"}}`,
			},
			expectedValue: "secret-value",
		},
		"explicit version": {
			secret: common.Secret{
				GCPSecretManager: &common.GCPSecretManagerSecret{
					Name:    "db-pass",
					Version: "5",
				},
			},
			setupEnv: defaultEnv,
			server: serverCase{
				secretPath: smAccess("1234567890", "db-pass", "5"),
				status:     200,
				body:       `{"payload":{"data":"` + b64("v5") + `"}}`,
			},
			expectedValue: "v5",
		},
		"permission denied bubble up": {
			secret: common.Secret{
				GCPSecretManager: &common.GCPSecretManagerSecret{
					Name: "locked",
				},
			},
			setupEnv: defaultEnv,
			server: serverCase{
				secretPath: smAccess("1234567890", "locked", "latest"),
				status:     403,
				body:       `{"error":"Permission 'secretmanager.versions.access' denied"}`,
			},
			expectErrSub: "Permission",
		},
		"empty string allowed": {
			secret: common.Secret{
				GCPSecretManager: &common.GCPSecretManagerSecret{
					Name: "empty",
				},
			},
			setupEnv: defaultEnv,
			server: serverCase{
				secretPath: smAccess("1234567890", "empty", "latest"),
				status:     200,
				body:       `{"payload":{"data":"` + b64("") + `"}}`,
			},
			expectedValue: "",
		},
		"env defaults missing -> resolver should error before call": {
			secret: common.Secret{
				GCPSecretManager: &common.GCPSecretManagerSecret{
					Name: "x",
				},
			},
			// no env
			// We still spin a server to avoid nil endpoint, but it shouldn't be hit if resolver validates.
			server: serverCase{
				secretPath: smAccess("1234567890", "x", "latest"),
				status:     200,
				body:       `{"payload":{"data":"` + b64("ok") + `"}}`,
			},
			expectErrSub: "GCP_PROJECT_NUMBER",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			srv := newServer(t, tc.server)
			defer srv.Close()

			// Environment
			if tc.setupEnv != nil {
				setEnvMap(t, tc.setupEnv)
			}
			// Point client to mock endpoint
			t.Setenv("GCP_SECRET_MANAGER_ENDPOINT", srv.URL)

			// Wire real client into resolver
			r := &resolver{
				secret: tc.secret,
				client: newRealClientFromEnv(),
			}

			assert.Equal(t, tc.secret.GCPSecretManager != nil, r.IsSupported())

			val, err := r.Resolve()
			if tc.expectErrSub != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectErrSub)
				assert.Empty(t, val)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tc.expectedValue, val)
			}
		})
	}
}
