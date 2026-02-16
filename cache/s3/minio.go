package s3

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"

	"github.com/minio/minio-go/v7/pkg/credentials"
)

const DefaultAWSS3Server = "s3.amazonaws.com"

var s3AcceleratePattern = regexp.MustCompile(`s3-accelerate.*\.amazonaws\.com$`)

type minioClient interface {
	PresignHeader(
		ctx context.Context,
		method string,
		bucketName string,
		objectName string,
		expires time.Duration,
		reqParams url.Values,
		extraHeaders http.Header,
	) (*url.URL, error)
}

var newMinio = minio.New
var newMinioWithIAM = func(serverAddress, bucketLocation string) (*minio.Client, error) {
	return minio.New(serverAddress, &minio.Options{
		Creds:  credentials.NewIAM(""),
		Secure: true,
		Transport: &bucketLocationTripper{
			bucketLocation: bucketLocation,
		},
	})
}

var newMinioClient = func(s3 *cacheconfig.CacheS3Config) (minioClient, error) {
	serverAddress := s3.ServerAddress

	if serverAddress == "" {
		serverAddress = DefaultAWSS3Server
	}

	var isS3AccelerateEndpoint = s3AcceleratePattern.MatchString(serverAddress)
	var s3AccelerateEndpoint string
	if isS3AccelerateEndpoint {
		s3AccelerateEndpoint = serverAddress
		serverAddress = strings.Replace(serverAddress, "s3-accelerate", "s3", 1)
	}

	var client *minio.Client
	var err error
	switch s3.AuthType() {
	case cacheconfig.S3AuthTypeIAM:
		client, err = newMinioWithIAM(serverAddress, s3.BucketLocation)
	case cacheconfig.S3AuthTypeAccessKey:
		client, err = newMinio(serverAddress, &minio.Options{
			Creds:  credentials.NewStaticV4(s3.AccessKey, s3.SecretKey, s3.SessionToken),
			Secure: !s3.Insecure,
			Transport: &bucketLocationTripper{
				bucketLocation: s3.BucketLocation,
			},
		})
	default:
		return nil, errors.New("invalid s3 authentication type")
	}

	if err == nil && isS3AccelerateEndpoint {
		client.SetS3TransferAccelerate(s3AccelerateEndpoint)
	}

	return client, err
}
