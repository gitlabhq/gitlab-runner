//go:build !integration

package helpers

import (
	"encoding/pem"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProbeURLCommand(t *testing.T) {
	t.Parallel()

	closedPortURL := func(t *testing.T) string {
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		addr := listener.Addr().String()
		require.NoError(t, listener.Close())
		return "http://" + addr
	}

	statusServer := func(t *testing.T, status int) string {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		}))
		t.Cleanup(srv.Close)
		return srv.URL
	}

	tlsServer := func(t *testing.T) *httptest.Server {
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		}))
		t.Cleanup(srv.Close)
		return srv
	}

	serverCAFile := func(t *testing.T, srv *httptest.Server) string {
		f := filepath.Join(t.TempDir(), "ca.pem")
		pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
		require.NoError(t, os.WriteFile(f, pemBytes, 0o600))
		return f
	}

	tests := map[string]struct {
		setup   func(t *testing.T, cmd *ProbeURLCommand)
		wantErr string
	}{
		"missing url": {
			setup:   func(*testing.T, *ProbeURLCommand) {},
			wantErr: "missing --url",
		},
		"http 200 passes": {
			setup: func(t *testing.T, cmd *ProbeURLCommand) {
				cmd.URL = statusServer(t, http.StatusOK)
			},
		},
		"http 401 passes": {
			setup: func(t *testing.T, cmd *ProbeURLCommand) {
				cmd.URL = statusServer(t, http.StatusUnauthorized)
			},
		},
		"http 404 passes": {
			setup: func(t *testing.T, cmd *ProbeURLCommand) {
				cmd.URL = statusServer(t, http.StatusNotFound)
			},
		},
		"untrusted tls certificate fails": {
			setup: func(t *testing.T, cmd *ProbeURLCommand) {
				cmd.URL = tlsServer(t).URL
			},
			wantErr: "certificate",
		},
		"untrusted tls certificate passes with insecure": {
			setup: func(t *testing.T, cmd *ProbeURLCommand) {
				cmd.URL = tlsServer(t).URL
				cmd.Insecure = true
			},
		},
		"tls certificate passes with matching ca-file": {
			setup: func(t *testing.T, cmd *ProbeURLCommand) {
				srv := tlsServer(t)
				cmd.URL = srv.URL
				cmd.CAFile = serverCAFile(t, srv)
			},
		},
		"ca-file without certificates fails": {
			setup: func(t *testing.T, cmd *ProbeURLCommand) {
				cmd.URL = tlsServer(t).URL
				f := filepath.Join(t.TempDir(), "empty.pem")
				require.NoError(t, os.WriteFile(f, []byte("not pem"), 0o600))
				cmd.CAFile = f
			},
			wantErr: "no CA certificates found",
		},
		"redirect is not followed": {
			setup: func(t *testing.T, cmd *ProbeURLCommand) {
				unreachable := closedPortURL(t)
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, unreachable, http.StatusFound)
				}))
				t.Cleanup(srv.Close)
				cmd.URL = srv.URL
			},
		},
		"connection refused fails": {
			setup: func(t *testing.T, cmd *ProbeURLCommand) {
				cmd.URL = closedPortURL(t)
			},
			wantErr: "dial tcp",
		},
		"dns failure fails": {
			setup: func(t *testing.T, cmd *ProbeURLCommand) {
				cmd.URL = "http://nonexistent.invalid"
			},
			wantErr: "no such host",
		},
		"timeout fails": {
			setup: func(t *testing.T, cmd *ProbeURLCommand) {
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					time.Sleep(3 * time.Second)
				}))
				t.Cleanup(srv.Close)
				cmd.URL = srv.URL
				cmd.Timeout = 1
			},
			wantErr: "probing",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cmd := &ProbeURLCommand{}
			tc.setup(t, cmd)

			err := cmd.probe()

			if tc.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}
