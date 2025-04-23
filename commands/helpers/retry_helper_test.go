//go:build !integration

package helpers

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
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

func TestRetryOnServerError(t *testing.T) {
	cases := map[string]struct {
		resp func() *http.Response
		err  error
	}{
		"successful request": {
			resp: func() *http.Response {
				return &http.Response{
					Status:     fmt.Sprintf("%d %s", http.StatusOK, http.StatusText(http.StatusOK)),
					StatusCode: http.StatusOK,
				}
			},
		},
		"failed request without xml format": {
			resp: func() *http.Response {
				return &http.Response{
					Status:     fmt.Sprintf("%d %s", http.StatusForbidden, http.StatusText(http.StatusForbidden)),
					StatusCode: http.StatusForbidden,
					Body:       io.NopCloser(strings.NewReader("Forbidden")),
				}
			},
			err: errors.New("received: 403 Forbidden"),
		},
		"failed request with xml format": {
			resp: func() *http.Response {
				return &http.Response{
					Status:     fmt.Sprintf("%d %s", http.StatusForbidden, http.StatusText(http.StatusForbidden)),
					StatusCode: http.StatusForbidden,
					Body: io.NopCloser(strings.NewReader(`<?xml version="1.0" encoding="UTF-8"?>
		<Error>
		  <Code>UploadFailure</Code>
		  <Message>Upload failure message</Message>
		  <Resource></Resource>
		  <RequestId></RequestId>
		</Error>`)),
				}
			},
			err: errors.New("received: 403 Forbidden. Request failed with code: UploadFailure, message: Upload failure message"),
		},
	}

	for tn, tc := range cases {
		t.Run(tn, func(t *testing.T) {
			err := retryOnServerError(tc.resp())

			assert.Equal(t, tc.err, err)
		})
	}
}
