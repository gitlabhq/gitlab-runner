package docker_helpers

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
	"golang.org/x/net/context"
)

// IsErrNotFound checks whether a returned error is due to an image or container
// not being found. Proxies the docker implementation.
func IsErrNotFound(err error) bool {
	return client.IsErrNotFound(err)
}

// type officialDockerClient wraps a "github.com/docker/docker/client".Client,
// giving it the methods it needs to satisfy the docker_helpers.Client interface
type officialDockerClient struct {
	*client.Client

	// Close() means "close idle connections held by engine-api's transport"
	Transport *http.Transport
}

func newOfficialDockerClient(c DockerCredentials, apiVersion string) (*officialDockerClient, error) {
	transport, err := newHTTPTransport(c)
	if err != nil {
		logrus.Errorln("Error creating TLS Docker client:", err)
		return nil, err
	}
	httpClient := &http.Client{Transport: transport}

	dockerClient, err := client.NewClient(c.Host, apiVersion, httpClient, nil)
	if err != nil {
		transport.CloseIdleConnections()
		logrus.Errorln("Error creating Docker client:", err)
		return nil, err
	}

	return &officialDockerClient{
		Client:    dockerClient,
		Transport: transport,
	}, nil
}

func (c *officialDockerClient) ImageImportBlocking(ctx context.Context, source types.ImageImportSource, ref string, options types.ImageImportOptions) error {
	readCloser, err := c.ImageImport(ctx, source, ref, options)
	if err != nil {
		return err
	}
	defer readCloser.Close()

	// TODO: respect the context here
	if _, err := io.Copy(ioutil.Discard, readCloser); err != nil {
		return fmt.Errorf("Failed to import image: %s", err)
	}

	return nil
}

func (c *officialDockerClient) ImagePullBlocking(ctx context.Context, ref string, options types.ImagePullOptions) error {
	readCloser, err := c.ImagePull(ctx, ref, options)
	if err != nil {
		return err
	}
	defer readCloser.Close()

	// TODO: respect the context here
	if _, err := io.Copy(ioutil.Discard, readCloser); err != nil {
		return fmt.Errorf("Failed to pull image: %s: %s", ref, err)
	}

	return nil
}

func (c *officialDockerClient) Close() error {
	c.Transport.CloseIdleConnections()
	return nil
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
