package s3

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/minio/minio-go/v7/pkg/encrypt"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
)

type s3Adapter struct {
	timeout    time.Duration
	config     *cacheconfig.CacheS3Config
	objectName string
	client     minioClient
	metadata   map[string]string
}

func (a *s3Adapter) GetDownloadURL(ctx context.Context) cache.PresignedURL {
	URL, err := a.client.PresignHeader(
		ctx, http.MethodGet, a.config.BucketName,
		a.objectName, a.timeout, nil, nil,
	)
	if err != nil {
		logrus.WithError(err).Error("error while generating S3 pre-signed URL")
		return cache.PresignedURL{}
	}

	return cache.PresignedURL{URL: URL}
}

func (a *s3Adapter) GetUploadURL(ctx context.Context) cache.PresignedURL {
	headers := a.GetUploadHeaders()

	// Note: PresignHeader means, we need the exact same headers to be used when getting the presigned URL and when
	// actuallt uploading.
	URL, err := a.client.PresignHeader(
		ctx, http.MethodPut, a.config.BucketName,
		a.objectName, a.timeout, nil, headers,
	)
	if err != nil {
		logrus.WithError(err).Error("error while generating S3 pre-signed URL")
		return cache.PresignedURL{}
	}

	return cache.PresignedURL{URL: URL, Headers: headers}
}

func (a *s3Adapter) GetUploadHeaders() http.Header {
	ss, err := func() (encrypt.ServerSide, error) {
		switch encrypt.Type(strings.ToUpper(a.config.ServerSideEncryption)) {
		case encrypt.S3:
			return encrypt.NewSSE(), nil
		case encrypt.KMS:
			ss, err := encrypt.NewSSEKMS(a.config.ServerSideEncryptionKeyID, nil)
			if err != nil {
				return nil, fmt.Errorf("initializing server-side-encryption key id: %w", err)
			}
			return ss, nil
		default:
			return nil, nil
		}
	}()
	if err != nil {
		logrus.WithError(err).Error("error configuring S3 SSE configuration")
		return nil
	}

	headers := http.Header{}

	if ss != nil {
		ss.Marshal(headers)
	}

	// Using e.g. a `x-amz-meta-cacheKey` header shows:
	//	- on the WebUI:
	//		| User defined | x-amz-meta-cachekey | qwe-protected-non_protected |
	//	- on the API:
	//		; aws s3api head-object --bucket $bucket --key $blob | jq .Metadata
	//		{
	//			"cachekey": "qwe-protected-non_protected"
	//		}
	for k, v := range a.metadata {
		headers.Set("x-amz-meta-"+k, v)
	}

	return headers
}

func (a *s3Adapter) GetGoCloudURL(_ context.Context, _ bool) (cache.GoCloudURL, error) {
	return cache.GoCloudURL{}, nil
}

func (a *s3Adapter) WithMetadata(metadata map[string]string) {
	a.metadata = metadata
}

func New(config *cacheconfig.Config, timeout time.Duration, objectName string) (cache.Adapter, error) {
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
