package docker

import (
	"context"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	system "github.com/docker/docker/api/types/system"
	"github.com/docker/docker/api/types/volume"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

//go:generate mockery --name=Client --inpackage
type Client interface {
	ClientVersion() string
	ServerVersion(context.Context) (types.Version, error)

	ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error)

	ImagePullBlocking(ctx context.Context, ref string, options types.ImagePullOptions) error
	ImageImportBlocking(
		ctx context.Context,
		source types.ImageImportSource,
		ref string,
		options types.ImageImportOptions,
	) error
	ImageLoad(ctx context.Context, input io.Reader, quiet bool) (types.ImageLoadResponse, error)
	ImageTag(ctx context.Context, source string, target string) error

	ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	ContainerCreate(
		ctx context.Context,
		config *container.Config,
		hostConfig *container.HostConfig,
		networkingConfig *network.NetworkingConfig,
		platform *v1.Platform,
		containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerKill(ctx context.Context, containerID, signal string) error
	ContainerStop(ctx context.Context, containerID string, opions container.StopOptions) error
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerAttach(
		ctx context.Context,
		container string,
		options container.AttachOptions,
	) (types.HijackedResponse, error)
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerWait(
		ctx context.Context,
		containerID string,
		condition container.WaitCondition,
	) (<-chan container.WaitResponse, <-chan error)
	ContainerLogs(ctx context.Context, container string, options container.LogsOptions) (io.ReadCloser, error)
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

	VolumeCreate(ctx context.Context, options volume.CreateOptions) (volume.Volume, error)
	VolumeRemove(ctx context.Context, volumeID string, force bool) error
	VolumeInspect(ctx context.Context, volumeID string) (volume.Volume, error)

	Info(ctx context.Context) (system.Info, error)

	Close() error
}
