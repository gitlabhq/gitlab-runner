package azure

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type signedURLGenerator func(ctx context.Context, name string, options *signedURLOptions) (*url.URL, error)
type blobTokenGenerator func(ctx context.Context, name string, options *signedURLOptions) (string, error)

type azureAdapter struct {
	timeout    time.Duration
	config     *common.CacheAzureConfig
	objectName string

	generateSignedURL   signedURLGenerator
	blobTokenGenerator  blobTokenGenerator
	credentialsResolver credentialsResolver
}

// GetDownloadURL returns a pre-signed URL for downloading the cache. In
// the future we should convert the cache extractor to use a GoCloud URL
// to eliminate this code.
func (a *azureAdapter) GetDownloadURL(ctx context.Context) cache.PresignedURL {
	return cache.PresignedURL{
		URL: a.presignURL(ctx, http.MethodGet),
	}
}

// GetUploadURL returns a legacy URL that is no longer used
// because uploading via a pre-signed URL is limited to 5 MB (https://learn.microsoft.com/en-us/rest/api/storageservices/put-blob-from-url?tabs=microsoft-entra-id).
// We depend on GoCloud to handle the upload.
func (a *azureAdapter) GetUploadURL(ctx context.Context) cache.PresignedURL {
	return cache.PresignedURL{
		URL:     a.presignURL(ctx, http.MethodPut),
		Headers: a.GetUploadHeaders(),
	}
}

// GetUploadHeaders returns legacy HTTP headers that are no longer used because
// uploading via a pre-signed URL is limited to 5 MB (https://learn.microsoft.com/en-us/rest/api/storageservices/put-blob-from-url?tabs=microsoft-entra-id).
// We depend on GoCloud to handle the upload.
func (a *azureAdapter) GetUploadHeaders() http.Header {
	httpHeaders := http.Header{}
	httpHeaders.Set(common.ContentType, "application/octet-stream")
	httpHeaders.Set("x-ms-blob-type", "BlockBlob")

	return httpHeaders
}

func (a *azureAdapter) GetGoCloudURL(_ context.Context) *url.URL {
	if a.config.ContainerName == "" {
		logrus.Error("ContainerName can't be empty")
		return nil
	}

	// Go Cloud omits the object name from the URL. Since object storage
	// providers use the URL host for the bucket name, we attach the
	// object name to avoid having to pass another parameter.
	raw := fmt.Sprintf("azblob://%s/%s", a.config.ContainerName, a.objectName)

	u, err := url.Parse(raw)
	if err != nil {
		logrus.WithError(err).WithField("url", raw).Errorf("error parsing blob URL")
		return nil
	}

	return u
}

func (a *azureAdapter) GetUploadEnv(ctx context.Context) map[string]string {
	token := a.generateWriteToken(ctx)

	// Return what we do have if the token is missing so the user
	// sees the right error message instead of "options.AccountName is required".
	env := map[string]string{
		"AZURE_STORAGE_ACCOUNT": a.config.AccountName,
		"AZURE_STORAGE_DOMAIN":  a.config.StorageDomain,
	}
	if token == "" {
		return env
	}

	env["AZURE_STORAGE_SAS_TOKEN"] = token
	return env
}

func (a *azureAdapter) presignURL(ctx context.Context, method string) *url.URL {
	signer := a.getSigner()
	if signer == nil {
		return nil
	}

	u, err := a.generateSignedURL(ctx, a.objectName, &signedURLOptions{
		ContainerName: a.config.ContainerName,
		Signer:        signer,
		Method:        method,
		Timeout:       a.timeout,
	})
	if err != nil {
		logrus.WithError(err).Errorf("error generating Azure pre-signed URL")
		return nil
	}

	return u
}

func (a *azureAdapter) generateWriteToken(ctx context.Context) string {
	signer := a.getSigner()
	if signer == nil {
		return ""
	}

	t, err := a.blobTokenGenerator(ctx, a.objectName, &signedURLOptions{
		ContainerName: a.config.ContainerName,
		Signer:        signer,
		Method:        http.MethodPut,
		Timeout:       a.timeout,
	})
	if err != nil {
		logrus.WithError(err).Errorf("error generating Azure SAS token")
		return ""
	}

	return t
}

func (a *azureAdapter) getSigner() sasSigner {
	err := a.credentialsResolver.Resolve()
	if err != nil {
		logrus.WithError(err).Errorf("error resolving Azure credentials")
		return nil
	}

	signer, err := a.credentialsResolver.Signer()
	if err != nil {
		logrus.WithError(err).Errorf("error creating Azure SAS signer")
		return nil
	}

	return signer
}

func New(config *common.CacheConfig, timeout time.Duration, objectName string) (cache.Adapter, error) {
	azure := config.Azure
	if azure == nil {
		return nil, fmt.Errorf("missing Azure configuration")
	}

	cr, err := credentialsResolverInitializer(azure)
	if err != nil {
		return nil, fmt.Errorf("error while initializing Azure credentials resolver: %w", err)
	}

	a := &azureAdapter{
		config:              azure,
		timeout:             timeout,
		objectName:          strings.TrimLeft(objectName, "/"),
		credentialsResolver: cr,
		generateSignedURL:   presignedURL,
		blobTokenGenerator:  getSASToken,
	}

	return a, nil
}

func init() {
	err := cache.Factories().Register("azure", New)
	if err != nil {
		panic(err)
	}
}
