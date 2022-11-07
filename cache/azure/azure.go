package azure

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"

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
	sas, err := getSASQueryParameters(name, o)
	if err != nil {
		return nil, err
	}

	domain := DefaultAzureServer
	if o.StorageDomain != "" {
		domain = o.StorageDomain
	}

	parts := azblob.BlobURLParts{
		Scheme:        "https",
		Host:          fmt.Sprintf("%s.%s", o.Credentials.AccountName, domain),
		ContainerName: o.ContainerName,
		BlobName:      name,
		SAS:           sas,
	}

	u, err := url.Parse(parts.URL())
	if err != nil {
		return nil, fmt.Errorf("failed to parse Azure URL '%s': %w", parts.URL(), err)
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

func getSASQueryParameters(name string, o *signedURLOptions) (azblob.SASQueryParameters, error) {
	empty := azblob.SASQueryParameters{}

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

	permissions := azblob.AccountSASPermissions{Read: true}
	if o.Method == http.MethodPut {
		permissions = azblob.AccountSASPermissions{Write: true}
	}

	// Set the desired SAS signature values and sign them with the
	// shared key credentials to get the SAS query parameters.
	// See https://docs.microsoft.com/en-us/rest/api/storageservices/create-service-sas
	serviceSASValues := azblob.BlobSASSignatureValues{
		Protocol:      azblob.SASProtocolHTTPS, // Users MUST use HTTPS (not HTTP)
		StartTime:     time.Now().Add(-1 * time.Hour).UTC(),
		ExpiryTime:    time.Now().Add(o.Timeout).UTC(),
		Permissions:   permissions.String(),
		ContainerName: o.ContainerName,
		BlobName:      name,
	}

	sas, err := serviceSASValues.NewSASQueryParameters(credential)
	if err != nil {
		return empty, fmt.Errorf("creating Azure SAS: %w", err)
	}

	return sas, nil
}
