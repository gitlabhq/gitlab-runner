//go:build !integration

package ca_chain

import (
	"crypto/x509"
	"encoding/pem"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func loadCertificate(t *testing.T, dump string) *x509.Certificate {
	block, _ := pem.Decode([]byte(dump))
	cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

	return cert
}

func TestErrorInvalidCertificate_Error(t *testing.T) {
	testError := errors.New("test-error")

	tests := map[string]struct {
		err            *ErrorInvalidCertificate
		expectedOutput string
	}{
		"no details provided": {
			err:            new(ErrorInvalidCertificate),
			expectedOutput: "invalid certificate",
		},
		"inner specified": {
			err: &ErrorInvalidCertificate{
				inner: testError,
			},
			expectedOutput: "invalid certificate: test-error",
		},
		"marked with nonCertBlockType": {
			err: &ErrorInvalidCertificate{
				inner:            testError,
				nonCertBlockType: true,
			},
			expectedOutput: "invalid certificate: non-certificate PEM block",
		},
		"marked with nilBlock": {
			err: &ErrorInvalidCertificate{
				inner:            testError,
				nonCertBlockType: true,
				nilBlock:         true,
			},
			expectedOutput: "invalid certificate: empty PEM block",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.EqualError(t, tc.err, tc.expectedOutput)
		})
	}
}

func TestDecodeCertificate(t *testing.T) {
	block, _ := pem.Decode([]byte(testCert))
	decodedPEMx509Data := block.Bytes

	testX509Certificate, err := x509.ParseCertificate(decodedPEMx509Data)
	require.NoError(t, err)

	block, _ = pem.Decode([]byte(testCertPKCS7))
	decodedPEMPKCS7Data := block.Bytes

	emptyBlock, _ := pem.Decode([]byte(testEmptyCertPKCS7))
	emptyPEMPKCS7Data := emptyBlock.Bytes

	tests := map[string]struct {
		data                []byte
		expectedError       string
		expectedCertificate *x509.Certificate
	}{
		"invalid data": {
			data:                []byte("test"),
			expectedError:       "invalid certificate: ber2der: BER tag length is more than available data",
			expectedCertificate: nil,
		},
		"invalid PEM type": {
			data:                []byte(testCertPubKey),
			expectedError:       "invalid certificate: non-certificate PEM block",
			expectedCertificate: nil,
		},
		"raw PEM x509 data": {
			data:                []byte(testCert),
			expectedError:       "",
			expectedCertificate: testX509Certificate,
		},
		"decoded PEM x509 data": {
			data:                decodedPEMx509Data,
			expectedError:       "",
			expectedCertificate: testX509Certificate,
		},
		"decoded PEM pkcs7 data": {
			data:                decodedPEMPKCS7Data,
			expectedError:       "",
			expectedCertificate: testX509Certificate,
		},
		"empty PEM pkcs7 data": {
			data:                emptyPEMPKCS7Data,
			expectedError:       "",
			expectedCertificate: nil,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			cert, err := decodeCertificate(tc.data)

			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				return
			}

			assert.NoError(t, err)

			if tc.expectedCertificate != nil {
				assert.Equal(t, tc.expectedCertificate.SerialNumber, cert.SerialNumber)
				return
			}

			assert.Nil(t, tc.expectedCertificate)
		})
	}
}

func TestIsPem(t *testing.T) {
	assert.True(t, isPEM([]byte(testCert)))

	block, _ := pem.Decode([]byte(testCert))
	assert.False(t, isPEM(block.Bytes))
}
