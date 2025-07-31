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

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNewRateLimitRequester(t *testing.T) {
	rl := newRateLimitRequester(http.DefaultClient)

	assert.Equal(t, rl.client, http.DefaultClient)
	assert.Equal(t, rl.fallbackDelay, defaultRateLimitFallbackDelay)
	assert.Equal(t, rl.retriesCount, defaultRateLimitRetriesCount)
}

func TestRateLimitRequester_Do(t *testing.T) {
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
			name:    "not 409 or 503",
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
			name:    "with reset header",
			request: httptest.NewRequest(http.MethodPost, "http://example.com", strings.NewReader("somebody")),
			setup: func(tb testing.TB) requester {
				tb.Helper()
				mr := newMockRequester(t)
				res := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}}
				res.Header.Set(rateLimitResetTimeHeader, time.Now().Add(time.Second).Format(time.RFC1123))
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
				res := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}}
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
				duration:   2 * time.Second,
				statusCode: http.StatusOK,
			},
		},
		{
			name:    "with retry header",
			request: httptest.NewRequest(http.MethodPost, "http://example.com", strings.NewReader("somebody")),
			setup: func(tb testing.TB) requester {
				tb.Helper()
				mr := newMockRequester(t)
				res := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}}
				res.Header.Set(rateLimitResetTimeHeader, "1")
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
				res := &http.Response{StatusCode: http.StatusTooManyRequests, Header: http.Header{}}
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
				duration:   2 * time.Second,
				statusCode: http.StatusOK,
			},
		},
		{
			name:    "request ctx cancellation",
			request: httptest.NewRequestWithContext(cancelledCtx, http.MethodPost, "http://example.com", strings.NewReader("somebody")),
			setup: func(tb testing.TB) requester {
				tb.Helper()
				mr := newMockRequester(t)
				res := &http.Response{StatusCode: http.StatusTooManyRequests}
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
				res := &http.Response{StatusCode: http.StatusTooManyRequests}
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
			rlr := newRateLimitRequester(mr)
			rlr.fallbackDelay = time.Second
			rlr.retriesCount = 3
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

func TestRateLimitRequester_Do_BodyCopiedBetweenRequests(t *testing.T) {
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

	rlr := newRateLimitRequester(http.DefaultClient)
	rlr.fallbackDelay = time.Millisecond
	rlr.retriesCount = 5

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

func TestCalculateWaitTime(t *testing.T) {
	testCases := []struct {
		name             string
		setup            func(tb testing.TB, status int) *http.Response
		expectedDuration time.Duration
	}{
		{
			name: "status code not 429 or 503",
			setup: func(tb testing.TB, status int) *http.Response {
				res := &http.Response{
					StatusCode: http.StatusBadRequest,
				}
				return res
			},
			expectedDuration: 0,
		},
		{
			name: "valid reset time",
			setup: func(tb testing.TB, status int) *http.Response {
				res := &http.Response{
					Header:     http.Header{},
					StatusCode: status,
				}
				res.Header.Set(rateLimitResetTimeHeader, time.Now().Add(2*time.Minute).Format(time.RFC1123))
				return res
			},
			expectedDuration: 2 * time.Minute,
		},
		{
			name: "fallback to retry time",
			setup: func(tb testing.TB, status int) *http.Response {
				res := &http.Response{
					Header:     http.Header{},
					StatusCode: status,
				}
				res.Header.Set(rateLimitResetTimeHeader, "invalid time")
				res.Header.Set(retryAfterHeader, "120")
				return res
			},
			expectedDuration: 2 * time.Minute,
		},
		{
			name: "valid retry time",
			setup: func(tb testing.TB, status int) *http.Response {
				res := &http.Response{
					Header:     http.Header{},
					StatusCode: status,
				}
				res.Header.Set(retryAfterHeader, "120")
				return res
			},
			expectedDuration: 2 * time.Minute,
		},
		{
			name: "fallback to default",
			setup: func(tb testing.TB, status int) *http.Response {
				res := &http.Response{
					Header:     http.Header{},
					StatusCode: status,
				}
				res.Header.Set(rateLimitResetTimeHeader, "invalid time")
				res.Header.Set(retryAfterHeader, "invalid time")
				return res
			},
			expectedDuration: time.Minute,
		},
	}

	for _, tc := range testCases {
		for _, status := range []int{http.StatusTooManyRequests, http.StatusServiceUnavailable} {
			t.Run(fmt.Sprintf("status %d %s", status, tc.name), func(t *testing.T) {
				t.Parallel()
				logger, _ := test.NewNullLogger()
				duration := calculateWaitTime(time.Minute, tc.setup(t, status), logger)

				assert.InDelta(t, tc.expectedDuration, duration, float64(time.Second))
			})
		}
	}
}
