package gcp_secret_manager

import (
	"context"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/gcp_secret_manager/service"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/secrets"
)

const (
	resolverName = "gcp_secret_manager"
)

//go:generate mockery --name=client --inpackage
type client interface {
	GetSecret(ctx context.Context, s *common.GCPSecretManagerSecret) (string, error)
}

type resolver struct {
	secret common.Secret
	client client
}

func newResolver(secret common.Secret) common.SecretResolver {
	return &resolver{
		secret: secret,
		client: service.NewClient(),
	}
}

func (v *resolver) Name() string {
	return resolverName
}

func (v *resolver) IsSupported() bool {
	return v.secret.GCPSecretManager != nil
}

func (v *resolver) Resolve() (string, error) {
	if !v.IsSupported() {
		return "", secrets.NewResolvingUnsupportedSecretError(resolverName)
	}

	return v.client.GetSecret(context.Background(), v.secret.GCPSecretManager)
}

func init() {
	common.GetSecretResolverRegistry().Register(newResolver)
}
