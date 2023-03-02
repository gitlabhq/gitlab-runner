package docker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/sirupsen/logrus"
)

// ErrRedirectNotAllowed is returned when we get a 3xx request from the Docker
// client to prevent any redirections to malicious docker clients.
var ErrRedirectNotAllowed = errors.New("redirects disallowed")

// IsErrNotFound checks whether a returned error is due to an image or container
// not being found. Proxies the docker implementation.
func IsErrNotFound(err error) bool {
	unwrapped := errors.Unwrap(err)
	if unwrapped != nil {
		err = unwrapped
	}
	return client.IsErrNotFound(err)
}

// type officialDockerClient wraps a "github.com/docker/docker/client".Client,
// giving it the methods it needs to satisfy the docker.Client interface
type officialDockerClient struct {
	client *client.Client
}

func newOfficialDockerClient(c Credentials) (*officialDockerClient, error) {
	options := []client.Opt{
		client.WithAPIVersionNegotiation(),
		client.WithVersionFromEnv(),
	}

	// options acting upon the client and transport need to be done in a
	// specific order.
	options = append(
		options,
		WithCustomHTTPClient(),
		WithCustomTLSClientConfig(c),
		client.WithHost(c.Host),
		WithCustomKeepalive(),
	)

	dockerClient, err := client.NewClientWithOpts(options...)
	if err != nil {
		logrus.Errorln("Error creating Docker client:", err)
		return nil, err
	}

	return &officialDockerClient{
		client: dockerClient,
	}, nil
}

func wrapError(method string, err error, started time.Time) error {
	if err == nil {
		return nil
	}

	seconds := int(time.Since(started).Seconds())

	if _, file, line, ok := runtime.Caller(2); ok {
		return fmt.Errorf("%w (%s:%d:%ds)", err, filepath.Base(file), line, seconds)
	}

	return fmt.Errorf("%w (%s:%ds)", err, method, seconds)
}

func (c *officialDockerClient) ClientVersion() string {
	return c.client.ClientVersion()
}

func (c *officialDockerClient) ImageInspectWithRaw(
	ctx context.Context,
	imageID string,
) (types.ImageInspect, []byte, error) {
	started := time.Now()
	image, data, err := c.client.ImageInspectWithRaw(ctx, imageID)
	return image, data, wrapError("ImageInspectWithRaw", err, started)
}

func (c *officialDockerClient) ContainerList(
	ctx context.Context,
	options types.ContainerListOptions,
) ([]types.Container, error) {
	started := time.Now()
	containers, err := c.client.ContainerList(ctx, options)
	return containers, wrapError("ContainerList", err, started)
}

func (c *officialDockerClient) ContainerCreate(
	ctx context.Context,
	config *container.Config,
	hostConfig *container.HostConfig,
	networkingConfig *network.NetworkingConfig,
	containerName string,
) (container.CreateResponse, error) {
	started := time.Now()
	container, err := c.client.ContainerCreate(ctx, config, hostConfig, networkingConfig, nil, containerName)
	return container, wrapError("ContainerCreate", err, started)
}

func (c *officialDockerClient) ContainerStart(
	ctx context.Context,
	containerID string,
	options types.ContainerStartOptions,
) error {
	started := time.Now()
	err := c.client.ContainerStart(ctx, containerID, options)
	return wrapError("ContainerCreate", err, started)
}

func (c *officialDockerClient) ContainerKill(ctx context.Context, containerID string, signal string) error {
	started := time.Now()
	err := c.client.ContainerKill(ctx, containerID, signal)
	return wrapError("ContainerKill", err, started)
}

func (c *officialDockerClient) ContainerStop(
	ctx context.Context,
	containerID string,
	options container.StopOptions,
) error {
	started := time.Now()
	err := c.client.ContainerStop(ctx, containerID, options)
	return wrapError("ContainerStop", err, started)
}

func (c *officialDockerClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	started := time.Now()
	data, err := c.client.ContainerInspect(ctx, containerID)
	return data, wrapError("ContainerInspect", err, started)
}

func (c *officialDockerClient) ContainerAttach(
	ctx context.Context,
	container string,
	options types.ContainerAttachOptions,
) (types.HijackedResponse, error) {
	started := time.Now()
	response, err := c.client.ContainerAttach(ctx, container, options)
	return response, wrapError("ContainerAttach", err, started)
}

func (c *officialDockerClient) ContainerRemove(
	ctx context.Context,
	containerID string,
	options types.ContainerRemoveOptions,
) error {
	started := time.Now()
	err := c.client.ContainerRemove(ctx, containerID, options)
	return wrapError("ContainerRemove", err, started)
}

func (c *officialDockerClient) ContainerWait(
	ctx context.Context,
	containerID string,
	condition container.WaitCondition,
) (<-chan container.WaitResponse, <-chan error) {
	return c.client.ContainerWait(ctx, containerID, condition)
}

func (c *officialDockerClient) ContainerLogs(
	ctx context.Context,
	container string,
	options types.ContainerLogsOptions,
) (io.ReadCloser, error) {
	started := time.Now()
	rc, err := c.client.ContainerLogs(ctx, container, options)
	return rc, wrapError("ContainerLogs", err, started)
}

func (c *officialDockerClient) ContainerExecCreate(
	ctx context.Context,
	container string,
	config types.ExecConfig,
) (types.IDResponse, error) {
	started := time.Now()
	resp, err := c.client.ContainerExecCreate(ctx, container, config)
	return resp, wrapError("ContainerExecCreate", err, started)
}

func (c *officialDockerClient) ContainerExecAttach(
	ctx context.Context,
	execID string,
	config types.ExecStartCheck,
) (types.HijackedResponse, error) {
	started := time.Now()
	resp, err := c.client.ContainerExecAttach(ctx, execID, config)
	return resp, wrapError("ContainerExecAttach", err, started)
}

func (c *officialDockerClient) NetworkCreate(
	ctx context.Context,
	networkName string,
	options types.NetworkCreate,
) (types.NetworkCreateResponse, error) {
	started := time.Now()
	response, err := c.client.NetworkCreate(ctx, networkName, options)
	return response, wrapError("NetworkCreate", err, started)
}

func (c *officialDockerClient) NetworkRemove(ctx context.Context, networkID string) error {
	started := time.Now()
	err := c.client.NetworkRemove(ctx, networkID)
	return wrapError("NetworkRemove", err, started)
}

func (c *officialDockerClient) NetworkDisconnect(ctx context.Context, networkID, containerID string, force bool) error {
	started := time.Now()
	err := c.client.NetworkDisconnect(ctx, networkID, containerID, force)
	return wrapError("NetworkDisconnect", err, started)
}

func (c *officialDockerClient) NetworkList(
	ctx context.Context,
	options types.NetworkListOptions,
) ([]types.NetworkResource, error) {
	started := time.Now()
	networks, err := c.client.NetworkList(ctx, options)
	return networks, wrapError("NetworkList", err, started)
}

func (c *officialDockerClient) NetworkInspect(ctx context.Context, networkID string) (types.NetworkResource, error) {
	started := time.Now()
	resource, err := c.client.NetworkInspect(ctx, networkID, types.NetworkInspectOptions{})
	return resource, wrapError("NetworkInspect", err, started)
}

func (c *officialDockerClient) VolumeCreate(
	ctx context.Context,
	options volume.CreateOptions,
) (volume.Volume, error) {
	started := time.Now()
	v, err := c.client.VolumeCreate(ctx, options)
	return v, wrapError("VolumeCreate", err, started)
}

func (c *officialDockerClient) VolumeRemove(ctx context.Context, volumeID string, force bool) error {
	started := time.Now()
	err := c.client.VolumeRemove(ctx, volumeID, force)
	return wrapError("VolumeRemove", err, started)
}

func (c *officialDockerClient) VolumeInspect(ctx context.Context, volumeID string) (volume.Volume, error) {
	started := time.Now()
	v, err := c.client.VolumeInspect(ctx, volumeID)
	return v, wrapError("VolumeInspect", err, started)
}

func (c *officialDockerClient) Info(ctx context.Context) (types.Info, error) {
	started := time.Now()
	info, err := c.client.Info(ctx)
	return info, wrapError("Info", err, started)
}

func (c *officialDockerClient) ImageImportBlocking(
	ctx context.Context,
	source types.ImageImportSource,
	ref string,
	options types.ImageImportOptions,
) error {
	started := time.Now()
	rc, err := c.client.ImageImport(ctx, source, ref, options)
	if err != nil {
		return wrapError("ImageImport", err, started)
	}

	return wrapError("ImageImport", c.handleEventStream(rc), started)
}

func (c *officialDockerClient) ImagePullBlocking(
	ctx context.Context,
	ref string,
	options types.ImagePullOptions,
) error {
	started := time.Now()
	rc, err := c.client.ImagePull(ctx, ref, options)
	if err != nil {
		return wrapError("ImagePull", err, started)
	}

	return wrapError("ImagePull", c.handleEventStream(rc), started)
}

func (c *officialDockerClient) handleEventStream(rc io.ReadCloser) error {
	defer func() { _ = rc.Close() }()

	return jsonmessage.DisplayJSONMessagesStream(rc, io.Discard, 0, false, nil)
}

func (c *officialDockerClient) Close() error {
	return c.client.Close()
}

// New attempts to create a new Docker client of the specified version. If the
// specified version is empty, it will use the default version.
//
// If no host is given in the Credentials, it will attempt to look up
// details from the environment. If that fails, it will use the default
// connection details for your platform.
func New(c Credentials) (Client, error) {
	if c.Host == "" {
		c = credentialsFromEnv()
	}

	// Use the default if nothing is specified by caller *or* environment
	if c.Host == "" {
		c.Host = client.DefaultDockerHost
	}

	return newOfficialDockerClient(c)
}
