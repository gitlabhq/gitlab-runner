package network

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	rangeHeader               = "Range"
	traceUpdateIntervalHeader = "X-GitLab-Trace-Update-Interval"
)

type TracePatchResponse struct {
	*RemoteJobStateResponse

	RemoteRange               string
	RemoteTraceUpdateInterval time.Duration
}

func (p *TracePatchResponse) NewOffset() int {
	remoteRangeParts := strings.Split(p.RemoteRange, "-")
	if len(remoteRangeParts) == 2 {
		newOffset, _ := strconv.Atoi(remoteRangeParts[1])
		return newOffset
	}

	return 0
}

func NewTracePatchResponse(response *http.Response, logger logrus.FieldLogger) *TracePatchResponse {
	if response == nil {
		return new(TracePatchResponse)
	}

	var (
		err                       error
		remoteTraceUpdateInterval int
	)
	updateIntervalRaw := response.Header.Get(traceUpdateIntervalHeader)
	if updateIntervalRaw != "" {
		remoteTraceUpdateInterval, err = strconv.Atoi(updateIntervalRaw)
		if err != nil {
			remoteTraceUpdateInterval = emptyRemoteTraceUpdateInterval
			logger.WithError(err).
				WithField("header-value", updateIntervalRaw).
				Warningf("Failed to parse %q header", traceUpdateIntervalHeader)
		}
	}

	return &TracePatchResponse{
		RemoteJobStateResponse:    NewRemoteJobStateResponse(response),
		RemoteRange:               response.Header.Get(rangeHeader),
		RemoteTraceUpdateInterval: time.Duration(remoteTraceUpdateInterval) * time.Second,
	}
}
