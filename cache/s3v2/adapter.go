package s3v2

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type s3Adapter struct {
	timeout    time.Duration
	config     *common.CacheS3Config
	objectName string
	client     s3Presigner
}

func (a *s3Adapter) GetDownloadURL(ctx context.Context) cache.PresignedURL {
	presignedURL, err := a.presignURL(ctx, http.MethodGet)
	if err != nil {
		logrus.WithError(err).Error("error while generating S3 pre-signed URL")
		return cache.PresignedURL{}
	}

	return presignedURL
}

func (a *s3Adapter) GetUploadURL(ctx context.Context) cache.PresignedURL {
	presignedURL, err := a.presignURL(ctx, http.MethodPut)
	if err != nil {
		logrus.WithError(err).Error("error while generating S3 pre-signed URL")
		return cache.PresignedURL{}
	}

	return presignedURL
}

func (a *s3Adapter) GetUploadHeaders() http.Header {
	return nil
}

func (a *s3Adapter) GetGoCloudURL(_ context.Context) *url.URL {
	return nil
}

func (a *s3Adapter) GetUploadEnv(_ context.Context) map[string]string {
	return nil
}

func (a *s3Adapter) presignURL(ctx context.Context, method string) (cache.PresignedURL, error) {
	if a.config.BucketName == "" {
		return cache.PresignedURL{}, fmt.Errorf("config BucketName cannot be empty")
	}

	if a.objectName == "" {
		return cache.PresignedURL{}, fmt.Errorf("object name cannot be empty")
	}

	return a.client.PresignURL(ctx, method, a.config.BucketName, a.objectName, a.timeout)
}

func New(config *common.CacheConfig, timeout time.Duration, objectName string) (cache.Adapter, error) {
	s3Config := config.S3
	if s3Config == nil {
		return nil, fmt.Errorf("missing S3 configuration")
	}

	client, err := newS3Client(s3Config)
	if err != nil {
		return nil, fmt.Errorf("error while creating S3 cache storage client: %w", err)
	}

	a := &s3Adapter{
		config:     s3Config,
		timeout:    timeout,
		objectName: objectName,
		client:     client,
	}

	return a, nil
}

func init() {
	err := cache.Factories().Register("s3v2", New)
	if err != nil {
		panic(err)
	}
}
