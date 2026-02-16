package azure

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/service"
	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
)

const DefaultAzureServer = "blob.core.windows.net"

type sasSigner interface {
	ServiceURL() string
	Prepare(ctx context.Context, o *signedURLOptions) error
	Sign(values sas.BlobSignatureValues) (sas.QueryParameters, error)
}

type accountKeySigner struct {
	blobServiceURL string
	credential     *service.SharedKeyCredential
}

type userDelegationKeySigner struct {
	blobServiceURL  string
	credTransporter policy.Transporter
	transport       *http.Transport
	userCredential  *service.UserDelegationCredential
	credential      *azidentity.DefaultAzureCredential
}

type userDelegationKeyOption func(*userDelegationKeySigner)

type signedURLOptions struct {
	ContainerName string
	Signer        sasSigner
	Method        string
	Timeout       time.Duration
}

// withBlobServiceEndpoint allows the caller to override the default service
// URL. This should only be used in testing.
func withBlobServiceEndpoint(endpoint string) userDelegationKeyOption {
	return func(s *userDelegationKeySigner) {
		s.blobServiceURL = endpoint
	}
}

// withBlobServiceTransports allows the caller to override the underlying
// HTTP transport for the service URL. This should only be used in testing.
func withBlobServiceTransport(transport *http.Transport) userDelegationKeyOption {
	return func(s *userDelegationKeySigner) {
		s.transport = transport
	}
}

func withDefaultCredentialTransporter(transporter policy.Transporter) userDelegationKeyOption {
	return func(s *userDelegationKeySigner) {
		s.credTransporter = transporter
	}
}

// transportAdapter wraps http.Transport to implement service.Transporter
type transportAdapter struct {
	transport *http.Transport
}

func (t *transportAdapter) Do(req *http.Request) (*http.Response, error) {
	return t.transport.RoundTrip(req)
}

func presignedURL(ctx context.Context, name string, o *signedURLOptions) (*url.URL, error) {
	sasQueryParams, err := getSASQueryParameters(ctx, name, o)
	if err != nil {
		return nil, err
	}

	endpoint := o.Signer.ServiceURL()
	parts, err := sas.ParseURL(endpoint)
	if err != nil {
		return nil, err
	}

	parts.ContainerName = o.ContainerName
	parts.BlobName = name
	parts.SAS = sasQueryParams

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

func getBlobServiceURL(config *cacheconfig.CacheAzureConfig) string {
	domain := DefaultAzureServer
	if config.StorageDomain != "" {
		domain = config.StorageDomain
	}
	return fmt.Sprintf("https://%s.%s", config.CacheAzureCredentials.AccountName, domain)
}

func newAccountKeySigner(config *cacheconfig.CacheAzureConfig) (sasSigner, error) {
	credentials := config.CacheAzureCredentials
	if credentials.AccountName == "" {
		return nil, fmt.Errorf("missing Azure storage account name")
	}
	if credentials.AccountKey == "" {
		return nil, fmt.Errorf("missing Azure storage account key")
	}
	if config.ContainerName == "" {
		return nil, fmt.Errorf("ContainerName can't be empty")
	}

	blobServiceURL := getBlobServiceURL(config)
	credential, err := azblob.NewSharedKeyCredential(credentials.AccountName, credentials.AccountKey)
	if err != nil {
		return nil, fmt.Errorf("creating Azure signature: %w", err)
	}

	return &accountKeySigner{blobServiceURL: blobServiceURL, credential: credential}, nil
}

func newUserDelegationKeySigner(config *cacheconfig.CacheAzureConfig, options ...userDelegationKeyOption) (sasSigner, error) {
	if config.AccountName == "" {
		return nil, fmt.Errorf("no Azure storage account name provided")
	}

	blobServiceURL := getBlobServiceURL(config)
	signer := &userDelegationKeySigner{blobServiceURL: blobServiceURL}

	for _, opt := range options {
		opt(signer)
	}

	opts := &azidentity.DefaultAzureCredentialOptions{}
	if signer.credTransporter != nil {
		opts.ClientOptions = policy.ClientOptions{Transport: signer.credTransporter}
	}

	credential, err := azidentity.NewDefaultAzureCredential(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure identity credentials: %w", err)
	}

	signer.credential = credential

	return signer, nil
}

func getSASQueryParameters(ctx context.Context, name string, o *signedURLOptions) (sas.QueryParameters, error) {
	serviceSASValues := generateBlobSignatureValues(name, o)

	err := o.Signer.Prepare(ctx, o)
	if err != nil {
		return sas.QueryParameters{}, err
	}

	return o.Signer.Sign(serviceSASValues)
}

func generateBlobSignatureValues(name string, o *signedURLOptions) sas.BlobSignatureValues {
	permissions := sas.BlobPermissions{Read: true}
	if o.Method == http.MethodPut {
		permissions = sas.BlobPermissions{Write: true}
	}

	// Set the desired SAS signature values.
	// See https://docs.microsoft.com/en-us/rest/api/storageservices/create-service-sas
	return sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPS, // Users MUST use HTTPS (not HTTP)
		StartTime:     time.Now().Add(-1 * time.Hour).UTC(),
		ExpiryTime:    time.Now().Add(o.Timeout).UTC(),
		Permissions:   permissions.String(),
		ContainerName: o.ContainerName,
		BlobName:      name,
	}
}

func (s *accountKeySigner) ServiceURL() string {
	return s.blobServiceURL
}

func (s *accountKeySigner) Prepare(ctx context.Context, o *signedURLOptions) error {
	return nil
}

func (s *accountKeySigner) Sign(values sas.BlobSignatureValues) (sas.QueryParameters, error) {
	empty := sas.QueryParameters{}
	sas, err := values.SignWithSharedKey(s.credential)
	if err != nil {
		return empty, fmt.Errorf("creating Azure SAS: %w", err)
	}

	return sas, nil
}

func (s *userDelegationKeySigner) ServiceURL() string {
	return s.blobServiceURL
}

func (s *userDelegationKeySigner) Prepare(ctx context.Context, o *signedURLOptions) error {
	userDelegationKey, err := s.retrieveUserCredentials(ctx, o)
	if err != nil {
		return fmt.Errorf("failed to get User Delegation Key: %w", err)
	}

	s.userCredential = userDelegationKey

	return nil
}

func (s *userDelegationKeySigner) Sign(values sas.BlobSignatureValues) (sas.QueryParameters, error) {
	empty := sas.QueryParameters{}
	sas, err := values.SignWithUserDelegation(s.userCredential)
	if err != nil {
		return empty, fmt.Errorf("creating Azure SAS: %w", err)
	}

	return sas, nil
}

func (s *userDelegationKeySigner) retrieveUserCredentials(ctx context.Context, o *signedURLOptions) (*service.UserDelegationCredential, error) {
	start := time.Now().UTC()
	expiry := start.Add(o.Timeout)
	info := service.KeyInfo{
		Start:  to.Ptr(start.UTC().Format(sas.TimeFormat)),
		Expiry: to.Ptr(expiry.UTC().Format(sas.TimeFormat)),
	}

	clientOptions := &service.ClientOptions{}
	if s.transport != nil {
		clientOptions.Transport = &transportAdapter{transport: s.transport}
	}

	blobServiceClient, err := service.NewClient(s.blobServiceURL, s.credential, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure Blob Service Client: %w", err)
	}

	return blobServiceClient.GetUserDelegationCredential(ctx, info, nil)
}
