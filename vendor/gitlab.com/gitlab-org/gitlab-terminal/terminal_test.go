package terminal

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const StreamMessage = "this is a test"

func TestProxyStream(t *testing.T) {
	downstream := bufferCloser{}
	downstream.Write([]byte(StreamMessage))

	srv := streamServer{
		downstream: downstream,
	}

	s := httptest.NewServer(&srv)
	defer s.Close()

	c, _, err := websocket.DefaultDialer.Dial("ws://"+s.Listener.Addr().String()+"/ws", nil)
	require.NoError(t, err)

	// Check if writing to websocket works
	c.WriteMessage(websocket.BinaryMessage, []byte(StreamMessage))
	b := make([]byte, len(StreamMessage))
	_, err = downstream.Read(b)
	require.NoError(t, err)
	assert.Equal(t, []byte(StreamMessage), b)

	// Check if reading from websocket works
	typ, b, err := c.ReadMessage()
	require.NoError(t, err)
	assert.Equal(t, typ, websocket.BinaryMessage)
	assert.Equal(t, []byte(StreamMessage), b)
}

type streamServer struct {
	downstream bufferCloser
}

func (d *streamServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	proxy := NewStreamProxy(1)
	ProxyStream(w, r, &d.downstream, proxy)
}

type bufferCloser struct {
	bytes.Buffer
}

func (b *bufferCloser) Close() error {
	return nil
}
