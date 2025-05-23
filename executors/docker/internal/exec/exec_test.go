//go:build !integration

package exec

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
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

	input := func(t *testing.T, err error) io.Reader {
		r := newMockReader(t)
		r.On("Read", mock.Anything).
			Return(0, err).
			Maybe()

		return r
	}

	mockWorkingClient := func(
		t *testing.T,
		clientMock *docker.MockClient,
		reader io.Reader,
		expectedCtx context.Context,
	) {
		conn := newMockConn(t)
		conn.On("Close").Return(nil).Maybe()
		conn.On("Write", mock.Anything).Return(0, nil).Maybe()

		hijacked := types.HijackedResponse{
			Conn:   conn,
			Reader: bufio.NewReader(reader),
		}

		clientMock.On("ContainerAttach", expectedCtx, id, attachOptions()).
			Return(hijacked, nil).
			Once()
		clientMock.On("ContainerStart", expectedCtx, id, container.StartOptions{}).
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
		expectedStdOut    string
		expectedStdErr    string
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
				conn := newMockConn(t)
				conn.On("Close").Return(nil).Once()

				hijacked := types.HijackedResponse{
					Conn: conn,
				}

				clientMock.On("ContainerAttach", expectedCtx, id, attachOptions()).
					Return(hijacked, nil).
					Once()
				clientMock.On("ContainerStart", expectedCtx, id, container.StartOptions{}).
					Return(assert.AnError).
					Once()
			},
			setupKillWaiter: func(t *testing.T, waiterMock *wait.MockKillWaiter, expectedCtx context.Context) {},
			assertLogOutput: func(t *testing.T, logOutput string) {},
			expectedError:   assert.AnError,
		},
		"context done": {
			input:         input(t, io.EOF),
			cancelContext: true,
			setupDockerClient: func(t *testing.T, clientMock *docker.MockClient, expectedCtx context.Context) {
				reader := newMockReader(t)
				reader.On("Read", mock.Anything).
					Return(0, nil).Maybe()

				mockWorkingClient(t, clientMock, reader, expectedCtx)
			},
			setupKillWaiter: func(t *testing.T, waiterMock *wait.MockKillWaiter, expectedCtx context.Context) {
				waiterMock.On("StopKillWait", expectedCtx, id, mock.Anything, mock.AnythingOfType("wait.GracefulExitFunc")).Return(nil).Once()
			},
			assertLogOutput: func(t *testing.T, logOutput string) {
				assert.Contains(t, logOutput, "finished with aborted")
			},
			expectedError: nil,
		},
		"input error": {
			input:         input(t, errors.New("input error")),
			cancelContext: false,
			setupDockerClient: func(t *testing.T, clientMock *docker.MockClient, expectedCtx context.Context) {
				reader := newMockReader(t)
				reader.On("Read", mock.Anything).
					Return(0, nil)

				mockWorkingClient(t, clientMock, reader, expectedCtx)
			},
			setupKillWaiter: func(t *testing.T, waiterMock *wait.MockKillWaiter, expectedCtx context.Context) {
				waiterMock.On("StopKillWait", expectedCtx, id, mock.Anything, mock.AnythingOfType("wait.GracefulExitFunc")).Return(nil).Once()
			},
			assertLogOutput: func(t *testing.T, logOutput string) {
				assert.Contains(t, logOutput, "finished with input error")
			},
			expectedError: nil,
		},
		"output error": {
			input:         input(t, io.EOF),
			cancelContext: false,
			setupDockerClient: func(t *testing.T, clientMock *docker.MockClient, expectedCtx context.Context) {
				reader := newMockReader(t)
				reader.On("Read", mock.Anything).
					Return(0, errors.New("output error"))

				mockWorkingClient(t, clientMock, reader, expectedCtx)
			},
			setupKillWaiter: func(t *testing.T, waiterMock *wait.MockKillWaiter, expectedCtx context.Context) {
				waiterMock.On("StopKillWait", expectedCtx, id, mock.Anything, mock.AnythingOfType("wait.GracefulExitFunc")).Return(nil).Once()
			},
			assertLogOutput: func(t *testing.T, logOutput string) {
				assert.Contains(t, logOutput, "finished with output error")
			},
			expectedError: nil,
		},
		"killWaiter error": {
			input:         input(t, io.EOF),
			cancelContext: false,
			setupDockerClient: func(t *testing.T, clientMock *docker.MockClient, expectedCtx context.Context) {
				reader := newMockReader(t)
				reader.On("Read", mock.Anything).
					Return(0, io.EOF).
					Once()

				mockWorkingClient(t, clientMock, reader, expectedCtx)
			},
			setupKillWaiter: func(t *testing.T, waiterMock *wait.MockKillWaiter, expectedCtx context.Context) {
				waiterMock.On("StopKillWait", expectedCtx, id, mock.Anything, mock.AnythingOfType("wait.GracefulExitFunc")).Return(assert.AnError).Once()
			},
			assertLogOutput: func(t *testing.T, logOutput string) {},
			expectedError:   assert.AnError,
		},
		"output passed to the writers": {
			input:         input(t, io.EOF),
			cancelContext: false,
			setupDockerClient: func(t *testing.T, clientMock *docker.MockClient, expectedCtx context.Context) {
				pr, pw := io.Pipe()

				outWriter := stdcopy.NewStdWriter(pw, stdcopy.Stdout)
				errWriter := stdcopy.NewStdWriter(pw, stdcopy.Stderr)

				var wg sync.WaitGroup
				t.Cleanup(wg.Wait)
				wg.Add(1)
				go func() {
					defer wg.Done()

					var err error
					_, err = fmt.Fprintln(outWriter, "out line 1")
					require.NoError(t, err)
					_, err = fmt.Fprintln(errWriter, "err line 1")
					require.NoError(t, err)
					_, err = fmt.Fprintln(outWriter, "out line 2")
					require.NoError(t, err)
					_, err = fmt.Fprintln(errWriter, "err line 2")
					require.NoError(t, err)
					err = pw.Close()
					require.NoError(t, err)
				}()

				mockWorkingClient(t, clientMock, pr, expectedCtx)
			},
			setupKillWaiter: func(t *testing.T, waiterMock *wait.MockKillWaiter, expectedCtx context.Context) {
				waiterMock.On("StopKillWait", expectedCtx, id, mock.Anything, mock.AnythingOfType("wait.GracefulExitFunc")).Return(nil).Once()
			},
			assertLogOutput: func(t *testing.T, logOutput string) {},
			expectedError:   nil,
			expectedStdOut:  "out line 1\nout line 2\n",
			expectedStdErr:  "err line 1\nerr line 2\n",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			clientMock := docker.NewMockClient(t)
			waiterMock := wait.NewMockKillWaiter(t)

			logger, hook := test.NewNullLogger()
			logger.SetLevel(logrus.DebugLevel)

			executorCtx, executorCancelFn := context.WithCancel(context.Background())
			defer executorCancelFn()

			ctx, cancelFn := context.WithCancel(executorCtx)
			defer cancelFn()

			outBuf := new(bytes.Buffer)
			errBuf := new(bytes.Buffer)

			tt.setupDockerClient(t, clientMock, ctx)
			tt.setupKillWaiter(t, waiterMock, executorCtx)

			if tt.cancelContext {
				cancelFn()
			}

			streams := IOStreams{
				Stdin:  tt.input,
				Stdout: outBuf,
				Stderr: errBuf,
			}

			dockerExec := NewDocker(executorCtx, clientMock, waiterMock, logger)
			err := dockerExec.Exec(ctx, id, streams, nil)

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

			assert.Equal(t, tt.expectedStdOut, outBuf.String())
			assert.Equal(t, tt.expectedStdErr, errBuf.String())
		})
	}
}
