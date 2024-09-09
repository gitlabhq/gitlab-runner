package azure

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const DefaultAzureServer = "blob.core.windows.net"

type signedURLOptions struct {
	ContainerName string
	StorageDomain string
	Credentials   *common.CacheAzureCredentials
	Method        string
	Timeout       time.Duration
}

func presignedURL(ctx context.Context, name string, o *signedURLOptions) (*url.URL, error) {
	sasQueryParams, err := getSASQueryParameters(ctx, name, o)
	if err != nil {
		return nil, err
	}

	domain := DefaultAzureServer
	if o.StorageDomain != "" {
		domain = o.StorageDomain
	}

	parts := sas.URLParts{
		Scheme:        "https",
		Host:          fmt.Sprintf("%s.%s", o.Credentials.AccountName, domain),
		ContainerName: o.ContainerName,
		BlobName:      name,
		SAS:           sasQueryParams,
	}

	u, err := url.Parse(parts.String())
	if err != nil {
		return nil, fmt.Errorf("failed to parse Azure URL '%s': %w", parts.String(), err)
	}
	return u, nil
}

func getSASToken(ctx context.Context, name string, o *signedURLOptions) (string, error) {
	sas, err := getSASQueryParameters(ctx, name, o)
	if err != nil {
		return "", err
	}

	return sas.Encode(), nil
}

func getSASQueryParameters(ctx context.Context, name string, o *signedURLOptions) (sas.QueryParameters, error) {
	if o.Credentials.AccountName == "" {
		return sas.QueryParameters{}, errors.New("missing Azure storage account name")
	}

	if o.Credentials.AccountKey == "" {
		return getSASWithManagedIdentity(ctx, name, o)
	}

	return getSASWithSharedCredentials(name, o)
}

func generateBlobSignatureValues(name string, o *signedURLOptions) sas.BlobSignatureValues {
	permissions := sas.BlobPermissions{Read: true}
	if o.Method == http.MethodPut {
		permissions = sas.BlobPermissions{Write: true}
	}

	// Set the desired SAS signature values.
	// See https://docs.microsoft.com/en-us/rest/api/storageservices/create-service-sas
	return sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPS,
		StartTime:     time.Now().Add(-1 * time.Hour).UTC(),
		ExpiryTime:    time.Now().Add(o.Timeout).UTC(),
		Permissions:   permissions.String(),
		ContainerName: o.ContainerName,
		BlobName:      name,
	}
}

func getSASWithSharedCredentials(name string, o *signedURLOptions) (sas.QueryParameters, error) {
	empty := sas.QueryParameters{}

	credential, err := azblob.NewSharedKeyCredential(o.Credentials.AccountName, o.Credentials.AccountKey)
	if err != nil {
		return empty, fmt.Errorf("creating Azure signature: %w", err)
	}

	serviceSASValues := generateBlobSignatureValues(name, o)
	sas, err := serviceSASValues.SignWithSharedKey(credential)
	if err != nil {
		return empty, fmt.Errorf("creating Azure SAS: %w", err)
	}

	return sas, nil
}

var retrieveUserCredentials = func(ctx context.Context, o *signedURLOptions) (*service.UserDelegationCredential, error) {
	domain := DefaultAzureServer
	if o.StorageDomain != "" {
		domain = o.StorageDomain
	}

	credential, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure identity credentials: %w", err)
	}

	start := time.Now().UTC()
	expiry := start.Add(o.Timeout)
	info := service.KeyInfo{
		Start:  to.Ptr(start.UTC().Format(sas.TimeFormat)),
		Expiry: to.Ptr(expiry.UTC().Format(sas.TimeFormat)),
	}

	blobServiceURL := fmt.Sprintf("https://%s.%s", o.Credentials.AccountName, domain)
	blobServiceClient, err := service.NewClient(blobServiceURL, credential, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Blob Service Client: %w", err)
	}

	return blobServiceClient.GetUserDelegationCredential(ctx, info, nil)
}

func getSASWithManagedIdentity(ctx context.Context, name string, o *signedURLOptions) (sas.QueryParameters, error) {
	empty := sas.QueryParameters{}

	userDelegationKey, err := retrieveUserCredentials(ctx, o)
	if err != nil {
		return empty, fmt.Errorf("failed to get User Delegation Key: %w", err)
	}

	serviceSASValues := generateBlobSignatureValues(name, o)
	sasQueryParams, err := serviceSASValues.SignWithUserDelegation(userDelegationKey)
	if err != nil {
		return empty, fmt.Errorf("creating Azure SAS with User Delegation Key: %w", err)
	}

	return sasQueryParams, nil
}
