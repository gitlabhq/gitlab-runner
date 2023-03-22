package docker

import (
	"io"
	"net"
)

type wrappedConnCloser struct {
	net.Conn
	io.Closer
}

// NewWrapperConnCloser returns an implementation of net.Conn that also closes
// an additional closer. It is useful for tunnelled connections, where a dialer
// might initiate both a parent and tunnelled connection, but wish to have the
// caller close both together.
func NewWrappedConnCloser(conn net.Conn, closer io.Closer) net.Conn {
	return &wrappedConnCloser{
		Conn:   conn,
		Closer: closer,
	}
}

func (c *wrappedConnCloser) Close() error {
	defer c.Conn.Close()

	if err := c.Closer.Close(); err != nil {
		return err
	}

	return c.Conn.Close()
}
