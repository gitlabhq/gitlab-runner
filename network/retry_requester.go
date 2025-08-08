package network

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/jpillora/backoff"
	"github.com/sirupsen/logrus"
)

// NOTE: The functionality of the rate limiting below as well as the constant values
// are documented in `docs/configuration/proxy.md#handling-rate-limited-requests`

const (
	backOffMinDelay              = 100 * time.Millisecond
	backOffMaxDelay              = 60 * time.Second
	backOffDelayFactor           = 2.0
	backOffDelayJitter           = true
	defaultRateLimitRetriesCount = 5
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
	client       requester
	retriesCount int
	logger       *logrus.Logger
}

func newRetryRequester(client requester) *retryRequester {
	return &retryRequester{
		client:       client,
		retriesCount: defaultRateLimitRetriesCount,
		logger:       logrus.StandardLogger(),
	}
}

func (r *retryRequester) Do(req *http.Request) (res *http.Response, err error) {
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

	// Worst case would be the configured timeout from reverse proxy * retriesCount
	for i := 0; i < r.retriesCount; i++ {
		res, err = r.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("couldn't execute %s against %s: %w", req.Method, req.URL, err)
		}

		if !shouldRetryRequest(res) {
			return res, nil
		}

		waitTime := r.calculateWaitTime(res, bo)
		logger.
			WithField("duration", waitTime).
			Infoln("Waiting before making the next call")

		select {
		case <-time.After(waitTime):
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}

		if req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("failed to get body: %w", err)
			}

			req.Body = body
		}
	}

	return res, err
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
