//go:build integration

package docker_test

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/shells/shellstest"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/hashicorp/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	execDocker "gitlab.com/gitlab-org/gitlab-runner/executors/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/windows"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/test"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

var getWindowsImageOnce sync.Once
var windowsImage string
var systemIDState = common.NewSystemIDState()

var windowsDockerImageTagMappings = map[string]string{
	windows.V20H2: "20H2",
	windows.V21H1: "ltsc2022",
}

func TestMain(m *testing.M) {
	execDocker.PrebuiltImagesPaths = []string{"../../out/helper-images/"}
	_ = systemIDState.EnsureSystemID()

	os.Exit(m.Run())
}

// safeBuffer is used for tests that are writing build logs to a buffer and
// reading the build logs waiting for a log line.
type safeBuffer struct {
	buf *bytes.Buffer
	mu  sync.RWMutex
}

func newSafeBuffer() *safeBuffer {
	return &safeBuffer{
		buf: &bytes.Buffer{},
		mu:  sync.RWMutex{},
	}
}

func (s *safeBuffer) Read(p []byte) (n int, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buf.Read(p)
}

func (s *safeBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.buf.String()
}

func TestDockerCommandMultistepBuild(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	tests := map[string]struct {
		buildGetter    func() (common.JobResponse, error)
		expectedOutput []string
		unwantedOutput []string
		errExpected    bool
	}{
		"Successful build with release and after_script step": {
			buildGetter: common.GetRemoteSuccessfulMultistepBuild,
			expectedOutput: []string{
				"echo Hello World",
				"echo Release",
				"echo After Script",
			},
			errExpected: false,
		},
		"Failure on script step. Release is skipped. After script runs.": {
			buildGetter: func() (common.JobResponse, error) {
				return common.GetRemoteFailingMultistepBuild(common.StepNameScript)
			},
			expectedOutput: []string{
				"echo Hello World",
				"echo After Script",
			},
			unwantedOutput: []string{
				"echo Release",
			},
			errExpected: true,
		},
		"Failure on release step. After script runs.": {
			buildGetter: func() (common.JobResponse, error) {
				return common.GetRemoteFailingMultistepBuild("release")
			},
			expectedOutput: []string{
				"echo Hello World",
				"echo Release",
				"echo After Script",
			},
			errExpected: true,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			build := getBuildForOS(t, tt.buildGetter)

			var buf bytes.Buffer
			err := build.Run(&common.Config{}, &common.Trace{Writer: &buf})

			out := buf.String()
			for _, output := range tt.expectedOutput {
				assert.Contains(t, out, output)
			}

			for _, output := range tt.unwantedOutput {
				assert.NotContains(t, out, output)
			}

			if tt.errExpected {
				var buildErr *common.BuildError
				assert.ErrorAs(t, err, &buildErr)
				assert.Contains(t, err.Error(), "exit code 1")
				return
			}
			assert.NoError(t, err)
		})
	}
}

func getBuildForOS(t *testing.T, getJobResp func() (common.JobResponse, error)) common.Build {
	jobResp, err := getJobResp()
	require.NoError(t, err)

	return common.Build{
		JobResponse: jobResp,
		Runner:      getRunnerConfigForOS(t),
	}
}

func getRunnerConfigForOS(t *testing.T) *common.RunnerConfig {
	executor := "docker"
	image := common.TestAlpineImage
	shell := "bash"

	if runtime.GOOS == "windows" {
		executor = "docker-windows"
		shell = shells.SNPowershell
		image = getWindowsImage(t)
	}

	require.NoError(t, systemIDState.EnsureSystemID())

	return &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Executor: executor,
			Shell:    shell,
			Docker: &common.DockerConfig{
				Image:      image,
				PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
			},
			Cache: &common.CacheConfig{},
		},
		RunnerCredentials: common.RunnerCredentials{
			Token: fmt.Sprintf("%x", md5.Sum([]byte(t.Name()))),
		},
		SystemIDState: systemIDState,
	}
}

// windowsDockerImageTag checks the specified operatingSystem to see if it's one of the
// supported Windows version. If true, it maps the os version to the corresponding mcr.microsoft.com Docker image tag.
// UnsupportedWindowsVersionError is returned when no supported Windows version
// is found in the string.
func windowsDockerImageTag(operatingSystem string) (string, error) {
	version, err := windows.Version(operatingSystem)
	if err != nil {
		return "", err
	}

	dockerTag, ok := windowsDockerImageTagMappings[version]
	if !ok {
		dockerTag = version
	}

	return dockerTag, nil
}

func getWindowsImage(t *testing.T) string {
	getWindowsImageOnce.Do(func() {
		client, err := docker.New(docker.Credentials{})
		require.NoError(t, err, "creating docker client")
		defer client.Close()

		info, err := client.Info(context.Background())
		require.NoError(t, err, "docker info")

		dockerImageTag, err := windowsDockerImageTag(info.OperatingSystem)
		require.NoError(t, err)

		windowsImage = fmt.Sprintf(common.TestWindowsImage, dockerImageTag)
	})

	return windowsImage
}

func TestDockerCommandSuccessRunRawVariable(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	build := getBuildForOS(t, func() (common.JobResponse, error) {
		return common.GetRemoteBuildResponse("echo $TEST")
	})

	value := "$VARIABLE$WITH$DOLLARS$$"
	build.Variables = append(build.Variables, common.JobVariable{
		Key:   "TEST",
		Value: value,
		Raw:   true,
	})

	out, err := buildtest.RunBuildReturningOutput(t, &build)
	assert.NoError(t, err)
	assert.Contains(t, out, value)
}

func TestDockerCommandSuccessRunFileVariableContent(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	build := getBuildForOS(t, func() (common.JobResponse, error) {
		if runtime.GOOS == "windows" {
			return common.GetRemoteBuildResponse(`Get-Filehash -Algorithm SHA1 -Path $TEST`)
		}
		return common.GetRemoteBuildResponse(`sha1sum $TEST | tr "[a-z]" "[A-Z]"`)
	})

	value := "this is the content"
	build.Variables = append(build.Variables, common.JobVariable{
		Key:   "TEST",
		Value: value,
		File:  true,
		Raw:   true,
	})

	out, err := buildtest.RunBuildReturningOutput(t, &build)
	assert.NoError(t, err)
	assert.Contains(t, out, fmt.Sprintf("%X", sha1.Sum([]byte(value))))
}

func TestBuildScriptSections(t *testing.T) {
	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		if shell == "cmd" || shell == "pwsh" || shell == "powershell" {
			// support for pwsh and powershell tracked in https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28119
			t.Skip("CMD, pwsh, powershell not supported")
		}

		build := getBuildForOS(t, func() (common.JobResponse, error) {
			return common.GetRemoteSuccessfulBuild()
		})

		build.Runner.RunnerSettings.Shell = shell

		buildtest.RunBuildWithSections(t, &build)
	})
}

func TestDockerCommandUsingCustomClonePath(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	remoteBuild := func() (common.JobResponse, error) {
		cmd := "ls -al $CI_BUILDS_DIR/go/src/gitlab.com/gitlab-org/repo"
		if runtime.GOOS == "windows" {
			cmd = "Get-Item -Path $CI_BUILDS_DIR/go/src/gitlab.com/gitlab-org/repo"
		}

		return common.GetRemoteBuildResponse(cmd)
	}

	tests := map[string]struct {
		clonePath   string
		expectedErr bool
	}{
		"uses custom clone path": {
			clonePath:   "$CI_BUILDS_DIR/go/src/gitlab.com/gitlab-org/repo",
			expectedErr: false,
		},
		"path has to be within CI_BUILDS_DIR": {
			clonePath:   "/unknown/go/src/gitlab.com/gitlab-org/repo",
			expectedErr: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			build := getBuildForOS(t, remoteBuild)
			build.Runner.Environment = []string{
				"GIT_CLONE_PATH=" + test.clonePath,
			}

			err := buildtest.RunBuild(t, &build)
			if test.expectedErr {
				var buildErr *common.BuildError
				assert.ErrorAs(t, err, &buildErr)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func TestDockerCommandNoRootImage(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	successfulBuild, err := common.GetRemoteSuccessfulBuildWithDumpedVariables()

	assert.NoError(t, err)
	successfulBuild.Image.Name = common.TestAlpineNoRootImage
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
				},
			},
			SystemIDState: systemIDState,
		},
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func TestDockerCommandEntrypointWithStderrOutput(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	resp, err := common.GetRemoteSuccessfulBuild()
	assert.NoError(t, err)

	resp.Image.Name = common.TestAlpineEntrypointStderrImage
	build := &common.Build{
		JobResponse: resp,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
				},
				FeatureFlags: map[string]bool{
					featureflags.DisableUmaskForDockerExecutor: true,
				},
			},
			SystemIDState: systemIDState,
		},
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func TestDockerCommandOwnershipOverflow(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	resp, err := common.GetRemoteSuccessfulBuild()
	assert.NoError(t, err)

	resp.Image.Name = common.TestAlpineIDOverflowImage
	build := &common.Build{
		JobResponse: resp,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
				},
				FeatureFlags: map[string]bool{
					featureflags.DisableUmaskForDockerExecutor: true,
				},
			},
			SystemIDState: systemIDState,
		},
	}

	trace := &common.Trace{Writer: os.Stdout}
	timeoutTimer := time.AfterFunc(time.Minute, func() {
		trace.Abort()
	})
	defer timeoutTimer.Stop()

	err = build.Run(&common.Config{}, trace)
	assert.Error(t, err)

	// error is only canceled if it timed out, something that will only happen
	// if data from the overflow isn't safely limited.
	assert.NotErrorIs(t, err, &common.BuildError{FailureReason: common.JobCanceled})
}

func TestDockerCommandWithAllowedImagesRun(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

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
					PullPolicy:      common.StringOrArray{common.PullPolicyIfNotPresent},
				},
			},
			SystemIDState: systemIDState,
		},
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func TestDockerCommandDisableEntrypointOverwrite(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

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
							PullPolicy:                 common.StringOrArray{common.PullPolicyIfNotPresent},
							DisableEntrypointOverwrite: test.disabled,
						},
					},
					SystemIDState: systemIDState,
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
	client, err := docker.New(docker.Credentials{})
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
	helpers.SkipIntegrationTests(t, "docker", "info")

	build := getBuildForOS(t, common.GetSuccessfulBuild)
	build.Runner.Docker.Image = "some/non-existing/image"

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.ErrorIs(t, err, &common.BuildError{FailureReason: common.ScriptFailure})

	contains := "repository does not exist"
	if isDockerOlderThan17_07(t) {
		contains = "not found"
	}

	assert.Contains(t, err.Error(), contains)
}

func TestDockerCommandMissingTag(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	build := getBuildForOS(t, common.GetSuccessfulBuild)
	build.Runner.Docker.Image = "docker:missing-tag"

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.ErrorIs(t, err, &common.BuildError{FailureReason: common.ScriptFailure})
	assert.Contains(t, err.Error(), "not found")
}

func TestDockerCommandMissingServiceImage(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	build := getBuildForOS(t, common.GetSuccessfulBuild)
	build.Services = common.Services{
		{
			Name: "some/non-existing/image",
		},
	}

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.ErrorIs(t, err, &common.BuildError{FailureReason: common.ScriptFailure})

	contains := "repository does not exist"
	if isDockerOlderThan17_07(t) {
		contains = "not found"
	}

	assert.Contains(t, err.Error(), contains)
}

// TestDockerCommandPullingImageNoHost tests if the DNS resolution failure for the registry host
// is categorized as a script failure.
func TestDockerCommandPullingImageNoHost(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	build := getBuildForOS(t, common.GetSuccessfulBuild)
	build.Runner.RunnerSettings.Docker.Image = "docker.repo.example.com/docker:18.09.7-dind"

	var buildError *common.BuildError
	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.ErrorAs(t, err, &buildError)

	assert.Equal(t, common.ScriptFailure, buildError.FailureReason, "expected script failure error")
}

func TestDockerCommandBuildCancel(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	buildtest.RunBuildWithCancel(t, getRunnerConfigForOS(t), nil)
}

func TestBuildMasking(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	buildtest.RunBuildWithMasking(t, getRunnerConfigForOS(t), nil)
}

func TestBuildExpandedFileVariable(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		build := getBuildForOS(t, common.GetSuccessfulBuild)
		buildtest.RunBuildWithExpandedFileVariable(t, build.Runner, nil)
	})
}

func TestDockerCommandTwoServicesFromOneImage(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

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
					PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
				},
			},
			SystemIDState: systemIDState,
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
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:      common.TestAlpineImage,
					PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
				},
			},
			SystemIDState: systemIDState,
		},
	}

	var buffer bytes.Buffer

	err = build.Run(&common.Config{}, &common.Trace{Writer: &buffer})
	assert.NoError(t, err)

	re, err :=
		regexp.Compile("(?m)^Initialized empty Git repository in /builds/gitlab-org/ci-cd/gitlab-runner-pipeline-tests/gitlab-test/.git/")
	require.NoError(t, err)
	assert.Regexp(t, re, buffer.String())
}

func TestDockerPrivilegedServiceAccessingBuildsFolder(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

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
						PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
						Privileged: true,
					},
				},
				SystemIDState: systemIDState,
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
					PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
					Privileged: true,
				},
			},
			SystemIDState: systemIDState,
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
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

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

func TestCacheInContainer(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	assert.NoError(t, err)

	successfulBuild.JobInfo.ProjectID = time.Now().Unix()
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
			When:   common.CacheWhenOnSuccess,
		},
	}

	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:      common.TestAlpineImage,
					PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
					Volumes:    []string{"/cache"},
				},
				Cache: &common.CacheConfig{},
			},
			SystemIDState: systemIDState,
		},
	}

	cacheNotPresentRE := regexp.MustCompile("(?m)^no cached directory")
	skipCacheDownload := "Not downloading cache key due to policy"
	skipCacheUpload := "Not uploading cache key due to policy"

	// The first job lacks any cache to pull, but tries to both pull and push
	output, err := buildtest.RunBuildReturningOutput(t, build)
	require.NoError(t, err)
	assert.Regexp(t, cacheNotPresentRE, output, "First job execution should not have cached data")
	assert.NotContains(
		t,
		output,
		skipCacheDownload,
		"Cache download should be performed with policy: %s",
		common.CachePolicyPullPush,
	)
	assert.NotContains(
		t,
		output,
		skipCacheUpload,
		"Cache upload should be performed with policy: %s",
		common.CachePolicyPullPush,
	)

	// pull-only jobs should skip the push step
	build.JobResponse.Cache[0].Policy = common.CachePolicyPull
	output, err = buildtest.RunBuildReturningOutput(t, build)
	require.NoError(t, err)
	assert.NotRegexp(t, cacheNotPresentRE, output, "Second job execution should have cached data")
	assert.NotContains(
		t,
		output,
		skipCacheDownload,
		"Cache download should be performed with policy: %s",
		common.CachePolicyPull,
	)
	assert.Contains(
		t,
		output,
		skipCacheUpload,
		"Cache upload should be skipped with policy: %s",
		common.CachePolicyPull,
	)

	// push-only jobs should skip the pull step
	build.JobResponse.Cache[0].Policy = common.CachePolicyPush
	output, err = buildtest.RunBuildReturningOutput(t, build)
	require.NoError(t, err)
	assert.Regexp(t, cacheNotPresentRE, output, "Third job execution should not have cached data")
	assert.Contains(t, output, skipCacheDownload, "Cache download be skipped with policy: push")
	assert.NotContains(t, output, skipCacheUpload, "Cache upload should be performed with policy: push")

	// For failed job it should push cache as well.
	build.JobResponse.Cache[0].Policy = common.CachePolicyPullPush
	build.JobResponse.Cache[0].When = common.CacheWhenAlways
	build.JobResponse.Steps[0].Script = append(build.JobResponse.Steps[0].Script, "exit 1")
	output, err = buildtest.RunBuildReturningOutput(t, build)
	require.Error(t, err)
	assert.NotRegexp(t, cacheNotPresentRE, output, "Second job execution should have cached data")
	assert.Contains(t, output, "Saving cache for failed job")
	assert.Contains(t, output, "Created cache")
}

func TestDockerImageNameFromVariable(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

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
					PullPolicy:      common.StringOrArray{common.PullPolicyIfNotPresent},
					AllowedServices: []string{common.TestAlpineImage},
				},
			},
			SystemIDState: systemIDState,
		},
	}

	re := regexp.MustCompile("(?m)^ERROR: The [^ ]+ is not present on list of allowed images")

	output, err := buildtest.RunBuildReturningOutput(t, build)
	require.NoError(t, err)
	assert.NotRegexp(t, re, output, "Image's name should be expanded from variable")
}

func TestDockerServiceNameFromVariable(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

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
					PullPolicy:      common.StringOrArray{common.PullPolicyIfNotPresent},
					AllowedServices: []string{common.TestAlpineImage},
				},
			},
			SystemIDState: systemIDState,
		},
	}

	re := regexp.MustCompile("(?m)^ERROR: The [^ ]+ is not present on list of allowed services")

	output, err := buildtest.RunBuildReturningOutput(t, build)
	require.NoError(t, err)
	assert.NotRegexp(t, re, output, "Service's name should be expanded from variable")
}

func TestDockerServiceHealthcheck(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	tests := map[string]struct {
		command         []string
		serviceStarted  bool
		networkPerBuild string
		skip            bool
	}{
		"successful service (FF_NETWORK_PER_BUILD=false)": {
			command:         []string{"server"},
			serviceStarted:  true,
			networkPerBuild: "false",
			skip:            runtime.GOOS == "windows",
		},
		"successful service (FF_NETWORK_PER_BUILD=true)": {
			command:         []string{"server"},
			serviceStarted:  true,
			networkPerBuild: "true",
			skip:            false,
		},
		"failed service (FF_NETWORK_PER_BUILD=false)": {
			command:         []string{"server", "--addr", ":8888"},
			serviceStarted:  false,
			networkPerBuild: "false",
			skip:            runtime.GOOS == "windows",
		},
		"failed service (FF_NETWORK_PER_BUILD=true)": {
			command:         []string{"server", "--addr", ":8888"},
			serviceStarted:  false,
			networkPerBuild: "true",
			skip:            false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if tc.skip {
				t.Skipf("OS %q does not support 'link' networking", runtime.GOOS)
			}

			resp, err := common.GetRemoteBuildResponse(
				"liveness client db",
				"liveness client registry.gitlab.com__gitlab-org__ci-cd__tests__liveness",
				"liveness client registry.gitlab.com-gitlab-org-ci-cd-tests-liveness",
			)
			require.NoError(t, err)

			build := common.Build{
				JobResponse: resp,
				Runner: &common.RunnerConfig{
					RunnerSettings: common.RunnerSettings{
						Executor: "docker",
						Docker:   &common.DockerConfig{},
					},
					SystemIDState: systemIDState,
				},
			}

			build.Image = common.Image{
				Name:       common.TestLivenessImage,
				Entrypoint: []string{"sh", "-c"},
			}

			if runtime.GOOS == "windows" {
				build.Runner.RunnerSettings.Shell = shells.SNPwsh
				build.Runner.RunnerSettings.Executor = "docker-windows"
				build.Image.Entrypoint = []string{""}
			}

			build.Services = append(build.Services, common.Image{
				Name:    common.TestLivenessImage,
				Alias:   "db",
				Command: tc.command,
			})

			build.Variables = append(build.Variables, common.JobVariable{
				Key:    "FF_NETWORK_PER_BUILD",
				Value:  tc.networkPerBuild,
				Public: true,
			})

			out, err := buildtest.RunBuildReturningOutput(t, &build)
			if !tc.serviceStarted {
				assert.Error(t, err)
				assert.Contains(t, out, "probably didn't start properly")
				return
			}

			assert.NoError(t, err)
			assert.NotContains(t, out, "probably didn't start properly")
		})
	}
}

func TestDockerServiceHealthcheckOverflow(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	resp, err := common.GetRemoteSuccessfulBuild()
	assert.NoError(t, err)

	build := &common.Build{
		JobResponse: resp,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker:   &common.DockerConfig{},
			},
			SystemIDState: systemIDState,
		},
	}

	build.Image = common.Image{
		Name: common.TestAlpineImage,
	}

	build.Services = append(build.Services, common.Image{
		Name:    common.TestAlpineImage,
		Command: []string{"printf", "datastart: %" + strconv.Itoa(execDocker.ServiceLogOutputLimit) + "s", ":dataend"},
	})

	build.Variables = append(build.Variables, common.JobVariable{
		Key:    "FF_NETWORK_PER_BUILD",
		Value:  "true",
		Public: true,
	})

	out, err := buildtest.RunBuildReturningOutput(t, build)
	assert.NoError(t, err)
	assert.Contains(t, out, "datastart:")
	assert.NotContains(t, out, ":dataend")
}

func TestDockerHandlesAliasDuplicates(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	resp, err := common.GetRemoteBuildResponse("ping -c 1 alpine && ping -c 1 svc-1")
	assert.NoError(t, err)

	build := &common.Build{
		JobResponse: resp,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker:   &common.DockerConfig{},
			},
			SystemIDState: systemIDState,
		},
	}

	build.Image = common.Image{
		Name: common.TestAlpineImage,
	}

	build.Services = append(build.Services, common.Image{
		Name:    common.TestAlpineImage,
		Command: []string{"sleep", "5"},
		Alias:   "alpine alpine svc-1 svc-1",
	})

	build.Variables = append(build.Variables, common.JobVariable{
		Key:    "FF_NETWORK_PER_BUILD",
		Value:  "true",
		Public: true,
	})

	out, err := buildtest.RunBuildReturningOutput(t, build)
	assert.NoError(t, err)
	assert.Contains(t, out, "PING alpine")
	assert.Contains(t, out, "PING svc-1")
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
	client, err := docker.New(credentials)
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
		_ = exec.Command("docker", "rm", "-f", "-v", id).Run()
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
					PullPolicy:  common.StringOrArray{common.PullPolicyIfNotPresent},
					Credentials: credentials,
					CPUS:        "0.1",
				},
			},
			SystemIDState: systemIDState,
		},
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func TestDocker1_8Compatibility(t *testing.T) {
	test.SkipIfGitLabCIWithMessage(t, "This test doesn't work in nested dind")
	helpers.SkipIntegrationTests(t, "docker", "info")

	testDockerVersion(t, "1.8")
}

func TestDocker1_9Compatibility(t *testing.T) {
	test.SkipIfGitLabCIWithMessage(t, "This test doesn't work in nested dind")
	helpers.SkipIntegrationTests(t, "docker", "info")

	testDockerVersion(t, "1.9")
}

func TestDocker1_10Compatibility(t *testing.T) {
	test.SkipIfGitLabCIWithMessage(t, "This test doesn't work in nested dind")
	helpers.SkipIntegrationTests(t, "docker", "info")

	testDockerVersion(t, "1.10")
}

func TestDocker1_11Compatibility(t *testing.T) {
	test.SkipIfGitLabCIWithMessage(t, "This test doesn't work in nested dind")
	helpers.SkipIntegrationTests(t, "docker", "info")

	testDockerVersion(t, "1.11")
}

func TestDocker1_12Compatibility(t *testing.T) {
	test.SkipIfGitLabCIWithMessage(t, "This test doesn't work in nested dind")
	helpers.SkipIntegrationTests(t, "docker", "info")

	testDockerVersion(t, "1.12")
}

func TestDocker1_13Compatibility(t *testing.T) {
	test.SkipIfGitLabCIWithMessage(t, "This test doesn't work in nested dind")
	helpers.SkipIntegrationTests(t, "docker", "info")

	testDockerVersion(t, "1.13")
}

func TestDockerCommandWithBrokenGitSSLCAInfo(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

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
					PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
				},
			},
			SystemIDState: systemIDState,
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
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

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
					PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
				},
			},
			SystemIDState: systemIDState,
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
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

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
					PullPolicy:  common.StringOrArray{common.PullPolicyIfNotPresent},
				},
			},
			SystemIDState: systemIDState,
		},
	}

	var buffer bytes.Buffer
	err = build.Run(&common.Config{}, &common.Trace{Writer: &buffer})
	assert.NoError(t, err)
	out := buffer.String()
	assert.Contains(
		t,
		out,
		"Using docker image sha256:3cf24b1b62b6a4c55c5de43db4f50c0ff8b455238c836945d4b5c645411bfc77 for "+
			"gitlab/gitlab-runner-helper:x86_64-5a147c92 with digest "+
			"gitlab/gitlab-runner-helper@sha256:836120f351e800cd483402a5910b0a347f9e32e97ac3e94e67e2c005a289cc4c ...",
	)
}

func TestDockerCommand_Pwsh(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	build := getBuildForOS(t, common.GetRemoteSuccessfulBuild)
	build.Image.Name = common.TestPwshImage
	build.Runner.Shell = shells.SNPwsh
	build.JobResponse.Steps = common.Steps{
		common.Step{
			Name: common.StepNameScript,
			Script: []string{
				"Write-Output $PSVersionTable",
			},
			Timeout:      120,
			When:         common.StepWhenAlways,
			AllowFailure: false,
		},
	}

	out, err := buildtest.RunBuildReturningOutput(t, &build)
	assert.NoError(t, err)
	assert.Regexp(t, `PSVersion\s+7.1.1`, out)
	assert.Regexp(t, `PSEdition\s+Core`, out)
}

func TestDockerCommandWithDoingPruneAndAfterScript(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

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
					PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
					Volumes: []string{
						"/var/run/docker.sock:/var/run/docker.sock",
					},
				},
			},
			SystemIDState: systemIDState,
		},
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func TestDockerCommandRunAttempts(t *testing.T) {
	t.Skip("Skipping until https://gitlab.com/gitlab-org/gitlab-runner/-/issues/25385 is resolved.")

	helpers.SkipIntegrationTests(t, "docker", "info")

	sleepCMD := "sleep 60"
	executorStageAttempts := 2

	build := getBuildForOS(t, common.GetRemoteSuccessfulBuild)
	build.Runner.RunnerCredentials.Token = "misscont"
	build.JobResponse.Steps = common.Steps{
		common.Step{
			Name: common.StepNameScript,
			Script: []string{
				sleepCMD,
			},
			Timeout:      120,
			When:         common.StepWhenAlways,
			AllowFailure: false,
		},
	}
	build.JobResponse.Variables = append(build.JobResponse.Variables, common.JobVariable{
		Key:    common.ExecutorJobSectionAttempts,
		Value:  strconv.Itoa(executorStageAttempts),
		Public: true,
	})

	trace := newSafeBuffer()

	runFinished := make(chan struct{})
	go func() {
		err := build.Run(&common.Config{}, &common.Trace{Writer: io.MultiWriter(trace, os.Stdout)})
		// Only make sure that the build failed. Docker can return different
		// kind of errors when a container is removed for example exit code 137,
		// there is no guarantee on what failure is returned.
		assert.Error(t, err)
		close(runFinished)
	}()

	// Waiting until we reach the first sleep command in the build.
	for {
		if !strings.Contains(trace.String(), sleepCMD) {
			time.Sleep(time.Second)
			continue
		}

		break
	}

	attempts := 0
	for i := 0; i < executorStageAttempts; i++ {
		assertFailedToInspectContainer(t, trace, &attempts)
	}

	assert.Equal(
		t,
		executorStageAttempts,
		attempts,
		"The %s stage should be retried at least once",
		"step_script",
	)
	<-runFinished
}

func assertFailedToInspectContainer(t *testing.T, trace *safeBuffer, attempts *int) {
	// If there is already an exit code, return early since a new container will
	// never be scheduled.
	if strings.Contains(trace.String(), "exit code") {
		return
	}

	containerID := <-removeBuildContainer(t)
	for {
		if !strings.Contains(trace.String(), fmt.Sprintf("Container %q not found or removed", containerID)) {
			time.Sleep(time.Second)

			continue
		}

		*attempts++
		break
	}
}

func removeBuildContainer(t *testing.T) <-chan string {
	removedContainer := make(chan string, 1)
	defer close(removedContainer)

	client, err := docker.New(docker.Credentials{})
	require.NoError(t, err, "creating docker client")
	defer client.Close()

	var list []types.Container
	// Keep checking containers until we get the container that we want.
	for len(list) == 0 {
		time.Sleep(time.Second)
		nameFilter := filters.Arg("name", "misscont")
		containerList := types.ContainerListOptions{
			Filters: filters.NewArgs(nameFilter),
		}
		list, err = client.ContainerList(context.Background(), containerList)
		require.NoError(t, err)
	}

	for _, ctr := range list {
		err := client.ContainerRemove(context.Background(), ctr.ID, types.ContainerRemoveOptions{Force: true})
		require.NoError(t, err)
	}

	removedContainer <- list[0].ID

	return removedContainer
}

func TestDockerCommandRunAttempts_InvalidAttempts(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	build := getBuildForOS(t, common.GetRemoteSuccessfulBuild)
	build.JobResponse.Variables = append(build.JobResponse.Variables, common.JobVariable{
		Key:    common.ExecutorJobSectionAttempts,
		Value:  strconv.Itoa(999),
		Public: true,
	})

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.Error(t, err)
}

func TestDockerCommand_WriteToVolumeNonRootImage(t *testing.T) {
	// non root images on Windows work differently, and `cache-init` doesn't
	// work on Windows
	// https://gitlab.com/gitlab-org/gitlab-runner/-/issues/25480.
	if runtime.GOOS == "windows" {
		t.Skip("Skipping unix test on windows")
	}

	helpers.SkipIntegrationTests(t, "docker", "info")

	const volumeBind = "/test"
	const helperImage = "gitlab/gitlab-runner-helper:x86_64-4c96e5ad"

	client, err := docker.New(docker.Credentials{})
	require.NoError(t, err, "creating docker client")

	build := getBuildForOS(t, common.GetRemoteSuccessfulBuild)
	build.Runner.Docker.Volumes = append(build.Runner.Docker.Volumes, volumeBind)
	build.Runner.Docker.HelperImage = helperImage
	build.JobResponse.Steps = common.Steps{
		common.Step{
			Name: common.StepNameScript,
			Script: []string{
				"echo test > /test/test.txt",
			},
			Timeout:      120,
			When:         common.StepWhenAlways,
			AllowFailure: false,
		},
	}
	build.Image.Name = common.TestAlpineNoRootImage

	defer func() {
		volumeName := fmt.Sprintf("%s-cache-%x", build.ProjectUniqueName(), md5.Sum([]byte(volumeBind)))

		err = client.VolumeRemove(context.Background(), volumeName, true)
		require.NoError(t, err)
	}()

	defer client.Close()

	err = buildtest.RunBuild(t, &build)
	assert.NoError(t, err)
}

func TestChownAndUmaskUsage(t *testing.T) {
	// On Windows we don't have the chown/umask problem so no need
	// for doing the tests. Especially that the test is specific for
	// Unix like platform
	if runtime.GOOS == "windows" {
		t.Skip("Skipping unix test on windows")
	}

	helpers.SkipIntegrationTests(t, "docker", "info")

	// nolint:lll
	umaskUsedUserNotChanged := func(t *testing.T, output string) {
		assert.NotContains(t, output, "Changing ownership of files")
		assert.Regexp(t, `drwxrwxrwx\s+[0-9]+\s+root\s+root\s+[0-9a-zA-Z: ]+\s+director`, output, "directory permissions changed by umask, user root")
		assert.Regexp(t, `-rwxrwxrwx\s+[0-9]+\s+root\s+root\s+[0-9a-zA-Z: ]+\s+executable-file`, output, "executable-file permissions changed by umask, user root")
		assert.Regexp(t, `-rw-rw-rw-\s+[0-9]+\s+root\s+root\s+[0-9a-zA-Z: ]+\s+regular-file`, output, "regular-file permissions changed by umask, user root")
	}
	// nolint:lll
	umaskNotUsedUserNotChanged := func(t *testing.T, output string) {
		assert.NotContains(t, output, "Changing ownership of files")
		assert.Regexp(t, `drwxr-xr-x\s+[0-9]+\s+root\s+root\s+[0-9a-zA-Z: ]+\s+director`, output, "directory permissions not changed by umask, user root")
		assert.Regexp(t, `-rwxr-xr-x\s+[0-9]+\s+root\s+root\s+[0-9a-zA-Z: ]+\s+executable-file`, output, "executable-file permissions not changed by umask, user root")
		assert.Regexp(t, `-rw-r--r--\s+[0-9]+\s+root\s+root\s+[0-9a-zA-Z: ]+\s+regular-file`, output, "regular-file permissions not changed by umask, user root")
	}
	// nolint:lll
	umaskNotUsedUserChanged := func(t *testing.T, output string) {
		assert.Contains(t, output, "Changing ownership of files")
		assert.Regexp(t, `drwxr-xr-x\s+[0-9]+\s+alpine\s+alpine\s+[0-9a-zA-Z: ]+\s+director`, output, "directory permissions not changed by umask, user alpine")
		assert.Regexp(t, `-rwxr-xr-x\s+[0-9]+\s+alpine\s+alpine\s+[0-9a-zA-Z: ]+\s+executable-file`, output, "executable-file permissions not changed by umask, user alpine")
		assert.Regexp(t, `-rw-r--r--\s+[0-9]+\s+alpine\s+alpine\s+[0-9a-zA-Z: ]+\s+regular-file`, output, "regular-file permissions not changed by umask, user alpine")
	}

	gitInfo := common.GitInfo{
		RepoURL:   "https://gitlab.com/gitlab-org/ci-cd/tests/file-permissions.git",
		Sha:       "050d238e16c5962fc16e49ab1b6be1be39778b6c",
		BeforeSha: "0000000000000000000000000000000000000000",
		Ref:       "main",
		RefType:   common.RefTypeBranch,
		Refspecs:  []string{"+refs/heads/*:refs/origin/heads/*", "+refs/tags/*:refs/tags/*"},
	}

	tests := map[string]struct {
		ffValue      string
		testImage    string
		assertOutput func(t *testing.T, output string)
	}{
		"FF_DISABLE_UMASK_FOR_DOCKER_EXECUTOR not set on root image": {
			ffValue:      "",
			testImage:    common.TestAlpineImage,
			assertOutput: umaskUsedUserNotChanged,
		},
		"FF_DISABLE_UMASK_FOR_DOCKER_EXECUTOR set explicitly to false on root image": {
			ffValue:      "false",
			testImage:    common.TestAlpineImage,
			assertOutput: umaskUsedUserNotChanged,
		},
		"FF_DISABLE_UMASK_FOR_DOCKER_EXECUTOR set to true on root image": {
			ffValue:      "true",
			testImage:    common.TestAlpineImage,
			assertOutput: umaskNotUsedUserNotChanged,
		},
		"FF_DISABLE_UMASK_FOR_DOCKER_EXECUTOR not set on non-root image": {
			ffValue:      "",
			testImage:    common.TestAlpineNoRootImage,
			assertOutput: umaskUsedUserNotChanged,
		},
		"FF_DISABLE_UMASK_FOR_DOCKER_EXECUTOR set explicitly to false on non-root image": {
			ffValue:      "false",
			testImage:    common.TestAlpineNoRootImage,
			assertOutput: umaskUsedUserNotChanged,
		},
		"FF_DISABLE_UMASK_FOR_DOCKER_EXECUTOR set to true on non-root image": {
			ffValue:      "true",
			testImage:    common.TestAlpineNoRootImage,
			assertOutput: umaskNotUsedUserChanged,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			jobResponse, err := common.GetRemoteBuildResponse("ls -l")
			require.NoError(t, err)

			jobResponse.GitInfo = gitInfo
			jobResponse.Variables = append(jobResponse.Variables, common.JobVariable{
				Key:   featureflags.DisableUmaskForDockerExecutor,
				Value: tt.ffValue,
			})

			build := &common.Build{
				JobResponse: jobResponse,
				Runner: &common.RunnerConfig{
					RunnerSettings: common.RunnerSettings{
						Executor: "docker",
						Docker: &common.DockerConfig{
							Image:      tt.testImage,
							PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
						},
					},
					SystemIDState: systemIDState,
				},
			}

			output, err := buildtest.RunBuildReturningOutput(t, build)
			require.NoError(t, err)

			tt.assertOutput(t, output)
		})
	}
}

func TestBuildLogLimitExceeded(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	buildtest.RunRemoteBuildWithJobOutputLimitExceeded(t, getRunnerConfigForOS(t), nil)
}

func TestCleanupProjectGitClone(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	build := getBuildForOS(t, common.GetRemoteSuccessfulBuild)
	buildtest.RunBuildWithCleanupGitClone(t, &build)
}

func TestCleanupProjectGitFetch(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	untrackedFilename := "untracked"

	build := getBuildForOS(t, func() (common.JobResponse, error) {
		return common.GetRemoteBuildResponse(
			buildtest.GetNewUntrackedFileIntoSubmodulesCommands(untrackedFilename, "", "")...,
		)
	})

	buildtest.RunBuildWithCleanupGitFetch(t, &build, untrackedFilename)
}

func TestCleanupProjectGitSubmoduleNormal(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	untrackedFile := "untracked"
	untrackedSubmoduleFile := "untracked_submodule"

	build := getBuildForOS(t, func() (common.JobResponse, error) {
		return common.GetRemoteBuildResponse(
			buildtest.GetNewUntrackedFileIntoSubmodulesCommands(untrackedFile, untrackedSubmoduleFile, "")...,
		)
	})

	buildtest.RunBuildWithCleanupNormalSubmoduleStrategy(t, &build, untrackedFile, untrackedSubmoduleFile)
}

func TestCleanupProjectGitSubmoduleRecursive(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	untrackedFile := "untracked"
	untrackedSubmoduleFile := "untracked_submodule"
	untrackedSubSubmoduleFile := "untracked_submodule_submodule"

	build := getBuildForOS(t, func() (common.JobResponse, error) {
		return common.GetRemoteBuildResponse(
			buildtest.GetNewUntrackedFileIntoSubmodulesCommands(
				untrackedFile,
				untrackedSubmoduleFile,
				untrackedSubSubmoduleFile)...,
		)
	})

	buildtest.RunBuildWithCleanupRecursiveSubmoduleStrategy(
		t,
		&build,
		untrackedFile,
		untrackedSubmoduleFile,
		untrackedSubSubmoduleFile,
	)
}

func TestDockerCommandServiceVariables(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	build := getBuildForOS(t, common.GetRemoteSuccessfulBuild)
	build.Variables = append(build.JobResponse.Variables,
		common.JobVariable{
			Key:    "FF_NETWORK_PER_BUILD",
			Value:  "true",
			Public: true,
		},
		common.JobVariable{
			Key:    "BUILD_VAR",
			Value:  "BUILD_VAR_VALUE",
			Public: true,
		},
	)

	shell := "sh"
	if runtime.GOOS == "windows" {
		shell = shells.SNPowershell
	}

	// immediately timeout as triggering an error is the  only way to get a
	// service to send its output to the log
	build.Runner.Docker.WaitForServicesTimeout = 1

	build.Services = common.Services{
		common.Image{
			Name: common.TestLivenessImage,
			Variables: []common.JobVariable{
				{
					Key:   "SERVICE_VAR",
					Value: "SERVICE_VAR_VALUE",
				},
				{
					Key:   "SERVICE_VAR_REF_BUILD_VAR",
					Value: "$BUILD_VAR",
				},
			},
			Entrypoint: append([]string{shell, "-c"}, "echo SERVICE_VAR=$SERVICE_VAR SERVICE_VAR_REF_BUILD_VAR=$SERVICE_VAR_REF_BUILD_VAR"),
		},
	}

	var buffer bytes.Buffer
	err := build.Run(&common.Config{}, &common.Trace{Writer: &buffer})
	assert.NoError(t, err)
	out := buffer.String()
	assert.Contains(t, out, "SERVICE_VAR=SERVICE_VAR_VALUE")
	assert.Contains(t, out, "SERVICE_VAR_REF_BUILD_VAR=BUILD_VAR_VALUE")
}

func TestDockerCommandConflictingPullPolicies(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	require.NoError(t, err)

	successfulBuild.Image = common.Image{Name: common.TestAlpineImage}
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image: common.TestAlpineImage,
				},
			},
			SystemIDState: systemIDState,
		},
	}

	tests := map[string]struct {
		imagePullPolicies   []common.DockerPullPolicy
		pullPolicy          common.StringOrArray
		allowedPullPolicies []common.DockerPullPolicy
		wantErrMsg          string
	}{
		"allowed_pull_policies configured, default pull_policy": {
			imagePullPolicies:   nil,
			pullPolicy:          nil,
			allowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
			wantErrMsg:          fmt.Sprintf(common.IncompatiblePullPolicy, "[always]", "Runner config (default)", "[if-not-present]"),
		},
		"allowed_pull_policies and pull_policy configured": {
			imagePullPolicies:   nil,
			pullPolicy:          common.StringOrArray{common.PullPolicyNever},
			allowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
			wantErrMsg:          fmt.Sprintf(common.IncompatiblePullPolicy, "[never]", "Runner config", "[if-not-present]"),
		},
		"allowed_pull_policies and image pull_policy configured": {
			imagePullPolicies:   []common.DockerPullPolicy{common.PullPolicyAlways},
			pullPolicy:          nil,
			allowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
			wantErrMsg:          fmt.Sprintf(common.IncompatiblePullPolicy, "[always]", "GitLab pipeline config", "[if-not-present]"),
		},
		"all configured": {
			imagePullPolicies:   []common.DockerPullPolicy{common.PullPolicyAlways},
			pullPolicy:          common.StringOrArray{common.PullPolicyNever},
			allowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
			wantErrMsg:          fmt.Sprintf(common.IncompatiblePullPolicy, "[always]", "GitLab pipeline config", "[if-not-present]"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			build.JobResponse.Image.PullPolicies = test.imagePullPolicies
			build.Runner.RunnerSettings.Docker.PullPolicy = test.pullPolicy
			build.Runner.RunnerSettings.Docker.AllowedPullPolicies = test.allowedPullPolicies

			gotErr := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})

			require.Error(t, gotErr)
			assert.Contains(t, gotErr.Error(), test.wantErrMsg)
			assert.Contains(t, gotErr.Error(), "failed to pull image '"+common.TestAlpineImage)
		})
	}
}

func Test_CaptureServiceLogs(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	tests := map[string]struct {
		buildVars []common.JobVariable
		assert    func(string, error)
	}{
		"enabled": {
			buildVars: []common.JobVariable{
				{
					Key:    "CI_DEBUG_SERVICES",
					Value:  "true",
					Public: true,
				}, {
					Key:    "POSTGRES_PASSWORD",
					Value:  "password",
					Public: true,
				},
			},
			assert: func(out string, err error) {
				assert.NoError(t, err)
				assert.NotContains(t, out, "WARNING: failed to parse value 'trace' for CI_DEBUG_SERVICES variable")
				assert.Regexp(t, `\[service:(postgres-db|db-postgres)\] .* The files belonging to this database system will be owned by user "postgres"`, out)
				assert.Regexp(t, `\[service:(postgres-db|db-postgres)\] .* database system is ready to accept connections`, out)
				assert.Regexp(t, `\[service:(redis-cache|cache-redis)\] .* oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0O`, out)
				assert.Regexp(t, `\[service:(redis-cache|cache-redis)\] .* Ready to accept connections`, out)
			},
		},
		"not enabled": {
			assert: func(out string, err error) {
				assert.NoError(t, err)
				assert.NotRegexp(t, `\[service:(postgres-db|db-postgres)\] .* Error: Database is uninitialized and superuser password is not specified`, out)
				assert.NotRegexp(t, `\[service:(redis-cache|cache-redis)\] .* oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0O`, out)
				assert.NotRegexp(t, `\[service:(redis-cache|cache-redis)\] .* Ready to accept connections`, out)
			},
		},
		"bogus value": {
			buildVars: []common.JobVariable{{
				Key:    "CI_DEBUG_SERVICES",
				Value:  "blammo",
				Public: true,
			}},
			assert: func(out string, err error) {
				assert.NoError(t, err)
				assert.Contains(t, out, "WARNING: failed to parse value 'blammo' for CI_DEBUG_SERVICES variable")
				assert.NotRegexp(t, `\[service:(postgres-db|db-postgres)\] .* Error: Database is uninitialized and superuser password is not specified`, out)
				assert.NotRegexp(t, `\[service:(redis-cache|cache-redis)\] .* oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0O`, out)
				assert.NotRegexp(t, `\[service:(redis-cache|cache-redis)\] .* Ready to accept connections`, out)
			},
		},
	}

	build := getBuildForOS(t, common.GetRemoteSuccessfulBuild)
	build.Services = append(build.Services, common.Image{Name: "postgres:14.4", Alias: "db"})
	build.Services = append(build.Services, common.Image{Name: "redis:7.0", Alias: "cache"})

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			build.Variables = tt.buildVars
			out, err := buildtest.RunBuildReturningOutput(t, &build)
			tt.assert(out, err)
		})
	}
}

func TestDockerCommandWithRunnerServiceEnvironmentVariables(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	assert.NoError(t, err)

	// Adding a gitlab-ci.yml variable to test the expansion of the service env variables
	successfulBuild.Variables = append(successfulBuild.Variables, common.JobVariable{
		Key:    "MY_GLOBAL_VAR",
		Value:  "my_global_var_value",
		Public: true,
	})

	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:      common.TestAlpineImage,
					PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
					Services: []common.Service{
						{
							Name: common.TestAlpineImage,
							Environment: []string{
								// expanded service env var
								"EXPANDED=$MY_GLOBAL_VAR",
								"FOO=value from [[runners.docker.services]]",
							},
							Entrypoint: []string{"/bin/sh", "-c"},
							Command:    []string{"echo -e \"FOO = $FOO\nEXPANDED = $EXPANDED\""},
						},
					},
				},
			},
		},
	}

	out := bytes.NewBuffer(nil)
	err = build.Run(&common.Config{}, &common.Trace{Writer: out})
	assert.NoError(t, err)
	assert.Contains(t, out.String(), "FOO = value from [[runners.docker.services]]")
	assert.Contains(t, out.String(), "EXPANDED = my_global_var_value")

}
