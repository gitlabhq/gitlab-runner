//go:build !integration

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	smpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/stretchr/testify/assert"
	"golang.org/x/oauth2"
	"google.golang.org/api/sts/v1"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

func TestClient_GetSecret(t *testing.T) {
	secretName := "my-secret"
	secretVersion := "latest"
	projectNumber := "1234"
	workloadIdentityPoolId := "pool-id"
	workloadIdentityProviderID := "provider-id"
	jwtToken := "jwt token"

	secret := &spec.GCPSecretManagerSecret{
		Name:    secretName,
		Version: secretVersion,
		Server: spec.GCPSecretManagerServer{
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
			DataCrc32C: calculateCrc32C(stubData),
		},
	}

	tests := map[string]struct {
		secret             *spec.GCPSecretManagerSecret
		verifyGetToken     func(c *Client) func(t *testing.T)
		verifyAccessSecret func(c *Client) func(t *testing.T)
		assertError        assert.ErrorAssertionFunc
		expectedResult     string
	}{
		"successful token exchange and accessing secret": {
			secret: secret,
			verifyGetToken: func(c *Client) func(t *testing.T) {
				callCount := 0
				c.getToken = func(ctx context.Context, secret *spec.GCPSecretManagerSecret) (*sts.GoogleIdentityStsV1ExchangeTokenResponse, error) {
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

				c.accessSecret = func(ctx context.Context, secret *spec.GCPSecretManagerSecret, source oauth2.TokenSource) (*smpb.AccessSecretVersionResponse, error) {
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
				c.getToken = func(ctx context.Context, secret *spec.GCPSecretManagerSecret) (*sts.GoogleIdentityStsV1ExchangeTokenResponse, error) {
					callCount += 1
					return nil, errors.New("failed getToken")
				}

				return func(t *testing.T) {
					assert.Equal(t, 1, callCount)
				}
			},
			verifyAccessSecret: func(c *Client) func(t *testing.T) {
				callCount := 0
				c.accessSecret = func(ctx context.Context, secret *spec.GCPSecretManagerSecret, source oauth2.TokenSource) (*smpb.AccessSecretVersionResponse, error) {
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
				c.getToken = func(ctx context.Context, secret *spec.GCPSecretManagerSecret) (*sts.GoogleIdentityStsV1ExchangeTokenResponse, error) {
					callCount += 1
					return stubTokenResponse, nil
				}

				return func(t *testing.T) {
					assert.Equal(t, 1, callCount)
				}
			},
			verifyAccessSecret: func(c *Client) func(t *testing.T) {
				callCount := 0
				c.accessSecret = func(ctx context.Context, secret *spec.GCPSecretManagerSecret, source oauth2.TokenSource) (*smpb.AccessSecretVersionResponse, error) {
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
				c.getToken = func(ctx context.Context, secret *spec.GCPSecretManagerSecret) (*sts.GoogleIdentityStsV1ExchangeTokenResponse, error) {
					callCount += 1
					return stubTokenResponse, nil
				}

				return func(t *testing.T) {
					assert.Equal(t, 1, callCount)
				}
			},
			verifyAccessSecret: func(c *Client) func(t *testing.T) {
				callCount := 0
				c.accessSecret = func(ctx context.Context, secret *spec.GCPSecretManagerSecret, source oauth2.TokenSource) (*smpb.AccessSecretVersionResponse, error) {
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
			ctx := t.Context()

			c := &Client{}
			defer tt.verifyGetToken(c)(t)
			defer tt.verifyAccessSecret(c)(t)

			result, err := c.GetSecret(ctx, tt.secret)
			tt.assertError(t, err)

			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestSecretVersionResourceName(t *testing.T) {
	wifPoolProjectNumber := "1234"
	otherProjectNumber := "9876"
	baseSecretName := "my-secret"
	secretVersion := "345"

	tests := map[string]struct {
		secretName           string
		expectedResourceName string
	}{
		"bare secret name using implicit project number": {
			secretName:           baseSecretName,
			expectedResourceName: fmt.Sprintf("projects/%s/secrets/%s/versions/%s", wifPoolProjectNumber, baseSecretName, secretVersion),
		},
		"full secret resource name using implicit project number": {
			secretName:           fmt.Sprintf("projects/%s/secrets/%s", wifPoolProjectNumber, baseSecretName),
			expectedResourceName: fmt.Sprintf("projects/%s/secrets/%s/versions/%s", wifPoolProjectNumber, baseSecretName, secretVersion),
		},
		"full secret resource name from another project": {
			secretName:           fmt.Sprintf("projects/%s/secrets/%s", otherProjectNumber, baseSecretName),
			expectedResourceName: fmt.Sprintf("projects/%s/secrets/%s/versions/%s", otherProjectNumber, baseSecretName, secretVersion),
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			secret := &spec.GCPSecretManagerSecret{
				Name:    tt.secretName,
				Version: secretVersion,
				Server: spec.GCPSecretManagerServer{
					ProjectNumber:                        wifPoolProjectNumber,
					WorkloadIdentityFederationPoolId:     "pool-id",
					WorkloadIdentityFederationProviderID: "provider-id",
					JWT:                                  "jwt token",
				},
			}
			assert.Equal(t, tt.expectedResourceName, secretVersionResourceName(secret))
		})
	}
}
