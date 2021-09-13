//go:build !integration
// +build !integration

package docker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/docker/docker/api/types"
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

	client, err := New(credentials, "")
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

func TestNew_Version(t *testing.T) {
	cases := []struct {
		version         string
		host            string
		expectedVersion string
	}{
		{
			version:         "0.11",
			expectedVersion: "0.11",
		},
		{
			version:         "",
			expectedVersion: DefaultAPIVersion,
		},
	}

	for _, c := range cases {
		t.Run(c.expectedVersion, func(t *testing.T) {
			client, err := New(Credentials{}, c.version)
			require.NoError(t, err)

			test, ok := client.(*officialDockerClient)
			assert.True(t, ok)
			assert.Equal(t, c.expectedVersion, test.client.ClientVersion())
		})
	}
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

	client, err := New(credentials, "")
	require.NoError(t, err)

	_, err = client.Info(context.Background())
	require.Error(t, err)
	// The latest version of github.com/pkg/errors still doesn't provide the
	// Unwrap method for withStack and withMessage types, so we can't leverage
	// errors.Is and must resort to string search
	assert.Contains(t, err.Error(), "error during connect")
	assert.ErrorIs(t, err, ErrRedirectNotAllowed)
}
