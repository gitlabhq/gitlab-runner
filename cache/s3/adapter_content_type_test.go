//go:build !integration

package s3

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
)

// Drives the real minio-go presigner (the other tests in this package mock it) and checks
// that every header handed to the cache archiver is inside the signature, Content-Type
// included. SigV4-strict servers (for example Ceph RGW >= 19.2.6 / 20.2.4) reject the PUT
// otherwise. See https://gitlab.com/gitlab-org/gitlab-runner/-/work_items/39701.
func TestGetUploadURLSignsEverySentHeader(t *testing.T) {
	tests := map[string]struct {
		config   func() *cacheconfig.Config
		metadata map[string]string
	}{
		"no metadata":           {config: defaultCacheFactory},
		"with metadata":         {config: defaultCacheFactory, metadata: map[string]string{"cachekey": "demo"}},
		"sse-s3 with metadata":  {config: defaultCacheFactoryEncryptionAES, metadata: map[string]string{"cachekey": "demo"}},
		"sse-kms with metadata": {config: defaultCacheFactoryEncryptionKMS, metadata: map[string]string{"cachekey": "demo"}},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			adapter, err := New(tc.config(), defaultTimeout, objectName)
			require.NoError(t, err)
			adapter.WithMetadata(tc.metadata)

			u := adapter.GetUploadURL(t.Context())
			require.NotNil(t, u.URL, "presigned URL")
			assert.Equal(t, "application/octet-stream", u.Headers.Get("Content-Type"))

			signed := strings.Split(u.URL.Query().Get("X-Amz-SignedHeaders"), ";")
			assert.Contains(t, signed, "content-type")
			assert.Contains(t, signed, "host")
			for name := range u.Headers {
				assert.Contains(t, signed, strings.ToLower(name),
					"header %q is sent with the upload, so it must be signed", name)
			}
		})
	}
}

// Download and HEAD requests carry no body and no Content-Type.
func TestGetDownloadAndHeadURLDoNotSignContentType(t *testing.T) {
	adapter, err := New(defaultCacheFactory(), defaultTimeout, objectName)
	require.NoError(t, err)

	for name, get := range map[string]func(t *testing.T) (string, http.Header){
		"download": func(t *testing.T) (string, http.Header) {
			p := adapter.GetDownloadURL(t.Context())
			require.NotNil(t, p.URL)
			return p.URL.Query().Get("X-Amz-SignedHeaders"), p.Headers
		},
		"head": func(t *testing.T) (string, http.Header) {
			p := adapter.GetHeadURL(t.Context())
			require.NotNil(t, p.URL)
			return p.URL.Query().Get("X-Amz-SignedHeaders"), p.Headers
		},
	} {
		t.Run(name, func(t *testing.T) {
			signed, headers := get(t)
			assert.Equal(t, "host", signed)
			assert.Empty(t, headers.Get("Content-Type"))
		})
	}
}
