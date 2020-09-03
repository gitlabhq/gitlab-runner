package azure

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Azure/azure-storage-blob-go/azblob"

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

func PresignedURL(name string, o *signedURLOptions) (*url.URL, error) {
	if o.Credentials.AccountName == "" {
		return nil, fmt.Errorf("missing Azure storage account name")
	}
	if o.Credentials.AccountKey == "" {
		return nil, fmt.Errorf("missing Azure storage account key")
	}

	credential, err := azblob.NewSharedKeyCredential(o.Credentials.AccountName, o.Credentials.AccountKey)
	if err != nil {
		return nil, fmt.Errorf("creating Azure signature: %w", err)
	}

	permissions := azblob.AccountSASPermissions{Read: true}
	if o.Method == http.MethodPut {
		permissions = azblob.AccountSASPermissions{Write: true}
	}

	// Set the desired SAS signature values and sign them with the
	// shared key credentials to get the SAS query parameters.
	// See https://docs.microsoft.com/en-us/rest/api/storageservices/create-account-sas
	sasSignatureValues := azblob.AccountSASSignatureValues{
		Protocol:      azblob.SASProtocolHTTPS, // Users MUST use HTTPS (not HTTP)
		StartTime:     time.Now().Add(-1 * time.Hour).UTC(),
		ExpiryTime:    time.Now().Add(o.Timeout).UTC(),
		Permissions:   permissions.String(),
		Services:      azblob.AccountSASServices{Blob: true}.String(),
		ResourceTypes: azblob.AccountSASResourceTypes{Object: true}.String(),
	}

	sas, err := sasSignatureValues.NewSASQueryParameters(credential)
	if err != nil {
		return nil, fmt.Errorf("creating Azure signature: %w", err)
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

	u := parts.URL()
	return &u, nil
}
