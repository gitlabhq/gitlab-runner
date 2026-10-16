//go:build !integration

package user

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"strconv"
	"strings"
	"testing"

	dockerspec "github.com/moby/docker-image-spec/specs-go/v1"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/client"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

func TestDefaultInspect_IsRoot(t *testing.T) {
	containerID := "container-id"

	tests := map[string]struct {
		setupDockerClientMock func(t *testing.T, clientMock *docker.MockClient, expectedCtx context.Context)
		expectedIsRoot        bool
		expectedError         error
	}{
		"ImageInspectWithRaw error": {
			setupDockerClientMock: func(t *testing.T, clientMock *docker.MockClient, expectedCtx context.Context) {
				clientMock.On("ImageInspectWithRaw", expectedCtx, containerID, mock.Anything).
					Return(image.InspectResponse{}, nil, assert.AnError).
					Once()
			},
			expectedIsRoot: true,
			expectedError:  assert.AnError,
		},
		"empty Config": {
			setupDockerClientMock: func(t *testing.T, clientMock *docker.MockClient, expectedCtx context.Context) {
				clientMock.On("ImageInspectWithRaw", expectedCtx, containerID, mock.Anything).
					Return(image.InspectResponse{}, nil, nil).
					Once()
			},
			expectedIsRoot: true,
			expectedError:  nil,
		},
		"empty user entry in Config": {
			setupDockerClientMock: func(t *testing.T, clientMock *docker.MockClient, expectedCtx context.Context) {
				clientMock.On("ImageInspectWithRaw", expectedCtx, containerID, mock.Anything).
					Return(image.InspectResponse{Config: &dockerspec.DockerOCIImageConfig{ImageConfig: ocispec.ImageConfig{User: ""}}}, nil, nil).
					Once()
			},
			expectedIsRoot: true,
			expectedError:  nil,
		},
		"user entry in Config set to root": {
			setupDockerClientMock: func(t *testing.T, clientMock *docker.MockClient, expectedCtx context.Context) {
				clientMock.On("ImageInspectWithRaw", expectedCtx, containerID, mock.Anything).
					Return(image.InspectResponse{Config: &dockerspec.DockerOCIImageConfig{ImageConfig: ocispec.ImageConfig{User: "root"}}}, nil, nil).
					Once()
			},
			expectedIsRoot: true,
			expectedError:  nil,
		},
		"user entry in Config set to non-root": {
			setupDockerClientMock: func(t *testing.T, clientMock *docker.MockClient, expectedCtx context.Context) {
				clientMock.On("ImageInspectWithRaw", expectedCtx, containerID, mock.Anything).
					Return(image.InspectResponse{Config: &dockerspec.DockerOCIImageConfig{ImageConfig: ocispec.ImageConfig{User: "non-root"}}}, nil, nil).
					Once()
			},
			expectedIsRoot: false,
			expectedError:  nil,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			clientMock := docker.NewMockClient(t)

			ctx := t.Context()

			tt.setupDockerClientMock(t, clientMock, ctx)

			inspect := NewInspect(clientMock)
			isRoot, err := inspect.IsRoot(ctx, containerID)

			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedIsRoot, isRoot, "user root-status mismatch")
		})
	}
}

type uidAndGidTestCase struct {
	stdout        string
	stderr        string
	createErr     error
	attachErr     error
	inspectErr    error
	exitCode      int
	running       bool
	expectedID    int
	expectedError string
}

func dockerExecTestOutput(stdout, stderr string) *bufio.Reader {
	var output bytes.Buffer

	writeFrame := func(stream byte, content string) {
		if content == "" {
			return
		}
		header := make([]byte, 8)
		header[0] = stream
		binary.BigEndian.PutUint32(header[4:], uint32(len(content)))
		output.Write(header)
		output.WriteString(content)
	}

	writeFrame(1, stdout)
	writeFrame(2, stderr)

	return bufio.NewReader(bytes.NewReader(output.Bytes()))
}

func dockerExecTestConn(t *testing.T) net.Conn {
	t.Helper()
	conn, peer := net.Pipe()
	require.NoError(t, peer.Close())
	return conn
}

func TestDefaultInspect_UID(t *testing.T) {
	testDefaultInspectUIDandGID(t, commandIDU, func(inspect Inspect, ctx context.Context, containerID string) (int, error) {
		return inspect.UID(ctx, containerID)
	})
}

func TestDefaultInspect_GID(t *testing.T) {
	testDefaultInspectUIDandGID(t, commandIDG, func(inspect Inspect, ctx context.Context, containerID string) (int, error) {
		return inspect.GID(ctx, containerID)
	})
}

func testDefaultInspectUIDandGID(
	t *testing.T,
	expectedCommand string,
	testCall func(inspect Inspect, ctx context.Context, containerID string) (int, error),
) {
	containerID := "container-id"

	tests := map[string]uidAndGidTestCase{
		"exec create error": {
			createErr:     assert.AnError,
			expectedError: "create",
		},
		"exec attach error": {
			attachErr:     assert.AnError,
			expectedError: "attach",
		},
		"exec inspect error": {
			inspectErr:    assert.AnError,
			expectedError: "inspect",
		},
		"non-zero exit code": {
			exitCode:      1,
			expectedError: "exit",
		},
		"exec still running": {
			running:       true,
			expectedError: "running",
		},
		"ID parsing error": {
			stdout:        "\n\ntest\n\n",
			expectedError: "parse",
		},
		"stderr mixed with expected stdout output": {
			stdout:     "\n\n123\n\n",
			stderr:     "Some mixed error output",
			expectedID: 123,
		},
		"empty output of the id command": {
			stdout:        "\n\n\n\n",
			expectedError: "empty",
		},
		"proper ID received from output": {
			stdout:     "\n\n123\n\n",
			expectedID: 123,
		},
		"blank lines": {
			stdout:        "  \n  \n  ",
			expectedError: "empty",
		},
		"empty lines received at the end of the output": {
			stdout:     "1000 \n \n",
			expectedID: 1000,
		},
		"ID received at the end of the output": {
			stdout:     "\nHello world\n1000",
			expectedID: 1000,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			clientMock := docker.NewMockClient(t)
			ctx := t.Context()

			clientMock.On(
				"ContainerExecCreate",
				ctx,
				containerID,
				client.ExecCreateOptions{
					AttachStdout: true,
					AttachStderr: true,
					Cmd:          strings.Fields(expectedCommand),
				},
			).Return(container.ExecCreateResponse{ID: "exec-id"}, tt.createErr).Once()

			if tt.createErr == nil {
				if tt.attachErr != nil {
					clientMock.On(
						"ContainerExecAttach",
						ctx,
						"exec-id",
						client.ExecAttachOptions{},
					).Return(client.HijackedResponse{}, tt.attachErr).Once()
				} else {
					response := client.HijackedResponse{
						Conn:   dockerExecTestConn(t),
						Reader: dockerExecTestOutput(tt.stdout, tt.stderr),
					}
					clientMock.On(
						"ContainerExecAttach",
						ctx,
						"exec-id",
						client.ExecAttachOptions{},
					).Return(response, nil).Once()

					clientMock.On(
						"ContainerExecInspect",
						ctx,
						"exec-id",
					).Return(client.ExecInspectResult{Running: tt.running, ExitCode: tt.exitCode}, tt.inspectErr).Once()
				}
			}

			inspect := NewInspect(clientMock)
			id, err := testCall(inspect, ctx, containerID)

			assert.Equal(t, tt.expectedID, id)

			switch tt.expectedError {
			case "create", "attach", "inspect":
				assert.ErrorIs(t, err, assert.AnError)
			case "exit", "running":
				assert.Error(t, err)
			case "parse":
				var parseErr *strconv.NumError
				assert.ErrorAs(t, err, &parseErr)
			case "empty":
				assert.ErrorIs(t, err, errIDNoOutput)
			default:
				assert.NoError(t, err)
			}
		})
	}
}
