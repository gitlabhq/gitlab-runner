package secrets

import (
	"fmt"
)

type ResolvingUnsupportedSecretError struct {
	name string
}

func NewResolvingUnsupportedSecretError(name string) error {
	return &ResolvingUnsupportedSecretError{name: name}
}

func (e *ResolvingUnsupportedSecretError) Error() string {
	return fmt.Sprintf("trying to resolve unsupported secret: %s", e.name)
}

func (e *ResolvingUnsupportedSecretError) Is(err error) bool {
	customErr, ok := err.(*ResolvingUnsupportedSecretError)
	if !ok {
		return false
	}

	return customErr.name == e.name
}
