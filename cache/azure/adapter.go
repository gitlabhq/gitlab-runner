package azure

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type signedURLGenerator func(name string, options *signedURLOptions) (*url.URL, error)
type blobTokenGenerator func(name string, options *signedURLOptions) (string, error)

type azureAdapter struct {
	timeout    time.Duration
	config     *common.CacheAzureConfig
	objectName string

	generateSignedURL   signedURLGenerator
	blobTokenGenerator  blobTokenGenerator
	credentialsResolver credentialsResolver
}

func (a *azureAdapter) GetDownloadURL() *url.URL {
	return a.presignURL(http.MethodGet)
}

func (a *azureAdapter) GetUploadURL() *url.URL {
	return a.presignURL(http.MethodPut)
}

func (a *azureAdapter) GetUploadHeaders() http.Header {
	httpHeaders := http.Header{}
	httpHeaders.Set("Content-Type", "application/octet-stream")
	httpHeaders.Set("x-ms-blob-type", "BlockBlob")

	return httpHeaders
}

func (a *azureAdapter) GetGoCloudURL() *url.URL {
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

func (a *azureAdapter) GetUploadEnv() map[string]string {
	token := a.generateWriteToken()
	if token == "" {
		return map[string]string{}
	}

	return map[string]string{
		"AZURE_STORAGE_ACCOUNT":   a.config.AccountName,
		"AZURE_STORAGE_SAS_TOKEN": token,
		"AZURE_STORAGE_DOMAIN":    a.config.StorageDomain,
	}
}

func (a *azureAdapter) presignURL(method string) *url.URL {
	credentials := a.getCredentials()
	if credentials == nil {
		return nil
	}

	u, err := a.generateSignedURL(a.objectName, &signedURLOptions{
		ContainerName: a.config.ContainerName,
		StorageDomain: a.config.StorageDomain,
		Credentials:   credentials,
		Method:        method,
		Timeout:       a.timeout,
	})
	if err != nil {
		logrus.WithError(err).Errorf("error generating Azure pre-signed URL")
		return nil
	}

	return u
}

func (a *azureAdapter) generateWriteToken() string {
	credentials := a.getCredentials()
	if credentials == nil {
		return ""
	}

	t, err := a.blobTokenGenerator(a.objectName, &signedURLOptions{
		ContainerName: a.config.ContainerName,
		StorageDomain: a.config.StorageDomain,
		Credentials:   credentials,
		Method:        http.MethodPut,
		Timeout:       a.timeout,
	})
	if err != nil {
		logrus.WithError(err).Errorf("error generating Azure SAS token")
		return ""
	}

	return t
}

func (a *azureAdapter) getCredentials() *common.CacheAzureCredentials {
	if a.config.ContainerName == "" {
		logrus.Errorf("ContainerName can't be empty")
		return nil
	}

	err := a.credentialsResolver.Resolve()
	if err != nil {
		logrus.WithError(err).Errorf("error resolving Azure credentials")
		return nil
	}

	return a.credentialsResolver.Credentials()
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
