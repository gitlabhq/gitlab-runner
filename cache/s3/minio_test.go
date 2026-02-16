//go:build !integration

package s3

import (
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
)

type minioClientInitializationTest struct {
	errorOnInitialization bool
	configurationFactory  func() *cacheconfig.Config
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

type minioClientInitializationTestS3Accelerate struct {
	serverAddress string
	endpointURL   string
	targetURL     string
	accelerated   bool
	err           error
}

func TestMinioClientInitializationWithAccelerate(t *testing.T) {
	tests := map[string]minioClientInitializationTestS3Accelerate{
		"standard-accelerate-endpoint": {
			serverAddress: "s3-accelerate.amazonaws.com",
			endpointURL:   "s3.amazonaws.com",
			targetURL:     "foo.s3-accelerate.amazonaws.com",
			accelerated:   true,
		},
		"dualstack-region-endpoint": {
			serverAddress: "s3-accelerate.dualstack.us-east-1.amazonaws.com",
			endpointURL:   "s3.dualstack.us-east-1.amazonaws.com",
			targetURL:     "foo.s3-accelerate.dualstack.us-east-1.amazonaws.com",
			accelerated:   true,
		},
		"non-aws-endpoint": {
			serverAddress: "s3-accelerate.min.io",
			endpointURL:   "s3-accelerate.min.io",
			targetURL:     "s3-accelerate.min.io",
		},
		"client-with-error": {
			serverAddress: "s3-accelerate.amazonaws.com",
			endpointURL:   "s3.amazonaws.com",
			targetURL:     "foo.s3-accelerate.amazonaws.com",
			accelerated:   true,
			err:           assert.AnError,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			cleanupMinioMock := runOnFakeMinioWithAccelerateEndpoint(t, test.accelerated, test.err)
			defer cleanupMinioMock()

			cacheConfig := serverAddressAccelerateFactory(test.serverAddress)
			cacheConfig.S3.AccessKey = "TOKEN"
			cacheConfig.S3.SecretKey = "TOKEN"

			client, err := newMinioClient(cacheConfig.S3)
			if test.err != nil {
				require.ErrorIs(t, err, test.err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, client)

			url, err := client.PresignHeader(t.Context(), "GET", "foo", "bar", time.Hour, url.Values{}, http.Header{})
			require.NoError(t, err)
			assert.Equal(t, test.targetURL, url.Host)

			mc, ok := client.(*minio.Client)
			require.True(t, ok)
			assert.Equal(t, test.endpointURL, mc.EndpointURL().Host)
		})
	}
}

func insecureCacheFactory() *cacheconfig.Config {
	cacheConfig := defaultCacheFactory()
	cacheConfig.S3.Insecure = true

	return cacheConfig
}

func emptyCredentialsCacheFactory() *cacheconfig.Config {
	cacheConfig := defaultCacheFactory()
	cacheConfig.S3.ServerAddress = ""
	cacheConfig.S3.AccessKey = ""
	cacheConfig.S3.SecretKey = ""

	return cacheConfig
}

func emptyServerAddressFactory() *cacheconfig.Config {
	cacheConfig := emptyCredentialsCacheFactory()
	cacheConfig.S3.AccessKey = "TOKEN"
	cacheConfig.S3.SecretKey = "TOKEN"

	return cacheConfig
}

func emptyAccessKeyFactory() *cacheconfig.Config {
	cacheConfig := emptyCredentialsCacheFactory()
	cacheConfig.S3.ServerAddress = "s3.amazonaws.com"
	cacheConfig.S3.SecretKey = "TOKEN"

	return cacheConfig
}

func emptySecretKeyFactory() *cacheconfig.Config {
	cacheConfig := emptyCredentialsCacheFactory()
	cacheConfig.S3.ServerAddress = "s3.amazonaws.com"
	cacheConfig.S3.AccessKey = "TOKEN"

	return cacheConfig
}

func onlyServerAddressFactory() *cacheconfig.Config {
	cacheConfig := emptyCredentialsCacheFactory()
	cacheConfig.S3.ServerAddress = "s3.customurl.com"

	return cacheConfig
}

func serverAddressAccelerateFactory(serverAddress string) *cacheconfig.Config {
	cacheConfig := emptyCredentialsCacheFactory()
	cacheConfig.S3.ServerAddress = serverAddress

	return cacheConfig
}

func onlyAccessKeyFactory() *cacheconfig.Config {
	cacheConfig := emptyCredentialsCacheFactory()
	cacheConfig.S3.AccessKey = "TOKEN"

	return cacheConfig
}

func onlySecretKeyFactory() *cacheconfig.Config {
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

func runOnFakeMinioWithAccelerateEndpoint(t *testing.T, accelerated bool, err error) func() {
	oldNewMinio := newMinio
	newMinio = func(endpoint string, opts *minio.Options) (*minio.Client, error) {
		if accelerated {
			assert.NotContains(t, endpoint, "s3-accelerate")
		}

		if err != nil {
			return nil, err
		}

		return minio.New(endpoint, opts)
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
