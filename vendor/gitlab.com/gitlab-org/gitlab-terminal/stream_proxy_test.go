package terminal

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type chStringReadWriter struct {
	wrieDone chan struct{}
	bytes.Buffer
}

func (c *chStringReadWriter) Write(p []byte) (int, error) {
	defer func() {
		c.wrieDone <- struct{}{}
	}()
	return c.Buffer.Write(p)
}

func TestServe(t *testing.T) {
	encProxy := NewStreamProxy(1)
	downstream := bytes.Buffer{}
	upstream := chStringReadWriter{
		wrieDone: make(chan struct{}),
	}
	writeString := []byte("data from downstream")

	downstream.Write([]byte(writeString))

	go func() {
		err := encProxy.Serve(&upstream, &downstream)
		if err != nil {
			t.Fatalf("unexpected error from serve: %v", err)
		}
	}()

	// Wait until the write is done
	<-upstream.wrieDone

	b := make([]byte, 20)
	_, err := upstream.Read(b)
	require.NoError(t, err)

	assert.Equal(t, writeString, b)
}

func TestServeError(t *testing.T) {
	encProxy := NewStreamProxy(1)
	downstream := errorReadWriter{}
	upstream := bytes.Buffer{}

	err := encProxy.Serve(&upstream, &downstream)
	assert.Error(t, err)
}

type errorReadWriter struct {
}

func (rw *errorReadWriter) Read(p []byte) (int, error) {
	return 0, errors.New("failed to read")
}

func (rw *errorReadWriter) Write(p []byte) (int, error) {
	return 0, errors.New("failed to read")
}
