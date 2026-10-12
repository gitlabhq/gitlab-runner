//go:build integration

package conjur

import (
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/conjur/service"
)

// TestConjurResolver_Integration exercises the resolver against the real
// service.Client and its HTTP calls end to end, without a mocked client
// interface. Unlike GCP/AWS, Conjur's appliance URL comes straight from the
// job payload rather than from runner-side environment/SDK configuration, so
// pointing this at an httptest server that emulates the authn-jwt and
// secrets endpoints is enough to cover the real wire format without a live
// Conjur appliance.
func TestConjurResolver_Integration(t *testing.T) {
	const (
		account   = "myAccount"
		serviceID = "gitlab"
		jwt       = "the-ci-jwt"
		path      = "app/prod/db-password"
		// Conjur returns the access token as plain JSON; the client must
		// base64-encode it for the Authorization header.
		token = `{"protected":"p","payload":"pl","signature":"s"}`
	)

	tests := map[string]struct {
		authStatus   int
		authBody     string
		fetchStatus  int
		fetchBody    string
		expectedVal  string
		expectErrSub string
	}{
		"happy path": {
			authStatus:  http.StatusOK,
			authBody:    token,
			fetchStatus: http.StatusOK,
			fetchBody:   "s3cr3t",
			expectedVal: "s3cr3t",
		},
		"auth rejected": {
			authStatus:   http.StatusUnauthorized,
			authBody:     "invalid jwt",
			expectErrSub: "authenticating with conjur",
		},
		"secret not found": {
			authStatus:   http.StatusOK,
			authBody:     token,
			fetchStatus:  http.StatusNotFound,
			fetchBody:    "not found",
			expectErrSub: "secret not found",
		},
		"appliance failure": {
			authStatus:   http.StatusOK,
			authBody:     token,
			fetchStatus:  http.StatusServiceUnavailable,
			fetchBody:    "down for maintenance",
			expectErrSub: "fetching secret from conjur",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc(fmt.Sprintf("/authn-jwt/%s/%s/authenticate", serviceID, account), func(w http.ResponseWriter, r *http.Request) {
				require.NoError(t, r.ParseForm())
				assert.Equal(t, jwt, r.FormValue("jwt"))

				w.WriteHeader(tc.authStatus)
				_, _ = w.Write([]byte(tc.authBody))
			})
			mux.HandleFunc(fmt.Sprintf("/secrets/%s/variable/app%%2Fprod%%2Fdb-password", account), func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, fmt.Sprintf(`Token token="%s"`, base64.StdEncoding.EncodeToString([]byte(token))), r.Header.Get("Authorization"))

				w.WriteHeader(tc.fetchStatus)
				_, _ = w.Write([]byte(tc.fetchBody))
			})

			server := httptest.NewTLSServer(mux)
			defer server.Close()

			pool := x509.NewCertPool()
			pool.AddCert(server.Certificate())

			r := &resolver{
				secret: spec.Secret{
					Conjur: &spec.ConjurSecret{
						Path: path,
						Server: spec.ConjurServer{
							ApplianceURL:      server.URL,
							Account:           account,
							AuthnJWTServiceID: serviceID,
							JWT:               jwt,
						},
					},
				},
				client: service.NewClient(service.WithRootCAs(pool)),
			}

			val, err := r.Resolve()

			if tc.expectErrSub != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectErrSub)
				assert.Empty(t, val)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedVal, val)
		})
	}
}
