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
	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
)

type s3Adapter struct {
	timeout    time.Duration
	config     *cacheconfig.CacheS3Config
	objectName string
	client     s3Presigner
	metadata   map[string]string
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

	if len(a.metadata) > 0 {
		if presignedURL.Headers == nil {
			presignedURL.Headers = http.Header{}
		}
		for k, v := range a.metadata {
			presignedURL.Headers.Set("x-amz-meta-"+k, v)
		}
	}

	return presignedURL
}

func (a *s3Adapter) WithMetadata(metadata map[string]string) {
	a.metadata = metadata
}

func (a *s3Adapter) getARNForGoCloud(upload bool) string {
	if a.config.RoleARN != "" {
		return a.config.RoleARN
	}

	if upload && a.config.UploadRoleARN != "" {
		return a.config.UploadRoleARN
	}

	return ""
}

func (a *s3Adapter) GetGoCloudURL(ctx context.Context, upload bool) (cache.GoCloudURL, error) {
	goCloudURL := cache.GoCloudURL{}

	roleARN := a.getARNForGoCloud(upload)
	if roleARN == "" {
		return goCloudURL, nil
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
	// We don't need to set the endpoint if the global S3 endpoint is used.
	// If we did, this may result in failures since AWS requires regional
	// endpoints to be used.
	if endpoint != "" && endpoint != DEFAULT_AWS_S3_ENDPOINT {
		q.Set("endpoint", a.config.GetEndpoint())

		if a.config.PathStyleEnabled() {
			q.Set("hostname_immutable", "true")
		}
	}
	if a.config.PathStyleEnabled() {
		q.Set("use_path_style", "true")
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
	goCloudURL.URL = &u

	credentials, err := a.client.FetchCredentialsForRole(
		ctx,
		roleARN,
		a.config.BucketName,
		a.objectName,
		upload,
		a.timeout)
	if err != nil {
		return goCloudURL, err
	}

	goCloudURL.Environment = credentials

	return goCloudURL, nil
}

func (a *s3Adapter) presignURL(ctx context.Context, method string) (cache.PresignedURL, error) {
	if a.config.BucketName == "" {
		return cache.PresignedURL{}, fmt.Errorf("config BucketName cannot be empty")
	}

	if a.objectName == "" {
		return cache.PresignedURL{}, fmt.Errorf("object name cannot be empty")
	}

	return a.client.PresignURL(ctx, method, a.config.BucketName, a.objectName, a.metadata, a.timeout)
}

func New(config *cacheconfig.Config, timeout time.Duration, objectName string) (cache.Adapter, error) {
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
