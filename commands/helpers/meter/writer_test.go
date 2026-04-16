//go:build !integration

package meter

import (
	"bytes"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestWriter_WriteAt_underlyingFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "meter-writeat")
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })

	complete := new(sync.WaitGroup)
	complete.Add(1)

	m := NewWriter(f, 50*time.Millisecond, func(written uint64, since time.Duration, done bool) {
		if done {
			assert.Equal(t, uint64(5), written)
			complete.Done()
		}
	})

	wa, ok := m.(io.WriterAt)
	require.True(t, ok)

	n, err := wa.WriteAt([]byte("hello"), 0)
	require.NoError(t, err)
	assert.Equal(t, 5, n)

	require.NoError(t, m.Close())
	complete.Wait()

	got, err := os.ReadFile(f.Name())
	require.NoError(t, err)
	assert.Equal(t, "hello", string(got))
}
