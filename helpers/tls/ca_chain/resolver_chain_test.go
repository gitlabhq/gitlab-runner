//go:build !integration

package ca_chain

import (
	"crypto/x509"
	"errors"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

type resolverMockFactory func(t *testing.T) resolver

func newResolverMock(inputCerts, returnCerts []*x509.Certificate, returnErr error) resolverMockFactory {
	return func(t *testing.T) resolver {
		mock := newMockResolver(t)
		mock.
			On("Resolve", inputCerts).
			Return(returnCerts, returnErr).
			Once()

		return mock
	}
}

func TestChainResolver_Resolve(t *testing.T) {
	testError := errors.New("test error")

	certs := []*x509.Certificate{{SerialNumber: big.NewInt(1)}}
	urlCerts := []*x509.Certificate{{SerialNumber: big.NewInt(2)}}
	verifyCerts := []*x509.Certificate{{SerialNumber: big.NewInt(3)}}

	noopMock := func(t *testing.T) resolver { return nil }

	tests := map[string]struct {
		urlResolver    resolverMockFactory
		verifyResolver resolverMockFactory
		expectedError  string
		expectedCerts  []*x509.Certificate
	}{
		"error on urlResolver": {
			urlResolver:    newResolverMock(certs, nil, testError),
			verifyResolver: noopMock,
			expectedError:  "error while resolving certificates chain with URL: test error",
			expectedCerts:  nil,
		},
		"error on verifyResolver": {
			urlResolver:    newResolverMock(certs, urlCerts, nil),
			verifyResolver: newResolverMock(urlCerts, nil, testError),
			expectedError:  "error while resolving certificates chain with verification: test error",
			expectedCerts:  nil,
		},
		"certificates resolved properly": {
			urlResolver:    newResolverMock(certs, urlCerts, nil),
			verifyResolver: newResolverMock(urlCerts, verifyCerts, nil),
			expectedError:  "",
			expectedCerts:  verifyCerts,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			urlResolver := tc.urlResolver(t)
			verifyResolver := tc.verifyResolver(t)

			r := newChainResolver(urlResolver, verifyResolver)
			newCerts, err := r.Resolve(certs)

			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedCerts, newCerts)
		})
	}
}
