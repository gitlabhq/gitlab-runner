package gcsv2

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus"
	"google.golang.org/api/option"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type gcsAdapter struct {
	timeout                time.Duration
	config                 *common.CacheGCSConfig
	objectName             string
	maxUploadedArchiveSize int64
}

func (a *gcsAdapter) GetDownloadURL(ctx context.Context) cache.PresignedURL {
	u, err := a.presignURL(ctx, http.MethodGet, "")
	if err != nil {
		logrus.Error(err)
	}

	return cache.PresignedURL{URL: u}
}

func (a *gcsAdapter) GetUploadURL(ctx context.Context) cache.PresignedURL {
	u, err := a.presignURL(ctx, http.MethodPut, "application/octet-stream")
	if err != nil {
		logrus.Error(err)
	}

	return cache.PresignedURL{URL: u, Headers: a.GetUploadHeaders()}
}

func (a *gcsAdapter) GetUploadHeaders() http.Header {
	if a.maxUploadedArchiveSize > 0 {
		return http.Header{"X-Goog-Content-Length-Range": []string{fmt.Sprintf("0,%d", a.maxUploadedArchiveSize)}}
	}
	return nil
}

func (a *gcsAdapter) GetGoCloudURL(_ context.Context, _ bool) (cache.GoCloudURL, error) {
	return cache.GoCloudURL{}, nil
}

func (a *gcsAdapter) presignURL(ctx context.Context, method string, contentType string) (*url.URL, error) {
	if a.config.BucketName == "" {
		return nil, fmt.Errorf("config BucketName cannot be empty")
	}

	var options []option.ClientOption
	switch {
	case a.config.CredentialsFile != "":
		options = append(options, option.WithCredentialsFile(a.config.CredentialsFile))
	case a.config.AccessID != "" || a.config.PrivateKey != "":
		// if providing accessID / privateKey for signing, then we don't need the
		// storage client to authenticate
		options = append(options, option.WithoutAuthentication())
	}

	client, err := storage.NewClient(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("creating storage client: %w", err)
	}
	defer client.Close()

	// if accessID/private key is not provided, then the storage client's
	// authentication will be used.
	suo := &storage.SignedURLOptions{
		GoogleAccessID: a.config.AccessID,
		Method:         method,
		Expires:        time.Now().Add(a.timeout),
		ContentType:    contentType,
	}

	if a.config.PrivateKey != "" {
		suo.PrivateKey = []byte(a.config.PrivateKey)
	}

	if method == http.MethodPut {
		suo.Headers = []string{}
		for key, values := range a.GetUploadHeaders() {
			suo.Headers = append(suo.Headers, fmt.Sprintf("%s:%s", key, strings.Join(values, ";")))
		}
	}

	rawURL, err := client.Bucket(a.config.BucketName).SignedURL(a.objectName, suo)
	if err != nil {
		return nil, fmt.Errorf("generating signed URL: %w", err)
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parsing signed URL: %w", err)
	}

	return u, nil
}

func New(config *common.CacheConfig, timeout time.Duration, objectName string) (cache.Adapter, error) {
	gcs := config.GCS
	if gcs == nil {
		return nil, fmt.Errorf("missing GCS configuration")
	}

	return &gcsAdapter{
		config:                 gcs,
		timeout:                timeout,
		objectName:             objectName,
		maxUploadedArchiveSize: config.MaxUploadedArchiveSize,
	}, nil
}

func init() {
	err := cache.Factories().Register("gcsv2", New)
	if err != nil {
		panic(err)
	}
}
