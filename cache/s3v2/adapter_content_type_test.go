//go:build !integration

package s3v2

import (
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The presigned upload URL must sign every header the cache archiver will send with it.
// SigV4-strict servers (for example Ceph RGW >= 19.2.6 / 20.2.4) reject the PUT when
// Content-Type or any x-amz-* header is present but unsigned, and the archiver always
// sends a Content-Type. See https://gitlab.com/gitlab-org/gitlab-runner/-/work_items/39701.
func TestGetUploadURLSignsEverySentHeader(t *testing.T) {
	tests := map[string]struct {
		encryption      string
		encryptionKeyID string
		metadata        map[string]string
		expectedHeaders http.Header
	}{
		"no metadata": {
			expectedHeaders: http.Header{
				"Content-Type": {"application/octet-stream"},
				"Host":         {"server.com"},
			},
		},
		"with metadata": {
			metadata: map[string]string{"cachekey": "demo"},
			expectedHeaders: http.Header{
				"Content-Type":        {"application/octet-stream"},
				"Host":                {"server.com"},
				"X-Amz-Meta-Cachekey": {"demo"},
			},
		},
		"sse-s3 with metadata": {
			encryption: "S3",
			metadata:   map[string]string{"cachekey": "demo"},
			expectedHeaders: http.Header{
				"Content-Type":                 {"application/octet-stream"},
				"Host":                         {"server.com"},
				"X-Amz-Meta-Cachekey":          {"demo"},
				"X-Amz-Server-Side-Encryption": {"AES256"},
			},
		},
		"sse-kms with metadata": {
			encryption:      "KMS",
			encryptionKeyID: "alias/my-key",
			metadata:        map[string]string{"cachekey": "demo"},
			expectedHeaders: http.Header{
				"Content-Type":                 {"application/octet-stream"},
				"Host":                         {"server.com"},
				"X-Amz-Meta-Cachekey":          {"demo"},
				"X-Amz-Server-Side-Encryption": {"aws:kms"},
				"X-Amz-Server-Side-Encryption-Aws-Kms-Key-Id": {"alias/my-key"},
			},
		},
		"dsse-kms": {
			encryption:      "DSSE-KMS",
			encryptionKeyID: "alias/my-key",
			expectedHeaders: http.Header{
				"Content-Type":                 {"application/octet-stream"},
				"Host":                         {"server.com"},
				"X-Amz-Server-Side-Encryption": {"aws:kms:dsse"},
				"X-Amz-Server-Side-Encryption-Aws-Kms-Key-Id": {"alias/my-key"},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := defaultCacheFactory()
			cfg.S3.ServerSideEncryption = tc.encryption
			cfg.S3.ServerSideEncryptionKeyID = tc.encryptionKeyID

			adapter, err := New(cfg, defaultTimeout, objectName)
			require.NoError(t, err)

			if tc.metadata != nil {
				adapter.WithMetadata(tc.metadata)
			}

			u := adapter.GetUploadURL(t.Context())
			require.NotNil(t, u.URL, "presigned URL")
			assert.Equal(t, tc.expectedHeaders, u.Headers, "headers handed to the archiver")

			signed := strings.Split(u.URL.Query().Get("X-Amz-SignedHeaders"), ";")
			assert.Contains(t, signed, "content-type")
			for name := range u.Headers {
				assert.Contains(t, signed, strings.ToLower(name),
					"header %q is sent with the upload, so it must be signed", name)
			}
		})
	}
}

// Download and HEAD requests carry no body and no Content-Type, so nothing extra is signed
// or handed out for them.
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
