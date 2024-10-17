package runner_wrapper

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

type Status int64

const (
	StatusUnknown Status = iota
	StatusRunning
	StatusInShutdown
	StatusStopped
)

const (
	DefaultTerminationTimeout = 10 * time.Second
)

var (
	errProcessNotInitialized    = fmt.Errorf("process not initialized")
	errFailedToStartProcess     = fmt.Errorf("failed to start process")
	errFailedToTerminateProcess = fmt.Errorf("could not send SIGTERM")
	errProcessExitTimeout       = fmt.Errorf("timed out waiting for process to exit")
)

//go:generate mockery --name=initGracefulShutdownRequest --inpackage --with-expecter
type initGracefulShutdownRequest interface {
	ShutdownCallbackDef() shutdownCallbackDef
}

type commanderFactory func(path string, args []string) commander

type Wrapper struct {
	log logrus.FieldLogger

	path string
	args []string

	errCh   chan error
	lock    sync.RWMutex
	process process

	terminationTimeout time.Duration

	commanderFactory commanderFactory

	status           Status
	failureReason    error
	shutdownCallback shutdownCallback
}

func New(log logrus.FieldLogger, path string, args []string) *Wrapper {
	return &Wrapper{
		log:                log,
		path:               path,
		args:               args,
		errCh:              make(chan error),
		terminationTimeout: DefaultTerminationTimeout,
		status:             StatusUnknown,
		commanderFactory:   newDefaultCommander,
	}
}

func (w *Wrapper) SetTerminationTimeout(timeout time.Duration) {
	w.terminationTimeout = timeout
}

func (w *Wrapper) Run(ctx context.Context) error {
	go w.start()

	return w.wait(ctx)
}

func (w *Wrapper) start() {
	cmd := w.commanderFactory(w.path, w.args)

	w.log.
		WithField("path", w.path).
		WithField("args", w.args).
		Debug("Starting process")

	err := cmd.Start()
	if err != nil {
		w.errCh <- fmt.Errorf("%w: %v", errFailedToStartProcess, err)
		return
	}

	w.setProcess(cmd.Process())
	w.setStatus(StatusRunning)

	w.errCh <- cmd.Wait()
}

func (w *Wrapper) setProcess(process process) {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.process = process
}

func (w *Wrapper) setStatus(status Status) {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.status = status
}

func (w *Wrapper) wait(ctx context.Context) error {
	for {
		select {
		case err := <-w.errCh:
			w.handleWrappedProcessShutdown(ctx, err)

		case <-ctx.Done():
			return w.terminateWrapper()
		}
	}
}

func (w *Wrapper) handleWrappedProcessShutdown(ctx context.Context, err error) {
	if err != nil {
		w.setFailureReason(err)
	}

	w.setProcess(nil)
	w.setStatus(StatusStopped)

	go w.sendShutdownCallback(ctx)
}

func (w *Wrapper) setFailureReason(err error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.failureReason = err
}

func (w *Wrapper) sendShutdownCallback(ctx context.Context) {
	w.lock.Lock()
	c := w.shutdownCallback
	w.lock.Unlock()

	if c == nil {
		w.log.Info("No shutdown callback registered; skipping")
		return
	}

	c.Run(ctx)
}

func (w *Wrapper) terminateWrapper() error {
	w.log.Info("Shutting down wrapper process...")

	err := w.terminateWrappedProcess()
	if err != nil {
		if errors.Is(err, errProcessNotInitialized) {
			return nil
		}
		return err
	}

	select {
	case err := <-w.errCh:
		w.log.WithError(err).Info("Wrapped application exited")

		return nil

	case <-time.After(w.terminationTimeout):
		return errProcessExitTimeout
	}
}

func (w *Wrapper) terminateWrappedProcess() error {
	w.lock.RLock()
	p := w.process
	w.lock.RUnlock()

	if p == nil {
		w.log.Info("No process to shutdown; exiting")

		return errProcessNotInitialized
	}

	err := p.Signal(syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("%w: %v", errFailedToTerminateProcess, err)
	}

	return nil
}

func (w *Wrapper) Status() Status {
	w.lock.RLock()
	defer w.lock.RUnlock()

	w.log.WithField("status", w.status).Debug("Checking process status")

	return w.status
}

func (w *Wrapper) FailureReason() string {
	w.lock.RLock()
	defer w.lock.RUnlock()

	w.log.WithError(w.failureReason).Debug("Checking process failure reason")

	if w.failureReason == nil {
		return ""
	}

	return w.failureReason.Error()
}

func (w *Wrapper) InitiateGracefulShutdown(req initGracefulShutdownRequest) error {
	w.lock.RLock()
	p := w.process
	w.lock.RUnlock()

	if p == nil {
		return errProcessNotInitialized
	}

	w.log.Info("Initiating graceful shutdown of the process")

	err := p.Signal(gracefulShutdownSignal)
	if err != nil {
		return fmt.Errorf("could not send graceful shutdown signal: %w", err)
	}

	if req.ShutdownCallbackDef().URL() != "" {
		w.log.
			WithField("target", req.ShutdownCallbackDef().URL()).
			WithField("method", req.ShutdownCallbackDef().Method()).
			Debug("Registering shutdown callback")

		w.setShutdownCallback(newShutdownCallback(w.log, req.ShutdownCallbackDef()))
	}

	w.setStatus(StatusInShutdown)

	return nil
}

func (w *Wrapper) setShutdownCallback(callback shutdownCallback) {
	w.lock.Lock()
	defer w.lock.Unlock()

	w.shutdownCallback = callback
}
