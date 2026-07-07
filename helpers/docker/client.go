package docker

import (
	"context"
	"io"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/network"
	system "github.com/moby/moby/api/types/system"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

// Client abstracts the Docker Engine SDK so the runner can insulate its
// executors from upstream API churn. Option and result structs come from the
// github.com/moby/moby/client package while the data types come from
// github.com/moby/moby/api/types/*. officialDockerClient adapts this interface
// to the SDK.
type Client interface {
	ClientVersion() string
	ServerVersion(context.Context) (client.ServerVersionResult, error)

	ImageInspectWithRaw(ctx context.Context, imageID string, platform *v1.Platform) (image.InspectResponse, []byte, error)

	ImagePullBlocking(ctx context.Context, ref string, options client.ImagePullOptions) error
	ImageImportBlocking(
		ctx context.Context,
		source client.ImageImportSource,
		ref string,
		options client.ImageImportOptions,
	) error
	ImageLoad(ctx context.Context, input io.Reader, quiet bool) (io.ReadCloser, error)
	ImageTag(ctx context.Context, source string, target string) error

	ContainerList(ctx context.Context, options client.ContainerListOptions) ([]container.Summary, error)
	ContainerCreate(
		ctx context.Context,
		config *container.Config,
		hostConfig *container.HostConfig,
		networkingConfig *network.NetworkingConfig,
		platform *v1.Platform,
		containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options client.ContainerStartOptions) error
	ContainerKill(ctx context.Context, containerID, signal string) error
	ContainerStop(ctx context.Context, containerID string, options client.ContainerStopOptions) error
	ContainerInspect(ctx context.Context, containerID string) (container.InspectResponse, error)
	ContainerAttach(
		ctx context.Context,
		container string,
		options client.ContainerAttachOptions,
	) (client.HijackedResponse, error)
	ContainerRemove(ctx context.Context, containerID string, options client.ContainerRemoveOptions) error
	ContainerWait(
		ctx context.Context,
		containerID string,
		condition container.WaitCondition,
	) (<-chan container.WaitResponse, <-chan error)
	ContainerLogs(ctx context.Context, container string, options client.ContainerLogsOptions) (io.ReadCloser, error)
	ContainerExecCreate(ctx context.Context, container string, config client.ExecCreateOptions) (container.ExecCreateResponse, error)
	ContainerExecAttach(ctx context.Context, execID string, config client.ExecAttachOptions) (client.HijackedResponse, error)

	NetworkCreate(
		ctx context.Context,
		networkName string,
		options client.NetworkCreateOptions,
	) (client.NetworkCreateResult, error)
	NetworkRemove(ctx context.Context, networkID string) error
	NetworkDisconnect(ctx context.Context, networkID, containerID string, force bool) error
	NetworkList(ctx context.Context, options client.NetworkListOptions) ([]network.Summary, error)
	NetworkInspect(ctx context.Context, networkID string) (network.Inspect, error)

	VolumeCreate(ctx context.Context, options client.VolumeCreateOptions) (volume.Volume, error)
	VolumeRemove(ctx context.Context, volumeID string, force bool) error
	VolumeInspect(ctx context.Context, volumeID string) (volume.Volume, error)
	VolumeList(ctx context.Context, options client.VolumeListOptions) (client.VolumeListResult, error)

	Info(ctx context.Context) (system.Info, error)

	Close() error
}
