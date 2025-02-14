package client

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/runner_wrapper/api"
	pb "gitlab.com/gitlab-org/gitlab-runner/helpers/runner_wrapper/api/proto"
)

const (
	DefaultConnectTimeout = 5 * time.Second
)

type Dialer func(network string, address string) (net.Conn, error)

type Client struct {
	logger     *slog.Logger
	grpcConn   *grpc.ClientConn
	grpcClient pb.ProcessWrapperClient
}

func New(target string, opts ...Option) (*Client, error) {
	o := setupOptions(opts)

	logger := o.logger.WithGroup("client").With("target", target)

	grpcOpts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithContextDialer(func(_ context.Context, addr string) (net.Conn, error) {
			network, address := parseDialTarget(addr)
			logger.Debug("dialing gRPC server", "network", network, "address", address)

			return o.dialer(network, address)
		}),
	}

	conn, err := grpc.NewClient(target, grpcOpts...)
	if err != nil {
		return nil, fmt.Errorf("creating gRPC client: %w", err)
	}

	c := &Client{
		logger:     logger,
		grpcConn:   conn,
		grpcClient: pb.NewProcessWrapperClient(conn),
	}

	return c, nil
}

func (c *Client) Connect(ctx context.Context) error {
	return c.ConnectWithTimeout(ctx, DefaultConnectTimeout)
}

func (c *Client) ConnectWithTimeout(ctx context.Context, timeout time.Duration) error {
	c.logger.Debug("connecting to gRPC server")

	c.grpcConn.Connect()

	err := RetryWithBackoff(ctx, timeout, func() error {
		state := c.grpcConn.GetState()
		if state != connectivity.Ready {
			return fmt.Errorf("gRPC connection is not ready: %s", state)
		}

		return nil
	})

	if err != nil {
		c.logger.Warn("gRPC connection failure", "error", err)

		return err
	}

	c.logger.Debug("gRPC connection succeeded")

	return nil
}

type CheckStatusResponse struct {
	Status        api.Status
	FailureReason string
}

func (c *Client) CheckStatus(ctx context.Context) (CheckStatusResponse, error) {
	c.logger.Info("Checking status")

	var resp CheckStatusResponse

	s, err := c.grpcClient.CheckStatus(ctx, new(pb.Empty))
	if err != nil {
		return resp, err
	}

	resp.Status = api.Statuses.Reverse(s.Status)
	resp.FailureReason = s.FailureReason

	return resp, nil
}

type InitGracefulShutdownResponse struct {
	Status        api.Status
	FailureReason string
}

func (c *Client) InitGracefulShutdown(ctx context.Context, req api.InitGracefulShutdownRequest) (CheckStatusResponse, error) {
	c.logger.Info("Initializing graceful shutdown")

	var resp CheckStatusResponse

	var shutdownCallback *pb.ShutdownCallback
	if req != nil {
		shutdownCallbackDef := req.ShutdownCallbackDef()
		if shutdownCallbackDef != nil {
			shutdownCallback.Url = shutdownCallbackDef.URL()
			shutdownCallback.Method = shutdownCallbackDef.Method()
			shutdownCallback.Headers = shutdownCallbackDef.Headers()
		}
	}

	s, err := c.grpcClient.InitGracefulShutdown(ctx, &pb.InitGracefulShutdownRequest{
		ShutdownCallback: shutdownCallback,
	})
	if err != nil {
		return resp, err
	}

	resp.Status = api.Statuses.Reverse(s.Status)
	resp.FailureReason = s.FailureReason

	return resp, nil
}

func (c *Client) InitForcefulShutdown(ctx context.Context) (CheckStatusResponse, error) {
	c.logger.Info("Initializing forceful shutdown")

	var resp CheckStatusResponse

	s, err := c.grpcClient.InitForcefulShutdown(ctx, new(pb.Empty))
	if err != nil {
		return resp, err
	}

	resp.Status = api.Statuses.Reverse(s.Status)
	resp.FailureReason = s.FailureReason

	return resp, nil
}
