package omitwriter

import (
	"fmt"
)

type omitWriter struct {
	buf   []byte
	start int
	end   int
	n     int64
}

func New() *omitWriter {
	return &omitWriter{
		buf: make([]byte, 32*1024),
	}
}

// Write keeps the most recent bytes written to it, as many as its buffer
// holds, and drops older ones. It always reports all of p as written, as
// io.Writer requires: io.Copy and stdcopy.StdCopy fail with
// io.ErrShortWrite on a write that reports less.
func (r *omitWriter) Write(p []byte) (int, error) {
	r.n += int64(len(p))

	for _, b := range p {
		r.buf[r.end] = b
		r.end = (r.end + 1) % cap(r.buf)
		if r.end == r.start {
			r.start = (r.start + 1) % cap(r.buf)
		}
	}
	return len(p), nil
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

// Error returns what was written as an error, prefixed with how many bytes
// were dropped when it no longer fit, or nil when nothing was written.
func (r *omitWriter) Error() error {
	if r.n == 0 {
		return nil
	}

	length := int64(r.end - r.start)
	if r.end < r.start {
		length = int64(cap(r.buf) - (r.start - r.end))
	}

	if r.n > length {
		return fmt.Errorf("omitted %d... %s", r.n-length, string(r.bytes()))
	}

	return fmt.Errorf("%s", string(r.bytes()))
}
