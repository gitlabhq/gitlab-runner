package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/omitwriter"
)

const bootstrappedBinary = "/opt/gitlab-runner/gitlab-runner-helper"

type conn struct {
	resp    types.HijackedResponse
	reader  *io.PipeReader
	cleanup func()
}

func (c *conn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}

func (c *conn) Write(p []byte) (int, error) {
	return c.resp.Conn.Write(p)
}

func (c *conn) Close() error {
	err := c.reader.Close()
	_ = c.resp.Conn.Close()
	c.cleanup()

	return err
}

//nolint:gocognit
func (s *commandExecutor) Connect(ctx context.Context) (func() (io.ReadWriteCloser, error), error) {
	ctr, err := s.requestBuildContainer()
	if err != nil {
		return nil, fmt.Errorf("creating build container for dialer: %w", err)
	}

	// initCtx is used for starting the container and waiting until it is ready, we're okay
	// to cancel on defer, because the closure returned does not rely on it. The closure only
	// relies on the container having started, and if that changes between now and the closure
	// being executed, that will safely result in a timeout during the connection.
	initCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stdout := s.BuildLogger.Stream(buildlogger.StreamWorkLevel, buildlogger.Stdout)
	defer stdout.Close()

	stderr := s.BuildLogger.Stream(buildlogger.StreamWorkLevel, buildlogger.Stderr)
	defer stderr.Close()

	hijacked, err := s.dockerConn.ContainerAttach(initCtx, ctr.ID, container.AttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("connect container attach: %w", err)
	}
	defer hijacked.Close()

	okCh, errCh := s.dockerConn.ContainerWait(initCtx, ctr.ID, container.WaitConditionNextExit)

	if err := s.dockerConn.ContainerStart(initCtx, ctr.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("connect container start: %w", err)
	}

	readyWriter, readyCh := newReadyWriter(initCtx, stderr)

	// stream container log to job
	go func() {
		_, _ = stdcopy.StdCopy(stdout, readyWriter, hijacked.Reader)
	}()

	select {
	case <-readyCh:
		// success
	case err := <-errCh:
		return nil, fmt.Errorf("connect container wait: %w", err)
	case result := <-okCh:
		if result.Error != nil {
			return nil, fmt.Errorf("connect container exit: %s", result.Error.Message)
		}
		return nil, fmt.Errorf("connect container exited unexpectedly with code %d", result.StatusCode)
	case <-time.After(time.Minute):
		return nil, fmt.Errorf("connect container ready timeout")
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return func() (io.ReadWriteCloser, error) {
		resp, err := s.dockerConn.ContainerExecCreate(ctx, ctr.ID, container.ExecOptions{
			Cmd:          []string{bootstrappedBinary, "steps", "proxy"},
			AttachStdin:  true,
			AttachStderr: true,
			AttachStdout: true,
		})
		if err != nil {
			return nil, err
		}

		hijacked, err := s.dockerConn.ContainerExecAttach(ctx, resp.ID, container.ExecAttachOptions{})
		if err != nil {
			return nil, err
		}

		r, w := io.Pipe()
		go func() {
			stderr := omitwriter.New()
			_, err := stdcopy.StdCopy(w, stderr, hijacked.Reader)
			if err != nil {
				err = fmt.Errorf("%w: %w", err, stderr.Error())
			}
			w.CloseWithError(err)
		}()

		return &conn{
			resp:   hijacked,
			reader: r,
			cleanup: func() {
				if err := s.dockerConn.ContainerStop(s.Context, ctr.ID, container.StopOptions{}); err != nil {
					s.BuildLogger.Errorln("Stopping steps container", err)
				}
			},
		}, nil
	}, nil
}

func (e *executor) bootstrap() error {
	if !e.Build.UseNativeSteps() {
		return nil
	}

	e.SetCurrentStage(ExecutorStageBootstrap)
	e.BuildLogger.Debugln("Creating bootstrap volume...")

	ctx, cancel := context.WithCancel(e.Context)
	defer cancel()

	if err := e.volumesManager.CreateTemporary(ctx, path.Dir(bootstrappedBinary)); err != nil {
		return fmt.Errorf("bootstrap volume: %w", err)
	}

	helperImage, err := e.getHelperImage()
	if err != nil {
		return fmt.Errorf("bootstrap helper image: %w", err)
	}

	containerConfig := &container.Config{
		Image:           helperImage.ID,
		Cmd:             []string{"gitlab-runner-helper", "steps", "bootstrap", bootstrappedBinary},
		Tty:             false,
		AttachStdin:     false,
		AttachStdout:    true,
		AttachStderr:    true,
		OpenStdin:       false,
		StdinOnce:       true,
		NetworkDisabled: true,
	}
	hostConfig := &container.HostConfig{
		AutoRemove:     true,
		ReadonlyRootfs: true, // todo: windows doesn't support read-only fs
		RestartPolicy:  neverRestartPolicy,
		Binds:          e.volumesManager.Binds(),
		NetworkMode:    network.NetworkNone,
		Runtime:        e.Config.Docker.Runtime,
		Isolation:      container.Isolation(e.Config.Docker.Isolation),
	}

	bootstrapContainer, err := e.dockerConn.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return fmt.Errorf("bootstrap container create: %w", err)
	}
	defer func() {
		_ = e.dockerConn.ContainerRemove(ctx, bootstrapContainer.ID, container.RemoveOptions{
			RemoveVolumes: true,
			Force:         true,
		})
	}()

	hijacked, err := e.dockerConn.ContainerAttach(ctx, bootstrapContainer.ID, container.AttachOptions{
		Stream: true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return fmt.Errorf("bootstrap container attach: %w", err)
	}
	defer hijacked.Close()

	okCh, errCh := e.dockerConn.ContainerWait(ctx, bootstrapContainer.ID, container.WaitConditionNextExit)

	if err := e.dockerConn.ContainerStart(ctx, bootstrapContainer.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("bootstrap container start: %w", err)
	}

	select {
	case err := <-errCh:
		buf := new(bytes.Buffer)
		_, _ = stdcopy.StdCopy(buf, buf, io.LimitReader(hijacked.Reader, 1024))

		return fmt.Errorf("bootstrap container wait: %w (%v)", err, buf.String())

	case ok := <-okCh:
		if ok.StatusCode != 0 {
			buf := new(bytes.Buffer)
			_, _ = stdcopy.StdCopy(buf, buf, io.LimitReader(hijacked.Reader, 1024))

			// detect if this helper is too old to support the functions subcommand
			if strings.Contains(buf.String(), "Command steps not found") {
				return fmt.Errorf("helper does not contain CI Steps support: please upgrade your version of the GitLab Runner helper binary")
			}
			return fmt.Errorf("bootstrap container non zero exit: %v (%v) %v", ok.Error, ok.StatusCode, buf.String())
		}
	}

	return nil
}

var readyMarker = []byte("step-runner is ready.\n")

// readyWriter signals on a channel when readyMarker has been written to the
// writer and is used to detect that the step-runner is ready for connections.
type readyWriter struct {
	io.Writer
	ctx     context.Context
	ready   chan struct{}
	matched int
	once    sync.Once
}

func newReadyWriter(ctx context.Context, w io.Writer) (io.Writer, <-chan struct{}) {
	ch := make(chan struct{})
	rw := &readyWriter{Writer: w, ctx: ctx, ready: ch}
	go func() {
		<-ctx.Done()
		rw.close()
	}()
	return rw, ch
}

func (rw *readyWriter) close() {
	rw.once.Do(func() { close(rw.ready) })
}

func (rw *readyWriter) Write(p []byte) (int, error) {
	n, err := rw.Writer.Write(p)
	if rw.matched < 0 || n == 0 {
		return n, err
	}

	data := p[:n]
	if rw.matched == 0 {
		if idx := bytes.IndexByte(data, readyMarker[0]); idx >= 0 {
			data = data[idx:]
		} else {
			return n, err
		}
	}

	for _, b := range data {
		switch b {
		case readyMarker[rw.matched]:
			rw.matched++
			if rw.matched == len(readyMarker) {
				rw.close()
				rw.matched = -1
				return n, err
			}
		case readyMarker[0]:
			rw.matched = 1
		default:
			rw.matched = 0
		}
	}
	return n, err
}
