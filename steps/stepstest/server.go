// Package stepstest provides an in-process fake step-runner gRPC service for
// unit tests that drive the steps.Connector / steps.Execute flow without a
// real step-runner subprocess.
package stepstest

import (
	"context"
	"io"
	"sync"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/timestamppb"

	"gitlab.com/gitlab-org/step-runner/proto"
)

// bufSize is generous enough that gRPC framing never blocks on the in-memory
// listener for the tiny request/response sizes these tests exchange.
const bufSize = 1 << 16

// Server is a minimal proto.StepRunnerServer suitable for unit tests. It
// records Cancel-RPC job IDs, accepts Run/Close as no-ops, and unblocks
// FollowLogs (and flips Status to cancelled) once Cancel is invoked. This is
// the smallest surface that lets steps.Execute reach its post-Run code path.
type Server struct {
	proto.UnimplementedStepRunnerServer

	listener *bufconn.Listener
	server   *grpc.Server

	mu        sync.Mutex
	cancels   []string
	cancelled chan struct{}
}

// New starts the fake on an in-memory bufconn listener and stops it via
// t.Cleanup. The returned Server is ready to use.
func New(t *testing.T) *Server {
	t.Helper()

	s := &Server{
		listener:  bufconn.Listen(bufSize),
		cancelled: make(chan struct{}),
	}
	s.server = grpc.NewServer()
	proto.RegisterStepRunnerServer(s.server, s)

	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		_ = s.server.Serve(s.listener)
	}()

	t.Cleanup(func() {
		s.server.Stop()
		<-serveDone
	})

	return s
}

// Cancels returns a snapshot of the job IDs the server has received Cancel
// RPCs for, in arrival order.
func (s *Server) Cancels() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.cancels...)
}

// Connector returns a value implementing steps.Connector that dials this
// in-process server. The returned Connector can be passed directly into
// steps.Options or steps.Execute.
func (s *Server) Connector() *Connector {
	return &Connector{listener: s.listener}
}

// Run accepts the request and returns immediately; the job is treated as
// "running" until Cancel is invoked.
func (s *Server) Run(_ context.Context, _ *proto.RunRequest) (*proto.RunResponse, error) {
	return &proto.RunResponse{}, nil
}

// Cancel records the job ID and unblocks any in-flight FollowLogs streams.
// Repeated calls are recorded but only signal once.
func (s *Server) Cancel(_ context.Context, req *proto.CancelRequest) (*proto.CancelResponse, error) {
	s.mu.Lock()
	first := len(s.cancels) == 0
	s.cancels = append(s.cancels, req.Id)
	s.mu.Unlock()
	if first {
		close(s.cancelled)
	}
	return &proto.CancelResponse{}, nil
}

// FollowLogs streams nothing and returns when the job is cancelled or the
// stream context is done — emulating step-runner shutting the log stream
// after a graceful cancel.
func (s *Server) FollowLogs(_ *proto.FollowLogsRequest, srv grpc.ServerStreamingServer[proto.FollowLogsResponse]) error {
	select {
	case <-s.cancelled:
		return nil
	case <-srv.Context().Done():
		return srv.Context().Err()
	}
}

// Status returns proto.StepResult_cancelled / StatusError_cancelled once the
// job has been cancelled, so that extended.RunAndFollow's post-Follow Status
// query produces the same result step-runner emits after a graceful cancel.
func (s *Server) Status(_ context.Context, req *proto.StatusRequest) (*proto.StatusResponse, error) {
	select {
	case <-s.cancelled:
		return &proto.StatusResponse{Jobs: []*proto.Status{{
			Id:        req.Id,
			Status:    proto.StepResult_cancelled,
			StartTime: timestamppb.Now(),
			EndTime:   timestamppb.Now(),
			Error: &proto.StatusError{
				Kind:    proto.StatusError_cancelled,
				Message: "cancelled",
			},
		}}}, nil
	default:
		return &proto.StatusResponse{Jobs: []*proto.Status{{
			Id:        req.Id,
			Status:    proto.StepResult_running,
			StartTime: timestamppb.Now(),
		}}}, nil
	}
}

// Close acknowledges the request without doing anything; the fake holds no
// resources tied to a job ID.
func (s *Server) Close(_ context.Context, _ *proto.CloseRequest) (*proto.CloseResponse, error) {
	return &proto.CloseResponse{}, nil
}

// Connector dials the in-process bufconn listener. It satisfies
// steps.Connector structurally without importing the steps package.
type Connector struct {
	listener *bufconn.Listener
}

// Connect returns a dialFn that produces a single connection to the fake
// server. The signature matches steps.Connector.Connect exactly.
func (c *Connector) Connect(ctx context.Context) (func() (io.ReadWriteCloser, error), error) {
	return func() (io.ReadWriteCloser, error) {
		return c.listener.DialContext(ctx)
	}, nil
}
