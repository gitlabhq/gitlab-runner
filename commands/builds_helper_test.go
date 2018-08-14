package commands

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/session"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

var fakeRunner = &common.RunnerConfig{
	RunnerCredentials: common.RunnerCredentials{
		Token: "a1b2c3d4e5f6",
	},
}

func TestBuildsHelperCollect(t *testing.T) {
	ch := make(chan prometheus.Metric, 50)
	b := &buildsHelper{}
	b.builds = append(b.builds, &common.Build{
		CurrentState: common.BuildRunStatePending,
		CurrentStage: common.BuildStagePrepare,
		Runner:       fakeRunner,
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

func TestBuildsHelperFindSessionByURL(t *testing.T) {
	sess, err := session.NewSession(nil)
	require.NoError(t, err)
	build := common.Build{
		Session: sess,
	}

	h := &buildsHelper{}
	h.addBuild(&build)

	foundSession := h.findSessionByURL(sess.Endpoint + "/action")
	assert.Equal(t, sess, foundSession)

	foundSession = h.findSessionByURL("/session/hash/action")
	assert.Nil(t, foundSession)
}

var testBuildCurrentID int

func getTestBuild() *common.Build {
	testBuildCurrentID++

	runner := common.RunnerConfig{}
	runner.Token = "a1b2c3d4"
	jobInfo := common.JobInfo{
		ProjectID: 1,
	}

	build := &common.Build{}
	build.ID = testBuildCurrentID
	build.Runner = &runner
	build.JobInfo = jobInfo

	return build
}
