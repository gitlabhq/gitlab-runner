// Helper functions that are shared between unit tests and integration tests

package networks

import (
	mock "github.com/stretchr/testify/mock"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/labels"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

func newDebugLoggerMock() *mockDebugLogger {
	loggerMock := new(mockDebugLogger)
	loggerMock.On("Debugln", mock.Anything, mock.Anything)

	return loggerMock
}

func newDefaultManager() *manager {
	b := &common.Build{
		ProjectRunnerID: 0,
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{Token: "test-token"},
		},
		JobResponse: common.JobResponse{
			JobInfo: common.JobInfo{
				ProjectID: 0,
			},
		},
	}
	m := &manager{
		logger:  newDebugLoggerMock(),
		build:   b,
		labeler: labels.NewLabeler(b),
	}
	return m
}

// NewDefaultManagerForTest is used to expose a new manager to the integration tests package
func NewDefaultManagerForTest(c docker.Client, vars common.JobVariables, jobResponse common.JobResponse) Manager {
	m := newDefaultManager()
	m.client = c
	m.build.JobResponse = jobResponse
	m.build.Variables = append(m.build.Variables, vars...)

	return m
}
