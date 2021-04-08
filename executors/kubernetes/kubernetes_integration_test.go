package kubernetes_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	k8s "k8s.io/client-go/kubernetes"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes/internal/pull"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/helperimage"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/session"
)

type featureFlagTest func(t *testing.T, flagName string, flagValue bool)

func TestRunIntegrationTestsWithFeatureFlag(t *testing.T) {
	tests := map[string]featureFlagTest{
		"testKubernetesSuccessRun":                                testKubernetesSuccessRunFeatureFlag,
		"testKubernetesMultistepRun":                              testKubernetesMultistepRunFeatureFlag,
		"testKubernetesTimeoutRun":                                testKubernetesTimeoutRunFeatureFlag,
		"testKubernetesBuildFail":                                 testKubernetesBuildFailFeatureFlag,
		"testKubernetesBuildCancel":                               testKubernetesBuildCancelFeatureFlag,
		"testKubernetesBuildLogLimitExceeded":                     testKubernetesBuildLogLimitExceededFeatureFlag,
		"testKubernetesBuildMasking":                              testKubernetesBuildMaskingFeatureFlag,
		"testKubernetesCustomClonePath":                           testKubernetesCustomClonePathFeatureFlag,
		"testKubernetesNoRootImage":                               testKubernetesNoRootImageFeatureFlag,
		"testKubernetesMissingImage":                              testKubernetesMissingImageFeatureFlag,
		"testKubernetesMissingTag":                                testKubernetesMissingTagFeatureFlag,
		"testKubernetesFailingToPullImageTwiceFeatureFlag":        testKubernetesFailingToPullImageTwiceFeatureFlag,
		"testKubernetesFailingToPullServiceImageTwiceFeatureFlag": testKubernetesFailingToPullSvcImageTwiceFeatureFlag,
		"testKubernetesFailingToPullHelperTwiceFeatureFlag":       testKubernetesFailingToPullHelperTwiceFeatureFlag,
		"testOverwriteNamespaceNotMatch":                          testOverwriteNamespaceNotMatchFeatureFlag,
		"testOverwriteServiceAccountNotMatch":                     testOverwriteServiceAccountNotMatchFeatureFlag,
		"testInteractiveTerminal":                                 testInteractiveTerminalFeatureFlag,
		"testKubernetesReplaceEnvFeatureFlag":                     testKubernetesReplaceEnvFeatureFlag,
		"testKubernetesReplaceMissingEnvVarFeatureFlag":           testKubernetesReplaceMissingEnvVarFeatureFlag,
		"testKubernetesWithNonRootSecurityContext":                testKubernetesWithNonRootSecurityContext,
	}

	featureFlags := []string{
		featureflags.UseLegacyKubernetesExecutionStrategy,
	}

	for tn, tt := range tests {
		for _, ff := range featureFlags {
			t.Run(fmt.Sprintf("%s %s true", tn, ff), func(t *testing.T) {
				tt(t, ff, true)
			})

			t.Run(fmt.Sprintf("%s %s false", tn, ff), func(t *testing.T) {
				tt(t, ff, false)
			})
		}
	}
}

func testKubernetesSuccessRunFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
	build.Image.Name = common.TestDockerGitImage
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func testKubernetesMultistepRunFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	successfulBuild, err := common.GetRemoteSuccessfulMultistepBuild()
	require.NoError(t, err)

	failingScriptBuild, err := common.GetRemoteFailingMultistepBuild(common.StepNameScript)
	require.NoError(t, err)

	failingReleaseBuild, err := common.GetRemoteFailingMultistepBuild("release")
	require.NoError(t, err)

	successfulBuild.Image.Name = common.TestDockerGitImage
	failingScriptBuild.Image.Name = common.TestDockerGitImage
	failingReleaseBuild.Image.Name = common.TestDockerGitImage

	tests := map[string]struct {
		jobResponse    common.JobResponse
		expectedOutput []string
		unwantedOutput []string
		errExpected    bool
	}{
		"Successful build with release and after_script step": {
			jobResponse: successfulBuild,
			expectedOutput: []string{
				"echo Hello World",
				"echo Release",
				"echo After Script",
			},
			errExpected: false,
		},
		"Failure on script step. Release is skipped. After script runs.": {
			jobResponse: failingScriptBuild,
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
			jobResponse: failingReleaseBuild,
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
			build := getTestBuild(t, func() (common.JobResponse, error) {
				return tt.jobResponse, nil
			})
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

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
				return
			}
			assert.NoError(t, err)
		})
	}
}

func testKubernetesTimeoutRunFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteLongRunningBuild)
	build.Image.Name = common.TestDockerGitImage
	build.RunnerInfo.Timeout = 10 // seconds
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	var buildError *common.BuildError
	assert.ErrorAs(t, err, &buildError)
	assert.Equal(t, common.JobExecutionTimeout, buildError.FailureReason)
}

func testKubernetesBuildFailFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteFailedBuild)
	build.Image.Name = common.TestDockerGitImage
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err, "error")
	var buildError *common.BuildError
	assert.ErrorAs(t, err, &buildError)
	assert.Contains(t, err.Error(), "command terminated with exit code 1")
	assert.Equal(t, 1, buildError.ExitCode)
}

func testKubernetesBuildCancelFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, func() (common.JobResponse, error) {
		return common.JobResponse{}, nil
	})
	buildtest.RunBuildWithCancel(
		t,
		build.Runner,
		func(build *common.Build) {
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
		},
	)
}

func testKubernetesBuildLogLimitExceededFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, func() (common.JobResponse, error) {
		return common.JobResponse{}, nil
	})
	buildtest.RunRemoteBuildWithJobOutputLimitExceeded(
		t,
		build.Runner,
		func(build *common.Build) {
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
		},
	)
}

func testKubernetesBuildMaskingFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, func() (common.JobResponse, error) {
		return common.JobResponse{}, nil
	})
	buildtest.RunBuildWithMasking(
		t,
		build.Runner,
		func(build *common.Build) {
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
		},
	)
}

func testKubernetesCustomClonePathFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	jobResponse, err := common.GetRemoteBuildResponse(
		"ls -al $CI_BUILDS_DIR/go/src/gitlab.com/gitlab-org/repo",
	)
	require.NoError(t, err)

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
			build := getTestBuild(t, func() (common.JobResponse, error) {
				return jobResponse, nil
			})
			build.Runner.Environment = []string{
				"GIT_CLONE_PATH=" + test.clonePath,
			}
			buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

			err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
			if test.expectedErr {
				var buildErr *common.BuildError
				assert.ErrorAs(t, err, &buildErr)
				return
			}

			assert.NoError(t, err)
		})
	}
}

func testKubernetesNoRootImageFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteSuccessfulBuildWithDumpedVariables)
	build.Image.Name = common.TestAlpineNoRootImage
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func testKubernetesMissingImageFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteFailedBuild)
	build.Image.Name = "some/non-existing/image"
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	var buildErr *common.BuildError
	assert.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "image pull failed")
}

func testKubernetesMissingTagFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteFailedBuild)
	build.Image.Name = "docker:missing-tag"
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	var buildErr *common.BuildError
	assert.ErrorAs(t, err, &buildErr)
	assert.Contains(t, err.Error(), "image pull failed")
}

func testKubernetesFailingToPullImageTwiceFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteFailedBuild)
	build.Image.Name = "some/non-existing/image"
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := runMultiPullPolicyBuild(t, build)

	var imagePullErr *pull.ImagePullError
	require.ErrorAs(t, err, &imagePullErr)
	assert.Equal(t, build.Image.Name, imagePullErr.Image)
}

func testKubernetesFailingToPullSvcImageTwiceFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteFailedBuild)
	build.Services = common.Services{
		{
			Name: "some/non-existing/image",
		},
	}
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := runMultiPullPolicyBuild(t, build)

	var imagePullErr *pull.ImagePullError
	require.ErrorAs(t, err, &imagePullErr)
	assert.Equal(t, build.Services[0].Name, imagePullErr.Image)
}

func testKubernetesFailingToPullHelperTwiceFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, common.GetRemoteFailedBuild)
	build.Runner.Kubernetes.HelperImage = "some/non-existing/image"
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := runMultiPullPolicyBuild(t, build)

	var imagePullErr *pull.ImagePullError
	require.ErrorAs(t, err, &imagePullErr)
	assert.Equal(t, build.Runner.Kubernetes.HelperImage, imagePullErr.Image)
}

func testOverwriteNamespaceNotMatchFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, func() (common.JobResponse, error) {
		return common.JobResponse{
			GitInfo: common.GitInfo{
				Sha: "1234567890",
			},
			Image: common.Image{
				Name: "test-image",
			},
			Variables: []common.JobVariable{
				{Key: kubernetes.NamespaceOverwriteVariableName, Value: "namespace"},
			},
		}, nil
	})
	build.Runner.Kubernetes.NamespaceOverwriteAllowed = "^not_a_match$"
	build.SystemInterrupt = make(chan os.Signal, 1)
	build.Image.Name = common.TestDockerGitImage
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func testOverwriteServiceAccountNotMatchFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, func() (common.JobResponse, error) {
		return common.JobResponse{
			GitInfo: common.GitInfo{
				Sha: "1234567890",
			},
			Image: common.Image{
				Name: "test-image",
			},
			Variables: []common.JobVariable{
				{Key: kubernetes.ServiceAccountOverwriteVariableName, Value: "service-account"},
			},
		}, nil
	})
	build.Runner.Kubernetes.ServiceAccountOverwriteAllowed = "^not_a_match$"
	build.SystemInterrupt = make(chan os.Signal, 1)
	build.Image.Name = common.TestDockerGitImage
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func testInteractiveTerminalFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	if os.Getenv("GITLAB_CI") == "true" {
		t.Skip("Skipping inside of GitLab CI check https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26421")
	}

	client := getTestKubeClusterClient(t)
	secrets, err := client.CoreV1().Secrets("default").List(metav1.ListOptions{})
	require.NoError(t, err)

	build := getTestBuild(t, func() (common.JobResponse, error) {
		return common.GetRemoteBuildResponse("sleep 5")
	})
	build.Image.Name = "docker:git"
	build.Runner.Kubernetes.BearerToken = string(secrets.Items[0].Data["token"])
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	sess, err := session.NewSession(nil)
	build.Session = sess

	outBuffer := bytes.NewBuffer(nil)
	outCh := make(chan string)

	go func() {
		err = build.Run(
			&common.Config{
				SessionServer: common.SessionServer{
					SessionTimeout: 2,
				},
			},
			&common.Trace{Writer: outBuffer},
		)
		require.NoError(t, err)

		outCh <- outBuffer.String()
	}()

	for build.Session.Mux() == nil {
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(5 * time.Second)

	srv := httptest.NewServer(build.Session.Mux())
	defer srv.Close()

	u := url.URL{
		Scheme: "ws",
		Host:   srv.Listener.Addr().String(),
		Path:   build.Session.Endpoint + "/exec",
	}
	headers := http.Header{
		"Authorization": []string{build.Session.Token},
	}
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), headers)
	defer func() {
		resp.Body.Close()
		if conn != nil {
			_ = conn.Close()
		}
	}()
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusSwitchingProtocols)

	out := <-outCh
	t.Log(out)

	assert.Contains(t, out, "Terminal is connected, will time out in 2s...")
}

func testKubernetesReplaceEnvFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")
	build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
	build.Image.Name = "$IMAGE:$VERSION"
	build.JobResponse.Variables = append(
		build.JobResponse.Variables,
		common.JobVariable{Key: "IMAGE", Value: "alpine"},
		common.JobVariable{Key: "VERSION", Value: "latest"},
	)
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)
	out, err := buildtest.RunBuildReturningOutput(t, build)
	require.NoError(t, err)
	assert.Contains(t, out, "alpine:latest")
}

func testKubernetesReplaceMissingEnvVarFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")
	build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
	build.Image.Name = "alpine:$NOT_EXISTING_VARIABLE"
	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "image pull failed: Failed to apply default image tag \"alpine:\"")
}

// This test reproduces the bug reported in https://gitlab.com/gitlab-org/gitlab-runner/issues/2583
func TestPrepareIssue2583(t *testing.T) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	namespace := "my_namespace"
	serviceAccount := "my_account"

	runnerConfig := &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Executor: "kubernetes",
			Kubernetes: &common.KubernetesConfig{
				Image:                          "an/image:latest",
				Namespace:                      namespace,
				NamespaceOverwriteAllowed:      ".*",
				ServiceAccount:                 serviceAccount,
				ServiceAccountOverwriteAllowed: ".*",
			},
		},
	}

	build := getTestBuild(t, func() (common.JobResponse, error) {
		return common.JobResponse{
			Variables: []common.JobVariable{
				{Key: kubernetes.NamespaceOverwriteVariableName, Value: "namespace"},
				{Key: kubernetes.ServiceAccountOverwriteVariableName, Value: "sa"},
			},
		}, nil
	})

	e := kubernetes.NewDefaultExecutorForTest()

	prepareOptions := common.ExecutorPrepareOptions{
		Config:  runnerConfig,
		Build:   build,
		Context: context.TODO(),
	}

	err := e.Prepare(prepareOptions)
	assert.NoError(t, err)
	assert.Equal(t, namespace, runnerConfig.Kubernetes.Namespace)
	assert.Equal(t, serviceAccount, runnerConfig.Kubernetes.ServiceAccount)
}

func TestHelperImageRegistryLogs(t *testing.T) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	for _, tt := range []bool{true, false} {
		t.Run(fmt.Sprintf("%s is %t", featureflags.GitLabRegistryHelperImage, tt), func(t *testing.T) {
			build := getTestBuild(t, common.GetRemoteSuccessfulBuild)
			build.Runner.Kubernetes.PullPolicy = common.StringOrArray{common.PullPolicyAlways}
			buildtest.SetBuildFeatureFlag(build, featureflags.GitLabRegistryHelperImage, tt)

			trace := bytes.Buffer{}
			err := build.Run(&common.Config{}, &common.Trace{Writer: &trace})
			require.NoError(t, err)

			if !tt {
				assert.Contains(
					t,
					trace.String(),
					helperimage.DockerHubWarningMessage,
				)
				return
			}
			assert.NotContains(
				t,
				trace.String(),
				helperimage.DockerHubWarningMessage,
			)
		})
	}
}

func TestDeletedPodSystemFailureDuringExecution(t *testing.T) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	tests := []struct {
		stage            string
		outputAssertions func(t *testing.T, out string, pod string)
	}{
		{
			stage: "step_", // Any script the user defined
			outputAssertions: func(t *testing.T, out string, pod string) {
				assert.Contains(
					t,
					out,
					fmt.Sprintf("ERROR: Job failed (system failure): pods %q not found", pod),
				)
			},
		},
		{
			stage: string(common.BuildStagePrepare),
			outputAssertions: func(t *testing.T, out string, pod string) {
				assert.Contains(
					t,
					out,
					"ERROR: Job failed (system failure):",
				)

				assert.Contains(
					t,
					out,
					fmt.Sprintf("pods %q not found", pod),
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.stage, func(t *testing.T) {
			build := getTestBuild(t, common.GetRemoteLongRunningBuild)

			// It's not possible to get this kind of information on the legacy execution path.
			buildtest.SetBuildFeatureFlag(build, featureflags.UseLegacyKubernetesExecutionStrategy, false)

			deletedPodCh := make(chan string)
			defer buildtest.OnStage(build, tt.stage, func() {
				client := getTestKubeClusterClient(t)
				pods, err := client.CoreV1().Pods("default").List(metav1.ListOptions{
					LabelSelector: labels.Set(build.Runner.Kubernetes.PodLabels).String(),
				})
				require.NoError(t, err)
				require.NotEmpty(t, pods.Items)
				pod := pods.Items[0]
				err = client.CoreV1().Pods("default").Delete(pod.Name, &metav1.DeleteOptions{})
				require.NoError(t, err)

				deletedPodCh <- pod.Name
			})()

			out, err := buildtest.RunBuildReturningOutput(t, build)
			assert.True(t, kubernetes.IsKubernetesPodNotFoundError(err), "expected err NotFound, but got %T", err)

			tt.outputAssertions(t, out, <-deletedPodCh)
		})
	}
}

func testKubernetesWithNonRootSecurityContext(t *testing.T, featureFlagName string, featureFlagValue bool) {
	helpers.SkipIntegrationTests(t, "kubectl", "cluster-info")

	build := getTestBuild(t, func() (common.JobResponse, error) {
		return common.GetRemoteBuildResponse("id")
	})
	build.Image.Name = common.TestAlpineNoRootImage

	runAsNonRoot := true
	runAsUser := int64(1895034)
	build.Runner.Kubernetes.PodSecurityContext = common.KubernetesPodSecurityContext{
		RunAsNonRoot: &runAsNonRoot,
		RunAsUser:    &runAsUser,
	}

	buildtest.SetBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	out, err := buildtest.RunBuildReturningOutput(t, build)
	assert.NoError(t, err)
	assert.Contains(t, out, fmt.Sprintf("uid=%d gid=0(root)", runAsUser))
}

func getTestBuild(t *testing.T, getJobResponse func() (common.JobResponse, error)) *common.Build {
	jobResponse, err := getJobResponse()
	assert.NoError(t, err)

	podUUID, err := helpers.GenerateRandomUUID(8)
	require.NoError(t, err)

	return &common.Build{
		JobResponse: jobResponse,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					Image:      common.TestAlpineImage,
					PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
					PodLabels: map[string]string{
						"test.k8s.gitlab.com/name": podUUID,
					},
				},
			},
		},
	}
}

func getTestKubeClusterClient(t *testing.T) *k8s.Clientset {
	config, err := kubernetes.GetKubeClientConfig(new(common.KubernetesConfig))
	require.NoError(t, err)
	client, err := k8s.NewForConfig(config)
	require.NoError(t, err)

	return client
}

func runMultiPullPolicyBuild(t *testing.T, build *common.Build) error {
	build.Runner.Kubernetes.PullPolicy = common.StringOrArray{
		common.PullPolicyAlways,
		common.PullPolicyIfNotPresent,
	}

	outBuffer := bytes.NewBuffer(nil)

	err := build.Run(&common.Config{}, &common.Trace{Writer: outBuffer})
	require.Error(t, err)
	var buildErr *common.BuildError
	assert.ErrorAs(t, err, &buildErr)

	assert.Regexp(
		t,
		`(?s).*WARNING: Failed to pull image with policy "Always": image pull failed:.*`+
			`Attempt #2: Trying "IfNotPresent" pull policy for "some\/non-existing\/image" image.*`+
			`WARNING: Failed to pull image with policy "IfNotPresent":.*`+
			`image pull failed:.*`,
		outBuffer.String(),
	)

	return err
}
