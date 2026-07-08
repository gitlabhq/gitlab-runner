//go:build integration

package docker_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/test"
)

// TestDockerTLSConnection connects to the dind service over TCP+TLS instead
// of the local unix socket CI normally uses, to exercise the real TLS
// handshake and certificate verification path in helpers/docker/options.go's
// configureTransport -- which is otherwise only unit-tested with a stubbed
// dialer and fake certs. The dind image already generates real client certs
// under /certs/client by default; CI shares that directory with the job
// container via the same volume dind itself writes to.
//
// TEST_DOCKER_TLS_HOST / TEST_DOCKER_TLS_CERT_PATH let this be pointed at a
// locally-run dind container for development (e.g. `docker run -d
// --name dind-tls-test --privileged -p 12376:2376 -e DOCKER_TLS_CERTDIR=/certs
// docker:27-dind`, then `docker cp dind-tls-test:/certs/client ./certs`),
// since a local dev machine has no "docker" hostname or shared /certs volume.
func TestDockerTLSConnection(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	host := os.Getenv("TEST_DOCKER_TLS_HOST")
	if host == "" {
		host = "tcp://docker:2376"
	}
	certPath := os.Getenv("TEST_DOCKER_TLS_CERT_PATH")
	if certPath == "" {
		certPath = "/certs/client"
	}
	if _, err := os.Stat(certPath); err != nil {
		t.Skipf("TLS client certs not available at %q: %v", certPath, err)
	}

	client, err := docker.New(docker.Credentials{
		Host:      host,
		TLSVerify: true,
		CertPath:  certPath,
	})
	require.NoError(t, err)
	defer client.Close()

	_, err = client.Info(context.Background())
	require.NoError(t, err, "TLS handshake and certificate verification should succeed against the real daemon")
}
