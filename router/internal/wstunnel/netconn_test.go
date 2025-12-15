//go:build !integration

package wstunnel

import (
	"crypto/rand"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	echoProto = "echo-proto"
	numWrites = 128
)

func TestNetConnVariousBufferSizes(t *testing.T) {
	for _, dataSize := range []int{1024, 64 * 1024, 128 * 1024} {
		t.Run(fmt.Sprintf("%d bytes", dataSize), func(t *testing.T) {
			srv := httptest.NewServer(echoHandler(t))
			defer srv.Close()
			d := websocket.Dialer{
				Subprotocols: []string{echoProto},
			}
			u, err := url.Parse(srv.URL)
			require.NoError(t, err)
			u.Scheme = "ws"
			conn, _, err := d.DialContext(t.Context(), u.String(), nil)
			require.NoError(t, err)
			c := NetConn(conn)
			defer func() {
				assert.NoError(t, c.Close())
			}()
			writeHash := fnv.New128()
			readHash := fnv.New128()

			var wg sync.WaitGroup

			wg.Add(1)
			go func() {
				defer wg.Done()
				data := make([]byte, dataSize)
				for range numWrites {
					_, _ = rand.Read(data)
					_, writeErr := writeHash.Write(data)
					assert.NoError(t, writeErr)
					_, writeErr = c.Write(data)
					assert.NoError(t, writeErr)
				}
			}()
			wg.Add(1)
			go func() {
				defer wg.Done()
				toRead := int64(dataSize * numWrites)
				_, readErr := io.Copy(readHash, io.LimitReader(c, toRead))
				assert.NoError(t, readErr)
			}()
			wg.Wait()

			assert.Equal(t, writeHash.Sum(nil), readHash.Sum(nil))
		})
	}
}

func echoHandler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := websocket.Upgrader{
			Subprotocols: []string{echoProto},
		}
		conn, err := u.Upgrade(w, r, nil)
		if !assert.NoError(t, err) {
			return
		}
		defer func() {
			closeErr := conn.Close()
			assert.NoError(t, closeErr)
		}()
		for {
			mt, data, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
					return
				}
				assert.NoError(t, err)
				return
			}
			if !assert.Equal(t, websocket.BinaryMessage, mt) {
				return
			}
			err = conn.WriteMessage(websocket.BinaryMessage, data)
			if !assert.NoError(t, err) {
				return
			}
		}
	})
}
