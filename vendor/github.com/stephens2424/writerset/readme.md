# writerset
--

Package writerset implements a mechanism to add and remove writers from a
construct similar to io.MultiWriter.

[![GoDoc](https://godoc.org/github.com/stephens2424/writerset?status.svg)](https://godoc.org/github.com/stephens2424/writerset)

## Usage

#### type ErrPartialWrite

```go
type ErrPartialWrite struct {
	Writer          io.Writer
	Err             error
	Expected, Wrote int
}
```

ErrPartialWrite encapsulates an error from a WriterSet.

#### func (ErrPartialWrite) Error

```go
func (e ErrPartialWrite) Error() string
```
Error returns the error string from the underlying error.

#### type WriterSet

```go
type WriterSet struct {
}
```

WriterSet wraps multiple writers like io.MultiWriter, but such that individual
writers are easy to add or remove as necessary.

#### func  New

```go
func New() *WriterSet
```
New initializes a new empty writer set. This function is here for ease of use
and backward compatibility, but a zero-value WriterSet is valid and ready for
use.

#### func (*WriterSet) Add

```go
func (ws *WriterSet) Add(w io.Writer) <-chan error
```
Add ensures w is in the set. w must be a valid map key or Add will panic. The
returned channel is written to if an error occurs writing to this writer, and in
that case, the writer is removed from the set. The error will be of type
ErrorPartialWrite. The channel is closed when the writer is removed from the
set, with or without an error.

#### func (*WriterSet) Contains

```go
func (ws *WriterSet) Contains(w io.Writer) bool
```
Contains determines if w is in the set.

#### func (*WriterSet) Flush

```go
func (ws *WriterSet) Flush()
```
Flush implements http.Flusher by calling flush on all the underlying writers if
they are also http.Flushers.

#### func (*WriterSet) Remove

```go
func (ws *WriterSet) Remove(w io.Writer)
```
Remove ensures w is not in the set. If it is in the set, the error channel
associated with it will be closed.

#### func (*WriterSet) Write

```go
func (ws *WriterSet) Write(b []byte) (int, error)
```
Write writes data to each underlying writer. If an error occurs on an underlying
writer, that writer is removed from the set. The error will be wrapped as an
ErrPartialWrite and sent on the channel created when the writer was added.
