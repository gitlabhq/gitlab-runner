package gcs

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type signedURLGenerator func(bucket string, name string, opts *storage.SignedURLOptions) (string, error)

type gcsAdapter struct {
	timeout                time.Duration
	config                 *common.CacheGCSConfig
	objectName             string
	maxUploadedArchiveSize int64

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
	if a.maxUploadedArchiveSize > 0 {
		return http.Header{"X-Goog-Content-Length-Range": []string{fmt.Sprintf("0,%d", a.maxUploadedArchiveSize)}}
	}
	return nil
}

func (a *gcsAdapter) GetGoCloudURL() *url.URL {
	return nil
}

func (a *gcsAdapter) GetUploadEnv() map[string]string {
	return nil
}

func (a *gcsAdapter) presignURL(method string, contentType string) *url.URL {
	if a.config.BucketName == "" {
		logrus.Error("BucketName can't be empty")
		return nil
	}

	err := a.credentialsResolver.Resolve()
	if err != nil {
		logrus.Errorf("error while resolving GCS credentials: %v", err)
		return nil
	}

	credentials := a.credentialsResolver.Credentials()

	suo := storage.SignedURLOptions{
		GoogleAccessID: credentials.AccessID,
		Method:         method,
		Expires:        time.Now().Add(a.timeout),
		ContentType:    contentType,
	}

	if method == http.MethodPut {
		suo.Headers = []string{}
		for key, values := range a.GetUploadHeaders() {
			suo.Headers = append(suo.Headers, fmt.Sprintf("%s:%s", key, strings.Join(values, ";")))
		}
	}

	if credentials.PrivateKey != "" {
		suo.PrivateKey = []byte(credentials.PrivateKey)
	} else {
		logrus.Debug("No private key was provided for GCS cache. Attempting to use instance credentials.")
		suo.SignBytes = a.credentialsResolver.SignBytesFunc()
	}

	rawURL, err := a.generateSignedURL(a.config.BucketName, a.objectName, &suo)
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
		config:                 gcs,
		timeout:                timeout,
		objectName:             objectName,
		maxUploadedArchiveSize: config.MaxUploadedArchiveSize,
		generateSignedURL:      storage.SignedURL,
		credentialsResolver:    cr,
	}

	return a, nil
}

func init() {
	err := cache.Factories().Register("gcs", New)
	if err != nil {
		panic(err)
	}
}
