package shell

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	runnersteps "gitlab.com/gitlab-org/gitlab-runner/steps"
	"gitlab.com/gitlab-org/gitlab-runner/steps/localserver"
)

// shell is a step-runner connector, and its provider is managed so the shared
// server is shut down with the runner.
var (
	_ runnersteps.Connector          = (*executor)(nil)
	_ common.ManagedExecutorProvider = (*provider)(nil)
)

// provider owns the per-runtime step-runner server that concrete jobs dispatch
// to. The runner calls Shutdown on termination (see commands/multi.go) to stop
// it.
type provider struct {
	executors.DefaultExecutorProvider
	stepRunner *stepRunnerServer
}

// Init is a no-op; the step-runner starts lazily on first Connect.
func (p *provider) Init() {}

func (p *provider) Shutdown(ctx context.Context, _ *common.Config) {
	p.stepRunner.shutdown(ctx)
}

// Connect implements steps.Connector. It lazily starts the shared in-process
// step-runner and returns a dialer to its unix socket. The dialer captures only
// the socket path and returns a fresh connection per call.
func (s *executor) Connect(_ context.Context) (func() (io.ReadWriteCloser, error), error) {
	if s.stepRunner == nil {
		// Only an executor built outside NewProvider reaches this.
		return nil, fmt.Errorf("shell executor was not constructed with a step-runner")
	}

	sockPath, err := s.stepRunner.ensureStarted()
	if err != nil {
		return nil, fmt.Errorf("starting step-runner: %w", err)
	}

	return func() (io.ReadWriteCloser, error) {
		conn, err := net.Dial("unix", sockPath)
		if err != nil {
			return nil, fmt.Errorf("dialing step-runner socket %q: %w", sockPath, err)
		}
		return conn, nil
	}, nil
}

// stepRunnerServer is the lifecycle wrapper around the in-process step-runner
// (steps/localserver) shared by every shell build. Concurrent builds are
// isolated by step-runner's per-job request id, not by separate processes.
type stepRunnerServer struct {
	logger logrus.FieldLogger

	// operatorLog receives step-runner's operator-facing logs, bridged into
	// the runner's log at debug level. Built once and reused across respawns;
	// the logrus pipe writer behind it lives for the runner's lifetime.
	operatorLog *slog.Logger

	mu  sync.Mutex
	srv *localserver.Server
}

func newStepRunnerServer() *stepRunnerServer {
	logger := logrus.WithField("subsystem", "shell-step-runner")
	return &stepRunnerServer{
		logger:      logger,
		operatorLog: slog.New(slog.NewTextHandler(logger.WriterLevel(logrus.DebugLevel), nil)),
	}
}

// ensureStarted lazily launches the server and returns its socket path. Startup
// failures are not cached (the next Connect retries); a server that has exited
// unexpectedly is logged and respawned.
func (m *stepRunnerServer) ensureStarted() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.srv != nil {
		select {
		case err := <-m.srv.Done():
			m.logger.WithError(err).Warn("shared step-runner exited unexpectedly; respawning")
			m.srv.RemoveSocketDir()
			m.srv = nil
		default:
			return m.srv.SockPath, nil
		}
	}

	// The lock is held across this blocking call intentionally, so the server
	// is spawned once and concurrent first-Connects coalesce onto it.
	srv, err := localserver.Start(localserver.Options{OperatorLogger: m.operatorLog})
	if err != nil {
		return "", err
	}

	m.srv = srv
	return srv.SockPath, nil
}

// shutdown gracefully stops the server (if running) and removes its socket
// directory, bounded by ctx.
func (m *stepRunnerServer) shutdown(ctx context.Context) {
	m.mu.Lock()
	srv := m.srv
	m.srv = nil
	m.mu.Unlock()

	if srv == nil {
		return
	}

	srv.Stop()

	// Drain in-flight jobs, bounded by the runner's shutdown timeout.
	select {
	case err := <-srv.Done():
		if err != nil {
			m.logger.WithError(err).Debug("shared step-runner stopped")
		}
	case <-ctx.Done():
		m.logger.Warn("timed out waiting for step-runner to drain on shutdown")
	}

	srv.RemoveSocketDir()
}
