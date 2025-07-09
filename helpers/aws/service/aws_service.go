package service

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

type SecretsManagerAPI interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

type awsSecretsManager struct {
	client SecretsManagerAPI
}

type staticWebTokenRetriever struct {
	Token string
}

func (s *staticWebTokenRetriever) GetIdentityToken() ([]byte, error) {
	return []byte(s.Token), nil
}

func NewWebIdentityRoleProvider(region, roleArn, token, roleSessionName string) *stscreds.WebIdentityRoleProvider {
	awsConfig := aws.NewConfig()
	awsConfig.Region = region
	stsClient := sts.NewFromConfig(*awsConfig)

	return stscreds.NewWebIdentityRoleProvider(stsClient, roleArn, &staticWebTokenRetriever{
		Token: token,
	}, func(o *stscreds.WebIdentityRoleOptions) {
		o.RoleSessionName = roleSessionName
	})
}

func NewAWSSecretsManager(ctx context.Context, region string, webIdentityProvider *stscreds.WebIdentityRoleProvider) (*awsSecretsManager, error) {
	var cfg aws.Config
	var err error

	if webIdentityProvider != nil {
		cfg = aws.Config{
			Region:      region,
			Credentials: webIdentityProvider,
		}
	} else {
		cfg, err = config.LoadDefaultConfig(ctx,
			config.WithRegion(region),
		)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config with region %s: %w", region, err)
	}

	v := &awsSecretsManager{
		client: secretsmanager.NewFromConfig(cfg),
	}

	return v, nil
}

func (v *awsSecretsManager) GetSecretString(ctx context.Context, secretId string, versionId *string, versionStage *string) (string, error) {
	resp, err := v.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId:     &secretId,
		VersionId:    versionId,
		VersionStage: versionStage,
	})

	if err != nil {
		return "", err
	}
	if resp.SecretString != nil {
		return *resp.SecretString, nil
	}
	if resp.SecretBinary != nil {
		return base64.StdEncoding.EncodeToString(resp.SecretBinary), nil
	}
	return "", fmt.Errorf("secret contains no value")
}
