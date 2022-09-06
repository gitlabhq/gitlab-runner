//go:build !integration

package s3

import (
	"errors"
	"testing"

	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type minioClientInitializationTest struct {
	errorOnInitialization bool
	configurationFactory  func() *common.CacheConfig
	serverAddress         string

	expectedToUseIAM bool
	expectedInsecure bool
}

func TestMinioClientInitialization(t *testing.T) {
	tests := map[string]minioClientInitializationTest{
		"error-on-initialization": {
			errorOnInitialization: true,
			configurationFactory:  defaultCacheFactory,
		},
		"all-credentials-empty": {
			configurationFactory: emptyCredentialsCacheFactory,
			expectedToUseIAM:     true,
		},
		"serverAddress-empty": {
			configurationFactory: emptyServerAddressFactory,
			expectedToUseIAM:     true,
		},
		"accessKey-empty": {
			configurationFactory: emptyAccessKeyFactory,
			expectedToUseIAM:     true,
		},
		"secretKey-empty": {
			configurationFactory: emptySecretKeyFactory,
			expectedToUseIAM:     true,
		},
		"only-ServerAddress-defined": {
			configurationFactory: onlyServerAddressFactory,
			expectedToUseIAM:     true,
			serverAddress:        "s3.customurl.com",
		},
		"only-AccessKey-defined": {
			configurationFactory: onlyAccessKeyFactory,
			expectedToUseIAM:     true,
		},
		"only-SecretKey-defined": {
			configurationFactory: onlySecretKeyFactory,
			expectedToUseIAM:     true,
		},
		"should-use-explicit-credentials": {
			configurationFactory: defaultCacheFactory,
		},
		"should-use-explicit-credentials-with-insecure": {
			configurationFactory: insecureCacheFactory,
			expectedInsecure:     true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			cleanupMinioMock := runOnFakeMinio(t, test)
			defer cleanupMinioMock()

			cleanupMinioCredentialsMock := runOnFakeMinioWithCredentials(t, test)
			defer cleanupMinioCredentialsMock()

			cacheConfig := test.configurationFactory()
			client, err := newMinioClient(cacheConfig.S3)

			if test.errorOnInitialization {
				assert.Error(t, err, "test error")
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, client)
		})
	}
}

func insecureCacheFactory() *common.CacheConfig {
	cacheConfig := defaultCacheFactory()
	cacheConfig.S3.Insecure = true

	return cacheConfig
}

func emptyCredentialsCacheFactory() *common.CacheConfig {
	cacheConfig := defaultCacheFactory()
	cacheConfig.S3.ServerAddress = ""
	cacheConfig.S3.AccessKey = ""
	cacheConfig.S3.SecretKey = ""

	return cacheConfig
}

func emptyServerAddressFactory() *common.CacheConfig {
	cacheConfig := emptyCredentialsCacheFactory()
	cacheConfig.S3.AccessKey = "TOKEN"
	cacheConfig.S3.SecretKey = "TOKEN"

	return cacheConfig
}

func emptyAccessKeyFactory() *common.CacheConfig {
	cacheConfig := emptyCredentialsCacheFactory()
	cacheConfig.S3.ServerAddress = "s3.amazonaws.com"
	cacheConfig.S3.SecretKey = "TOKEN"

	return cacheConfig
}

func emptySecretKeyFactory() *common.CacheConfig {
	cacheConfig := emptyCredentialsCacheFactory()
	cacheConfig.S3.ServerAddress = "s3.amazonaws.com"
	cacheConfig.S3.AccessKey = "TOKEN"

	return cacheConfig
}

func onlyServerAddressFactory() *common.CacheConfig {
	cacheConfig := emptyCredentialsCacheFactory()
	cacheConfig.S3.ServerAddress = "s3.customurl.com"

	return cacheConfig
}

func onlyAccessKeyFactory() *common.CacheConfig {
	cacheConfig := emptyCredentialsCacheFactory()
	cacheConfig.S3.AccessKey = "TOKEN"

	return cacheConfig
}

func onlySecretKeyFactory() *common.CacheConfig {
	cacheConfig := emptyCredentialsCacheFactory()
	cacheConfig.S3.SecretKey = "TOKEN"

	return cacheConfig
}

func runOnFakeMinio(t *testing.T, test minioClientInitializationTest) func() {
	oldNewMinio := newMinio
	newMinio = func(endpoint string, opts *minio.Options) (*minio.Client, error) {
		if test.expectedToUseIAM {
			t.Error("Should not use regular minio client initializer")
		}

		if test.errorOnInitialization {
			return nil, errors.New("test error")
		}

		if test.expectedInsecure {
			assert.False(t, opts.Secure)
		} else {
			assert.True(t, opts.Secure)
		}

		client, err := minio.New(endpoint, opts)
		require.NoError(t, err)

		return client, nil
	}

	return func() {
		newMinio = oldNewMinio
	}
}

func runOnFakeMinioWithCredentials(t *testing.T, test minioClientInitializationTest) func() {
	oldNewMinioWithCredentials := newMinioWithIAM
	newMinioWithIAM =
		func(serverAddress, bucketLocation string) (*minio.Client, error) {
			if !test.expectedToUseIAM {
				t.Error("Should not use minio with IAM client initializator")
			}

			assert.Equal(t, "location", bucketLocation)

			if test.serverAddress == "" {
				assert.Equal(t, DefaultAWSS3Server, serverAddress)
			} else {
				assert.Equal(t, test.serverAddress, serverAddress)
			}

			if test.errorOnInitialization {
				return nil, errors.New("test error")
			}

			client, err := minio.New(serverAddress, &minio.Options{
				Creds:  credentials.NewIAM(""),
				Secure: true,
				Transport: &bucketLocationTripper{
					bucketLocation: bucketLocation,
				},
			})
			require.NoError(t, err)

			return client, nil
		}

	return func() {
		newMinioWithIAM = oldNewMinioWithCredentials
	}
}
