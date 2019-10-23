// Inspired by https://github.com/zakjan/cert-chain-resolver/blob/master/certUtil/io.go
// which is licensed on a MIT license.
//
// Shout out to Jan Žák (http://zakjan.cz) original author of `certUtil` package and other
// contributors who updated it!

package ca_chain

import (
	"bytes"
	"crypto/x509"
	"encoding/pem"
	"strings"

	"github.com/fullsailor/pkcs7"
)

const (
	pemStart         = "-----BEGIN "
	pemCertBlockType = "CERTIFICATE"
)

type ErrorInvalidCertificate struct {
	inner            error
	nilBlock         bool
	nonCertBlockType bool
}

func (e *ErrorInvalidCertificate) Error() string {
	msg := []string{"invalid certificate"}

	if e.nilBlock {
		msg = append(msg, "empty PEM block")
	} else if e.nonCertBlockType {
		msg = append(msg, "non certificate PEM block")
	} else if e.inner != nil {
		msg = append(msg, e.inner.Error())
	}

	return strings.Join(msg, ": ")
}

func DecodeCertificate(data []byte) (*x509.Certificate, error) {
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
		return p.Certificates[0], nil
	}

	return nil, &ErrorInvalidCertificate{inner: err}
}

func isPEM(data []byte) bool {
	return bytes.HasPrefix(data, []byte(pemStart))
}
