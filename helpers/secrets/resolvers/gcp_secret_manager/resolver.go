package gcp_secret_manager

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/gcp_secret_manager/service"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/secrets"
)

const (
	resolverName = "gcp_secret_manager"
)

type client interface {
	GetSecret(ctx context.Context, s *spec.GCPSecretManagerSecret) (string, error)
}

type resolver struct {
	secret spec.Secret
	client client
}

func newResolver(secret spec.Secret) common.SecretResolver {
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

	value, err := v.client.GetSecret(context.Background(), v.secret.GCPSecretManager)
	if err != nil {
		return "", classifyError(err)
	}

	return value, nil
}

// classifyError reports IAM/lookup failures on the requested secret as a
// configuration error and Unavailable/Internal as an external dependency
// error, so the caller can report an accurate job failure reason instead of
// a generic runner system failure. Unauthenticated keeps the existing
// classification: it means the runner- or admin-configured workload
// identity federation trust is broken, not a job mistake. Any other failure
// is also returned unmodified.
func classifyError(err error) error {
	if err == nil {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	switch st.Code() {
	case codes.PermissionDenied, codes.NotFound:
		return secrets.NewResolvingConfigurationError(err)
	case codes.Unavailable, codes.Internal:
		return secrets.NewResolvingExternalDependencyError(err)
	default:
		return err
	}
}

func init() {
	common.GetSecretResolverRegistry().Register(newResolver)
}
