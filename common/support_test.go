//go:build !integration

package common

import (
	"errors"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
)

func TestResolveTestGitServer(t *testing.T) {
	lookup := func(addrs map[string][]string) func(string) ([]net.IP, error) {
		return func(host string) ([]net.IP, error) {
			found, ok := addrs[host]
			if !ok {
				return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
			}
			ips := make([]net.IP, 0, len(found))
			for _, a := range found {
				ips = append(ips, net.ParseIP(a))
			}
			return ips, nil
		}
	}
	dual := map[string][]string{"git-server": {"fd00::5", "10.0.0.5", "10.0.0.6"}}
	v6only := map[string][]string{"git-server": {"fd00::5"}}

	tests := map[string]struct {
		raw      string
		addrs    map[string][]string
		expected string
		wantErr  string
	}{
		"ip and port":           {raw: "http://10.0.0.5:8080", expected: "http://10.0.0.5:8080/"},
		"trailing slash":        {raw: "http://10.0.0.5:8080/", expected: "http://10.0.0.5:8080/"},
		"no port":               {raw: "http://10.0.0.5", expected: "http://10.0.0.5/"},
		"ipv6 literal":          {raw: "http://[::1]:8080", expected: "http://[::1]:8080/"},
		"ipv6 literal no port":  {raw: "http://[::1]", expected: "http://[::1]/"},
		"name takes first ipv4": {raw: "http://git-server:8080", addrs: dual, expected: "http://10.0.0.5:8080/"},
		"name without port":     {raw: "http://git-server", addrs: dual, expected: "http://10.0.0.5/"},
		"name with only ipv6":   {raw: "http://git-server:8080", addrs: v6only, wantErr: "no IPv4 address"},
		"unresolvable name":     {raw: "http://no-such-host:8080", addrs: dual, wantErr: "no such host"},
		"not a url":             {raw: "git-server:8080", wantErr: "is not a URL"},
		"empty":                 {raw: "", wantErr: "is not a URL"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := resolveTestGitServer(tc.raw, lookup(tc.addrs))
			if tc.wantErr != "" {
				assert.ErrorContains(t, err, tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestTestGitServerRules(t *testing.T) {
	const (
		base = "http://10.0.0.5:8080/"
		prt  = "gitlab-org/ci-cd/gitlab-runner-pipeline-tests/"
	)

	// Exactly the mirrored repositories are redirected: the other gitlab.com
	// repositories the tests use must keep reaching gitlab.com.
	assert.Equal(t, spec.Variables{
		{Key: "GIT_CONFIG_COUNT", Value: "5"},
		{Key: "GIT_CONFIG_KEY_0", Value: "url." + base + prt + "gitlab-test.git.insteadOf"},
		{Key: "GIT_CONFIG_VALUE_0", Value: "https://gitlab.com/" + prt + "gitlab-test.git"},
		{Key: "GIT_CONFIG_KEY_1", Value: "url." + base + prt + "gitlab-grack.git.insteadOf"},
		{Key: "GIT_CONFIG_VALUE_1", Value: "https://gitlab.com/" + prt + "gitlab-grack.git"},
		{Key: "GIT_CONFIG_KEY_2", Value: "url." + base + prt + "simplegit.git.insteadOf"},
		{Key: "GIT_CONFIG_VALUE_2", Value: "https://gitlab.com/" + prt + "simplegit.git"},
		{Key: "GIT_CONFIG_KEY_3", Value: "url." + base + "gitlab-org/ci-cd/tests/lfs.git.insteadOf"},
		{Key: "GIT_CONFIG_VALUE_3", Value: "https://gitlab.com/gitlab-org/ci-cd/tests/lfs.git"},
		{Key: "GIT_CONFIG_KEY_4", Value: "url." + base + "gitlab-org/ci-cd/tests/file-permissions.git.insteadOf"},
		{Key: "GIT_CONFIG_VALUE_4", Value: "https://gitlab.com/gitlab-org/ci-cd/tests/file-permissions.git"},
	}, testGitServerRules(base))
}

// fixedGitServer stands in for testGitServerVariables.
func fixedGitServer(vars spec.Variables, err error) gitServerVariablesFn {
	return func() (spec.Variables, error) { return vars, err }
}

func gitConfigVariables(vars spec.Variables) spec.Variables {
	var got spec.Variables
	for _, v := range vars {
		if strings.HasPrefix(v.Key, "GIT_CONFIG_") {
			got = append(got, v)
		}
	}
	return got
}

func TestRemoteFixturesCarryTestGitServerRedirect(t *testing.T) {
	const stepsYAML = "- name: hello\n  script: echo hello\n"

	rules := testGitServerRules("http://10.0.0.5:8080/")
	configured := fixedGitServer(rules, nil)
	unconfigured := fixedGitServer(nil, nil)
	broken := fixedGitServer(nil, errors.New("resolving \"git-server\": no such host"))

	t.Run("remote build", func(t *testing.T) {
		job, err := remoteBuildResponse(configured, []string{"echo"})
		require.NoError(t, err)
		assert.Equal(t, rules, gitConfigVariables(job.Variables))
		assert.Equal(t, repoRemoteURL, job.GitInfo.RepoURL, "the job itself still looks like a gitlab.com job")

		job, err = remoteBuildResponse(unconfigured, []string{"echo"})
		require.NoError(t, err)
		assert.Empty(t, gitConfigVariables(job.Variables))
	})

	t.Run("remote steps build", func(t *testing.T) {
		job, err := remoteStepsBuildResponse(configured, stepsYAML)
		require.NoError(t, err)
		assert.Equal(t, rules, gitConfigVariables(job.Variables))
		require.Len(t, job.Run, 1)

		job, err = remoteStepsBuildResponse(unconfigured, stepsYAML)
		require.NoError(t, err)
		assert.Empty(t, gitConfigVariables(job.Variables))
	})

	t.Run("resolution error reaches the caller", func(t *testing.T) {
		_, err := remoteBuildResponse(broken, []string{"echo"})
		assert.ErrorContains(t, err, "no such host")

		_, err = remoteStepsBuildResponse(broken, stepsYAML)
		assert.ErrorContains(t, err, "no such host")
	})

	t.Run("tls build is never redirected", func(t *testing.T) {
		job, err := getRemoteCustomTLSBuild("not-a-real-chain")
		require.NoError(t, err)
		assert.Empty(t, gitConfigVariables(job.Variables))
		assert.Equal(t, "not-a-real-chain", job.TLSData.CAChain)
	})
}

func TestLocalFixtureSource(t *testing.T) {
	const cloneURL = "/src/tmp/gitlab-test/.git"

	rules := testGitServerRules("http://10.0.0.5:8080/")
	configured := fixedGitServer(rules, nil)
	unconfigured := fixedGitServer(nil, nil)

	clone := func() (string, error) { return cloneURL, nil }
	noClone := func() (string, error) { return "", os.ErrNotExist }

	t.Run("git-server when configured", func(t *testing.T) {
		job, err := localBuildResponse(configured, clone, []string{"echo"})
		require.NoError(t, err)
		assert.Equal(t, repoRemoteURL, job.GitInfo.RepoURL)
		assert.Equal(t, rules, gitConfigVariables(job.Variables))
	})

	t.Run("local clone without a git-server", func(t *testing.T) {
		job, err := localBuildResponse(unconfigured, clone, []string{"echo"})
		require.NoError(t, err)
		assert.Equal(t, cloneURL, job.GitInfo.RepoURL)
		assert.Empty(t, gitConfigVariables(job.Variables))
	})

	t.Run("gitlab.com with neither", func(t *testing.T) {
		job, err := localBuildResponse(unconfigured, noClone, []string{"echo"})
		require.NoError(t, err)
		assert.Equal(t, repoRemoteURL, job.GitInfo.RepoURL)
		assert.Empty(t, gitConfigVariables(job.Variables))
	})

	t.Run("errors reach the caller", func(t *testing.T) {
		_, err := localBuildResponse(fixedGitServer(nil, errors.New("no such host")), clone, nil)
		assert.ErrorContains(t, err, "no such host")

		_, err = localBuildResponse(unconfigured, func() (string, error) { return "", errors.New("permission denied") }, nil)
		assert.ErrorContains(t, err, "permission denied")
	})
}
