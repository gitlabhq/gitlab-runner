//go:build integration

package helpers

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

func TestServiceWaiterCommand_NoEnvironmentVariables(t *testing.T) {
	removeHook := helpers.MakeFatalToPanic()
	defer removeHook()

	// Make sure there are no env vars that match the pattern
	for _, e := range os.Environ() {
		if strings.Contains(e, "_TCP_") {
			err := os.Unsetenv(strings.Split(e, "=")[0])
			require.NoError(t, err)
		}
	}

	cmd := HealthCheckCommand{}

	assert.Panics(t, func() {
		cmd.Execute(nil)
	})
}

func TestHealthCheckCommand_Execute(t *testing.T) {
	cases := []struct {
		name            string
		expectedConnect bool
		exposeHigher    bool
		exposeLower     bool
	}{
		{
			name:            "Successful connect",
			expectedConnect: true,
			exposeHigher:    false,
			exposeLower:     false,
		},
		{
			name:            "Unsuccessful connect because service is down",
			expectedConnect: false,
			exposeHigher:    false,
			exposeLower:     false,
		},
		{
			name:            "Successful connect with higher port exposed",
			expectedConnect: true,
			exposeHigher:    true,
			exposeLower:     false,
		},
		{
			name:            "Unsuccessful connect because lower port exposed",
			expectedConnect: false,
			exposeHigher:    false,
			exposeLower:     true,
		},
		{
			name:            "Unsuccessful connect because both lower and higher port exposed",
			expectedConnect: false,
			exposeHigher:    true,
			exposeLower:     true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Start listening to reverse addr
			listener, err := net.Listen("tcp", "127.0.0.1:")
			require.NoError(t, err)

			port := listener.Addr().(*net.TCPAddr).Port

			err = os.Setenv("SERVICE_TCP_ADDR", "127.0.0.1")
			require.NoError(t, err)

			err = os.Setenv("SERVICE_TCP_PORT", strconv.Itoa(port))
			require.NoError(t, err)

			if c.exposeHigher {
				err = os.Setenv("SERVICE_HIGHER_TCP_PORT", strconv.Itoa(port + 1))
				require.NoError(t, err)
			}

			if c.exposeLower {
				err = os.Setenv("SERVICE_LOWER_TCP_PORT", strconv.Itoa(port - 1))
				require.NoError(t, err)
			}

			// If we don't expect to connect we close the listener.
			if !c.expectedConnect {
				listener.Close()
			}

			ctx, cancelFn := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancelFn()
			done := make(chan struct{})
			go func() {
				cmd := HealthCheckCommand{}
				cmd.Execute(nil)
				done <- struct{}{}
			}()

			select {
			case <-ctx.Done():
				if c.expectedConnect {
					require.Fail(t, "Timeout waiting to start service.")
				}
			case <-done:
				if !c.expectedConnect {
					require.Fail(t, "Expected to not connect to server")
				}
			}
		})
	}
}
