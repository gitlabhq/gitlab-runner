package network

import (
	"net/http"
	"testing"
	"time"

	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func responseWithHeader(key string, value string) *http.Response {
	r := new(http.Response)
	r.Header = make(http.Header)
	r.Header.Add(key, value)

	return r
}

func TestNewTracePatchResponse(t *testing.T) {
	tracePatchTestCases := map[string]struct {
		response                          *http.Response
		expectedRemoteTraceUpdateInterval time.Duration
	}{
		"nil response": {
			response:                          nil,
			expectedRemoteTraceUpdateInterval: time.Duration(emptyRemoteTraceUpdateInterval),
		},
		"no remote trace period in header": {
			response:                          new(http.Response),
			expectedRemoteTraceUpdateInterval: time.Duration(emptyRemoteTraceUpdateInterval),
		},
		"invalid remote trace period in header": {
			response:                          responseWithHeader(traceUpdateIntervalHeader, "invalid"),
			expectedRemoteTraceUpdateInterval: time.Duration(emptyRemoteTraceUpdateInterval),
		},
		"negative remote trace period in header": {
			response:                          responseWithHeader(traceUpdateIntervalHeader, "-10"),
			expectedRemoteTraceUpdateInterval: time.Duration(-10) * time.Second,
		},
		"valid remote trace period in header": {
			response:                          responseWithHeader(traceUpdateIntervalHeader, "10"),
			expectedRemoteTraceUpdateInterval: time.Duration(10) * time.Second,
		},
	}

	for tn, tc := range tracePatchTestCases {
		t.Run(tn, func(t *testing.T) {
			log, _ := test.NewNullLogger()
			tpr := NewTracePatchResponse(tc.response, log)

			assert.NotNil(t, tpr)
			assert.IsType(t, &TracePatchResponse{}, tpr)
			assert.Equal(t, tc.expectedRemoteTraceUpdateInterval, tpr.RemoteTraceUpdateInterval)
		})
	}
}
