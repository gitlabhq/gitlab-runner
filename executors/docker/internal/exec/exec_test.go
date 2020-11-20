package exec

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/wait"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

func TestDefaultDocker_Exec(t *testing.T) {
	id := "container-id"

	input := func(err error) io.Reader {
		r := new(mockReader)
		r.On("Read", mock.Anything).
			Return(0, err).
			Once()

		return r
	}

	mockWorkingClient := func(
		clientMock *docker.MockClient,
		reader io.Reader,
		expectedCtx context.Context,
	) {
		conn := new(mockConn)
		conn.On("Close").Return(nil).Once()
		conn.On("Write", mock.Anything).Return(0, nil)

		hijacked := types.HijackedResponse{
			Conn:   conn,
			Reader: bufio.NewReader(reader),
		}

		clientMock.On("ContainerAttach", expectedCtx, id, attachOptions()).
			Return(hijacked, nil).
			Once()
		clientMock.On("ContainerStart", expectedCtx, id, types.ContainerStartOptions{}).
			Return(nil).
			Once()
	}

	tests := map[string]struct {
		input             io.Reader
		cancelContext     bool
		setupDockerClient func(t *testing.T, clientMock *docker.MockClient, expectedCtx context.Context)
		setupKillWaiter   func(t *testing.T, waiterMock *wait.MockKillWaiter, expectedCtx context.Context)
		assertLogOutput   func(t *testing.T, logOutput string)
		expectedError     error
	}{
		"ContainerAttach error": {
			cancelContext: false,
			setupDockerClient: func(t *testing.T, clientMock *docker.MockClient, expectedCtx context.Context) {
				clientMock.On("ContainerAttach", expectedCtx, id, attachOptions()).
					Return(types.HijackedResponse{}, assert.AnError).
					Once()
			},
			setupKillWaiter: func(t *testing.T, waiterMock *wait.MockKillWaiter, expectedCtx context.Context) {},
			assertLogOutput: func(t *testing.T, logOutput string) {},
			expectedError:   assert.AnError,
		},
		"ContainerStart error": {
			cancelContext: false,
			setupDockerClient: func(t *testing.T, clientMock *docker.MockClient, expectedCtx context.Context) {
				conn := new(mockConn)
				conn.On("Close").Return(nil).Once()
				conn.On("Write", mock.Anything).Return(0, nil)

				hijacked := types.HijackedResponse{
					Conn: conn,
				}

				clientMock.On("ContainerAttach", expectedCtx, id, attachOptions()).
					Return(hijacked, nil).
					Once()
				clientMock.On("ContainerStart", expectedCtx, id, types.ContainerStartOptions{}).
					Return(assert.AnError).
					Once()
			},
			setupKillWaiter: func(t *testing.T, waiterMock *wait.MockKillWaiter, expectedCtx context.Context) {},
			assertLogOutput: func(t *testing.T, logOutput string) {},
			expectedError:   assert.AnError,
		},
		"context done": {
			input:         input(io.EOF),
			cancelContext: true,
			setupDockerClient: func(t *testing.T, clientMock *docker.MockClient, expectedCtx context.Context) {
				reader := new(mockReader)
				reader.On("Read", mock.Anything).
					Return(0, nil)

				mockWorkingClient(clientMock, reader, expectedCtx)
			},
			setupKillWaiter: func(t *testing.T, waiterMock *wait.MockKillWaiter, expectedCtx context.Context) {
				waiterMock.On("KillWait", expectedCtx, id).Return(nil).Once()
			},
			assertLogOutput: func(t *testing.T, logOutput string) {
				assert.Contains(t, logOutput, "finished with aborted")
			},
			expectedError: nil,
		},
		"input error": {
			input:         input(errors.New("input error")),
			cancelContext: false,
			setupDockerClient: func(t *testing.T, clientMock *docker.MockClient, expectedCtx context.Context) {
				reader := new(mockReader)
				reader.On("Read", mock.Anything).
					Return(0, nil)

				mockWorkingClient(clientMock, reader, expectedCtx)
			},
			setupKillWaiter: func(t *testing.T, waiterMock *wait.MockKillWaiter, expectedCtx context.Context) {
				waiterMock.On("KillWait", expectedCtx, id).Return(nil).Once()
			},
			assertLogOutput: func(t *testing.T, logOutput string) {
				assert.Contains(t, logOutput, "finished with input error")
			},
			expectedError: nil,
		},
		"output error": {
			input:         input(io.EOF),
			cancelContext: false,
			setupDockerClient: func(t *testing.T, clientMock *docker.MockClient, expectedCtx context.Context) {
				reader := new(mockReader)
				reader.On("Read", mock.Anything).
					Return(0, errors.New("output error"))

				mockWorkingClient(clientMock, reader, expectedCtx)
			},
			setupKillWaiter: func(t *testing.T, waiterMock *wait.MockKillWaiter, expectedCtx context.Context) {
				waiterMock.On("KillWait", expectedCtx, id).Return(nil).Once()
			},
			assertLogOutput: func(t *testing.T, logOutput string) {
				assert.Contains(t, logOutput, "finished with output error")
			},
			expectedError: nil,
		},
		"killWaiter error": {
			input:         input(io.EOF),
			cancelContext: false,
			setupDockerClient: func(t *testing.T, clientMock *docker.MockClient, expectedCtx context.Context) {
				reader := new(mockReader)
				reader.On("Read", mock.Anything).
					Return(0, io.EOF).
					Once()

				mockWorkingClient(clientMock, reader, expectedCtx)
			},
			setupKillWaiter: func(t *testing.T, waiterMock *wait.MockKillWaiter, expectedCtx context.Context) {
				waiterMock.On("KillWait", expectedCtx, id).Return(assert.AnError).Once()
			},
			assertLogOutput: func(t *testing.T, logOutput string) {},
			expectedError:   assert.AnError,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			clientMock := new(docker.MockClient)
			defer clientMock.AssertExpectations(t)

			waiterMock := new(wait.MockKillWaiter)
			defer waiterMock.AssertExpectations(t)

			logger, hook := test.NewNullLogger()
			logger.SetLevel(logrus.DebugLevel)

			ctx, cancelFn := context.WithCancel(context.Background())
			defer cancelFn()

			out := new(bytes.Buffer)

			tt.setupDockerClient(t, clientMock, ctx)
			tt.setupKillWaiter(t, waiterMock, ctx)

			if tt.cancelContext {
				cancelFn()
			}

			dockerExec := NewDocker(clientMock, waiterMock, logger)
			err := dockerExec.Exec(ctx, id, tt.input, out)

			logOutput := ""
			for _, entry := range hook.AllEntries() {
				line, e := entry.String()
				require.NoError(t, e)
				logOutput += line
			}

			tt.assertLogOutput(t, logOutput)

			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
		})
	}
}
