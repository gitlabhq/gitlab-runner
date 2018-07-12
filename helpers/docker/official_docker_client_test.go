package docker_helpers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const dockerAPIVersion = "1.18"

func prepareDockerClientAndFakeServer(t *testing.T, handler http.HandlerFunc) (Client, *httptest.Server) {
	server := httptest.NewServer(handler)

	credentials := DockerCredentials{
		Host:      server.URL,
		TLSVerify: false,
	}

	client, err := New(credentials, dockerAPIVersion)
	require.NoError(t, err)

	return client, server
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
	assert.Regexp(t, "\\(official_docker_client_test.go:38:\\d+s\\)", err.Error())
}
