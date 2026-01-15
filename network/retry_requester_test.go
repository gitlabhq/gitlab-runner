//go:build !integration

package network

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jpillora/backoff"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewRetryRequester(t *testing.T) {
	t.Parallel()
	apiRequestCollector := NewAPIRequestsCollector()
	rl := newRetryRequester(http.DefaultClient, apiRequestCollector)

	assert.Equal(t, apiRequestCollector, rl.apiRequestCollector)
	assert.Equal(t, rl.client, http.DefaultClient)
	assert.Equal(t, rl.maxAttempts, defaultRateLimitMaxAttempts)
	assert.NotNil(t, rl.logger)
}

func TestRetryRequester_Do(t *testing.T) {
	t.Parallel()

	cancelledCtx, cancel := context.WithCancel(t.Context())
	cancel()

	type expectations struct {
		err        error
		duration   time.Duration
		statusCode int
	}

	testCases := []struct {
		name         string
		request      *http.Request
		setup        func(tb testing.TB) requester
		expectations expectations
	}{
		{
			name:    "success",
			request: httptest.NewRequest(http.MethodGet, "http://example.com", nil),
			setup: func(tb testing.TB) requester {
				tb.Helper()
				mr := newMockRequester(t)
				mr.On("Do", mock.Anything).Once().Return(&http.Response{StatusCode: http.StatusOK}, nil)
				return mr
			},
			expectations: expectations{
				statusCode: http.StatusOK,
			},
		},
		{
			name:    "client error",
			request: httptest.NewRequest(http.MethodGet, "http://example.com", nil),
			setup: func(tb testing.TB) requester {
				tb.Helper()
				mr := newMockRequester(t)
				mr.On("Do", mock.Anything).Once().Return(nil, errors.New("client error"))
				return mr
			},
			expectations: expectations{
				err: fmt.Errorf("couldn't execute %s against %s: %w", http.MethodGet, "http://example.com", errors.New("client error")),
			},
		},
		{
			name:    "non retry-able status code",
			request: httptest.NewRequest(http.MethodGet, "http://example.com", nil),
			setup: func(tb testing.TB) requester {
				tb.Helper()
				mr := newMockRequester(t)
				mr.On("Do", mock.Anything).Once().Return(&http.Response{StatusCode: http.StatusBadRequest}, nil)
				return mr
			},
			expectations: expectations{
				statusCode: http.StatusBadRequest,
			},
		},
		{
			name:    "retry-able status code",
			request: httptest.NewRequest(http.MethodPost, "http://example.com", strings.NewReader("somebody")),
			setup: func(tb testing.TB) requester {
				tb.Helper()
				mr := newMockRequester(t)
				res := &http.Response{StatusCode: http.StatusInternalServerError, Body: io.NopCloser(strings.NewReader(""))}
				call1 := mr.On("Do", mock.Anything).Twice().Return(res, nil)
				call2 := mr.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					rawBytes, _ := io.ReadAll(req.Body)
					return string(rawBytes) == "somebody"
				})).Once().Return(&http.Response{StatusCode: http.StatusOK}, nil)
				call2.NotBefore(call1)
				return mr
			},
			expectations: expectations{
				duration:   200 * time.Millisecond,
				statusCode: http.StatusOK,
			},
		},
		{
			name:    "with reset header",
			request: httptest.NewRequest(http.MethodPost, "http://example.com", strings.NewReader("somebody")),
			setup: func(tb testing.TB) requester {
				tb.Helper()
				mr := newMockRequester(t)
				res := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}
				res.Header.Set(rateLimitResetTimeHeader, time.Now().Add(2*time.Second).Format(time.RFC1123))
				call1 := mr.On("Do", mock.Anything).Twice().Return(res, nil)
				call2 := mr.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					rawBytes, _ := io.ReadAll(req.Body)
					return string(rawBytes) == "somebody"
				})).Once().Return(&http.Response{StatusCode: http.StatusOK}, nil)
				call2.NotBefore(call1)
				return mr
			},
			expectations: expectations{
				duration:   2 * time.Second,
				statusCode: http.StatusOK,
			},
		},
		{
			name:    "invalid reset header",
			request: httptest.NewRequest(http.MethodPost, "http://example.com", strings.NewReader("somebody")),
			setup: func(tb testing.TB) requester {
				tb.Helper()
				mr := newMockRequester(t)
				res := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}
				res.Header.Set(rateLimitResetTimeHeader, "invalid")
				call1 := mr.On("Do", mock.Anything).Twice().Return(res, nil)
				call2 := mr.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					rawBytes, _ := io.ReadAll(req.Body)
					return string(rawBytes) == "somebody"
				})).Once().Return(&http.Response{StatusCode: http.StatusOK}, nil)
				call2.NotBefore(call1)
				return mr
			},
			expectations: expectations{
				duration:   backOffMinDelay,
				statusCode: http.StatusOK,
			},
		},
		{
			name:    "with retry header",
			request: httptest.NewRequest(http.MethodPost, "http://example.com", strings.NewReader("somebody")),
			setup: func(tb testing.TB) requester {
				tb.Helper()
				mr := newMockRequester(t)
				res := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}
				res.Header.Set(retryAfterHeader, "1")
				call1 := mr.On("Do", mock.Anything).Twice().Return(res, nil)
				call2 := mr.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					rawBytes, _ := io.ReadAll(req.Body)
					return string(rawBytes) == "somebody"
				})).Once().Return(&http.Response{StatusCode: http.StatusOK}, nil)
				call2.NotBefore(call1)
				return mr
			},
			expectations: expectations{
				duration:   2 * time.Second,
				statusCode: http.StatusOK,
			},
		},
		{
			name:    "with invalid retry header",
			request: httptest.NewRequest(http.MethodPost, "http://example.com", strings.NewReader("somebody")),
			setup: func(tb testing.TB) requester {
				tb.Helper()
				mr := newMockRequester(t)
				res := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(""))}
				res.Header.Set(retryAfterHeader, "invalid")
				call1 := mr.On("Do", mock.Anything).Twice().Return(res, nil)
				call2 := mr.On("Do", mock.MatchedBy(func(req *http.Request) bool {
					rawBytes, _ := io.ReadAll(req.Body)
					return string(rawBytes) == "somebody"
				})).Once().Return(&http.Response{StatusCode: http.StatusOK}, nil)
				call2.NotBefore(call1)
				return mr
			},
			expectations: expectations{
				duration:   backOffMinDelay,
				statusCode: http.StatusOK,
			},
		},
		{
			name:    "request ctx cancellation",
			request: httptest.NewRequestWithContext(cancelledCtx, http.MethodPost, "http://example.com", strings.NewReader("somebody")),
			setup: func(tb testing.TB) requester {
				tb.Helper()
				mr := newMockRequester(t)
				res := &http.Response{StatusCode: http.StatusTooManyRequests, Body: io.NopCloser(strings.NewReader(""))}
				mr.On("Do", mock.Anything).Once().Return(res, nil)
				return mr
			},
			expectations: expectations{
				err: context.Canceled,
			},
		},
		{
			name:    "retries exhausted",
			request: httptest.NewRequest(http.MethodPost, "http://example.com", strings.NewReader("somebody")),
			setup: func(tb testing.TB) requester {
				mr := newMockRequester(t)
				res := &http.Response{StatusCode: http.StatusTooManyRequests, Body: io.NopCloser(strings.NewReader(""))}
				mr.On("Do", mock.Anything).Times(3).Return(res, nil)
				return mr
			},
			expectations: expectations{
				statusCode: http.StatusTooManyRequests,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			mr := tc.setup(t)
			rlr := newRetryRequester(mr, NewAPIRequestsCollector())
			rlr.maxAttempts = 3
			logger, _ := test.NewNullLogger()
			rlr.logger = logger

			start := time.Now()
			res, err := rlr.Do(tc.request)
			timeTaken := time.Since(start)

			if tc.expectations.duration != 0 {
				assert.InDelta(t, tc.expectations.duration, timeTaken, float64(time.Second))
			}

			if tc.expectations.err != nil {
				assert.Error(t, err)
				assert.ErrorContains(t, err, tc.expectations.err.Error())
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, res)
				assert.Equal(t, tc.expectations.statusCode, res.StatusCode)
			}
		})
	}
}

func TestRetryRequester_Do_BodyCopiedBetweenRequests(t *testing.T) {
	t.Parallel()

	requestCount := 0
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			requestCount++
			r.Body.Close()
		}()

		if requestCount <= 2 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		body, err := io.ReadAll(r.Body)

		assert.NoError(t, err)
		assert.Equal(t, "somebody", string(body))

		_, err = w.Write(body)
		assert.NoError(t, err)
	}))
	defer testServer.Close()

	rlr := newRetryRequester(http.DefaultClient, NewAPIRequestsCollector())
	rlr.maxAttempts = 5
	logger, _ := test.NewNullLogger()
	rlr.logger = logger

	req, err := http.NewRequest(http.MethodPost, testServer.URL, strings.NewReader("somebody"))
	assert.NoError(t, err)

	res, err := rlr.Do(req)
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, http.StatusOK, res.StatusCode)

	body, err := io.ReadAll(res.Body)
	res.Body.Close()
	assert.NoError(t, err)
	assert.Equal(t, "somebody", string(body))
	assert.Equal(t, 4, requestCount)
}

// trackingReadCloser tracks whether Close was called on the response body.
type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (t *trackingReadCloser) Close() error {
	t.closed = true
	return nil
}

func TestRetryRequester_Do_ResponseBodyClosedOnRetry(t *testing.T) {
	t.Parallel()

	var responseBodies []*trackingReadCloser

	mr := newMockRequester(t)
	mr.On("Do", mock.Anything).Times(3).Return(func(*http.Request) *http.Response {
		body := &trackingReadCloser{Reader: strings.NewReader("rate limited")}
		responseBodies = append(responseBodies, body)
		return &http.Response{
			StatusCode: http.StatusTooManyRequests,
			Body:       body,
		}
	}, nil)

	rlr := newRetryRequester(mr, NewAPIRequestsCollector())
	rlr.maxAttempts = 3
	logger, _ := test.NewNullLogger()
	rlr.logger = logger

	req := httptest.NewRequest(http.MethodGet, "http://example.com", nil)
	res, err := rlr.Do(req)

	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, http.StatusTooManyRequests, res.StatusCode)
	assert.Len(t, responseBodies, 3)

	// All response bodies except the last one should have been closed before retrying
	for i, body := range responseBodies[:len(responseBodies)-1] {
		assert.True(t, body.closed, "response body %d should have been closed before retry", i)
	}
	// The last response body is returned to the caller and should NOT be closed by retryRequester
	assert.False(t, responseBodies[len(responseBodies)-1].closed, "last response body should not be closed by retryRequester")
}

func TestRetryRequester_calculateWaitTime(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name             string
		setup            func(tb testing.TB) *http.Response
		expectedDuration time.Duration
	}{
		{
			name: "valid reset time",
			setup: func(tb testing.TB) *http.Response {
				res := &http.Response{
					Header: http.Header{},
				}
				res.Header.Set(rateLimitResetTimeHeader, time.Now().Add(2*time.Minute).Format(time.RFC1123))
				return res
			},
			expectedDuration: 2 * time.Minute,
		},
		{
			name: "fallback to retry time",
			setup: func(tb testing.TB) *http.Response {
				res := &http.Response{
					Header: http.Header{},
				}
				res.Header.Set(rateLimitResetTimeHeader, "invalid time")
				res.Header.Set(retryAfterHeader, "120")
				return res
			},
			expectedDuration: 2 * time.Minute,
		},
		{
			name: "valid retry time",
			setup: func(tb testing.TB) *http.Response {
				res := &http.Response{
					Header: http.Header{},
				}
				res.Header.Set(retryAfterHeader, "120")
				return res
			},
			expectedDuration: 2 * time.Minute,
		},
		{
			name: "fallback to provided backoff",
			setup: func(tb testing.TB) *http.Response {
				res := &http.Response{
					Header: http.Header{},
				}
				res.Header.Set(rateLimitResetTimeHeader, "invalid time")
				res.Header.Set(retryAfterHeader, "invalid time")
				return res
			},
			expectedDuration: 100 * time.Millisecond,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			logger, _ := test.NewNullLogger()
			rtr := newRetryRequester(nil, NewAPIRequestsCollector())
			rtr.logger = logger
			duration := rtr.calculateWaitTime(tc.setup(t), &backoff.Backoff{})

			assert.InDelta(t, tc.expectedDuration, duration, float64(time.Second))
		})
	}
}

func TestShouldRetryRequest(t *testing.T) {
	t.Parallel()

	for status, shouldRetry := range map[int]bool{
		http.StatusRequestTimeout:      true,
		http.StatusTooManyRequests:     true,
		http.StatusInternalServerError: true,
		http.StatusBadGateway:          true,
		http.StatusServiceUnavailable:  true,
		http.StatusGatewayTimeout:      true,
		515:                            true,
		http.StatusOK:                  false,
		http.StatusPermanentRedirect:   false,
	} {
		t.Run(fmt.Sprintf("status: %d should be retried: %v", status, shouldRetry), func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, shouldRetryRequest(&http.Response{StatusCode: status}), shouldRetry)
		})
	}
}

func TestNormalizedURI(t *testing.T) {
	t.Parallel()

	normalizeURItestCases := []struct {
		requestPath string
		expect      string
	}{
		{
			requestPath: "/",
			expect:      "/",
		},
		{
			requestPath: "/runners",
			expect:      "/runners",
		},
		{
			requestPath: "/runners/verify",
			expect:      "/runners/verify",
		},
		{
			requestPath: "/jobs/12345",
			expect:      "/jobs/{id}",
		},
		{
			requestPath: "/jobs/12345/trace",
			expect:      "/jobs/{id}/trace",
		},
		{
			requestPath: "/1",
			expect:      "/{id}",
		},
		{
			requestPath: "/1/2/3",
			expect:      "/{id}/{id}/{id}",
		},
		{
			requestPath: "/1/",
			expect:      "/{id}/",
		},
	}

	for _, tc := range normalizeURItestCases {
		t.Run(fmt.Sprintf("%s from %s", tc.requestPath, tc.expect), func(t *testing.T) {
			t.Parallel()

			res := normalizedURI(tc.requestPath)

			assert.Equal(t, tc.expect, res)
		})
	}
}
