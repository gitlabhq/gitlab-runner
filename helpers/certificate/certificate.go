package certificate

import "crypto/tls"

type Generator interface {
	Generate(host string) (tls.Certificate, []byte, error)
}
