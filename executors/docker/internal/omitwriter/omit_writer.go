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
