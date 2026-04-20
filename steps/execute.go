package steps

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	grpcstatus "google.golang.org/grpc/status"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/step-runner/pkg/api/client"
	"gitlab.com/gitlab-org/step-runner/pkg/api/client/extended"
	"gitlab.com/gitlab-org/step-runner/schema/v1"
)

type Connector interface {
	Connect(ctx context.Context) (func() (io.ReadWriteCloser, error), error)
}

type JobInfo struct {
	ID         int64
	Timeout    time.Duration
	ProjectDir string
	Variables  spec.Variables
}

type ClientStatusError struct {
	Status client.Status
	Err    error
}

func (cserr *ClientStatusError) Error() string {
	return cserr.Err.Error()
}

func (cserr *ClientStatusError) Is(err error) bool {
	cserr2, ok := err.(*ClientStatusError)
	if !ok {
		return false
	}

	return cserr.Status.State == cserr2.Status.State
}

func (cserr *ClientStatusError) Unwrap() error {
	return cserr.Err
}

// ClientInternalError signals a step-runner client failure that is not tied
// to a job Status — specifically, a gRPC handler panic surfacing as
// codes.Internal via step-runner's panic-recovery interceptor. Distinct from
// ClientStatusError, which reports a Status returned by step-runner.
type ClientInternalError struct {
	Err error
}

func (e *ClientInternalError) Error() string { return e.Err.Error() }
func (e *ClientInternalError) Unwrap() error { return e.Err }

func Execute(ctx context.Context, connector Connector, jobInfo JobInfo, steps []schema.Step, trace io.Writer) error {
	dialFn, err := connector.Connect(ctx)
	if err != nil {
		return fmt.Errorf("creating connect dialer: %w", err)
	}

	dialer := &stdioDialer{dialFn: dialFn}
	c, err := extended.New(dialer)
	if err != nil {
		return fmt.Errorf("creating steps client: %w", err)
	}
	//nolint:errcheck
	defer c.CloseConn()

	out := extended.FollowOutput{Logs: trace}

	request, err := NewRequest(jobInfo, steps)
	if err != nil {
		return fmt.Errorf("creating steps request: %w", err)
	}

	status, err := c.RunAndFollow(ctx, request, &out)
	if err != nil {
		wrapped := fmt.Errorf("executing steps request: %w", err)
		if grpcstatus.Code(err) == codes.Internal {
			return &ClientInternalError{Err: wrapped}
		}
		return wrapped
	}

	if status.State == client.StateSuccess {
		return nil
	}

	return &ClientStatusError{Status: status, Err: errors.New(status.Message)}
}

type stdioDialer struct {
	dialFn func() (io.ReadWriteCloser, error)
}

func (d *stdioDialer) Dial() (*grpc.ClientConn, error) {
	return grpc.NewClient("unix:step-runner",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			rwc, err := d.dialFn()
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
