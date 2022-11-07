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
)

const (
	pemTypeCertificate = "CERTIFICATE"
)

type pemEncoder func(out io.Writer, b *pem.Block) error

type Builder interface {
	fmt.Stringer

	BuildChainFromTLSConnectionState(TLS *tls.ConnectionState) error
}

func NewBuilder(logger logrus.FieldLogger, resolveFullChain bool) Builder {
	logger = logger.
		WithField("context", "certificate-chain-build")

	return &defaultBuilder{
		certificates:     make([]*x509.Certificate, 0),
		seenCertificates: make(map[string]bool),
		resolver: newChainResolver(
			newURLResolver(logger),
			newVerifyResolver(logger),
		),
		encodePEM:        pem.Encode,
		logger:           logger,
		resolveFullChain: resolveFullChain,
	}
}

type defaultBuilder struct {
	certificates     []*x509.Certificate
	seenCertificates map[string]bool
	resolveFullChain bool

	resolver  resolver
	encodePEM pemEncoder

	logger logrus.FieldLogger
}

func (b *defaultBuilder) BuildChainFromTLSConnectionState(tls *tls.ConnectionState) error {
	for _, verifiedChain := range tls.VerifiedChains {
		b.logger.
			WithFields(logrus.Fields{
				"chain-leaf":         fmt.Sprintf("%v", verifiedChain),
				"resolve-full-chain": b.resolveFullChain,
			}).Debug("Processing chain")
		err := b.fetchCertificatesFromVerifiedChain(verifiedChain)
		if err != nil {
			return fmt.Errorf("error while fetching certificates into the CA Chain: %w", err)
		}
	}

	return nil
}

func (b *defaultBuilder) fetchCertificatesFromVerifiedChain(verifiedChain []*x509.Certificate) error {
	var err error

	if len(verifiedChain) < 1 {
		return nil
	}

	if b.resolveFullChain {
		verifiedChain, err = b.resolver.Resolve(verifiedChain)
		if err != nil {
			return fmt.Errorf("couldn't resolve certificates chain from the leaf certificate: %w", err)
		}
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
			b.logger.
				WithError(err).
				Warning("Failed to encode certificate from chain")
		}
	}

	return strings.TrimSpace(out.String())
}
