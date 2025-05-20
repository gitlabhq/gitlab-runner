//go:build !integration

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	akeyless_api "github.com/akeylesslabs/akeyless-go/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func ptr[T any](t T) *T {
	return &t
}

func TestAkeyless(t *testing.T) {
	tests := map[string]struct {
		secret   common.AkeylessSecret
		itemType ItemType
		setup    func(secret common.AkeylessSecret, mock *mockAkeylessAPIClient)
		value    any
		err      error
	}{
		"no token auth called": {
			secret: common.AkeylessSecret{
				Server: common.AkeylessServer{
					AccessId:           "access_id",
					AccessKey:          "access_key",
					AkeylessAccessType: string(AccessTypeApiKey),
				},
			},
			setup: func(secret common.AkeylessSecret, m *mockAkeylessAPIClient) {
				m.On("Auth", mock.Anything, akeyless_api.Auth{
					AccessId:    ptr(secret.Server.AccessId),
					AccessKey:   ptr(secret.Server.AccessKey),
					AccessType:  ptr(string(AccessTypeApiKey)),
					GcpAudience: ptr("akeyless.io"),
					OciAuthType: ptr("apikey"),
					Json:        ptr(false),
				}).Once().Return(akeyless_api.AuthOutput{
					Token: ptr("token"),
				}, nil)
			},
			value: "token",
		},
		"token no auth called": {
			secret: common.AkeylessSecret{
				Server: common.AkeylessServer{
					AkeylessToken: "token",
				},
			},
			setup: func(secret common.AkeylessSecret, m *mockAkeylessAPIClient) {},
			value: "token",
		},
		"static secret": {
			secret: common.AkeylessSecret{
				Name: "name",
				Server: common.AkeylessServer{
					AkeylessToken: "token",
				},
			},
			itemType: ItemTypeStaticSecret,
			setup: func(secret common.AkeylessSecret, m *mockAkeylessAPIClient) {
				m.On("GetSecretValue", mock.Anything, akeyless_api.GetSecretValue{
					Names: []string{secret.Name},
					Token: ptr(secret.Server.AkeylessToken),
				}).Return(map[string]any{
					secret.Name: "value",
				}, nil).Once()
			},
			value: "value",
		},
		"static secret wrong name no value": {
			secret: common.AkeylessSecret{
				Name: "name",
				Server: common.AkeylessServer{
					AkeylessToken: "token",
				},
			},
			itemType: ItemTypeStaticSecret,
			setup: func(secret common.AkeylessSecret, m *mockAkeylessAPIClient) {
				m.On("GetSecretValue", mock.Anything, akeyless_api.GetSecretValue{
					Names: []string{secret.Name},
					Token: ptr(secret.Server.AkeylessToken),
				}).Return(map[string]any{
					"invalid": "value",
				}, nil).Once()
			},
			err: getSecretNotFoundError("name"),
		},
		"static secret err": {
			secret: common.AkeylessSecret{
				Name: "name",
				Server: common.AkeylessServer{
					AkeylessToken: "token",
				},
			},
			itemType: ItemTypeStaticSecret,
			setup: func(secret common.AkeylessSecret, m *mockAkeylessAPIClient) {
				m.On("GetSecretValue", mock.Anything, akeyless_api.GetSecretValue{
					Names: []string{secret.Name},
					Token: ptr(secret.Server.AkeylessToken),
				}).Return(nil, errors.New("err")).Once()
			},
			err: errors.New("err"),
		},
		"dynamic secret": {
			secret: common.AkeylessSecret{
				Name: "name",
				Server: common.AkeylessServer{
					AkeylessToken: "token",
				},
			},
			itemType: ItemTypeDynamicSecret,
			setup: func(secret common.AkeylessSecret, m *mockAkeylessAPIClient) {
				m.On("GetDynamicSecretValue", mock.Anything, akeyless_api.GetDynamicSecretValue{
					Name:  secret.Name,
					Token: ptr(secret.Server.AkeylessToken),
				}).Return(map[string]any{
					"key": "value",
				}, nil).Once()
			},
			value: map[string]any{
				"key": "value",
			},
		},
		"dynamic secret error": {
			secret: common.AkeylessSecret{
				Name: "name",
				Server: common.AkeylessServer{
					AkeylessToken: "token",
				},
			},
			itemType: ItemTypeDynamicSecret,
			setup: func(secret common.AkeylessSecret, m *mockAkeylessAPIClient) {
				m.On("GetDynamicSecretValue", mock.Anything, akeyless_api.GetDynamicSecretValue{
					Name:  secret.Name,
					Token: ptr(secret.Server.AkeylessToken),
				}).Return(nil, errors.New("err")).Once()
			},
			err: errors.New("err"),
		},
		"rotated secret": {
			secret: common.AkeylessSecret{
				Name: "name",
				Server: common.AkeylessServer{
					AkeylessToken: "token",
				},
			},
			itemType: ItemTypeRotatedSecret,
			setup: func(secret common.AkeylessSecret, m *mockAkeylessAPIClient) {
				m.On("GetRotatedSecretValue", mock.Anything, akeyless_api.GetRotatedSecretValue{
					Names: secret.Name,
					Token: ptr(secret.Server.AkeylessToken),
				}).Return(map[string]any{
					"value": "value",
				}, nil).Once()
			},
			value: "value",
		},
		"rotated secret wrong name no value": {
			secret: common.AkeylessSecret{
				Name: "name",
				Server: common.AkeylessServer{
					AkeylessToken: "token",
				},
			},
			itemType: ItemTypeRotatedSecret,
			setup: func(secret common.AkeylessSecret, m *mockAkeylessAPIClient) {
				m.On("GetRotatedSecretValue", mock.Anything, akeyless_api.GetRotatedSecretValue{
					Names: secret.Name,
					Token: ptr(secret.Server.AkeylessToken),
				}).Return(map[string]any{
					"invalid": "value",
				}, nil).Once()
			},
			err: getSecretNotFoundError("name"),
		},
		"rotated secret err": {
			secret: common.AkeylessSecret{
				Name: "name",
				Server: common.AkeylessServer{
					AkeylessToken: "token",
				},
			},
			itemType: ItemTypeRotatedSecret,
			setup: func(secret common.AkeylessSecret, m *mockAkeylessAPIClient) {
				m.On("GetRotatedSecretValue", mock.Anything, akeyless_api.GetRotatedSecretValue{
					Names: secret.Name,
					Token: ptr(secret.Server.AkeylessToken),
				}).Return(nil, errors.New("err")).Once()
			},
			err: errors.New("err"),
		},
		"ssh cert secret": {
			secret: common.AkeylessSecret{
				Name:          "name",
				CertUserName:  "username",
				PublicKeyData: "publickey",
				Server: common.AkeylessServer{
					AkeylessToken: "token",
				},
			},
			itemType: ItemTypeSSHCertIssuer,
			setup: func(secret common.AkeylessSecret, m *mockAkeylessAPIClient) {
				m.On("GetSSHCertificate", mock.Anything, akeyless_api.GetSSHCertificate{
					CertIssuerName: secret.Name,
					CertUsername:   secret.CertUserName,
					PublicKeyData:  ptr(secret.PublicKeyData),
					Token:          ptr(secret.Server.AkeylessToken),
				}).Return(akeyless_api.GetSSHCertificateOutput{
					Data: ptr("data"),
				}, nil).Once()
			},
			value: "data",
		},
		"ssh cert secret no data": {
			secret: common.AkeylessSecret{
				Name:          "name",
				CertUserName:  "username",
				PublicKeyData: "publickey",
				Server: common.AkeylessServer{
					AkeylessToken: "token",
				},
			},
			itemType: ItemTypeSSHCertIssuer,
			setup: func(secret common.AkeylessSecret, m *mockAkeylessAPIClient) {
				m.On("GetSSHCertificate", mock.Anything, akeyless_api.GetSSHCertificate{
					CertIssuerName: secret.Name,
					CertUsername:   secret.CertUserName,
					PublicKeyData:  ptr(secret.PublicKeyData),
					Token:          ptr(secret.Server.AkeylessToken),
				}).Return(akeyless_api.GetSSHCertificateOutput{}, nil).Once()
			},
			err: getSecretNotFoundError("name"),
		},
		"ssh cert secret err": {
			secret: common.AkeylessSecret{
				Name:          "name",
				CertUserName:  "username",
				PublicKeyData: "publickey",
				Server: common.AkeylessServer{
					AkeylessToken: "token",
				},
			},
			itemType: ItemTypeSSHCertIssuer,
			setup: func(secret common.AkeylessSecret, m *mockAkeylessAPIClient) {
				m.On("GetSSHCertificate", mock.Anything, akeyless_api.GetSSHCertificate{
					CertIssuerName: secret.Name,
					CertUsername:   secret.CertUserName,
					PublicKeyData:  ptr(secret.PublicKeyData),
					Token:          ptr(secret.Server.AkeylessToken),
				}).Return(akeyless_api.GetSSHCertificateOutput{}, errors.New("err")).Once()
			},
			err: errors.New("err"),
		},
		"pki cert secret": {
			secret: common.AkeylessSecret{
				Name:          "name",
				CsrData:       "csr",
				PublicKeyData: "publickey",
				Server: common.AkeylessServer{
					AkeylessToken: "token",
				},
			},
			itemType: ItemTypePkiCertIssuer,
			setup: func(secret common.AkeylessSecret, m *mockAkeylessAPIClient) {
				m.On("GetPKICertificate", mock.Anything, akeyless_api.GetPKICertificate{
					CertIssuerName: secret.Name,
					CsrDataBase64:  ptr(secret.CsrData),
					KeyDataBase64:  ptr(secret.PublicKeyData),
					Token:          ptr(secret.Server.AkeylessToken),
				}).Return(akeyless_api.GetPKICertificateOutput{
					Data: ptr("data"),
				}, nil).Once()
			},
			value: "data",
		},
		"pki cert secret no data": {
			secret: common.AkeylessSecret{
				Name:          "name",
				CsrData:       "csr",
				PublicKeyData: "publickey",
				Server: common.AkeylessServer{
					AkeylessToken: "token",
				},
			},
			itemType: ItemTypePkiCertIssuer,
			setup: func(secret common.AkeylessSecret, m *mockAkeylessAPIClient) {
				m.On("GetPKICertificate", mock.Anything, akeyless_api.GetPKICertificate{
					CertIssuerName: secret.Name,
					CsrDataBase64:  ptr(secret.CsrData),
					KeyDataBase64:  ptr(secret.PublicKeyData),
					Token:          ptr(secret.Server.AkeylessToken),
				}).Return(akeyless_api.GetPKICertificateOutput{}, nil).Once()
			},
			err: getSecretNotFoundError("name"),
		},
		"pki cert secret err": {
			secret: common.AkeylessSecret{
				Name:          "name",
				CsrData:       "csr",
				PublicKeyData: "publickey",
				Server: common.AkeylessServer{
					AkeylessToken: "token",
				},
			},
			itemType: ItemTypePkiCertIssuer,
			setup: func(secret common.AkeylessSecret, m *mockAkeylessAPIClient) {
				m.On("GetPKICertificate", mock.Anything, akeyless_api.GetPKICertificate{
					CertIssuerName: secret.Name,
					CsrDataBase64:  ptr(secret.CsrData),
					KeyDataBase64:  ptr(secret.PublicKeyData),
					Token:          ptr(secret.Server.AkeylessToken),
				}).Return(akeyless_api.GetPKICertificateOutput{}, errors.New("err")).Once()
			},
			err: errors.New("err"),
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			m := newMockAkeylessAPIClient(t)

			if tt.itemType != "" {
				m.
					On("DescribeItem", mock.Anything, akeyless_api.DescribeItem{
						Name:  tt.secret.Name,
						Token: ptr(tt.secret.Server.AkeylessToken),
					}).
					Return(akeyless_api.Item{ItemType: ptr(string(tt.itemType))}, nil).
					Once()
			}

			tt.setup(tt.secret, m)
			akeyless := NewAkeyless(&tt.secret)
			akeyless.client = m

			value, err := akeyless.GetSecret(ctx)
			assert.Equal(t, tt.err, err)
			assert.Equal(t, tt.value, value)
		})
	}
}
