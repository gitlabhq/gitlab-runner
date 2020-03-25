package docker_test

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

func TestDockerCommandSuccessRun(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:      common.TestAlpineImage,
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func TestDockerCommandUsingCustomClonePath(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	jobResponse, err := common.GetRemoteBuildResponse(
		"ls -al $CI_BUILDS_DIR/go/src/gitlab.com/gitlab-org/repo")
	require.NoError(t, err)

	tests := map[string]struct {
		clonePath         string
		expectedErrorType interface{}
	}{
		"uses custom clone path": {
			clonePath:         "$CI_BUILDS_DIR/go/src/gitlab.com/gitlab-org/repo",
			expectedErrorType: nil,
		},
		"path has to be within CI_BUILDS_DIR": {
			clonePath:         "/unknown/go/src/gitlab.com/gitlab-org/repo",
			expectedErrorType: &common.BuildError{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			build := &common.Build{
				JobResponse: jobResponse,
				Runner: &common.RunnerConfig{
					RunnerSettings: common.RunnerSettings{
						Executor: "docker",
						Docker: &common.DockerConfig{
							Image:      common.TestAlpineImage,
							PullPolicy: common.PullPolicyIfNotPresent,
						},
						Environment: []string{
							"GIT_CLONE_PATH=" + test.clonePath,
						},
					},
				},
			}

			err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
			assert.IsType(t, test.expectedErrorType, err)
		})
	}
}

func TestDockerCommandNoRootImage(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	successfulBuild, err := common.GetRemoteSuccessfulBuildWithDumpedVariables()

	assert.NoError(t, err)
	successfulBuild.Image.Name = common.TestAlpineNoRootImage
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func TestDockerCommandBuildFail(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	failedBuild, err := common.GetRemoteFailedBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: failedBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:      common.TestAlpineImage,
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err, "error")
	assert.IsType(t, err, &common.BuildError{})
	assert.Contains(t, err.Error(), "exit code 1")
}

func TestDockerCommandWithAllowedImagesRun(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	successfulBuild.Image = common.Image{Name: "$IMAGE_NAME"}
	successfulBuild.Variables = append(successfulBuild.Variables, common.JobVariable{
		Key:      "IMAGE_NAME",
		Value:    common.TestAlpineImage,
		Public:   true,
		Internal: false,
		File:     false,
	})
	successfulBuild.Services = append(successfulBuild.Services, common.Image{Name: common.TestDockerDindImage})
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					AllowedImages:   []string{common.TestAlpineImage},
					AllowedServices: []string{common.TestDockerDindImage},
					Privileged:      true,
					PullPolicy:      common.PullPolicyIfNotPresent,
				},
			},
		},
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func TestDockerCommandDisableEntrypointOverwrite(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	tests := []struct {
		name     string
		services bool
		disabled bool
	}{
		{
			name:     "Disabled - no services",
			disabled: true,
		},
		{
			name:     "Disabled - services",
			disabled: true,
			services: true,
		},
		{
			name: "Enabled - no services",
		},
		{
			name:     "Enabled - services",
			services: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			successfulBuild, err := common.GetRemoteSuccessfulBuild()
			require.NoError(t, err)

			successfulBuild.Image.Entrypoint = []string{"/bin/sh", "-c", "echo 'image overwritten'"}

			if test.services {
				successfulBuild.Services = common.Services{
					common.Image{
						Name:       common.TestDockerDindImage,
						Entrypoint: []string{"/bin/sh", "-c", "echo 'service overwritten'"},
					},
				}
			}

			build := &common.Build{
				JobResponse: successfulBuild,
				Runner: &common.RunnerConfig{
					RunnerSettings: common.RunnerSettings{
						Executor: "docker",
						Docker: &common.DockerConfig{
							Privileged:                 true,
							Image:                      common.TestAlpineImage,
							PullPolicy:                 common.PullPolicyIfNotPresent,
							DisableEntrypointOverwrite: test.disabled,
						},
					},
				},
			}

			var buffer bytes.Buffer
			err = build.Run(&common.Config{}, &common.Trace{Writer: &buffer})
			assert.NoError(t, err)
			out := buffer.String()
			if test.disabled {
				assert.NotContains(t, out, "image overwritten")
				assert.NotContains(t, out, "service overwritten")
				assert.Contains(t, out, "Entrypoint override disabled")
			} else {
				assert.Contains(t, out, "image overwritten")
				if test.services {
					assert.Contains(t, out, "service overwritten")
				}
			}
		})
	}
}

func isDockerOlderThan17_07(t *testing.T) bool {
	client, err := docker.New(docker.Credentials{}, "")
	require.NoError(t, err, "should be able to connect to docker")

	types, err := client.Info(context.Background())
	require.NoError(t, err, "should be able to get docker info")

	localVersion, err := version.NewVersion(types.ServerVersion)
	require.NoError(t, err)

	checkedVersion, err := version.NewVersion("17.07.0-ce")
	require.NoError(t, err)

	return localVersion.LessThan(checkedVersion)
}

func TestDockerCommandMissingImage(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	build := &common.Build{
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image: "some/non-existing/image",
				},
			},
		},
	}

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.IsType(t, &common.BuildError{}, err)

	contains := "repository does not exist"
	if isDockerOlderThan17_07(t) {
		contains = "not found"
	}

	assert.Contains(t, err.Error(), contains)
}

func TestDockerCommandMissingTag(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	build := &common.Build{
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image: "docker:missing-tag",
				},
			},
		},
	}

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.IsType(t, &common.BuildError{}, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestDockerCommandBuildAbort(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	longRunningBuild, err := common.GetRemoteLongRunningBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: longRunningBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:      common.TestAlpineImage,
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
		SystemInterrupt: make(chan os.Signal, 1),
	}

	abortTimer := time.AfterFunc(time.Second, func() {
		t.Log("Interrupt")
		build.SystemInterrupt <- os.Interrupt
	})
	defer abortTimer.Stop()

	timeoutTimer := time.AfterFunc(time.Minute, func() {
		t.Log("Timedout")
		t.FailNow()
	})
	defer timeoutTimer.Stop()

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "aborted: interrupt")
}

func TestDockerCommandBuildCancel(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	longRunningBuild, err := common.GetRemoteLongRunningBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: longRunningBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:      common.TestAlpineImage,
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
	}

	trace := &common.Trace{Writer: os.Stdout}

	abortTimer := time.AfterFunc(time.Second, func() {
		t.Log("Interrupt")
		trace.CancelFunc()
	})
	defer abortTimer.Stop()

	timeoutTimer := time.AfterFunc(time.Minute, func() {
		t.Log("Timedout")
		t.FailNow()
	})
	defer timeoutTimer.Stop()

	err = build.Run(&common.Config{}, trace)
	assert.IsType(t, err, &common.BuildError{})
	assert.Contains(t, err.Error(), "canceled")
}

func TestDockerCommandTwoServicesFromOneImage(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	tests := map[string]struct {
		variables common.JobVariables
	}{
		"bridge network": {
			variables: common.JobVariables{},
		},
		"network per build": {
			variables: common.JobVariables{
				{
					Key:   featureflags.NetworkPerBuild,
					Value: "true",
				},
			},
		},
	}

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	successfulBuild.Services = common.Services{
		{Name: common.TestAlpineImage, Alias: "service-1"},
		{Name: common.TestAlpineImage, Alias: "service-2"},
	}
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:      common.TestAlpineImage,
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var buffer bytes.Buffer

			build.Variables = tt.variables
			err = build.Run(&common.Config{}, &common.Trace{Writer: &buffer})
			assert.NoError(t, err)
			str := buffer.String()

			re, err := regexp.Compile("(?m)Conflict. The container name [^ ]+ is already in use by container")
			require.NoError(t, err)
			assert.NotRegexp(t, re, str, "Both service containers should be started and use different name")
		})
	}
}

func TestDockerCommandOutput(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:      common.TestAlpineImage,
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
	}

	var buffer bytes.Buffer

	err = build.Run(&common.Config{}, &common.Trace{Writer: &buffer})
	assert.NoError(t, err)

	re, err := regexp.Compile("(?m)^Initialized empty Git repository in /builds/gitlab-org/ci-cd/tests/gitlab-test/.git/")
	assert.NoError(t, err)
	assert.Regexp(t, re, buffer.String())
}

func TestDockerPrivilegedServiceAccessingBuildsFolder(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	commands := []string{
		"docker info",
		"docker run -v $(pwd):$(pwd) -w $(pwd) busybox touch test",
		"cat test",
	}

	strategies := []string{
		"fetch",
		"clone",
	}

	for _, strategy := range strategies {
		t.Log("Testing", strategy, "strategy...")
		longRunningBuild, err := common.GetRemoteLongRunningBuild()
		assert.NoError(t, err)
		build := &common.Build{
			JobResponse: longRunningBuild,
			Runner: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Executor: "docker",
					Docker: &common.DockerConfig{
						Image:      common.TestAlpineImage,
						PullPolicy: common.PullPolicyIfNotPresent,
						Privileged: true,
					},
				},
			},
		}
		build.Steps = common.Steps{
			common.Step{
				Name:         common.StepNameScript,
				Script:       common.StepScript(commands),
				When:         common.StepWhenOnSuccess,
				AllowFailure: false,
			},
		}
		build.Image.Name = common.TestDockerGitImage
		build.Services = common.Services{
			common.Image{
				Name: common.TestDockerDindImage,
			},
		}
		build.Variables = append(build.Variables, common.JobVariable{
			Key: "GIT_STRATEGY", Value: strategy,
		})

		err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
		assert.NoError(t, err)
	}
}

func getTestDockerJob(t *testing.T) *common.Build {
	commands := []string{
		"docker info",
	}

	longRunningBuild, err := common.GetRemoteLongRunningBuild()
	assert.NoError(t, err)

	build := &common.Build{
		JobResponse: longRunningBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:      common.TestAlpineImage,
					PullPolicy: common.PullPolicyIfNotPresent,
					Privileged: true,
				},
			},
		},
	}
	build.Steps = common.Steps{
		common.Step{
			Name:         common.StepNameScript,
			Script:       common.StepScript(commands),
			When:         common.StepWhenOnSuccess,
			AllowFailure: false,
		},
	}

	return build
}

func TestDockerExtendedConfigurationFromJob(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	examples := []struct {
		image     common.Image
		services  common.Services
		variables common.JobVariables
	}{
		{
			image: common.Image{
				Name:       "$IMAGE_NAME",
				Entrypoint: []string{"sh", "-c"},
			},
			services: common.Services{
				common.Image{
					Name:       "$SERVICE_NAME",
					Entrypoint: []string{"sh", "-c"},
					Command:    []string{"dockerd-entrypoint.sh"},
					Alias:      "my-docker-service",
				},
			},
			variables: common.JobVariables{
				{Key: "DOCKER_HOST", Value: "tcp://my-docker-service:2375"},
				{Key: "IMAGE_NAME", Value: common.TestDockerGitImage},
				{Key: "SERVICE_NAME", Value: common.TestDockerDindImage},
			},
		},
		{
			image: common.Image{
				Name: "$IMAGE_NAME",
			},
			services: common.Services{
				common.Image{
					Name: "$SERVICE_NAME",
				},
			},
			variables: common.JobVariables{
				{Key: "DOCKER_HOST", Value: "tcp://docker:2375"},
				{Key: "IMAGE_NAME", Value: common.TestDockerGitImage},
				{Key: "SERVICE_NAME", Value: common.TestDockerDindImage},
			},
		},
	}

	for exampleID, example := range examples {
		t.Run(fmt.Sprintf("example-%d", exampleID), func(t *testing.T) {
			build := getTestDockerJob(t)
			build.Image = example.image
			build.Services = example.services
			build.Variables = append(build.Variables, example.variables...)

			err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
			assert.NoError(t, err)
		})
	}
}

func runTestJobWithOutput(t *testing.T, build *common.Build) (output string) {
	var buffer bytes.Buffer

	err := build.Run(&common.Config{}, &common.Trace{Writer: &buffer})
	assert.NoError(t, err)

	output = buffer.String()
	return
}

func TestCacheInContainer(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	assert.NoError(t, err)

	successfulBuild.JobInfo.ProjectID = int(time.Now().Unix())
	successfulBuild.Steps[0].Script = common.StepScript{
		"(test -d cached/ && ls -lh cached/) || echo \"no cached directory\"",
		"(test -f cached/date && cat cached/date) || echo \"no cached date\"",
		"mkdir -p cached",
		"date > cached/date",
	}
	successfulBuild.Cache = common.Caches{
		common.Cache{
			Key:    "key",
			Paths:  common.ArtifactPaths{"cached/*"},
			Policy: common.CachePolicyPullPush,
		},
	}

	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:      common.TestAlpineImage,
					PullPolicy: common.PullPolicyIfNotPresent,
					Volumes:    []string{"/cache"},
				},
			},
		},
	}

	cacheNotPresentRE := regexp.MustCompile("(?m)^no cached directory")
	skipCacheDownload := "Not downloading cache key due to policy"
	skipCacheUpload := "Not uploading cache key due to policy"

	// The first job lacks any cache to pull, but tries to both pull and push
	output := runTestJobWithOutput(t, build)
	assert.Regexp(t, cacheNotPresentRE, output, "First job execution should not have cached data")
	assert.NotContains(t, output, skipCacheDownload, "Cache download should be performed with policy: %s", common.CachePolicyPullPush)
	assert.NotContains(t, output, skipCacheUpload, "Cache upload should be performed with policy: %s", common.CachePolicyPullPush)

	// pull-only jobs should skip the push step
	build.JobResponse.Cache[0].Policy = common.CachePolicyPull
	output = runTestJobWithOutput(t, build)
	assert.NotRegexp(t, cacheNotPresentRE, output, "Second job execution should have cached data")
	assert.NotContains(t, output, skipCacheDownload, "Cache download should be performed with policy: %s", common.CachePolicyPull)
	assert.Contains(t, output, skipCacheUpload, "Cache upload should be skipped with policy: %s", common.CachePolicyPull)

	// push-only jobs should skip the pull step
	build.JobResponse.Cache[0].Policy = common.CachePolicyPush
	output = runTestJobWithOutput(t, build)
	assert.Regexp(t, cacheNotPresentRE, output, "Third job execution should not have cached data")
	assert.Contains(t, output, skipCacheDownload, "Cache download be skipped with policy: push")
	assert.NotContains(t, output, skipCacheUpload, "Cache upload should be performed with policy: push")
}

func TestDockerImageNameFromVariable(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	successfulBuild.Variables = append(successfulBuild.Variables, common.JobVariable{
		Key:   "CI_REGISTRY_IMAGE",
		Value: common.TestAlpineImage,
	})
	successfulBuild.Image = common.Image{
		Name: "$CI_REGISTRY_IMAGE",
	}
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:           common.TestAlpineImage,
					PullPolicy:      common.PullPolicyIfNotPresent,
					AllowedServices: []string{common.TestAlpineImage},
				},
			},
		},
	}

	re := regexp.MustCompile("(?m)^ERROR: The [^ ]+ is not present on list of allowed images")

	output := runTestJobWithOutput(t, build)
	assert.NotRegexp(t, re, output, "Image's name should be expanded from variable")
}

func TestDockerServiceNameFromVariable(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	successfulBuild.Variables = append(successfulBuild.Variables, common.JobVariable{
		Key:   "CI_REGISTRY_IMAGE",
		Value: common.TestAlpineImage,
	})
	successfulBuild.Services = append(successfulBuild.Services, common.Image{
		Name: "$CI_REGISTRY_IMAGE",
	})
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:           common.TestAlpineImage,
					PullPolicy:      common.PullPolicyIfNotPresent,
					AllowedServices: []string{common.TestAlpineImage},
				},
			},
		},
	}

	re := regexp.MustCompile("(?m)^ERROR: The [^ ]+ is not present on list of allowed services")

	output := runTestJobWithOutput(t, build)
	assert.NotRegexp(t, re, output, "Service's name should be expanded from variable")
}

func runDockerInDocker(version string) (id string, err error) {
	cmd := exec.Command("docker", "run", "--detach", "--privileged", "-p", "2375", "docker:"+version+"-dind")
	cmd.Stderr = os.Stderr
	data, err := cmd.Output()
	if err != nil {
		return
	}
	id = strings.TrimSpace(string(data))
	return
}

func getDockerCredentials(id string) (credentials docker.Credentials, err error) {
	cmd := exec.Command("docker", "port", id, "2375")
	cmd.Stderr = os.Stderr
	data, err := cmd.Output()
	if err != nil {
		return
	}

	hostPort := strings.Split(strings.TrimSpace(string(data)), ":")
	if dockerHost, err := url.Parse(os.Getenv("DOCKER_HOST")); err == nil {
		dockerHostPort := strings.Split(dockerHost.Host, ":")
		hostPort[0] = dockerHostPort[0]
	} else if hostPort[0] == "0.0.0.0" {
		hostPort[0] = "localhost"
	}
	credentials.Host = "tcp://" + hostPort[0] + ":" + hostPort[1]
	return
}

func waitForDocker(credentials docker.Credentials) error {
	client, err := docker.New(credentials, "")
	if err != nil {
		return err
	}

	for i := 0; i < 20; i++ {
		_, err = client.Info(context.Background())
		if err == nil {
			break
		}
		time.Sleep(time.Second)
	}
	return err
}

func testDockerVersion(t *testing.T, version string) {
	t.Log("Running docker", version, "...")
	id, err := runDockerInDocker(version)
	if err != nil {
		t.Error("Docker run:", err)
		return
	}

	defer func() {
		exec.Command("docker", "rm", "-f", "-v", id).Run()
	}()

	t.Log("Getting address of", version, "...")
	credentials, err := getDockerCredentials(id)
	if err != nil {
		t.Error("Docker credentials:", err)
		return
	}

	t.Log("Connecting to", credentials.Host, "...")
	err = waitForDocker(credentials)
	if err != nil {
		t.Error("Wait for docker:", err)
		return
	}

	t.Log("Docker", version, "is running at", credentials.Host)

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:       common.TestAlpineImage,
					PullPolicy:  common.PullPolicyIfNotPresent,
					Credentials: credentials,
					CPUS:        "0.1",
				},
			},
		},
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func TestDocker1_8Compatibility(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}
	if os.Getenv("CI") != "" {
		t.Skip("This test doesn't work in nested dind")
		return
	}

	testDockerVersion(t, "1.8")
}

func TestDocker1_9Compatibility(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}
	if os.Getenv("CI") != "" {
		t.Skip("This test doesn't work in nested dind")
		return
	}

	testDockerVersion(t, "1.9")
}

func TestDocker1_10Compatibility(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}
	if os.Getenv("CI") != "" {
		t.Skip("This test doesn't work in nested dind")
		return
	}

	testDockerVersion(t, "1.10")
}

func TestDocker1_11Compatibility(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}
	if os.Getenv("CI") != "" {
		t.Skip("This test doesn't work in nested dind")
		return
	}

	testDockerVersion(t, "1.11")
}

func TestDocker1_12Compatibility(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}
	if os.Getenv("CI") != "" {
		t.Skip("This test doesn't work in nested dind")
		return
	}

	testDockerVersion(t, "1.12")
}

func TestDocker1_13Compatibility(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}
	if os.Getenv("CI") != "" {
		t.Skip("This test doesn't work in nested dind")
		return
	}

	testDockerVersion(t, "1.13")
}

func TestDockerCommandWithBrokenGitSSLCAInfo(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	successfulBuild, err := common.GetRemoteBrokenTLSBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{
				URL: "https://gitlab.com",
			},
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:      common.TestAlpineImage,
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
	}

	var buffer bytes.Buffer

	err = build.Run(&common.Config{}, &common.Trace{Writer: &buffer})
	assert.Error(t, err)
	out := buffer.String()
	assert.Contains(t, out, "Created fresh repository")
	assert.NotContains(t, out, "Updating/initializing submodules")
}

func TestDockerCommandWithGitSSLCAInfo(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	successfulBuild, err := common.GetRemoteGitLabComTLSBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{
				URL: "https://gitlab.com",
			},
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:      common.TestAlpineImage,
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
	}

	var buffer bytes.Buffer

	err = build.Run(&common.Config{}, &common.Trace{Writer: &buffer})
	assert.NoError(t, err)
	out := buffer.String()
	assert.Contains(t, out, "Created fresh repository")
	assert.Contains(t, out, "Updating/initializing submodules")
}

func TestDockerCommandWithHelperImageConfig(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	helperImageConfig := "gitlab/gitlab-runner-helper:x86_64-5a147c92"

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:       common.TestAlpineImage,
					HelperImage: helperImageConfig,
					PullPolicy:  common.PullPolicyIfNotPresent,
				},
			},
		},
	}

	var buffer bytes.Buffer
	err = build.Run(&common.Config{}, &common.Trace{Writer: &buffer})
	assert.NoError(t, err)
	out := buffer.String()
	assert.Contains(t, out, "Using docker image sha256:3cf24b1b62b6a4c55c5de43db4f50c0ff8b455238c836945d4b5c645411bfc77 for gitlab/gitlab-runner-helper:x86_64-5a147c92 ...")
}

func TestDockerCommandWithDoingPruneAndAfterScript(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	successfulBuild, err := common.GetRemoteSuccessfulBuildWithAfterScript()

	// This scripts removes self-created containers that do exit
	// It will fail if: cannot be removed, or no containers is found
	// It is assuming that name of each runner created container starts
	// with `runner-doprune-`
	successfulBuild.Steps[0].Script = common.StepScript{
		"docker ps -a -f status=exited | grep runner-doprune-",
		"docker rm $(docker ps -a -f status=exited | grep runner-doprune- | awk '{print $1}')",
	}

	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{
				Token: "doprune",
			},
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:      common.TestDockerGitImage,
					PullPolicy: common.PullPolicyIfNotPresent,
					Volumes: []string{
						"/var/run/docker.sock:/var/run/docker.sock",
					},
				},
			},
		},
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func TestDockerCommandUsingBuildsVolume(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	const buildsPath = "/builds"
	// the path is taken from `repoRemoteURL`
	const buildsGroupPath = "/builds/gitlab-org/ci-cd/tests"

	tests := map[string]struct {
		validPath   string
		invalidPath string
		variable    string
	}{
		"uses default state of FF_USE_LEGACY_BUILDS_DIR_FOR_DOCKER": {
			validPath:   buildsPath,
			invalidPath: buildsGroupPath,
			variable:    "",
		},
		"disables FF_USE_LEGACY_BUILDS_DIR_FOR_DOCKER": {
			validPath:   buildsPath,
			invalidPath: buildsGroupPath,
			variable:    "FF_USE_LEGACY_BUILDS_DIR_FOR_DOCKER=false",
		},
		"enables FF_USE_LEGACY_BUILDS_DIR_FOR_DOCKER": {
			validPath:   buildsGroupPath,
			invalidPath: buildsPath,
			variable:    "FF_USE_LEGACY_BUILDS_DIR_FOR_DOCKER=true",
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			jobResponse, err := common.GetRemoteBuildResponse(
				"mountpoint "+test.validPath,
				"! mountpoint "+test.invalidPath,
			)
			require.NoError(t, err)

			build := &common.Build{
				JobResponse: jobResponse,
				Runner: &common.RunnerConfig{
					RunnerSettings: common.RunnerSettings{
						Executor: "docker",
						Docker: &common.DockerConfig{
							Image:      common.TestAlpineImage,
							PullPolicy: common.PullPolicyIfNotPresent,
						},
						Environment: []string{
							"GIT_STRATEGY=none",
							test.variable,
						},
					},
				},
			}

			err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
			assert.NoError(t, err)
		})
	}
}
