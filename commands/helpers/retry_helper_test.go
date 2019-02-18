package helpers

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoRetryError(t *testing.T) {
	r := retryHelper{
		Retry: 3,
	}

	retryCount := 0
	err := r.doRetry(func() error {
		retryCount++
		return retryableErr{err: errors.New("error")}
	})
	assert.Error(t, err)
	assert.Equal(t, r.Retry+1, retryCount)
}

func TestDoRetry(t *testing.T) {
	r := retryHelper{
		Retry: 3,
	}

	retryCount := 0
	err := r.doRetry(func() error {
		retryCount++
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 1, retryCount)
}
