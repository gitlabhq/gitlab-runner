package network

import (
	"net/http"
)

type RemoteJobStateResponse struct {
	StatusCode  int
	RemoteState string
}

func (r *RemoteJobStateResponse) IsAborted() bool {
	if r.RemoteState == "canceled" || r.RemoteState == "failed" {
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
		RemoteState: response.Header.Get("Job-Status"),
	}
}
