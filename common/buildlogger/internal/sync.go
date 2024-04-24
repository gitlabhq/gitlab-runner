package internal

import (
	"io"
	"sync"
)

type syncWriter struct {
	mu sync.Mutex

	w io.WriteCloser
}

func NewSync(w io.WriteCloser) *syncWriter {
	return &syncWriter{w: w}
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.w.Write(p)
}

func (s *syncWriter) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.w.Close()
}
