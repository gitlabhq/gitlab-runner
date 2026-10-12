//go:build !integration

package service

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

// fakeAccessToken is the raw, unencoded JSON body Conjur returns from
// /authenticate. The client must base64-encode this itself before use in the
// Authorization header - Conjur base64-decodes that header value server-side
// and expects to find this JSON underneath.
const fakeAccessToken = `{"protected":"p","payload":"pl","signature":"s"}`

func fakeAccessTokenHeader() string {
	return fmt.Sprintf(`Token token="%s"`, base64.StdEncoding.EncodeToString([]byte(fakeAccessToken)))
}

// newTestClient trusts server's certificate, so appliance_url can point at an
// httptest.NewTLSServer and still pass the https requirement.
func newTestClient(server *httptest.Server) *Client {
	pool := x509.NewCertPool()
	pool.AddCert(server.Certificate())

	return NewClient(WithRootCAs(pool))
}

func newTestSecret(applianceURL string) *spec.ConjurSecret {
	return &spec.ConjurSecret{
		Path: "app/prod/db-password",
		Server: spec.ConjurServer{
			ApplianceURL:      applianceURL,
			Account:           "myAccount",
			AuthnJWTServiceID: "gitlab",
			JWT:               "the-ci-jwt",
		},
	}
}

func TestClient_GetSecret_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/authn-jwt/gitlab/myAccount/authenticate", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

		require.NoError(t, r.ParseForm())
		assert.Equal(t, "the-ci-jwt", r.FormValue("jwt"))

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fakeAccessToken))
	})
	mux.HandleFunc("/secrets/myAccount/variable/app%2Fprod%2Fdb-password", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, fakeAccessTokenHeader(), r.Header.Get("Authorization"))

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("s3cr3t"))
	})

	server := httptest.NewTLSServer(mux)
	defer server.Close()

	client := newTestClient(server)
	value, err := client.GetSecret(t.Context(), newTestSecret(server.URL))

	assert.NoError(t, err)
	assert.Equal(t, "s3cr3t", value)
}

func TestClient_GetSecret_AuthenticationFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/authn-jwt/gitlab/myAccount/authenticate", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid token"))
	})

	server := httptest.NewTLSServer(mux)
	defer server.Close()

	client := newTestClient(server)
	value, err := client.GetSecret(t.Context(), newTestSecret(server.URL))

	assert.Empty(t, value)
	require.Error(t, err)

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusUnauthorized, apiErr.StatusCode())
}

// A 200 with an empty body is not a usable secret value - without this check
// the job would silently get an empty variable instead of a clear failure.
func TestClient_GetSecret_RejectsEmptyValue(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/authn-jwt/gitlab/myAccount/authenticate", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fakeAccessToken))
	})
	mux.HandleFunc("/secrets/myAccount/variable/app%2Fprod%2Fdb-password", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	server := httptest.NewTLSServer(mux)
	defer server.Close()

	client := newTestClient(server)
	value, err := client.GetSecret(t.Context(), newTestSecret(server.URL))

	assert.Empty(t, value)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestClient_GetSecret_FetchNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/authn-jwt/gitlab/myAccount/authenticate", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fakeAccessToken))
	})
	mux.HandleFunc("/secrets/myAccount/variable/app%2Fprod%2Fdb-password", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("not found"))
	})

	server := httptest.NewTLSServer(mux)
	defer server.Close()

	client := newTestClient(server)
	value, err := client.GetSecret(t.Context(), newTestSecret(server.URL))

	assert.Empty(t, value)
	assert.True(t, errors.Is(err, common.ErrSecretNotFound))
}

func TestClient_GetSecret_FetchServerError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/authn-jwt/gitlab/myAccount/authenticate", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fakeAccessToken))
	})
	mux.HandleFunc("/secrets/myAccount/variable/app%2Fprod%2Fdb-password", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("appliance down"))
	})

	server := httptest.NewTLSServer(mux)
	defer server.Close()

	client := newTestClient(server)
	value, err := client.GetSecret(t.Context(), newTestSecret(server.URL))

	assert.Empty(t, value)

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusServiceUnavailable, apiErr.StatusCode())
}

func TestNewAPIError_TruncatesLongBody(t *testing.T) {
	short := "invalid token"
	assert.Equal(t, short, newAPIError(http.StatusUnauthorized, short).Body)

	long := strings.Repeat("a", maxErrorBodyLen+100)
	err := newAPIError(http.StatusUnauthorized, long)

	assert.True(t, strings.HasSuffix(err.Body, "...(truncated)"))
	assert.Less(t, len(err.Body), len(long))
	assert.True(t, strings.HasPrefix(long, strings.TrimSuffix(err.Body, "...(truncated)")))
}

func TestClient_Do_RejectsOversizedBody(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("a"), maxResponseBodySize+1))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL, nil)
	require.NoError(t, err)

	client := NewClient()
	_, _, err = client.do(req)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds")
}

func TestClient_GetSecret_DoesNotFollowRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("redirect target must not be requested")
	}))
	defer target.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/authn-jwt/gitlab/myAccount/authenticate", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(fakeAccessToken))
	})
	mux.HandleFunc("/secrets/myAccount/variable/app%2Fprod%2Fdb-password", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	})

	server := httptest.NewTLSServer(mux)
	defer server.Close()

	client := newTestClient(server)
	value, err := client.GetSecret(t.Context(), newTestSecret(server.URL))

	assert.Empty(t, value)

	var apiErr *APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, http.StatusFound, apiErr.StatusCode())
}

func TestApplianceBaseURL(t *testing.T) {
	tests := map[string]struct {
		raw         string
		expected    string
		expectedErr string
	}{
		"https appliance": {
			raw:      "https://conjur.example.com",
			expected: "https://conjur.example.com",
		},
		"trailing slashes trimmed": {
			raw:      "https://conjur.example.com//",
			expected: "https://conjur.example.com",
		},
		"path prefix kept": {
			raw:      "https://proxy.example.com/conjur",
			expected: "https://proxy.example.com/conjur",
		},
		// A fragment would swallow the API path appended to this base, letting
		// a job pick the path the manager requests.
		"fragment rejected": {
			raw:         "https://conjur.example.com/latest/meta-data/#",
			expectedErr: "must not contain a fragment",
		},
		"query rejected": {
			raw:         "https://conjur.example.com/admin?x=",
			expectedErr: "must not contain a query string",
		},
		"plaintext http rejected": {
			raw:         "http://conjur.example.com",
			expectedErr: `scheme "http" is not supported`,
		},
		"non-http scheme rejected": {
			raw:         "file://localhost/etc/passwd",
			expectedErr: `scheme "file" is not supported`,
		},
		"missing host rejected": {
			raw:         "https://",
			expectedErr: "no host",
		},
		"user info rejected": {
			raw:         "https://user:pw@conjur.example.com",
			expectedErr: "must not contain user info",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			got, err := applianceBaseURL(tt.raw)

			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				assert.Empty(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

// A job that points appliance_url at a service only the runner manager can
// reach must not receive that service's response as the secret value, so a 200
// that is not a Conjur access token has to stop the flow before the fetch.
func TestClient_GetSecret_RejectsNonTokenAuthenticateResponse(t *testing.T) {
	var paths []string

	// Answers 200 on any path, like a metadata endpoint or a catch-all router.
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("AKIA-INTERNAL-CREDENTIALS"))
	}))
	defer server.Close()

	client := newTestClient(server)
	value, err := client.GetSecret(t.Context(), newTestSecret(server.URL))

	assert.Empty(t, value, "an arbitrary 200 body must not become the secret")
	require.ErrorIs(t, err, errNotAnAccessToken)
	assert.Len(t, paths, 1, "the secret must not be fetched after a bad token")
}

func TestClient_GetSecret_RejectsApplianceURLWithFragment(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("appliance must not be requested, got %s %s", r.Method, r.URL.RequestURI())
	}))
	defer server.Close()

	client := newTestClient(server)
	value, err := client.GetSecret(t.Context(), newTestSecret(server.URL+"/latest/meta-data/#"))

	assert.Empty(t, value)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid conjur appliance url")
}

func TestEncodeVariableID(t *testing.T) {
	tests := map[string]struct {
		id       string
		expected string
	}{
		"slashes become %2F": {
			id:       "app/prod/db-password",
			expected: "app%2Fprod%2Fdb-password",
		},
		"spaces become %20": {
			id:       "app prod",
			expected: "app%20prod",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.Equal(t, tt.expected, encodeVariableID(tt.id))
		})
	}
}
