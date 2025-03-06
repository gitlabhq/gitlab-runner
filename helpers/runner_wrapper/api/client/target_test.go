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
		{
			target:          "127.0.0.1:8080",
			expectedNetwork: "tcp",
			expectedAddress: "127.0.0.1:8080",
		},
		{
			target:          "127.0.0.1",
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

func TestFormatGRPCCompatible(t *testing.T) {
	tests := []struct {
		target   string
		expected string
	}{
		{target: "unix:///tmp/test.sock", expected: "unix:///tmp/test.sock"},
		{target: "unix:tmp/test.sock", expected: "unix:tmp/test.sock"},
		{target: "tcp://127.0.0.1:8080", expected: "127.0.0.1:8080"},
		{target: "tcp://127.0.0.1", expected: "127.0.0.1"},
		{target: "127.0.0.1:8080", expected: "127.0.0.1:8080"},
		{target: "127.0.0.1", expected: "127.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.target, func(t *testing.T) {
			assert.Equal(t, tt.expected, formatGRPCCompatible(tt.target))
		})
	}
}
