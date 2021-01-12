package s3

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/cache"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type s3CredentialsAdapter struct {
	config *common.CacheS3Config
}

func (a *s3CredentialsAdapter) GetCredentials() map[string]string {
	credMap := make(map[string]string)

	// For IAM instance profiles, Go Cloud will fetch the credentials with the AWS SDK.
	if a.config.AccessKey == "" || a.config.SecretKey == "" {
		return credMap
	}

	credMap["AWS_ACCESS_KEY_ID"] = a.config.AccessKey
	credMap["AWS_SECRET_ACCESS_KEY"] = a.config.SecretKey

	return credMap
}

func NewS3CredentialsAdapter(config *common.CacheConfig) (cache.CredentialsAdapter, error) {
	s3 := config.S3
	if s3 == nil {
		return nil, fmt.Errorf("missing S3 configuration")
	}

	a := &s3CredentialsAdapter{
		config: s3,
	}

	return a, nil
}

func init() {
	err := cache.CredentialsFactories().Register("s3", NewS3CredentialsAdapter)
	if err != nil {
		panic(err)
	}
}
