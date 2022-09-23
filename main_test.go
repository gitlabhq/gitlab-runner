//go:build !integration || integration

package main_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const failure = `Environment variables from GitLab detected in tests,
these should be cleared: https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27965`

func TestEnvVariablesCleaned(t *testing.T) {
	assert.Empty(t, os.Getenv("CI_API_V4_URL"), failure)
	assert.NotEmpty(t, os.Getenv("CI"), "If running locally, use `export CI=0` explicitly.")
}
