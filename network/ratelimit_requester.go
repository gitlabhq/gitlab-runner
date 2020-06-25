package network

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// NOTE: The functionality of the rate limiting below as well as the constant values
// are documented in `docs/configuration/proxy.md#handling-rate-limited-requests`

const (
	// RateLimit-ResetTime: Wed, 21 Oct 2015 07:28:00 GMT
	rateLimitResetTimeHeader = "RateLimit-ResetTime"
	// The fallback is used if the reset header's value is present but cannot be parsed
	defaultRateLimitFallbackDelay = time.Minute
	defaultRateLimitRetriesCount  = 5
)

var (
	errRateLimitGaveUp = errors.New("gave up due to rate limit")
)

type rateLimitRequester struct {
	client        requester
	fallbackDelay time.Duration
	retriesCount  int
}

func newRateLimitRequester(client requester) *rateLimitRequester {
	return &rateLimitRequester{
		client:        client,
		fallbackDelay: defaultRateLimitFallbackDelay,
		retriesCount:  defaultRateLimitRetriesCount,
	}
}

func (r *rateLimitRequester) Do(req *http.Request) (*http.Response, error) {
	logger := logrus.
		WithFields(logrus.Fields{
			"context": "ratelimit-requester-gitlab-request",
			"url":     req.URL.String(),
			"method":  req.Method,
		})

	// Worst case would be the configured timeout from reverse proxy * retriesCount
	for i := 0; i < r.retriesCount; i++ {
		res, rateLimitDuration, err := r.do(req, logger)
		if rateLimitDuration == nil {
			return res, err
		}

		logger.
			WithField("duration", *rateLimitDuration).
			Infoln("Sleeping due to rate limit")
		// In some rare cases where the network is slow or the machine hosting
		// the runner is resource constrained by the time we get the header
		// it might be in the past, but that's ok since sleep will return immediately
		time.Sleep(*rateLimitDuration)
	}

	return nil, errRateLimitGaveUp
}

// If this method returns a non-nil duration this means that we got a rate limited response
// and the called should sleep for the duration. If the duration is nil, return the response and the error
// meaning that we got a non rate limited response
func (r *rateLimitRequester) do(req *http.Request, logger *logrus.Entry) (*http.Response, *time.Duration, error) {
	res, err := r.client.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't execute %s against %s: %w", req.Method, req.URL, err)
	}

	// The request passed and we got some non rate limited response
	if res.StatusCode != http.StatusTooManyRequests {
		return res, nil, nil
	}

	rateLimitResetTimeValue := res.Header.Get(rateLimitResetTimeHeader)
	if rateLimitResetTimeValue == "" {
		// if we get a 429 but don't have a rate limit reset header we just return the response
		// since we can't know how much to wait for the rate limit to reset
		return res, nil, nil
	}

	resetTime, err := time.Parse(time.RFC1123, rateLimitResetTimeValue)
	if err != nil {
		// If we can't parse the rate limit reset header there's something wrong with it
		// we shouldn't fail, to avoid a case where a misconfiguration in the reverse proxy can cause
		// all runners to stop working. Wait for the configured fallback instead
		logger.
			WithError(err).
			WithFields(logrus.Fields{
				"header":      rateLimitResetTimeHeader,
				"headerValue": rateLimitResetTimeValue,
			}).
			Warnln("Couldn't parse rate limit header, falling back")
		return res, &r.fallbackDelay, nil
	}

	resetDuration := time.Until(resetTime)
	return res, &resetDuration, nil
}
