// Inspired by https://github.com/zakjan/cert-chain-resolver/blob/master/certUtil/chain.go
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

const defaultURLResolverLoopLimit = 15

type fetcher func(url string) ([]byte, error)
type decoder func(data []byte) (*x509.Certificate, error)

type urlResolver struct {
	logger  logrus.FieldLogger
	fetcher fetcher
	decoder decoder

	loopLimit int
}

func newURLResolver(logger logrus.FieldLogger) resolver {
	return &urlResolver{
		logger:    logger,
		fetcher:   fetchRemoteCertificate,
		decoder:   decodeCertificate,
		loopLimit: defaultURLResolverLoopLimit,
	}
}

func (r *urlResolver) Resolve(certs []*x509.Certificate) ([]*x509.Certificate, error) {
	if len(certs) < 1 {
		return nil, nil
	}

	loop := 0
	for {
		loop++
		if loop >= r.loopLimit {
			r.
				logger.
				Warning("urlResolver loop limit exceeded; exiting the loop")

			break
		}

		certificate := certs[len(certs)-1]
		log := prepareCertificateLogger(r.logger, certificate)

		if certificate.IssuingCertificateURL == nil {
			log.Debug("Certificate doesn't provide parent URL: exiting the loop")
			break
		}

		newCert, err := r.fetchIssuerCertificate(certificate)
		if err != nil {
			return nil, fmt.Errorf("error while fetching issuer certificate: %v", err)
		}

		certs = append(certs, newCert)

		if isSelfSigned(newCert) {
			log.Debug("Fetched issuer certificate is a ROOT certificate so exiting the loop")
			break
		}
	}

	return certs, nil
}

func (r *urlResolver) fetchIssuerCertificate(cert *x509.Certificate) (*x509.Certificate, error) {
	log := prepareCertificateLogger(r.logger, cert).
		WithField("method", "fetchIssuerCertificate")

	issuerURL := cert.IssuingCertificateURL[0]

	data, err := r.fetcher(issuerURL)
	if err != nil {
		log.
			WithError(err).
			WithField("issuerURL", issuerURL).
			Warning("Remote certificate fetching error")

		return nil, fmt.Errorf("remote fetch failure: %v", err)
	}

	newCert, err := r.decoder(data)
	if err != nil {
		log.
			WithError(err).
			Warning("Certificate decoding error")

		return nil, fmt.Errorf("decoding failure: %v", err)
	}

	preparePrefixedCertificateLogger(log, newCert, "newCert").
		Debug("Appending the certificate to the chain")

	return newCert, nil
}
