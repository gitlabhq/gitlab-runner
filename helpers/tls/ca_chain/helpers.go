// Inspired by https://github.com/zakjan/cert-chain-resolver/blob/1.0.3/certUtil/io.go
// which is licensed on a MIT license.
//
// Shout out to Jan Žák (http://zakjan.cz) original author of `certUtil` package and other
// contributors who updated it!

package ca_chain

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/fullsailor/pkcs7"
	"github.com/sirupsen/logrus"
)

const (
	pemStart         = "-----BEGIN "
	pemCertBlockType = "CERTIFICATE"
)

type ErrorInvalidCertificate struct {
	inner            error
	nonCertBlockType bool
	nilBlock         bool
}

func (e *ErrorInvalidCertificate) Error() string {
	msg := []string{"invalid certificate"}

	switch {
	case e.nilBlock:
		msg = append(msg, "empty PEM block")
	case e.nonCertBlockType:
		msg = append(msg, "non-certificate PEM block")
	case e.inner != nil:
		msg = append(msg, e.inner.Error())
	}

	return strings.Join(msg, ": ")
}

func decodeCertificate(data []byte) (*x509.Certificate, error) {
	if isPEM(data) {
		block, _ := pem.Decode(data)
		if block == nil {
			return nil, &ErrorInvalidCertificate{nilBlock: true}
		}
		if block.Type != pemCertBlockType {
			return nil, &ErrorInvalidCertificate{nonCertBlockType: true}
		}

		data = block.Bytes
	}

	cert, err := x509.ParseCertificate(data)
	if err == nil {
		return cert, nil
	}

	p, err := pkcs7.Parse(data)
	if err == nil {
		// pkcs7.Parse() can return a nil payload if no certs were decoded
		if p == nil || len(p.Certificates) == 0 {
			return nil, nil
		}

		return p.Certificates[0], nil
	}

	return nil, &ErrorInvalidCertificate{inner: err}
}

func isPEM(data []byte) bool {
	return bytes.HasPrefix(data, []byte(pemStart))
}

func isSelfSigned(cert *x509.Certificate) bool {
	return cert.CheckSignatureFrom(cert) == nil
}

func prepareCertificateLogger(logger logrus.FieldLogger, cert *x509.Certificate) logrus.FieldLogger {
	return preparePrefixedCertificateLogger(logger, cert, "")
}

func preparePrefixedCertificateLogger(
	logger logrus.FieldLogger,
	cert *x509.Certificate,
	prefix string,
) logrus.FieldLogger {
	return logger.
		WithFields(logrus.Fields{
			fmt.Sprintf("%sSubject", prefix):       cert.Subject.CommonName,
			fmt.Sprintf("%sIssuer", prefix):        cert.Issuer.CommonName,
			fmt.Sprintf("%sSerial", prefix):        cert.SerialNumber.String(),
			fmt.Sprintf("%sIssuerCertURL", prefix): cert.IssuingCertificateURL,
		})
}

func verifyCertificate(cert *x509.Certificate) ([][]*x509.Certificate, error) {
	return cert.Verify(x509.VerifyOptions{})
}
