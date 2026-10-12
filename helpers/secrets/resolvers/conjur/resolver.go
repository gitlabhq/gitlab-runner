package conjur

import (
	"context"
	"errors"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/conjur/service"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/secrets"
)

const resolverName = "conjur"

type client interface {
	GetSecret(ctx context.Context, secret *spec.ConjurSecret) (string, error)
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

func (r *resolver) Name() string {
	return resolverName
}

func (r *resolver) IsSupported() bool {
	return r.secret.Conjur != nil
}

func (r *resolver) Resolve() (string, error) {
	if !r.IsSupported() {
		return "", secrets.NewResolvingUnsupportedSecretError(resolverName)
	}

	conjurSecret := r.secret.Conjur

	// Rails gates attaching the server block behind a feature flag while the
	// job can still be dispatched, so a conjur secret with no server is a
	// valid state that must be reported clearly instead of failing deep
	// inside the HTTP client.
	s := conjurSecret.Server
	for _, f := range []struct{ name, value string }{
		{"appliance_url", s.ApplianceURL},
		{"account", s.Account},
		{"authn_jwt_service_id", s.AuthnJWTServiceID},
		{"jwt", s.JWT},
		{"path", conjurSecret.Path},
	} {
		if f.value == "" {
			return "", secrets.NewResolvingConfigurationError(errors.New("conjur configuration missing " + f.name))
		}
	}

	value, err := r.client.GetSecret(context.Background(), conjurSecret)
	if err != nil {
		return "", classifyError(err)
	}

	return value, nil
}

// apiStatusCoder is implemented by the Conjur API errors that back this
// resolver, carrying the HTTP status code of the API response.
type apiStatusCoder interface {
	StatusCode() int
}

// classifyError reports authentication/lookup failures caused by job or
// appliance configuration (4xx) as a configuration error and appliance-side
// failures (5xx) as an external dependency error, so the caller can report an
// accurate job failure reason instead of a generic runner system failure. Any
// other failure, including a transport-level error, is returned unmodified.
func classifyError(err error) error {
	if err == nil {
		return nil
	}

	var apiErr apiStatusCoder
	if !errors.As(err, &apiErr) {
		return err
	}

	switch code := apiErr.StatusCode(); {
	case code >= 400 && code < 500:
		return secrets.NewResolvingConfigurationError(err)
	case code >= 500:
		return secrets.NewResolvingExternalDependencyError(err)
	case code >= 300:
		return secrets.NewResolvingConfigurationError(err)
	default:
		return err
	}
}

func init() {
	common.GetSecretResolverRegistry().Register(newResolver)
}
