package network

import (
	"net/http"
	"strconv"
	"strings"
)

type TracePatchResponse struct {
	*RemoteJobStateResponse

	RemoteRange string
}

func (p *TracePatchResponse) NewOffset() int {
	remoteRangeParts := strings.Split(p.RemoteRange, "-")
	if len(remoteRangeParts) == 2 {
		newOffset, _ := strconv.Atoi(remoteRangeParts[1])
		return newOffset
	}

	return 0
}

func NewTracePatchResponse(response *http.Response) *TracePatchResponse {
	return &TracePatchResponse{
		RemoteJobStateResponse: NewRemoteJobStateResponse(response),
		RemoteRange:            response.Header.Get("Range"),
	}
}
