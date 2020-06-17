package session

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
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
		expectedError   error
	}{
		{
			name: "Default to listen address when Advertising address not defined",
			config: ServerConfig{
				ListenAddress: "127.0.0.1:0",
			},
			expectedAddress: "https://127.0.0.1:0",
			expectedError:   nil,
		},
		{
			name: "Advertising address take precedence over listen address",
			config: ServerConfig{
				ListenAddress:    "0.0.0.0:0",
				AdvertiseAddress: "terminal.example.com",
			},
			expectedAddress: "https://terminal.example.com",
			expectedError:   nil,
		},
		{
			name: "Advertising address not valid ip/domain",
			config: ServerConfig{
				ListenAddress:    "0.0.0.0:0",
				AdvertiseAddress: "%^*",
			},
			expectedError: errors.New(`parse https://%^*: invalid URL escape "%^*"`),
		},
		{
			name: "Advertising address already has https schema",
			config: ServerConfig{
				ListenAddress:    "127.0.0.1:0",
				AdvertiseAddress: "https://terminal.example.com",
			},
			expectedError: errors.New("url not valid, scheme defined"),
		},
		{
			name: "Advertising address has http as scheme",
			config: ServerConfig{
				ListenAddress:    "127.0.0.1:0",
				AdvertiseAddress: "http://terminal.example.com",
			},
			expectedError: errors.New("url not valid, scheme defined"),
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			server, err := NewServer(c.config, nil, certificate.X509Generator{}, fakeSessionFinder)

			if c.expectedError != nil {
				assert.EqualError(t, err, c.expectedError.Error())
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
