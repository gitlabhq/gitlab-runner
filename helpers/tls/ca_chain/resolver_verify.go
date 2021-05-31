// Inspired by https://github.com/zakjan/cert-chain-resolver/blob/1.0.3/certUtil/chain.go
// which is licensed on a MIT license.
//
// Shout out to Jan Žák (http://zakjan.cz) original author of `certUtil` package and other
// contributors who updated it!

package ca_chain

import (
	"crypto/x509"
	"fmt"

	"github.com/sirupsen/logrus"
)

type verifier func(cert *x509.Certificate) ([][]*x509.Certificate, error)

type verifyResolver struct {
	logger   logrus.FieldLogger
	verifier verifier
}

func newVerifyResolver(logger logrus.FieldLogger) resolver {
	return &verifyResolver{
		logger:   logger,
		verifier: verifyCertificate,
	}
}

func (r *verifyResolver) Resolve(certs []*x509.Certificate) ([]*x509.Certificate, error) {
	if len(certs) < 1 {
		return certs, nil
	}

	lastCert := certs[len(certs)-1]

	if isSelfSigned(lastCert) {
		return certs, nil
	}

	prepareCertificateLogger(r.logger, lastCert).
		Debug("Verifying last certificate to find the final root certificate")

	verifyChains, err := r.verifier(lastCert)
	if err != nil {
		_, ok := err.(x509.UnknownAuthorityError)
		if ok {
			prepareCertificateLogger(r.logger, lastCert).
				WithError(err).
				Warning("Last certificate signed by unknown authority; will not update the chain")

			return certs, nil
		}

		return nil, fmt.Errorf("error while verifying last certificate from the chain: %w", err)
	}

	for _, cert := range verifyChains[0] {
		if lastCert.Equal(cert) {
			continue
		}

		prepareCertificateLogger(r.logger, cert).
			Debug("Adding cert from verify chain to the final chain")

		certs = append(certs, cert)
	}

	return certs, nil
}
