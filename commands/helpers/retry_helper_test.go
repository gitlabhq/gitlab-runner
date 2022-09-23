//go:build !integration

package helpers

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoRetry(t *testing.T) {
	cases := []struct {
		name          string
		err           error
		expectedCount int
	}{
		{
			name:          "Error is of type retryableErr",
			err:           retryableErr{err: errors.New("error")},
			expectedCount: 4,
		},
		{
			name:          "Error is not type of retryableErr",
			err:           errors.New("error"),
			expectedCount: 1,
		},
		{
			name:          "Error is nil",
			err:           nil,
			expectedCount: 1,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := retryHelper{
				Retry: 3,
			}

			retryCount := 0
			err := r.doRetry(func(_ int) error {
				retryCount++
				return c.err
			})

			assert.Equal(t, c.err, err)
			assert.Equal(t, c.expectedCount, retryCount)
		})
	}
}
