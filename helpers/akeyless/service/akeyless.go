package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/akeylesslabs/akeyless-go-cloud-id/cloudprovider/aws"
	"github.com/akeylesslabs/akeyless-go-cloud-id/cloudprovider/azure"
	"github.com/akeylesslabs/akeyless-go-cloud-id/cloudprovider/gcp"
	akeyless_api "github.com/akeylesslabs/akeyless-go/v3"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

//go:generate mockery --name=akeylessAPIClient --inpackage
type akeylessAPIClient interface {
	GetSecretValue(ctx context.Context, body akeyless_api.GetSecretValue) (map[string]string, error)
	Auth(ctx context.Context, params akeyless_api.Auth) (akeyless_api.AuthOutput, error)
	DescribeItem(ctx context.Context, params akeyless_api.DescribeItem) (akeyless_api.Item, error)
	GetDynamicSecretValue(ctx context.Context, params akeyless_api.GetDynamicSecretValue) (map[string]string, error)
	GetRotatedSecretValue(ctx context.Context, params akeyless_api.GetRotatedSecretValue) (map[string]any, error)
	GetSSHCertificate(ctx context.Context, params akeyless_api.GetSSHCertificate) (akeyless_api.GetSSHCertificateOutput, error)
	GetPKICertificate(ctx context.Context, params akeyless_api.GetPKICertificate) (akeyless_api.GetPKICertificateOutput, error)
}

type akeylessClient struct {
	api *akeyless_api.V2ApiService
}

func newClient(secret *common.AkeylessSecret) *akeylessClient {
	apiService := akeyless_api.NewAPIClient(&akeyless_api.Configuration{
		Servers:       []akeyless_api.ServerConfiguration{{URL: secret.Server.AkeylessApiUrl}},
		DefaultHeader: map[string]string{"akeylessclienttype": "gitlab"},
	}).V2Api

	return &akeylessClient{
		api: apiService,
	}
}

func (c *akeylessClient) GetSecretValue(ctx context.Context, body akeyless_api.GetSecretValue) (map[string]string, error) {
	resp, _, err := c.api.GetSecretValue(ctx).Body(body).Execute()
	return resp, err
}

func (c *akeylessClient) Auth(ctx context.Context, body akeyless_api.Auth) (akeyless_api.AuthOutput, error) {
	out, _, err := c.api.Auth(ctx).Body(body).Execute()
	return out, err
}

func (c *akeylessClient) DescribeItem(ctx context.Context, body akeyless_api.DescribeItem) (akeyless_api.Item, error) {
	out, _, err := c.api.DescribeItem(ctx).Body(body).Execute()
	return out, err
}

func (c *akeylessClient) GetDynamicSecretValue(ctx context.Context, body akeyless_api.GetDynamicSecretValue) (map[string]string, error) {
	resp, _, err := c.api.GetDynamicSecretValue(ctx).Body(body).Execute()
	return resp, err
}

func (c *akeylessClient) GetRotatedSecretValue(ctx context.Context, body akeyless_api.GetRotatedSecretValue) (map[string]any, error) {
	resp, _, err := c.api.GetRotatedSecretValue(ctx).Body(body).Execute()
	return resp, err
}

func (c *akeylessClient) GetSSHCertificate(ctx context.Context, body akeyless_api.GetSSHCertificate) (akeyless_api.GetSSHCertificateOutput, error) {
	resp, _, err := c.api.GetSSHCertificate(ctx).Body(body).Execute()
	return resp, err
}

func (c *akeylessClient) GetPKICertificate(ctx context.Context, body akeyless_api.GetPKICertificate) (akeyless_api.GetPKICertificateOutput, error) {
	resp, _, err := c.api.GetPKICertificate(ctx).Body(body).Execute()
	return resp, err
}

type AccessType string

const (
	AccessTypeApiKey  AccessType = "api_key"
	AccessTypeAwsIAM  AccessType = "aws_iam"
	AccessTypeAzureAd AccessType = "azure_ad"
	AccessTypeGCP     AccessType = "gcp"
	AccessTypeUid     AccessType = "universal_identity"
	AccessTypeK8S     AccessType = "k8s"
	AccessTypeJWT     AccessType = "jwt"
)

type ItemType string

const (
	ItemTypeStaticSecret  ItemType = "STATIC_SECRET"
	ItemTypeDynamicSecret ItemType = "DYNAMIC_SECRET"
	ItemTypeRotatedSecret ItemType = "ROTATED_SECRET"
	ItemTypeSSHCertIssuer ItemType = "SSH_CERT_ISSUER"
	ItemTypePkiCertIssuer ItemType = "PKI_CERT_ISSUER"
)

type Config struct {
	AccessType             AccessType
	AccessId               string
	AccessKey              string
	ApiURL                 string
	AzureObjectId          string
	Path                   string
	GcpAudience            string
	UidToken               string
	K8SServiceAccountToken string
	K8SAuthConfigName      string
	JWT                    string
}

//go:generate mockery --name=Akeyless --inpackage
type Akeyless interface {
	GetSecret(ctx context.Context) (any, error)
}

type AkeylessAPI struct {
	secret *common.AkeylessSecret
	client akeylessAPIClient
}

func NewAkeyless(secret *common.AkeylessSecret) *AkeylessAPI {
	return &AkeylessAPI{
		secret: secret,
		client: newClient(secret),
	}
}

func (v *AkeylessAPI) GetSecret(ctx context.Context) (any, error) {
	token := v.secret.Server.AkeylessToken
	if token == "" {
		var err error
		token, err = v.authenticate(ctx, v.secret.Server)
		if err != nil {
			return nil, err
		}
	}

	if v.secret.Name == "" {
		return token, nil
	}

	return v.getSecret(ctx, token)
}

func (v *AkeylessAPI) authenticate(ctx context.Context, server common.AkeylessServer) (string, error) {
	authParams, err := setupAuthParams(server)
	if err != nil {
		return "", err
	}

	out, err := v.client.Auth(ctx, *authParams)
	if err != nil {
		return "", getAklApiErrMsg(err)
	}

	return out.GetToken(), nil
}

func (v *AkeylessAPI) getSecret(ctx context.Context, token string) (any, error) {
	name := v.secret.Name
	itemType, err := v.getItemType(ctx, name, token)
	if err != nil {
		return nil, err
	}

	var value any

	switch ItemType(itemType) {
	case ItemTypeStaticSecret:
		value, err = v.getStaticSecret(ctx, name, token)
	case ItemTypeDynamicSecret:
		value, err = v.getDynamicSecret(ctx, name, token)
	case ItemTypeRotatedSecret:
		value, err = v.getRotatedSecret(ctx, name, token)
	case ItemTypeSSHCertIssuer:
		value, err = v.getSSHCertificate(ctx, name, v.secret.CertUserName, v.secret.PublicKeyData, token)
	case ItemTypePkiCertIssuer:
		value, err = v.getPKICertificate(ctx, name, v.secret.CsrData, v.secret.PublicKeyData, token)
	default:
		return nil, fmt.Errorf("unknown item type: %s", itemType)
	}

	// since we return all kinds of values avoid wrapping the value in the `any` type,
	// so it's consistently nil
	if err != nil {
		return nil, err
	}

	return value, nil
}

func (v *AkeylessAPI) getStaticSecret(ctx context.Context, name, token string) (any, error) {
	secretsVal, err := v.client.GetSecretValue(ctx, akeyless_api.GetSecretValue{
		Names: []string{name},
		Token: akeyless_api.PtrString(token),
	})
	if err != nil {
		return nil, err
	}
	if val, ok := secretsVal[name]; ok {
		return val, nil
	}

	return nil, getSecretNotFoundError(name)
}

func (v *AkeylessAPI) getDynamicSecret(ctx context.Context, name string, token string) (any, error) {
	return v.client.GetDynamicSecretValue(ctx, akeyless_api.GetDynamicSecretValue{
		Name:  name,
		Token: akeyless_api.PtrString(token),
	})
}

func (v *AkeylessAPI) getRotatedSecret(ctx context.Context, name string, token string) (any, error) {
	resp, err := v.client.GetRotatedSecretValue(ctx, akeyless_api.GetRotatedSecretValue{
		Names: name,
		Token: akeyless_api.PtrString(token),
	})
	if err != nil {
		return nil, err
	}
	if val, ok := resp["value"]; ok {
		return val, nil
	}
	return nil, getSecretNotFoundError(name)
}

func (v *AkeylessAPI) getSSHCertificate(ctx context.Context, name, certUserName, publicKeyData, token string) (any, error) {
	resp, err := v.client.GetSSHCertificate(ctx, akeyless_api.GetSSHCertificate{
		CertIssuerName: name,
		CertUsername:   certUserName,
		PublicKeyData:  akeyless_api.PtrString(publicKeyData),
		Token:          akeyless_api.PtrString(token),
	})
	if err != nil {
		return nil, err
	}
	if resp.Data != nil {
		return *resp.Data, nil
	}

	return nil, getSecretNotFoundError(name)
}

func (v *AkeylessAPI) getPKICertificate(ctx context.Context, name, csrData, publicKeyData, token string) (any, error) {
	resp, err := v.client.GetPKICertificate(ctx, akeyless_api.GetPKICertificate{
		CertIssuerName: name,
		CsrDataBase64:  akeyless_api.PtrString(csrData),
		KeyDataBase64:  akeyless_api.PtrString(publicKeyData),
		Token:          akeyless_api.PtrString(token),
	})
	if err != nil {
		return nil, err
	}

	if resp.Data != nil {
		return *resp.Data, nil
	}

	return nil, getSecretNotFoundError(name)
}

func (v *AkeylessAPI) getItemType(ctx context.Context, name, token string) (string, error) {
	describeItemOut, err := v.client.DescribeItem(ctx, akeyless_api.DescribeItem{
		Name:  name,
		Token: akeyless_api.PtrString(token),
	})
	if err != nil {
		return "", getAklApiErrMsg(err)
	}

	return describeItemOut.GetItemType(), nil
}

func setupAuthParams(server common.AkeylessServer) (*akeyless_api.Auth, error) {
	authParams := akeyless_api.NewAuth()
	authParams.SetAccessType(server.AkeylessAccessType)
	authParams.SetAccessId(server.AccessId)

	if server.GatewayCaCert != "" {
		authParams.SetCertData(server.GatewayCaCert)
	}

	switch AccessType(server.AkeylessAccessType) {
	case AccessTypeApiKey:
		authParams.SetAccessKey(server.AccessKey)
	case AccessTypeAwsIAM:
		id, err := aws.GetCloudId()
		if err != nil {
			return nil, fmt.Errorf("failed to get AWS cloud id: %w", err)
		}
		authParams.SetCloudId(id)

	case AccessTypeAzureAd:
		id, err := azure.GetCloudId(server.AzureObjectId)
		if err != nil {
			return nil, fmt.Errorf("failed to get azure cloud id: %w", err)
		}
		if _, err := base64.StdEncoding.DecodeString(id); err != nil {
			id = base64.StdEncoding.EncodeToString([]byte(id))
		}
		authParams.SetCloudId(id)

	case AccessTypeGCP:
		id, err := gcp.GetCloudID(server.GcpAudience)
		if err != nil {
			return nil, fmt.Errorf("failed to get GCP cloud id: %w", err)
		}
		authParams.SetCloudId(id)

	case AccessTypeUid:
		if server.UidToken == "" {
			return nil, fmt.Errorf("UidToken is required for access type %q", AccessTypeUid)
		}
		authParams.SetUidToken(server.UidToken)

	case AccessTypeK8S:
		authParams.SetGatewayUrl(server.AkeylessApiUrl)
		authParams.SetK8sServiceAccountToken(server.K8SServiceAccountToken)
		authParams.SetK8sAuthConfigName(server.K8SAuthConfigName)

	case AccessTypeJWT:
		authParams.SetJwt(server.JWT)

	default:
		return nil, fmt.Errorf("unknown Access type: %s", server.AkeylessAccessType)
	}

	return authParams, nil
}

func getSecretNotFoundError(name string) error {
	return fmt.Errorf("secret %v not found", name)
}

func getAklApiErrMsg(err error) error {
	msg := "no response body"

	var apiErr akeyless_api.GenericOpenAPIError
	if errors.As(err, &apiErr) {
		msg = string(apiErr.Body())
	}

	return fmt.Errorf("can't authenticate with static creds: %s: %w", msg, err)
}
