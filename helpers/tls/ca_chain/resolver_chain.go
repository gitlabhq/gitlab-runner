// Inspired by https://github.com/zakjan/cert-chain-resolver/blob/1.0.3/certUtil/chain.go
// which is licensed on a MIT license.
//
// Shout out to Jan Žák (http://zakjan.cz) original author of `certUtil` package and other
// contributors who updated it!

package ca_chain

import (
	"crypto/x509"
	"fmt"
)

type chainResolver struct {
	urlResolver    resolver
	verifyResolver resolver
}

func newChainResolver(urlResolver, verifyResolver resolver) resolver {
	return &chainResolver{
		urlResolver:    urlResolver,
		verifyResolver: verifyResolver,
	}
}

func (r *chainResolver) Resolve(certs []*x509.Certificate) ([]*x509.Certificate, error) {
	certs, err := r.urlResolver.Resolve(certs)
	if err != nil {
		return nil, fmt.Errorf("error while resolving certificates chain with URL: %w", err)
	}

	certs, err = r.verifyResolver.Resolve(certs)
	if err != nil {
		return nil, fmt.Errorf("error while resolving certificates chain with verification: %w", err)
	}

	return certs, err
}
