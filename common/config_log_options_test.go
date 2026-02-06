//go:build !integration

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDockerConfig_ValidateLogOptions(t *testing.T) {
	tests := []struct {
		name           string
		logOptions     map[string]string
		expectedErrMsg string
	}{
		{
			name: "nil config",
		},
		{
			name:       "empty log options",
			logOptions: map[string]string{},
		},
		{
			name: "valid env option",
			logOptions: map[string]string{
				"env": "GITLAB_CI_JOB_ID,GITLAB_CI_JOB_NAME",
			},
		},
		{
			name: "valid labels option",
			logOptions: map[string]string{
				"labels": "com.gitlab.gitlab-runner.type",
			},
		},
		{
			name: "valid env and labels options",
			logOptions: map[string]string{
				"env":    "GITLAB_CI_JOB_ID,GITLAB_CI_JOB_NAME",
				"labels": "com.gitlab.gitlab-runner.type",
			},
		},
		{
			name: "invalid single option",
			logOptions: map[string]string{
				"max-size": "10m",
			},
			expectedErrMsg: `invalid log options: only ["env" "labels"] are allowed, but found: ["max-size"]`,
		},
		{
			name: "invalid multiple options",
			logOptions: map[string]string{
				"max-size": "10m",
				"max-file": "3",
			},
			expectedErrMsg: `invalid log options: only ["env" "labels"] are allowed, but found: ["max-file" "max-size"]`,
		},
		{
			name: "mixed valid and invalid options",
			logOptions: map[string]string{
				"env":      "CI_JOB_ID",
				"max-size": "10m",
				"labels":   "job_name",
			},
			expectedErrMsg: `invalid log options: only ["env" "labels"] are allowed, but found: ["max-size"]`,
		},
		{
			name: "unknown option",
			logOptions: map[string]string{
				"unknown-option": "value",
			},
			expectedErrMsg: `invalid log options: only ["env" "labels"] are allowed, but found: ["unknown-option"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dockerConfig := &DockerConfig{
				LogOptions: tt.logOptions,
			}

			logConfig, err := dockerConfig.GetLogConfig()

			if tt.expectedErrMsg != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErrMsg)
			} else {
				assert.NoError(t, err)
				assertMapMatches(t, tt.logOptions, logConfig.Config)
			}
		})
	}
}

func assertMapMatches(t *testing.T, expected, actual map[string]string) {
	t.Helper()
	if len(expected) == 0 {
		assert.Len(t, actual, 0)
		return
	}
	assert.Equal(t, expected, actual)
}
