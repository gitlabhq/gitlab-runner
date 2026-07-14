// Package localserver runs an in-process step-runner gRPC server on a
// generated unix socket. It serves the shell executor's shared server and
// the helper's one-shot 'steps run' command. Only the concrete builtin is
// registered; the helper's long-lived 'steps serve' command remains separate.
package localserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"

	"gitlab.com/gitlab-org/gitlab-runner/functions/concrete"
	concreterun "gitlab.com/gitlab-org/gitlab-runner/functions/concrete/run"
	"gitlab.com/gitlab-org/step-runner/pkg/api"
	"gitlab.com/gitlab-org/step-runner/pkg/di"
	"gitlab.com/gitlab-org/step-runner/proto"
)

// startTimeout bounds how long Start waits for the server to accept
// connections before giving up.
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

	dir    string
	cancel context.CancelFunc
	done   chan error
}

// Start launches the server and blocks until it accepts connections. The
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

	done := make(chan error, 1)
	go func() { done <- serve(ctx, sockPath, opts) }()

	if err := waitForListening(sockPath, done); err != nil {
		cancel()
		_ = os.RemoveAll(dir)
		return nil, err
	}

	// Register the socket for concrete's nested run: dial-back, without
	// touching the process environment, which child processes would inherit.
	// Registered here, once the server is confirmed accepting, rather than in
	// the serve goroutine. Process global: the last Start wins, matching the
	// single active server per process that callers maintain.
	concreterun.SetSocketPath(sockPath)

	return &Server{SockPath: sockPath, dir: dir, cancel: cancel, done: done}, nil
}

// Stop begins a graceful stop (in-flight jobs drain). It does not wait; the
// serve result arrives on Done. Idempotent.
func (s *Server) Stop() {
	s.cancel()
}

// Done receives the serve result exactly once, when the server exits.
func (s *Server) Done() <-chan error {
	return s.done
}

// RemoveSocketDir deletes the generated socket directory. Call it only after
// the server has exited.
func (s *Server) RemoveSocketDir() {
	_ = os.RemoveAll(s.dir)
}

// serve runs the step-runner gRPC server on sockPath until ctx is cancelled.
func serve(ctx context.Context, sockPath string, opts Options) error {
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

	if err := srv.Serve(listener); err != nil {
		return fmt.Errorf("step-runner server: %w", err)
	}
	return nil
}

// waitForListening polls the socket until the server accepts a connection,
// the server exits, or the start timeout elapses.
func waitForListening(sockPath string, done <-chan error) error {
	deadline := time.NewTimer(startTimeout)
	defer deadline.Stop()

	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()

	for {
		if conn, derr := net.Dial("unix", sockPath); derr == nil {
			_ = conn.Close()
			return nil
		}

		select {
		case err := <-done:
			return fmt.Errorf("step-runner serve exited during startup: %w", err)
		case <-deadline.C:
			return fmt.Errorf("timed out waiting for step-runner to listen on %s", sockPath)
		case <-ticker.C:
		}
	}
}
