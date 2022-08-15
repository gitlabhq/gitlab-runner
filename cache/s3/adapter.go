package s3

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7/pkg/encrypt"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type s3Adapter struct {
	timeout    time.Duration
	config     *common.CacheS3Config
	objectName string
	client     minioClient
}

func (a *s3Adapter) GetDownloadURL() *url.URL {
	URL, err := a.client.PresignHeader(
		context.Background(), http.MethodGet, a.config.BucketName,
		a.objectName, a.timeout, nil, nil,
	)
	if err != nil {
		logrus.WithError(err).Error("error while generating S3 pre-signed URL")

		return nil
	}

	return URL
}

func (a *s3Adapter) GetUploadURL() *url.URL {
	URL, err := a.client.PresignHeader(
		context.Background(), http.MethodPut, a.config.BucketName,
		a.objectName, a.timeout, nil, a.GetUploadHeaders(),
	)
	if err != nil {
		logrus.WithError(err).Error("error while generating S3 pre-signed URL")

		return nil
	}

	return URL
}

func (a *s3Adapter) GetUploadHeaders() http.Header {
	var ss encrypt.ServerSide

	var err error
	switch encrypt.Type(strings.ToUpper(a.config.ServerSideEncryption)) {
	case encrypt.S3:
		ss = encrypt.NewSSE()

	case encrypt.KMS:
		ss, err = encrypt.NewSSEKMS(a.config.ServerSideEncryptionKeyID, nil)
		if err != nil {
			err = fmt.Errorf("initializing server-side-encryption key id: %w", err)
		}

	default:
		return nil
	}

	if err != nil {
		logrus.WithError(err).Error("error configuring S3 SSE configuration")
		return nil
	}

	headers := http.Header{}
	ss.Marshal(headers)

	return headers
}

func (a *s3Adapter) GetGoCloudURL() *url.URL {
	return nil
}

func (a *s3Adapter) GetUploadEnv() map[string]string {
	return nil
}

func New(config *common.CacheConfig, timeout time.Duration, objectName string) (cache.Adapter, error) {
	s3 := config.S3
	if s3 == nil {
		return nil, fmt.Errorf("missing S3 configuration")
	}

	client, err := newMinioClient(s3)
	if err != nil {
		return nil, fmt.Errorf("error while creating S3 cache storage client: %w", err)
	}

	a := &s3Adapter{
		config:     s3,
		timeout:    timeout,
		objectName: objectName,
		client:     client,
	}

	return a, nil
}

func init() {
	err := cache.Factories().Register("s3", New)
	if err != nil {
		panic(err)
	}
}
