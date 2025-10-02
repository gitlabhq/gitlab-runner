package steps

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/step-runner/pkg/api/client"
	"gitlab.com/gitlab-org/step-runner/pkg/api/client/extended"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func Execute(ctx context.Context, connector common.Connector, build *common.Build, trace common.JobTrace) error {
	dialer := &stdioDialer{connector: connector}
	client, err := extended.New(dialer)
	if err != nil {
		return fmt.Errorf("creating steps client: %w", err)
	}
	//nolint:errcheck
	defer client.CloseConn()

	out := extended.FollowOutput{Logs: trace}

	request, err := NewRequest(build)
	if err != nil {
		return fmt.Errorf("creating steps request: %w", err)
	}

	status, err := client.RunAndFollow(ctx, request, &out)
	if err != nil {
		return fmt.Errorf("executing steps request: %w", err)
	}

	return errFromStatus(status)
}

func errFromStatus(status client.Status) error {
	berr := &common.BuildError{Inner: errors.New(status.Message)}

	switch status.State {
	case client.StateSuccess:
		return nil
	case client.StateUnspecified:
		berr.FailureReason = common.UnknownFailure
	case client.StateFailure:
		berr.FailureReason = common.ScriptFailure
	case client.StateRunning:
		// this should not happen!!!
	case client.StateCancelled:
		// nothing to do here since there is no CancelledFailure
	}

	// TODO: also set berr.ExitCode if we add an exit-code to client.Status

	return berr
}

type stdioDialer struct {
	connector common.Connector
}

func (d *stdioDialer) Dial() (*grpc.ClientConn, error) {
	return grpc.NewClient("unix:step-runner",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			rwc, err := d.connector.Connect(ctx)
			if err != nil {
				return nil, err
			}

			return &stdioConn{rwc}, nil
		}),
	)
}

type stdioConn struct {
	io.ReadWriteCloser
}

func (conn *stdioConn) Close() error {
	return conn.ReadWriteCloser.Close()
}

func (*stdioConn) LocalAddr() net.Addr                { return addr{} }
func (*stdioConn) RemoteAddr() net.Addr               { return addr{} }
func (*stdioConn) SetDeadline(t time.Time) error      { return fmt.Errorf("unsupported") }
func (*stdioConn) SetReadDeadline(t time.Time) error  { return fmt.Errorf("unsupported") }
func (*stdioConn) SetWriteDeadline(t time.Time) error { return fmt.Errorf("unsupported") }

type addr struct{}

func (addr) Network() string { return "stdio.Conn" }
func (addr) String() string  { return "stdio.Conn" }
