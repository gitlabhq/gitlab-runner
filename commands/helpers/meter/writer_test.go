//go:build !integration

package meter

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type nopWriteCloser struct {
	w io.Writer
}

func (wc *nopWriteCloser) Write(p []byte) (int, error) {
	return wc.w.Write(p)
}

func (wc *nopWriteCloser) Close() error {
	return nil
}

func TestWriter_New_NoUpdateFrequency(t *testing.T) {
	// the original io.ReadCloser is returned if the meter update frequency
	// is zero.
	writer := &nopWriteCloser{w: nil}
	m := NewWriter(writer, 0, func(uint64, time.Duration, bool) {})
	assert.Equal(t, writer, m)
}

func TestWriter_New(t *testing.T) {
	complete := new(sync.WaitGroup)
	complete.Add(1)

	buf := new(bytes.Buffer)

	m := NewWriter(
		&nopWriteCloser{w: buf},
		50*time.Millisecond,
		func(written uint64, since time.Duration, done bool) {
			if done {
				assert.Equal(t, uint64(6), written)
				complete.Done()
			}
		},
	)

	_, err := io.Copy(m, strings.NewReader("foobar"))
	assert.NoError(t, err)
	assert.NoError(t, m.Close())
	complete.Wait()

	// another close shouldn't be a problem
	assert.NoError(t, m.Close())
}
