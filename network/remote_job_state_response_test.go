//go:build !integration

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
	testCases := map[string]struct {
		response                     *http.Response
		expectedRemoteUpdateInterval time.Duration
	}{
		"nil response": {
			response:                     nil,
			expectedRemoteUpdateInterval: 0,
		},
		"no remote update period in header": {
			response:                     new(http.Response),
			expectedRemoteUpdateInterval: 0,
		},
		"invalid remote update period in header": {
			response:                     responseWithHeader(updateIntervalHeader, "invalid"),
			expectedRemoteUpdateInterval: 0,
		},
		"negative remote update period in header": {
			response:                     responseWithHeader(updateIntervalHeader, "-10"),
			expectedRemoteUpdateInterval: time.Duration(-10) * time.Second,
		},
		"valid remote update period in header": {
			response:                     responseWithHeader(updateIntervalHeader, "10"),
			expectedRemoteUpdateInterval: time.Duration(10) * time.Second,
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			log, _ := test.NewNullLogger()
			tpr := NewRemoteJobStateResponse(tc.response, log)

			assert.NotNil(t, tpr)
			assert.IsType(t, &RemoteJobStateResponse{}, tpr)
			assert.Equal(t, tc.expectedRemoteUpdateInterval, tpr.RemoteUpdateInterval)
		})
	}
}
