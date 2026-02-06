//go:build integration

package docker_test

import (
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

func TestDockerLogOptions(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	tests := map[string]struct {
		skip          map[bool]string
		logOptions    map[string]string
		services      spec.Services
		expectedErrRE string
	}{
		"invalid key rejected early": {
			logOptions: map[string]string{
				"max-size": "10m",
			},
			expectedErrRE: "invalid log options: only \\[\"env\" \"labels\"] are allowed, but found: \\[\"max-size\"\\]",
		},
		"multiple invalid keys rejected early": {
			logOptions: map[string]string{
				"max-size":         "10m",
				"max-file":         "3",
				"invalid-option-1": "value1",
			},
			expectedErrRE: "invalid log options: only \\[\"env\" \"labels\"] are allowed, but found: \\[\"invalid-option-1\" \"max-file\" \"max-size\"\\]",
		},
		"valid env configuration": {
			logOptions: map[string]string{
				"env": "GITLAB_CI_JOB_ID,GITLAB_CI_JOB_NAME",
			},
		},
		"valid labels configuration": {
			logOptions: map[string]string{
				"labels": "com.gitlab.gitlab-runner.type",
			},
		},
		"valid env and labels configuration": {
			logOptions: map[string]string{
				"env":    "GITLAB_CI_JOB_ID,GITLAB_CI_JOB_NAME",
				"labels": "com.gitlab.gitlab-runner.type",
			},
		},
		"empty configuration": {
			logOptions: map[string]string{},
		},
		"service container with invalid options": {
			skip: map[bool]string{
				runtime.GOOS == "windows": "Service containers work differently on Windows",
			},
			logOptions: map[string]string{
				"max-size": "10m",
			},
			services: spec.Services{
				spec.Image{
					Name: common.TestAlpineImage,
				},
			},
			expectedErrRE: "invalid log options: only \\[\"env\" \"labels\"] are allowed, but found: \\[\"max-size\"\\]",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			// Check if test should be skipped
			for condition, reason := range test.skip {
				if condition {
					t.Skip(reason)
				}
			}

			build := getBuildForOS(t, common.GetSuccessfulBuild)
			build.Runner.Docker.LogOptions = test.logOptions

			// Configure services if specified
			if len(test.services) > 0 {
				build.Job.Services = test.services
			}

			build.Job.Variables = append(
				build.Job.Variables,
				spec.Variable{Key: "GIT_STRATEGY", Value: "none"},
			)

			err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})

			if test.expectedErrRE == "" {
				assert.NoError(t, err)
			} else {
				var eerr *common.BuildError
				assert.ErrorAs(t, err, &eerr)
				assert.Equal(t, common.RunnerSystemFailure, eerr.FailureReason)
				assert.Regexp(t, test.expectedErrRE, eerr.Inner.Error())
			}
		})
	}
}
