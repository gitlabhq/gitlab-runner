package certificate

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"time"
)

const (
	x509CertificatePrivateKeyBits = 2048
	x509CertificateExpiryInYears  = 2
	x509CertificateSerialNumber   = 1
	x509CertificateOrganization   = "GitLab Runner"
)

type X509Generator struct{}

func (c X509Generator) Generate(host string) (tls.Certificate, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, x509CertificatePrivateKeyBits)
	if err != nil {
		return tls.Certificate{}, []byte{}, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(x509CertificateSerialNumber),
		NotBefore:    time.Now(),
		NotAfter:     time.Now().AddDate(x509CertificateExpiryInYears, 0, 0),
		Subject: pkix.Name{
			Organization: []string{x509CertificateOrganization},
		},
		KeyUsage: x509.KeyUsageKeyEncipherment |
			x509.KeyUsageDigitalSignature |
			x509.KeyUsageDataEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	if ip := net.ParseIP(host); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	} else {
		template.DNSNames = append(template.DNSNames, host)
	}

	publicKeyBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, priv.Public(), priv)
	if err != nil {
		return tls.Certificate{}, []byte{}, errors.New("failed to create certificate")
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: publicKeyBytes})
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	parsedCertificate, err := tls.X509KeyPair(publicKeyPEM, privateKeyPEM)
	if err != nil {
		return tls.Certificate{}, []byte{}, err
	}

	return parsedCertificate, publicKeyPEM, nil
}
