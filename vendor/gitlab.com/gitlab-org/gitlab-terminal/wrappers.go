package terminal

import (
	"encoding/base64"
	"io"
	"net"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/unicode"
)

func Wrap(conn Connection, subprotocol string) Connection {
	switch subprotocol {
	case "channel.k8s.io":
		return &kubeWrapper{base64: false, conn: conn}
	case "base64.channel.k8s.io":
		return &kubeWrapper{base64: true, conn: conn}
	case "terminal.gitlab.com":
		return &gitlabWrapper{base64: false, conn: conn}
	case "base64.terminal.gitlab.com":
		return &gitlabWrapper{base64: true, conn: conn}
	}

	return conn
}

func NewIOWrapper(conn Connection) *ioWrapper {
	return &ioWrapper{
		Connection:  conn,
		messageType: websocket.BinaryMessage,
		encoder:     unicode.UTF8.NewEncoder(),
		decoder:     unicode.UTF8.NewDecoder(),
	}
}

type kubeWrapper struct {
	base64 bool
	conn   Connection
}

type gitlabWrapper struct {
	base64 bool
	conn   Connection
}

type ioWrapper struct {
	Connection
	messageType int

	encoder *encoding.Encoder
	decoder *encoding.Decoder
}

func (w *gitlabWrapper) ReadMessage() (int, []byte, error) {
	mt, data, err := w.conn.ReadMessage()
	if err != nil {
		return mt, data, err
	}

	if isData(mt) {
		mt = websocket.BinaryMessage
		if w.base64 {
			data, err = decodeBase64(data)
			if err != nil {
			}
		}
	}

	return mt, data, err
}

func (w *gitlabWrapper) WriteMessage(mt int, data []byte) error {
	if isData(mt) {
		if w.base64 {
			mt = websocket.TextMessage
			data = encodeBase64(data)
		} else {
			mt = websocket.BinaryMessage
		}
	}

	return w.conn.WriteMessage(mt, data)
}

func (w *gitlabWrapper) WriteControl(mt int, data []byte, deadline time.Time) error {
	return w.conn.WriteControl(mt, data, deadline)
}

func (w *gitlabWrapper) Close() error {
	return w.conn.UnderlyingConn().Close()
}

func (w *gitlabWrapper) UnderlyingConn() net.Conn {
	return w.conn.UnderlyingConn()
}

// Coalesces all wsstreams into a single stream. In practice, we should only
// receive data on stream 1.
func (w *kubeWrapper) ReadMessage() (int, []byte, error) {
	mt, data, err := w.conn.ReadMessage()
	if err != nil {
		return mt, data, err
	}

	if isData(mt) {
		mt = websocket.BinaryMessage

		// Remove the WSStream channel number, decode to raw
		if len(data) > 0 {
			data = data[1:]
			if w.base64 {
				data, err = decodeBase64(data)
			}
		}
	}

	return mt, data, err
}

// Always sends to wsstream 0
func (w *kubeWrapper) WriteMessage(mt int, data []byte) error {
	if isData(mt) {
		if w.base64 {
			mt = websocket.TextMessage
			data = append([]byte{'0'}, encodeBase64(data)...)
		} else {
			mt = websocket.BinaryMessage
			data = append([]byte{0}, data...)
		}
	}

	return w.conn.WriteMessage(mt, data)
}

func (w *kubeWrapper) WriteControl(mt int, data []byte, deadline time.Time) error {
	return w.conn.WriteControl(mt, data, deadline)
}

func (w *kubeWrapper) UnderlyingConn() net.Conn {
	return w.conn.UnderlyingConn()
}

// encodes the given data as utf-8 and writes it to the websocket
func (w *ioWrapper) Write(data []byte) (n int, err error) {
	n = len(data)
	if w.messageType != websocket.BinaryMessage {
		utf8, err := w.encoder.String(string(data))
		if err != nil {
			return 0, err
		}
		data = []byte(utf8)
	}
	err = w.WriteMessage(w.messageType, data)
	return n, err
}

// decodes utf-8 encoded data from the websocket
func (w *ioWrapper) Read(out []byte) (n int, err error) {
	mt, data, err := w.ReadMessage()
	if mt != websocket.BinaryMessage {
		switch err {
		case nil:
			data, err = w.decoder.Bytes(data)
		case io.EOF:
			return 0, io.EOF
		}
	}
	if err != nil {
		return 0, err
	}
	w.messageType = mt
	return copy(out, data), nil
}

func isData(mt int) bool {
	return mt == websocket.BinaryMessage || mt == websocket.TextMessage
}

func encodeBase64(data []byte) []byte {
	buf := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
	base64.StdEncoding.Encode(buf, data)

	return buf
}

func decodeBase64(data []byte) ([]byte, error) {
	buf := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
	n, err := base64.StdEncoding.Decode(buf, data)
	return buf[:n], err
}
