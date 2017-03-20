package commands

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
)

func TestBuildsHelperCollect(t *testing.T) {
	ch := make(chan prometheus.Metric, 50)
	b := &buildsHelper{}
	b.builds = append(b.builds, &common.Build{
		CurrentState: common.BuildRunStatePending,
		CurrentStage: common.BuildStagePrepare,
	})
	b.Collect(ch)
	assert.Len(t, ch, 1)
}

func TestBuildsHelperAcquireRequestWithLimit(t *testing.T) {
	runner := common.RunnerConfig{
		RequestConcurrency: 2,
	}

	b := &buildsHelper{}
	result := b.acquireRequest(&runner)
	require.True(t, result)

	result = b.acquireRequest(&runner)
	require.True(t, result)

	result = b.acquireRequest(&runner)
	require.False(t, result, "allow only two requests")

	result = b.releaseRequest(&runner)
	require.True(t, result)

	result = b.releaseRequest(&runner)
	require.True(t, result)

	result = b.releaseRequest(&runner)
	require.False(t, result, "release only two requests")
}

func TestBuildsHelperAcquireRequestWithDefault(t *testing.T) {
	runner := common.RunnerConfig{
		RequestConcurrency: 0,
	}

	b := &buildsHelper{}
	result := b.acquireRequest(&runner)
	require.True(t, result)

	result = b.acquireRequest(&runner)
	require.False(t, result, "allow only one request")

	result = b.releaseRequest(&runner)
	require.True(t, result)

	result = b.releaseRequest(&runner)
	require.False(t, result, "release only one request")

	result = b.acquireRequest(&runner)
	require.True(t, result)

	result = b.releaseRequest(&runner)
	require.True(t, result)

	result = b.releaseRequest(&runner)
	require.False(t, result, "nothing to release")
}

func TestBuildsHelperAcquireBuildWithLimit(t *testing.T) {
	runner := common.RunnerConfig{
		Limit: 1,
	}

	b := &buildsHelper{}
	result := b.acquireBuild(&runner)
	require.True(t, result)

	result = b.acquireBuild(&runner)
	require.False(t, result, "allow only one build")

	result = b.releaseBuild(&runner)
	require.True(t, result)

	result = b.releaseBuild(&runner)
	require.False(t, result, "release only one build")
}

func TestBuildsHelperAcquireBuildUnlimited(t *testing.T) {
	runner := common.RunnerConfig{
		Limit: 0,
	}

	b := &buildsHelper{}
	result := b.acquireBuild(&runner)
	require.True(t, result)

	result = b.acquireBuild(&runner)
	require.True(t, result)

	result = b.releaseBuild(&runner)
	require.True(t, result)

	result = b.releaseBuild(&runner)
	require.True(t, result)
}
