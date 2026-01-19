//go:build !integration

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
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
