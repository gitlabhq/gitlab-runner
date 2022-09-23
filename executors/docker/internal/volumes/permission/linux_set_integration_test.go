//go:build integration

package permission_test

import (
	"context"
	"fmt"
	"runtime"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/permission"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/wait"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

func TestDockerLinuxSetter(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping unix test on windows")
	}

	helpers.SkipIntegrationTests(t, "docker", "info")

	volumeName, setter, client, cleanup := setupTestDockerLinuxSetter(t)
	defer cleanup()

	err := setter.Set(context.Background(), volumeName, map[string]string{})
	assert.NoError(t, err)

	config := &container.Config{
		Image: common.TestAlpineNoRootImage,
		Cmd:   []string{"/bin/sh", "-c", "echo test > /test/test.txt"},
	}

	hostConfig := &container.HostConfig{
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
		Binds: []string{
			fmt.Sprintf("%s:/test", volumeName),
		},
	}

	uuid, err := helpers.GenerateRandomUUID(8)
	require.NoError(t, err)

	containerName := fmt.Sprintf("%s-set-permission-%s", volumeName, uuid)
	testContainer, err := client.ContainerCreate(context.Background(), config, hostConfig, nil, containerName)
	require.NoError(t, err)

	defer func() {
		_ = client.ContainerRemove(context.Background(), testContainer.ID, types.ContainerRemoveOptions{Force: true})
	}()

	err = client.ContainerStart(context.Background(), testContainer.ID, types.ContainerStartOptions{})
	require.NoError(t, err)

	waiter := wait.NewDockerKillWaiter(client)

	err = waiter.Wait(context.Background(), testContainer.ID)
	assert.NoError(t, err)
}

func setupTestDockerLinuxSetter(t *testing.T) (string, permission.Setter, docker.Client, func()) {
	client, err := docker.New(docker.Credentials{})
	require.NoError(t, err, "creating docker client")

	const helperImageRef = "gitlab/gitlab-runner-helper:x86_64-4c96e5ad"

	err = client.ImagePullBlocking(
		context.Background(),
		helperImageRef,
		types.ImagePullOptions{},
	)
	require.NoError(t, err)

	image, _, err := client.ImageInspectWithRaw(context.Background(), helperImageRef)
	require.NoError(t, err)

	debugLogger := logrus.New()
	debugLogger.Level = logrus.DebugLevel
	setter := permission.NewDockerLinuxSetter(client, debugLogger, &image)

	err = client.ImagePullBlocking(
		context.Background(),
		common.TestAlpineNoRootImage,
		types.ImagePullOptions{},
	)
	require.NoError(t, err)

	uuid, err := helpers.GenerateRandomUUID(8)
	require.NoError(t, err)

	volumeName := fmt.Sprintf("%s-%s", t.Name(), uuid)

	return volumeName,
		setter,
		client,
		func() {
			err := client.VolumeRemove(context.Background(), volumeName, true)
			require.NoError(t, err)

			err = client.Close()
			require.NoError(t, err)
		}
}
