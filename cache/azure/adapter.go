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
	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
)

type signedURLGenerator func(ctx context.Context, name string, options *signedURLOptions) (*url.URL, error)
type blobTokenGenerator func(ctx context.Context, name string, options *signedURLOptions) (string, error)

type azureAdapter struct {
	timeout    time.Duration
	config     *cacheconfig.CacheAzureConfig
	objectName string

	generateSignedURL   signedURLGenerator
	blobTokenGenerator  blobTokenGenerator
	credentialsResolver credentialsResolver
}

// GetDownloadURL returns a blank value because we use GoCloud to handle the download.
func (a *azureAdapter) GetDownloadURL(ctx context.Context) cache.PresignedURL {
	return cache.PresignedURL{}
}

// GetUploadURL returns a blank value because uploading via a pre-signed URL is
// limited to 5 MB (https://learn.microsoft.com/en-us/rest/api/storageservices/put-blob-from-url?tabs=microsoft-entra-id).
// We depend on GoCloud to handle the upload.
func (a *azureAdapter) GetUploadURL(ctx context.Context) cache.PresignedURL {
	return cache.PresignedURL{}
}

// WithMetadata for Azure is a no-op. We use GoCloud and metadata is directly managed at upload time in the
// cache-archiver.
func (a *azureAdapter) WithMetadata(metadata map[string]string) {}

func (a *azureAdapter) GetGoCloudURL(ctx context.Context, upload bool) (cache.GoCloudURL, error) {
	goCloudURL := cache.GoCloudURL{}

	if a.config.ContainerName == "" {
		logrus.Error("ContainerName can't be empty")
		return goCloudURL, fmt.Errorf("ContainerName can't be empty")
	}

	// Go Cloud omits the object name from the URL. Since object storage
	// providers use the URL host for the bucket name, we attach the
	// object name to avoid having to pass another parameter.
	raw := fmt.Sprintf("azblob://%s/%s", a.config.ContainerName, a.objectName)

	u, err := url.Parse(raw)
	if err != nil {
		logrus.WithError(err).WithField("url", raw).Errorf("error parsing blob URL")
		return goCloudURL, fmt.Errorf("error parsing blob URL: %q: %w", raw, err)
	}

	env, err := a.getEnv(ctx, upload)
	if err != nil {
		logrus.WithError(err).Errorf("error retrieving upload headers for GoCloud URL")
		return goCloudURL, err
	}

	goCloudURL.URL = u
	goCloudURL.Environment = env
	return goCloudURL, nil
}

func (a *azureAdapter) getEnv(ctx context.Context, upload bool) (map[string]string, error) {
	env := map[string]string{
		"AZURE_STORAGE_ACCOUNT": a.config.AccountName,
		"AZURE_STORAGE_DOMAIN":  a.config.StorageDomain,
	}

	token, err := a.generateSASToken(ctx, upload)
	// Return what we do have if the token is missing so the user
	// sees the right error message instead of "options.AccountName is required".
	if token != "" {
		env["AZURE_STORAGE_SAS_TOKEN"] = token
	}

	return env, err
}

func (a *azureAdapter) generateSASToken(ctx context.Context, upload bool) (string, error) {
	method := http.MethodGet
	if upload {
		method = http.MethodPut
	}

	signer := a.getSigner()
	if signer == nil {
		return "", nil
	}

	t, err := a.blobTokenGenerator(ctx, a.objectName, &signedURLOptions{
		ContainerName: a.config.ContainerName,
		Signer:        signer,
		Method:        method,
		Timeout:       a.timeout,
	})
	if err != nil {
		logrus.WithError(err).Errorf("error generating Azure SAS token")
		return t, err
	}

	return t, nil
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

func New(config *cacheconfig.Config, timeout time.Duration, objectName string) (cache.Adapter, error) {
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
