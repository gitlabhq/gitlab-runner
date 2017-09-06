package docker_helpers

import (
	"io"
	"runtime/debug"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"golang.org/x/net/context"
)

func printPersistentConnectionClosedError(err error) {
	if err == nil {
		return
	}

	if strings.Contains(err.Error(), "Cannot connect to the Docker daemon") || strings.Contains(err.Error(), "Persistent connection closed") {
		logrus.Println("IMPORTANT ERROR HAPPENED:", err)
		debug.PrintStack()
	}
}

type clientWithErrorLogger struct {
	Client Client
}

func (c *clientWithErrorLogger) ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error) {
	image, data, err := c.Client.ImageInspectWithRaw(ctx, imageID)
	printPersistentConnectionClosedError(err)
	return image, data, err
}

func (c *clientWithErrorLogger) ImagePullBlocking(ctx context.Context, ref string, options types.ImagePullOptions) error {
	err := c.Client.ImagePullBlocking(ctx, ref, options)
	printPersistentConnectionClosedError(err)
	return err
}

func (c *clientWithErrorLogger) ImageImportBlocking(ctx context.Context, source types.ImageImportSource, ref string, options types.ImageImportOptions) error {
	err := c.Client.ImageImportBlocking(ctx, source, ref, options)
	printPersistentConnectionClosedError(err)
	return err
}

func (c *clientWithErrorLogger) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (container.ContainerCreateCreatedBody, error) {
	body, err := c.Client.ContainerCreate(ctx, config, hostConfig, networkingConfig, containerName)
	printPersistentConnectionClosedError(err)
	return body, err
}

func (c *clientWithErrorLogger) ContainerStart(ctx context.Context, containerID string, options types.ContainerStartOptions) error {
	err := c.Client.ContainerStart(ctx, containerID, options)
	printPersistentConnectionClosedError(err)
	return err
}

func (c *clientWithErrorLogger) ContainerWait(ctx context.Context, containerID string) (int64, error) {
	errorCode, err := c.Client.ContainerWait(ctx, containerID)
	printPersistentConnectionClosedError(err)
	return errorCode, err
}

func (c *clientWithErrorLogger) ContainerKill(ctx context.Context, containerID string, signal string) error {
	err := c.Client.ContainerKill(ctx, containerID, signal)
	printPersistentConnectionClosedError(err)
	return err
}

func (c *clientWithErrorLogger) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	container, err := c.Client.ContainerInspect(ctx, containerID)
	printPersistentConnectionClosedError(err)
	return container, err
}

func (c *clientWithErrorLogger) ContainerAttach(ctx context.Context, container string, options types.ContainerAttachOptions) (types.HijackedResponse, error) {
	response, err := c.Client.ContainerAttach(ctx, container, options)
	printPersistentConnectionClosedError(err)
	return response, err
}

func (c *clientWithErrorLogger) ContainerRemove(ctx context.Context, containerID string, options types.ContainerRemoveOptions) error {
	err := c.Client.ContainerRemove(ctx, containerID, options)
	printPersistentConnectionClosedError(err)
	return err
}

func (c *clientWithErrorLogger) ContainerLogs(ctx context.Context, container string, options types.ContainerLogsOptions) (io.ReadCloser, error) {
	rc, err := c.Client.ContainerLogs(ctx, container, options)
	printPersistentConnectionClosedError(err)
	return rc, err
}

func (c *clientWithErrorLogger) NetworkDisconnect(ctx context.Context, networkID string, containerID string, force bool) error {
	err := c.Client.NetworkDisconnect(ctx, networkID, containerID, force)
	printPersistentConnectionClosedError(err)
	return err
}

func (c *clientWithErrorLogger) NetworkList(ctx context.Context, options types.NetworkListOptions) ([]types.NetworkResource, error) {
	rc, err := c.Client.NetworkList(ctx, options)
	printPersistentConnectionClosedError(err)
	return rc, err
}

func (c *clientWithErrorLogger) Info(ctx context.Context) (types.Info, error) {
	info, err := c.Client.Info(ctx)
	printPersistentConnectionClosedError(err)
	return info, err
}

func (c *clientWithErrorLogger) Close() error {
	err := c.Client.Close()
	printPersistentConnectionClosedError(err)
	return err
}
