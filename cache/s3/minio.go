package s3

import (
	"net/url"
	"time"

	"github.com/minio/minio-go/v6"
	"github.com/minio/minio-go/v6/pkg/credentials"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const DefaultAWSS3Server = "s3.amazonaws.com"

type minioClient interface {
	PresignedGetObject(
		bucketName string,
		objectName string,
		expires time.Duration,
		reqParams url.Values,
	) (*url.URL, error)
	PresignedPutObject(bucketName string, objectName string, expires time.Duration) (*url.URL, error)
}

var newMinio = minio.New
var newMinioWithCredentials = minio.NewWithCredentials

var newMinioClient = func(s3 *common.CacheS3Config) (minioClient, error) {
	var client *minio.Client
	var err error

	if s3.ShouldUseIAMCredentials() {
		iam := credentials.NewIAM("")
		client, err = newMinioWithCredentials(DefaultAWSS3Server, iam, true, "")
	} else {
		client, err = newMinio(s3.ServerAddress, s3.AccessKey, s3.SecretKey, !s3.Insecure)
	}

	if err != nil {
		return nil, err
	}

	client.SetCustomTransport(&bucketLocationTripper{
		bucketLocation: s3.BucketLocation,
	})

	return client, nil
}
