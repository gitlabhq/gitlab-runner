package gcs

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type signedURLGenerator func(bucket string, name string, opts *storage.SignedURLOptions) (string, error)

type gcsAdapter struct {
	timeout    time.Duration
	config     *common.CacheGCSConfig
	objectName string

	generateSignedURL   signedURLGenerator
	credentialsResolver credentialsResolver
}

func (a *gcsAdapter) GetDownloadURL() *url.URL {
	return a.presignURL(http.MethodGet, "")
}

func (a *gcsAdapter) GetUploadURL() *url.URL {
	return a.presignURL(http.MethodPut, "application/octet-stream")
}

func (a *gcsAdapter) GetUploadHeaders() http.Header {
	return nil
}

func (a *gcsAdapter) GetGoCloudURL() *url.URL {
	return nil
}

func (a *gcsAdapter) GetUploadEnv() map[string]string {
	return nil
}

func (a *gcsAdapter) presignURL(method string, contentType string) *url.URL {
	err := a.credentialsResolver.Resolve()
	if err != nil {
		logrus.Errorf("error while resolving GCS credentials: %v", err)
		return nil
	}

	credentials := a.credentialsResolver.Credentials()

	var privateKey []byte
	if credentials.PrivateKey != "" {
		privateKey = []byte(credentials.PrivateKey)
	}

	if a.config.BucketName == "" {
		logrus.Error("BucketName can't be empty")
		return nil
	}

	rawURL, err := a.generateSignedURL(a.config.BucketName, a.objectName, &storage.SignedURLOptions{
		GoogleAccessID: credentials.AccessID,
		PrivateKey:     privateKey,
		Method:         method,
		Expires:        time.Now().Add(a.timeout),
		ContentType:    contentType,
	})
	if err != nil {
		logrus.Errorf("error while generating GCS pre-signed URL: %v", err)
		return nil
	}

	URL, err := url.Parse(rawURL)
	if err != nil {
		logrus.Errorf("error while parsing generated URL: %v", err)
		return nil
	}

	return URL
}

func New(config *common.CacheConfig, timeout time.Duration, objectName string) (cache.Adapter, error) {
	gcs := config.GCS
	if gcs == nil {
		return nil, fmt.Errorf("missing GCS configuration")
	}

	cr, err := credentialsResolverInitializer(gcs)
	if err != nil {
		return nil, fmt.Errorf("error while initializing GCS credentials resolver: %w", err)
	}

	a := &gcsAdapter{
		config:              gcs,
		timeout:             timeout,
		objectName:          objectName,
		generateSignedURL:   storage.SignedURL,
		credentialsResolver: cr,
	}

	return a, nil
}

func init() {
	err := cache.Factories().Register("gcs", New)
	if err != nil {
		panic(err)
	}
}
