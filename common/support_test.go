//go:build !integration

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

func TestResolveTestGitServer(t *testing.T) {
	tests := map[string]struct {
		raw      string
		expected string
		wantErr  bool
	}{
		"ip and port":       {raw: "http://10.0.0.5:8080", expected: "http://10.0.0.5:8080/"},
		"trailing slash":    {raw: "http://10.0.0.5:8080/", expected: "http://10.0.0.5:8080/"},
		"name is resolved":  {raw: "http://localhost:8080", expected: "http://127.0.0.1:8080/"},
		"no port":           {raw: "http://10.0.0.5", expected: "http://10.0.0.5/"},
		"not a url":         {raw: "git-server:8080", wantErr: true},
		"unresolvable name": {raw: "http://no-such-host.invalid:8080", wantErr: true},
		"empty":             {raw: "", wantErr: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := resolveTestGitServer(tc.raw)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestWithoutTestGitServer(t *testing.T) {
	job := spec.Job{Variables: spec.Variables{
		{Key: "CI_JOB_TOKEN", Value: "t"},
		{Key: "GIT_CONFIG_COUNT", Value: "2"},
		{Key: "GIT_CONFIG_KEY_0", Value: "url.http://10.0.0.5:8080/a.git.insteadOf"},
		{Key: "GIT_CONFIG_VALUE_0", Value: "https://gitlab.com/a.git"},
		{Key: "GIT_CONFIG_KEY_1", Value: "url.http://10.0.0.5:8080/b.git.insteadOf"},
		{Key: "GIT_CONFIG_VALUE_1", Value: "https://gitlab.com/b.git"},
		{Key: "GIT_STRATEGY", Value: "clone"},
	}}

	got := withoutTestGitServer(job)

	assert.Equal(t, spec.Variables{
		{Key: "CI_JOB_TOKEN", Value: "t"},
		{Key: "GIT_STRATEGY", Value: "clone"},
	}, got.Variables)
}

func TestTestGitServerRules(t *testing.T) {
	const (
		base = "http://10.0.0.5:8080/"
		prt  = "gitlab-org/ci-cd/gitlab-runner-pipeline-tests/"
	)

	// Exactly the mirrored repositories are redirected: the other gitlab.com
	// repositories the tests use must keep reaching gitlab.com.
	assert.Equal(t, spec.Variables{
		{Key: "GIT_CONFIG_COUNT", Value: "4"},
		{Key: "GIT_CONFIG_KEY_0", Value: "url." + base + prt + "gitlab-test.git.insteadOf"},
		{Key: "GIT_CONFIG_VALUE_0", Value: "https://gitlab.com/" + prt + "gitlab-test.git"},
		{Key: "GIT_CONFIG_KEY_1", Value: "url." + base + prt + "gitlab-grack.git.insteadOf"},
		{Key: "GIT_CONFIG_VALUE_1", Value: "https://gitlab.com/" + prt + "gitlab-grack.git"},
		{Key: "GIT_CONFIG_KEY_2", Value: "url." + base + prt + "simplegit.git.insteadOf"},
		{Key: "GIT_CONFIG_VALUE_2", Value: "https://gitlab.com/" + prt + "simplegit.git"},
		{Key: "GIT_CONFIG_KEY_3", Value: "url." + base + "gitlab-org/ci-cd/tests/lfs.git.insteadOf"},
		{Key: "GIT_CONFIG_VALUE_3", Value: "https://gitlab.com/gitlab-org/ci-cd/tests/lfs.git"},
	}, testGitServerRules(base))
}
