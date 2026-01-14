package service

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type AzureKeyVault interface {
	GetSecret(name string, version string) (interface{}, error)
}

type defaultAzureKeyVault struct {
	client *azsecrets.Client
}

func NewAzureKeyVault(server common.AzureKeyVaultServer) (AzureKeyVault, error) {
	v := new(defaultAzureKeyVault)

	getAssertion := func(c context.Context) (string, error) {
		return server.JWT, nil
	}

	cred, err := azidentity.NewClientAssertionCredential(
		server.TenantID,
		server.ClientID,
		getAssertion,
		&azidentity.ClientAssertionCredentialOptions{
			ClientOptions: azcore.ClientOptions{},
		})

	if err != nil {
		return nil, fmt.Errorf("getting credential failed: %w", err)
	}

	vaultURL := server.URL
	client, err := azsecrets.NewClient(vaultURL, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("initializing azure key Vault service: %w", err)
	}

	v.client = client
	return v, err
}

func (v *defaultAzureKeyVault) GetSecret(name string, version string) (interface{}, error) {
	resp, err := v.client.GetSecret(context.Background(), name, version, nil)
	if err != nil {
		return nil, fmt.Errorf("getting secret failed: %w", err)
	}

	if resp.Value == nil {
		return "", common.ErrSecretNotFound
	}

	return *resp.Value, err
}
