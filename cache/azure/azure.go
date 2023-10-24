package azure

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"

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

func presignedURL(name string, o *signedURLOptions) (*url.URL, error) {
	sasQueryParams, err := getSASQueryParameters(name, o)
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

func getSASToken(name string, o *signedURLOptions) (string, error) {
	sas, err := getSASQueryParameters(name, o)
	if err != nil {
		return "", err
	}

	return sas.Encode(), nil
}

func getSASQueryParameters(name string, o *signedURLOptions) (sas.QueryParameters, error) {
	empty := sas.QueryParameters{}

	if o.Credentials.AccountName == "" {
		return empty, errors.New("missing Azure storage account name")
	}
	if o.Credentials.AccountKey == "" {
		return empty, errors.New("missing Azure storage account key")
	}

	credential, err := azblob.NewSharedKeyCredential(o.Credentials.AccountName, o.Credentials.AccountKey)
	if err != nil {
		return empty, fmt.Errorf("creating Azure signature: %w", err)
	}

	permissions := sas.AccountPermissions{Read: true}
	if o.Method == http.MethodPut {
		permissions = sas.AccountPermissions{Write: true}
	}

	// Set the desired SAS signature values and sign them with the
	// shared key credentials to get the SAS query parameters.
	// See https://docs.microsoft.com/en-us/rest/api/storageservices/create-service-sas
	serviceSASValues := sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPS, // Users MUST use HTTPS (not HTTP)
		StartTime:     time.Now().Add(-1 * time.Hour).UTC(),
		ExpiryTime:    time.Now().Add(o.Timeout).UTC(),
		Permissions:   permissions.String(),
		ContainerName: o.ContainerName,
		BlobName:      name,
	}

	sas, err := serviceSASValues.SignWithSharedKey(credential)
	if err != nil {
		return empty, fmt.Errorf("creating Azure SAS: %w", err)
	}

	return sas, nil
}
