package network

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jpillora/backoff"
	"github.com/sirupsen/logrus"
)

// NOTE: The functionality of the rate limiting below as well as the constant values
// are documented in `docs/configuration/proxy.md#handling-rate-limited-requests`

const (
	backOffMinDelay             = 100 * time.Millisecond
	backOffMaxDelay             = 60 * time.Second
	backOffDelayFactor          = 2.0
	backOffDelayJitter          = true
	defaultRateLimitMaxAttempts = 5
	// RateLimit-ResetTime: Wed, 21 Oct 2015 07:28:00 GMT
	rateLimitResetTimeHeader = "RateLimit-ResetTime"
	retryAfterHeader         = "Retry-After"
)

var retryStatuses = map[int]struct{}{
	http.StatusRequestTimeout:      {},
	http.StatusTooManyRequests:     {},
	http.StatusInternalServerError: {},
	http.StatusBadGateway:          {},
	http.StatusServiceUnavailable:  {},
	http.StatusGatewayTimeout:      {},
}

type retryRequester struct {
	apiRequestCollector *APIRequestsCollector
	client              requester
	maxAttempts         int
	logger              *logrus.Logger
}

func newRetryRequester(client requester, apiRequestCollector *APIRequestsCollector) *retryRequester {
	return &retryRequester{
		apiRequestCollector: apiRequestCollector,
		client:              client,
		maxAttempts:         defaultRateLimitMaxAttempts,
		logger:              logrus.StandardLogger(),
	}
}

func (r *retryRequester) Do(req *http.Request) (*http.Response, error) {
	logger := r.logger.
		WithFields(logrus.Fields{
			"context": "ratelimit-requester-gitlab-request",
			"url":     req.URL.String(),
			"method":  req.Method,
		})

	bo := &backoff.Backoff{
		Min:    backOffMinDelay,
		Max:    backOffMaxDelay,
		Factor: backOffDelayFactor,
		Jitter: backOffDelayJitter,
	}

	res, attempts, err := r.executeRequestWithRetries(req, bo, logger)

	// Track total attempts (including initial request) for metrics.
	// Note: Despite the method name "AddRetries", this tracks all attempts, not just retries.
	// This maintains backward compatibility with existing metrics collection behavior.
	// See discussion: https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/6041#note_3004520228
	r.apiRequestCollector.AddRetries(logger, normalizedURI(req.URL.Path), req.Method, float64(attempts))
	return res, err
}

func (r *retryRequester) executeRequestWithRetries(req *http.Request, bo *backoff.Backoff, logger *logrus.Entry) (*http.Response, int, error) {
	var attempts int
	var resp *http.Response
	success := false

	defer func() {
		if !success {
			closeResponseBody(resp, true)
		}
	}()

	for {
		var err error
		resp, err = r.client.Do(req)
		attempts++
		if err != nil {
			return nil, attempts, fmt.Errorf("couldn't execute %s against %s: %w", req.Method, req.URL, err)
		}

		if !shouldRetryRequest(resp) || attempts >= r.maxAttempts {
			success = true
			return resp, attempts, nil
		}

		closeResponseBody(resp, true)

		if err := r.waitForRetry(req, resp, bo, logger); err != nil {
			return nil, attempts, err
		}

		if err := r.regenerateRequestBody(req); err != nil {
			return nil, attempts, err
		}
	}
}

func (r *retryRequester) waitForRetry(req *http.Request, resp *http.Response, bo *backoff.Backoff, logger *logrus.Entry) error {
	waitTime := r.calculateWaitTime(resp, bo)
	logger.
		WithField("duration", waitTime).
		Infoln("Waiting before making the next call")

	timer := time.NewTimer(waitTime)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-req.Context().Done():
		return req.Context().Err()
	}
}

func (r *retryRequester) regenerateRequestBody(req *http.Request) error {
	if req.GetBody == nil {
		return nil
	}

	body, err := req.GetBody()
	if err != nil {
		return fmt.Errorf("failed to get body: %w", err)
	}

	req.Body = body
	return nil
}

func shouldRetryRequest(res *http.Response) bool {
	_, ok := retryStatuses[res.StatusCode]
	return ok || res.StatusCode >= 512
}

func (r *retryRequester) calculateWaitTime(resp *http.Response, bo *backoff.Backoff) time.Duration {
	if waitTime := parseResetTime(resp, r.logger); waitTime > 0 {
		return waitTime
	}

	if waitTime := parseRetryAfter(resp, r.logger); waitTime > 0 {
		return waitTime
	}

	return bo.Duration()
}

func parseResetTime(resp *http.Response, logger *logrus.Logger) time.Duration {
	resetTimeStr := resp.Header.Get(rateLimitResetTimeHeader)
	if resetTimeStr == "" {
		return 0
	}

	resetTime, err := time.Parse(time.RFC1123, resetTimeStr)
	if err != nil {
		logger.
			WithError(err).
			WithFields(logrus.Fields{
				"header":      rateLimitResetTimeHeader,
				"headerValue": resetTimeStr,
			}).
			Warnln("Couldn't parse rate limit header")
		return 0
	}

	return time.Until(resetTime)
}

func parseRetryAfter(resp *http.Response, logger *logrus.Logger) time.Duration {
	retryAfter := resp.Header.Get(retryAfterHeader)
	if retryAfter == "" {
		return 0
	}

	retrySeconds, err := strconv.Atoi(retryAfter)
	if err != nil {
		logger.
			WithError(err).
			WithFields(logrus.Fields{
				"header":      retryAfterHeader,
				"headerValue": retryAfter,
			}).
			Warnln("Couldn't parse retry after header")
		return 0
	}

	return time.Duration(retrySeconds) * time.Second
}

func normalizedURI(path string) string {
	if path == "" || path == "/" {
		return path
	}

	// Split path into segments
	segments := strings.Split(path, "/")

	for i, segment := range segments {
		if segment == "" {
			continue
		}
		if _, err := strconv.ParseInt(segment, 10, 64); err == nil {
			segments[i] = "{id}"
		}
	}

	return strings.Join(segments, "/")
}
