//go:build !integration

package s3

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestGetCredentials(t *testing.T) {
	tests := map[string]struct {
		s3            *common.CacheS3Config
		expectedError string
		credsExpected bool
	}{
		"static credentials": {
			s3: &common.CacheS3Config{
				BucketName: bucketName,
				AccessKey:  "somekey",
				SecretKey:  "somesecret",
			},
			credsExpected: true,
		},
		"no S3 credentials": {
			expectedError: `missing S3 configuration`,
		},
		"empty access and secret key": {
			s3: &common.CacheS3Config{
				BucketName: bucketName,
			},
			credsExpected: false,
		},
		"empty access key": {
			s3: &common.CacheS3Config{
				BucketName: bucketName,
				SecretKey:  "somesecret",
			},
			credsExpected: false,
		},
		"empty secret key": {
			s3: &common.CacheS3Config{
				BucketName: bucketName,
				AccessKey:  "somekey",
			},
			credsExpected: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			config := &common.CacheConfig{S3: tt.s3}
			adapter, err := NewS3CredentialsAdapter(config)

			if tt.expectedError != "" {
				require.EqualError(t, err, tt.expectedError)
			} else {
				require.NoError(t, err)

				creds := adapter.GetCredentials()

				if tt.credsExpected {
					assert.Equal(t, 2, len(creds))
					assert.Equal(t, tt.s3.AccessKey, creds["AWS_ACCESS_KEY_ID"])
					assert.Equal(t, tt.s3.SecretKey, creds["AWS_SECRET_ACCESS_KEY"])
				} else {
					assert.Empty(t, creds)
				}
			}
		})
	}
}
