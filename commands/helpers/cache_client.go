package helpers

import (
	"net"
	"net/http"
	"time"
)

type CacheClient struct {
	http.Client
}

func (c *CacheClient) prepareClient() {
	c.Timeout = 3 * time.Minute
}

func (c *CacheClient) prepareTransport() {
	c.Transport = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
	}
}

func NewCacheClient() *CacheClient {
	client := &CacheClient{}
	client.prepareClient()
	client.prepareTransport()

	return client
}
