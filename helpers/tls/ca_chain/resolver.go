package ca_chain

import (
	"crypto/x509"
)

//go:generate mockery --name=resolver --inpackage
type resolver interface {
	Resolve(certs []*x509.Certificate) ([]*x509.Certificate, error)
}
