package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// failUploadsRegistry fails the first failures blob upload requests with
// method, as GitLab's registry does when it loses an upload session.
type failUploadsRegistry struct {
	next     http.Handler
	method   string
	status   int
	code     string
	failures int32
	requests atomic.Int32
}

func (f *failUploadsRegistry) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.Method == f.method && strings.Contains(req.URL.Path, "/blobs/uploads/") && f.requests.Add(1) <= f.failures {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(f.status)
		_, _ = fmt.Fprintf(w, `{"errors":[{"code":%q,"message":"injected"}]}`, f.code)
		return
	}
	f.next.ServeHTTP(w, req)
}

func TestPushWithRetry(t *testing.T) {
	tests := []struct {
		name         string
		method       string
		status       int
		code         string
		failures     int32
		wantRequests int32
		wantErr      bool
	}{
		{
			name:         "lost upload session on PUT is retried",
			method:       http.MethodPut,
			status:       http.StatusNotFound,
			code:         "BLOB_UPLOAD_UNKNOWN",
			failures:     1,
			wantRequests: 2,
		},
		{
			name:         "invalid resumable upload on PATCH is retried",
			method:       http.MethodPatch,
			status:       http.StatusBadRequest,
			code:         "RESUMABLE_BLOB_UPLOAD_INVALID",
			failures:     1,
			wantRequests: 2,
		},
		{
			name:         "persistent lost upload session gives up",
			method:       http.MethodPut,
			status:       http.StatusNotFound,
			code:         "BLOB_UPLOAD_UNKNOWN",
			failures:     pushRetries + 1,
			wantRequests: pushRetries + 1,
			wantErr:      true,
		},
		{
			name:         "denied is not retried",
			method:       http.MethodPut,
			status:       http.StatusForbidden,
			code:         "DENIED",
			failures:     1,
			wantRequests: 1,
			wantErr:      true,
		},
	}

	oldBackoff := pushRetryBackoff
	pushRetryBackoff = 0
	t.Cleanup(func() { pushRetryBackoff = oldBackoff })

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := &failUploadsRegistry{
				next:     registry.New(registry.Logger(log.New(io.Discard, "", 0))),
				method:   tt.method,
				status:   tt.status,
				code:     tt.code,
				failures: tt.failures,
			}
			srv := httptest.NewServer(reg)
			defer srv.Close()

			ref, err := name.ParseReference(srv.Listener.Addr().String() + "/dev:latest")
			if err != nil {
				t.Fatal(err)
			}
			// No layers, so each attempt uploads only the config blob.
			img, err := random.Image(1024, 0)
			if err != nil {
				t.Fatal(err)
			}
			pusher, err := remote.NewPusher()
			if err != nil {
				t.Fatal(err)
			}

			err = pushWithRetry(t.Context(), pusher, ref, img)
			if got := reg.requests.Load(); got != tt.wantRequests {
				t.Fatalf("expected %d %s blob upload requests, got %d", tt.wantRequests, tt.method, got)
			}
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), tt.code) {
					t.Fatalf("expected %s error, got: %v", tt.code, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("pushWithRetry: %v", err)
			}
			if _, err := remote.Head(ref); err != nil {
				t.Fatalf("image missing on remote: %v", err)
			}
		})
	}
}
