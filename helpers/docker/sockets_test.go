//go:build !integration

package docker

import (
	"io"
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewWrappedConnCloser(t *testing.T) {
	a, b := net.Pipe()

	wrapped := NewWrappedConnCloser(a, b)

	require.Equal(t, a, wrapped.(*wrappedConnCloser).Conn)
	require.Equal(t, b, wrapped.(*wrappedConnCloser).Closer)
	require.NoError(t, wrapped.Close())
	_, aClosedErr := a.Read(nil)
	_, bClosedErr := b.Read(nil)
	require.ErrorIs(t, aClosedErr, io.ErrClosedPipe)
	require.ErrorIs(t, bClosedErr, io.ErrClosedPipe)
}
