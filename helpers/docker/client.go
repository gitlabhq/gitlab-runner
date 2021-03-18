package docker

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/volume"
)

type Client interface {
	ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error)

	ImagePullBlocking(ctx context.Context, ref string, options types.ImagePullOptions) error
	ImageImportBlocking(
		ctx context.Context,
		source types.ImageImportSource,
		ref string,
		options types.ImageImportOptions,
	) error

	ContainerList(ctx context.Context, options types.ContainerListOptions) ([]types.Container, error)
	ContainerCreate(
		ctx context.Context,
		config *container.Config,
		hostConfig *container.HostConfig,
		networkingConfig *network.NetworkingConfig,
		containerName string) (container.ContainerCreateCreatedBody, error)
	ContainerStart(ctx context.Context, containerID string, options types.ContainerStartOptions) error
	ContainerKill(ctx context.Context, containerID, signal string) error
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerAttach(
		ctx context.Context,
		container string,
		options types.ContainerAttachOptions,
	) (types.HijackedResponse, error)
	ContainerRemove(ctx context.Context, containerID string, options types.ContainerRemoveOptions) error
	ContainerWait(
		ctx context.Context,
		containerID string,
		condition container.WaitCondition,
	) (<-chan container.ContainerWaitOKBody, <-chan error)
	ContainerLogs(ctx context.Context, container string, options types.ContainerLogsOptions) (io.ReadCloser, error)
	ContainerExecCreate(ctx context.Context, container string, config types.ExecConfig) (types.IDResponse, error)
	ContainerExecAttach(ctx context.Context, execID string, config types.ExecStartCheck) (types.HijackedResponse, error)

	NetworkCreate(
		ctx context.Context,
		networkName string,
		options types.NetworkCreate,
	) (types.NetworkCreateResponse, error)
	NetworkRemove(ctx context.Context, networkID string) error
	NetworkDisconnect(ctx context.Context, networkID, containerID string, force bool) error
	NetworkList(ctx context.Context, options types.NetworkListOptions) ([]types.NetworkResource, error)
	NetworkInspect(ctx context.Context, networkID string) (types.NetworkResource, error)

	VolumeCreate(ctx context.Context, options volume.VolumeCreateBody) (types.Volume, error)
	VolumeRemove(ctx context.Context, volumeID string, force bool) error
	VolumeInspect(ctx context.Context, volumeID string) (types.Volume, error)

	Info(ctx context.Context) (types.Info, error)

	Close() error
}
