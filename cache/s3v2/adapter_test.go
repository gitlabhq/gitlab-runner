//go:build !integration

package s3v2

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
)

var defaultTimeout = 1 * time.Hour

const (
	bucketName     = "test"
	objectName     = "key"
	bucketLocation = "location"
)

func defaultCacheFactory() *cacheconfig.Config {
	return &cacheconfig.Config{
		Type: "s3v2",
		S3: &cacheconfig.CacheS3Config{
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

	metadata                map[string]string
	presignedURL            *url.URL
	expectedURL             *url.URL
	expectedUploadHeaders   http.Header
	expectedDownloadHeaders http.Header
}

func onFakeS3URLGenerator(t *testing.T, tc cacheOperationTest) {
	client := newMockS3Presigner(t)

	var err error
	if tc.errorOnURLPresigning {
		err = errors.New("test error")
	}

	client.
		On(
			"PresignURL",
			mock.Anything, // context
			mock.Anything, // http method
			mock.Anything, // bucket name
			mock.Anything, // object name
			mock.Anything, // metadata
			mock.Anything, // valid time
		).
		Return(cache.PresignedURL{URL: tc.presignedURL}, err).Maybe()

	oldS3URLGenerator := newS3Client
	newS3Client = func(s3 *cacheconfig.CacheS3Config, opts ...s3ClientOption) (s3Presigner, error) {
		if tc.errorOnS3ClientInitialization {
			return nil, errors.New("test error")
		}
		return client, nil
	}

	t.Cleanup(func() {
		newS3Client = oldS3URLGenerator
	})
}

func testCacheOperation(
	t *testing.T,
	operationName string,
	operation func(adapter cache.Adapter) cache.PresignedURL,
	tc cacheOperationTest,
	cacheConfig *cacheconfig.Config,
) {
	t.Run(operationName, func(t *testing.T) {
		onFakeS3URLGenerator(t, tc)

		adapter, err := New(cacheConfig, defaultTimeout, objectName)

		if tc.errorOnS3ClientInitialization {
			assert.EqualError(t, err, "error while creating S3 cache storage client: test error")

			return
		}
		require.NoError(t, err)

		adapter.WithMetadata(tc.metadata)

		URL := operation(adapter)
		assert.Equal(t, tc.expectedURL, URL.URL)

		switch operationName {
		case "GetUploadURL":
			assert.Equal(t, tc.expectedUploadHeaders, URL.Headers, "upload headers")
		case "GetDownloadURL":
			assert.Equal(t, tc.expectedDownloadHeaders, URL.Headers, "download headers")
		default:
			// nothing to do (yet)
		}

		ctx := t.Context()

		goCloudURL, err := adapter.GetGoCloudURL(ctx, true)
		assert.NoError(t, err)
		assert.Nil(t, goCloudURL.URL)
		assert.Empty(t, goCloudURL.Environment)

		goCloudURL, err = adapter.GetGoCloudURL(ctx, false)
		assert.NoError(t, err)
		assert.Nil(t, goCloudURL.URL)
		assert.Empty(t, goCloudURL.Environment)
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
		"presigned-url-with-metadata": {
			presignedURL:          URL,
			metadata:              map[string]string{"foo": "some foo"},
			expectedURL:           URL,
			expectedUploadHeaders: http.Header{"X-Amz-Meta-Foo": []string{"some foo"}},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			testCacheOperation(
				t,
				"GetDownloadURL",
				func(adapter cache.Adapter) cache.PresignedURL { return adapter.GetDownloadURL(t.Context()) },
				test,
				defaultCacheFactory(),
			)
			testCacheOperation(
				t,
				"GetUploadURL",
				func(adapter cache.Adapter) cache.PresignedURL { return adapter.GetUploadURL(t.Context()) },
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

func TestGoCloudURLWithRoleARN(t *testing.T) {
	enabled := true
	disabled := false
	roleARN := "aws:arn:role:1234"

	expectedCredentials := map[string]string{
		"AWS_ACCESS_KEY_ID":     "mock-access-key",
		"AWS_SECRET_ACCESS_KEY": "mock-secret-key",
		"AWS_SESSION_TOKEN":     "mock-session-token",
	}

	tests := map[string]struct {
		objectName    string
		config        *cacheconfig.CacheS3Config
		expected      string
		noCredentials bool
		failedFetch   bool
	}{
		"no role ARN": {
			config:        defaultCacheFactory().S3,
			noCredentials: true,
		},
		"role ARN": {
			config: &cacheconfig.CacheS3Config{
				BucketName:     "role-bucket",
				BucketLocation: "us-west-1",
				RoleARN:        roleARN,
			},
			expected: "s3://role-bucket/key?awssdk=v2&dualstack=true&region=us-west-1",
		},
		"role ARN with leading slashes in object": {
			objectName: "//" + objectName,
			config: &cacheconfig.CacheS3Config{
				BucketName:     "role-bucket",
				BucketLocation: "us-west-1",
				RoleARN:        roleARN,
			},
			expected: "s3://role-bucket/key?awssdk=v2&dualstack=true&region=us-west-1",
		},
		"global S3 endpoint": {
			config: &cacheconfig.CacheS3Config{
				ServerAddress:  "s3.amazonaws.com",
				BucketName:     "custom-bucket",
				BucketLocation: "custom-location",
				RoleARN:        roleARN,
			},
			expected: "s3://custom-bucket/key?awssdk=v2&dualstack=true&region=custom-location",
		},
		"custom endpoint": {
			config: &cacheconfig.CacheS3Config{
				ServerAddress:  "custom.s3.endpoint.com",
				BucketName:     "custom-bucket",
				BucketLocation: "custom-location",
				RoleARN:        roleARN,
			},
			expected: "s3://custom-bucket/key?awssdk=v2&dualstack=true&endpoint=https%3A%2F%2Fcustom.s3.endpoint.com&hostname_immutable=true&region=custom-location&use_path_style=true",
		},
		"path style": {
			config: &cacheconfig.CacheS3Config{
				ServerAddress:  "minio.example.com:8080",
				BucketName:     "path-style-bucket",
				BucketLocation: "us-west-2",
				PathStyle:      &enabled,
				RoleARN:        roleARN,
			},
			expected: "s3://path-style-bucket/key?awssdk=v2&dualstack=true&endpoint=https%3A%2F%2Fminio.example.com%3A8080&hostname_immutable=true&region=us-west-2&use_path_style=true",
		},
		"HTTP and path style": {
			config: &cacheconfig.CacheS3Config{
				ServerAddress:  "minio.example.com:8080",
				Insecure:       true,
				BucketName:     "path-style-bucket",
				BucketLocation: "us-west-2",
				PathStyle:      &enabled,
				RoleARN:        roleARN,
			},
			expected: "s3://path-style-bucket/key?awssdk=v2&dualstack=true&endpoint=http%3A%2F%2Fminio.example.com%3A8080&hostname_immutable=true&region=us-west-2&use_path_style=true",
		},
		"S3 regional endpoint and path style": {
			config: &cacheconfig.CacheS3Config{
				ServerAddress:  "eu-north-1.s3.amazon.aws.com:443",
				BucketName:     "path-style-bucket",
				BucketLocation: "eu-north-1",
				PathStyle:      &enabled,
				RoleARN:        roleARN,
			},
			expected: "s3://path-style-bucket/key?awssdk=v2&dualstack=true&endpoint=https%3A%2F%2Feu-north-1.s3.amazon.aws.com&hostname_immutable=true&region=eu-north-1&use_path_style=true",
		},
		"dual stack disabled": {
			config: &cacheconfig.CacheS3Config{
				BucketName:     "dual-stack-bucket",
				BucketLocation: "eu-central-1",
				DualStack:      &disabled,
				RoleARN:        roleARN,
			},
			expected: "s3://dual-stack-bucket/key?awssdk=v2&region=eu-central-1",
		},
		"accelerate": {
			config: &cacheconfig.CacheS3Config{
				BucketName:     "accelerate-bucket",
				BucketLocation: "us-east-1",
				Accelerate:     true,
				RoleARN:        roleARN,
			},
			expected: "s3://accelerate-bucket/key?accelerate=true&awssdk=v2&dualstack=true&region=us-east-1",
		},
		"S3 encryption": {
			config: &cacheconfig.CacheS3Config{
				BucketName:           "encrypted-bucket",
				BucketLocation:       "ap-southeast-1",
				RoleARN:              roleARN,
				ServerSideEncryption: "S3",
			},
			expected: "s3://encrypted-bucket/key?awssdk=v2&dualstack=true&region=ap-southeast-1&ssetype=AES256",
		},
		"KMS encryption": {
			config: &cacheconfig.CacheS3Config{
				BucketName:                "encrypted-bucket",
				BucketLocation:            "ap-southeast-1",
				RoleARN:                   roleARN,
				ServerSideEncryption:      "KMS",
				ServerSideEncryptionKeyID: "my-kms-key-id",
			},
			expected: "s3://encrypted-bucket/key?awssdk=v2&dualstack=true&kmskeyid=my-kms-key-id&region=ap-southeast-1&ssetype=aws%3Akms",
		},
		"DSSE-KMS encryption": {
			config: &cacheconfig.CacheS3Config{
				BucketName:                "encrypted-bucket",
				BucketLocation:            "ap-southeast-1",
				RoleARN:                   roleARN,
				ServerSideEncryption:      "DSSE-KMS",
				ServerSideEncryptionKeyID: "my-kms-key-id",
			},
			expected: "s3://encrypted-bucket/key?awssdk=v2&dualstack=true&kmskeyid=my-kms-key-id&region=ap-southeast-1&ssetype=aws%3Akms%3Adsse",
		},
		"with failed credentials": {
			config: &cacheconfig.CacheS3Config{
				BucketName:     "role-bucket",
				BucketLocation: "us-west-1",
				RoleARN:        roleARN,
			},
			failedFetch: true,
			expected:    "s3://role-bucket/key?awssdk=v2&dualstack=true&region=us-west-1",
		},
	}

	for tn, tt := range tests {
		for _, uploadMode := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s upload=%v", tn, uploadMode), func(t *testing.T) {
				onFakeS3URLGenerator(t, cacheOperationTest{})

				s3Cache := defaultCacheFactory()
				s3Cache.S3 = tt.config

				if tt.objectName == "" {
					tt.objectName = objectName
				}

				adapter, err := New(s3Cache, defaultTimeout, tt.objectName)
				require.NoError(t, err)

				mockClient := adapter.(*s3Adapter).client.(*mockS3Presigner)
				mockClient.On("ServerSideEncryptionType").Return(s3EncryptionType(tt.config.EncryptionType())).Maybe()

				if tt.failedFetch {
					mockClient.On("FetchCredentialsForRole", mock.Anything, tt.config.RoleARN, tt.config.BucketName, mock.Anything, uploadMode, mock.Anything).
						Return(nil, errors.New("error fetching credentials"))
				} else {
					mockClient.On("FetchCredentialsForRole", mock.Anything, tt.config.RoleARN, tt.config.BucketName, mock.Anything, uploadMode, mock.Anything).
						Return(expectedCredentials, nil).Maybe()
				}

				u, err := adapter.GetGoCloudURL(t.Context(), uploadMode)

				if tt.failedFetch {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}

				if tt.noCredentials || tt.failedFetch {
					assert.Empty(t, u.Environment)
				} else {
					assert.Equal(t, expectedCredentials, u.Environment)
				}

				if tt.expected != "" {
					assert.Equal(t, tt.expected, u.URL.String())
				} else {
					assert.Nil(t, u.URL)
				}
			})
		}
	}
}

func TestGoCloudURLWithUploadRoleARN(t *testing.T) {
	enabled := true
	disabled := false
	roleARN := "aws:arn:role:1234"

	expectedCredentials := map[string]string{
		"AWS_ACCESS_KEY_ID":     "mock-access-key",
		"AWS_SECRET_ACCESS_KEY": "mock-secret-key",
		"AWS_SESSION_TOKEN":     "mock-session-token",
	}

	tests := map[string]struct {
		objectName    string
		config        *cacheconfig.CacheS3Config
		expected      string
		noCredentials bool
		failedFetch   bool
	}{
		"no role ARN": {
			config:        defaultCacheFactory().S3,
			noCredentials: true,
		},
		"role ARN": {
			config: &cacheconfig.CacheS3Config{
				BucketName:     "role-bucket",
				BucketLocation: "us-west-1",
				UploadRoleARN:  roleARN,
			},
			expected: "s3://role-bucket/key?awssdk=v2&dualstack=true&region=us-west-1",
		},
		"role ARN with leading slashes in object": {
			objectName: "//" + objectName,
			config: &cacheconfig.CacheS3Config{
				BucketName:     "role-bucket",
				BucketLocation: "us-west-1",
				UploadRoleARN:  roleARN,
			},
			expected: "s3://role-bucket/key?awssdk=v2&dualstack=true&region=us-west-1",
		},
		"global S3 endpoint": {
			config: &cacheconfig.CacheS3Config{
				ServerAddress:  "s3.amazonaws.com",
				BucketName:     "custom-bucket",
				BucketLocation: "custom-location",
				UploadRoleARN:  roleARN,
			},
			expected: "s3://custom-bucket/key?awssdk=v2&dualstack=true&region=custom-location",
		},
		"custom endpoint": {
			config: &cacheconfig.CacheS3Config{
				ServerAddress:  "custom.s3.endpoint.com",
				BucketName:     "custom-bucket",
				BucketLocation: "custom-location",
				UploadRoleARN:  roleARN,
			},
			expected: "s3://custom-bucket/key?awssdk=v2&dualstack=true&endpoint=https%3A%2F%2Fcustom.s3.endpoint.com&hostname_immutable=true&region=custom-location&use_path_style=true",
		},
		"path style": {
			config: &cacheconfig.CacheS3Config{
				ServerAddress:  "minio.example.com:8080",
				BucketName:     "path-style-bucket",
				BucketLocation: "us-west-2",
				PathStyle:      &enabled,
				UploadRoleARN:  roleARN,
			},
			expected: "s3://path-style-bucket/key?awssdk=v2&dualstack=true&endpoint=https%3A%2F%2Fminio.example.com%3A8080&hostname_immutable=true&region=us-west-2&use_path_style=true",
		},
		"HTTP and path style": {
			config: &cacheconfig.CacheS3Config{
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
			config: &cacheconfig.CacheS3Config{
				ServerAddress:  "eu-north-1.s3.amazon.aws.com:443",
				BucketName:     "path-style-bucket",
				BucketLocation: "eu-north-1",
				PathStyle:      &enabled,
				UploadRoleARN:  roleARN,
			},
			expected: "s3://path-style-bucket/key?awssdk=v2&dualstack=true&endpoint=https%3A%2F%2Feu-north-1.s3.amazon.aws.com&hostname_immutable=true&region=eu-north-1&use_path_style=true",
		},
		"dual stack disabled": {
			config: &cacheconfig.CacheS3Config{
				BucketName:     "dual-stack-bucket",
				BucketLocation: "eu-central-1",
				DualStack:      &disabled,
				UploadRoleARN:  roleARN,
			},
			expected: "s3://dual-stack-bucket/key?awssdk=v2&region=eu-central-1",
		},
		"accelerate": {
			config: &cacheconfig.CacheS3Config{
				BucketName:     "accelerate-bucket",
				BucketLocation: "us-east-1",
				Accelerate:     true,
				UploadRoleARN:  roleARN,
			},
			expected: "s3://accelerate-bucket/key?accelerate=true&awssdk=v2&dualstack=true&region=us-east-1",
		},
		"S3 encryption": {
			config: &cacheconfig.CacheS3Config{
				BucketName:           "encrypted-bucket",
				BucketLocation:       "ap-southeast-1",
				UploadRoleARN:        roleARN,
				ServerSideEncryption: "S3",
			},
			expected: "s3://encrypted-bucket/key?awssdk=v2&dualstack=true&region=ap-southeast-1&ssetype=AES256",
		},
		"KMS encryption": {
			config: &cacheconfig.CacheS3Config{
				BucketName:                "encrypted-bucket",
				BucketLocation:            "ap-southeast-1",
				UploadRoleARN:             roleARN,
				ServerSideEncryption:      "KMS",
				ServerSideEncryptionKeyID: "my-kms-key-id",
			},
			expected: "s3://encrypted-bucket/key?awssdk=v2&dualstack=true&kmskeyid=my-kms-key-id&region=ap-southeast-1&ssetype=aws%3Akms",
		},
		"DSSE-KMS encryption": {
			config: &cacheconfig.CacheS3Config{
				BucketName:                "encrypted-bucket",
				BucketLocation:            "ap-southeast-1",
				UploadRoleARN:             roleARN,
				ServerSideEncryption:      "DSSE-KMS",
				ServerSideEncryptionKeyID: "my-kms-key-id",
			},
			expected: "s3://encrypted-bucket/key?awssdk=v2&dualstack=true&kmskeyid=my-kms-key-id&region=ap-southeast-1&ssetype=aws%3Akms%3Adsse",
		},
		"with failed credentials": {
			config: &cacheconfig.CacheS3Config{
				BucketName:     "role-bucket",
				BucketLocation: "us-west-1",
				UploadRoleARN:  roleARN,
			},
			failedFetch: true,
			expected:    "s3://role-bucket/key?awssdk=v2&dualstack=true&region=us-west-1",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			onFakeS3URLGenerator(t, cacheOperationTest{})

			s3Cache := defaultCacheFactory()
			s3Cache.S3 = tt.config

			if tt.objectName == "" {
				tt.objectName = objectName
			}

			adapter, err := New(s3Cache, defaultTimeout, tt.objectName)
			require.NoError(t, err)

			mockClient := adapter.(*s3Adapter).client.(*mockS3Presigner)

			if !tt.noCredentials {
				mockClient.On("ServerSideEncryptionType").Return(s3EncryptionType(tt.config.EncryptionType()))

				if tt.failedFetch {
					mockClient.On("FetchCredentialsForRole", mock.Anything, tt.config.UploadRoleARN, tt.config.BucketName, mock.Anything, true, mock.Anything).
						Return(nil, errors.New("error fetching credentials"))
				} else {
					mockClient.On("FetchCredentialsForRole", mock.Anything, tt.config.UploadRoleARN, tt.config.BucketName, mock.Anything, true, mock.Anything).
						Return(expectedCredentials, nil)
				}
			}

			u, err := adapter.GetGoCloudURL(t.Context(), true)

			if tt.failedFetch {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if tt.noCredentials || tt.failedFetch {
				assert.Empty(t, u.Environment)
			} else {
				assert.Equal(t, expectedCredentials, u.Environment)
			}

			if tt.expected != "" {
				assert.Equal(t, tt.expected, u.URL.String())
			} else {
				assert.Nil(t, u.URL)
			}

			du, err := adapter.GetGoCloudURL(t.Context(), false)
			require.NoError(t, err)
			assert.Nil(t, du.URL)
			assert.Empty(t, du.Environment)
		})
	}
}
