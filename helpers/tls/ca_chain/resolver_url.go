// Inspired by https://github.com/zakjan/cert-chain-resolver/blob/1.0.3/certUtil/chain.go
// which is licensed on a MIT license.
//
// Shout out to Jan Žák (http://zakjan.cz) original author of `certUtil` package and other
// contributors who updated it!

package ca_chain

import (
	"crypto/x509"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

const defaultURLResolverLoopLimit = 15
const defaultURLResolverFetchTimeout = 15 * time.Second

//go:generate mockery --name=fetcher --inpackage
type fetcher interface {
	Fetch(url string) ([]byte, error)
}

type httpFetcher struct {
	client *http.Client
}

func newHTTPFetcher(timeout time.Duration) *httpFetcher {
	return &httpFetcher{
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (f *httpFetcher) Fetch(url string) ([]byte, error) {
	resp, err := f.client.Get(url)
	if resp != nil {
		defer func() { _ = resp.Body.Close() }()
	}
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

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
		fetcher:   newHTTPFetcher(defaultURLResolverFetchTimeout),
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
			return nil, fmt.Errorf("error while fetching issuer certificate: %w", err)
		}

		if newCert == nil {
			log.Debug("Fetched issuer certificate file does not contain any certificates: exiting the loop")
			break
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

	data, err := r.fetcher.Fetch(issuerURL)
	if err != nil {
		log.
			WithError(err).
			WithField("issuerURL", issuerURL).
			Warning("Remote certificate fetching error")

		return nil, fmt.Errorf("remote fetch failure: %w", err)
	}

	newCert, err := r.decoder(data)
	if err != nil {
		log.
			WithError(err).
			Warning("Certificate decoding error")

		return nil, fmt.Errorf("decoding failure: %w", err)
	}

	if newCert == nil {
		log.Debug("Issuer certificate file decoded properly but did not include any certificates")
		return nil, nil
	}

	preparePrefixedCertificateLogger(log, newCert, "newCert").
		Debug("Appending the certificate to the chain")

	return newCert, nil
}
