package terminal

import (
	"net/http"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/gorilla/websocket"
	"fmt"
	"errors"
)

var (
	// See doc/terminal.md for documentation of this subprotocol
	subprotocols             = []string{"terminal.gitlab.com", "base64.terminal.gitlab.com"}
	upgrader                 = &websocket.Upgrader{Subprotocols: subprotocols}
	BrowserPingInterval      = 30 * time.Second
)

func ProxyWebSocket(w http.ResponseWriter, r *http.Request, terminal *TerminalSettings, proxy *WebSocketProxy) {
	server, err := connectToServer(terminal, r)
	if err != nil {
		fail500(w, r, err)
		log.WithError(err).Print("Terminal: connecting to server failed")
		return
	}
	defer server.UnderlyingConn().Close()
	serverAddr := server.UnderlyingConn().RemoteAddr().String()

	client, err := upgradeClient(w, r)
	if err != nil {
		log.WithError(err).Print("Terminal: upgrading client to websocket failed")
		return
	}

	// Regularly send ping messages to the browser to keep the websocket from
	// being timed out by intervening proxies.
	go pingLoop(client)

	defer client.UnderlyingConn().Close()
	clientAddr := getClientAddr(r) // We can't know the port with confidence

	logEntry := log.WithFields(log.Fields{
		"clientAddr": clientAddr,
		"serverAddr": serverAddr,
	})

	logEntry.Print("Terminal: started proxying")

	defer logEntry.Print("Terminal: finished proxying")

	if err := proxy.Serve(server, client, serverAddr, clientAddr); err != nil {
		logEntry.WithError(err).Print("Terminal: error proxying")
	}
}

func ProxyFileDescriptor(w http.ResponseWriter, r *http.Request, fd *os.File, proxy *FileDescriptorProxy) {
	clientConn, err := upgradeClient(w, r)
	if err != nil {
		log.WithError(err).Print("Terminal: upgrading client to websocket failed")
		return
	}
	client := NewIOWrapper(clientConn)

	// Regularly send ping messages to the browser to keep the websocket from
	// being timed out by intervening proxies.
	go pingLoop(clientConn)

	defer clientConn.UnderlyingConn().Close()
	clientAddr := getClientAddr(r) // We can't know the port with confidence

	serverAddr := "shell"
	logEntry := log.WithFields(log.Fields{
		"clientAddr": clientAddr,
		"serverAddr": serverAddr,
	})

	logEntry.Print("Terminal: started proxying")

	defer logEntry.Print("Terminal: finished proxying")

	if err := proxy.Serve(fd, client, serverAddr, clientAddr); err != nil {
		logEntry.WithError(err).Print("Terminal: error proxying")
	}
}

// In the future, we might want to look at X-Client-Ip or X-Forwarded-For
func getClientAddr(r *http.Request) string {
	return r.RemoteAddr
}

func upgradeClient(w http.ResponseWriter, r *http.Request) (Connection, error) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return nil, err
	}

	return Wrap(conn, conn.Subprotocol()), nil
}

func pingLoop(conn Connection) {
	for {
		time.Sleep(BrowserPingInterval)
		deadline := time.Now().Add(5 * time.Second)
		if err := conn.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
			// Either the connection was already closed so no further pings are
			// needed, or this connection is now dead and no further pings can
			// be sent.
			break
		}
	}
}

func connectToServer(terminal *TerminalSettings, r *http.Request) (Connection, error) {
	terminal = terminal.Clone()

	setForwardedFor(&terminal.Header, r)

	conn, _, err := terminal.Dial()
	if err != nil {
		return nil, err
	}

	return Wrap(conn, conn.Subprotocol()), nil
}

func CloseAfterMaxTime(proxy Proxy, maxSessionTime int) {
	if maxSessionTime == 0 {
		return
	}

	<-time.After(time.Duration(maxSessionTime) * time.Second)
	stopCh := proxy.GetStopCh()
	stopCh <- errors.New(
		fmt.Sprintf(
			"Connection closed: session time greater than maximum time allowed - %v seconds",
			maxSessionTime,
		),
	)
}
