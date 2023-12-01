//go:build !integration

package service

import (
	"context"
	"errors"
	"hash/crc32"
	"testing"

	smpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
		secret           *common.GCPSecretManagerSecret
		setupAuthMock    func(a *mockAuthenticator, ctx context.Context, secret *common.GCPSecretManagerSecret)
		setupServiceMock func(s *mockService, ctx context.Context, secret *common.GCPSecretManagerSecret)
		assertError      assert.ErrorAssertionFunc
		expectedResult   string
	}{
		"successful token exchange and accessing secret": {
			secret: secret,
			setupAuthMock: func(a *mockAuthenticator, ctx context.Context, secret *common.GCPSecretManagerSecret) {
				a.On("getToken", ctx, secret).Return(stubTokenResponse, nil).Once()
			},
			setupServiceMock: func(s *mockService, ctx context.Context, secret *common.GCPSecretManagerSecret) {
				expectedTokenSource := oauth2.StaticTokenSource(&oauth2.Token{
					AccessToken: stubTokenResponse.AccessToken,
					TokenType:   stubTokenResponse.TokenType,
				})
				s.On("access", ctx, secret, expectedTokenSource).Return(stubAccessSecretResponse, nil).Once()
			},
			assertError:    assert.NoError,
			expectedResult: string(stubData),
		},
		"failed authentication": {
			secret: secret,
			setupAuthMock: func(a *mockAuthenticator, ctx context.Context, secret *common.GCPSecretManagerSecret) {
				a.On("getToken", ctx, secret).Return(nil, errors.New("failed auth")).Once()
			},
			setupServiceMock: func(s *mockService, ctx context.Context, secret *common.GCPSecretManagerSecret) {
				s.AssertNotCalled(t, "access")
			},
			assertError: func(t assert.TestingT, err error, msgAndArgs ...interface{}) bool {
				assert.ErrorContains(t, err, "failed auth")
				return false
			},
		},
		"failed secret access": {
			secret: secret,
			setupAuthMock: func(a *mockAuthenticator, ctx context.Context, secret *common.GCPSecretManagerSecret) {
				a.On("getToken", ctx, secret).Return(stubTokenResponse, nil).Once()
			},
			setupServiceMock: func(s *mockService, ctx context.Context, secret *common.GCPSecretManagerSecret) {
				s.On("access", ctx, secret, mock.Anything).Return(nil, errors.New("failed to get secret")).Once()
			},
			assertError: func(t assert.TestingT, err error, msgAndArgs ...interface{}) bool {
				assert.ErrorContains(t, err, "failed to get secret")
				return false
			},
		},
		"corrupted data": {
			secret: secret,
			setupAuthMock: func(a *mockAuthenticator, ctx context.Context, secret *common.GCPSecretManagerSecret) {
				a.On("getToken", ctx, secret).Return(stubTokenResponse, nil).Once()
			},
			setupServiceMock: func(s *mockService, ctx context.Context, secret *common.GCPSecretManagerSecret) {
				incorrectChecksum := int64(1234)

				stubAccessSecretResponse := &smpb.AccessSecretVersionResponse{
					Name: secretName,
					Payload: &smpb.SecretPayload{
						Data:       stubData,
						DataCrc32C: &incorrectChecksum,
					},
				}
				s.On("access", ctx, secret, mock.Anything).Return(stubAccessSecretResponse, nil).Once()
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

			authMock := new(mockAuthenticator)
			tt.setupAuthMock(authMock, ctx, tt.secret)
			defer authMock.AssertExpectations(t)

			serviceMock := new(mockService)
			tt.setupServiceMock(serviceMock, ctx, tt.secret)
			defer serviceMock.AssertExpectations(t)

			c := Client{
				auth: authMock,
				svc:  serviceMock,
			}

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
