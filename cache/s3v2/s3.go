package s3v2

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/sirupsen/logrus"

	"github.com/minio/minio-go/v7/pkg/s3utils"
)

var s3AcceleratePattern = regexp.MustCompile(`s3-accelerate.*\.amazonaws\.com$`)

//go:generate mockery --name=s3Presigner --inpackage
type s3Presigner interface {
	PresignURL(
		ctx context.Context,
		method string,
		bucketName string,
		objectName string,
		expires time.Duration,
	) (*url.URL, error)
}

type s3Client struct {
	s3Config      *common.CacheS3Config
	client        *s3.Client
	presignClient *s3.PresignClient
}

func (c *s3Client) PresignURL(ctx context.Context,
	method string,
	bucketName string,
	objectName string,
	expires time.Duration) (*url.URL, error) {
	var presignedReq *v4.PresignedHTTPRequest
	var err error

	switch method {
	case http.MethodGet:
		getObjectInput := &s3.GetObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(objectName),
		}
		presignedReq, err = c.presignClient.PresignGetObject(ctx, getObjectInput, s3.WithPresignExpires(expires))
	case http.MethodPut:
		putObjectInput := &s3.PutObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(objectName),
		}
		switch strings.ToUpper(c.s3Config.ServerSideEncryption) {
		case "S3":
			putObjectInput.ServerSideEncryption = types.ServerSideEncryptionAes256
		case "KMS":
			putObjectInput.ServerSideEncryption = types.ServerSideEncryptionAwsKms
			putObjectInput.SSEKMSKeyId = aws.String(c.s3Config.ServerSideEncryptionKeyID)
		}
		presignedReq, err = c.presignClient.PresignPutObject(ctx, putObjectInput, s3.WithPresignExpires(expires))
	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}

	if err != nil {
		logrus.WithError(err).Error("error while generating S3 pre-signed URL")
		return nil, err
	}

	u, err := url.Parse(presignedReq.URL)
	if err != nil {
		logrus.WithError(err).WithField("url", presignedReq.URL).Errorf("error parsing S3 URL")
		return nil, err
	}

	return u, nil
}

func newRawS3Client(s3Config *common.CacheS3Config) (*s3.Client, error) {
	var cfg aws.Config
	var err error

	endpoint := s3Config.ServerAddress
	var endpointURL *url.URL
	if endpoint != "" {
		scheme := "https"
		if s3Config.Insecure {
			scheme = "http"
		}
		endpoint = fmt.Sprintf("%s://%s", scheme, s3Config.ServerAddress)
		endpointURL, err = url.Parse(endpoint)
		if err != nil {
			return nil, fmt.Errorf("parsing S3 endpoint URL: %w", err)
		}
	}

	options := []func(*config.LoadOptions) error{config.WithRegion(s3Config.BucketLocation)}

	switch s3Config.AuthType() {
	case common.S3AuthTypeIAM:
		break
	case common.S3AuthTypeAccessKey:
		options = append(options,
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(s3Config.AccessKey, s3Config.SecretKey, "")),
		)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err = config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, err
	}

	usePathStyle := false
	if endpoint != "" {
		usePathStyle = !s3utils.IsVirtualHostSupported(*endpointURL, s3Config.BucketName)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = usePathStyle
		o.UseAccelerate = s3AcceleratePattern.MatchString(endpoint)
	})

	return client, nil
}

var newS3Client = func(s3Config *common.CacheS3Config) (s3Presigner, error) {
	client, err := newRawS3Client(s3Config)
	if err != nil {
		return nil, err
	}

	presignClient := s3.NewPresignClient(client)

	return &s3Client{
		s3Config:      s3Config,
		client:        client,
		presignClient: presignClient,
	}, nil
}
