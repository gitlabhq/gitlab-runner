//go:build !integration

package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseDialTarget(t *testing.T) {
	tt := []struct {
		target          string
		expectedNetwork string
		expectedAddress string
	}{
		{
			target:          "unix:///tmp/test.sock",
			expectedNetwork: "unix",
			expectedAddress: "/tmp/test.sock",
		},
		{
			target:          "unix:tmp/test.sock",
			expectedNetwork: "unix",
			expectedAddress: "tmp/test.sock",
		},
		{
			target:          "tcp://127.0.0.1:8080",
			expectedNetwork: "tcp",
			expectedAddress: "127.0.0.1:8080",
		},
		{
			target:          "tcp://127.0.0.1",
			expectedNetwork: "tcp",
			expectedAddress: "127.0.0.1",
		},
	}

	for _, tc := range tt {
		t.Run(tc.target, func(t *testing.T) {
			network, address := parseDialTarget(tc.target)

			assert.Equal(t, tc.expectedNetwork, network)
			assert.Equal(t, tc.expectedAddress, address)
		})
	}
}
