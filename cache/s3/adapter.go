package s3

import (
	"fmt"
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
	client     minioClient
}

func (a *s3Adapter) GetDownloadURL() *url.URL {
	URL, err := a.client.PresignedGetObject(a.config.BucketName, a.objectName, a.timeout, nil)
	if err != nil {
		logrus.WithError(err).Error("error while generating S3 pre-signed URL")

		return nil
	}

	return URL
}

func (a *s3Adapter) GetUploadURL() *url.URL {
	URL, err := a.client.PresignedPutObject(a.config.BucketName, a.objectName, a.timeout)
	if err != nil {
		logrus.WithError(err).Error("error while generating S3 pre-signed URL")

		return nil
	}

	return URL
}

func New(config *common.CacheConfig, timeout time.Duration, objectName string) (cache.Adapter, error) {
	c := deprecatedConfigHandler(config)

	s3 := c.S3
	if s3 == nil {
		return nil, fmt.Errorf("missing S3 configuration")
	}

	client, err := newMinioClient(s3)
	if err != nil {
		return nil, fmt.Errorf("error while creating S3 cache storage client: %v", err)
	}

	a := &s3Adapter{
		config:     s3,
		timeout:    timeout,
		objectName: objectName,
		client:     client,
	}

	return a, nil
}

// TODO: Remove in 12.0
var deprecatedConfigHandler = func(config *common.CacheConfig) *common.CacheConfig {
	if config.S3 != nil {
		return config
	}

	logrus.Warningln("Runner uses S3 caching with deprecated configuration format. Support for deprecated format will be removed in GitLab Runner 12.0")

	config.S3 = &common.CacheS3Config{
		ServerAddress:  config.GetServerAddress(),
		AccessKey:      config.GetAccessKey(),
		SecretKey:      config.GetSecretKey(),
		BucketName:     config.GetBucketName(),
		BucketLocation: config.GetBucketLocation(),
		Insecure:       config.GetInsecure(),
	}

	return config
}

func init() {
	err := cache.Factories().Register("s3", New)
	if err != nil {
		panic(err)
	}
}
