package writer

import (
	"io"
)

func NewProxy(inner io.Writer) Writer {
	return &proxy{
		inner: inner,
	}
}

type proxy struct {
	inner io.Writer
}

func (w *proxy) Write(p []byte) (int, error) {
	return w.inner.Write(p)
}

func (w *proxy) Flush() error {
	return nil
}
