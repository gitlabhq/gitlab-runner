package s3

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const DefaultAWSS3Server = "s3.amazonaws.com"

//go:generate mockery --name=minioClient --inpackage
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

var newMinioClient = func(s3 *common.CacheS3Config) (minioClient, error) {
	serverAddress := s3.ServerAddress

	if serverAddress == "" {
		serverAddress = DefaultAWSS3Server
	}

	switch s3.AuthType() {
	case common.S3AuthTypeIAM:
		return newMinioWithIAM(serverAddress, s3.BucketLocation)
	case common.S3AuthTypeAccessKey:
		return newMinio(serverAddress, &minio.Options{
			Creds:  credentials.NewStaticV4(s3.AccessKey, s3.SecretKey, ""),
			Secure: !s3.Insecure,
			Transport: &bucketLocationTripper{
				bucketLocation: s3.BucketLocation,
			},
		})
	default:
		return nil, errors.New("invalid s3 authentication type")
	}
}
