package network

import "net/http"

type requester interface {
	Do(*http.Request) (*http.Response, error)
}
