package ca_chain

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/zakjan/cert-chain-resolver/certUtil"
)

const (
	pemTypeCertificate = "CERTIFICATE"
)

type certificateChainFetcher func(cert *x509.Certificate) ([]*x509.Certificate, error)
type rootCAAdder func(certs []*x509.Certificate) ([]*x509.Certificate, error)
type pemEncoder func(out io.Writer, b *pem.Block) error

type Builder interface {
	fmt.Stringer

	FetchCertificatesFromTLSConnectionState(TLS *tls.ConnectionState) error
}

func NewBuilder() Builder {
	return &defaultBuilder{
		certificates:          make([]*x509.Certificate, 0),
		seenCertificates:      make(map[string]bool, 0),
		fetchCertificateChain: certUtil.FetchCertificateChain,
		addRootCA:             certUtil.AddRootCA,
		encodePEM:             pem.Encode,
		logger:                logrus.StandardLogger(),
	}
}

type defaultBuilder struct {
	certificates     []*x509.Certificate
	seenCertificates map[string]bool

	fetchCertificateChain certificateChainFetcher
	addRootCA             rootCAAdder
	encodePEM             pemEncoder

	logger logrus.FieldLogger
}

func (b *defaultBuilder) FetchCertificatesFromTLSConnectionState(TLS *tls.ConnectionState) error {
	for _, verifiedChain := range TLS.VerifiedChains {
		err := b.fetchCertificatesFromVerifiedChain(verifiedChain)
		if err != nil {
			return fmt.Errorf("error while fetching certificates into the CA Chain: %v", err)
		}
	}

	return nil
}

func (b *defaultBuilder) fetchCertificatesFromVerifiedChain(verifiedChain []*x509.Certificate) error {
	var err error

	if len(verifiedChain) < 1 {
		return nil
	}

	verifiedChain, err = b.fetchCertificateChain(verifiedChain[0])
	if err != nil {
		return fmt.Errorf("couldn't fetch certificates chain: %v", err)
	}

	verifiedChain, err = b.addRootCA(verifiedChain)
	if err != nil {
		return fmt.Errorf("couldn't add root CA to the chain: %v", err)
	}

	for _, certificate := range verifiedChain {
		b.addCertificate(certificate)
	}

	return nil
}

func (b *defaultBuilder) addCertificate(certificate *x509.Certificate) {
	signature := hex.EncodeToString(certificate.Signature)
	if b.seenCertificates[signature] {
		return
	}

	b.seenCertificates[signature] = true
	b.certificates = append(b.certificates, certificate)
}

func (b *defaultBuilder) String() string {
	out := bytes.NewBuffer(nil)
	for _, certificate := range b.certificates {
		err := b.encodePEM(out, &pem.Block{Type: pemTypeCertificate, Bytes: certificate.Raw})
		if err != nil {
			b.logger.WithError(err).Warning("Failed to encode certificate from chain")
		}
	}

	return strings.TrimSpace(out.String())
}
