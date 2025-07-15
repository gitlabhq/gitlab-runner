package network

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

// NOTE: The functionality of the rate limiting below as well as the constant values
// are documented in `docs/configuration/proxy.md#handling-rate-limited-requests`

const (
	// RateLimit-ResetTime: Wed, 21 Oct 2015 07:28:00 GMT
	rateLimitResetTimeHeader = "RateLimit-ResetTime"
	retryAfterHeader         = "Retry-After"
	// The fallback is used if the reset header's value is present but cannot be parsed
	defaultRateLimitFallbackDelay = time.Minute
	defaultRateLimitRetriesCount  = 5
)

type rateLimitRequester struct {
	client        requester
	fallbackDelay time.Duration
	retriesCount  int
	logger        *logrus.Logger
}

func newRateLimitRequester(client requester) *rateLimitRequester {
	return &rateLimitRequester{
		client:        client,
		fallbackDelay: defaultRateLimitFallbackDelay,
		retriesCount:  defaultRateLimitRetriesCount,
		logger:        logrus.StandardLogger(),
	}
}

func (r *rateLimitRequester) Do(req *http.Request) (res *http.Response, err error) {
	logger := r.logger.
		WithFields(logrus.Fields{
			"context": "ratelimit-requester-gitlab-request",
			"url":     req.URL.String(),
			"method":  req.Method,
		})

	// Worst case would be the configured timeout from reverse proxy * retriesCount
	for i := 0; i < r.retriesCount; i++ {
		res, err = r.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("couldn't execute %s against %s: %w", req.Method, req.URL, err)
		}
		waitTime := calculateWaitTime(r.fallbackDelay, res, logger.Logger)
		if waitTime <= 0 {
			return res, nil
		}

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

func calculateWaitTime(fallbackDelay time.Duration, resp *http.Response, logger *logrus.Logger) time.Duration {
	if resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode != http.StatusServiceUnavailable {
		return 0
	}

	if waitTime := parseResetTime(resp, logger); waitTime > 0 {
		return waitTime
	}

	if waitTime := parseRetryAfter(resp, logger); waitTime > 0 {
		return waitTime
	}

	return fallbackDelay
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
