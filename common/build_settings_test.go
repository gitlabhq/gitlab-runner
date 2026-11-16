//go:build !integration

package common

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

// For https://gitlab.com/gitlab-org/gitlab-runner/-/issues/37386
func TestBuildVariablesAsFileType(t *testing.T) {
	tests := map[string]bool{
		"file vars":    true,
		"regular vars": false,
	}

	for name, isFileType := range tests {
		t.Run(name, func(t *testing.T) {
			vars := []spec.Variable{
				{Key: "DOCKER_AUTH_CONFIG", Value: "foobarbaz", File: isFileType},
				{Key: "GIT_CLONE_PATH", Value: "/root/dir/foobarbaz", File: isFileType},
				{Key: "GIT_SUBMODULE_STRATEGY", Value: "recursive", File: isFileType},
			}

			build := runSuccessfulMockBuild(t, func(options ExecutorPrepareOptions) error {
				options.Build.Variables = append(options.Build.Variables, vars...)
				return options.Build.StartBuild("/root/dir", "/cache/dir", true, false, false)
			})

			assert.Equal(t, "foobarbaz", build.Settings().DockerAuthConfig)
			assert.Equal(t, "/root/dir/foobarbaz", build.Settings().GitClonePath)
			assert.Equal(t, SubmoduleRecursive, build.Settings().GitSubmoduleStrategy)
		})
	}
}

func TestBuildSettings_ArtifactDirectDownload(t *testing.T) {
	const (
		ff     = featureflags.UseDirectDownload
		jobVar = "ARTIFACT_DIRECT_DOWNLOAD"
	)

	deprecated := ff + ": deprecated and will be removed in 19.9. Use direct_download in [runners.artifact] or the " + jobVar + " variable instead"
	ignoredForSetting := ff + ": ignored because direct_download is set in [runners.artifact]. The flag is deprecated and will be removed in 19.9"
	ignoredForJobVar := ff + ": ignored because " + jobVar + " is set. The flag is deprecated and will be removed in 19.9"

	tests := map[string]struct {
		setting       *bool
		configFlags   map[string]bool
		environment   []string
		jobVariables  spec.Variables
		expected      bool
		expectedError string
	}{
		"defaults to true": {
			expected: true,
		},
		"setting false": {
			setting:  new(false),
			expected: false,
		},
		"setting true overrides the flag in config.toml": {
			setting:       new(true),
			configFlags:   map[string]bool{ff: false},
			expected:      true,
			expectedError: ignoredForSetting,
		},
		"setting overrides the job variable": {
			setting:      new(false),
			jobVariables: spec.Variables{{Key: jobVar, Value: "true"}},
			expected:     false,
		},
		"setting overrides the flag as a job variable": {
			setting:       new(true),
			jobVariables:  spec.Variables{{Key: ff, Value: "false"}},
			expected:      true,
			expectedError: ignoredForSetting,
		},
		"flag false in config.toml": {
			configFlags:   map[string]bool{ff: false},
			expected:      false,
			expectedError: deprecated,
		},
		"flag in config.toml overrides the job variable": {
			configFlags:   map[string]bool{ff: false},
			jobVariables:  spec.Variables{{Key: jobVar, Value: "true"}},
			expected:      false,
			expectedError: deprecated,
		},
		"flag true in config.toml overrides the job variable": {
			configFlags:  map[string]bool{ff: true},
			jobVariables: spec.Variables{{Key: jobVar, Value: "false"}},
			expected:     true,
		},
		"job variable false": {
			jobVariables: spec.Variables{{Key: jobVar, Value: "false"}},
			expected:     false,
		},
		"job variable in the runner environment": {
			environment: []string{jobVar + "=false"},
			expected:    false,
		},
		"job variable overrides the flag as a job variable": {
			jobVariables: spec.Variables{
				{Key: ff, Value: "false"},
				{Key: jobVar, Value: "true"},
			},
			expected:      true,
			expectedError: ignoredForJobVar,
		},
		"invalid job variable is ignored": {
			jobVariables:  spec.Variables{{Key: jobVar, Value: "maybe"}},
			expected:      true,
			expectedError: jobVar + `: expected bool got "maybe", ignoring`,
		},
		"flag false as a job variable": {
			jobVariables:  spec.Variables{{Key: ff, Value: "false"}},
			expected:      false,
			expectedError: deprecated,
		},
		"flag false in the runner environment": {
			environment:   []string{ff + "=false"},
			expected:      false,
			expectedError: deprecated,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			build := &Build{
				Runner: &RunnerConfig{
					RunnerSettings: RunnerSettings{
						FeatureFlags: tc.configFlags,
						Environment:  tc.environment,
						Artifact:     ArtifactConfig{DirectDownload: tc.setting},
					},
				},
				Job: spec.Job{Variables: tc.jobVariables},
			}

			settings := build.Settings()
			assert.Equal(t, tc.expected, settings.ArtifactDirectDownload)
			assert.Equal(t, strconv.FormatBool(tc.expected), build.GetAllVariables().Value(ff))

			if tc.expectedError == "" {
				assert.Empty(t, settings.Errors)
				return
			}
			require.Len(t, settings.Errors, 1)
			assert.EqualError(t, settings.Errors[0], tc.expectedError)
		})
	}
}
