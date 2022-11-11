package network

import (
	"net/http"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	updateIntervalHeader = "X-GitLab-Trace-Update-Interval"
	remoteStateHeader    = "Job-Status"

	statusCanceled  = "canceled"
	statusCanceling = "canceling"
	statusFailed    = "failed"
	statusRunning   = "running"
)

type RemoteJobStateResponse struct {
	StatusCode           int
	RemoteState          string
	RemoteUpdateInterval time.Duration
}

func (r *RemoteJobStateResponse) IsFailed() bool {
	if r.RemoteState == statusCanceled || r.RemoteState == statusFailed {
		return true
	}

	if r.StatusCode == http.StatusForbidden {
		return true
	}

	return false
}

func (r *RemoteJobStateResponse) IsCanceled() bool {
	return r.RemoteState == statusCanceling
}

func NewRemoteJobStateResponse(response *http.Response, logger logrus.FieldLogger) *RemoteJobStateResponse {
	if response == nil {
		return &RemoteJobStateResponse{}
	}

	result := &RemoteJobStateResponse{
		StatusCode:  response.StatusCode,
		RemoteState: response.Header.Get(remoteStateHeader),
	}

	if updateIntervalRaw := response.Header.Get(updateIntervalHeader); updateIntervalRaw != "" {
		if updateInterval, err := strconv.Atoi(updateIntervalRaw); err == nil {
			result.RemoteUpdateInterval = time.Duration(updateInterval) * time.Second
		} else {
			logger.WithError(err).
				WithField("header-value", updateIntervalRaw).
				Warningf("Failed to parse %q header", updateIntervalHeader)
		}
	}

	return result
}
