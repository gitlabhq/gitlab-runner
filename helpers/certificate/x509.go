package certificate

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"net"
	"time"
)

const (
	x509CertificatePrivateKeyBits = 2048
	x509CertificateExpiryInYears  = 2
	x509CertificateOrganization   = "GitLab Runner"
)

type X509Generator struct{}

func (c X509Generator) GenerateCA() ([]byte, []byte, *x509.Certificate, *ecdsa.PrivateKey, error) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	publicKey := privateKey.Public()

	tpl := &x509.Certificate{
		Subject: pkix.Name{
			Organization:       []string{"GitLab test CA"},
			OrganizationalUnit: []string{"group::runner core"},
			CommonName:         "test CA cert",
		},

		NotAfter:  time.Now().AddDate(x509CertificateExpiryInYears, 0, 0),
		NotBefore: time.Now(),

		KeyUsage: x509.KeyUsageCertSign,

		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	caCert, err := x509.CreateCertificate(rand.Reader, tpl, tpl, publicKey, privateKey)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	certTyped, err := x509.ParseCertificate(caCert)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	caCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert})
	caKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})

	return caCertPEM, caKeyPEM, certTyped, privateKey, nil
}

func (c X509Generator) Generate(host string) (tls.Certificate, []byte, error) {
	return c.GenerateWithCA(host, nil, nil)
}

func (c X509Generator) GenerateWithCA(host string, caCert *x509.Certificate, caPrivateKey any) (tls.Certificate, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, x509CertificatePrivateKeyBits)
	if err != nil {
		return tls.Certificate{}, nil, err
	}

	template := x509.Certificate{
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(x509CertificateExpiryInYears, 0, 0),
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

	if caCert == nil {
		caCert = &template
		caPrivateKey = priv
	}
	publicKeyBytes, err := x509.CreateCertificate(rand.Reader, &template, caCert, priv.Public(), caPrivateKey)
	if err != nil {
		return tls.Certificate{}, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: publicKeyBytes})
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	parsedCertificate, err := tls.X509KeyPair(publicKeyPEM, privateKeyPEM)
	if err != nil {
		return tls.Certificate{}, nil, err
	}

	return parsedCertificate, publicKeyPEM, nil
}
