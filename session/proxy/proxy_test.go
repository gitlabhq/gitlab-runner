//go:build !integration

package proxy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPoolInitializer(t *testing.T) {
	assert.Equal(t, Pool{}, NewPool())
}

func TestProxySettings(t *testing.T) {
	settings := &Settings{
		ServiceName: "serviceName",
		Ports: []Port{
			{
				Number: 80,
				Name:   "port-80",
			},
			{
				Number: 81,
				Name:   "port-81",
			},
		},
	}

	assert.Equal(t, settings, NewProxySettings(settings.ServiceName, settings.Ports))
}

func TestPortByNameOrNumber(t *testing.T) {
	port1 := Port{
		Number: 80,
		Name:   "port-80",
	}

	port2 := Port{
		Number: 81,
		Name:   "port-81",
	}

	settings := Settings{
		ServiceName: "ServiceName",
		Ports:       []Port{port1, port2},
	}

	tests := map[string]struct {
		port          string
		expectedPort  Port
		expectedError bool
	}{
		"Port number does not exist": {
			port:          "8080",
			expectedError: true,
		},
		"Port name does not exist": {
			port:          "Foo",
			expectedError: true,
		},
		"Port number exists": {
			port:         "80",
			expectedPort: port1,
		},
		"Port name exists": {
			port:         "port-81",
			expectedPort: port2,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			result, err := settings.PortByNameOrNumber(test.port)
			if test.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.expectedPort, result)
		})
	}
}

func TestScheme(t *testing.T) {
	tests := map[string]struct {
		protocol         string
		expectedProtocol string
		expectedError    bool
	}{
		"Port protocol is HTTP": {
			protocol:         "http",
			expectedProtocol: "http",
		},
		"Port protocol is HTTPS": {
			protocol:         "https",
			expectedProtocol: "https",
		},
		"Port protocol does not exist": {
			protocol:      "foo",
			expectedError: true,
		},
		"Port protocol is empty": {
			protocol:      "",
			expectedError: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			port := Port{Protocol: test.protocol}
			scheme, err := port.Scheme()

			if test.expectedError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.expectedProtocol, scheme)
		})
	}
}

func TestWebsocketProtocolFor(t *testing.T) {
	tests := map[string]struct {
		protocol           string
		expectedWSProtocol string
	}{
		"Protocol is HTTPS": {
			protocol:           "https",
			expectedWSProtocol: "wss",
		},
		"Protocol is HTTP": {
			protocol:           "http",
			expectedWSProtocol: "ws",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, test.expectedWSProtocol, WebsocketProtocolFor(test.protocol))
		})
	}
}
