package docker_helpers

import "github.com/stretchr/testify/mock"

import "io"
import "github.com/docker/docker/api/types"
import "github.com/docker/docker/api/types/container"
import "github.com/docker/docker/api/types/network"
import "golang.org/x/net/context"

type MockClient struct {
	mock.Mock
}

func (m *MockClient) ImageInspectWithRaw(ctx context.Context, imageID string) (types.ImageInspect, []byte, error) {
	ret := m.Called(ctx, imageID)

	r0 := ret.Get(0).(types.ImageInspect)
	var r1 []byte
	if ret.Get(1) != nil {
		r1 = ret.Get(1).([]byte)
	}
	r2 := ret.Error(2)

	return r0, r1, r2
}
func (m *MockClient) ImagePullBlocking(ctx context.Context, ref string, options types.ImagePullOptions) error {
	ret := m.Called(ctx, ref, options)

	r0 := ret.Error(0)

	return r0
}
func (m *MockClient) ImageImportBlocking(ctx context.Context, source types.ImageImportSource, ref string, options types.ImageImportOptions) error {
	ret := m.Called(ctx, source, ref, options)

	r0 := ret.Error(0)

	return r0
}
func (m *MockClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (container.ContainerCreateCreatedBody, error) {
	ret := m.Called(ctx, config, hostConfig, networkingConfig, containerName)

	r0 := ret.Get(0).(container.ContainerCreateCreatedBody)
	r1 := ret.Error(1)

	return r0, r1
}
func (m *MockClient) ContainerStart(ctx context.Context, containerID string, options types.ContainerStartOptions) error {
	ret := m.Called(ctx, containerID, options)

	r0 := ret.Error(0)

	return r0
}
func (m *MockClient) ContainerWait(ctx context.Context, containerID string) (int64, error) {
	ret := m.Called(ctx, containerID)

	r0 := ret.Get(0).(int64)
	r1 := ret.Error(1)

	return r0, r1
}
func (m *MockClient) ContainerKill(ctx context.Context, containerID string, signal string) error {
	ret := m.Called(ctx, containerID, signal)

	r0 := ret.Error(0)

	return r0
}
func (m *MockClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	ret := m.Called(ctx, containerID)

	r0 := ret.Get(0).(types.ContainerJSON)
	r1 := ret.Error(1)

	return r0, r1
}
func (m *MockClient) ContainerAttach(ctx context.Context, container string, options types.ContainerAttachOptions) (types.HijackedResponse, error) {
	ret := m.Called(ctx, container, options)

	r0 := ret.Get(0).(types.HijackedResponse)
	r1 := ret.Error(1)

	return r0, r1
}
func (m *MockClient) ContainerRemove(ctx context.Context, containerID string, options types.ContainerRemoveOptions) error {
	ret := m.Called(ctx, containerID, options)

	r0 := ret.Error(0)

	return r0
}
func (m *MockClient) ContainerLogs(ctx context.Context, container string, options types.ContainerLogsOptions) (io.ReadCloser, error) {
	ret := m.Called(ctx, container, options)

	r0 := ret.Get(0).(io.ReadCloser)
	r1 := ret.Error(1)

	return r0, r1
}
func (m *MockClient) NetworkDisconnect(ctx context.Context, networkID string, containerID string, force bool) error {
	ret := m.Called(ctx, networkID, containerID, force)

	r0 := ret.Error(0)

	return r0
}
func (m *MockClient) NetworkList(ctx context.Context, options types.NetworkListOptions) ([]types.NetworkResource, error) {
	ret := m.Called(ctx, options)

	var r0 []types.NetworkResource
	if ret.Get(0) != nil {
		r0 = ret.Get(0).([]types.NetworkResource)
	}
	r1 := ret.Error(1)

	return r0, r1
}
func (m *MockClient) Info(ctx context.Context) (types.Info, error) {
	ret := m.Called(ctx)

	r0 := ret.Get(0).(types.Info)
	r1 := ret.Error(1)

	return r0, r1
}
func (m *MockClient) Close() error {
	ret := m.Called()

	r0 := ret.Error(0)

	return r0
}
