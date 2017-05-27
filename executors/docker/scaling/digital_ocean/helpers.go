package digital_ocean

import (
	"net/http"
	"time"

	"github.com/digitalocean/godo"
	"github.com/cenkalti/backoff"
)

func isNotFound(resp *godo.Response) bool {
	return resp != nil && resp.StatusCode == http.StatusNotFound
}

func retry(resp *godo.Response, backoff *backoff.ExponentialBackOff) bool {
	if resp == nil {
		return false
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		when := resp.Rate.Reset.Sub(time.Now())
		nextBackOff := backoff.NextBackOff()
		if when < nextBackOff {
			time.Sleep(nextBackOff)
		} else {
			time.Sleep(when)
		}
		return true
	}

	if resp.StatusCode/100 == 5 {
		time.Sleep(backoff.NextBackOff())
	}
	return false
}
