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

type AccessType string

const (
	AccessTypeApiKey  AccessType = "api_key"
	AccessTypeAwsIAM  AccessType = "aws_iam"
	AccessTypeAzureAd AccessType = "azure_ad"
	AccessTypeGCP     AccessType = "gcp"
	AccessTypeUid     AccessType = "universal_identity"
	K8S               AccessType = "k8s"
	JWT               AccessType = "jwt"
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
	GetAkeylessSecret(ctx context.Context, secret *common.AkeylessSecret) (any, error)
}
type defaultAkeyless struct {
	authFunc      authenticateFunc
	getSecretFunc getSecretFunc
}

func NewAkeyless() Akeyless {
	return &defaultAkeyless{
		authFunc:      authenticate,
		getSecretFunc: getSecret,
	}
}

func (v *defaultAkeyless) GetAkeylessSecret(ctx context.Context, secret *common.AkeylessSecret) (any, error) {
	apiService := akeyless_api.NewAPIClient(&akeyless_api.Configuration{
		Servers:       []akeyless_api.ServerConfiguration{{URL: secret.Server.AkeylessApiUrl}},
		DefaultHeader: map[string]string{"akeylessclienttype": "gitlab"},
	}).V2Api

	token, err := v.authFunc(ctx, secret.Server, apiService)
	if err != nil {
		return nil, err
	}

	return v.getSecretFunc(ctx, secret, token, apiService)
}

type getSecretFunc func(ctx context.Context, secret *common.AkeylessSecret, token string, apiService *akeyless_api.V2ApiService) (any, error)

func getSecret(ctx context.Context, secret *common.AkeylessSecret, token string, apiService *akeyless_api.V2ApiService) (any, error) {
	name := secret.Name
	itemType, err := getItemType(ctx, name, token, apiService)
	if err != nil {
		return nil, err
	}

	switch ItemType(itemType) {
	case ItemTypeStaticSecret:
		return getStaticSecret(ctx, name, token, apiService)
	case ItemTypeDynamicSecret:
		return getDynamicSecret(ctx, name, token, apiService)
	case ItemTypeRotatedSecret:
		return getRotatedSecret(ctx, name, token, apiService)
	case ItemTypeSSHCertIssuer:
		return getSSHCertificate(ctx, name, secret.CertUserName, secret.PublicKeyData, token, apiService)
	case ItemTypePkiCertIssuer:
		return getPKICertificate(ctx, name, secret.CsrData, secret.PublicKeyData, token, apiService)
	default:
		return nil, fmt.Errorf("unknown item type: %s", itemType)
	}
}

type authenticateFunc func(ctx context.Context, server common.AkeylessServer, apiService *akeyless_api.V2ApiService) (string, error)

func authenticate(ctx context.Context, server common.AkeylessServer, apiService *akeyless_api.V2ApiService) (string, error) {
	authParams, err := setupAuthParams(server)
	if err != nil {
		return "", err
	}

	out, _, err := apiService.Auth(ctx).Body(*authParams).Execute()
	if err != nil {
		return "", getAklApiErrMsg(err)
	}
	return out.GetToken(), nil
}

func getStaticSecret(ctx context.Context, name, token string, apiService *akeyless_api.V2ApiService) (string, error) {
	secretsVal, _, err := apiService.GetSecretValue(ctx).Body(akeyless_api.GetSecretValue{
		Names: []string{name},
		Token: akeyless_api.PtrString(token),
	}).Execute()
	if err != nil {
		return "", err
	}
	if val, ok := secretsVal[name]; ok {
		return val, nil
	}
	return "", getSecretNotFoundError(name)
}

func getDynamicSecret(ctx context.Context, name string, token string, apiService *akeyless_api.V2ApiService) (any, error) {
	resp, _, err := apiService.GetDynamicSecretValue(ctx).Body(akeyless_api.GetDynamicSecretValue{
		Name:  name,
		Token: akeyless_api.PtrString(token),
	}).Execute()
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func getRotatedSecret(ctx context.Context, name string, token string, apiService *akeyless_api.V2ApiService) (any, error) {
	resp, _, err := apiService.GetRotatedSecretValue(ctx).Body(akeyless_api.GetRotatedSecretValue{
		Names: name,
		Token: akeyless_api.PtrString(token),
	}).Execute()
	if err != nil {
		return nil, err
	}
	if val, ok := resp["value"]; ok {
		return val, nil
	}
	return nil, getSecretNotFoundError(name)
}

func getSSHCertificate(ctx context.Context, name, certUserName, publicKeyData, token string, apiService *akeyless_api.V2ApiService) (string, error) {
	resp, _, err := apiService.GetSSHCertificate(ctx).Body(akeyless_api.GetSSHCertificate{
		CertIssuerName: name,
		CertUsername:   certUserName,
		PublicKeyData:  akeyless_api.PtrString(publicKeyData),
		Token:          akeyless_api.PtrString(token),
	}).Execute()
	if err != nil {
		return "", err
	}
	if resp.Data != nil {
		return *resp.Data, nil
	}
	return "", getSecretNotFoundError(name)
}

func getPKICertificate(ctx context.Context, name, csrData, publicKeyData, token string, apiService *akeyless_api.V2ApiService) (string, error) {
	resp, _, err := apiService.GetPKICertificate(ctx).Body(akeyless_api.GetPKICertificate{
		CertIssuerName: name,
		CsrDataBase64:  akeyless_api.PtrString(csrData),
		KeyDataBase64:  akeyless_api.PtrString(publicKeyData),
		Token:          akeyless_api.PtrString(token),
	}).Execute()
	if err != nil {
		return "", err
	}
	if resp.Data != nil {
		return *resp.Data, nil
	}
	return "", getSecretNotFoundError(name)
}

func getItemType(ctx context.Context, name, token string, apiService *akeyless_api.V2ApiService) (string, error) {
	describeItemOut, _, err := apiService.DescribeItem(ctx).Body(akeyless_api.DescribeItem{
		Name:  name,
		Token: akeyless_api.PtrString(token),
	}).Execute()

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

	case K8S:
		authParams.SetGatewayUrl(server.AkeylessApiUrl)
		authParams.SetK8sServiceAccountToken(server.K8SServiceAccountToken)
		authParams.SetK8sAuthConfigName(server.K8SAuthConfigName)

	case JWT:
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
