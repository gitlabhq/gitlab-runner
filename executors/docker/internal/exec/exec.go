package exec

import (
	"context"
	"errors"
	"io"
	"net"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/wait"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

// conn is an interface wrapper used to generate mocks that are next used for tests
// nolint:deadcode
type conn interface {
	net.Conn
}

// reader is an interface wrapper used to generate mocks that are next used for tests
// nolint:deadcode
type reader interface {
	io.Reader
}

type Docker interface {
	Exec(ctx context.Context, containerID string, input io.Reader, output io.Writer) error
}

func NewDocker(c docker.Client, waiter wait.KillWaiter, logger logrus.FieldLogger) Docker {
	return &defaultDocker{
		c:      c,
		waiter: waiter,
		logger: logger,
	}
}

type defaultDocker struct {
	c      docker.Client
	waiter wait.KillWaiter
	logger logrus.FieldLogger
}

func (d *defaultDocker) Exec(ctx context.Context, containerID string, input io.Reader, output io.Writer) error {
	d.logger.Debugln("Attaching to container", containerID, "...")

	hijacked, err := d.c.ContainerAttach(ctx, containerID, attachOptions())
	if err != nil {
		return err
	}
	defer hijacked.Close()

	d.logger.Debugln("Starting container", containerID, "...")
	err = d.c.ContainerStart(ctx, containerID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	// Copy any output to the build trace
	stdoutErrCh := make(chan error)
	go func() {
		_, errCopy := stdcopy.StdCopy(output, output, hijacked.Reader)
		stdoutErrCh <- errCopy
	}()

	// Write the input to the container and close its STDIN to get it to finish
	stdinErrCh := make(chan error)
	go func() {
		_, errCopy := io.Copy(hijacked.Conn, input)
		_ = hijacked.CloseWrite()
		if errCopy != nil {
			stdinErrCh <- errCopy
		}
	}()

	// Wait until either:
	// - the job is aborted/cancelled/deadline exceeded
	// - stdin has an error
	// - stdout returns an error or nil, indicating the stream has ended and
	//   the container has exited
	select {
	case <-ctx.Done():
		err = errors.New("aborted")
	case err = <-stdinErrCh:
	case err = <-stdoutErrCh:
	}

	if err != nil {
		d.logger.Debugln("Container", containerID, "finished with", err)
	}

	// Kill and wait for exit.
	// Containers are stopped so that they can be reused by the job.
	return d.waiter.KillWait(ctx, containerID)
}

func attachOptions() types.ContainerAttachOptions {
	return types.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	}
}
