//go:build !integration

package limitwriter

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLimitWriterMultipleWritesOverLimit(t *testing.T) {
	buf := new(bytes.Buffer)

	lw := New(buf, 123)
	n, err := lw.Write(bytes.Repeat([]byte{'a'}, 100))
	require.Equal(t, 100, n)
	require.NoError(t, err)

	n, err = lw.Write(bytes.Repeat([]byte{'a'}, 24))
	require.Equal(t, 23, n)
	require.Error(t, ErrWriteLimitExceeded, err)

	n, err = lw.Write(bytes.Repeat([]byte{'a'}, 10))
	require.Equal(t, 0, n)
	require.Error(t, ErrWriteLimitExceeded, err)

	require.Equal(t, bytes.Repeat([]byte{'a'}, 123), buf.Bytes())
}

func TestLimitWriterSingleWriteExact(t *testing.T) {
	buf := new(bytes.Buffer)

	lw := New(buf, 100)
	n, err := lw.Write(bytes.Repeat([]byte{'a'}, 100))
	require.Equal(t, 100, n)
	require.NoError(t, err)
}

func TestLimitWriterSingleWriteOverLimit(t *testing.T) {
	buf := new(bytes.Buffer)

	lw := New(buf, 100)
	n, err := lw.Write(bytes.Repeat([]byte{'a'}, 101))
	require.Equal(t, 100, n)
	require.Error(t, ErrWriteLimitExceeded, err)
}
