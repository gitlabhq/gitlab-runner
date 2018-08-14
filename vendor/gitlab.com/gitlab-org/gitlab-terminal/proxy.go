package terminal

import (
	"net"
	"time"
)

// ANSI "end of terminal" code
var eot = []byte{0x04}

// An abstraction of gorilla's *websocket.Conn
type Connection interface {
	UnderlyingConn() net.Conn
	ReadMessage() (int, []byte, error)
	WriteMessage(int, []byte) error
	WriteControl(int, []byte, time.Time) error
}

type Proxy interface {
	GetStopCh() chan error
}
