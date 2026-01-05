package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"runtime"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	system "github.com/docker/docker/api/types/system"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
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
	return errdefs.IsNotFound(err)
}

// type officialDockerClient wraps a "github.com/docker/docker/client".Client,
// giving it the methods it needs to satisfy the docker.Client interface
type officialDockerClient struct {
	client    *client.Client
	transport *http.Transport
}

func newOfficialDockerClient(c Credentials, opts ...client.Opt) (*officialDockerClient, error) {
	options := []client.Opt{
		client.WithAPIVersionNegotiation(),
		client.WithVersionFromEnv(),
	}

	// create the http.Transport instance here so we can cache it. In docker SKD >= v25 the http.Client's Transport
	// instance is overwritten with an otelhttp.Transport, which does not expose its TSLCientConfig. Some tests need to
	// access the TSLCientConfig to assert TSL was configured correctly.
	transport := http.Transport{}

	// options acting upon the client and transport need to be done in a
	// specific order.
	options = append(
		options,
		client.WithHost(c.Host),
		WithCustomHTTPClient(&transport),
		WithCustomTLSClientConfig(c),
	)

	options = append(options, opts...)

	dockerClient, err := client.NewClientWithOpts(options...)
	if err != nil {
		logrus.Errorln("Error creating Docker client:", err)
		return nil, err
	}

	return &officialDockerClient{
		client:    dockerClient,
		transport: &transport,
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

func (c *officialDockerClient) ServerVersion(ctx context.Context) (types.Version, error) {
	return c.client.ServerVersion(ctx)
}

func (c *officialDockerClient) ImageInspectWithRaw(
	ctx context.Context,
	imageID string,
) (image.InspectResponse, []byte, error) {
	started := time.Now()
	raw := &bytes.Buffer{}
	inspectOpts := client.ImageInspectWithRawResponse(raw)
	image, err := c.client.ImageInspect(ctx, imageID, inspectOpts)
	return image, raw.Bytes(), wrapError("ImageInspectWithRaw", err, started)
}

func (c *officialDockerClient) ContainerList(
	ctx context.Context,
	options container.ListOptions,
) ([]container.Summary, error) {
	started := time.Now()
	containers, err := c.client.ContainerList(ctx, options)
	return containers, wrapError("ContainerList", err, started)
}

func (c *officialDockerClient) ContainerCreate(
	ctx context.Context,
	config *container.Config,
	hostConfig *container.HostConfig,
	networkingConfig *network.NetworkingConfig,
	platform *v1.Platform,
	containerName string,
) (container.CreateResponse, error) {
	started := time.Now()
	container, err := c.client.ContainerCreate(ctx, config, hostConfig, networkingConfig, platform, containerName)
	return container, wrapError("ContainerCreate", err, started)
}

func (c *officialDockerClient) ContainerStart(
	ctx context.Context,
	containerID string,
	options container.StartOptions,
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

func (c *officialDockerClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	started := time.Now()
	data, err := c.client.ContainerInspect(ctx, containerID)
	return data, wrapError("ContainerInspect", err, started)
}

func (c *officialDockerClient) ContainerAttach(
	ctx context.Context,
	container string,
	options container.AttachOptions,
) (types.HijackedResponse, error) {
	started := time.Now()
	response, err := c.client.ContainerAttach(ctx, container, options)
	return response, wrapError("ContainerAttach", err, started)
}

func (c *officialDockerClient) ContainerRemove(
	ctx context.Context,
	containerID string,
	options container.RemoveOptions,
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
	options container.LogsOptions,
) (io.ReadCloser, error) {
	started := time.Now()
	rc, err := c.client.ContainerLogs(ctx, container, options)
	return rc, wrapError("ContainerLogs", err, started)
}

func (c *officialDockerClient) ContainerExecCreate(
	ctx context.Context,
	container string,
	config container.ExecOptions,
) (container.ExecCreateResponse, error) {
	started := time.Now()
	resp, err := c.client.ContainerExecCreate(ctx, container, config)
	return resp, wrapError("ContainerExecCreate", err, started)
}

func (c *officialDockerClient) ContainerExecAttach(
	ctx context.Context,
	execID string,
	config container.ExecStartOptions,
) (types.HijackedResponse, error) {
	started := time.Now()
	resp, err := c.client.ContainerExecAttach(ctx, execID, config)
	return resp, wrapError("ContainerExecAttach", err, started)
}

func (c *officialDockerClient) NetworkCreate(
	ctx context.Context,
	networkName string,
	options network.CreateOptions,
) (network.CreateResponse, error) {
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
	options network.ListOptions,
) ([]network.Summary, error) {
	started := time.Now()
	networks, err := c.client.NetworkList(ctx, options)
	return networks, wrapError("NetworkList", err, started)
}

func (c *officialDockerClient) NetworkInspect(ctx context.Context, networkID string) (network.Inspect, error) {
	started := time.Now()
	resource, err := c.client.NetworkInspect(ctx, networkID, network.InspectOptions{})
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

func (c *officialDockerClient) VolumeList(ctx context.Context, options volume.ListOptions) (volume.ListResponse, error) {
	started := time.Now()
	v, err := c.client.VolumeList(ctx, options)
	return v, wrapError("VolumeList", err, started)
}

func (c *officialDockerClient) Info(ctx context.Context) (system.Info, error) {
	started := time.Now()
	info, err := c.client.Info(ctx)
	return info, wrapError("Info", err, started)
}

func (c *officialDockerClient) ImageLoad(ctx context.Context, input io.Reader, quiet bool) (image.LoadResponse, error) {
	started := time.Now()
	resp, err := c.client.ImageLoad(ctx, input, client.ImageLoadWithQuiet(quiet))
	return resp, wrapError("ImageLoad", err, started)
}

func (c *officialDockerClient) ImageTag(ctx context.Context, source string, target string) error {
	started := time.Now()
	return wrapError("ImageTag", c.client.ImageTag(ctx, source, target), started)
}

func (c *officialDockerClient) ImageImportBlocking(
	ctx context.Context,
	source image.ImportSource,
	ref string,
	options image.ImportOptions,
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
	options image.PullOptions,
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
func New(c Credentials, options ...client.Opt) (Client, error) {
	if c.Host == "" {
		c = credentialsFromEnv()
	}

	// Use the default if nothing is specified by caller *or* environment
	if c.Host == "" {
		c.Host = client.DefaultDockerHost
	}

	return newOfficialDockerClient(c, options...)
}
