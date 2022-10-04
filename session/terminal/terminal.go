package terminal

import (
	"errors"
	"net/http"
)

//go:generate mockery --name=InteractiveTerminal --inpackage
type InteractiveTerminal interface {
	Connect() (Conn, error)
}

//go:generate mockery --name=Conn --inpackage
type Conn interface {
	Start(w http.ResponseWriter, r *http.Request, timeoutCh, disconnectCh chan error)
	Close() error
}

func ProxyTerminal(timeoutCh, disconnectCh, proxyStopCh chan error, proxyFunc func()) {
	disconnected := make(chan bool, 1)
	// terminal exit handler
	go func() {
		// wait for either session timeout or disconnection from the client
		select {
		case err := <-timeoutCh:
			proxyStopCh <- err
		case <-disconnected:
			// forward the disconnection event if there is any waiting receiver
			nonBlockingSend(
				disconnectCh,
				errors.New("finished proxying (client disconnected?)"),
			)
		}
	}()

	proxyFunc()
	disconnected <- true
}

func nonBlockingSend(ch chan error, err error) {
	select {
	case ch <- err:
	default:
	}
}
