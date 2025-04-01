package exec

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/sirupsen/logrus"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/step-runner/pkg/api"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type dialerFactory func(io.ReadCloser, io.Writer) func(context.Context) error

// proxyConn dials and provides a io.Reader and io.Writer to dialerFn and returns
// a connected net.Conn implementation.
//
// It can be used to connect code expecting a net.Conn with code expecting an
// io.Reader and io.Writer.
func proxyConn(ctx context.Context, ctxDialerFactory dialerFactory) (net.Conn, error) {
	connReader, w := io.Pipe()
	r, connWriter := io.Pipe()

	go func() {
		err := ctxDialerFactory(r, w)(ctx)
		if err != nil {
			err = fmt.Errorf("dialing step-runner client dialer: %w", err)
			r.CloseWithError(err)
			w.CloseWithError(err)
		}
	}()

	return &rwConn{WriteCloser: connWriter, ReadCloser: connReader}, nil
}

type tunnelingDialer struct {
	containerID string
	client      docker.Client
	logger      logrus.FieldLogger
}

func (td *tunnelingDialer) Dial() (*grpc.ClientConn, error) {
	ctxDialer := func(ctx context.Context, _ string) (net.Conn, error) {
		dialerFactory := func(source io.ReadCloser, sink io.Writer) func(context.Context) error {
			return func(ctx context.Context) error {
				return td.containerExec(ctx, source, sink)
			}
		}
		return proxyConn(ctx, dialerFactory)
	}

	conn, err := grpc.NewClient("unix:"+api.DefaultSocketPath(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(ctxDialer))
	if err != nil {
		return nil, fmt.Errorf("creating gRPC client: %w", err)
	}
	return conn, nil
}

func (td *tunnelingDialer) containerExec(ctx context.Context, source io.ReadCloser, sink io.Writer) error {
	execCreateResp, err := td.client.ContainerExecCreate(ctx, td.containerID, types.ExecConfig{
		Cmd:          []string{"step-runner", "proxy"},
		AttachStdin:  true,
		AttachStderr: true,
		AttachStdout: true,
	})
	if err != nil {
		return fmt.Errorf("creating container exec for %q: %w", td.containerID, err)
	}

	hijacked, err := td.client.ContainerExecAttach(ctx, execCreateResp.ID, types.ExecStartCheck{})
	if err != nil {
		return fmt.Errorf("attaching to container exec for %q: %w", td.containerID, err)
	}

	td.logger.Debugln("exec attached to container", td.containerID)

	eg := errgroup.Group{}

	eg.Go(func() error {
		//nolint:errcheck
		defer hijacked.Close()
		_, err := io.Copy(hijacked.Conn, source)
		if err != nil {
			return fmt.Errorf("streaming into container %q: %w", td.containerID, err)
		}
		return nil
	})

	eg.Go(func() error {
		//nolint:errcheck
		defer hijacked.Close()
		stderr := newOmitWriter()
		_, err = stdcopy.StdCopy(sink, stderr, hijacked.Reader)
		if err != nil {
			return fmt.Errorf("streaming out of container %q: %w (%w)", td.containerID, err, stderr.Error())
		}
		return nil
	})

	return eg.Wait()
}
