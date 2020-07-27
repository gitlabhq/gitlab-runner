package common

type SecretsResolver interface {
	Resolve(secrets Secrets) (JobVariables, error)
}

var secretsResolver = new(defaultResolver)

var getSecretsResolver = func() SecretsResolver {
	return secretsResolver
}

type defaultResolver struct{}

func (r *defaultResolver) Resolve(secrets Secrets) (JobVariables, error) {
	return nil, nil
}
