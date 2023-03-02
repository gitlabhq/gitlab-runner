//go:build !integration

package docker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func prepareDockerClientAndFakeServer(t *testing.T, handler http.HandlerFunc) (Client, *httptest.Server) {
	server := httptest.NewServer(handler)

	credentials := Credentials{
		Host:      server.URL,
		TLSVerify: false,
	}

	client, err := New(credentials)
	require.NoError(t, err)

	return client, server
}

func TestEventStreamError(t *testing.T) {
	client, server := prepareDockerClientAndFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"errorDetail": {
				"code": 0,
				"message": "stream error"
			}
		}`))
	})
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := client.ImagePullBlocking(ctx, "test", types.ImagePullOptions{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stream error")
	assert.ErrorAs(t, new(jsonmessage.JSONError), &err)
}

func TestWrapError(t *testing.T) {
	client, server := prepareDockerClientAndFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	})
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := client.Info(ctx)
	require.Error(t, err, "The request should respond with an error")
	assert.Regexp(t, "\\(official_docker_client_test.go:\\d\\d:\\d+s\\)", err.Error())
}

func TestRedirectsNotAllowed(t *testing.T) {
	_, server := prepareDockerClientAndFakeServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Fail(t, "This server should not be hit")
	})
	defer server.Close()

	handler := http.RedirectHandler(server.URL, http.StatusMovedPermanently)
	redirectingServer := httptest.NewServer(handler)
	defer redirectingServer.Close()

	credentials := Credentials{
		Host:      redirectingServer.URL,
		TLSVerify: false,
	}

	client, err := New(credentials)
	require.NoError(t, err)

	_, err = client.Info(context.Background())
	require.Error(t, err)
	// The latest version of github.com/pkg/errors still doesn't provide the
	// Unwrap method for withStack and withMessage types, so we can't leverage
	// errors.Is and must resort to string search
	assert.Contains(t, err.Error(), "error during connect")
	assert.ErrorIs(t, err, ErrRedirectNotAllowed)
}

func TestCredentialsConfigEnvOverride(t *testing.T) {
	tests := map[string]struct {
		credentials Credentials
		envVars     map[string]string
		assert      func(t *testing.T, client *officialDockerClient)
	}{
		"env host": {
			envVars: map[string]string{
				client.EnvOverrideHost: "http://envprovided",
			},
			assert: func(t *testing.T, c *officialDockerClient) {
				assert.Equal(t, "http://envprovided", c.client.DaemonHost())
			},
		},
		"credentials host": {
			credentials: Credentials{
				Host: "http://credprovided",
			},
			assert: func(t *testing.T, c *officialDockerClient) {
				assert.Equal(t, "http://credprovided", c.client.DaemonHost())
			},
		},
		"credentials host overrides env host": {
			credentials: Credentials{
				Host: "http://credprovided",
			},
			envVars: map[string]string{
				client.EnvOverrideHost: "http://envprovided",
			},
			assert: func(t *testing.T, c *officialDockerClient) {
				assert.Equal(t, "http://credprovided", c.client.DaemonHost())
			},
		},

		"env tls verify": {
			envVars: map[string]string{
				client.EnvTLSVerify: "1",
			},
			assert: func(t *testing.T, c *officialDockerClient) {
				assert.False(t, c.client.HTTPClient().Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify)
			},
		},
		// When DOCKER_TLS_VERIFY is "", TLS setup is entirely disabled. For
		// the docker cli client, this typically just disables TLS
		// verification, whilst still using the certificates, but Runner's
		// parsing of DOCKER_TLS_VERIFY has always acted differently.
		// We maintain this for now for backwards compatibility.
		"env skip tls verify (backwards compatibility)": {
			envVars: map[string]string{
				client.EnvTLSVerify: "",
			},
			assert: func(t *testing.T, c *officialDockerClient) {
				assert.Nil(t, c.client.HTTPClient().Transport.(*http.Transport).TLSClientConfig)
			},
		},

		"credentials tls verify does nothing when host is empty": {
			credentials: Credentials{
				Host:      "",
				TLSVerify: true,
			},
			assert: func(t *testing.T, c *officialDockerClient) {
				assert.Nil(t, c.client.HTTPClient().Transport.(*http.Transport).TLSClientConfig)
			},
		},
		"credentials tls verify set when host is provided": {
			credentials: Credentials{
				Host:      "http://credprovided",
				TLSVerify: true,
			},
			assert: func(t *testing.T, c *officialDockerClient) {
				assert.Equal(t, "http://credprovided", c.client.DaemonHost())
				assert.False(t, c.client.HTTPClient().Transport.(*http.Transport).TLSClientConfig.InsecureSkipVerify)
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			// unset docker variables so they don't influence these tests, and either
			// set them back or unset them at the end of the test.
			for _, key := range []string{
				client.EnvOverrideHost,
				client.EnvOverrideAPIVersion,
				client.EnvOverrideCertPath,
				client.EnvTLSVerify} {
				original, found := os.LookupEnv(key)
				if found {
					defer os.Setenv(key, original)
				}
				os.Unsetenv(key)
			}

			for key, val := range tc.envVars {
				os.Setenv(key, val)
			}

			client, err := New(tc.credentials)
			require.NoError(t, err)
			tc.assert(t, client.(*officialDockerClient))
		})
	}
}

func TestClientConfiguration(t *testing.T) {
	useTestDialerFunc = true
	defer func() {
		useTestDialerFunc = false
	}()

	for _, scheme := range []string{"http", "unix"} {
		t.Run(scheme, func(t *testing.T) {
			if runtime.GOOS == "windows" && scheme == "unix" {
				t.Skip("unix scheme unsupported on windows")
			}

			client, err := newOfficialDockerClient(Credentials{
				Host:      scheme + "://example.org",
				TLSVerify: true,
			})
			require.NoError(t, err)

			httpClient := client.client.HTTPClient()
			require.NotNil(t, httpClient.Transport)

			transport := httpClient.Transport.(*http.Transport)

			assert.Equal(t, defaultTLSHandshakeTimeout, transport.TLSHandshakeTimeout)
			assert.Equal(t, defaultResponseHeaderTimeout, transport.ResponseHeaderTimeout)
			assert.Equal(t, defaultExpectContinueTimeout, transport.ExpectContinueTimeout)
			assert.Equal(t, defaultIdleConnTimeout, transport.IdleConnTimeout)
			assert.NotNil(t, transport.TLSClientConfig)
			assert.Equal(t, scheme == "unix", transport.DisableCompression)
			assert.Equal(t, scheme+"://example.org", client.client.DaemonHost())

			//nolint:staticcheck
			require.NotNil(t, transport.Dial)
			if scheme == "http" {
				//nolint:staticcheck
				_, err = transport.Dial("", "")
				assert.ErrorIs(t, errDialerTest, err)
			}
		})
	}
}
