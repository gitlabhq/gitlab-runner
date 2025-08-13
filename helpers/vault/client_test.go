//go:build !integration

package vault

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/openbao/openbao/api/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockHealthCheckServer(tb testing.TB, initialized, sealed bool, healthErr error) func(w http.ResponseWriter, r *http.Request) {
	tb.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/sys/health":
			if healthErr != nil {
				w.WriteHeader(http.StatusInternalServerError)
				require.NoError(tb, json.NewEncoder(w).Encode(map[string]interface{}{
					"error": healthErr.Error(),
				}))
				return
			}
			require.NoError(tb, json.NewEncoder(w).Encode(api.HealthResponse{
				Initialized: initialized,
				Sealed:      sealed,
			}))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func mockOperationServer(tb testing.TB, operationPath string, operationHandler func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
	tb.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/sys/health":
			// Health check always succeeds
			require.NoError(tb, json.NewEncoder(w).Encode(api.HealthResponse{
				Initialized: true,
				Sealed:      false,
			}))
		case "/v1/" + operationPath:
			operationHandler(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

func mockServerWithNonAPIError(tb testing.TB) func(w http.ResponseWriter, r *http.Request) {
	tb.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			require.NoError(tb, json.NewEncoder(w).Encode(api.HealthResponse{
				Initialized: true,
				Sealed:      false,
			}))
			return
		}

		// For subsequent calls, close the connection to simulate network error
		hj, ok := w.(http.Hijacker)
		if ok {
			conn, _, _ := hj.Hijack()
			conn.Close()
		}
	}
}

func mockServerWithAPIError(tb testing.TB) func(w http.ResponseWriter, r *http.Request) {
	tb.Helper()
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			require.NoError(tb, json.NewEncoder(w).Encode(api.HealthResponse{
				Initialized: true,
				Sealed:      false,
			}))
			return
		}

		w.WriteHeader(http.StatusForbidden)
		require.NoError(tb, json.NewEncoder(w).Encode(map[string]interface{}{
			"errors": []string{"permission denied"},
		}))
	}
}

func TestNewClient(t *testing.T) {
	namespace := "test_namespace"

	tests := map[string]struct {
		mockHandler   func(w http.ResponseWriter, r *http.Request)
		clientURL     string
		expectedError error
	}{
		"vault health check error": {
			mockHandler:   mockHealthCheckServer(t, false, false, errors.New("health check failed")),
			expectedError: errors.New("checking Vault server health"),
		},
		"vault server not initialized": {
			mockHandler:   mockHealthCheckServer(t, false, false, nil),
			expectedError: errors.New("not initialized or sealed Vault server"),
		},
		"vault server sealed": {
			mockHandler:   mockHealthCheckServer(t, true, true, nil),
			expectedError: errors.New("not initialized or sealed Vault server"),
		},
		"vault client creation error": {
			mockHandler:   mockHealthCheckServer(t, true, false, nil),
			clientURL:     "://invalid-url",
			expectedError: errors.New("creating new Vault client"),
		},
		"vault client initialized": {
			mockHandler: mockHealthCheckServer(t, true, false, nil),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.mockHandler))
			defer server.Close()

			url := tt.clientURL
			if url == "" {
				url = server.URL
			}
			c, err := NewClient(url, namespace)

			if tt.expectedError != nil {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError.Error())
				assert.Nil(t, c)
				return
			}

			assert.NoError(t, err)
			require.NotNil(t, c)

			// Verify no inline auth headers are set when not configured
			dc, ok := c.(*defaultClient)
			require.True(t, ok)
			headers := dc.internal.Headers()
			assert.Empty(t, headers.Get("X-Vault-Inline-Auth-Path"))
			assert.Empty(t, headers.Get("X-Vault-Inline-Auth-Parameter-jwt"))
			assert.Empty(t, headers.Get("X-Vault-Inline-Auth-Parameter-role"))
		})
	}
}

func TestNewClient_WithInlineAuth(t *testing.T) {
	type testCase struct {
		mockHandler   func(w http.ResponseWriter, r *http.Request)
		namespace     string
		inlineAuth    *InlineAuth
		expectedError string
	}

	tests := map[string]testCase{
		"valid configuration": {
			mockHandler: mockHealthCheckServer(t, true, false, nil),
			namespace:   "test-namespace",
			inlineAuth: &InlineAuth{
				Path: "auth/jwt/login",
				JWT:  "test-jwt",
				Role: "test-role",
			},
		},
		"nil inline auth passed to WithInlineAuth": {
			mockHandler:   mockHealthCheckServer(t, true, false, nil),
			namespace:     "test-namespace",
			inlineAuth:    nil,
			expectedError: "inline auth is required",
		},
		"missing auth path": {
			mockHandler: mockHealthCheckServer(t, true, false, nil),
			namespace:   "test-namespace",
			inlineAuth: &InlineAuth{
				JWT:  "test-jwt",
				Role: "test-role",
			},
			expectedError: "inline auth path is required",
		},
		"missing JWT": {
			mockHandler: mockHealthCheckServer(t, true, false, nil),
			namespace:   "test-namespace",
			inlineAuth: &InlineAuth{
				Path: "auth/jwt/login",
				Role: "test-role",
			},
			expectedError: "inline auth JWT is required",
		},
		"missing role": {
			mockHandler: mockHealthCheckServer(t, true, false, nil),
			namespace:   "test-namespace",
			inlineAuth: &InlineAuth{
				Path: "auth/jwt/login",
				JWT:  "test-jwt",
			},
			expectedError: "inline auth role is required",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.mockHandler))
			defer server.Close()

			c, err := NewClient(server.URL, tt.namespace, WithInlineAuth(tt.inlineAuth))

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, c)
				return
			}

			assert.NoError(t, err)
			require.NotNil(t, c)

			dc, ok := c.(*defaultClient)
			require.True(t, ok)
			headers := dc.internal.Headers()

			// Verify the path is set correctly
			pathHeader := headers[api.InlineAuthPathHeaderName]
			assert.Equal(t, tt.inlineAuth.Path, pathHeader[0])

			// Decode and verify JWT parameter
			jwtHeader := headers[fmt.Sprintf("%s%s", api.InlineAuthParameterHeaderPrefix, "jwt")]
			assert.NotEmpty(t, jwtHeader)

			jwtDecoded, err := base64.RawURLEncoding.DecodeString(jwtHeader[0])
			require.NoError(t, err)

			var jwtData map[string]interface{}
			err = json.Unmarshal(jwtDecoded, &jwtData)
			require.NoError(t, err)
			assert.Equal(t, "jwt", jwtData["key"])
			assert.Equal(t, tt.inlineAuth.JWT, jwtData["value"])

			// Decode and verify Role parameter
			roleHeader := headers[fmt.Sprintf("%s%s", api.InlineAuthParameterHeaderPrefix, "role")]
			assert.NotEmpty(t, roleHeader)

			roleDecoded, err := base64.RawURLEncoding.DecodeString(roleHeader[0])
			require.NoError(t, err)

			var roleData map[string]interface{}
			err = json.Unmarshal(roleDecoded, &roleData)
			require.NoError(t, err)
			assert.Equal(t, "role", roleData["key"])
			assert.Equal(t, tt.inlineAuth.Role, roleData["value"])
		})
	}
}

func TestDefaultClient_Authenticate(t *testing.T) {
	tests := map[string]struct {
		setupAuthMock func(t *testing.T, c *defaultClient) *MockAuthMethod
		expectedError string
	}{
		"authentication error": {
			setupAuthMock: func(t *testing.T, c *defaultClient) *MockAuthMethod {
				mockAuthMethod := NewMockAuthMethod(t)
				mockAuthMethod.On("Authenticate", c).Return(assert.AnError).Once()
				return mockAuthMethod
			},
			expectedError: "authenticating Vault client",
		},
		"authentication succeeded": {
			setupAuthMock: func(t *testing.T, c *defaultClient) *MockAuthMethod {
				mockAuthMethod := NewMockAuthMethod(t)
				mockAuthMethod.On("Authenticate", c).Return(nil).Once()
				mockAuthMethod.On("Token").Return("test-token").Once()
				return mockAuthMethod
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			handler := mockHealthCheckServer(t, true, false, nil)
			server := httptest.NewServer(http.HandlerFunc(handler))
			defer server.Close()

			client, err := NewClient(server.URL, "namespace")
			require.NoError(t, err)

			mockAuthMethod := tt.setupAuthMock(t, client.(*defaultClient))

			err = client.Authenticate(mockAuthMethod)
			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				return
			}

			assert.NoError(t, err)

			// Verify the token was set on the internal client
			dc, ok := client.(*defaultClient)
			require.True(t, ok)
			assert.Equal(t, "test-token", dc.internal.Token())
		})
	}
}

func TestDefaultClient_Write(t *testing.T) {
	secretData := map[string]interface{}{"key1": "value1"}
	data := map[string]interface{}{"key": "value"}

	tests := map[string]struct {
		mockHandler func(w http.ResponseWriter, r *http.Request)
		verifyError func(t *testing.T, err error)
	}{
		"non-api error (connection refused)": {
			mockHandler: mockServerWithNonAPIError(t),
			verifyError: func(t *testing.T, err error) {
				var apiErr *unwrappedAPIResponseError
				assert.False(t, errors.As(err, &apiErr))
			},
		},
		"api error": {
			mockHandler: mockServerWithAPIError(t),
			verifyError: func(t *testing.T, err error) {
				var apiErr *unwrappedAPIResponseError
				assert.ErrorAs(t, err, &apiErr)
			},
		},
		"successful writing": {
			mockHandler: mockOperationServer(t, "path/to/write", func(w http.ResponseWriter, r *http.Request) {
				require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
					"data": secretData,
				}))
			}),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.mockHandler))
			defer server.Close()

			client, err := NewClient(server.URL, "namespace")
			require.NoError(t, err)

			res, err := client.Write("path/to/write", data)

			if tt.verifyError != nil {
				assert.Error(t, err)
				tt.verifyError(t, err)
				return
			}

			assert.NoError(t, err)
			require.NotNil(t, res)
			assert.Equal(t, secretData, res.Data())
		})
	}
}

func TestDefaultClient_Read(t *testing.T) {
	secretData := map[string]interface{}{"key1": "value1"}

	tests := map[string]struct {
		mockHandler func(w http.ResponseWriter, r *http.Request)
		verifyError func(t *testing.T, err error)
	}{
		"non-api error (connection error)": {
			mockHandler: mockServerWithNonAPIError(t),
			verifyError: func(t *testing.T, err error) {
				var apiErr *unwrappedAPIResponseError
				assert.False(t, errors.As(err, &apiErr))
			},
		},
		"api error": {
			mockHandler: mockServerWithAPIError(t),
			verifyError: func(t *testing.T, err error) {
				var apiErr *unwrappedAPIResponseError
				assert.ErrorAs(t, err, &apiErr)
			},
		},
		"successful reading": {
			mockHandler: mockOperationServer(t, "path/to/read", func(w http.ResponseWriter, r *http.Request) {
				require.NoError(t, json.NewEncoder(w).Encode(map[string]interface{}{
					"data": secretData,
				}))
			}),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.mockHandler))
			defer server.Close()

			client, err := NewClient(server.URL, "namespace")
			require.NoError(t, err)

			res, err := client.Read("path/to/read")

			if tt.verifyError != nil {
				assert.Error(t, err)
				tt.verifyError(t, err)
				return
			}

			assert.NoError(t, err)
			require.NotNil(t, res)
			assert.Equal(t, secretData, res.Data())
		})
	}
}

func TestDefaultClient_Delete(t *testing.T) {
	tests := map[string]struct {
		mockHandler func(w http.ResponseWriter, r *http.Request)
		verifyError func(t *testing.T, err error)
	}{
		"non-api error (connection error)": {
			mockHandler: mockServerWithNonAPIError(t),
			verifyError: func(t *testing.T, err error) {
				var apiErr *unwrappedAPIResponseError
				assert.False(t, errors.As(err, &apiErr))
			},
		},
		"api error": {
			mockHandler: mockServerWithAPIError(t),
			verifyError: func(t *testing.T, err error) {
				var apiErr *unwrappedAPIResponseError
				assert.ErrorAs(t, err, &apiErr)
			},
		},
		"successful deleting": {
			mockHandler: mockOperationServer(t, "path/to/delete", func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			}),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(tt.mockHandler))
			defer server.Close()

			client, err := NewClient(server.URL, "namespace")
			require.NoError(t, err)

			err = client.Delete("path/to/delete")

			if tt.verifyError != nil {
				assert.Error(t, err)
				tt.verifyError(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
