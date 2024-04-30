//go:build !integration

package gcsv2

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

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
)

func TestNew(t *testing.T) {
	t.Run("no config", func(t *testing.T) {
		adapter, err := New(&common.CacheConfig{}, time.Second, "bucket")
		require.ErrorContains(t, err, "missing GCS configuration")
		require.Nil(t, adapter)
	})

	t.Run("valid", func(t *testing.T) {
		adapter, err := New(&common.CacheConfig{GCS: &common.CacheGCSConfig{}}, time.Second, "bucket")
		require.NoError(t, err)
		require.NotNil(t, adapter)
	})
}

func TestAdapter(t *testing.T) {
	tests := map[string]struct {
		config         *common.CacheConfig
		timeout        time.Duration
		objectName     string
		newExpectedErr string
		getExpectedErr string
		putExpectedErr string
	}{
		"missing config": {
			config:         &common.CacheConfig{},
			objectName:     "object-key",
			newExpectedErr: "missing GCS configuration",
		},
		"no bucket name": {
			config:         &common.CacheConfig{GCS: &common.CacheGCSConfig{}},
			objectName:     "object-key",
			getExpectedErr: "config BucketName cannot be empty",
			putExpectedErr: "config BucketName cannot be empty",
		},
		"valid": {
			config:     &common.CacheConfig{GCS: &common.CacheGCSConfig{BucketName: "test", CacheGCSCredentials: common.CacheGCSCredentials{AccessID: accessID, PrivateKey: privateKey}}},
			objectName: "object-key",
		},
		"valid with max upload size": {
			config:     &common.CacheConfig{MaxUploadedArchiveSize: 100, GCS: &common.CacheGCSConfig{BucketName: "test", CacheGCSCredentials: common.CacheGCSCredentials{AccessID: accessID, PrivateKey: privateKey}}},
			objectName: "object-key",
		},
	}

	const expectedURL = "https://storage.googleapis.com/test/object-key"

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			adapter, err := New(tc.config, tc.timeout, tc.objectName)
			if tc.newExpectedErr != "" {
				require.EqualError(t, err, tc.newExpectedErr)
				require.Nil(t, adapter)
				return
			} else {
				require.NoError(t, err)
				require.NotNil(t, adapter)
			}

			getURL, err := adapter.(*gcsAdapter).presignURL(context.Background(), http.MethodGet, "")
			if tc.getExpectedErr != "" {
				assert.EqualError(t, err, tc.getExpectedErr)
			} else {
				assert.NoError(t, err)
			}

			putURL, err := adapter.(*gcsAdapter).presignURL(context.Background(), http.MethodPut, "application/octet-stream")
			if tc.putExpectedErr != "" {
				assert.EqualError(t, err, tc.putExpectedErr)
			} else {
				assert.NoError(t, err)
			}

			if getURL != nil {
				assert.Contains(t, getURL.String(), expectedURL)

				u := adapter.GetDownloadURL(context.Background())
				require.NotNil(t, u)
				assert.Contains(t, u.String(), expectedURL)
			}

			if putURL != nil {
				assert.Contains(t, putURL.String(), expectedURL)

				u := adapter.GetUploadURL(context.Background())
				require.NotNil(t, u)
				assert.Contains(t, u.String(), expectedURL)
			}

			if tc.config.MaxUploadedArchiveSize != 0 {
				assert.Equal(t, adapter.GetUploadHeaders(), http.Header{"X-Goog-Content-Length-Range": []string{fmt.Sprintf("0,%d", tc.config.MaxUploadedArchiveSize)}})
			} else {
				assert.Nil(t, adapter.GetUploadHeaders())
			}
		})
	}
}
