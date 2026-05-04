//go:build !integration

package network

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestWithRetryTracker(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name           string
		setup          func(tb testing.TB) requester
		installTracker bool
		expectRetried  bool
	}{
		{
			name: "first-try success leaves tracker false",
			setup: func(tb testing.TB) requester {
				tb.Helper()
				mr := newMockRequester(t)
				mr.On("Do", mock.Anything).Once().
					Return(&http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil)
				return mr
			},
			installTracker: true,
			expectRetried:  false,
		},
		{
			name: "429 then success sets tracker",
			setup: func(tb testing.TB) requester {
				tb.Helper()
				mr := newMockRequester(t)
				mr.On("Do", mock.Anything).Once().Return(&http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header:     http.Header{"Retry-After": []string{"0"}},
					Body:       http.NoBody,
				}, nil)
				mr.On("Do", mock.Anything).Once().
					Return(&http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil)
				return mr
			},
			installTracker: true,
			expectRetried:  true,
		},
		{
			name: "503 then success sets tracker",
			setup: func(tb testing.TB) requester {
				tb.Helper()
				mr := newMockRequester(t)
				mr.On("Do", mock.Anything).Once().
					Return(&http.Response{StatusCode: http.StatusServiceUnavailable, Body: http.NoBody}, nil)
				mr.On("Do", mock.Anything).Once().
					Return(&http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil)
				return mr
			},
			installTracker: true,
			expectRetried:  true,
		},
		{
			name: "no tracker installed completes without panic",
			setup: func(tb testing.TB) requester {
				tb.Helper()
				mr := newMockRequester(t)
				mr.On("Do", mock.Anything).Once().Return(&http.Response{
					StatusCode: http.StatusTooManyRequests,
					Header:     http.Header{"Retry-After": []string{"0"}},
					Body:       http.NoBody,
				}, nil)
				mr.On("Do", mock.Anything).Once().
					Return(&http.Response{StatusCode: http.StatusOK, Body: http.NoBody}, nil)
				return mr
			},
			installTracker: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			r := newRetryRequester(tc.setup(t), NewAPIRequestsCollector())

			var (
				ctx     = t.Context()
				retried *atomic.Bool
			)
			if tc.installTracker {
				ctx, retried = WithRetryTracker(ctx)
			}
			req := httptest.NewRequestWithContext(ctx, http.MethodGet, "http://example.com", nil)

			resp, err := r.Do(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, resp.StatusCode)

			if tc.installTracker {
				assert.Equal(t, tc.expectRetried, retried.Load())
			}
		})
	}
}
