package wstunnel

import (
	"context"
	"net"
	"net/http"

	"github.com/gorilla/websocket"
)

// DialerForGRPC can be used as an adapter between "ws"/"wss" URL scheme that the websocket library wants and
// gRPC target naming scheme.
func DialerForGRPC(readLimit int64, dialer websocket.Dialer, requestHeader http.Header) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, address string) (net.Conn, error) {
		conn, _, err := dialer.DialContext(ctx, address, requestHeader) //nolint: bodyclose
		if err != nil {
			return nil, err
		}
		if readLimit != 0 {
			conn.SetReadLimit(readLimit)
		}
		return NetConn(conn), nil
	}
}
