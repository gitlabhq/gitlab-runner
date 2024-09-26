package s3v2

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/sirupsen/logrus"

	"github.com/minio/minio-go/v7/pkg/s3utils"
)

//go:generate mockery --name=s3Presigner --inpackage
type s3Presigner interface {
	PresignURL(
		ctx context.Context,
		method string,
		bucketName string,
		objectName string,
		expires time.Duration,
	) (cache.PresignedURL, error)
	FetchCredentialsForRole(ctx context.Context, roleARN, bucketName, objectName string) (map[string]string, error)
}

type s3Client struct {
	s3Config      *common.CacheS3Config
	awsConfig     *aws.Config
	client        *s3.Client
	presignClient *s3.PresignClient
}

func (c *s3Client) PresignURL(ctx context.Context,
	method string,
	bucketName string,
	objectName string,
	expires time.Duration) (cache.PresignedURL, error) {
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
		return cache.PresignedURL{}, fmt.Errorf("unsupported method: %s", method)
	}

	if err != nil {
		logrus.WithError(err).Error("error while generating S3 pre-signed URL")
		return cache.PresignedURL{}, err
	}

	u, err := url.Parse(presignedReq.URL)
	if err != nil {
		logrus.WithError(err).WithField("url", presignedReq.URL).Errorf("error parsing S3 URL")
		return cache.PresignedURL{}, err
	}

	return cache.PresignedURL{URL: u, Headers: presignedReq.SignedHeader}, nil
}

func (c *s3Client) FetchCredentialsForRole(ctx context.Context, roleARN, bucketName, objectName string) (map[string]string, error) {
	sessionPolicy := fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Effect": "Allow",
					"Action": ["s3:PutObject"],
					"Resource": "arn:aws:s3:::%s/%s"
				}
			]
		}`, bucketName, objectName)

	stsClient := sts.NewFromConfig(*c.awsConfig)

	uuid, err := helpers.GenerateRandomUUID(8)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random UUID: %v", err)
	}
	sessionName := fmt.Sprintf("gitlab-runner-cache-upload-%s", uuid)

	roleCredentials, err := stsClient.AssumeRole(ctx, &sts.AssumeRoleInput{
		RoleArn:         aws.String(roleARN),
		RoleSessionName: aws.String(sessionName),
		Policy:          aws.String(sessionPolicy), // Limit the role's access
		DurationSeconds: aws.Int32(3600),           // Set a short lifetime for the session
	})
	if err != nil {
		return nil, fmt.Errorf("failed to assume role: %v", err)
	}

	return map[string]string{
		"AWS_ACCESS_KEY_ID":     *roleCredentials.Credentials.AccessKeyId,
		"AWS_SECRET_ACCESS_KEY": *roleCredentials.Credentials.SecretAccessKey,
		"AWS_SESSION_TOKEN":     *roleCredentials.Credentials.SessionToken,
	}, nil
}

func newRawS3Client(s3Config *common.CacheS3Config) (*aws.Config, *s3.Client, error) {
	var cfg aws.Config
	var err error

	endpoint := s3Config.GetEndpoint()
	var endpointURL *url.URL
	if endpoint != "" {
		endpointURL, err = url.Parse(endpoint)
		if err != nil {
			return nil, nil, fmt.Errorf("parsing S3 endpoint URL: %w", err)
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
		return nil, nil, err
	}

	usePathStyle := false
	if endpoint != "" {
		usePathStyle = !s3utils.IsVirtualHostSupported(*endpointURL, s3Config.BucketName)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		} else {
			o.UseDualstack = s3Config.DualStackEnabled() // nolint:staticcheck
			o.UseAccelerate = s3Config.Accelerate
		}
		o.UsePathStyle = usePathStyle
	})

	return &cfg, client, nil
}

var newS3Client = func(s3Config *common.CacheS3Config) (s3Presigner, error) {
	cfg, client, err := newRawS3Client(s3Config)
	if err != nil {
		return nil, err
	}

	presignClient := s3.NewPresignClient(client)

	return &s3Client{
		s3Config:      s3Config,
		awsConfig:     cfg,
		client:        client,
		presignClient: presignClient,
	}, nil
}
