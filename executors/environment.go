package executors

import (
	"context"
	"io"
	"net"
)

type Environment interface {
	ID() string
	OS() string
	Arch() string
	Dial(ctx context.Context) (Client, error)
}

type Client interface {
	Dial(n string, addr string) (net.Conn, error)
	Run(RunOptions) error
	Close() error
}

type RunOptions struct {
	Command string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
}
