package network

import (
	"net/http"
	"strconv"
	"strings"
)

type TracePatchResponse struct {
	response *http.Response

	RemoteState string
	RemoteRange string
}

func (p *TracePatchResponse) IsAborted() bool {
	if p.RemoteState == "canceled" || p.RemoteState == "failed" {
		return true
	}

	if p.response.StatusCode == http.StatusForbidden {
		return true
	}

	return false
}

func (p *TracePatchResponse) NewOffset() int {
	remoteRangeParts := strings.Split(p.RemoteRange, "-")
	newOffset, _ := strconv.Atoi(remoteRangeParts[1])

	return newOffset
}

func NewTracePatchResponse(response *http.Response) *TracePatchResponse {
	return &TracePatchResponse{
		response:    response,
		RemoteState: response.Header.Get("Job-Status"),
		RemoteRange: response.Header.Get("Range"),
	}
}
