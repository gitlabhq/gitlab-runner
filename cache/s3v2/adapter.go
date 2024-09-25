package s3v2

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
	if a.config.UploadRoleARN == "" {
		return nil
	}

	u := url.URL{
		Scheme: "s3",
		Host:   a.config.BucketName,
		Path:   a.objectName,
	}

	q := u.Query()
	// These are GoCloud AWS SDK v2 query parameters:
	// https://github.com/google/go-cloud/blob/e5b1bc66f5c42c0a4bb43d179cefdab454559325/blob/s3blob/s3blob.go#L133-L136
	// https://github.com/google/go-cloud/blob/e5b1bc66f5c42c0a4bb43d179cefdab454559325/aws/aws.go#L194-L199
	q.Set("awssdk", "v2")

	if a.config.BucketLocation != "" {
		q.Set("region", a.config.BucketLocation)
	}
	endpoint := a.config.GetEndpoint()
	if endpoint != "" {
		q.Set("endpoint", a.config.GetEndpoint())
	}
	if a.config.PathStyleEnabled() {
		q.Set("hostname_immutable", "true")
	}
	if a.config.DualStackEnabled() {
		q.Set("dualstack", "true")
	}
	if a.config.Accelerate {
		q.Set("accelerate", "true")
	}

	ssetype := a.client.ServerSideEncryptionType()
	if ssetype != "" {
		q.Set("ssetype", ssetype)
	}
	if a.config.ServerSideEncryptionKeyID != "" {
		q.Set("kmskeyid", a.config.ServerSideEncryptionKeyID)
	}

	u.RawQuery = q.Encode()

	return &u
}

func (a *s3Adapter) GetUploadEnv(ctx context.Context) (map[string]string, error) {
	if a.config.UploadRoleARN == "" {
		return nil, nil
	}

	credentials, err := a.client.FetchCredentialsForRole(
		ctx,
		a.config.UploadRoleARN,
		a.config.BucketName,
		a.objectName,
		a.timeout)
	if err != nil {
		return nil, err
	}

	return credentials, nil
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
		objectName: strings.TrimLeft(objectName, "/"),
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
