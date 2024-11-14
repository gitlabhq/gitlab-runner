//go:build !integration

package s3v2

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var defaultTimeout = 1 * time.Hour

const (
	bucketName     = "test"
	objectName     = "key"
	bucketLocation = "location"
)

func defaultCacheFactory() *common.CacheConfig {
	return &common.CacheConfig{
		Type: "s3v2",
		S3: &common.CacheS3Config{
			ServerAddress:  "server.com",
			AccessKey:      "access",
			SecretKey:      "key",
			BucketName:     bucketName,
			BucketLocation: bucketLocation},
	}
}

type cacheOperationTest struct {
	errorOnS3ClientInitialization bool
	errorOnURLPresigning          bool

	presignedURL *url.URL
	expectedURL  *url.URL
}

func onFakeS3URLGenerator(tc cacheOperationTest) func() {
	client := new(mockS3Presigner)

	var err error
	if tc.errorOnURLPresigning {
		err = errors.New("test error")
	}

	client.
		On(
			"PresignURL", mock.Anything, mock.Anything, mock.Anything,
			mock.Anything, mock.Anything,
		).
		Return(cache.PresignedURL{URL: tc.presignedURL}, err)

	oldS3URLGenerator := newS3Client
	newS3Client = func(s3 *common.CacheS3Config, opts ...s3ClientOption) (s3Presigner, error) {
		if tc.errorOnS3ClientInitialization {
			return nil, errors.New("test error")
		}
		return client, nil
	}

	return func() {
		newS3Client = oldS3URLGenerator
	}
}

func testCacheOperation(
	t *testing.T,
	operationName string,
	operation func(adapter cache.Adapter) cache.PresignedURL,
	tc cacheOperationTest,
	cacheConfig *common.CacheConfig,
) {
	t.Run(operationName, func(t *testing.T) {
		cleanupS3URLGeneratorMock := onFakeS3URLGenerator(tc)
		defer cleanupS3URLGeneratorMock()

		adapter, err := New(cacheConfig, defaultTimeout, objectName)

		if tc.errorOnS3ClientInitialization {
			assert.EqualError(t, err, "error while creating S3 cache storage client: test error")

			return
		}
		require.NoError(t, err)

		URL := operation(adapter)
		assert.Equal(t, tc.expectedURL, URL.URL)

		ctx := context.Background()
		assert.Nil(t, adapter.GetGoCloudURL(ctx))

		env, err := adapter.GetUploadEnv(ctx)
		assert.Empty(t, env)
		assert.NoError(t, err)
	})
}

func TestCacheOperation(t *testing.T) {
	URL, err := url.Parse("https://s3.example.com")
	require.NoError(t, err)

	tests := map[string]cacheOperationTest{
		"error-on-s3-client-initialization": {
			errorOnS3ClientInitialization: true,
		},
		"error-on-presigning-url": {
			errorOnURLPresigning: true,
			presignedURL:         URL,
			expectedURL:          nil,
		},
		"presigned-url": {
			presignedURL: URL,
			expectedURL:  URL,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			testCacheOperation(
				t,
				"GetDownloadURL",
				func(adapter cache.Adapter) cache.PresignedURL { return adapter.GetDownloadURL(context.Background()) },
				test,
				defaultCacheFactory(),
			)
			testCacheOperation(
				t,
				"GetUploadURL",
				func(adapter cache.Adapter) cache.PresignedURL { return adapter.GetUploadURL(context.Background()) },
				test,
				defaultCacheFactory(),
			)
		})
	}
}

func TestNoConfiguration(t *testing.T) {
	s3Cache := defaultCacheFactory()
	s3Cache.S3 = nil

	adapter, err := New(s3Cache, defaultTimeout, objectName)
	assert.Nil(t, adapter)

	assert.EqualError(t, err, "missing S3 configuration")
}

func TestGoCloudURL(t *testing.T) {
	enabled := true
	disabled := false
	roleARN := "aws:arn:role:1234"

	tests := map[string]struct {
		objectName string
		config     *common.CacheS3Config
		expected   string
	}{
		"no role ARN": {
			config: defaultCacheFactory().S3,
		},
		"role ARN": {
			config: &common.CacheS3Config{
				BucketName:     "role-bucket",
				BucketLocation: "us-west-1",
				UploadRoleARN:  roleARN,
			},
			expected: "s3://role-bucket/key?awssdk=v2&dualstack=true&region=us-west-1",
		},
		"role ARN with leading slashes in object": {
			objectName: "//" + objectName,
			config: &common.CacheS3Config{
				BucketName:     "role-bucket",
				BucketLocation: "us-west-1",
				UploadRoleARN:  roleARN,
			},
			expected: "s3://role-bucket/key?awssdk=v2&dualstack=true&region=us-west-1",
		},
		"global S3 endpoint": {
			config: &common.CacheS3Config{
				ServerAddress:  "s3.amazonaws.com",
				BucketName:     "custom-bucket",
				BucketLocation: "custom-location",
				UploadRoleARN:  roleARN,
			},
			expected: "s3://custom-bucket/key?awssdk=v2&dualstack=true&region=custom-location",
		},
		"custom endpoint": {
			config: &common.CacheS3Config{
				ServerAddress:  "custom.s3.endpoint.com",
				BucketName:     "custom-bucket",
				BucketLocation: "custom-location",
				UploadRoleARN:  roleARN,
			},
			expected: "s3://custom-bucket/key?awssdk=v2&dualstack=true&endpoint=https%3A%2F%2Fcustom.s3.endpoint.com&hostname_immutable=true&region=custom-location&use_path_style=true",
		},
		"path style": {
			config: &common.CacheS3Config{
				ServerAddress:  "minio.example.com:8080",
				BucketName:     "path-style-bucket",
				BucketLocation: "us-west-2",
				PathStyle:      &enabled,
				UploadRoleARN:  roleARN,
			},
			expected: "s3://path-style-bucket/key?awssdk=v2&dualstack=true&endpoint=https%3A%2F%2Fminio.example.com%3A8080&hostname_immutable=true&region=us-west-2&use_path_style=true",
		},
		"HTTP and path style": {
			config: &common.CacheS3Config{
				ServerAddress:  "minio.example.com:8080",
				Insecure:       true,
				BucketName:     "path-style-bucket",
				BucketLocation: "us-west-2",
				PathStyle:      &enabled,
				UploadRoleARN:  roleARN,
			},
			expected: "s3://path-style-bucket/key?awssdk=v2&dualstack=true&endpoint=http%3A%2F%2Fminio.example.com%3A8080&hostname_immutable=true&region=us-west-2&use_path_style=true",
		},
		"S3 regional endpoint and path style": {
			config: &common.CacheS3Config{
				ServerAddress:  "eu-north-1.s3.amazon.aws.com:443",
				BucketName:     "path-style-bucket",
				BucketLocation: "eu-north-1",
				PathStyle:      &enabled,
				UploadRoleARN:  roleARN,
			},
			expected: "s3://path-style-bucket/key?awssdk=v2&dualstack=true&endpoint=https%3A%2F%2Feu-north-1.s3.amazon.aws.com&hostname_immutable=true&region=eu-north-1&use_path_style=true",
		},
		"dual stack disabled": {
			config: &common.CacheS3Config{
				BucketName:     "dual-stack-bucket",
				BucketLocation: "eu-central-1",
				DualStack:      &disabled,
				UploadRoleARN:  roleARN,
			},
			expected: "s3://dual-stack-bucket/key?awssdk=v2&region=eu-central-1",
		},
		"accelerate": {
			config: &common.CacheS3Config{
				BucketName:     "accelerate-bucket",
				BucketLocation: "us-east-1",
				Accelerate:     true,
				UploadRoleARN:  roleARN,
			},
			expected: "s3://accelerate-bucket/key?accelerate=true&awssdk=v2&dualstack=true&region=us-east-1",
		},
		"S3 encryption": {
			config: &common.CacheS3Config{
				BucketName:           "encrypted-bucket",
				BucketLocation:       "ap-southeast-1",
				UploadRoleARN:        roleARN,
				ServerSideEncryption: "S3",
			},
			expected: "s3://encrypted-bucket/key?awssdk=v2&dualstack=true&region=ap-southeast-1&ssetype=AES256",
		},
		"KMS encryption": {
			config: &common.CacheS3Config{
				BucketName:                "encrypted-bucket",
				BucketLocation:            "ap-southeast-1",
				UploadRoleARN:             roleARN,
				ServerSideEncryption:      "KMS",
				ServerSideEncryptionKeyID: "my-kms-key-id",
			},
			expected: "s3://encrypted-bucket/key?awssdk=v2&dualstack=true&kmskeyid=my-kms-key-id&region=ap-southeast-1&ssetype=aws%3Akms",
		},
		"DSSE-KMS encryption": {
			config: &common.CacheS3Config{
				BucketName:                "encrypted-bucket",
				BucketLocation:            "ap-southeast-1",
				UploadRoleARN:             roleARN,
				ServerSideEncryption:      "DSSE-KMS",
				ServerSideEncryptionKeyID: "my-kms-key-id",
			},
			expected: "s3://encrypted-bucket/key?awssdk=v2&dualstack=true&kmskeyid=my-kms-key-id&region=ap-southeast-1&ssetype=aws%3Akms%3Adsse",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			s3Cache := defaultCacheFactory()
			s3Cache.S3 = tt.config

			if tt.objectName == "" {
				tt.objectName = objectName
			}

			adapter, err := New(s3Cache, defaultTimeout, tt.objectName)
			require.NoError(t, err)

			u := adapter.GetGoCloudURL(context.Background())
			if tt.expected != "" {
				assert.Equal(t, tt.expected, u.String())
			} else {
				assert.Nil(t, u)
			}
		})
	}
}

func TestGetUploadEnv(t *testing.T) {
	tests := map[string]struct {
		config      *common.CacheConfig
		failedFetch bool
		expected    map[string]string
	}{
		"no upload role ARN": {
			config: &common.CacheConfig{
				S3: &common.CacheS3Config{},
			},
			expected: nil,
		},
		"with upload role ARN": {
			config: &common.CacheConfig{
				S3: &common.CacheS3Config{
					BucketName:    bucketName,
					UploadRoleARN: "arn:aws:iam::123456789012:role/S3UploadRole",
				},
			},
			expected: map[string]string{
				"AWS_ACCESS_KEY_ID":     "mock-access-key",
				"AWS_SECRET_ACCESS_KEY": "mock-secret-key",
				"AWS_SESSION_TOKEN":     "mock-session-token",
			},
		},
		"with failed credentials": {
			config: &common.CacheConfig{
				S3: &common.CacheS3Config{
					BucketName:    bucketName,
					UploadRoleARN: "arn:aws:iam::123456789012:role/S3UploadRole",
				},
			},
			failedFetch: true,
			expected:    nil,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			cleanupS3URLGeneratorMock := onFakeS3URLGenerator(cacheOperationTest{})
			defer cleanupS3URLGeneratorMock()

			adapter, err := New(tt.config, defaultTimeout, objectName)
			require.NoError(t, err)

			mockClient := adapter.(*s3Adapter).client.(*mockS3Presigner)

			if tt.failedFetch {
				mockClient.On("FetchCredentialsForRole", mock.Anything, tt.config.S3.UploadRoleARN, bucketName, objectName, mock.Anything).
					Return(nil, errors.New("error fetching credentials"))
			} else {
				mockClient.On("FetchCredentialsForRole", mock.Anything, tt.config.S3.UploadRoleARN, bucketName, objectName, mock.Anything).
					Return(tt.expected, nil)
			}
			env, err := adapter.GetUploadEnv(context.Background())
			assert.Equal(t, tt.expected, env)

			if tt.failedFetch {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
