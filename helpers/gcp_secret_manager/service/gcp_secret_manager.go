package service

import (
	"context"
	"fmt"
	"hash/crc32"

	sm "cloud.google.com/go/secretmanager/apiv1"
	smpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"golang.org/x/oauth2"
	"google.golang.org/api/option"
	"google.golang.org/api/sts/v1"
)

const (
	grantType            = "urn:ietf:params:oauth:grant-type:token-exchange"
	requestedTokenType   = "urn:ietf:params:oauth:token-type:access_token"
	subjectTokenTypeOIDC = "urn:ietf:params:oauth:token-type:id_token"
	gcpAuthScope         = "https://www.googleapis.com/auth/cloud-platform"
)

type Client struct {
	auth authenticator
	svc  service
}

func NewClient() Client {
	return Client{
		auth: authFunc(getToken),
		svc:  accessFunc(access),
	}
}

func (c Client) GetSecret(ctx context.Context, secret *common.GCPSecretManagerSecret) (string, error) {
	tokenResponse, err := c.auth.getToken(ctx, secret)
	if err != nil {
		return "", fmt.Errorf("failed to exchange sts token: %w", err)
	}

	tokenSource := toTokenSource(tokenResponse)

	accessSecretVersionResponse, err := c.svc.access(ctx, secret, tokenSource)

	if err != nil {
		return "", fmt.Errorf("failed to get secret: %w", err)
	}

	if accessSecretVersionResponse.Payload == nil {
		return "", common.ErrSecretNotFound
	}

	if !validChecksum(accessSecretVersionResponse.Payload) {
		return "", fmt.Errorf("data corruption detected")
	}

	return string(accessSecretVersionResponse.Payload.Data), nil
}

//go:generate mockery --name=authenticator --inpackage
type authenticator interface {
	getToken(ctx context.Context, secret *common.GCPSecretManagerSecret) (*sts.GoogleIdentityStsV1ExchangeTokenResponse, error)
}

type authFunc func(ctx context.Context, secret *common.GCPSecretManagerSecret) (*sts.GoogleIdentityStsV1ExchangeTokenResponse, error)

func (f authFunc) getToken(ctx context.Context, secret *common.GCPSecretManagerSecret) (*sts.GoogleIdentityStsV1ExchangeTokenResponse, error) {
	return f(ctx, secret)
}

func getToken(ctx context.Context, secret *common.GCPSecretManagerSecret) (*sts.GoogleIdentityStsV1ExchangeTokenResponse, error) {
	// option.WithoutAuthentication() is required for STS service.
	// https://cloud.google.com/iam/docs/reference/sts/rest/v1/TopLevel/token
	// specifies clients NOT to send `Authorization` header. Without this option,
	// the request would include `Authorization` header and the request would fail.
	stsService, err := sts.NewService(ctx, option.WithoutAuthentication())
	if err != nil {
		return nil, fmt.Errorf("unable to create sts service client: %w", err)
	}

	stsTokenRequest := &sts.GoogleIdentityStsV1ExchangeTokenRequest{
		Audience:           stsAudience(secret),
		GrantType:          grantType,
		RequestedTokenType: requestedTokenType,
		Scope:              gcpAuthScope,
		SubjectToken:       secret.Server.JWT,
		SubjectTokenType:   subjectTokenTypeOIDC,
	}

	return stsService.V1.Token(stsTokenRequest).Do()
}

func stsAudience(secret *common.GCPSecretManagerSecret) string {
	return fmt.Sprintf(
		"//iam.googleapis.com/projects/%s/locations/global/workloadIdentityPools/%s/providers/%s",
		secret.Server.ProjectNumber,
		secret.Server.WorkloadIdentityFederationPoolId,
		secret.Server.WorkloadIdentityFederationProviderID)
}

//go:generate mockery --name=service --inpackage
type service interface {
	access(ctx context.Context, secret *common.GCPSecretManagerSecret, source oauth2.TokenSource) (*smpb.AccessSecretVersionResponse, error)
}

type accessFunc func(ctx context.Context, secret *common.GCPSecretManagerSecret, source oauth2.TokenSource) (*smpb.AccessSecretVersionResponse, error)

func (f accessFunc) access(ctx context.Context, secret *common.GCPSecretManagerSecret, source oauth2.TokenSource) (*smpb.AccessSecretVersionResponse, error) {
	return f(ctx, secret, source)
}

func access(ctx context.Context, secret *common.GCPSecretManagerSecret, source oauth2.TokenSource) (*smpb.AccessSecretVersionResponse, error) {
	smClient, err := sm.NewClient(ctx, option.WithTokenSource(source))
	if err != nil {
		return nil, fmt.Errorf("unable to create secrets manager client: %w", err)
	}

	smAccessSecretVersionRequest := &smpb.AccessSecretVersionRequest{
		Name: secretVersionResourceName(secret),
	}

	return smClient.AccessSecretVersion(ctx, smAccessSecretVersionRequest)
}

func toTokenSource(resp *sts.GoogleIdentityStsV1ExchangeTokenResponse) oauth2.TokenSource {
	return oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: resp.AccessToken,
		TokenType:   resp.TokenType,
	})
}

func secretVersionResourceName(secret *common.GCPSecretManagerSecret) string {
	return fmt.Sprintf("projects/%s/secrets/%s/versions/%s", secret.Server.ProjectNumber, secret.Name, secret.Version)
}

func validChecksum(payload *smpb.SecretPayload) bool {
	crc32c := crc32.MakeTable(crc32.Castagnoli)
	checksum := int64(crc32.Checksum(payload.Data, crc32c))

	return checksum == *payload.DataCrc32C
}
