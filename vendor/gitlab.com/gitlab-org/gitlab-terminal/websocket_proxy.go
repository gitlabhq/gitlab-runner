package terminal

import (
	"fmt"

	"github.com/gorilla/websocket"
)

type WebSocketProxy struct {
	StopCh chan error
}

// stoppers is the number of goroutines that may attempt to call Stop()
func NewWebSocketProxy(stoppers int) *WebSocketProxy {
	return &WebSocketProxy{
		StopCh: make(chan error, stoppers+2), // each proxy() call is a stopper
	}
}

func (p *WebSocketProxy) GetStopCh() chan error {
	return p.StopCh
}

func (p *WebSocketProxy) Serve(upstream, downstream Connection, upstreamAddr, downstreamAddr string) error {
	// This signals the upstream terminal to kill the exec'd process
	defer upstream.WriteMessage(websocket.BinaryMessage, eot)

	go p.proxy(upstream, downstream, upstreamAddr, downstreamAddr)
	go p.proxy(downstream, upstream, downstreamAddr, upstreamAddr)

	err := <-p.StopCh
	return err
}

func (p *WebSocketProxy) proxy(to, from Connection, toAddr, fromAddr string) {
	for {
		messageType, data, err := from.ReadMessage()
		if err != nil {
			p.StopCh <- fmt.Errorf("reading from %s: %s", fromAddr, err)
			break
		}

		if err := to.WriteMessage(messageType, data); err != nil {
			p.StopCh <- fmt.Errorf("writing to %s: %s", toAddr, err)
			break
		}
	}
}
