package terminal

import (
	"fmt"
	"io"
)

type FileDescriptorProxy struct {
	StopCh chan error
}

// stoppers is the number of goroutines that may attempt to call Stop()
func NewFileDescriptorProxy(stoppers int) *FileDescriptorProxy {
	return &FileDescriptorProxy{
		StopCh: make(chan error, stoppers+2), // each proxy() call is a stopper
	}
}

func (p *FileDescriptorProxy) GetStopCh() chan error {
	return p.StopCh
}

func (p *FileDescriptorProxy) Serve(upstream, downstream io.ReadWriter, upstreamAddr, downstreamAddr string) error {
	// This signals the upstream terminal to kill the exec'd process
	defer upstream.Write(eot)

	go p.proxy(upstream, downstream, upstreamAddr, downstreamAddr)
	go p.proxy(downstream, upstream, downstreamAddr, upstreamAddr)

	err := <-p.StopCh
	return err
}

func (p *FileDescriptorProxy) proxy(to, from io.ReadWriter, toAddr, fromAddr string) {
	_, err := io.Copy(to, from)
	if err != nil {
		p.StopCh <- fmt.Errorf("copying from %s to %s: %s", fromAddr, toAddr, err)
	}
}
