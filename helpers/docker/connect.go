package docker_helpers

import (
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/docker/docker/client"
	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
)

var dockerDialer = &net.Dialer{
	Timeout:   30 * time.Second,
	KeepAlive: 30 * time.Second,
}

// New attempts to create a new Docker client of the specified version.
//
// If no host is given in the DockerCredentials, it will attempt to look up
// details from the environment. If that fails, it will use the default
// connection details for your platform.
func New(c DockerCredentials, apiVersion string) (Client, error) {
	if c.Host == "" {
		c = credentialsFromEnv()
	}

	// Use the default if nothing is specified by caller *or* environment
	if c.Host == "" {
		c.Host = client.DefaultDockerHost
	}

	return newOfficialDockerClient(c, apiVersion)
}

func newHTTPTransport(c DockerCredentials) (*http.Transport, error) {
	proto, addr, _, err := client.ParseHost(c.Host)
	if err != nil {
		return nil, err
	}

	tr := &http.Transport{}
	if err := sockets.ConfigureTransport(tr, proto, addr); err != nil {
		return nil, err
	}

	// FIXME: is a TLS connection with InsecureSkipVerify == true ever wanted?
	if c.TLSVerify {
		options := tlsconfig.Options{}

		if c.CertPath != "" {
			options.CAFile = filepath.Join(c.CertPath, "ca.pem")
			options.CertFile = filepath.Join(c.CertPath, "cert.pem")
			options.KeyFile = filepath.Join(c.CertPath, "key.pem")
		}

		tlsConfig, err := tlsconfig.Client(options)
		if err != nil {
			tr.CloseIdleConnections()
			return nil, err
		}

		tr.TLSHandshakeTimeout = 10 * time.Second
		tr.TLSClientConfig = tlsConfig
	}

	return tr, nil
}
