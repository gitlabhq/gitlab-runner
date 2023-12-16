package executors

import (
	"context"
	"io"
	"net"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
)

type Environment interface {
	Prepare(context.Context, buildlogger.Logger, common.ExecutorPrepareOptions) (Client, error)
}

type Client interface {
	Dial(n string, addr string) (net.Conn, error)
	Run(context.Context, RunOptions) error
	DialRun(context.Context, string) (net.Conn, error)
	Close() error
}

type RunOptions struct {
	Command string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}
