//go:build !integration

package gcs

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var (
	accessID   = "test-access-id@X.iam.gserviceaccount.com"
	privateKey = `-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEAzIrvApxNX3VxH5eYe4vI2kLTqOA9uFTV4clGy8uzQsGQvMjl
frTWCffayxaSvoKxPlvUYbecYpqqqaByLTE+kSDU/D44yrCiLAyWHWXYGZqfEMEG
uHBg4fJK6KcIXlJ3Hp3EGTPw92sCKKzLXyoY7mNN9iP8mnshc39wjdrqm2YgKvQU
ZWDxIL/MTtLcWyK07zJ2RamilcjpKtQL5GFgvHCsV1CvQHuKtmZF5kfHlD2E/e+I
uEg+fntGkKJpDYtSn1fbLcg/ctFJKQBLfAaJ59Hgyewd8fKveJ6Vn1C7gCXagMPb
q54RS8J0dolPaxUtRbzGMJ5Amag8m3dm6U3FbwIDAQABAoIBAQCxC+U8Vjymzwoe
9WIYNnOhcMyy1X63Cj+j00wDZQuCUffNYPs8xJysPizVM3HLk2aF+oiIGJ01wHjO
oMGTmpd0mX2h5N3VnDSTekWJprj52Jusrdf6V9OUX9w1KzeUJT9Ucezmf84o6ygQ
OxlCAzdXSP+XeajRspjO11V+hCokXSICAMMnUYyqT+Yr34YldjpVJ3VWFHipByww
1BCHBveJuH4wgVW4QICDKBzzYyFCqi8kFFv8ijQ9QOAD2xkVYiP8sOR1K6h/FuHN
KV+axHtQjkYgOlyYN7/oe9L0XroCa4h7XibcWLuLQ56G3oBzTFur0la3A1SuKLGm
LwBfeVpxAoGBAPCKUiqan24h8RgscEXtbACVa3WmEmOe4qqjnEChof8U5xP4YdfZ
cg+k7eBqXBgVtmxozJOQxcPwkZrHIRP59d2h8vjcjOBrMeI3D9BCjTKGYySv0iRT
FI0akA0c0Ec7utN4t7AfY7sUpx+wvX/klYy5bsIzOceU/9rYYoudXLnZAoGBANmw
VWykOgJZLv8aSTLCDEl2WV6nsl1jRYONVzlthcgQ1wpdgAJvLoTJMuXuSzOQQbUa
08Zm2LhbDErX7YA8MslaiQERSfedV/EXjZn86CBw6wB4IPv8uWh9zSK7E4IH4Den
Ow2RE5XjEDiyMA2PUCAGqVEmF/V4nRCFvEfS52SHAoGBAI56MA9CRTsz6Z3a/Km+
5yE1YFBwjSXq//H5NV1nIBB6riE7F6GGEDTKCYjLFz/A5Kw0KzEhKLNV9LkMSECP
551fBw93fA6WEBchbEF8miwaQ/GAH2Yau+qUmEzcC1aWP6RxNcSh4y32HsP7qVNu
71JKqBtpwkjArghP8ZcnH7yJAoGBAJnHDxFoEfKGvcRH9V195uAeUpOjM0T1U63S
ssNGszLZco9H7Z3KnLoAx4vWAhmy1jfxc5i8HmxdJRnZ31SvMdE7u3ydkfrxk6Yk
VUtqdTA1lE0Ij4Ryyycdd0QJk4ZPufyWjgjPa15+wH7MoVVy388/5WwF1Pb69Tku
wAqc2gkRAoGAcj8a+peaNKa1d5EPE0CtTBUypupZh/R1ewTC9y7OyBPczYhxN5NQ
vvm6J1WGbnxmuhzzvGNNExeZx9dfGLmcvSAvrweiFbi2yHAc1cBLBkc5/CqfS6QW
336Qe2lgsM61/jrYYYqu7W8l6W2juCz0SPqml6rugsP8r6IMJxfziO8=
-----END RSA PRIVATE KEY-----`

	bucketName             = "test"
	objectName             = "key"
	defaultTimeout         = 1 * time.Hour
	maxUploadedArchiveSize = int64(100)
)

func defaultGCSCache() *common.CacheConfig {
	return &common.CacheConfig{
		Type: "gcs",
		GCS: &common.CacheGCSConfig{
			BucketName: bucketName,
		},
	}
}

type adapterOperationInvalidConfigTestCase struct {
	noGCSConfig bool

	errorOnCredentialsResolverInitialization bool
	credentialsResolverResolveError          bool

	accessID      string
	privateKey    string
	bucketName    string
	expectedError string
}

func prepareMockedCredentialsResolverInitializer(tc adapterOperationInvalidConfigTestCase) func() {
	oldCredentialsResolverInitializer := credentialsResolverInitializer
	credentialsResolverInitializer = func(config *common.CacheGCSConfig) (*defaultCredentialsResolver, error) {
		if tc.errorOnCredentialsResolverInitialization {
			return nil, errors.New("test error")
		}

		return newDefaultCredentialsResolver(config)
	}

	return func() {
		credentialsResolverInitializer = oldCredentialsResolverInitializer
	}
}

func prepareMockedCredentialsResolverForInvalidConfig(adapter *gcsAdapter, tc adapterOperationInvalidConfigTestCase) {
	cr := &mockCredentialsResolver{}

	resolveCall := cr.On("Resolve")
	if tc.credentialsResolverResolveError {
		resolveCall.Return(fmt.Errorf("test error"))
	} else {
		resolveCall.Return(nil)
	}

	cr.On("Credentials").Return(&common.CacheGCSCredentials{
		AccessID:   tc.accessID,
		PrivateKey: tc.privateKey,
	})

	cr.On("SignBytesFunc").Return(func(payload []byte) ([]byte, error) {
		return []byte("output"), nil
	})

	adapter.credentialsResolver = cr
}

func testAdapterOperationWithInvalidConfig(
	t *testing.T,
	name string,
	tc adapterOperationInvalidConfigTestCase,
	adapter *gcsAdapter,
	operation func() *url.URL,
) {
	t.Run(name, func(t *testing.T) {
		prepareMockedCredentialsResolverForInvalidConfig(adapter, tc)
		hook := test.NewGlobal()

		u := operation()
		assert.Nil(t, u)

		message, err := hook.LastEntry().String()
		require.NoError(t, err)
		assert.Contains(t, message, tc.expectedError)
	})
}

func TestAdapterOperation_InvalidConfig(t *testing.T) {
	tests := map[string]adapterOperationInvalidConfigTestCase{
		"no-gcs-config": {
			noGCSConfig:   true,
			bucketName:    bucketName,
			expectedError: "Missing GCS configuration",
		},
		"error-on-credentials-resolver-initialization": {
			errorOnCredentialsResolverInitialization: true,
		},
		"credentials-resolver-resolve-error": {
			credentialsResolverResolveError: true,
			bucketName:                      bucketName,
			expectedError:                   "error while resolving GCS credentials: test error",
		},
		"no-credentials": {
			bucketName:    bucketName,
			expectedError: "storage: missing required GoogleAccessID",
		},
		"no-access-id": {
			privateKey:    privateKey,
			bucketName:    bucketName,
			expectedError: "storage: missing required GoogleAccessID",
		},
		"bucket-not-specified": {
			accessID:      "access-id",
			privateKey:    privateKey,
			expectedError: "BucketName can't be empty",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cleanupCredentialsResolverInitializerMock := prepareMockedCredentialsResolverInitializer(tc)
			defer cleanupCredentialsResolverInitializerMock()

			config := defaultGCSCache()
			if tc.noGCSConfig {
				config.GCS = nil
			} else {
				config.GCS.BucketName = tc.bucketName
			}

			a, err := New(config, defaultTimeout, objectName)
			if tc.noGCSConfig {
				assert.Nil(t, a)
				assert.EqualError(t, err, "missing GCS configuration")
				return
			}

			if tc.errorOnCredentialsResolverInitialization {
				assert.Nil(t, a)
				assert.EqualError(t, err, "error while initializing GCS credentials resolver: test error")
				return
			}

			require.NotNil(t, a)
			require.NoError(t, err)

			adapter, ok := a.(*gcsAdapter)
			require.True(t, ok, "Adapter should be properly casted to *adapter type")

			testAdapterOperationWithInvalidConfig(t, "GetDownloadURL", tc, adapter, a.GetDownloadURL)
			testAdapterOperationWithInvalidConfig(t, "GetUploadURL", tc, adapter, a.GetUploadURL)
		})
	}
}

type adapterOperationTestCase struct {
	returnedURL            string
	returnedError          error
	assertErrorMessage     func(t *testing.T, message string)
	signBlobAPITest        bool
	maxUploadedArchiveSize int64
	expectedHeaders        http.Header
}

func mockSignBytesFunc() func([]byte) ([]byte, error) {
	return func(payload []byte) ([]byte, error) {
		return []byte("output"), nil
	}
}

func prepareMockedCredentialsResolver(adapter *gcsAdapter, tc adapterOperationTestCase) func(t *testing.T) {
	cr := &mockCredentialsResolver{}
	cr.On("Resolve").Return(nil)

	pk := privateKey
	if tc.signBlobAPITest {
		pk = ""
		cr.On("SignBytesFunc").Return(mockSignBytesFunc)
	}
	cr.On("Credentials").Return(&common.CacheGCSCredentials{
		AccessID:   accessID,
		PrivateKey: pk,
	})

	adapter.credentialsResolver = cr

	return func(t *testing.T) {
		cr.AssertExpectations(t)
	}
}

func prepareMockedSignedURLGenerator(
	t *testing.T,
	tc adapterOperationTestCase,
	expectedMethod string,
	expectedContentType string,
	adapter *gcsAdapter,
) {
	adapter.generateSignedURL = func(bucket string, name string, opts *storage.SignedURLOptions) (string, error) {
		require.Equal(t, accessID, opts.GoogleAccessID)
		if tc.signBlobAPITest {
			require.NotNil(t, opts.SignBytes)
			require.Nil(t, opts.PrivateKey)
		} else {
			require.Equal(t, privateKey, string(opts.PrivateKey))
			require.Nil(t, opts.SignBytes)
		}
		require.Equal(t, expectedMethod, opts.Method)
		require.Equal(t, expectedContentType, opts.ContentType)

		return tc.returnedURL, tc.returnedError
	}
}

func testAdapterOperation(
	t *testing.T,
	tc adapterOperationTestCase,
	name string,
	expectedMethod string,
	expectedContentType string,
	adapter *gcsAdapter,
	operation func() *url.URL,
) {
	t.Run(name, func(t *testing.T) {
		cleanupCredentialsResolverMock := prepareMockedCredentialsResolver(adapter, tc)
		defer cleanupCredentialsResolverMock(t)

		prepareMockedSignedURLGenerator(t, tc, expectedMethod, expectedContentType, adapter)
		hook := test.NewGlobal()

		u := operation()

		if tc.assertErrorMessage != nil {
			message, err := hook.LastEntry().String()
			require.NoError(t, err)
			tc.assertErrorMessage(t, message)
			return
		}

		require.Len(t, hook.AllEntries(), 0)

		assert.Equal(t, tc.returnedURL, u.String())
	})
}

func TestAdapterOperation(t *testing.T) {
	//nolint:lll
	tests := map[string]adapterOperationTestCase{
		"error-on-URL-signing": {
			returnedURL:   "",
			returnedError: fmt.Errorf("test error"),
			assertErrorMessage: func(t *testing.T, message string) {
				assert.Contains(t, message, "error while generating GCS pre-signed URL: test error")
			},
			signBlobAPITest: false,
		},
		"invalid-URL-returned": {
			returnedURL:   "://test",
			returnedError: nil,
			assertErrorMessage: func(t *testing.T, message string) {
				assert.Contains(t, message, "error while parsing generated URL: parse")
				assert.Contains(t, message, "://test")
				assert.Contains(t, message, "missing protocol scheme")
			},
			signBlobAPITest: false,
		},
		"valid-configuration": {
			returnedURL:        "https://storage.googleapis.com/test/key?Expires=123456789&GoogleAccessId=test-access-id%40X.iam.gserviceaccount.com&Signature=XYZ",
			returnedError:      nil,
			assertErrorMessage: nil,
			signBlobAPITest:    false,
		},
		"sign-blob-api-valid-configuration": {
			returnedURL:        "https://storage.googleapis.com/test/key?Expires=123456789&GoogleAccessId=test-access-id%40X.iam.gserviceaccount.com&Signature=XYZ",
			returnedError:      nil,
			assertErrorMessage: nil,
			signBlobAPITest:    true,
		},
		"max-cache-archive-size": {
			returnedURL:            "https://storage.googleapis.com/test/key?Expires=123456789&GoogleAccessId=test-access-id%40X.iam.gserviceaccount.com&Signature=XYZ",
			returnedError:          nil,
			assertErrorMessage:     nil,
			signBlobAPITest:        false,
			maxUploadedArchiveSize: maxUploadedArchiveSize,
			expectedHeaders:        http.Header{"X-Goog-Content-Length-Range": []string{"0,100"}},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			config := defaultGCSCache()

			config.MaxUploadedArchiveSize = tc.maxUploadedArchiveSize

			a, err := New(config, defaultTimeout, objectName)
			require.NoError(t, err)

			adapter, ok := a.(*gcsAdapter)
			require.True(t, ok, "Adapter should be properly casted to *adapter type")

			testAdapterOperation(
				t,
				tc,
				"GetDownloadURL",
				http.MethodGet,
				"",
				adapter,
				a.GetDownloadURL,
			)
			testAdapterOperation(
				t,
				tc,
				"GetUploadURL",
				http.MethodPut,
				"application/octet-stream",
				adapter,
				a.GetUploadURL,
			)

			headers := adapter.GetUploadHeaders()
			assert.Equal(t, headers, tc.expectedHeaders)

			assert.Nil(t, adapter.GetGoCloudURL())
			assert.Empty(t, adapter.GetUploadEnv())
		})
	}
}
