//go:build integration

package docker_test

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/sha1"
	"fmt"
	"io"
	"math/rand"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"gitlab.com/gitlab-org/gitlab-runner/shells/shellstest"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	execDocker "gitlab.com/gitlab-org/gitlab-runner/executors/docker"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/prebuilt"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/windows"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/test"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

// Specifying container image platform requires API version >= 1.41
const minDockerDaemonVersion = "1.41"

var (
	getWindowsImageOnce sync.Once
	windowsImage        string
	systemIDState       = common.NewSystemIDState()
)

var windowsDockerImageTagMappings = map[string]string{
	windows.V1809: "ltsc2019",
	windows.V21H2: "ltsc2022",
}

func TestMain(m *testing.M) {
	prebuilt.PrebuiltImagesPaths = []string{"../../out/helper-images/"}
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

	build := common.Build{
		JobResponse: jobResp,
		Runner:      getRunnerConfigForOS(t),
	}

	return build
}

func getRunnerConfigForOS(t *testing.T) *common.RunnerConfig {
	executor := "docker"
	image := common.TestAlpineImage
	shell := "bash"

	if runtime.GOOS == "windows" {
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

// windowsDockerImageTag checks the specified kernel version to see if it's one of the
// supported Windows version. If true, it maps a compatible mcr.microsoft.com Docker image tag.
// UnsupportedWindowsVersionError is returned when no supported Windows version
// is found in the string.
func windowsDockerImageTag(version string) (string, error) {
	version, err := windows.Version(version)
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

		dockerImageTag, err := windowsDockerImageTag(info.KernelVersion)
		require.NoError(t, err)

		windowsImage = fmt.Sprintf(common.TestWindowsImage, dockerImageTag)
	})

	return windowsImage
}

func TestBuildPassingEnvsMultistep(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	shellstest.OnEachShell(t, func(t *testing.T, shell string) {
		if shell == "pwsh" {
			t.Skipf("%s not supported", shell)
		}

		runnerConfig := getRunnerConfigForOS(t)
		runnerConfig.RunnerSettings.Shell = shell

		buildtest.RunBuildWithPassingEnvsMultistep(t, runnerConfig, nil)
	})
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
		if shell == "pwsh" || shell == "powershell" {
			// support for pwsh and powershell tracked in https://gitlab.com/gitlab-org/gitlab-runner/-/issues/28119
			t.Skip("pwsh, powershell not supported")
		}

		build := getBuildForOS(t, func() (common.JobResponse, error) {
			return common.GetRemoteBuildResponse(`echo "Hello
World"`)
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

func TestDockerCommandMissingImage(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	build := getBuildForOS(t, common.GetSuccessfulBuild)
	build.Runner.Docker.Image = "some/non-existing/image"

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.ErrorIs(t, err, &common.BuildError{FailureReason: common.ImagePullFailure})
	assert.Regexp(t, regexp.MustCompile("not found|repository does not exist|invalid repository name"), err.Error())
}

func TestDockerCommandMissingTag(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	build := getBuildForOS(t, common.GetSuccessfulBuild)
	build.Runner.Docker.Image = "docker:missing-tag"

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.ErrorIs(t, err, &common.BuildError{FailureReason: common.ImagePullFailure})
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
	assert.ErrorIs(t, err, &common.BuildError{FailureReason: common.ImagePullFailure})
	assert.Regexp(t, regexp.MustCompile("not found|repository does not exist|invalid repository name"), err.Error())
}

// TestDockerCommandPullingImageNoHost tests if the DNS resolution failure for the registry host
// is categorized as a script failure.
func TestDockerCommandPullingImageNoHost(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	build := getBuildForOS(t, common.GetSuccessfulBuild)
	build.Runner.RunnerSettings.Docker.Image = "docker.repo.example.com/docker:23-dind"

	var buildError *common.BuildError
	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.ErrorAs(t, err, &buildError)

	assert.Equal(t, common.ImagePullFailure, buildError.FailureReason, "expected script failure error")
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

func TestDockerCommandServiceNameEmpty(t *testing.T) {
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
		{Name: "", Alias: "service-1"}, // Name can be empty if for example env variable expands to empty string.
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

			str := buffer.String()

			// Shouldn't be considered a system failure
			var buildErr *common.BuildError
			assert.ErrorAs(t, err, &buildErr)
			assert.NotContains(t, str, "system failure")
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

	re, err := regexp.Compile("(?m)^Initialized empty Git repository in /builds/gitlab-org/ci-cd/gitlab-runner-pipeline-tests/gitlab-test/.git/")
	require.NoError(t, err)
	assert.Regexp(t, re, buffer.String())
}

func TestDockerPrivilegedServiceAccessingBuildsFolder(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	job, err := common.GetRemoteBuildResponse(
		"docker info",
		"docker run -v $(pwd):$(pwd) -w $(pwd) busybox touch test",
		"cat test",
	)
	assert.NoError(t, err)

	strategies := []string{
		"fetch",
		"clone",
	}

	for _, strategy := range strategies {
		t.Log("Testing", strategy, "strategy...")

		build := getTestDockerJob(t, job)
		build.Image.Name = common.TestDockerGitImage
		build.Services = common.Services{
			common.Image{
				Name: common.TestDockerDindImage,
				// set bip manually to prevent DinD-ception networking problems
				// and avoid collision with:
				// - docker daemon on the host
				// - dind as a service to the CI job running this test
				// - dind as a service to this test
				Command: []string{"--bip", "172.30.0.1/16"},
			},
		}

		build.Variables = append(build.Variables, common.JobVariable{
			Key: "GIT_STRATEGY", Value: strategy,
		})

		err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
		assert.NoError(t, err)
	}
}

func getTestDockerJob(t *testing.T, job common.JobResponse) *common.Build {
	job.Variables = append(job.Variables,
		common.JobVariable{Key: "DOCKER_TLS_VERIFY", Value: "1"},
		common.JobVariable{Key: "DOCKER_TLS_CERTDIR", Value: "/certs"},
		common.JobVariable{Key: "DOCKER_CERT_PATH", Value: "/certs/client"},
	)

	build := &common.Build{
		JobResponse: job,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:      common.TestAlpineImage,
					PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
					Privileged: true,
					Volumes:    []string{"/certs"},
				},
			},
			SystemIDState: systemIDState,
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
				{Key: "DOCKER_HOST", Value: "tcp://docker:2376"},
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
				{Key: "DOCKER_HOST", Value: "tcp://docker:2376"},
				{Key: "IMAGE_NAME", Value: common.TestDockerGitImage},
				{Key: "SERVICE_NAME", Value: common.TestDockerDindImage},
			},
		},
	}

	for exampleID, example := range examples {
		t.Run(fmt.Sprintf("example-%d", exampleID), func(t *testing.T) {
			job, err := common.GetRemoteBuildResponse("docker info")
			assert.NoError(t, err)

			build := getTestDockerJob(t, job)
			build.Image = example.image
			build.Services = example.services
			build.Variables = append(build.Variables, example.variables...)

			err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
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
		port            int
		variables       common.JobVariables
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
		"successful service explicit port (FF_NETWORK_PER_BUILD=false)": {
			command:         []string{"server", "--addr", ":8888"},
			serviceStarted:  true,
			networkPerBuild: "false",
			skip:            runtime.GOOS == "windows",
			port:            8888,
			variables:       []common.JobVariable{{Key: "HEALTHCHECK_TCP_PORT", Value: "8888"}},
		},
		"successful service explicit port (FF_NETWORK_PER_BUILD=true)": {
			command:         []string{"server", "--addr", ":8888"},
			serviceStarted:  true,
			networkPerBuild: "true",
			skip:            false,
			port:            8888,
			variables:       []common.JobVariable{{Key: "HEALTHCHECK_TCP_PORT", Value: "8888"}},
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

			if tc.port == 0 {
				tc.port = 80
			}

			resp, err := common.GetRemoteBuildResponse(
				fmt.Sprintf("liveness client db:%d", tc.port),
				fmt.Sprintf("liveness client registry.gitlab.com__gitlab-org__ci-cd__tests__liveness:%d", tc.port),
				fmt.Sprintf("liveness client registry.gitlab.com-gitlab-org-ci-cd-tests-liveness:%d", tc.port),
			)
			require.NoError(t, err)

			build := common.Build{
				JobResponse: resp,
				Runner: &common.RunnerConfig{
					RunnerSettings: common.RunnerSettings{
						Executor: "docker",
						Docker: &common.DockerConfig{
							WaitForServicesTimeout: 15,
						},
					},
					SystemIDState: systemIDState,
				},
			}

			build.Image = common.Image{
				Name:       common.TestLivenessImage,
				Entrypoint: []string{""},
			}

			if runtime.GOOS == "windows" {
				build.Runner.Docker.WaitForServicesTimeout = 60
				build.Runner.RunnerSettings.Shell = shells.SNPwsh
			}

			build.Services = append(build.Services, common.Image{
				Name:      common.TestLivenessImage,
				Alias:     "db",
				Command:   tc.command,
				Variables: tc.variables,
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

	helperImageConfig := "gitlab/gitlab-runner-helper:x86_64-v16.9.1"

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
				// Ensure ProxyExec is disabled as the gitlab-runner-helper image above doesn't contain
				// the proxy_exec subcommand.
				ProxyExec: func() *bool { v := false; return &v }(),
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
		"Using docker image sha256:be0a1939d88dbce6f18b0885662080a6aabc49d7e5e51c6021f36ce327614b13 for "+
			"gitlab/gitlab-runner-helper:x86_64-v16.9.1 with digest "+
			"gitlab/gitlab-runner-helper@sha256:24432bb8b93507e7bc4b87327c24317029f1ea0315abf1bc7f71148f2555d681 ...",
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

	dockerSocket := "/var/run/docker.sock"
	successfulBuild.Variables = append(successfulBuild.Variables, common.JobVariable{
		Key:   "DOCKER_HOST",
		Value: "unix://" + dockerSocket,
	})

	// In CI, it's possible that DOCKER_HOST has been overridden to a different unix
	// path to usual, so we cater for that.
	//
	// This is not something we can typically do outside of CI, because overriding
	// won't always work (DOCKER_HOST pointing to a file that's on the host, and not VM
	// in a Docker/Rancher Desktop scenario). In that case, leaving the default is
	// more likely to work.
	if _, ok := os.LookupEnv("CI"); ok {
		if sock := os.Getenv("DOCKER_HOST"); strings.HasPrefix(sock, "unix://") {
			dockerSocket = strings.TrimPrefix(sock, "unix://")
		}
	}

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
						dockerSocket + ":/var/run/docker.sock",
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
		containerList := container.ListOptions{
			Filters: filters.NewArgs(nameFilter),
		}
		list, err = client.ContainerList(context.Background(), containerList)
		require.NoError(t, err)
	}

	for _, ctr := range list {
		err := client.ContainerRemove(context.Background(), ctr.ID, container.RemoveOptions{Force: true})
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

	buf := new(bytes.Buffer)
	err := build.Run(&common.Config{}, &common.Trace{Writer: buf})
	require.NoError(t, err)
	require.Contains(t, buf.String(), "WARNING: EXECUTOR_JOB_SECTION_ATTEMPTS: number of attempts out of the range [1, 10], using default 1")
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
	const helperImage = "gitlab/gitlab-runner-helper:x86_64-v16.9.1"

	client, err := docker.New(docker.Credentials{})
	require.NoError(t, err, "creating docker client")

	build := getBuildForOS(t, common.GetRemoteSuccessfulBuild)
	build.Runner.Docker.Volumes = append(build.Runner.Docker.Volumes, volumeBind)
	build.Runner.Docker.HelperImage = helperImage
	// Ensure ProxyExec is disabled as the gitlab-runner-helper image above doesn't contain
	// the proxy_exec subcommand.
	build.Runner.RunnerSettings.ProxyExec = func() *bool { v := false; return &v }()
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
			assert.Contains(t, gotErr.Error(), `invalid pull policy for image "`+common.TestAlpineImage)
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
				assert.NotContains(t, out, "WARNING: CI_DEBUG_SERVICES: expected bool got \"blammo\", using default value: false")
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
				assert.Contains(t, out, "WARNING: CI_DEBUG_SERVICES: expected bool got \"blammo\", using default value: false")
				assert.NotRegexp(t, `\[service:(postgres-db|db-postgres)\] .* Error: Database is uninitialized and superuser password is not specified`, out)
				assert.NotRegexp(t, `\[service:(redis-cache|cache-redis)\] .* oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0O`, out)
				assert.NotRegexp(t, `\[service:(redis-cache|cache-redis)\] .* Ready to accept connections`, out)
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			build := getBuildForOS(t, common.GetRemoteSuccessfulBuild)
			build.Services = append(build.Services, common.Image{Name: "postgres:14.4", Alias: "db"})
			build.Services = append(build.Services, common.Image{Name: "redis:7.0", Alias: "cache"})

			build.Variables = tt.buildVars
			out, err := buildtest.RunBuildReturningOutput(t, &build)
			tt.assert(out, err)
		})
	}
}

func Test_ExpandingVolumes(t *testing.T) {
	helpers.SkipIntegrationTests(t, "docker", "info")

	testScripts := map[string]map[string]string{
		"non-windows": {
			"prepare": `
				set -x
				testFile='%[1]s'
				test -e "$testFile" && {
					echo >&2 "Expected '$testFile' not to exist"
					exit 1
				}
				echo '%[2]s' > "$testFile"
			`,
			"check": `
				set -x
				testFile='%[1]s'
				test -e "$testFile" || {
					echo >&2 "Expected '$testFile' to exist"
					exit 1
				}
				test '%[2]s' == "$(cat "$testFile")"
			`,
		},
		"windows": {
			"prepare": `
				Set-PSDebug -Trace 2
				$testFile = '%[1]s'
				if ([System.IO.File]::Exists($testFile)) {
					$host.ui.WriteErrorLine("Expected '$testFile' not to exist")
					exit 1
				}
				echo '%[2]s' > $testFile
			`,
			"check": `
				Set-PSDebug -Trace 2
				$testFile = '%[1]s'
				if (-not [System.IO.File]::Exists($testFile)) {
					$host.ui.WriteErrorLine("Expected '$testFile' to exist")
					exit 1
				}
				if (-not (Get-Content $testFile).equals('%[2]s')) {
					exit 1
				}
			`,
		},
	}

	randString := strconv.Itoa(rand.Int())
	runnerEnv := []string{"FOO=theFoo"}
	jobVariables := common.JobVariables{
		{Key: "SOME_VAR", Value: "${FOO}-${BAR}-theBlipp"},
		{Key: "BAR", Value: "theBar"},
		{Key: "RANDOM", Value: randString},
		{Key: "GIT_STRATEGY", Value: string(common.GitNone)},
	}
	volumes := []string{"/tmp/${SOME_VAR}/${RANDOM}"}
	testFile := filepath.Join("/tmp/theFoo-theBar-theBlipp", randString, "testFile")
	prepareScript := testScripts["non-windows"]["prepare"]
	checkScript := testScripts["non-windows"]["check"]

	if runtime.GOOS == test.OSWindows {
		volumes = []string{`c:\tmp\${SOME_VAR}\${RANDOM}`}
		testFile = filepath.Join(`c:\tmp\theFoo-theBar-theBlipp`, randString, "testFile")
		prepareScript = testScripts["windows"]["prepare"]
		checkScript = testScripts["windows"]["check"]
	}

	build := getBuildForOS(t, common.GetRemoteSuccessfulBuild)
	build.JobResponse.Variables = jobVariables
	build.Runner.Docker.Volumes = volumes
	build.Runner.Environment = runnerEnv

	// ensures that the volume is mounted and can be written to.
	build.JobResponse.Steps[0].Script[0] = fmt.Sprintf(prepareScript, testFile, randString)
	_, err := buildtest.RunBuildReturningOutput(t, &build)
	if !assert.NoError(t, err) {
		return
	}

	// ensures that the volume with same vars is cached/kept around, and the same volume is mounted again.
	build.JobResponse.Steps[0].Script[0] = fmt.Sprintf(checkScript, testFile, randString)
	_, err = buildtest.RunBuildReturningOutput(t, &build)
	assert.NoError(t, err)
}

func Test_ContainerOptionsExpansion(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	successfulBuild, err := common.GetRemoteSuccessfulBuildWithDumpedVariables()
	assert.NoError(t, err)

	jobVars := common.JobVariables{
		{Key: "CI_DEBUG_SERVICES", Value: "true", Public: true},
		{Key: "POSTGRES_PASSWORD", Value: "password", Public: true},
		{Key: "JOB_IMAGE", Value: "alpine:latest"},
		{Key: "HELPER_IMAGE_FLAVOR", Value: "alpine"},
		{Key: "SRVS_IMAGE", Value: "postgres:latest"},
		{Key: "SRVS_IMAGE_ALIAS", Value: "db"},
	}
	successfulBuild.Variables = append(successfulBuild.Variables, jobVars...)

	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image:             "$JOB_IMAGE",
					HelperImageFlavor: "$HELPER_IMAGE_FLAVOR",
					Services: []common.Service{
						{Name: "$SRVS_IMAGE", Alias: "$SRVS_IMAGE_ALIAS"},
					},
				},
			},
		},
	}

	out, err := buildtest.RunBuildReturningOutput(t, build)
	assert.NoError(t, err)
	assert.Contains(t, out, "Pulling docker image alpine:latest")
	assert.Contains(t, out, "Pulling docker image postgres:latest")
	assert.Regexp(t, `\[service:(postgres-db|db-postgres)\]`, out)
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

func TestDockerBuildContainerGracefulShutdownNoInit(t *testing.T) {
	testDockerBuildContainerGracefulShutdown(t, false)
}

func TestDockerBuildContainerGracefulShutdownWithInit(t *testing.T) {
	testDockerBuildContainerGracefulShutdown(t, true)
}

func testDockerBuildContainerGracefulShutdown(t *testing.T, useInit bool) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	tests := map[string]func(*common.Build, *common.Trace) func(){
		// Comment this out for now. This test is flaky because the timeout includes docker image pull time, which will
		// always have outliers that exceed the entire job timeout.
		// "timeout exceeded": func(b *common.Build, _ *common.Trace) func() {
		// 	b.RunnerInfo.Timeout = 10
		// 	return func() {}
		// },
		"RUNNER_SCRIPT_TIMEOUT exceeded": func(b *common.Build, _ *common.Trace) func() {
			b.Variables = append(b.Variables, common.JobVariable{
				Key:   "RUNNER_SCRIPT_TIMEOUT",
				Value: "2s",
			})
			return func() {}
		},
		"job cancelled": func(build *common.Build, tr *common.Trace) func() {
			return buildtest.OnStage(build, "step_", func() {
				time.Sleep(2 * time.Second)
				assert.True(t, tr.Cancel())
			})
		},
		"job aborted": func(build *common.Build, tr *common.Trace) func() {
			return buildtest.OnStage(build, "step_", func() {
				time.Sleep(2 * time.Second)
				assert.True(t, tr.Abort())
			})
		},
	}

	for name, testSetup := range tests {
		t.Run(name, func(t *testing.T) {
			successfulBuild, err := common.GetRemoteBuildResponse("./long-script-with-cleanup.sh")
			assert.NoError(t, err)

			successfulBuild.GitInfo.Sha = "6353879af977aed75f7f75b7f8084a5cb6f1177a"

			build := &common.Build{
				JobResponse: successfulBuild,
				Runner: &common.RunnerConfig{
					RunnerSettings: common.RunnerSettings{
						Executor: "docker",
						Docker: &common.DockerConfig{
							Image:      "alpine:latest",
							PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
						},
					},
				},
			}

			if useInit {
				build.Variables = append(build.Variables, common.JobVariable{
					Key:   "FF_USE_INIT_WITH_DOCKER_EXECUTOR",
					Value: "true",
				})
			}

			out := bytes.NewBuffer(nil)
			trace := common.Trace{Writer: out}

			defer testSetup(build, &trace)()

			err = build.Run(&common.Config{}, &trace)

			assert.Error(t, err)

			assert.EventuallyWithT(t, func(t *assert.CollectT) {
				assert.Regexp(t, "Starting [0-9]{1,2}", out.String())
				assert.Regexp(t, "Caught SIGTERM", out.String())
				assert.Regexp(t, "Exiting [0-9]{1,2}", out.String())
			}, 5*time.Second, 1*time.Second)
		})
	}
}

func Test_FF_USE_INIT_WITH_DOCKER_EXECUTOR(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	tests := map[string]bool{
		"use init":        true,
		"do not use init": false,
	}

	for name, useInit := range tests {
		t.Run(name, func(t *testing.T) {
			successfulBuild, err := common.GetRemoteBuildResponse("ps -A")
			assert.NoError(t, err)

			build := &common.Build{
				JobResponse: successfulBuild,
				Runner: &common.RunnerConfig{
					RunnerSettings: common.RunnerSettings{
						Executor: "docker",
						Docker: &common.DockerConfig{
							Image:      "alpine:latest",
							PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
						},
					},
				},
			}

			if useInit {
				build.Variables = append(build.Variables, common.JobVariable{
					Key:   "FF_USE_INIT_WITH_DOCKER_EXECUTOR",
					Value: "true",
				})
			}

			out := bytes.NewBuffer(nil)
			assert.NoError(t, build.Run(&common.Config{}, &common.Trace{Writer: out}))

			if useInit {
				assert.Regexp(t, "1 root      0:00 /sbin/docker-init --", out.String())
			} else {
				assert.NotRegexp(t, "1 root      0:00 /sbin/docker-init --", out.String())
			}
		})
	}
}

func Test_ServiceLabels(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	client, err := docker.New(docker.Credentials{})
	require.NoError(t, err)
	defer client.Close()

	wg := sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()

		// wait for service container to appear and get its name
		nameFilter := filters.NewArgs(filters.Arg("name", "redis-0"))
		containerList := container.ListOptions{Filters: nameFilter}
		var container string

		require.Eventually(t, func() bool {
			list, err := client.ContainerList(context.Background(), containerList)
			require.NoError(t, err)
			if len(list) != 1 {
				return false
			}
			container = list[0].ID
			return true
		}, time.Second*10, time.Millisecond*500)

		// inspect container and assert expected labels exist...
		info, err := client.ContainerInspect(context.Background(), container)
		require.NoError(t, err)
		assert.Contains(t, info.Config.Labels, "com.gitlab.gitlab-runner.FOO")
		assert.Equal(t, "FOO", info.Config.Labels["com.gitlab.gitlab-runner.FOO"])
		assert.Contains(t, info.Config.Labels, "com.gitlab.gitlab-runner.BAR")
		assert.Equal(t, "BAR", info.Config.Labels["com.gitlab.gitlab-runner.BAR"])
	}()

	successfulBuild, err := common.GetRemoteBuildResponse("sleep 3")
	successfulBuild.Services = common.Services{{Name: "redis:7.0", Alias: "service-1"}}

	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker: &common.DockerConfig{
					Image: "alpine:latest",
					ContainerLabels: map[string]string{
						"FOO": "FOO",
						"BAR": "BAR",
					},
				},
			},
			SystemIDState: systemIDState,
		},
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)

	wg.Wait()
}

func Test_ServiceVolumeMounts(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")

	tests := map[string]struct {
		devices []string
		assert  func(string)
	}{
		"no device bindings": {
			assert: func(out string) {
				assert.Contains(t, out, "ls: /test: No such file or directory")
			},
		},
		"with device bindings": {
			devices: []string{"/dev/:/test/:ro"},
			assert: func(out string) {
				assert.NotContains(t, out, "ls: /test: No such file or directory")
				assert.Contains(t, out, "tty")
				assert.Contains(t, out, "cpu")
			},
		},
	}

	build := getBuildForOS(t, common.GetRemoteSuccessfulBuild)
	build.Services = append(build.Services, common.Image{
		Name:       "alpine:latest",
		Entrypoint: []string{"ls", "/test"},
	})

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			build.Runner.Docker.ServicesDevices = nil
			if len(tt.devices) != 0 {
				build.Runner.Docker.ServicesDevices = map[string][]string{
					"alpine:*": tt.devices,
				}
			}

			out, err := buildtest.RunBuildReturningOutput(t, &build)
			assert.NoError(t, err)
			tt.assert(out)
		})
	}
}

func TestDockerCommandWithPlatform(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")
	test.SkipIfDockerDaemonAPIVersionNotAtLeast(t, minDockerDaemonVersion)

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	assert.NoError(t, err)

	// leave platform empty
	successfulBuild.Image.Name = common.TestAlpineImage

	successfulBuild.Services = common.Services{
		{
			Name: "redis:7.0",
			ExecutorOptions: common.ImageExecutorOptions{
				Docker: common.ImageDockerOptions{Platform: "amd64"},
			},
		},
		{
			Name: "postgres:14.4",
			ExecutorOptions: common.ImageExecutorOptions{
				Docker: common.ImageDockerOptions{Platform: "arm64"}, // this image will download but fail to run, which is OK.
			},
		},
	}

	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker:   &common.DockerConfig{},
			},
			SystemIDState: systemIDState,
		},
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: &bytes.Buffer{}})
	require.NoError(t, err)

	images := map[string]string{
		"redis:7.0":     "amd64",
		"postgres:14.4": "arm64",
		// unspecified platform defaults to host arch
		common.TestAlpineImage: runtime.GOARCH,
	}

	client, err := docker.New(docker.Credentials{})
	require.NoError(t, err)
	defer client.Close()
	for img, arch := range images {
		info, _, err := client.ImageInspectWithRaw(context.Background(), img)
		require.NoError(t, err)
		assert.Equal(t, arch, info.Architecture)
	}
}

func TestDockerCommandWithUser(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")
	test.SkipIfDockerDaemonAPIVersionNotAtLeast(t, minDockerDaemonVersion)

	successfulBuild, err := common.GetRemoteBuildResponse("whoami")
	require.NoError(t, err)

	successfulBuild.Steps[0].Name = "wait"

	successfulBuild.Image.Name = common.TestAlpineImage
	successfulBuild.Image.ExecutorOptions.Docker.User = "squid"

	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "docker",
				Docker:   &common.DockerConfig{},
			},
			SystemIDState: systemIDState,
		},
	}

	var buffer bytes.Buffer
	require.NoError(t, build.Run(&common.Config{}, &common.Trace{Writer: &buffer}))

	assert.Regexp(t, "whoami.*\nsquid", buffer.String())
}

func TestDockerCommand_MacAddressConfig(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)
	helpers.SkipIntegrationTests(t, "docker", "info")
	test.SkipIfDockerDaemonAPIVersionNotAtLeast(t, minDockerDaemonVersion)

	macAddress := "92:d0:c6:0a:29:33"

	apiVersionAtLeast1_44, err := test.IsDockerDaemonAPIVersionAtLeast("1.44")
	require.NoError(t, err)

	type testCase struct {
		networkMode     string
		networkPerBuild bool
		expectedRunErr  bool
		validate        func(*testing.T, types.ContainerJSON)
	}

	tests := map[string]testCase{
		"empty (user defined), network per build enabled": {networkMode: "", networkPerBuild: true, validate: func(t *testing.T, info types.ContainerJSON) {
			assert.Equal(t, "", info.NetworkSettings.MacAddress, "net settings")
			assert.Len(t, info.NetworkSettings.Networks, 1)
			for k, v := range info.NetworkSettings.Networks {
				assert.Contains(t, k, "runner-")
				assert.Equal(t, macAddress, v.MacAddress, k+" network")
			}
		}},
		"empty (user defined), network per build disabled": {networkMode: "", networkPerBuild: false, validate: func(t *testing.T, info types.ContainerJSON) {
			assert.Equal(t, macAddress, info.NetworkSettings.MacAddress, "net settings")
			assert.Len(t, info.NetworkSettings.Networks, 1)
			for k, v := range info.NetworkSettings.Networks {
				assert.Equal(t, "bridge", k)
				assert.Equal(t, macAddress, v.MacAddress, k+" network")
			}
		}},
		"default, network per build enabled": {networkMode: "default", networkPerBuild: true, validate: func(t *testing.T, info types.ContainerJSON) {
			assert.Equal(t, macAddress, info.NetworkSettings.MacAddress, "net settings")
			assert.Len(t, info.NetworkSettings.Networks, 1)
			for k, v := range info.NetworkSettings.Networks {
				assert.Equal(t, "bridge", k)
				assert.Equal(t, macAddress, v.MacAddress, k+" network")
			}
		}},
		"default, network per build disabled": {networkMode: "default", networkPerBuild: false, validate: func(t *testing.T, info types.ContainerJSON) {
			assert.Equal(t, macAddress, info.NetworkSettings.MacAddress, "net settings")
			assert.Len(t, info.NetworkSettings.Networks, 1)
			for k, v := range info.NetworkSettings.Networks {
				assert.Equal(t, "bridge", k)
				assert.Equal(t, macAddress, v.MacAddress, k+" network")
			}
		}},
		"bridge, network per build enabled": {networkMode: "bridge", networkPerBuild: true, validate: func(t *testing.T, info types.ContainerJSON) {
			assert.Equal(t, macAddress, info.NetworkSettings.MacAddress, "net settings")
			assert.Len(t, info.NetworkSettings.Networks, 1)
			for k, v := range info.NetworkSettings.Networks {
				assert.Equal(t, "bridge", k)
				assert.Equal(t, macAddress, v.MacAddress, k+" network")
			}
		}},
		"bridge, network per build disabled": {networkMode: "bridge", networkPerBuild: false, validate: func(t *testing.T, info types.ContainerJSON) {
			assert.Equal(t, macAddress, info.NetworkSettings.MacAddress, "net settings")
			assert.Len(t, info.NetworkSettings.Networks, 1)
			for k, v := range info.NetworkSettings.Networks {
				assert.Equal(t, "bridge", k)
				assert.Equal(t, macAddress, v.MacAddress, k+" network")
			}
		}},
		// the cases below fail with "exit code 1" when run in a CI pipeline, and "conflicting options: mac-address and
		// the network mode" when run locally.
		"none, network per build enabled":  {networkMode: "none", networkPerBuild: true, expectedRunErr: true},
		"none, network per build disabled": {networkMode: "none", networkPerBuild: false, expectedRunErr: true},

		"host, network per build enabled": {
			networkMode: "host", networkPerBuild: true, expectedRunErr: !apiVersionAtLeast1_44,
			validate: func(t *testing.T, info types.ContainerJSON) {
				assert.Equal(t, "", info.NetworkSettings.MacAddress, "net settings")
				assert.Len(t, info.NetworkSettings.Networks, 1)
				for k, v := range info.NetworkSettings.Networks {
					assert.Equal(t, "host", k)
					assert.Equal(t, macAddress, v.MacAddress, k+" network")
				}
			},
		},
		"host, network per build disabled": {
			networkMode: "host", networkPerBuild: false, expectedRunErr: !apiVersionAtLeast1_44,
			validate: func(t *testing.T, info types.ContainerJSON) {
				assert.Equal(t, "", info.NetworkSettings.MacAddress, "net settings")
				assert.Len(t, info.NetworkSettings.Networks, 1)
				for k, v := range info.NetworkSettings.Networks {
					assert.Equal(t, "host", k)
					assert.Equal(t, macAddress, v.MacAddress, k+" network")
				}
			},
		},
	}

	// we'll make some direct docker API calls in this tests...
	client, err := docker.New(docker.Credentials{})
	require.NoError(t, err, "creating docker client")
	defer client.Close()

	ctx := context.Background()

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			runnerID := 987654321
			// make a build according to the test case parameters...
			rc := getRunnerConfigForOS(t)
			rc.Docker.MacAddress = macAddress
			rc.Docker.NetworkMode = tc.networkMode
			build := getBuildForOS(t, func() (common.JobResponse, error) {
				return common.GetRemoteBuildResponse("sleep 3")
			})
			build.Runner = rc
			build.ProjectRunnerID = runnerID
			build.Variables = append(build.Variables, common.JobVariable{
				Key:   featureflags.NetworkPerBuild,
				Value: strconv.FormatBool(tc.networkPerBuild),
			})

			wg := sync.WaitGroup{}
			wg.Add(1)
			defer wg.Wait() // wait for build job to finish

			go func(t *testing.T, tc testCase) {
				defer wg.Done()
				// run the build...
				err := build.Run(&common.Config{}, &common.Trace{Writer: &bytes.Buffer{}})

				if tc.expectedRunErr {
					assert.Error(t, err, "running build")
				} else {
					require.NoError(t, err, "running build")
				}
			}(t, tc)

			if tc.expectedRunErr {
				// we expect build.Run to fail so there's noting else to do...
				return
			}

			re := regexp.MustCompile("runner-.*-project-0-concurrent-" + strconv.Itoa(runnerID) + "-.*-build")
			var ctr types.Container
			// wait for the build container to be created...
			require.Eventually(t, func() bool {
				list, err := client.ContainerList(ctx, container.ListOptions{})
				assert.NoError(t, err, "listing containers")

				for _, l := range list {
					for _, n := range l.Names {
						if re.MatchString(n) {
							ctr = l
							return true
						}
					}
				}
				return false
			}, time.Second*10, time.Millisecond*500)

			// inspect the build container to examine the MacAddress configuration
			info, err := client.ContainerInspect(ctx, ctr.ID)
			assert.NoError(t, err, "inspecting container %q", ctr.ID)

			tc.validate(t, info)
		})
	}
}
