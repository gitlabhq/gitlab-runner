//go:build !integration

package network

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRateLimitRequester(t *testing.T) {
	rl := newRateLimitRequester(http.DefaultClient)

	assert.Equal(t, rl.client, http.DefaultClient)
	assert.Equal(t, rl.fallbackDelay, defaultRateLimitFallbackDelay)
	assert.Equal(t, rl.retriesCount, defaultRateLimitRetriesCount)
}

func TestRateLimitRequestExecutor(t *testing.T) {
	var callsCount int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rateLimitedCalls, _ := strconv.Atoi(r.Header.Get("rateLimitedCalls"))
		provideRateLimitResetHeader, _ := strconv.ParseBool(r.Header.Get("provideRateLimitResetHeader"))
		rateLimitResetHeaderValue := r.Header.Get("rateLimitResetHeaderValue")

		if atomic.AddInt32(&callsCount, 1) > int32(rateLimitedCalls) {
			w.WriteHeader(http.StatusOK)
			return
		}

		if provideRateLimitResetHeader {
			w.Header().Add(rateLimitResetTimeHeader, rateLimitResetHeaderValue)
		}
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer s.Close()

	tests := map[string]struct {
		rateLimitedCalls            int
		provideRateLimitResetHeader bool
		rateLimitResetHeaderValue   string
		fallbackDelay               time.Duration

		expectedCallsCount int
		expectedStatusCode int
		expectedError      error
	}{
		"no rate limit": {
			rateLimitedCalls:            0,
			provideRateLimitResetHeader: true,

			expectedCallsCount: 1,
			expectedStatusCode: http.StatusOK,
		},
		"rate limit 2 requests": {
			rateLimitedCalls:            2,
			provideRateLimitResetHeader: true,

			expectedCallsCount: 3,
			expectedStatusCode: http.StatusOK,
		},
		"too many requests but missing rate limit reset header": {
			rateLimitedCalls:            1,
			provideRateLimitResetHeader: false,

			expectedCallsCount: 1,
			expectedStatusCode: http.StatusTooManyRequests,
		},
		"invalid rate limit header value": {
			rateLimitedCalls:            1,
			provideRateLimitResetHeader: true,
			rateLimitResetHeaderValue:   "invalid",
			fallbackDelay:               time.Millisecond,

			expectedCallsCount: 2,
			expectedStatusCode: http.StatusOK,
		},
		"try more than max retries count": {
			rateLimitedCalls:            defaultRateLimitRetriesCount + 1,
			provideRateLimitResetHeader: true,

			expectedCallsCount: defaultRateLimitRetriesCount,
			expectedError:      errRateLimitGaveUp,
			expectedStatusCode: http.StatusOK,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			defer atomic.StoreInt32(&callsCount, 0)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			go func() {
				defer cancel()
				if tt.rateLimitResetHeaderValue == "" {
					tt.rateLimitResetHeaderValue = fmt.Sprint(time.Now().Format(time.RFC1123))
				}

				if tt.fallbackDelay == 0 {
					// set the delay to something large so the test can timeout if we hit the fallback
					// without expecting to
					tt.fallbackDelay = 30 * time.Second
				}

				rl := rateLimitRequester{
					client:        http.DefaultClient,
					retriesCount:  defaultRateLimitRetriesCount,
					fallbackDelay: tt.fallbackDelay,
				}

				req, err := http.NewRequest(http.MethodGet, s.URL, nil)
				require.NoError(t, err)

				req.Header.Set("rateLimitedCalls", fmt.Sprint(tt.rateLimitedCalls))
				req.Header.Set("provideRateLimitResetHeader", fmt.Sprint(tt.provideRateLimitResetHeader))
				req.Header.Set("rateLimitResetHeaderValue", tt.rateLimitResetHeaderValue)

				res, err := rl.Do(req)
				if tt.expectedError != nil {
					assert.EqualError(t, err, tt.expectedError.Error())
					return
				}

				require.NoError(t, err)
				assert.Equal(t, tt.expectedCallsCount, int(atomic.LoadInt32(&callsCount)))
				assert.Equal(t, tt.expectedStatusCode, res.StatusCode)
			}()

			<-ctx.Done()
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				t.Fatal("timeout, hit fallback delay when shouldn't")
			}
		})
	}
}
