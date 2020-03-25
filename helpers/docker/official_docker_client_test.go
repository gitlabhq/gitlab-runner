package docker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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
