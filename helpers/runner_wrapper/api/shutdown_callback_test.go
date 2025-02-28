//go:build !integration

package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShutdownCallback(t *testing.T) {
	const (
		testTimeout     = 10 * time.Second
		testRequestURI  = "/test"
		testStatusCode  = 444
		testStatus      = "444 status code 444"
		testHeader      = "Test-Header"
		testHeaderValue = "test header value"
	)

	tests := map[string]struct {
		prepareTestServer func(t *testing.T) (string, func())
		method            string
		expectedError     string
		assertError       func(t *testing.T, err error)
	}{
		"request creation failure": {
			prepareTestServer: func(t *testing.T) (string, func()) {
				return "", func() {}
			},
			method:        "wrong method",
			expectedError: "unsupported protocol scheme",
			assertError: func(t *testing.T, err error) {
				assert.Contains(t, err.Error(), `net/http: invalid method "wrong method"`)
			},
		},
		"HTTP request failure": {
			prepareTestServer: func(t *testing.T) (string, func()) {
				return "", func() {}
			},
			method: http.MethodGet,
			assertError: func(t *testing.T, err error) {
				var eerr *url.Error
				if assert.ErrorAs(t, err, &eerr) {
					assert.Contains(t, eerr.Error(), `Get "": unsupported protocol scheme ""`)
				}
			},
		},
		"HTTP request executed properly": {
			prepareTestServer: func(t *testing.T) (string, func()) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(testStatusCode)

					_, _ = fmt.Fprintln(w, "test response to discard")

					assert.Equal(t, http.MethodGet, r.Method)
					assert.Equal(t, testRequestURI, r.RequestURI)

					if assert.Contains(t, r.Header, testHeader) {
						assert.Equal(t, testHeaderValue, r.Header.Get(testHeader))
					}
				}))

				return server.URL + testRequestURI, server.Close
			},
			method: http.MethodGet,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
			defer server.Close()

			serverURL, serverCleanup := tc.prepareTestServer(t)
			defer serverCleanup()

			def := NewMockShutdownCallbackDef(t)
			def.EXPECT().URL().Return(serverURL).Once()
			def.EXPECT().Method().Return(tc.method).Once()
			def.EXPECT().Headers().Return(map[string]string{testHeader: testHeaderValue}).Once()

			ctx, cancelFn := context.WithTimeout(context.Background(), testTimeout)
			defer cancelFn()

			log := logrus.New()
			hook := test.NewLocal(log)

			c := NewShutdownCallback(log, def)
			c.Run(ctx)

			entry := hook.LastEntry()
			e, errorFieldExists := entry.Data[logrus.ErrorKey]

			if tc.assertError != nil {
				require.True(t, errorFieldExists)

				err, ok := e.(error)
				require.True(t, ok)

				tc.assertError(t, err)

				return
			}

			assert.False(t, errorFieldExists)

			statusCode, ok := entry.Data["status-code"]
			require.True(t, ok)
			assert.Equal(t, testStatusCode, statusCode)

			status, ok := entry.Data["status"]
			require.True(t, ok)
			assert.Equal(t, testStatus, status)
		})
	}
}
