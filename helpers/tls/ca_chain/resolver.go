// Inspired by https://github.com/zakjan/cert-chain-resolver/blob/master/certUtil/chain.go
// which is licensed on a MIT license.
//
// Shout out to Jan Žák (http://zakjan.cz) original author of `certUtil` package and other
// contributors who updated it!

package ca_chain

import (
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/sirupsen/logrus"
)

type resolver interface {
	Resolve(certs []*x509.Certificate) ([]*x509.Certificate, error)
}

func newResolver(logger logrus.FieldLogger) resolver {
	return &chainResolver{
		logger: logger,
	}
}

type chainResolver struct {
	logger        logrus.FieldLogger
	verifyOptions x509.VerifyOptions
}

func (d *chainResolver) Resolve(certs []*x509.Certificate) ([]*x509.Certificate, error) {
	certs, err := d.resolveChain(certs)
	if err != nil {
		return nil, fmt.Errorf("error while resolving certificates chain: %v", err)
	}

	certs, err = d.lookForRootIfMissing(certs)
	if err != nil {
		return nil, fmt.Errorf("error while looking for a missing root certificate: %v", err)
	}

	return certs, err
}

func (d *chainResolver) resolveChain(certs []*x509.Certificate) ([]*x509.Certificate, error) {
	if len(certs) < 1 {
		return certs, nil
	}

	for {
		certificate := certs[len(certs)-1]
		log := prepareCertificateLogger(d.logger, certificate)

		if certificate.IssuingCertificateURL == nil {
			log.Debug("Certificate doesn't provide parent URL: exiting the loop")
			break
		}

		newCert, err := d.fetchIssuerCertificate(certificate)
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

func prepareCertificateLogger(logger logrus.FieldLogger, cert *x509.Certificate) logrus.FieldLogger {
	return logger.
		WithFields(logrus.Fields{
			"subject":       cert.Subject.CommonName,
			"issuer":        cert.Issuer.CommonName,
			"serial":        cert.SerialNumber.String(),
			"issuerCertURL": cert.IssuingCertificateURL,
		})
}

func (d *chainResolver) fetchIssuerCertificate(cert *x509.Certificate) (*x509.Certificate, error) {
	log := prepareCertificateLogger(d.logger, cert).
		WithField("method", "fetchIssuerCertificate")

	parentURL := cert.IssuingCertificateURL[0]

	resp, err := http.Get(parentURL)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		log.
			WithError(err).
			Warning("HTTP request error")
		return nil, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.
			WithError(err).
			Warning("Response body read error")
		return nil, err
	}

	newCert, err := DecodeCertificate(data)
	if err != nil {
		log.
			WithError(err).
			Warning("Certificate decoding error")
		return nil, err
	}

	log.
		WithFields(logrus.Fields{
			"newCert-subject":       newCert.Subject.CommonName,
			"newCert-issuer":        newCert.Issuer.CommonName,
			"newCert-serial":        newCert.SerialNumber.String(),
			"newCert-issuerCertURL": newCert.IssuingCertificateURL,
		}).
		Debug("Appending the certificate to the chain")

	return newCert, nil
}

func (d *chainResolver) lookForRootIfMissing(certs []*x509.Certificate) ([]*x509.Certificate, error) {
	if len(certs) < 1 {
		return certs, nil
	}

	lastCert := certs[len(certs)-1]

	if isSelfSigned(lastCert) {
		return certs, nil
	}

	prepareCertificateLogger(d.logger, lastCert).
		Debug("Verifying last certificate to find the final root certificate")

	verifyChains, err := lastCert.Verify(d.verifyOptions)
	if err != nil {
		if _, e := err.(x509.UnknownAuthorityError); e {
			prepareCertificateLogger(d.logger, lastCert).
				WithError(err).
				Warning("Last certificate signed by unknown authority; will not update the chain")

			return certs, nil
		}

		return nil, fmt.Errorf("error while verifying last certificate from the chain: %v", err)
	}

	for _, cert := range verifyChains[0] {
		if lastCert.Equal(cert) {
			continue
		}

		prepareCertificateLogger(d.logger, cert).
			Debug("Adding cert from verify chain to the final chain")

		certs = append(certs, cert)
	}

	return certs, nil
}

func isSelfSigned(cert *x509.Certificate) bool {
	return cert.CheckSignatureFrom(cert) == nil
}
