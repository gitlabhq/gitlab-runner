//go:build !integration

package runner_wrapper

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	pb "gitlab.com/gitlab-org/gitlab-runner/helpers/runner_wrapper/proto"
)

func TestServer_Listen(t *testing.T) {
	runWithServer(t, func(_ *testing.T, _ *mockWrapper, _ *Server) {
		time.Sleep(100 * time.Millisecond)
	})
}

func runWithServer(t *testing.T, run func(t *testing.T, w *mockWrapper, s *Server)) {
	w := newMockWrapper(t)
	s := NewServer(logrus.StandardLogger(), w)
	wg := new(sync.WaitGroup)

	go listenServer(t, wg, s)
	time.Sleep(100 * time.Millisecond)

	run(t, w, s)

	s.Stop()
	wg.Wait()
}

func listenServer(t *testing.T, wg *sync.WaitGroup, s *Server) {
	wg.Add(1)
	defer wg.Done()

	l, err := net.Listen("tcp", "127.0.0.1:11111")
	require.NoError(t, err)

	s.Listen(l)
}

func TestServer_CheckStatus(t *testing.T) {
	const (
		testFailureReason = "test failure reason"
	)

	tests := map[string]struct {
		status         Status
		expectedStatus pb.Status
	}{
		"mapped status": {
			status:         StatusRunning,
			expectedStatus: pb.Status_running,
		},
		"unknown status": {
			status:         Status(-1),
			expectedStatus: pb.Status_unknown,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			runWithServer(t, func(t *testing.T, w *mockWrapper, s *Server) {
				w.EXPECT().Status().Return(tc.status).Once()
				w.EXPECT().FailureReason().Return(testFailureReason).Once()

				resp, err := s.CheckStatus(context.Background(), new(pb.Empty))
				assert.NoError(t, err)

				assert.Equal(t, tc.expectedStatus, resp.Status)
				assert.Equal(t, testFailureReason, resp.FailureReason)
			})
		})
	}
}

func TestServer_InitGracefulShutdown(t *testing.T) {
	const (
		testFailureReason = "test failure reason"
		testURL           = "https://example.com"
		testMethod        = "test-method"
	)

	var (
		testHeaders = map[string]string{
			"Test-Header": "Test-Value",
		}
	)

	tests := map[string]struct {
		wrapperError error
		assertError  func(t *testing.T, err error)
	}{
		"no error": {},
		"processNotInitialized error": {
			wrapperError: errProcessNotInitialized,
		},
		"other errors": {
			wrapperError: assert.AnError,
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, assert.AnError)
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			runWithServer(t, func(t *testing.T, w *mockWrapper, s *Server) {
				w.EXPECT().Status().Return(StatusInShutdown).Once()
				w.EXPECT().FailureReason().Return(testFailureReason).Once()

				w.EXPECT().
					InitiateGracefulShutdown(mock.Anything).
					Return(tc.wrapperError).
					Once().
					Run(func(args mock.Arguments) {
						req, ok := args.Get(0).(initGracefulShutdownRequest)
						require.True(t, ok)

						def := req.ShutdownCallbackDef()
						require.NotNil(t, def)

						assert.Equal(t, testURL, def.URL())
						assert.Equal(t, testMethod, def.Method())
						assert.Equal(t, testHeaders, def.Headers())
					})

				resp, err := s.InitGracefulShutdown(context.Background(), &pb.InitGracefulShutdownRequest{
					ShutdownCallback: &pb.ShutdownCallback{
						Url:     testURL,
						Method:  testMethod,
						Headers: testHeaders,
					},
				})
				assert.Equal(t, pb.Status_in_shutdown, resp.Status)
				assert.Equal(t, testFailureReason, resp.FailureReason)

				if tc.assertError != nil {
					tc.assertError(t, err)
					return
				}

				assert.NoError(t, err)
			})
		})
	}
}
