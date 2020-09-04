package network

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
)

const (
	rangeHeader = "Range"
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

func NewTracePatchResponse(response *http.Response, logger logrus.FieldLogger) *TracePatchResponse {
	result := &TracePatchResponse{
		RemoteJobStateResponse: NewRemoteJobStateResponse(response, logger),
	}

	if response != nil {
		result.RemoteRange = response.Header.Get(rangeHeader)
	}

	return result
}
