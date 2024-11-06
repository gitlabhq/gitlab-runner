//go:build !integration

package runner_wrapper

import (
	"context"
	"syscall"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestWrapper_Run(t *testing.T) {
	const (
		testPath    = "test-path-to-binary"
		testTimeout = 100 * time.Millisecond
		testCtxVal  = "test-ctx-value"
	)

	type testKey int64

	var (
		testArgs   = []string{"test", "args", "--for", "binary"}
		testCtxKey = testKey(1)
	)

	tests := map[string]struct {
		mockProcess          func(t *testing.T) *mockProcess
		mockCommander        func(t *testing.T, m *mockCommander, p *mockProcess)
		mockShutdownCallback func(t *testing.T, w *Wrapper)
		assertFailureReason  func(t *testing.T, failureReason error)
		expectedStatus       Status
		assertError          func(t *testing.T, err error)
	}{
		"wrapped process start failure": {
			mockCommander: func(t *testing.T, m *mockCommander, _ *mockProcess) {
				m.EXPECT().Start().Return(assert.AnError).Once()
			},
			assertFailureReason: func(t *testing.T, failureReason error) {
				assert.ErrorIs(t, failureReason, errFailedToStartProcess)
				assert.Contains(t, failureReason.Error(), assert.AnError.Error())
			},
			expectedStatus: StatusStopped,
		},
		"immediate wrapped process failure": {
			mockCommander: func(t *testing.T, m *mockCommander, p *mockProcess) {
				m.EXPECT().Start().Return(nil).Once()
				m.EXPECT().Process().Return(p).Once()
				m.EXPECT().Wait().Return(assert.AnError).Once()
			},
			assertFailureReason: func(t *testing.T, failureReason error) {
				assert.ErrorIs(t, failureReason, assert.AnError)
			},
			expectedStatus: StatusStopped,
		},
		"wrapped process termination error": {
			mockProcess: func(t *testing.T) *mockProcess {
				p := newMockProcess(t)
				p.EXPECT().Signal(syscall.SIGTERM).Return(assert.AnError).Once()

				return p
			},
			mockCommander: func(t *testing.T, m *mockCommander, p *mockProcess) {
				m.EXPECT().Start().Return(nil).Once()
				m.EXPECT().Process().Return(p).Once()
				m.EXPECT().Wait().Return(nil).Once().Run(func(_ mock.Arguments) {
					time.Sleep(testTimeout * 5)
				})
			},
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, errFailedToTerminateProcess)
			},
			expectedStatus: StatusRunning,
		},
		"wrapped process terminated properly": {
			mockProcess: func(t *testing.T) *mockProcess {
				p := newMockProcess(t)

				return p
			},
			mockCommander: func(t *testing.T, m *mockCommander, p *mockProcess) {
				doneCh := make(chan struct{})

				p.EXPECT().Signal(syscall.SIGTERM).Return(nil).Once().Run(func(_ mock.Arguments) {
					close(doneCh)
				})

				m.EXPECT().Start().Return(nil).Once()
				m.EXPECT().Process().Return(p).Once()
				m.EXPECT().Wait().Return(nil).Once().Run(func(_ mock.Arguments) {
					select {
					case <-doneCh:
						return
					case <-time.After(testTimeout * 5):
						return
					}
				})
			},
			expectedStatus: StatusRunning,
		},
		"timeout when waiting for wrapped process termination": {
			mockProcess: func(t *testing.T) *mockProcess {
				p := newMockProcess(t)
				p.EXPECT().Signal(syscall.SIGTERM).Return(nil).Once()

				return p
			},
			mockCommander: func(t *testing.T, m *mockCommander, p *mockProcess) {
				m.EXPECT().Start().Return(nil).Once()
				m.EXPECT().Process().Return(p).Once()
				m.EXPECT().Wait().Return(nil).Once().Run(func(_ mock.Arguments) {
					time.Sleep(testTimeout * 10)
				})
			},
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, errProcessExitTimeout)
			},
			expectedStatus: StatusRunning,
		},
		"shutdown callback run on process graceful shutdown end": {
			mockCommander: func(t *testing.T, m *mockCommander, p *mockProcess) {
				m.EXPECT().Start().Return(nil).Once()
				m.EXPECT().Process().Return(p).Once()
				m.EXPECT().Wait().Return(nil).Once()
			},
			assertFailureReason: func(t *testing.T, failureReason error) {
				assert.NoError(t, failureReason)
			},
			expectedStatus: StatusStopped,
			mockShutdownCallback: func(t *testing.T, w *Wrapper) {
				m := newMockShutdownCallback(t)
				w.shutdownCallback = m

				m.EXPECT().Run(mock.Anything).Once().Run(func(args mock.Arguments) {
					ctx, ok := args.Get(0).(context.Context)
					require.True(t, ok, "first argument must be of context.Context type")

					assert.Equal(t, testCtxVal, ctx.Value(testCtxKey))
				})
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			var p *mockProcess
			if tc.mockProcess != nil {
				p = tc.mockProcess(t)
			}

			commanderMock := newMockCommander(t)
			if tc.mockCommander != nil {
				tc.mockCommander(t, commanderMock, p)
			}

			ctx, cancelFn := context.WithTimeout(
				context.WithValue(context.Background(), testCtxKey, testCtxVal),
				testTimeout,
			)
			defer cancelFn()

			w := New(logrus.StandardLogger(), testPath, testArgs)
			w.SetTerminationTimeout(10 * time.Millisecond)

			w.commanderFactory = func(path string, args []string) commander {
				assert.Equal(t, testPath, path)
				assert.Equal(t, testArgs, args)
				return commanderMock
			}

			if tc.mockShutdownCallback != nil {
				tc.mockShutdownCallback(t, w)
			}

			err := w.Run(ctx)

			assert.Equal(t, tc.expectedStatus, w.status)
			if tc.assertFailureReason != nil {
				tc.assertFailureReason(t, w.failureReason)
			}

			if tc.assertError != nil {
				tc.assertError(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestWrapper_Status(t *testing.T) {
	const testStatus = StatusInShutdown

	w := &Wrapper{
		log:    logrus.StandardLogger(),
		status: testStatus,
	}

	assert.Equal(t, testStatus, w.Status())
}

func TestWrapper_FailureReason(t *testing.T) {
	tests := map[string]error{
		"failure reason exists":          assert.AnError,
		"failure reason does not exists": nil,
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			w := &Wrapper{
				log:           logrus.StandardLogger(),
				failureReason: tc,
			}

			if tc == nil {
				assert.Empty(t, w.FailureReason())
				return
			}

			assert.Equal(t, tc.Error(), w.FailureReason())
		})
	}
}

func TestWrapper_InitiateGracefulShutdown(t *testing.T) {
	const (
		testShutdownCallbackURL    = "https://example.com"
		testShutdownCallbackMethod = "POST"
	)
	var (
		testShutdownCallbackHeaders = map[string]string{
			"Test-Header": "Test-Value",
		}
	)

	tests := map[string]struct {
		process                func(t *testing.T) *mockProcess
		shutdownCallbackURL    string
		processKillerError     error
		assertError            func(t *testing.T, err error)
		assertShutdownCallback func(t *testing.T, w *Wrapper)
	}{
		"no process": {
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, errProcessNotInitialized)
			},
		},
		"process killer error": {
			process: func(t *testing.T) *mockProcess {
				p := newMockProcess(t)
				p.EXPECT().Signal(gracefulShutdownSignal).Return(assert.AnError).Once()

				return p
			},
			processKillerError: assert.AnError,
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
		"processed properly with empty shutdown callback URL": {
			process: func(t *testing.T) *mockProcess {
				p := newMockProcess(t)
				p.EXPECT().Signal(gracefulShutdownSignal).Return(nil).Once()

				return p
			},
			assertShutdownCallback: func(t *testing.T, w *Wrapper) {
				assert.Nil(t, w.shutdownCallback)
			},
		},
		"processed properly with existing shutdown callback URL": {
			process: func(t *testing.T) *mockProcess {
				p := newMockProcess(t)
				p.EXPECT().Signal(gracefulShutdownSignal).Return(nil).Once()

				return p
			},
			shutdownCallbackURL: testShutdownCallbackURL,
			assertShutdownCallback: func(t *testing.T, w *Wrapper) {
				callback, ok := w.shutdownCallback.(*defaultShutdownCallback)
				require.True(t, ok)
				assert.Equal(t, testShutdownCallbackURL, callback.url)
				assert.Equal(t, testShutdownCallbackMethod, callback.method)
				assert.Equal(t, testShutdownCallbackHeaders, callback.headers)
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			w := New(logrus.StandardLogger(), "", []string{})

			if tc.process != nil {
				w.process = tc.process(t)
			}

			assert.Equal(t, StatusUnknown, w.status)

			def := newMockShutdownCallbackDef(t)
			def.EXPECT().URL().Return(tc.shutdownCallbackURL).Maybe()
			def.EXPECT().Method().Return(testShutdownCallbackMethod).Maybe()
			def.EXPECT().Headers().Return(testShutdownCallbackHeaders).Maybe()

			req := newMockInitGracefulShutdownRequest(t)
			req.EXPECT().ShutdownCallbackDef().Return(def).Maybe()
			err := w.InitiateGracefulShutdown(req)

			if tc.assertError != nil {
				tc.assertError(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, StatusInShutdown, w.status)

			if tc.assertShutdownCallback != nil {
				tc.assertShutdownCallback(t, w)
			}
		})
	}
}
