package proxy

import (
	"errors"
	"net/http"
	"strconv"
)

type Pool map[string]*Proxy

type Pooler interface {
	Pool() Pool
}

type Proxy struct {
	Settings          *Settings
	ConnectionHandler Requester
}

type Settings struct {
	ServiceName string
	Ports       []Port
}

type Port struct {
	Number   int
	Protocol string
	Name     string
}

//go:generate mockery --name=Requester --inpackage
type Requester interface {
	ProxyRequest(w http.ResponseWriter, r *http.Request, requestedURI, port string, settings *Settings)
}

func NewPool() Pool {
	return Pool{}
}

func NewProxySettings(serviceName string, ports []Port) *Settings {
	return &Settings{
		ServiceName: serviceName,
		Ports:       ports,
	}
}

// PortByNameOrNumber accepts both a port number or a port name.
// It will try to convert the method into an integer and then
// search if there is any port number with that value or any
// port name by the param value.
func (p *Settings) PortByNameOrNumber(portNameOrNumber string) (Port, error) {
	intPort, _ := strconv.Atoi(portNameOrNumber)

	for _, port := range p.Ports {
		if port.Number == intPort || port.Name == portNameOrNumber {
			return port, nil
		}
	}

	return Port{}, errors.New("invalid port")
}

func (p *Port) Scheme() (string, error) {
	if p.Protocol == "http" || p.Protocol == "https" {
		return p.Protocol, nil
	}

	return "", errors.New("invalid port scheme")
}

// WebsocketProtocolFor returns the proper Websocket protocol
// based on the HTTP protocol
func WebsocketProtocolFor(httpProtocol string) string {
	if httpProtocol == "https" {
		return "wss"
	}

	return "ws"
}
