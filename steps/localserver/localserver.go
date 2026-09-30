// Package localserver runs an in-process step-runner gRPC server on a
// generated unix socket. It serves the shell executor's shared server and
// the helper's one-shot 'steps run' command. Only the concrete builtin is
// registered; the helper's long-lived 'steps serve' command remains separate.
package localserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete"
	concreterun "gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run"
	"gitlab.com/gitlab-org/step-runner/pkg/api"
	"gitlab.com/gitlab-org/step-runner/pkg/di"
	"gitlab.com/gitlab-org/step-runner/proto"
)

// startTimeout bounds how long Start waits for the server to become ready to
// serve RPCs before giving up. It covers the whole of serve's setup: bind,
// step-runner construction and service registration.
const startTimeout = 30 * time.Second

// dirPrefix names the generated temp directory that holds the socket. A
// generated path (not step-runner's well-known default) lets multiple runners
// and a future step-runner daemon coexist on one host.
const dirPrefix = "gitlab-runner-steps"

// maxSockPathLen is the longest socket path we accept before falling back to
// a shorter temp root. sun_path is 104 bytes on darwin and 108 on Linux
// (including the trailing NUL); binding beyond it fails with EINVAL. 100
// leaves headroom on both.
const maxSockPathLen = 100

// Options configures the server.
type Options struct {
	// OperatorLogger receives step-runner's operator-facing logs (e.g. job
	// failures). Leave nil to keep step-runner's default (process stderr).
	// Hosts whose stderr is the job trace should pass a discard logger; the
	// runner process should bridge into its own log.
	OperatorLogger *slog.Logger
}

// Server is a running in-process step-runner.
type Server struct {
	// SockPath is the unix socket the server accepts connections on.
	SockPath string

	cancel context.CancelFunc
	done   chan error
}

// Start launches the server and blocks until it is ready to serve RPCs. The
// server's lifetime is bound to Stop, not any caller context: it may outlive
// the build that started it.
func Start(opts Options) (*Server, error) {
	dir, err := os.MkdirTemp("", dirPrefix)
	if err != nil {
		return nil, fmt.Errorf("creating step-runner socket dir: %w", err)
	}
	sockPath := filepath.Join(dir, "s.sock")

	// A deep $TMPDIR (e.g. the custom executor exports its own temp dir under
	// macOS's already-long /var/folders path) can push the socket path past
	// the sun_path limit, failing bind with EINVAL. Fall back to /tmp.
	if len(sockPath) > maxSockPathLen {
		_ = os.RemoveAll(dir)
		dir, err = os.MkdirTemp("/tmp", dirPrefix)
		if err != nil {
			return nil, fmt.Errorf("creating step-runner socket dir under /tmp: %w", err)
		}
		sockPath = filepath.Join(dir, "s.sock")
	}

	ctx, cancel := context.WithCancel(context.Background())

	ready := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- serve(ctx, dir, sockPath, ready, opts) }()

	if err := waitForReady(sockPath, ready, done); err != nil {
		// serve owns dir and removes it on every exit path, including the
		// one this cancel triggers. Removing it here too would race serve's
		// own use of the directory when the timeout fires mid-setup.
		cancel()
		return nil, err
	}

	// Register the socket for concrete's nested run: dial-back, without
	// touching the process environment, which child processes would inherit.
	// Registered here, once the server is confirmed accepting, rather than in
	// the serve goroutine. Process global: the last Start wins, matching the
	// single active server per process that callers maintain.
	concreterun.SetSocketPath(sockPath)

	return &Server{SockPath: sockPath, cancel: cancel, done: done}, nil
}

// Dial opens a gRPC client connection to the server. It uses a context dialer
// so that gRPC never parses SockPath as a URI — on Windows, absolute paths
// contain a drive-letter colon (e.g. C:\...) that confuses gRPC's URI parser
// and produces "too many colons in address". The dummy target is never
// resolved; all connections go through the dialer.
func (s *Server) Dial() (*grpc.ClientConn, error) {
	sockPath := s.SockPath
	return grpc.NewClient("passthrough:///localserver",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "unix", sockPath)
		}),
	)
}

// Stop begins a graceful stop (in-flight jobs drain). It does not wait; the
// serve result arrives on Done. Idempotent.
func (s *Server) Stop() {
	s.cancel()
}

// Done receives the serve result exactly once, when the server exits. The
// generated socket directory is already removed by then.
func (s *Server) Done() <-chan error {
	return s.done
}

// serve runs the step-runner gRPC server on sockPath until ctx is cancelled,
// closing ready once it is about to accept RPCs. It owns dir: sole ownership
// keeps cleanup off Start's timeout path, which would otherwise remove the
// directory out from under a serve still setting up in it.
func serve(ctx context.Context, dir, sockPath string, ready chan<- struct{}, opts Options) error {
	defer os.RemoveAll(dir)

	listener, err := net.ListenUnix("unix", api.SocketAddr(sockPath))
	if err != nil {
		return fmt.Errorf("opening step-runner socket: %w", err)
	}
	defer listener.Close()

	diOpts := []func(*di.Container){
		di.WithBuiltinFunc("concrete", concrete.New()),
	}
	if opts.OperatorLogger != nil {
		diOpts = append(diOpts, di.WithOperatorLogger(opts.OperatorLogger))
	}

	service, err := di.NewContainer(diOpts...).StepRunnerService()
	if err != nil {
		return fmt.Errorf("initializing step-runner: %w", err)
	}

	srv := grpc.NewServer()
	proto.RegisterStepRunnerServer(srv, service)

	go func() {
		<-ctx.Done()
		srv.GracefulStop()
	}()

	// Ready means "about to accept RPCs", not merely "socket bound": the
	// kernel accepts connections into the listen backlog from the moment of
	// bind, so a dial succeeding says nothing about the service being
	// registered. Signal it here, once bind, construction and registration
	// have all succeeded, and Start's wait is exact rather than a poll.
	close(ready)

	if err := srv.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return fmt.Errorf("step-runner server: %w", err)
	}
	return nil
}

// waitForReady blocks until the server signals it is about to accept RPCs,
// the server exits, or the start timeout elapses.
func waitForReady(sockPath string, ready <-chan struct{}, done <-chan error) error {
	deadline := time.NewTimer(startTimeout)
	defer deadline.Stop()

	select {
	case <-ready:
		return nil
	case err := <-done:
		return fmt.Errorf("step-runner serve exited during startup: %w", err)
	case <-deadline.C:
		return fmt.Errorf("timed out waiting for step-runner to serve on %s", sockPath)
	}
}
