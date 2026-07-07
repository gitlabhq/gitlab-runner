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
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	system "github.com/moby/moby/api/types/system"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
	"github.com/moby/moby/client/pkg/jsonmessage"
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

// type officialDockerClient wraps a "github.com/moby/moby/client".Client,
// giving it the methods it needs to satisfy the docker.Client interface
type officialDockerClient struct {
	client    *client.Client
	transport *http.Transport
}

func newOfficialDockerClient(c Credentials, opts ...client.Opt) (*officialDockerClient, error) {
	// create the http.Transport instance here so we can cache it. In docker SDK >= v25 the http.Client's Transport
	// instance is overwritten with an otelhttp.Transport, which does not expose its TLSClientConfig. Some tests need to
	// access the TLSClientConfig to assert TLS was configured correctly.
	transport := http.Transport{}

	if err := configureTransport(&transport, c); err != nil {
		logrus.Errorln("Error configuring Docker client transport:", err)
		return nil, err
	}

	options := []client.Opt{
		client.WithAPIVersionFromEnv(),
		client.WithHost(c.Host),
		client.WithHTTPClient(newCustomHTTPClient(&transport)),
	}

	options = append(options, opts...)

	dockerClient, err := client.New(options...)
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

func (c *officialDockerClient) ServerVersion(ctx context.Context) (client.ServerVersionResult, error) {
	return c.client.ServerVersion(ctx, client.ServerVersionOptions{})
}

func (c *officialDockerClient) ImageInspectWithRaw(
	ctx context.Context,
	imageID string,
	platform *v1.Platform,
) (image.InspectResponse, []byte, error) {
	started := time.Now()
	raw := &bytes.Buffer{}
	inspectOpts := []client.ImageInspectOption{client.ImageInspectWithRawResponse(raw)}
	if platform != nil {
		inspectOpts = append(inspectOpts, client.ImageInspectWithPlatform(platform))
	}
	res, err := c.client.ImageInspect(ctx, imageID, inspectOpts...)
	return res.InspectResponse, raw.Bytes(), wrapError("ImageInspectWithRaw", err, started)
}

func (c *officialDockerClient) ContainerList(
	ctx context.Context,
	options client.ContainerListOptions,
) ([]container.Summary, error) {
	started := time.Now()
	res, err := c.client.ContainerList(ctx, options)
	return res.Items, wrapError("ContainerList", err, started)
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
	res, err := c.client.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config:           config,
		HostConfig:       hostConfig,
		NetworkingConfig: networkingConfig,
		Platform:         platform,
		Name:             containerName,
	})
	return container.CreateResponse{ID: res.ID, Warnings: res.Warnings}, wrapError("ContainerCreate", err, started)
}

func (c *officialDockerClient) ContainerStart(
	ctx context.Context,
	containerID string,
	options client.ContainerStartOptions,
) error {
	started := time.Now()
	_, err := c.client.ContainerStart(ctx, containerID, options)
	return wrapError("ContainerStart", err, started)
}

func (c *officialDockerClient) ContainerKill(ctx context.Context, containerID string, signal string) error {
	started := time.Now()
	_, err := c.client.ContainerKill(ctx, containerID, client.ContainerKillOptions{Signal: signal})
	return wrapError("ContainerKill", err, started)
}

func (c *officialDockerClient) ContainerStop(
	ctx context.Context,
	containerID string,
	options client.ContainerStopOptions,
) error {
	started := time.Now()
	_, err := c.client.ContainerStop(ctx, containerID, options)
	return wrapError("ContainerStop", err, started)
}

func (c *officialDockerClient) ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error) {
	started := time.Now()
	res, err := c.client.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	return res.Container, wrapError("ContainerInspect", err, started)
}

func (c *officialDockerClient) ContainerAttach(
	ctx context.Context,
	containerID string,
	options client.ContainerAttachOptions,
) (client.HijackedResponse, error) {
	started := time.Now()
	res, err := c.client.ContainerAttach(ctx, containerID, options)
	return res.HijackedResponse, wrapError("ContainerAttach", err, started)
}

func (c *officialDockerClient) ContainerRemove(
	ctx context.Context,
	containerID string,
	options client.ContainerRemoveOptions,
) error {
	started := time.Now()
	_, err := c.client.ContainerRemove(ctx, containerID, options)
	return wrapError("ContainerRemove", err, started)
}

func (c *officialDockerClient) ContainerWait(
	ctx context.Context,
	containerID string,
	condition container.WaitCondition,
) (<-chan container.WaitResponse, <-chan error) {
	res := c.client.ContainerWait(ctx, containerID, client.ContainerWaitOptions{Condition: condition})
	return res.Result, res.Error
}

func (c *officialDockerClient) ContainerLogs(
	ctx context.Context,
	containerID string,
	options client.ContainerLogsOptions,
) (io.ReadCloser, error) {
	started := time.Now()
	rc, err := c.client.ContainerLogs(ctx, containerID, options)
	return rc, wrapError("ContainerLogs", err, started)
}

func (c *officialDockerClient) ContainerExecCreate(
	ctx context.Context,
	containerID string,
	config client.ExecCreateOptions,
) (container.ExecCreateResponse, error) {
	started := time.Now()
	res, err := c.client.ExecCreate(ctx, containerID, config)
	return container.ExecCreateResponse{ID: res.ID}, wrapError("ContainerExecCreate", err, started)
}

func (c *officialDockerClient) ContainerExecAttach(
	ctx context.Context,
	execID string,
	config client.ExecAttachOptions,
) (client.HijackedResponse, error) {
	started := time.Now()
	res, err := c.client.ExecAttach(ctx, execID, config)
	return res.HijackedResponse, wrapError("ContainerExecAttach", err, started)
}

func (c *officialDockerClient) NetworkCreate(
	ctx context.Context,
	networkName string,
	options client.NetworkCreateOptions,
) (client.NetworkCreateResult, error) {
	started := time.Now()
	res, err := c.client.NetworkCreate(ctx, networkName, options)
	return res, wrapError("NetworkCreate", err, started)
}

func (c *officialDockerClient) NetworkRemove(ctx context.Context, networkID string) error {
	started := time.Now()
	_, err := c.client.NetworkRemove(ctx, networkID, client.NetworkRemoveOptions{})
	return wrapError("NetworkRemove", err, started)
}

func (c *officialDockerClient) NetworkDisconnect(ctx context.Context, networkID, containerID string, force bool) error {
	started := time.Now()
	_, err := c.client.NetworkDisconnect(ctx, networkID, client.NetworkDisconnectOptions{
		Container: containerID,
		Force:     force,
	})
	return wrapError("NetworkDisconnect", err, started)
}

func (c *officialDockerClient) NetworkList(
	ctx context.Context,
	options client.NetworkListOptions,
) ([]network.Summary, error) {
	started := time.Now()
	res, err := c.client.NetworkList(ctx, options)
	return res.Items, wrapError("NetworkList", err, started)
}

func (c *officialDockerClient) NetworkInspect(ctx context.Context, networkID string) (network.Inspect, error) {
	started := time.Now()
	res, err := c.client.NetworkInspect(ctx, networkID, client.NetworkInspectOptions{})
	return res.Network, wrapError("NetworkInspect", err, started)
}

func (c *officialDockerClient) VolumeCreate(
	ctx context.Context,
	options client.VolumeCreateOptions,
) (volume.Volume, error) {
	started := time.Now()
	res, err := c.client.VolumeCreate(ctx, options)
	return res.Volume, wrapError("VolumeCreate", err, started)
}

func (c *officialDockerClient) VolumeRemove(ctx context.Context, volumeID string, force bool) error {
	started := time.Now()
	_, err := c.client.VolumeRemove(ctx, volumeID, client.VolumeRemoveOptions{Force: force})
	return wrapError("VolumeRemove", err, started)
}

func (c *officialDockerClient) VolumeInspect(ctx context.Context, volumeID string) (volume.Volume, error) {
	started := time.Now()
	res, err := c.client.VolumeInspect(ctx, volumeID, client.VolumeInspectOptions{})
	return res.Volume, wrapError("VolumeInspect", err, started)
}

func (c *officialDockerClient) VolumeList(ctx context.Context, options client.VolumeListOptions) (client.VolumeListResult, error) {
	started := time.Now()
	res, err := c.client.VolumeList(ctx, options)
	return res, wrapError("VolumeList", err, started)
}

func (c *officialDockerClient) Info(ctx context.Context) (system.Info, error) {
	started := time.Now()
	res, err := c.client.Info(ctx, client.InfoOptions{})
	return res.Info, wrapError("Info", err, started)
}

func (c *officialDockerClient) ImageLoad(ctx context.Context, input io.Reader, quiet bool) (io.ReadCloser, error) {
	started := time.Now()
	resp, err := c.client.ImageLoad(ctx, input, client.ImageLoadWithQuiet(quiet))
	return resp, wrapError("ImageLoad", err, started)
}

func (c *officialDockerClient) ImageTag(ctx context.Context, source string, target string) error {
	started := time.Now()
	_, err := c.client.ImageTag(ctx, client.ImageTagOptions{Source: source, Target: target})
	return wrapError("ImageTag", err, started)
}

func (c *officialDockerClient) ImageImportBlocking(
	ctx context.Context,
	source client.ImageImportSource,
	ref string,
	options client.ImageImportOptions,
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
	options client.ImagePullOptions,
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
