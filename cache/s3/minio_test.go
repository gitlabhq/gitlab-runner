package s3

import (
	"errors"
	"testing"

	"github.com/minio/minio-go/v6"
	"github.com/minio/minio-go/v6/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type minioClientInitializationTest struct {
	errorOnInitialization bool
	configurationFactory  func() *common.CacheConfig

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
	cacheConfig.S3.ServerAddress = "s3.amazonaws.com"

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
	newMinio = func(endpoint string, accessKeyID string, secretAccessKey string, secure bool) (*minio.Client, error) {
		if test.expectedToUseIAM {
			t.Error("Should not use regular minio client initializator")
		}

		if test.errorOnInitialization {
			return nil, errors.New("test error")
		}

		if test.expectedInsecure {
			assert.False(t, secure)
		} else {
			assert.True(t, secure)
		}

		client, err := minio.New(endpoint, accessKeyID, secretAccessKey, secure)
		require.NoError(t, err)

		return client, nil
	}

	return func() {
		newMinio = oldNewMinio
	}
}

func runOnFakeMinioWithCredentials(t *testing.T, test minioClientInitializationTest) func() {
	oldNewMinioWithCredentials := newMinioWithCredentials
	newMinioWithCredentials =
		func(endpoint string, creds *credentials.Credentials, secure bool, region string) (*minio.Client, error) {
			if !test.expectedToUseIAM {
				t.Error("Should not use minio with IAM client initializator")
			}

			if test.errorOnInitialization {
				return nil, errors.New("test error")
			}

			assert.Equal(t, "s3.amazonaws.com", endpoint)
			assert.True(t, secure)
			assert.Empty(t, region)

			client, err := minio.NewWithCredentials(endpoint, creds, secure, region)
			require.NoError(t, err)

			return client, nil
		}

	return func() {
		newMinioWithCredentials = oldNewMinioWithCredentials
	}
}
