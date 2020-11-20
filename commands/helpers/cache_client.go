package helpers

import (
	"net"
	"net/http"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type CacheClient struct {
	http.Client
}

func (c *CacheClient) prepareClient(timeout int) {
	if timeout > 0 {
		c.Timeout = time.Duration(timeout) * time.Minute
	} else {
		c.Timeout = time.Duration(common.DefaultCacheRequestTimeout) * time.Minute
	}
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
		DisableCompression:    true,
	}
}

func NewCacheClient(timeout int) *CacheClient {
	client := &CacheClient{}
	client.prepareClient(timeout)
	client.prepareTransport()

	return client
}
