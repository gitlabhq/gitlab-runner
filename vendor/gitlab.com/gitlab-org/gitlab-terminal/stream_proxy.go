package terminal

import (
	"fmt"
	"io"
)

type StreamProxy struct {
	StopCh chan error
}

func NewStreamProxy(stoppers int) *StreamProxy {
	return &StreamProxy{
		StopCh: make(chan error, stoppers+2), // each proxy() call is a stopper
	}
}

func (p *StreamProxy) GetStopCh() chan error {
	return p.StopCh
}

func (p *StreamProxy) Serve(client io.ReadWriter, server io.ReadWriter) error {
	go p.proxy(client, server)
	go p.proxy(server, client)

	err := <-p.StopCh
	return err
}

func (p *StreamProxy) proxy(to, from io.ReadWriter) {
	_, err := io.Copy(to, from)
	if err != nil {
		p.StopCh <- fmt.Errorf("failed to pipe stream: %v", err)
	}
}
