package networks

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types/container"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

func TestCreateNetworkLabels(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	successfulJobResponse, err := common.GetRemoteSuccessfulBuild()
	require.NoError(t, err)

	manager := newDefaultManager()
	manager.build.JobResponse = successfulJobResponse
	manager.build.Variables = append(
		manager.build.Variables,
		common.JobVariable{Key: featureflags.NetworkPerBuild, Value: "true"},
		common.JobVariable{Key: "CI_PIPELINE_ID", Value: "1"},
	)

	client, err := docker.New(docker.Credentials{}, "")
	require.NoError(t, err, "should be able to connect to docker")
	defer client.Close()

	manager.client = client

	ctx := context.Background()

	networkMode, err := manager.Create(ctx, "")
	assert.NoError(t, err)
	assert.Equal(t, container.NetworkMode("runner-test-tok-project-0-concurrent-0-job-0-network"), networkMode)

	network, err := manager.Inspect(ctx)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"com.gitlab.gitlab-runner.job.before_sha":  "ca50079dac5293292f83a4d454922ba8db44e7a3",
		"com.gitlab.gitlab-runner.job.id":          "0",
		"com.gitlab.gitlab-runner.job.ref":         "master",
		"com.gitlab.gitlab-runner.job.sha":         "91956efe32fb7bef54f378d90c9bd74c19025872",
		"com.gitlab.gitlab-runner.pipeline.id":     "1",
		"com.gitlab.gitlab-runner.project.id":      "0",
		"com.gitlab.gitlab-runner.runner.id":       "test-tok",
		"com.gitlab.gitlab-runner.runner.local_id": "0",
	}, network.Labels)

	err = manager.Cleanup(ctx)
	assert.NoError(t, err)
}
