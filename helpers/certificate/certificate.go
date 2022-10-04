package certificate

import "crypto/tls"

//go:generate mockery --name=Generator --inpackage
type Generator interface {
	Generate(host string) (tls.Certificate, []byte, error)
}
