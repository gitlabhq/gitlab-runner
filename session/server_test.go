//go:build !integration

package session

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/certificate"
)

func fakeSessionFinder(url string) *Session {
	return nil
}

func TestAdvertisingAddress(t *testing.T) {
	cases := []struct {
		name            string
		config          ServerConfig
		expectedAddress string
		assertError     func(t *testing.T, err error)
	}{
		{
			name: "Default to listen address when Advertising address not defined",
			config: ServerConfig{
				ListenAddress: "127.0.0.1:0",
			},
			expectedAddress: "https://127.0.0.1:0",
			assertError:     nil,
		},
		{
			name: "Advertising address take precedence over listen address",
			config: ServerConfig{
				ListenAddress:    "0.0.0.0:0",
				AdvertiseAddress: "terminal.example.com",
			},
			expectedAddress: "https://terminal.example.com",
			assertError:     nil,
		},
		{
			name: "Advertising address not valid ip/domain",
			config: ServerConfig{
				ListenAddress:    "0.0.0.0:0",
				AdvertiseAddress: "%^*",
			},
			assertError: func(t *testing.T, err error) {
				var e *url.Error
				if assert.ErrorAs(t, err, &e) {
					assert.Equal(t, "https://%^*", e.URL)
					assert.Equal(t, "parse", e.Op)
					assert.ErrorIs(t, e.Err, url.EscapeError("%^*"))
				}
			},
		},
		{
			name: "Advertising address already has https schema",
			config: ServerConfig{
				ListenAddress:    "127.0.0.1:0",
				AdvertiseAddress: "https://terminal.example.com",
			},
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrInvalidURL)
			},
		},
		{
			name: "Advertising address has http as scheme",
			config: ServerConfig{
				ListenAddress:    "127.0.0.1:0",
				AdvertiseAddress: "http://terminal.example.com",
			},
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrInvalidURL)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server, err := NewServer(c.config, nil, certificate.X509Generator{}, fakeSessionFinder)

			if c.assertError != nil {
				c.assertError(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, c.expectedAddress, server.AdvertiseAddress)
		})
	}
}

func TestCertificate(t *testing.T) {
	cfg := ServerConfig{
		ListenAddress: "127.0.0.1:0",
	}

	requestSuccessful := false
	server, err := NewServer(cfg, nil, certificate.X509Generator{}, func(url string) *Session {
		requestSuccessful = true
		return nil
	})
	require.NoError(t, err)
	defer server.Close()

	go func() {
		errStart := server.Start()
		require.NoError(t, errStart)
	}()

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(server.CertificatePublicKey)

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: caCertPool,
			},
		},
	}

	req, err := http.NewRequest(http.MethodPost, "https://"+server.tlsListener.Addr().String(), nil)
	require.NoError(t, err)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.True(t, requestSuccessful)
}

func TestFailedToGenerateCertificate(t *testing.T) {
	cfg := ServerConfig{
		ListenAddress: "127.0.0.1:0",
	}

	m := new(certificate.MockGenerator)
	defer m.AssertExpectations(t)
	m.On("Generate", mock.Anything).Return(tls.Certificate{}, []byte{}, errors.New("something went wrong"))

	_, err := NewServer(cfg, nil, m, fakeSessionFinder)
	assert.Error(t, err, "something went wrong")
}
