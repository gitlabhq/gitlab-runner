package azure

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type signedURLGenerator func(name string, options *signedURLOptions) (*url.URL, error)

type azureAdapter struct {
	timeout    time.Duration
	config     *common.CacheAzureConfig
	objectName string

	generateSignedURL   signedURLGenerator
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

func (a *azureAdapter) presignURL(method string) *url.URL {
	if a.config.ContainerName == "" {
		logrus.Error("ContainerName can't be empty")
		return nil
	}

	err := a.credentialsResolver.Resolve()
	if err != nil {
		logrus.Errorf("error while resolving Azure credentials: %v", err)
		return nil
	}

	credentials := a.credentialsResolver.Credentials()

	u, err := a.generateSignedURL(a.objectName, &signedURLOptions{
		ContainerName: a.config.ContainerName,
		StorageDomain: a.config.StorageDomain,
		Credentials:   credentials,
		Method:        method,
		Timeout:       a.timeout,
	})
	if err != nil {
		logrus.Errorf("error generating Azure pre-signed URL: %v", err)
		return nil
	}

	return u
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
		objectName:          objectName,
		credentialsResolver: cr,
		generateSignedURL:   PresignedURL,
	}

	return a, nil
}

func init() {
	err := cache.Factories().Register("azure", New)
	if err != nil {
		panic(err)
	}
}
