//go:build !integration

package service

import (
	"context"
	"errors"
	"hash/crc32"
	"testing"

	smpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/stretchr/testify/assert"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"golang.org/x/oauth2"
	"google.golang.org/api/sts/v1"
)

func TestClient_GetSecret(t *testing.T) {
	secretName := "my-secret"
	secretVersion := "latest"
	projectNumber := "1234"
	workloadIdentityPoolId := "pool-id"
	workloadIdentityProviderID := "provider-id"
	jwtToken := "jwt token"

	secret := &common.GCPSecretManagerSecret{
		Name:    secretName,
		Version: secretVersion,
		Server: common.GCPSecretManagerServer{
			ProjectNumber:                        projectNumber,
			WorkloadIdentityFederationPoolId:     workloadIdentityPoolId,
			WorkloadIdentityFederationProviderID: workloadIdentityProviderID,
			JWT:                                  jwtToken,
		},
	}

	stubAccessToken := "access-token"
	stubTokenResponse := &sts.GoogleIdentityStsV1ExchangeTokenResponse{
		AccessToken: stubAccessToken,
		TokenType:   "Bearer",
	}

	stubData := []byte("my-secret-data")
	stubAccessSecretResponse := &smpb.AccessSecretVersionResponse{
		Name: secretName,
		Payload: &smpb.SecretPayload{
			Data:       stubData,
			DataCrc32C: calculateCrc32(stubData),
		},
	}

	tests := map[string]struct {
		secret             *common.GCPSecretManagerSecret
		verifyGetToken     func(c *Client) func(t *testing.T)
		verifyAccessSecret func(c *Client) func(t *testing.T)
		assertError        assert.ErrorAssertionFunc
		expectedResult     string
	}{
		"successful token exchange and accessing secret": {
			secret: secret,
			verifyGetToken: func(c *Client) func(t *testing.T) {
				callCount := 0
				c.getToken = func(ctx context.Context, secret *common.GCPSecretManagerSecret) (*sts.GoogleIdentityStsV1ExchangeTokenResponse, error) {
					callCount += 1
					return stubTokenResponse, nil
				}

				return func(t *testing.T) {
					assert.Equal(t, 1, callCount)
				}
			},
			verifyAccessSecret: func(c *Client) func(t *testing.T) {
				callCount := 0
				var accessToken string

				c.accessSecret = func(ctx context.Context, secret *common.GCPSecretManagerSecret, source oauth2.TokenSource) (*smpb.AccessSecretVersionResponse, error) {
					callCount += 1
					token, _ := source.Token()
					accessToken = token.AccessToken

					return stubAccessSecretResponse, nil
				}

				return func(t *testing.T) {
					assert.Equal(t, stubTokenResponse.AccessToken, accessToken)
					assert.Equal(t, 1, callCount)
				}
			},
			assertError:    assert.NoError,
			expectedResult: string(stubData),
		},
		"failed authentication": {
			secret: secret,
			verifyGetToken: func(c *Client) func(t *testing.T) {
				callCount := 0
				c.getToken = func(ctx context.Context, secret *common.GCPSecretManagerSecret) (*sts.GoogleIdentityStsV1ExchangeTokenResponse, error) {
					callCount += 1
					return nil, errors.New("failed getToken")
				}

				return func(t *testing.T) {
					assert.Equal(t, 1, callCount)
				}
			},
			verifyAccessSecret: func(c *Client) func(t *testing.T) {
				callCount := 0
				c.accessSecret = func(ctx context.Context, secret *common.GCPSecretManagerSecret, source oauth2.TokenSource) (*smpb.AccessSecretVersionResponse, error) {
					callCount += 1
					return stubAccessSecretResponse, nil
				}

				return func(t *testing.T) {
					assert.Equal(t, 0, callCount)
				}
			},
			assertError: func(t assert.TestingT, err error, msgAndArgs ...interface{}) bool {
				assert.ErrorContains(t, err, "failed getToken")
				return false
			},
		},
		"failed secret access": {
			secret: secret,
			verifyGetToken: func(c *Client) func(t *testing.T) {
				callCount := 0
				c.getToken = func(ctx context.Context, secret *common.GCPSecretManagerSecret) (*sts.GoogleIdentityStsV1ExchangeTokenResponse, error) {
					callCount += 1
					return stubTokenResponse, nil
				}

				return func(t *testing.T) {
					assert.Equal(t, 1, callCount)
				}
			},
			verifyAccessSecret: func(c *Client) func(t *testing.T) {
				callCount := 0
				c.accessSecret = func(ctx context.Context, secret *common.GCPSecretManagerSecret, source oauth2.TokenSource) (*smpb.AccessSecretVersionResponse, error) {
					callCount += 1
					return nil, errors.New("failed to get secret")
				}

				return func(t *testing.T) {
					assert.Equal(t, 1, callCount)
				}
			},
			assertError: func(t assert.TestingT, err error, msgAndArgs ...interface{}) bool {
				assert.ErrorContains(t, err, "failed to get secret")
				return false
			},
		},
		"corrupted data": {
			secret: secret,
			verifyGetToken: func(c *Client) func(t *testing.T) {
				callCount := 0
				c.getToken = func(ctx context.Context, secret *common.GCPSecretManagerSecret) (*sts.GoogleIdentityStsV1ExchangeTokenResponse, error) {
					callCount += 1
					return stubTokenResponse, nil
				}

				return func(t *testing.T) {
					assert.Equal(t, 1, callCount)
				}
			},
			verifyAccessSecret: func(c *Client) func(t *testing.T) {
				callCount := 0
				c.accessSecret = func(ctx context.Context, secret *common.GCPSecretManagerSecret, source oauth2.TokenSource) (*smpb.AccessSecretVersionResponse, error) {
					callCount += 1
					incorrectChecksum := int64(1234)

					stubAccessSecretResponse := &smpb.AccessSecretVersionResponse{
						Name: secretName,
						Payload: &smpb.SecretPayload{
							Data:       stubData,
							DataCrc32C: &incorrectChecksum,
						},
					}

					return stubAccessSecretResponse, nil
				}

				return func(t *testing.T) {
					assert.Equal(t, 1, callCount)
				}
			},
			assertError: func(t assert.TestingT, err error, msgAndArgs ...interface{}) bool {
				assert.ErrorContains(t, err, "data corruption detected")
				return false
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			ctx := context.Background()

			c := &Client{}
			defer tt.verifyGetToken(c)(t)
			defer tt.verifyAccessSecret(c)(t)

			result, err := c.GetSecret(ctx, tt.secret)
			tt.assertError(t, err)

			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func calculateCrc32(data []byte) *int64 {
	var checksum int64

	crc32c := crc32.MakeTable(crc32.Castagnoli)
	checksum = int64(crc32.Checksum(data, crc32c))

	return &checksum
}
