package internal

import "io"

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error {
	return nil
}

func NewNopCloser(w io.Writer) io.WriteCloser {
	return nopCloser{w}
}
