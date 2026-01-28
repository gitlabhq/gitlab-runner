package wstunnel

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// NetConn converts a *websocket.Conn into a net.Conn.
//
// It's for tunneling arbitrary protocols over WebSockets.
//
// Every Write to the net.Conn will correspond to a binary message write on *websocket.Conn.
//
// If a message is read that is not of the binary type, the connection
// will be closed with CloseUnsupportedData and an error will be returned.
//
// Close will close the *websocket.Conn with CloseNormalClosure.
//
// When a deadline is hit and there is an active read or write goroutine, the
// connection will be closed. This is different from most net.Conn implementations
// where only the reading/writing goroutines are interrupted but the connection
// is kept alive.
//
// A received CloseNormalClosure or CloseGoingAway close frame will be translated to
// io.EOF when reading.
func NetConn(c *websocket.Conn) net.Conn {
	return &netConn{
		c: c,
	}
}

type netConn struct {
	c                   *websocket.Conn
	reader              io.Reader
	futureWriteDeadline atomic.Pointer[time.Time]
	readEOFed           bool
}

func (nc *netConn) Close() (retErr error) {
	defer func() {
		// Always close the connection, even if WriteControl() returns an error or panics.
		retErr = errors.Join(retErr, nc.c.Close())
	}()
	return nc.c.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(time.Second),
	)
}

func (nc *netConn) Write(p []byte) (int, error) {
	old := nc.futureWriteDeadline.Swap(nil)
	if old != nil {
		// Unsynchronized write deadline field is read in the WriteMessage() call below.
		// Hence, it is safe to call SetWriteDeadline() here as it must not be called concurrently
		// since that would be a data race.
		err := nc.c.SetWriteDeadline(*old)
		if err != nil {
			return 0, err
		}
	}
	err := nc.c.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (nc *netConn) Read(p []byte) (int, error) {
	if nc.readEOFed {
		return 0, io.EOF
	}

	if nc.reader == nil {
		typ, r, err := nc.c.NextReader()
		if err != nil {
			// Check if it's a close message
			if websocket.IsUnexpectedCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				// It's an unexpected close, return the error
				return 0, err
			}
			// Normal closure or going away
			nc.readEOFed = true
			return 0, io.EOF
		}
		if typ != websocket.BinaryMessage {
			err := fmt.Errorf("unexpected frame type read (expected BinaryMessage): %v", typ)
			_ = nc.c.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseUnsupportedData, err.Error()),
				time.Now().Add(time.Second),
			)
			return 0, err
		}
		nc.reader = r
	}

	n, err := nc.reader.Read(p)
	if err == io.EOF {
		nc.reader = nil
		err = nil
	}
	return n, err
}

func (nc *netConn) LocalAddr() net.Addr {
	return nc.c.LocalAddr()
}

func (nc *netConn) RemoteAddr() net.Addr {
	return nc.c.RemoteAddr()
}

func (nc *netConn) SetDeadline(t time.Time) error {
	// Because we have extra stuff in SetWriteDeadline(), we cannot just call SetDeadline() on the underlying connection.
	err := nc.SetReadDeadline(t)
	if err != nil {
		return err
	}
	return nc.SetWriteDeadline(t)
}

func (nc *netConn) SetWriteDeadline(t time.Time) error {
	// This method must be thread safe - e.g. it is safe to call concurrently to abort a connection.
	// We cannot use nc.c.SetWriteDeadline() here directly since it is not thread safe - cannot be called
	// concurrently with WriteMessage(). So, we are making our own version with similar functionality.
	nc.futureWriteDeadline.Store(&t)
	return nc.c.NetConn().SetWriteDeadline(t)
}

func (nc *netConn) SetReadDeadline(t time.Time) error {
	return nc.c.NetConn().SetReadDeadline(t)
}
