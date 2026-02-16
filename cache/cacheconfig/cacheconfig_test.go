//go:build !integration

package cacheconfig_test

import (
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestCacheGCSConfig_UniverseDomain(t *testing.T) {
	tests := map[string]struct {
		config         string
		expectedDomain string
		validateConfig func(t *testing.T, config *common.Config)
	}{
		"universe domain not set": {
			config: `
[[runners]]
	[runners.cache.gcs]
		BucketName = "test-bucket"
`,
			expectedDomain: "",
		},
		"universe domain set to googleapis.com": {
			config: `
[[runners]]
	[runners.cache.gcs]
		BucketName = "test-bucket"
		UniverseDomain = "googleapis.com"
`,
			expectedDomain: "googleapis.com",
		},
		"universe domain set to custom universe": {
			config: `
[[runners]]
	[runners.cache.gcs]
		BucketName = "test-bucket"
		UniverseDomain = "custom.universe.com"
`,
			expectedDomain: "custom.universe.com",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := common.NewConfig()
			_, err := toml.Decode(tt.config, cfg)
			assert.NoError(t, err)

			require.Len(t, cfg.Runners, 1)
			require.NotNil(t, cfg.Runners[0].Cache)
			require.NotNil(t, cfg.Runners[0].Cache.GCS)
			assert.Equal(t, tt.expectedDomain, cfg.Runners[0].Cache.GCS.UniverseDomain)
		})
	}
}

func TestCacheS3Config_AuthType(t *testing.T) {
	tests := map[string]struct {
		s3       cacheconfig.CacheS3Config
		authType cacheconfig.S3AuthType
	}{
		"Everything is empty": {
			s3: cacheconfig.CacheS3Config{
				ServerAddress:  "",
				AccessKey:      "",
				SecretKey:      "",
				BucketName:     "name",
				BucketLocation: "us-east-1a",
			},
			authType: cacheconfig.S3AuthTypeIAM,
		},
		"Both AccessKey & SecretKey are empty": {
			s3: cacheconfig.CacheS3Config{
				ServerAddress:  "s3.amazonaws.com",
				AccessKey:      "",
				SecretKey:      "",
				BucketName:     "name",
				BucketLocation: "us-east-1a",
			},
			authType: cacheconfig.S3AuthTypeIAM,
		},
		"SecretKey is empty": {
			s3: cacheconfig.CacheS3Config{
				ServerAddress:  "s3.amazonaws.com",
				AccessKey:      "TOKEN",
				SecretKey:      "",
				BucketName:     "name",
				BucketLocation: "us-east-1a",
			},
			authType: cacheconfig.S3AuthTypeIAM,
		},
		"AccessKey is empty": {
			s3: cacheconfig.CacheS3Config{
				ServerAddress:  "s3.amazonaws.com",
				AccessKey:      "",
				SecretKey:      "TOKEN",
				BucketName:     "name",
				BucketLocation: "us-east-1a",
			},
			authType: cacheconfig.S3AuthTypeIAM,
		},
		"ServerAddress is empty": {
			s3: cacheconfig.CacheS3Config{
				ServerAddress:  "",
				AccessKey:      "TOKEN",
				SecretKey:      "TOKEN",
				BucketName:     "name",
				BucketLocation: "us-east-1a",
			},
			authType: cacheconfig.S3AuthTypeIAM,
		},
		"ServerAddress & AccessKey are empty": {
			s3: cacheconfig.CacheS3Config{
				ServerAddress:  "",
				AccessKey:      "",
				SecretKey:      "TOKEN",
				BucketName:     "name",
				BucketLocation: "us-east-1a",
			},
			authType: cacheconfig.S3AuthTypeIAM,
		},
		"ServerAddress & SecretKey are empty": {
			s3: cacheconfig.CacheS3Config{
				ServerAddress:  "",
				AccessKey:      "TOKEN",
				SecretKey:      "",
				BucketName:     "name",
				BucketLocation: "us-east-1a",
			},
			authType: cacheconfig.S3AuthTypeIAM,
		},
		"Nothing is empty": {
			s3: cacheconfig.CacheS3Config{
				ServerAddress:  "s3.amazonaws.com",
				AccessKey:      "TOKEN",
				SecretKey:      "TOKEN",
				BucketName:     "name",
				BucketLocation: "us-east-1a",
			},
			authType: cacheconfig.S3AuthTypeAccessKey,
		},
		"IAM set as auth type": {
			s3: cacheconfig.CacheS3Config{
				ServerAddress:      "s3.amazonaws.com",
				AccessKey:          "TOKEN",
				SecretKey:          "TOKEN",
				AuthenticationType: cacheconfig.S3AuthTypeIAM,
				BucketName:         "name",
				BucketLocation:     "us-east-1a",
			},
			authType: cacheconfig.S3AuthTypeIAM,
		},
		"Root credentials set as auth type": {
			s3: cacheconfig.CacheS3Config{
				AccessKey:          "TOKEN",
				SecretKey:          "TOKEN",
				AuthenticationType: cacheconfig.S3AuthTypeAccessKey,
				BucketName:         "name",
				BucketLocation:     "us-east-1a",
			},
			authType: cacheconfig.S3AuthTypeAccessKey,
		},
		"Explicitly set but lowercase auth type": {
			s3: cacheconfig.CacheS3Config{
				AccessKey:          "TOKEN",
				SecretKey:          "TOKEN",
				AuthenticationType: "access-key",
				BucketName:         "name",
				BucketLocation:     "us-east-1a",
			},
			authType: cacheconfig.S3AuthTypeAccessKey,
		},
		"Explicitly set invalid auth type": {
			s3: cacheconfig.CacheS3Config{
				AccessKey:          "TOKEN",
				SecretKey:          "TOKEN",
				AuthenticationType: "invalid",
				BucketName:         "name",
				BucketLocation:     "us-east-1a",
			},
			authType: "",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.s3.AuthType(), tt.authType)
		})
	}
}

func TestCacheS3Config_DualStack(t *testing.T) {
	useDualStack := true
	disableDualStack := false

	tests := map[string]struct {
		s3       cacheconfig.CacheS3Config
		expected bool
	}{
		"Dual Stack omitted": {
			s3:       cacheconfig.CacheS3Config{},
			expected: true,
		},
		"Dual Stack set to true": {
			s3:       cacheconfig.CacheS3Config{DualStack: &useDualStack},
			expected: true,
		},
		"Dual Stack set to false": {
			s3:       cacheconfig.CacheS3Config{DualStack: &disableDualStack},
			expected: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.s3.DualStackEnabled())
		})
	}
}

func TestCacheS3Config_Encryption(t *testing.T) {
	testARN := "aws:arn:::1234"

	tests := map[string]struct {
		s3                     cacheconfig.CacheS3Config
		expectedEncryptionType cacheconfig.S3EncryptionType
		expectedKeyID          string
	}{
		"no encryption": {
			s3:                     cacheconfig.CacheS3Config{},
			expectedEncryptionType: cacheconfig.S3EncryptionTypeNone,
		},
		"S3 encryption": {
			s3:                     cacheconfig.CacheS3Config{ServerSideEncryption: "S3"},
			expectedEncryptionType: cacheconfig.S3EncryptionTypeAes256,
		},
		"unknown encryption": {
			s3:                     cacheconfig.CacheS3Config{ServerSideEncryption: "BLAH"},
			expectedEncryptionType: cacheconfig.S3EncryptionTypeNone,
		},
		"AES256 encryption": {
			s3:                     cacheconfig.CacheS3Config{ServerSideEncryption: "aes256"},
			expectedEncryptionType: cacheconfig.S3EncryptionTypeAes256,
		},
		"KMS encryption": {
			s3:                     cacheconfig.CacheS3Config{ServerSideEncryption: "kms", ServerSideEncryptionKeyID: testARN},
			expectedEncryptionType: cacheconfig.S3EncryptionTypeKms,
			expectedKeyID:          testARN,
		},
		"AWS:KMS encryption": {
			s3:                     cacheconfig.CacheS3Config{ServerSideEncryption: "aws:kms", ServerSideEncryptionKeyID: testARN},
			expectedEncryptionType: cacheconfig.S3EncryptionTypeKms,
			expectedKeyID:          testARN,
		},
		"DSSE-KMS encryption": {
			s3:                     cacheconfig.CacheS3Config{ServerSideEncryption: "DSSE-KMS", ServerSideEncryptionKeyID: testARN},
			expectedEncryptionType: cacheconfig.S3EncryptionTypeDsseKms,
			expectedKeyID:          testARN,
		},
		"aws:kms:dsse encryption": {
			s3:                     cacheconfig.CacheS3Config{ServerSideEncryption: "aws:kms:dsse", ServerSideEncryptionKeyID: testARN},
			expectedEncryptionType: cacheconfig.S3EncryptionTypeDsseKms,
			expectedKeyID:          testARN,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.expectedEncryptionType, tt.s3.EncryptionType())
			assert.Equal(t, tt.expectedKeyID, tt.s3.ServerSideEncryptionKeyID)
		})
	}
}

func TestCacheS3Config_Endpoint(t *testing.T) {
	disabled := false

	tests := map[string]struct {
		s3                cacheconfig.CacheS3Config
		expected          string
		expectedPathStyle bool
	}{
		"no server address": {
			s3:                cacheconfig.CacheS3Config{},
			expected:          "",
			expectedPathStyle: false,
		},
		"bad hostname": {
			s3:                cacheconfig.CacheS3Config{ServerAddress: "local\x00host:8080"},
			expected:          "",
			expectedPathStyle: false,
		},
		"HTTPS server address": {
			s3:                cacheconfig.CacheS3Config{ServerAddress: "minio.example.com:8080"},
			expected:          "https://minio.example.com:8080",
			expectedPathStyle: true,
		},
		"HTTP server address": {
			s3:                cacheconfig.CacheS3Config{ServerAddress: "minio.example.com:8080", Insecure: true},
			expected:          "http://minio.example.com:8080",
			expectedPathStyle: true,
		},
		"AWS us-east-2 endpoint": {
			s3:                cacheconfig.CacheS3Config{ServerAddress: "s3.us-east-2.amazonaws.com"},
			expected:          "https://s3.us-east-2.amazonaws.com",
			expectedPathStyle: false,
		},
		"AWS us-east-2 endpoint with bucket": {
			s3:                cacheconfig.CacheS3Config{ServerAddress: "my-bucket.s3.us-east-2.amazonaws.com", BucketName: "my-bucket", BucketLocation: "us-east-2"},
			expected:          "https://my-bucket.s3.us-east-2.amazonaws.com",
			expectedPathStyle: true,
		},
		"AWS FIPS endpoint": {
			s3:                cacheconfig.CacheS3Config{ServerAddress: "s3-fips.us-west-1.amazonaws.com"},
			expected:          "https://s3-fips.us-west-1.amazonaws.com",
			expectedPathStyle: false,
		},
		"Google endpoint": {
			s3:                cacheconfig.CacheS3Config{ServerAddress: "storage.googleapis.com"},
			expected:          "https://storage.googleapis.com",
			expectedPathStyle: false,
		},
		"Custom HTTPS server on standard port": {
			s3:                cacheconfig.CacheS3Config{ServerAddress: "minio.example.com:443", PathStyle: &disabled},
			expected:          "https://minio.example.com",
			expectedPathStyle: false,
		},
		"Custom HTTP server on standard port": {
			s3:                cacheconfig.CacheS3Config{ServerAddress: "minio.example.com:80", Insecure: true, PathStyle: &disabled},
			expected:          "http://minio.example.com",
			expectedPathStyle: false,
		},
		"Custom HTTPS server on HTTP port": {
			s3:                cacheconfig.CacheS3Config{ServerAddress: "minio.example.com:80", PathStyle: &disabled},
			expected:          "https://minio.example.com:80",
			expectedPathStyle: false,
		},
		"Custom HTTPS server with path style disabled": {
			s3:                cacheconfig.CacheS3Config{ServerAddress: "minio.example.com:8080", PathStyle: &disabled},
			expected:          "https://minio.example.com:8080",
			expectedPathStyle: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.expectedPathStyle, tt.s3.PathStyleEnabled())
			if tt.expected != "" {
				assert.Equal(t, tt.expected, tt.s3.GetEndpoint())
				assert.Equal(t, tt.expected, tt.s3.GetEndpointURL().String())
			} else {
				assert.Nil(t, tt.s3.GetEndpointURL())
			}
		})
	}
}
