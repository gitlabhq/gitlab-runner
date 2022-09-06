//go:build !integration

package ca_chain

import (
	"bytes"
	"crypto/x509"
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

type verifierMockFactory func(t *testing.T) verifier

func newVerifierMock(inputCert *x509.Certificate, chain [][]*x509.Certificate, err error) verifierMockFactory {
	return func(t *testing.T) verifier {
		return func(cert *x509.Certificate) ([][]*x509.Certificate, error) {
			assert.Equal(t, inputCert, cert)

			return chain, err
		}
	}
}

func TestVerifyResolver_Resolve(t *testing.T) {
	testError := errors.New("test-error")
	testUnknownAuthorityError := x509.UnknownAuthorityError{}

	testCACertificate := loadCertificate(t, testCACert)
	testCertificate := loadCertificate(t, testCert)

	tests := map[string]struct {
		certs          []*x509.Certificate
		mockVerifier   verifierMockFactory
		expectedError  string
		expectedCerts  []*x509.Certificate
		expectedOutput []string
	}{
		"empty input chain": {
			certs:          nil,
			expectedError:  "",
			expectedCerts:  nil,
			expectedOutput: nil,
		},
		"last certificate is self signed": {
			certs:          []*x509.Certificate{testCACertificate},
			expectedError:  "",
			expectedCerts:  []*x509.Certificate{testCACertificate},
			expectedOutput: nil,
		},
		"last certificate is not self signed, verifier fails with unknown authority": {
			certs: []*x509.Certificate{testCertificate},
			mockVerifier: newVerifierMock(
				testCertificate,
				[][]*x509.Certificate{{testCACertificate}},
				testUnknownAuthorityError,
			),
			expectedError: "",
			expectedCerts: []*x509.Certificate{testCertificate},
			expectedOutput: []string{
				"Verifying last certificate to find the final root certificate",
				"Last certificate signed by unknown authority; will not update the chain",
			},
		},
		"last certificate is not self signed, verifier fails with unexpected error": {
			certs:         []*x509.Certificate{testCertificate},
			mockVerifier:  newVerifierMock(testCertificate, [][]*x509.Certificate{{testCACertificate}}, testError),
			expectedError: "error while verifying last certificate from the chain: test-error",
			expectedCerts: nil,
			expectedOutput: []string{
				"Verifying last certificate to find the final root certificate",
			},
		},
		"last certificate is not self signed, duplicate of input certificate in verify chain": {
			certs: []*x509.Certificate{testCertificate},
			mockVerifier: newVerifierMock(
				testCertificate,
				[][]*x509.Certificate{{testCertificate, testCertificate}, {testCertificate}},
				nil,
			),
			expectedError: "",
			expectedCerts: []*x509.Certificate{testCertificate},
			expectedOutput: []string{
				"Verifying last certificate to find the final root certificate",
			},
		},
		"last certificate is not self signed, other certificates in verify chain": {
			certs: []*x509.Certificate{testCertificate},
			mockVerifier: newVerifierMock(
				testCertificate,
				[][]*x509.Certificate{{testCACertificate}, {testCertificate}},
				nil,
			),
			expectedError: "",
			expectedCerts: []*x509.Certificate{testCertificate, testCACertificate},
			expectedOutput: []string{
				"Verifying last certificate to find the final root certificate",
				"Adding cert from verify chain to the final chain",
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			out := new(bytes.Buffer)

			logger := logrus.New()
			logger.SetLevel(logrus.DebugLevel)
			logger.SetOutput(out)

			r := newVerifyResolver(logger).(*verifyResolver)

			if tc.mockVerifier != nil {
				r.verifier = tc.mockVerifier(t)
			}

			newCerts, err := r.Resolve(tc.certs)

			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, tc.expectedCerts, newCerts)

			output := out.String()
			if len(tc.expectedOutput) > 0 {
				for _, expectedLine := range tc.expectedOutput {
					assert.Contains(t, output, expectedLine)
				}
			} else {
				assert.Empty(t, output)
			}
		})
	}
}
