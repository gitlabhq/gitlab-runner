//go:build integration

package network

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

// recordedUpdate captures a single PUT /api/v4/jobs/:id body received by the
// fake coordinator, so the test can assert the exact state exchange that
// happens across the runner/GitLab boundary.
type recordedUpdate struct {
	state         string
	failureReason string
}

// TestCancelingJobReportsFinalState is an integration test for
// https://gitlab.com/gitlab-org/gitlab-runner/-/issues/39455.
//
// The existing commands/multi_test.go and commands/single_test.go tests mock
// both the network and the trace, so they only confirm that requestJob takes
// the new branch and calls Fail instead of Finish. They do not exercise the
// actual PUT exchange where the bug lives:
//
//	job pickup -> first PUT state=running -> server responds Job-Status: canceling
//	-> runner sends a follow-up PUT state=failed failure_reason=job_canceled.
//
// This test drives the real GitLabClient and the real clientJobTrace against an
// httptest server and asserts the full sequence for both branches.
func TestCancelingJobReportsFinalState(t *testing.T) {
	const jobID = 123

	tests := map[string]struct {
		firstUpdateStatus string
		firstUpdateCode   int
		finalize          func(c *clientJobTrace) error
		expectedUpdates   []recordedUpdate
	}{
		"canceling sends a final failed state": {
			firstUpdateStatus: statusCanceling,
			firstUpdateCode:   http.StatusOK,
			finalize: func(c *clientJobTrace) error {
				return c.Fail(common.ErrJobCanceled, common.JobFailureData{Reason: common.JobCanceled})
			},
			expectedUpdates: []recordedUpdate{
				{state: statusRunning},
				{state: statusFailed, failureReason: string(common.JobCanceled)},
			},
		},
		"abort (403) sends no final state": {
			firstUpdateCode: http.StatusForbidden,
			finalize: func(c *clientJobTrace) error {
				c.Finish()
				return nil
			},
			expectedUpdates: []recordedUpdate{
				{state: statusRunning},
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			var (
				mu       sync.Mutex
				updates  []recordedUpdate
				firstPUT = true
			)

			handler := func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add(correlationIDHeader, "foobar")

				require.Equal(t, http.MethodPut, r.Method)
				require.Equal(t, "/api/v4/jobs/123", r.URL.Path)

				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)

				var req map[string]interface{}
				require.NoError(t, json.Unmarshal(body, &req))

				rec := recordedUpdate{}
				if state, ok := req["state"].(string); ok {
					rec.state = state
				}
				if reason, ok := req["failure_reason"].(string); ok {
					rec.failureReason = reason
				}

				mu.Lock()
				updates = append(updates, rec)
				isFirst := firstPUT
				firstPUT = false
				mu.Unlock()

				if isFirst {
					if tt.firstUpdateStatus != "" {
						w.Header().Set("Job-Status", tt.firstUpdateStatus)
					}
					w.WriteHeader(tt.firstUpdateCode)
					return
				}

				// Final-state PUT: accept it so the runner does not retry.
				w.WriteHeader(http.StatusOK)
			}

			s := httptest.NewServer(http.HandlerFunc(handler))
			defer s.Close()

			config := common.RunnerConfig{
				RunnerCredentials: common.RunnerCredentials{URL: s.URL},
				SystemID:          "test-system-id",
			}
			jobCredentials := &common.JobCredentials{ID: jobID, Token: "token"}

			client := NewGitLabClient()

			trace, err := newJobTrace(client, config, jobCredentials, logrus.New())
			require.NoError(t, err)

			// ProcessJob calls trace.start(), moving the trace to Running so the
			// state guard in complete() does not block the final update. We mirror
			// that here by starting the trace explicitly.
			trace.start()

			// The initial state=running PUT sent by requestJob right after pickup.
			updateResult := client.UpdateJob(config, jobCredentials, common.UpdateJobInfo{
				ID:    jobCredentials.ID,
				State: common.Running,
			})

			if tt.firstUpdateStatus == statusCanceling {
				require.True(t, updateResult.CancelRequested, "server should request cancel")
				require.Equal(t, common.UpdateSucceeded, updateResult.State)
			} else {
				require.Equal(t, common.UpdateAbort, updateResult.State)
			}

			require.NoError(t, tt.finalize(trace))

			mu.Lock()
			defer mu.Unlock()
			assert.Equal(t, tt.expectedUpdates, updates)
		})
	}
}
