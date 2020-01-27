package gpg

import (
	"bytes"
	"fmt"
	"os"

	"golang.org/x/crypto/openpgp"
)

type Signer interface {
	SignFile(sourcePath string, targetPath string) error
}

type defaultSigner struct {
	entity *openpgp.Entity
}

func NewSigner(key string, password string) (Signer, error) {
	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewBufferString(key))
	if err != nil {
		return nil, fmt.Errorf("couldn't parse the armoed keys: %w", err)
	}

	if len(entities) < 1 {
		return nil, fmt.Errorf("no keys found")
	}

	entity := entities[0]
	err = entity.PrivateKey.Decrypt([]byte(password))
	if err != nil {
		return nil, fmt.Errorf("couldn't decrypt private key: %w", err)
	}

	signer := &defaultSigner{
		entity: entity,
	}

	return signer, nil
}

func (ds *defaultSigner) SignFile(sourcePath string, targetPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("couldn't open file for signing %q: %w", sourcePath, err)
	}

	targetFile, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("couldn't create signature file %q: %w", targetPath, err)
	}

	err = openpgp.ArmoredDetachSign(targetFile, ds.entity, sourceFile, nil)
	if err != nil {
		return fmt.Errorf("error while preparing detached GPG sign: %w", err)
	}

	return nil
}
