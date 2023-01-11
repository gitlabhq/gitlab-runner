package executors

import (
	"context"
	"io"
	"net"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type Environment interface {
	Prepare(context.Context, common.BuildLogger, common.ExecutorPrepareOptions) (Client, error)
}

type Client interface {
	Dial(n string, addr string) (net.Conn, error)
	Run(context.Context, RunOptions) error
	Close() error
}

type RunOptions struct {
	Command string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}
