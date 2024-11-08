package exec

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

type omitWriter struct {
	buf   []byte
	start int
	end   int
	n     int64
}

func newOmitWriter() *omitWriter {
	return &omitWriter{
		buf: make([]byte, 32*1024),
	}
}

func (r *omitWriter) Write(p []byte) (n int, err error) {
	r.n += int64(len(p))

	for _, b := range p {
		r.buf[r.end] = b
		r.end = (r.end + 1) % cap(r.buf)
		if r.end == r.start {
			r.start = (r.start + 1) % cap(r.buf)
		}
	}
	return n, nil
}

func (r *omitWriter) bytes() []byte {
	if r.start == r.end {
		return nil
	}

	if r.end < r.start {
		part1 := r.buf[r.start:]
		part2 := r.buf[:r.end]
		return append(part1, part2...)
	}

	return r.buf[r.start:r.end]
}

func (r *omitWriter) Error() error {
	length := int64(r.end - r.start)
	if r.end < r.start {
		length = int64(cap(r.buf) - (r.start - r.end))
	}

	if r.n > length {
		return fmt.Errorf("omitted %d... %s", r.n-length, string(r.bytes()))
	}

	return fmt.Errorf("%s", string(r.bytes()))
}

type rwConn struct {
	io.WriteCloser
	io.ReadCloser
}

func (conn *rwConn) CloseWrite() error {
	return silenceAlreadyClosed(conn.WriteCloser.Close())
}

func (conn *rwConn) CloseRead() error {
	return silenceAlreadyClosed(conn.ReadCloser.Close())
}

func (conn *rwConn) Close() error {
	defer conn.WriteCloser.Close()

	if err := silenceAlreadyClosed(conn.ReadCloser.Close()); err != nil {
		return err
	}

	return silenceAlreadyClosed(conn.WriteCloser.Close())
}

func silenceAlreadyClosed(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, net.ErrClosed) {
		return nil
	}

	const winRMClosedErr = "Command has already been closed"
	if strings.Contains(err.Error(), winRMClosedErr) {
		return nil
	}

	return err
}

func (*rwConn) LocalAddr() net.Addr                { return addr{} }
func (*rwConn) RemoteAddr() net.Addr               { return addr{} }
func (*rwConn) SetDeadline(t time.Time) error      { return fmt.Errorf("unsupported") }
func (*rwConn) SetReadDeadline(t time.Time) error  { return fmt.Errorf("unsupported") }
func (*rwConn) SetWriteDeadline(t time.Time) error { return fmt.Errorf("unsupported") }

type addr struct{}

func (addr) Network() string { return "gocat.Conn" }
func (addr) String() string  { return "gocat.Conn" }
