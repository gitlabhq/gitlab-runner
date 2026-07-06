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

// ResolvingConfigurationError indicates that resolving a secret failed
// because of how the job or secrets provider is configured — for example
// the provider denied access (permission denied), the authentication role
// doesn't exist, or the secret path is invalid. These failures are caused
// by user or provider configuration, not by the runner or its host
// infrastructure, and are classified as a configuration failure instead of
// a runner system failure.
type ResolvingConfigurationError struct {
	Inner error
}

func NewResolvingConfigurationError(inner error) error {
	return &ResolvingConfigurationError{Inner: inner}
}

func (e *ResolvingConfigurationError) Error() string {
	return e.Inner.Error()
}

func (e *ResolvingConfigurationError) Unwrap() error {
	return e.Inner
}

// ResolvingExternalDependencyError indicates that resolving a secret failed
// because the external secrets provider itself failed — for example it
// responded with a server-side error. These failures are caused by an
// external dependency of the runner and are classified as an external
// dependency failure instead of a runner system failure.
type ResolvingExternalDependencyError struct {
	Inner error
}

func NewResolvingExternalDependencyError(inner error) error {
	return &ResolvingExternalDependencyError{Inner: inner}
}

func (e *ResolvingExternalDependencyError) Error() string {
	return e.Inner.Error()
}

func (e *ResolvingExternalDependencyError) Unwrap() error {
	return e.Inner
}
