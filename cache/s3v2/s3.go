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
)

const DEFAULT_AWS_S3_ENDPOINT = "https://s3.amazonaws.com"
const fallbackBucketLocation = "us-east-1"

type s3Presigner interface {
	PresignURL(
		ctx context.Context,
		method string,
		bucketName string,
		objectName string,
		metadata map[string]string,
		expires time.Duration,
	) (cache.PresignedURL, error)
	FetchCredentialsForRole(ctx context.Context, roleARN, bucketName, objectName string, upload bool, timeout time.Duration) (map[string]string, error)
	ServerSideEncryptionType() string
}

type s3Client struct {
	s3Config      *common.CacheS3Config
	awsConfig     *aws.Config
	client        *s3.Client
	presignClient *s3.PresignClient
	stsEndpoint   string
}

type s3ClientOption func(*s3Client)

func withSTSEndpoint(endpoint string) s3ClientOption {
	return func(c *s3Client) {
		c.stsEndpoint = endpoint
	}
}

func (c *s3Client) PresignURL(ctx context.Context,
	method string,
	bucketName string,
	objectName string,
	metadata map[string]string,
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
		if len(metadata) > 0 {
			putObjectInput.Metadata = metadata
		}
		switch c.s3Config.EncryptionType() {
		case common.S3EncryptionTypeAes256:
			putObjectInput.ServerSideEncryption = types.ServerSideEncryptionAes256
		case common.S3EncryptionTypeKms:
			putObjectInput.ServerSideEncryption = types.ServerSideEncryptionAwsKms
			putObjectInput.SSEKMSKeyId = aws.String(c.s3Config.ServerSideEncryptionKeyID)
		case common.S3EncryptionTypeDsseKms:
			putObjectInput.ServerSideEncryption = types.ServerSideEncryptionAwsKmsDsse
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

func (c *s3Client) generateSessionPolicy(bucketName, objectName string, upload bool) string {
	action := "s3:GetObject"
	if upload {
		action = "s3:PutObject"
	}

	// https://docs.aws.amazon.com/IAM/latest/UserGuide/reference-arns.html
	s3Partition := "aws"
	// https://docs.aws.amazon.com/govcloud-us/latest/UserGuide/using-govcloud-arns.html
	switch {
	case strings.HasPrefix(c.awsConfig.Region, "us-gov-"):
		s3Partition = "aws-us-gov"
	case strings.HasPrefix(c.awsConfig.Region, "cn-"):
		s3Partition = "aws-cn"
	}

	policy := fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Effect": "Allow",
				"Action": ["%s"],
				"Resource": "arn:%s:s3:::%s/%s"
			}`, action, s3Partition, bucketName, objectName)

	if c.s3Config.EncryptionType() == common.S3EncryptionTypeKms || c.s3Config.EncryptionType() == common.S3EncryptionTypeDsseKms {
		// Permissions needed for multipart upload: https://repost.aws/knowledge-center/s3-large-file-encryption-kms-key
		policy += fmt.Sprintf(`,
			{
				"Effect": "Allow",
				"Action": [
					"kms:Decrypt",
					"kms:GenerateDataKey"
				],
				"Resource": "%s"
			}`, c.s3Config.ServerSideEncryptionKeyID)
	}

	policy += `
	]
}`

	return policy
}

func (c *s3Client) FetchCredentialsForRole(ctx context.Context, roleARN, bucketName, objectName string, upload bool, timeout time.Duration) (map[string]string, error) {
	sessionPolicy := c.generateSessionPolicy(bucketName, objectName, upload)

	stsClient := sts.NewFromConfig(*c.awsConfig, func(o *sts.Options) {
		if c.stsEndpoint != "" {
			o.BaseEndpoint = aws.String(c.stsEndpoint)
		}
	})
	uuid, err := helpers.GenerateRandomUUID(8)
	if err != nil {
		return nil, fmt.Errorf("failed to generate random UUID: %w", err)
	}
	sessionName := fmt.Sprintf("gitlab-runner-cache-upload-%s", uuid)

	// According to https://docs.aws.amazon.com/IAM/latest/UserGuide/id_roles_manage-assume.html#id_roles_use_view-role-max-session,
	// session durations must be between 15 minutes and 12 hours. However,
	// when role chaining is in use, AWS limits the session duration to 1 hour.
	duration := 1 * time.Hour
	if timeout >= 15*time.Minute && timeout <= 1*time.Hour {
		duration = timeout
	}

	startTime := time.Now()
	roleCredentials, err := stsClient.AssumeRole(ctx, &sts.AssumeRoleInput{
		RoleArn:         aws.String(roleARN),
		RoleSessionName: aws.String(sessionName),
		Policy:          aws.String(sessionPolicy), // Limit the role's access
		DurationSeconds: aws.Int32(int32(duration.Seconds())),
	})
	elapsed := time.Since(startTime).Seconds()

	if err != nil {
		logrus.WithError(err).WithFields(logrus.Fields{
			"role_arn":   roleARN,
			"duration_s": elapsed,
		}).Error("Failed to assume role for cache credentials")
		return nil, fmt.Errorf("failed to assume role (took %.2fs): %w", elapsed, err)
	}
	// AssumeRole should always return credentials if successful, but
	// just in case it doesn't let's check this.
	if roleCredentials.Credentials == nil {
		logrus.WithFields(logrus.Fields{
			"role_arn":   roleARN,
			"duration_s": elapsed,
		}).Error("AssumeRole succeeded but returned no credentials")
		return nil, fmt.Errorf("failed to retrieve credentials (took %.2fs): %w", elapsed, err)
	}

	logrus.WithFields(logrus.Fields{
		"role_arn":   roleARN,
		"duration_s": elapsed,
	}).Debug("Successfully assumed role for cache credentials")

	return map[string]string{
		"AWS_ACCESS_KEY_ID":     *roleCredentials.Credentials.AccessKeyId,
		"AWS_SECRET_ACCESS_KEY": *roleCredentials.Credentials.SecretAccessKey,
		"AWS_SESSION_TOKEN":     *roleCredentials.Credentials.SessionToken,
		"AWS_PROFILE":           "", // Ignore user-defined values
	}, nil
}

func (c *s3Client) ServerSideEncryptionType() string {
	return s3EncryptionType(c.s3Config.EncryptionType())
}

func s3EncryptionType(encryptionType common.S3EncryptionType) string {
	switch encryptionType {
	case common.S3EncryptionTypeAes256:
		return string(types.ServerSideEncryptionAes256)
	case common.S3EncryptionTypeKms:
		return string(types.ServerSideEncryptionAwsKms)
	case common.S3EncryptionTypeDsseKms:
		return string(types.ServerSideEncryptionAwsKmsDsse)
	default:
		return ""
	}
}

func newRawS3Client(s3Config *common.CacheS3Config) (*aws.Config, *s3.Client, error) {
	var cfg aws.Config
	var err error
	options := make([]func(*config.LoadOptions) error, 0)

	endpoint := s3Config.GetEndpoint()

	switch s3Config.AuthType() {
	case common.S3AuthTypeIAM:
		break
	case common.S3AuthTypeAccessKey:
		options = append(options,
			config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(s3Config.AccessKey, s3Config.SecretKey, s3Config.SessionToken)),
		)
	}

	bucketLocation := s3Config.BucketLocation
	if bucketLocation == "" {
		bucketLocation = detectBucketLocation(s3Config.BucketName, options...)
	}

	options = append(options, config.WithRegion(bucketLocation))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err = config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, nil, err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		// To preserve backwards compatibility, configs that set ServerAddress to
		// "s3.amazonaws.com" don't need a custom endpoint since that is the default
		// S3 address.
		//
		// The AWS SDK doesn't allow you to generate a pre-signed URL with a custom endpoint
		// and DualStack or Accelerate options set.
		if endpoint != "" && endpoint != DEFAULT_AWS_S3_ENDPOINT {
			o.BaseEndpoint = aws.String(endpoint)
		} else {
			o.UseDualstack = s3Config.DualStackEnabled() // nolint:staticcheck
			o.UseAccelerate = s3Config.Accelerate
		}
		o.UsePathStyle = s3Config.PathStyleEnabled()
	})

	return &cfg, client, nil
}

func detectBucketLocation(bucketName string, optFuncs ...func(*config.LoadOptions) error) string {
	// The 30 seconds timeout here is arbritrary
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// When s3 is configured with an IAM profile, a default region must be set
	// We therefore set the default region to us-east-1
	configOpts := append(
		[]func(*config.LoadOptions) error{
			config.WithRegion(fallbackBucketLocation),
		},
		optFuncs...,
	)

	cfg, err := config.LoadDefaultConfig(ctx, configOpts...)
	if err != nil {
		return fallbackBucketLocation
	}

	client := s3.NewFromConfig(cfg)
	output, err := client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{
		Bucket: aws.String(bucketName),
	})

	switch {
	case err != nil || output.LocationConstraint == "":
		return fallbackBucketLocation
	case output.LocationConstraint == types.BucketLocationConstraintEu:
		return string(types.BucketLocationConstraintEuWest1)
	}

	return string(output.LocationConstraint)
}

var newS3Client = func(s3Config *common.CacheS3Config, options ...s3ClientOption) (s3Presigner, error) {
	cfg, client, err := newRawS3Client(s3Config)
	if err != nil {
		return nil, err
	}

	presignClient := s3.NewPresignClient(client)

	c := &s3Client{
		s3Config:      s3Config,
		awsConfig:     cfg,
		client:        client,
		presignClient: presignClient,
	}

	for _, opt := range options {
		opt(c)
	}

	return c, nil
}
