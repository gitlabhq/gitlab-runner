package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/aws/service"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/secrets"
)

const (
	resolverName   = "aws_secrets_manager"
	contextTimeout = 30 * time.Second
)

type (
	AWSSecretsManager interface {
		GetSecretString(ctx context.Context, secretId string, versionId *string, versionStage *string) (string, error)
	}
)

var newAWSSecretsManagerService = func(ctx context.Context, region string, webIdentityProvider *stscreds.WebIdentityRoleProvider) (AWSSecretsManager, error) {
	return service.NewAWSSecretsManager(ctx, region, webIdentityProvider)
}

type resolver struct {
	secret spec.Secret
}

func newResolver(secret spec.Secret) common.SecretResolver {
	return &resolver{
		secret: secret,
	}
}

func (v *resolver) Name() string {
	return resolverName
}

func (v *resolver) IsSupported() bool {
	return v.secret.AWSSecretsManager != nil
}

func (v *resolver) getRegion() string {
	if v.secret.AWSSecretsManager.Region != "" {
		return v.secret.AWSSecretsManager.Region
	}
	return v.secret.AWSSecretsManager.Server.Region
}

func (v *resolver) getRoleArn() string {
	if v.secret.AWSSecretsManager.Server.RoleArn != "" {
		return v.secret.AWSSecretsManager.Server.RoleArn
	}
	return v.secret.AWSSecretsManager.RoleARN
}

func (v *resolver) getRoleSessionName() string {
	if v.secret.AWSSecretsManager.Server.RoleSessionName != "" {
		return v.secret.AWSSecretsManager.Server.RoleSessionName
	}
	return v.secret.AWSSecretsManager.RoleSessionName
}

func (v *resolver) getVersionId() *string {
	if v.secret.AWSSecretsManager.VersionId != "" {
		return &v.secret.AWSSecretsManager.VersionId
	}
	return nil
}

func (v *resolver) getVersionStage() *string {
	if v.secret.AWSSecretsManager.VersionStage != "" {
		return &v.secret.AWSSecretsManager.VersionStage
	}
	return nil
}

func (v *resolver) Resolve() (string, error) {
	if !v.IsSupported() {
		return "", secrets.NewResolvingUnsupportedSecretError(resolverName)
	}

	region := v.getRegion()
	roleArn := v.getRoleArn()
	roleSessionName := v.getRoleSessionName()

	if roleArn == "" && v.secret.AWSSecretsManager.Server.JWT != "" {
		return "", fmt.Errorf("Role ARN is required when using JWT for AWS authentication")
	}

	var identity *stscreds.WebIdentityRoleProvider
	if roleArn != "" {
		identity = service.NewWebIdentityRoleProvider(region, roleArn, v.secret.AWSSecretsManager.Server.JWT, roleSessionName)
	}

	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	secret := v.secret.AWSSecretsManager

	s, err := newAWSSecretsManagerService(ctx, region, identity)
	if err != nil {
		return "", err
	}

	data, err := s.GetSecretString(ctx, secret.SecretId, v.getVersionId(), v.getVersionStage())
	if err != nil {
		return "", err
	}

	if secret.Field != "" {
		return extractFlatJSONField(data, secret.Field, secret.SecretId)
	}

	return data, nil
}

func extractFlatJSONField(jsonStr, field, secretId string) (string, error) {
	m := map[string]any{}
	err := json.Unmarshal([]byte(jsonStr), &m)
	if err != nil {
		return "", fmt.Errorf("failed to parse JSON for secret '%s': %w", secretId, err)
	}
	val, ok := m[field]
	if !ok {
		return "", fmt.Errorf(
			"key '%s' not found in AWS Secrets Manager response for secret '%s'", field, secretId)
	}

	// To unmarshal JSON into an interface value,
	// Unmarshal stores one of these in the interface value:
	//
	// - bool, for JSON booleans
	// - float64, for JSON numbers
	// - string, for JSON strings
	// - []any, for JSON arrays
	// - map[string]any, for JSON objects
	// - nil for JSON null
	//
	// We only support string, number and boolean types for now,
	// as that is what the AWS Secrets Manager Web UI can handle.
	// The Web UI will show
	// "The secret value can't be converted to key name and value pairs."
	// for null values and complex types like arrays and objects.
	// Even though the AWS Secrets Manager API allows
	// storing and retreiving them.

	switch val.(type) {
	case string, float64, bool:
		return fmt.Sprint(val), nil
	default:
		return "", fmt.Errorf("key '%s' in aws secrets manager response for secret '%s' is not a string, number or boolean", field, secretId)
	}
}

func init() {
	common.GetSecretResolverRegistry().Register(newResolver)
}
