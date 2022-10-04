package network

import "net/http"

//go:generate mockery --name=requester --inpackage
type requester interface {
	Do(*http.Request) (*http.Response, error)
}
