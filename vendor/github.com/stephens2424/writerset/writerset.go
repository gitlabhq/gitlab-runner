// Package writerset implements a mechanism to add and remove writers from a construct
// similar to io.MultiWriter.
package writerset

import (
	"io"
	"net/http"
	"sync"
)

// ErrPartialWrite encapsulates an error from a WriterSet.
type ErrPartialWrite struct {
	Writer          io.Writer
	Err             error
	Expected, Wrote int
}

// Error returns the error string from the underlying error.
func (e ErrPartialWrite) Error() string {
	return e.Err.Error()
}

// WriterSet wraps multiple writers like io.MultiWriter, but such that individual
// writers are easy to add or remove as necessary.
type WriterSet struct {
	m  map[io.Writer]chan error
	mu sync.Mutex
}

// New initializes a new empty writer set. This function is here for ease of
// use and backward compatibility, but a zero-value WriterSet is valid and
// ready for use.
func New() *WriterSet {
	return &WriterSet{}
}

func (ws *WriterSet) initWithMtx() {
	if ws.m == nil {
		ws.m = make(map[io.Writer]chan error)
	}
}

// Add ensures w is in the set. w must be a valid map key or Add will panic.
// The returned channel is written to if an error occurs writing to this writer,
// and in that case, the writer is removed from the set. The error will be of type
// ErrorPartialWrite. The channel is closed when the writer is removed from the set,
// with or without an error.
func (ws *WriterSet) Add(w io.Writer) <-chan error {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.initWithMtx()

	c, ok := ws.m[w]
	if ok {
		return c
	}

	c = make(chan error, 1)
	ws.m[w] = c
	return c
}

// Contains determines if w is in the set.
func (ws *WriterSet) Contains(w io.Writer) bool {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.initWithMtx()

	_, ok := ws.m[w]
	return ok
}

// Remove ensures w is not in the set. If it is in the set,
// the error channel associated with it will be closed.
func (ws *WriterSet) Remove(w io.Writer) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.initWithMtx()

	ch, ok := ws.m[w]
	if ok {
		close(ch)
		delete(ws.m, w)
	}
}

// Write writes data to each underlying writer. If an error occurs on an underlying writer,
// that writer is removed from the set. The error will be wrapped as an ErrPartialWrite and
// sent on the channel created when the writer was added.
func (ws *WriterSet) Write(b []byte) (int, error) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.initWithMtx()

	for w, c := range ws.m {
		bs, err := w.Write(b)
		if err != nil {
			c <- ErrPartialWrite{
				Err:      err,
				Wrote:    bs,
				Expected: len(b),
				Writer:   w,
			}
			close(c)
			delete(ws.m, w)
		}
	}

	return len(b), nil
}

// Flush implements http.Flusher by calling flush on all the underlying writers if they are
// also http.Flushers.
func (ws *WriterSet) Flush() {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	ws.initWithMtx()

	for w := range ws.m {
		if w, ok := w.(http.Flusher); ok {
			w.Flush()
		}
	}
}
