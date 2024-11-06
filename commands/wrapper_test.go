//go:build !integration

package commands

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunnerWrapperCommand_createListener(t *testing.T) {
	testSocketPath := filepath.Join(t.TempDir(), "test.sock")

	skipOnWindows := func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Test doesn't work reliably on Windows (unix socket usage)")
		}
	}

	tests := map[string]struct {
		skip            func(t *testing.T)
		grpcAddress     string
		expectedNetwork string
		expectedAddress string
		assertError     func(t *testing.T, err error)
	}{
		"empty address": {
			grpcAddress: "",
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, errFailedToParseGRPCAddress)
			},
		},
		"proper unix socket - unix://": {
			skip:            skipOnWindows,
			grpcAddress:     fmt.Sprintf("unix://%s", testSocketPath),
			expectedNetwork: "unix",
			expectedAddress: testSocketPath,
		},
		"proper unix socket - unix:": {
			skip:            skipOnWindows,
			grpcAddress:     fmt.Sprintf("unix://%s", testSocketPath),
			expectedNetwork: "unix",
			expectedAddress: testSocketPath,
		},
		"invalid unix socket": {
			skip:        skipOnWindows,
			grpcAddress: fmt.Sprintf("unix:/%s", testSocketPath),
			assertError: func(t *testing.T, err error) {
				var eerr *net.OpError
				if assert.ErrorAs(t, err, &eerr) {
					assert.Equal(t, "unix", eerr.Net)
					assert.Contains(t, testSocketPath, eerr.Addr.String())
					var eeerr *os.SyscallError
					if assert.ErrorAs(t, eerr, &eeerr) {
						assert.Equal(t, "bind", eeerr.Syscall)
					}
				}
			},
		},
		"proper tcp socket": {
			grpcAddress:     "tcp://127.0.0.1:1234",
			expectedNetwork: "tcp",
			expectedAddress: "127.0.0.1:1234",
		},
		"invalid tcp socket": {
			grpcAddress: "tcp://1:1234",
			assertError: func(t *testing.T, err error) {
				var eerr *net.OpError
				if assert.ErrorAs(t, err, &eerr) {
					assert.Equal(t, "listen", eerr.Op)
					assert.Equal(t, "tcp", eerr.Net)
				}
			},
		},
		"unsupported scheme": {
			grpcAddress: "udp://127.0.0.1:1234",
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, errUnsupportedGRPCAddressScheme)
			},
		},
		"default address": {
			grpcAddress:     defaultWrapperGRPCListen,
			expectedNetwork: "tcp",
			expectedAddress: "127.0.0.1:7777",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			if tc.skip != nil {
				tc.skip(t)
			}

			c := &RunnerWrapperCommand{
				GRPCListen: tc.grpcAddress,
			}

			l, err := c.createListener()
			if tc.assertError != nil {
				tc.assertError(t, err)
				return
			}

			defer func(l net.Listener) {
				if l != nil {
					l.Close()
				}
			}(l)

			assert.NoError(t, err)
			assert.Equal(t, tc.expectedNetwork, l.Addr().Network())
			assert.Equal(t, tc.expectedAddress, l.Addr().String())
		})
	}
}
