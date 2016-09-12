package docker_helpers

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
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
