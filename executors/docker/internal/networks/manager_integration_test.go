//go:build integration

package networks_test

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types/container"
	logrustest "github.com/sirupsen/logrus/hooks/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/labels"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/networks"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

func TestCreateNetworkLabels(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	successfulJobResponse, err := common.GetRemoteSuccessfulBuild()
	require.NoError(t, err)

	client, err := docker.New(docker.Credentials{})
	require.NoError(t, err, "should be able to connect to docker")
	defer client.Close()

	successfulJobResponse.GitInfo.RepoURL = "https://user:pass@gitlab.example.com/namespace/project.git"

	build := &common.Build{
		ProjectRunnerID: 0,
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{Token: "test-token"},
		},
		JobResponse: successfulJobResponse,
	}
	build.Variables = common.JobVariables{
		{Key: featureflags.NetworkPerBuild, Value: "true"},
		{Key: "CI_PIPELINE_ID", Value: "1"},
	}

	logger, _ := logrustest.NewNullLogger()

	manager := networks.NewManager(logger, client, build, labels.NewLabeler(build))

	ctx := context.Background()

	networkMode, err := manager.Create(ctx, "", false)
	assert.NoError(t, err)
	assert.Equal(t, container.NetworkMode("runner-test-tok-project-0-concurrent-0-job-0-network"), networkMode)

	network, err := manager.Inspect(ctx)
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"com.gitlab.gitlab-runner.job.before_sha":  "1ea27a9695f80d7816d9e8ce025d9b2df83d0dd7",
		"com.gitlab.gitlab-runner.job.id":          "0",
		"com.gitlab.gitlab-runner.job.url":         "https://gitlab.example.com/namespace/project/-/jobs/0",
		"com.gitlab.gitlab-runner.job.ref":         "main",
		"com.gitlab.gitlab-runner.job.sha":         "69b18e5ed3610cf646119c3e38f462c64ec462b7",
		"com.gitlab.gitlab-runner.managed":         "true",
		"com.gitlab.gitlab-runner.pipeline.id":     "1",
		"com.gitlab.gitlab-runner.project.id":      "0",
		"com.gitlab.gitlab-runner.runner.id":       "test-tok",
		"com.gitlab.gitlab-runner.runner.local_id": "0",
	}, network.Labels)

	err = manager.Cleanup(ctx)
	assert.NoError(t, err)
}
