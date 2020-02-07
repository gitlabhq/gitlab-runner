package network

import (
	"net/http"
)

const (
	remoteStateHeader = "Job-Status"

	statusCanceled = "canceled"
	statusFailed   = "failed"
)

type RemoteJobStateResponse struct {
	StatusCode  int
	RemoteState string
}

func (r *RemoteJobStateResponse) IsAborted() bool {
	if r.RemoteState == statusCanceled || r.RemoteState == statusFailed {
		return true
	}

	if r.StatusCode == http.StatusForbidden {
		return true
	}

	return false
}

func NewRemoteJobStateResponse(response *http.Response) *RemoteJobStateResponse {
	if response == nil {
		return &RemoteJobStateResponse{}
	}

	return &RemoteJobStateResponse{
		StatusCode:  response.StatusCode,
		RemoteState: response.Header.Get(remoteStateHeader),
	}
}
