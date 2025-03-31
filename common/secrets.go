package common

import (
	"errors"
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

type logger interface {
	Println(args ...interface{})
	Warningln(args ...interface{})
}

type SecretsResolver interface {
	Resolve(secrets Secrets) (JobVariables, error)
}

type SecretResolverRegistry interface {
	Register(f secretResolverFactory)
	GetFor(secret Secret) (SecretResolver, error)
}

type secretResolverFactory func(secret Secret) SecretResolver

type SecretResolver interface {
	Name() string
	IsSupported() bool
	Resolve() (string, error)
}

var (
	secretResolverRegistry = new(defaultSecretResolverRegistry)

	ErrMissingLogger = errors.New("logger not provided")

	ErrMissingSecretResolver = errors.New("no resolver that can handle the secret")

	ErrSecretNotFound = errors.New("secret not found")
)

func GetSecretResolverRegistry() SecretResolverRegistry {
	return secretResolverRegistry
}

type defaultSecretResolverRegistry struct {
	factories []secretResolverFactory
}

func (r *defaultSecretResolverRegistry) Register(f secretResolverFactory) {
	r.factories = append(r.factories, f)
}

func (r *defaultSecretResolverRegistry) GetFor(secret Secret) (SecretResolver, error) {
	for _, f := range r.factories {
		sr := f(secret)
		if sr.IsSupported() {
			return sr, nil
		}
	}

	return nil, ErrMissingSecretResolver
}

func newSecretsResolver(l logger, registry SecretResolverRegistry, featureFlagOn func(string) bool) (SecretsResolver, error) {
	if l == nil {
		return nil, ErrMissingLogger
	}

	sr := &defaultSecretsResolver{
		logger:                 l,
		secretResolverRegistry: registry,
		featureFlagOn:          featureFlagOn,
	}

	return sr, nil
}

type defaultSecretsResolver struct {
	logger                 logger
	secretResolverRegistry SecretResolverRegistry
	featureFlagOn          func(string) bool
}

func (r *defaultSecretsResolver) Resolve(secrets Secrets) (JobVariables, error) {
	if secrets == nil {
		return nil, nil
	}

	msg := fmt.Sprintf(
		"%sResolving secrets%s",
		helpers.ANSI_BOLD_CYAN,
		helpers.ANSI_RESET,
	)
	r.logger.Println(msg)

	variables := make(JobVariables, 0)
	for variableKey, secret := range secrets {
		r.logger.Println(fmt.Sprintf("Resolving secret %q...", variableKey))

		v, err := r.handleSecret(variableKey, secret)
		if err != nil {
			return nil, err
		}

		if v != nil {
			variables = append(variables, *v)
		}
	}

	return variables, nil
}

func (r *defaultSecretsResolver) handleSecret(variableKey string, secret Secret) (*JobVariable, error) {
	sr, err := r.secretResolverRegistry.GetFor(secret)
	if err != nil {
		r.logger.Warningln(fmt.Sprintf("Not resolved: %v", err))
		return nil, nil
	}

	r.logger.Println(fmt.Sprintf("Using %q secret resolver...", sr.Name()))

	value, err := sr.Resolve()
	if errors.Is(err, ErrSecretNotFound) {
		if !r.featureFlagOn(featureflags.EnableSecretResolvingFailsIfMissing) {
			err = nil
		} else {
			err = fmt.Errorf("%w: %v", err, variableKey)
		}
	}
	if err != nil {
		return nil, err
	}

	variable := &JobVariable{
		Key:    variableKey,
		Value:  value,
		File:   secret.IsFile(),
		Masked: true,
		Raw:    true,
	}

	return variable, nil
}
