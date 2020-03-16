package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jpillora/backoff"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	api "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest/fake"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/helperimage"
	dns_test "gitlab.com/gitlab-org/gitlab-runner/helpers/dns/test"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/session"
	"gitlab.com/gitlab-org/gitlab-runner/session/proxy"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

type featureFlagTest func(t *testing.T, flagName string, flagValue bool)

func TestRunTestsWithFeatureFlag(t *testing.T) {
	tests := map[string]featureFlagTest{
		"testKubernetesSuccessRun":              testKubernetesSuccessRunFeatureFlag,
		"testKubernetesTimeoutRun":              testKubernetesTimeoutRunFeatureFlag,
		"testKubernetesBuildFail":               testKubernetesBuildFailFeatureFlag,
		"testKubernetesBuildAbort":              testKubernetesBuildAbortFeatureFlag,
		"testKubernetesBuildCancel":             testKubernetesBuildCancelFeatureFlag,
		"testVolumeMounts":                      testVolumeMountsFeatureFlag,
		"testVolumes":                           testVolumesFeatureFlag,
		"testSetupBuildPodServiceCreationError": testSetupBuildPodServiceCreationErrorFeatureFlag,
		"testKubernetesCustomClonePath":         testKubernetesCustomClonePathFeatureFlag,
		"testKubernetesNoRootImage":             testKubernetesNoRootImageFeatureFlag,
		"testKubernetesMissingImage":            testKubernetesMissingImageFeatureFlag,
		"testKubernetesMissingTag":              testKubernetesMissingTagFeatureFlag,
		"testOverwriteNamespaceNotMatch":        testOverwriteNamespaceNotMatchFeatureFlag,
		"testOverwriteServiceAccountNotMatch":   testOverwriteServiceAccountNotMatchFeatureFlag,
		"testInteractiveTerminal":               testInteractiveTerminalFeatureFlag,
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
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	assert.NoError(t, err)
	successfulBuild.Image.Name = common.TestDockerGitImage
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
	}
	setBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func testKubernetesTimeoutRunFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	longRunningBuild, err := common.GetRemoteLongRunningBuild()
	assert.NoError(t, err)
	longRunningBuild.Image.Name = common.TestDockerGitImage
	build := &common.Build{
		JobResponse: longRunningBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
	}
	build.RunnerInfo.Timeout = 10 //seconds
	setBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	var buildError *common.BuildError
	assert.True(t, errors.As(err, &buildError))
	assert.Equal(t, common.JobExecutionTimeout, buildError.FailureReason)
}

func testKubernetesBuildFailFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	failedBuild, err := common.GetRemoteFailedBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: failedBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
	}
	build.Image.Name = common.TestDockerGitImage
	setBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err, "error")
	assert.IsType(t, err, &common.BuildError{})
	assert.Contains(t, err.Error(), "command terminated with exit code 1")
}

func testKubernetesBuildAbortFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	failedBuild, err := common.GetRemoteFailedBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: failedBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
		SystemInterrupt: make(chan os.Signal, 1),
	}
	build.Image.Name = common.TestDockerGitImage
	setBuildFeatureFlag(build, featureFlagName, featureFlagValue)

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

func testKubernetesBuildCancelFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	failedBuild, err := common.GetRemoteFailedBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: failedBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
		SystemInterrupt: make(chan os.Signal, 1),
	}
	build.Image.Name = common.TestDockerGitImage
	setBuildFeatureFlag(build, featureFlagName, featureFlagValue)

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
	assert.EqualError(t, err, "canceled")
}

func testVolumeMountsFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	tests := map[string]struct {
		GlobalConfig *common.Config
		RunnerConfig common.RunnerConfig
		Build        *common.Build

		Expected []api.VolumeMount
	}{
		"no custom volumes": {
			GlobalConfig: &common.Config{},
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: []api.VolumeMount{
				{Name: "repo"},
			},
		},
		"custom volumes": {
			GlobalConfig: &common.Config{},
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Volumes: common.KubernetesVolumes{
							HostPaths: []common.KubernetesHostPath{
								{Name: "docker", MountPath: "/var/run/docker.sock", HostPath: "/var/run/docker.sock"},
							},
							PVCs: []common.KubernetesPVC{
								{Name: "PVC", MountPath: "/path/to/whatever"},
							},
							EmptyDirs: []common.KubernetesEmptyDir{
								{Name: "emptyDir", MountPath: "/path/to/empty/dir"},
							},
						},
					},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: []api.VolumeMount{
				{Name: "repo"},
				{Name: "docker", MountPath: "/var/run/docker.sock"},
				{Name: "PVC", MountPath: "/path/to/whatever"},
				{Name: "emptyDir", MountPath: "/path/to/empty/dir"},
			},
		},
		"custom volumes with read-only settings": {
			GlobalConfig: &common.Config{},
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Volumes: common.KubernetesVolumes{
							HostPaths: []common.KubernetesHostPath{
								{Name: "test", MountPath: "/opt/test/readonly", ReadOnly: true, HostPath: "/opt/test/rw"},
								{Name: "docker", MountPath: "/var/run/docker.sock"},
							},
							ConfigMaps: []common.KubernetesConfigMap{
								{Name: "configMap", MountPath: "/path/to/configmap", ReadOnly: true},
							},
							Secrets: []common.KubernetesSecret{
								{Name: "secret", MountPath: "/path/to/secret", ReadOnly: true},
							},
						},
					},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: []api.VolumeMount{
				{Name: "repo"},
				{Name: "test", MountPath: "/opt/test/readonly", ReadOnly: true},
				{Name: "docker", MountPath: "/var/run/docker.sock"},
				{Name: "secret", MountPath: "/path/to/secret", ReadOnly: true},
				{Name: "configMap", MountPath: "/path/to/configmap", ReadOnly: true},
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			e := &executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: executorOptions,
					Build:           tt.Build,
					Config:          tt.RunnerConfig,
				},
			}
			setBuildFeatureFlag(e.Build, featureFlagName, featureFlagValue)

			mounts := e.getVolumeMounts()
			for _, expected := range tt.Expected {
				assert.Contains(t, mounts, expected, "Expected volumeMount definition for %s was not found", expected.Name)
			}
		})
	}
}

func testVolumesFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	tests := map[string]struct {
		GlobalConfig *common.Config
		RunnerConfig common.RunnerConfig
		Build        *common.Build

		Expected []api.Volume
	}{
		"no custom volumes": {
			GlobalConfig: &common.Config{},
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: []api.Volume{
				{Name: "repo", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{}}},
			},
		},
		"custom volumes": {
			GlobalConfig: &common.Config{},
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Volumes: common.KubernetesVolumes{
							HostPaths: []common.KubernetesHostPath{
								{Name: "docker", MountPath: "/var/run/docker.sock"},
								{Name: "host-path", MountPath: "/path/two", HostPath: "/path/one"},
							},
							PVCs: []common.KubernetesPVC{
								{Name: "PVC", MountPath: "/path/to/whatever"},
							},
							ConfigMaps: []common.KubernetesConfigMap{
								{Name: "ConfigMap", MountPath: "/path/to/config", Items: map[string]string{"key_1": "/path/to/key_1"}},
							},
							Secrets: []common.KubernetesSecret{
								{Name: "secret", MountPath: "/path/to/secret", ReadOnly: true, Items: map[string]string{"secret_1": "/path/to/secret_1"}},
							},
							EmptyDirs: []common.KubernetesEmptyDir{
								{Name: "emptyDir", MountPath: "/path/to/empty/dir", Medium: "Memory"},
							},
						},
					},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: []api.Volume{
				{Name: "repo", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{}}},
				{Name: "docker", VolumeSource: api.VolumeSource{HostPath: &api.HostPathVolumeSource{Path: "/var/run/docker.sock"}}},
				{Name: "host-path", VolumeSource: api.VolumeSource{HostPath: &api.HostPathVolumeSource{Path: "/path/one"}}},
				{Name: "PVC", VolumeSource: api.VolumeSource{PersistentVolumeClaim: &api.PersistentVolumeClaimVolumeSource{ClaimName: "PVC"}}},
				{Name: "emptyDir", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{Medium: "Memory"}}},
				{
					Name: "ConfigMap",
					VolumeSource: api.VolumeSource{
						ConfigMap: &api.ConfigMapVolumeSource{
							LocalObjectReference: api.LocalObjectReference{Name: "ConfigMap"},
							Items:                []api.KeyToPath{{Key: "key_1", Path: "/path/to/key_1"}},
						},
					},
				},
				{
					Name: "secret",
					VolumeSource: api.VolumeSource{
						Secret: &api.SecretVolumeSource{
							SecretName: "secret",
							Items:      []api.KeyToPath{{Key: "secret_1", Path: "/path/to/secret_1"}},
						},
					},
				},
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			e := &executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: executorOptions,
					Build:           tt.Build,
					Config:          tt.RunnerConfig,
				},
				configMap: fakeConfigMap(),
			}
			setBuildFeatureFlag(e.Build, featureFlagName, featureFlagValue)

			volumes := e.getVolumes()
			for _, expected := range tt.Expected {
				assert.Contains(t, volumes, expected, "Expected volume definition for %s was not found", expected.Name)
			}
		})
	}
}

func testSetupBuildPodServiceCreationErrorFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	version, _ := testVersionAndCodec()
	helperImageInfo, err := helperimage.Get(common.REVISION, helperimage.Config{
		OSType:       helperimage.OSTypeLinux,
		Architecture: "amd64",
	})
	require.NoError(t, err)

	runnerConfig := common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Kubernetes: &common.KubernetesConfig{
				Namespace:   "default",
				HelperImage: "custom/helper-image",
			},
		},
	}

	fakeRoundTripper := func(req *http.Request) (*http.Response, error) {
		body, err := ioutil.ReadAll(req.Body)
		if !assert.NoError(t, err, "failed to read request body") {
			return nil, err
		}

		p := new(api.Pod)
		err = json.Unmarshal(body, p)
		if !assert.NoError(t, err, "failed to read request body") {
			return nil, err
		}

		if req.URL.Path == "/api/v1/namespaces/default/services" {
			return nil, fmt.Errorf("foobar")
		}

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body: FakeReadCloser{
				Reader: bytes.NewBuffer(body),
			},
		}
		resp.Header = make(http.Header)
		resp.Header.Add("Content-Type", "application/json")

		return resp, nil
	}

	mockFc := &mockFeatureChecker{}
	mockFc.On("IsHostAliasSupported").Return(true, nil)
	ex := executor{
		kubeClient: testKubernetesClient(version, fake.CreateHTTPClient(fakeRoundTripper)),
		options: &kubernetesOptions{
			Image: common.Image{
				Name:  "test-image",
				Ports: []common.Port{{Number: 80}},
			},
			Services: common.Services{
				{
					Name:  "test-service",
					Alias: "custom_name",
					Ports: []common.Port{
						{
							Number:   81,
							Name:     "custom_port_name",
							Protocol: "http",
						},
					},
				},
			},
		},
		AbstractExecutor: executors.AbstractExecutor{
			Config:     runnerConfig,
			BuildShell: &common.ShellConfiguration{},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					Variables: []common.JobVariable{},
				},
				Runner: &runnerConfig,
			},
			ProxyPool: proxy.NewPool(),
		},
		helperImageInfo: helperImageInfo,
		featureChecker:  mockFc,
		configMap:       fakeConfigMap(),
	}
	setBuildFeatureFlag(ex.Build, featureFlagName, featureFlagValue)

	err = ex.prepareOverwrites(make(common.JobVariables, 0))
	assert.NoError(t, err)

	err = ex.setupBuildPod()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error creating the proxy service")
}

func testKubernetesCustomClonePathFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
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
						Executor: "kubernetes",
						Kubernetes: &common.KubernetesConfig{
							Image:      common.TestAlpineImage,
							PullPolicy: common.PullPolicyIfNotPresent,
						},
						Environment: []string{
							"GIT_CLONE_PATH=" + test.clonePath,
						},
					},
				},
			}
			setBuildFeatureFlag(build, featureFlagName, featureFlagValue)

			err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
			assert.IsType(t, test.expectedErrorType, err)
		})
	}
}

func testKubernetesNoRootImageFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	successfulBuild, err := common.GetRemoteSuccessfulBuildWithDumpedVariables()

	assert.NoError(t, err)
	successfulBuild.Image.Name = common.TestAlpineNoRootImage
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					Image:      common.TestAlpineImage,
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
	}
	setBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func testKubernetesMissingImageFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	failedBuild, err := common.GetRemoteFailedBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: failedBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor:   "kubernetes",
				Kubernetes: &common.KubernetesConfig{},
			},
		},
	}
	build.Image.Name = "some/non-existing/image"
	setBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.IsType(t, err, &common.BuildError{})
	assert.Contains(t, err.Error(), "image pull failed")
}

func testKubernetesMissingTagFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	failedBuild, err := common.GetRemoteFailedBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: failedBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor:   "kubernetes",
				Kubernetes: &common.KubernetesConfig{},
			},
		},
	}
	build.Image.Name = "docker:missing-tag"
	setBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.IsType(t, err, &common.BuildError{})
	assert.Contains(t, err.Error(), "image pull failed")
}

func testOverwriteNamespaceNotMatchFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	build := &common.Build{
		JobResponse: common.JobResponse{
			GitInfo: common.GitInfo{
				Sha: "1234567890",
			},
			Image: common.Image{
				Name: "test-image",
			},
			Variables: []common.JobVariable{
				{Key: NamespaceOverwriteVariableName, Value: "namespace"},
			},
		},
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					NamespaceOverwriteAllowed: "^not_a_match$",
					PullPolicy:                common.PullPolicyIfNotPresent,
				},
			},
		},
		SystemInterrupt: make(chan os.Signal, 1),
	}
	build.Image.Name = common.TestDockerGitImage
	setBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func testOverwriteServiceAccountNotMatchFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	build := &common.Build{
		JobResponse: common.JobResponse{
			GitInfo: common.GitInfo{
				Sha: "1234567890",
			},
			Image: common.Image{
				Name: "test-image",
			},
			Variables: []common.JobVariable{
				{Key: ServiceAccountOverwriteVariableName, Value: "service-account"},
			},
		},
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					ServiceAccountOverwriteAllowed: "^not_a_match$",
					PullPolicy:                     common.PullPolicyIfNotPresent,
				},
			},
		},
		SystemInterrupt: make(chan os.Signal, 1),
	}
	build.Image.Name = common.TestDockerGitImage
	setBuildFeatureFlag(build, featureFlagName, featureFlagValue)

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func testInteractiveTerminalFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	client, err := getKubeClient(&common.KubernetesConfig{}, &overwrites{})
	require.NoError(t, err)
	secrets, err := client.CoreV1().Secrets("default").List(metav1.ListOptions{})
	require.NoError(t, err)

	successfulBuild, err := common.GetRemoteBuildResponse("sleep 5")
	require.NoError(t, err)
	successfulBuild.Image.Name = "docker:git"
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					BearerToken: string(secrets.Items[0].Data["token"]),
				},
			},
		},
	}
	setBuildFeatureFlag(build, featureFlagName, featureFlagValue)

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

func TestCleanup(t *testing.T) {
	version, _ := testVersionAndCodec()
	objectMeta := metav1.ObjectMeta{Name: "test-resource", Namespace: "test-ns"}
	podsEndpointURI := "/api/" + version + "/namespaces/" + objectMeta.Namespace + "/pods/" + objectMeta.Name
	servicesEndpointURI := "/api/" + version + "/namespaces/" + objectMeta.Namespace + "/services/" + objectMeta.Name
	secretsEndpointURI := "/api/" + version + "/namespaces/" + objectMeta.Namespace + "/secrets/" + objectMeta.Name
	configMapsEndpointURI := "/api/" + version + "/namespaces/" + objectMeta.Namespace + "/configmaps/" + objectMeta.Name

	tests := []struct {
		Name        string
		Pod         *api.Pod
		ConfigMap   *api.ConfigMap
		Credentials *api.Secret
		ClientFunc  func(*http.Request) (*http.Response, error)
		Services    []api.Service
		Error       bool
	}{
		{
			Name: "Proper Cleanup",
			Pod:  &api.Pod{ObjectMeta: objectMeta},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == http.MethodDelete && p == podsEndpointURI:
					return fakeKubeDeleteResponse(http.StatusOK), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
		},
		{
			Name: "Delete failure",
			Pod:  &api.Pod{ObjectMeta: objectMeta},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("delete failed")
			},
			Error: true,
		},
		{
			Name: "POD already deleted",
			Pod:  &api.Pod{ObjectMeta: objectMeta},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == http.MethodDelete && p == podsEndpointURI:
					return fakeKubeDeleteResponse(http.StatusNotFound), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
			Error: true,
		},
		{
			Name:        "POD creation failed, Secrets provided",
			Pod:         nil, // a failed POD create request will cause a nil Pod
			Credentials: &api.Secret{ObjectMeta: objectMeta},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == http.MethodDelete && p == secretsEndpointURI:
					return fakeKubeDeleteResponse(http.StatusNotFound), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
			Error: true,
		},
		{
			Name:     "POD created, Services created",
			Pod:      &api.Pod{ObjectMeta: objectMeta},
			Services: []api.Service{{ObjectMeta: objectMeta}},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == http.MethodDelete && ((p == servicesEndpointURI) || (p == podsEndpointURI)):
					return fakeKubeDeleteResponse(http.StatusOK), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
		},
		{
			Name:     "POD created, Services creation failed",
			Pod:      &api.Pod{ObjectMeta: objectMeta},
			Services: []api.Service{{ObjectMeta: objectMeta}},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == http.MethodDelete && p == servicesEndpointURI:
					return fakeKubeDeleteResponse(http.StatusNotFound), nil
				case m == http.MethodDelete && p == podsEndpointURI:
					return fakeKubeDeleteResponse(http.StatusOK), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
			Error: true,
		},
		{
			Name:     "POD creation failed, Services created",
			Pod:      nil, // a failed POD create request will cause a nil Pod
			Services: []api.Service{{ObjectMeta: objectMeta}},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == http.MethodDelete && p == servicesEndpointURI:
					return fakeKubeDeleteResponse(http.StatusOK), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
		},
		{
			Name:     "POD creation failed, Services cleanup failed",
			Pod:      nil, // a failed POD create request will cause a nil Pod
			Services: []api.Service{{ObjectMeta: objectMeta}},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == http.MethodDelete && p == servicesEndpointURI:
					return fakeKubeDeleteResponse(http.StatusNotFound), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
			Error: true,
		},
		{
			Name:      "ConfigMap cleanup",
			ConfigMap: &api.ConfigMap{ObjectMeta: objectMeta},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == http.MethodDelete && p == configMapsEndpointURI:
					return fakeKubeDeleteResponse(http.StatusOK), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
		},
		{
			Name:      "ConfigMap cleanup failed",
			ConfigMap: &api.ConfigMap{ObjectMeta: objectMeta},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == http.MethodDelete && p == configMapsEndpointURI:
					return fakeKubeDeleteResponse(http.StatusNotFound), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
			Error: true,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			ex := executor{
				kubeClient:  testKubernetesClient(version, fake.CreateHTTPClient(test.ClientFunc)),
				pod:         test.Pod,
				credentials: test.Credentials,
				services:    test.Services,
				configMap:   test.ConfigMap,
			}
			ex.configurationOverwrites = &overwrites{namespace: "test-ns"}
			errored := false
			buildTrace := FakeBuildTrace{
				testWriter{
					call: func(b []byte) (int, error) {
						if !errored {
							if s := string(b); strings.Contains(s, "Error cleaning up") {
								errored = true
							} else if test.Error {
								t.Errorf("expected failure. got: '%s'", string(b))
							}
						}
						return len(b), nil
					},
				},
			}
			ex.AbstractExecutor.Trace = buildTrace
			ex.AbstractExecutor.BuildLogger = common.NewBuildLogger(buildTrace, logrus.WithFields(logrus.Fields{}))

			ex.Cleanup()

			if test.Error && !errored {
				t.Errorf("expected cleanup to fail but it didn't")
			} else if !test.Error && errored {
				t.Errorf("expected cleanup not to fail but it did")
			}
		})
	}
}

func TestPrepare(t *testing.T) {
	tests := []struct {
		GlobalConfig *common.Config
		RunnerConfig *common.RunnerConfig
		Build        *common.Build

		Expected *executor
		Error    bool
	}{
		{
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host:               "test-server",
						ServiceCPULimit:    "100m",
						ServiceMemoryLimit: "200Mi",
						CPULimit:           "1.5",
						MemoryLimit:        "4Gi",
						HelperCPULimit:     "50m",
						HelperMemoryLimit:  "100Mi",
						Privileged:         true,
						PullPolicy:         "if-not-present",
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
					Image: common.Image{
						Name: "test-image",
					},
					Variables: []common.JobVariable{
						{Key: "privileged", Value: "true"},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: &overwrites{
					namespace:   "default",
					cpuLimit:    "1.5",
					memoryLimit: "4Gi",
				},
				serviceLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("100m"),
					api.ResourceMemory: resource.MustParse("200Mi"),
				},
				buildLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("1.5"),
					api.ResourceMemory: resource.MustParse("4Gi"),
				},
				helperLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("50m"),
					api.ResourceMemory: resource.MustParse("100Mi"),
				},
				serviceRequests: api.ResourceList{},
				buildRequests:   api.ResourceList{},
				helperRequests:  api.ResourceList{},
				pullPolicy:      "IfNotPresent",
			},
		},
		{
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host:                           "test-server",
						ServiceAccount:                 "default",
						ServiceAccountOverwriteAllowed: ".*",
						BearerTokenOverwriteAllowed:    true,
						ServiceCPULimit:                "100m",
						ServiceMemoryLimit:             "200Mi",
						CPULimit:                       "1.5",
						MemoryLimit:                    "4Gi",
						HelperCPULimit:                 "50m",
						HelperMemoryLimit:              "100Mi",
						ServiceCPURequest:              "99m",
						ServiceMemoryRequest:           "5Mi",
						CPURequest:                     "1",
						MemoryRequest:                  "1.5Gi",
						HelperCPURequest:               "0.5m",
						HelperMemoryRequest:            "42Mi",
						Privileged:                     false,
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
					Image: common.Image{
						Name: "test-image",
					},
					Variables: []common.JobVariable{
						{Key: ServiceAccountOverwriteVariableName, Value: "not-default"},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: &overwrites{
					namespace:      "default",
					serviceAccount: "not-default",
					cpuLimit:       "1.5",
					memoryLimit:    "4Gi",
					cpuRequest:     "1",
					memoryRequest:  "1.5Gi",
				},
				serviceLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("100m"),
					api.ResourceMemory: resource.MustParse("200Mi"),
				},
				buildLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("1.5"),
					api.ResourceMemory: resource.MustParse("4Gi"),
				},
				helperLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("50m"),
					api.ResourceMemory: resource.MustParse("100Mi"),
				},
				serviceRequests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("99m"),
					api.ResourceMemory: resource.MustParse("5Mi"),
				},
				buildRequests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("1"),
					api.ResourceMemory: resource.MustParse("1.5Gi"),
				},
				helperRequests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("0.5m"),
					api.ResourceMemory: resource.MustParse("42Mi"),
				},
			},
			Error: false,
		},

		{
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host:                           "test-server",
						ServiceAccount:                 "default",
						ServiceAccountOverwriteAllowed: "allowed-.*",
						ServiceCPULimit:                "100m",
						ServiceMemoryLimit:             "200Mi",
						CPULimit:                       "1.5",
						MemoryLimit:                    "4Gi",
						HelperCPULimit:                 "50m",
						HelperMemoryLimit:              "100Mi",
						ServiceCPURequest:              "99m",
						ServiceMemoryRequest:           "5Mi",
						CPURequest:                     "1",
						MemoryRequest:                  "1.5Gi",
						HelperCPURequest:               "0.5m",
						HelperMemoryRequest:            "42Mi",
						Privileged:                     false,
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
					Image: common.Image{
						Name: "test-image",
					},
					Variables: []common.JobVariable{
						{Key: ServiceAccountOverwriteVariableName, Value: "not-default"},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: &overwrites{
					namespace:     "namespacee",
					cpuLimit:      "1.5",
					memoryLimit:   "4Gi",
					cpuRequest:    "1",
					memoryRequest: "1.5Gi",
				},
				serviceLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("100m"),
					api.ResourceMemory: resource.MustParse("200Mi"),
				},
				buildLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("1.5"),
					api.ResourceMemory: resource.MustParse("4Gi"),
				},
				helperLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("50m"),
					api.ResourceMemory: resource.MustParse("100Mi"),
				},
				serviceRequests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("99m"),
					api.ResourceMemory: resource.MustParse("5Mi"),
				},
				buildRequests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("1"),
					api.ResourceMemory: resource.MustParse("1.5Gi"),
				},
				helperRequests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("0.5m"),
					api.ResourceMemory: resource.MustParse("42Mi"),
				},
			},
			Error: true,
		},
		{
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host:                           "test-server",
						Namespace:                      "namespace",
						ServiceAccount:                 "a_service_account",
						ServiceAccountOverwriteAllowed: ".*",
						NamespaceOverwriteAllowed:      "^n.*?e$",
						ServiceCPULimit:                "100m",
						ServiceMemoryLimit:             "200Mi",
						CPULimit:                       "1.5",
						MemoryLimit:                    "4Gi",
						HelperCPULimit:                 "50m",
						HelperMemoryLimit:              "100Mi",
						ServiceCPURequest:              "99m",
						ServiceMemoryRequest:           "5Mi",
						CPURequest:                     "1",
						MemoryRequest:                  "1.5Gi",
						HelperCPURequest:               "0.5m",
						HelperMemoryRequest:            "42Mi",
						Privileged:                     false,
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
					Image: common.Image{
						Name: "test-image",
					},
					Variables: []common.JobVariable{
						{Key: NamespaceOverwriteVariableName, Value: "namespacee"},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: &overwrites{
					namespace:      "namespacee",
					serviceAccount: "a_service_account",
					cpuLimit:       "1.5",
					memoryLimit:    "4Gi",
					cpuRequest:     "1",
					memoryRequest:  "1.5Gi",
				},
				serviceLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("100m"),
					api.ResourceMemory: resource.MustParse("200Mi"),
				},
				buildLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("1.5"),
					api.ResourceMemory: resource.MustParse("4Gi"),
				},
				helperLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("50m"),
					api.ResourceMemory: resource.MustParse("100Mi"),
				},
				serviceRequests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("99m"),
					api.ResourceMemory: resource.MustParse("5Mi"),
				},
				buildRequests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("1"),
					api.ResourceMemory: resource.MustParse("1.5Gi"),
				},
				helperRequests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("0.5m"),
					api.ResourceMemory: resource.MustParse("42Mi"),
				},
			},
			Error: true,
		},
		{
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "namespace",
						Host:      "test-server",
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
					Image: common.Image{
						Name: "test-image",
					},
					Variables: []common.JobVariable{
						{Key: NamespaceOverwriteVariableName, Value: "namespace"},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: &overwrites{namespace: "namespace"},
				serviceLimits:           api.ResourceList{},
				buildLimits:             api.ResourceList{},
				helperLimits:            api.ResourceList{},
				serviceRequests:         api.ResourceList{},
				buildRequests:           api.ResourceList{},
				helperRequests:          api.ResourceList{},
			},
		},
		{
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image: "test-image",
						Host:  "test-server",
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: &overwrites{namespace: "default"},
				serviceLimits:           api.ResourceList{},
				buildLimits:             api.ResourceList{},
				helperLimits:            api.ResourceList{},
				serviceRequests:         api.ResourceList{},
				buildRequests:           api.ResourceList{},
				helperRequests:          api.ResourceList{},
			},
		},
		{
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host: "test-server",
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
					Image: common.Image{
						Name:       "test-image",
						Entrypoint: []string{"/init", "run"},
					},
					Services: common.Services{
						{
							Name:       "test-service",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
						},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name:       "test-image",
						Entrypoint: []string{"/init", "run"},
					},
					Services: common.Services{
						{
							Name:       "test-service",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
						},
					},
				},
				configurationOverwrites: &overwrites{namespace: "default"},
				serviceLimits:           api.ResourceList{},
				buildLimits:             api.ResourceList{},
				helperLimits:            api.ResourceList{},
				serviceRequests:         api.ResourceList{},
				buildRequests:           api.ResourceList{},
				helperRequests:          api.ResourceList{},
			},
		},
		{
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host: "test-server",
						Services: []common.Service{
							{Name: "test-service-k8s"},
							{Name: "test-service-k8s2"},
							{Name: ""},
						},
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
					Image: common.Image{
						Name:       "test-image",
						Entrypoint: []string{"/init", "run"},
					},
					Services: common.Services{
						{
							Name:       "test-service",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
						},
						{
							Name: "",
						},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name:       "test-image",
						Entrypoint: []string{"/init", "run"},
					},
					Services: common.Services{
						{
							Name: "test-service-k8s",
						},
						{
							Name: "test-service-k8s2",
						},
						{
							Name:       "test-service",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
						},
					},
				},
				configurationOverwrites: &overwrites{namespace: "default"},
				serviceLimits:           api.ResourceList{},
				buildLimits:             api.ResourceList{},
				helperLimits:            api.ResourceList{},
				serviceRequests:         api.ResourceList{},
				buildRequests:           api.ResourceList{},
				helperRequests:          api.ResourceList{},
			},
		},
	}

	for index, test := range tests {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			e := &executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: executorOptions,
				},
			}

			prepareOptions := common.ExecutorPrepareOptions{
				Config:  test.RunnerConfig,
				Build:   test.Build,
				Context: context.TODO(),
			}

			err := e.Prepare(prepareOptions)

			if err != nil {
				assert.False(t, test.Build.IsSharedEnv())
				if test.Error {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
				if !test.Error {
					t.Errorf("Got error. Expected: %v", test.Expected)
				}
				return
			}

			// Set this to nil so we aren't testing the functionality of the
			// base AbstractExecutor's Prepare method
			e.AbstractExecutor = executors.AbstractExecutor{}

			// TODO: Improve this so we don't have to nil-ify the kubeClient.
			// It currently contains some moving parts that are failing, meaning
			// we'll need to mock _something_
			e.kubeClient = nil
			e.featureChecker = nil
			assert.Equal(t, test.Expected, e)
		})
	}
}

// This test reproduces the bug reported in https://gitlab.com/gitlab-org/gitlab-runner/issues/2583
func TestPrepareIssue2583(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

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

	build := &common.Build{
		JobResponse: common.JobResponse{
			Variables: []common.JobVariable{
				{Key: NamespaceOverwriteVariableName, Value: "namespace"},
				{Key: ServiceAccountOverwriteVariableName, Value: "sa"},
			},
		},
		Runner: &common.RunnerConfig{},
	}

	e := &executor{
		AbstractExecutor: executors.AbstractExecutor{
			ExecutorOptions: executorOptions,
		},
	}

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

func TestSetupCredentials(t *testing.T) {
	version, _ := testVersionAndCodec()

	type testDef struct {
		RunnerCredentials *common.RunnerCredentials
		Credentials       []common.Credentials
		VerifyFn          func(*testing.T, testDef, *api.Secret)
	}
	tests := map[string]testDef{
		"no credentials": {
			// don't execute VerifyFn
			VerifyFn: nil,
		},
		"registry credentials": {
			Credentials: []common.Credentials{
				{
					Type:     "registry",
					URL:      "http://example.com",
					Username: "user",
					Password: "password",
				},
			},
			VerifyFn: func(t *testing.T, test testDef, secret *api.Secret) {
				assert.Equal(t, api.SecretTypeDockercfg, secret.Type)
				assert.NotEmpty(t, secret.Data[api.DockerConfigKey])
			},
		},
		"other credentials": {
			Credentials: []common.Credentials{
				{
					Type:     "other",
					URL:      "http://example.com",
					Username: "user",
					Password: "password",
				},
			},
			// don't execute VerifyFn
			VerifyFn: nil,
		},
		"non-DNS-1123-compatible-token": {
			RunnerCredentials: &common.RunnerCredentials{
				Token: "ToK3_?OF",
			},
			Credentials: []common.Credentials{
				{
					Type:     "registry",
					URL:      "http://example.com",
					Username: "user",
					Password: "password",
				},
			},
			VerifyFn: func(t *testing.T, test testDef, secret *api.Secret) {
				dns_test.AssertRFC1123Compatibility(t, secret.GetGenerateName())
			},
		},
	}

	executed := false
	fakeClientRoundTripper := func(test testDef) func(req *http.Request) (*http.Response, error) {
		return func(req *http.Request) (resp *http.Response, err error) {
			podBytes, err := ioutil.ReadAll(req.Body)
			executed = true

			if err != nil {
				t.Errorf("failed to read request body: %s", err.Error())
				return
			}

			p := new(api.Secret)

			err = json.Unmarshal(podBytes, p)

			if err != nil {
				t.Errorf("error decoding pod: %s", err.Error())
				return
			}

			if test.VerifyFn != nil {
				test.VerifyFn(t, test, p)
			}

			resp = &http.Response{StatusCode: http.StatusOK, Body: FakeReadCloser{
				Reader: bytes.NewBuffer(podBytes),
			}}
			resp.Header = make(http.Header)
			resp.Header.Add("Content-Type", "application/json")

			return
		}
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			ex := executor{
				kubeClient: testKubernetesClient(version, fake.CreateHTTPClient(fakeClientRoundTripper(test))),
				options:    &kubernetesOptions{},
				AbstractExecutor: executors.AbstractExecutor{
					Config: common.RunnerConfig{
						RunnerSettings: common.RunnerSettings{
							Kubernetes: &common.KubernetesConfig{
								Namespace: "default",
							},
						},
					},
					BuildShell: &common.ShellConfiguration{},
					Build: &common.Build{
						JobResponse: common.JobResponse{
							Variables:   []common.JobVariable{},
							Credentials: test.Credentials,
						},
						Runner: &common.RunnerConfig{},
					},
				},
			}

			if test.RunnerCredentials != nil {
				ex.Build.Runner = &common.RunnerConfig{
					RunnerCredentials: *test.RunnerCredentials,
				}
			}

			executed = false

			err := ex.prepareOverwrites(make(common.JobVariables, 0))
			assert.NoError(t, err)

			err = ex.setupCredentials()
			assert.NoError(t, err)

			if test.VerifyFn != nil {
				assert.True(t, executed)
			} else {
				assert.False(t, executed)
			}
		})
	}
}

type setupBuildPodTestDef struct {
	RunnerConfig             common.RunnerConfig
	Variables                []common.JobVariable
	Options                  *kubernetesOptions
	PrepareFn                func(*testing.T, setupBuildPodTestDef, *executor)
	VerifyFn                 func(*testing.T, setupBuildPodTestDef, *api.Pod)
	VerifyExecutorFn         func(*testing.T, setupBuildPodTestDef, *executor)
	VerifySetupBuildPodErrFn func(*testing.T, error)
}

type setupBuildPodFakeRoundTripper struct {
	t        *testing.T
	test     setupBuildPodTestDef
	executed bool
}

func (rt *setupBuildPodFakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.executed = true
	podBytes, err := ioutil.ReadAll(req.Body)
	if !assert.NoError(rt.t, err, "failed to read request body") {
		return nil, err
	}

	p := new(api.Pod)
	err = json.Unmarshal(podBytes, p)
	if !assert.NoError(rt.t, err, "failed to read request body") {
		return nil, err
	}

	if rt.test.VerifyFn != nil {
		rt.test.VerifyFn(rt.t, rt.test, p)
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: FakeReadCloser{
			Reader: bytes.NewBuffer(podBytes),
		},
	}
	resp.Header = make(http.Header)
	resp.Header.Add("Content-Type", "application/json")

	return resp, nil
}

func TestSetupBuildPod(t *testing.T) {
	version, _ := testVersionAndCodec()
	testErr := errors.New("fail")

	tests := map[string]setupBuildPodTestDef{
		"passes node selector setting": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
						NodeSelector: map[string]string{
							"a-selector":       "first",
							"another-selector": "second",
						},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Equal(t, test.RunnerConfig.RunnerSettings.Kubernetes.NodeSelector, pod.Spec.NodeSelector)
			},
		},
		"uses configured credentials": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
					},
				},
			},
			PrepareFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				e.credentials = &api.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "job-credentials",
					},
				}
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				secrets := []api.LocalObjectReference{{Name: "job-credentials"}}
				assert.Equal(t, secrets, pod.Spec.ImagePullSecrets)
			},
		},
		"uses configured image pull secrets": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
						ImagePullSecrets: []string{
							"docker-registry-credentials",
						},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				secrets := []api.LocalObjectReference{{Name: "docker-registry-credentials"}}
				assert.Equal(t, secrets, pod.Spec.ImagePullSecrets)
			},
		},
		"configures helper container": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				hasHelper := false
				for _, c := range pod.Spec.Containers {
					if c.Name == "helper" {
						hasHelper = true
					}
				}
				assert.True(t, hasHelper)
			},
		},
		"uses configured helper image": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace:   "default",
						HelperImage: "custom/helper-image",
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				for _, c := range pod.Spec.Containers {
					if c.Name == "helper" {
						assert.Equal(t, test.RunnerConfig.RunnerSettings.Kubernetes.HelperImage, c.Image)
					}
				}
			},
		},
		"expands variables for pod labels": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
						PodLabels: map[string]string{
							"test":    "label",
							"another": "label",
							"var":     "$test",
						},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Equal(t, map[string]string{
					"test":    "label",
					"another": "label",
					"var":     "sometestvar",
					"pod":     pod.GenerateName,
				}, pod.ObjectMeta.Labels)
			},
			Variables: []common.JobVariable{
				{Key: "test", Value: "sometestvar"},
			},
		},
		"expands variables for pod annotations": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
						PodAnnotations: map[string]string{
							"test":    "annotation",
							"another": "annotation",
							"var":     "$test",
						},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Equal(t, map[string]string{
					"test":    "annotation",
					"another": "annotation",
					"var":     "sometestvar",
				}, pod.ObjectMeta.Annotations)
			},
			Variables: []common.JobVariable{
				{Key: "test", Value: "sometestvar"},
			},
		},
		"expands variables for helper image": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace:   "default",
						HelperImage: "custom/helper-image:${CI_RUNNER_REVISION}",
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				for _, c := range pod.Spec.Containers {
					if c.Name == "helper" {
						assert.Equal(t, "custom/helper-image:HEAD", c.Image)
					}
				}
			},
		},
		"support setting kubernetes pod taint tolerations": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
						NodeTolerations: map[string]string{
							"node-role.kubernetes.io/master": "NoSchedule",
							"custom.toleration=value":        "NoSchedule",
							"empty.value=":                   "PreferNoSchedule",
							"onlyKey":                        "",
						},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				expectedTolerations := []api.Toleration{
					{
						Key:      "node-role.kubernetes.io/master",
						Operator: api.TolerationOpExists,
						Effect:   api.TaintEffectNoSchedule,
					},
					{
						Key:      "custom.toleration",
						Operator: api.TolerationOpEqual,
						Value:    "value",
						Effect:   api.TaintEffectNoSchedule,
					},
					{

						Key:      "empty.value",
						Operator: api.TolerationOpEqual,
						Value:    "",
						Effect:   api.TaintEffectPreferNoSchedule,
					},
					{
						Key:      "onlyKey",
						Operator: api.TolerationOpExists,
						Effect:   "",
					},
				}
				assert.ElementsMatch(t, expectedTolerations, pod.Spec.Tolerations)
			},
		},
		"supports extended docker configuration for image and services": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace:   "default",
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: common.Image{
					Name:       "test-image",
					Entrypoint: []string{"/init", "run"},
				},
				Services: common.Services{
					{
						Name:       "test-service",
						Entrypoint: []string{"/init", "run"},
						Command:    []string{"application", "--debug"},
					},
					{
						Name:    "test-service-2",
						Command: []string{"application", "--debug"},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				require.Len(t, pod.Spec.Containers, 4)

				assert.Equal(t, "build", pod.Spec.Containers[0].Name)
				assert.Equal(t, "test-image", pod.Spec.Containers[0].Image)
				assert.Equal(t, []string{"/init", "run"}, pod.Spec.Containers[0].Command)
				assert.Empty(t, pod.Spec.Containers[0].Args, "Build container args should be empty")

				assert.Equal(t, "helper", pod.Spec.Containers[1].Name)
				assert.Equal(t, "custom/helper-image", pod.Spec.Containers[1].Image)
				assert.Empty(t, pod.Spec.Containers[1].Command, "Helper container command should be empty")
				assert.Empty(t, pod.Spec.Containers[1].Args, "Helper container args should be empty")

				assert.Equal(t, "svc-0", pod.Spec.Containers[2].Name)
				assert.Equal(t, "test-service", pod.Spec.Containers[2].Image)
				assert.Equal(t, []string{"/init", "run"}, pod.Spec.Containers[2].Command)
				assert.Equal(t, []string{"application", "--debug"}, pod.Spec.Containers[2].Args)

				assert.Equal(t, "svc-1", pod.Spec.Containers[3].Name)
				assert.Equal(t, "test-service-2", pod.Spec.Containers[3].Image)
				assert.Empty(t, pod.Spec.Containers[3].Command, "Service container command should be empty")
				assert.Equal(t, []string{"application", "--debug"}, pod.Spec.Containers[3].Args)
			},
		},
		"creates services in kubernetes if ports are set": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace:   "default",
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: common.Image{
					Name: "test-image",
					Ports: []common.Port{
						{
							Number: 80,
						},
					},
				},
				Services: common.Services{
					{
						Name: "test-service",
						Ports: []common.Port{
							{
								Number: 82,
							},
							{
								Number: 84,
							},
						},
					},
					{
						Name: "test-service2",
						Ports: []common.Port{
							{
								Number: 85,
							},
						},
					},
					{
						Name: "test-service3",
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				expectedServices := []api.Service{
					{
						ObjectMeta: metav1.ObjectMeta{
							GenerateName: "build",
							Namespace:    "default",
						},
						Spec: api.ServiceSpec{
							Ports: []api.ServicePort{
								{
									Port:       80,
									TargetPort: intstr.FromInt(80),
									Name:       "build-80",
								},
							},
							Selector: map[string]string{"pod": e.pod.GenerateName},
							Type:     api.ServiceTypeClusterIP,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							GenerateName: "proxy-svc-0",
							Namespace:    "default",
						},
						Spec: api.ServiceSpec{
							Ports: []api.ServicePort{
								{
									Port:       82,
									TargetPort: intstr.FromInt(82),
									Name:       "proxy-svc-0-82",
								},
								{
									Port:       84,
									TargetPort: intstr.FromInt(84),
									Name:       "proxy-svc-0-84",
								},
							},
							Selector: map[string]string{"pod": e.pod.GenerateName},
							Type:     api.ServiceTypeClusterIP,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							GenerateName: "proxy-svc-1",
							Namespace:    "default",
						},
						Spec: api.ServiceSpec{
							Ports: []api.ServicePort{
								{
									Port:       85,
									TargetPort: intstr.FromInt(85),
									Name:       "proxy-svc-1-85",
								},
							},
							Selector: map[string]string{"pod": e.pod.GenerateName},
							Type:     api.ServiceTypeClusterIP,
						},
					},
				}

				assert.ElementsMatch(t, expectedServices, e.services)
			},
		},
		"the default service name for the build container is build": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace:   "default",
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: common.Image{
					Name: "test-image",
					Ports: []common.Port{
						{
							Number: 80,
						},
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				assert.Equal(t, "build", e.services[0].GenerateName)
			},
		},
		"the services have a selector pointing to the 'pod' label in the pod": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace:   "default",
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: common.Image{
					Name: "test-image",
					Ports: []common.Port{
						{
							Number: 80,
						},
					},
				},
				Services: common.Services{
					{
						Name: "test-service",
						Ports: []common.Port{
							{
								Number: 82,
							},
						},
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				for _, service := range e.services {
					assert.Equal(t, map[string]string{"pod": e.pod.GenerateName}, service.Spec.Selector)
				}
			},
		},
		"the service is named as the alias if set": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace:   "default",
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: common.Image{
					Name: "test-image",
				},
				Services: common.Services{
					{
						Name:  "test-service",
						Alias: "custom-name",
						Ports: []common.Port{
							{
								Number: 82,
							},
						},
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				assert.Equal(t, "custom-name", e.services[0].GenerateName)
			},
		},
		"proxies are configured if services have been created": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace:   "default",
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: common.Image{
					Name: "test-image",
					Ports: []common.Port{
						{
							Number: 80,
						},
					},
				},
				Services: common.Services{
					{
						Name:  "test-service",
						Alias: "custom_name",
						Ports: []common.Port{
							{
								Number:   81,
								Name:     "custom_port_name",
								Protocol: "http",
							},
						},
					},
					{
						Name: "test-service2",
						Ports: []common.Port{
							{
								Number: 82,
							},
						},
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				require.Len(t, e.ProxyPool, 3)

				assert.NotEmpty(t, "proxy-svc-1", e.ProxyPool)
				assert.NotEmpty(t, "custom_name", e.ProxyPool)
				assert.NotEmpty(t, "build", e.ProxyPool)

				port := e.ProxyPool["proxy-svc-1"].Settings.Ports[0]
				assert.Equal(t, 82, port.Number)

				port = e.ProxyPool["custom_name"].Settings.Ports[0]
				assert.Equal(t, 81, port.Number)
				assert.Equal(t, "custom_port_name", port.Name)
				assert.Equal(t, "http", port.Protocol)

				port = e.ProxyPool["build"].Settings.Ports[0]
				assert.Equal(t, 80, port.Number)
			},
		},
		"makes service name compatible with RFC1123": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace:   "default",
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: common.Image{
					Name: "test-image",
				},
				Services: common.Services{
					{
						Name:  "test-service",
						Alias: "service,name-.non-compat!ble",
						Ports: []common.Port{
							{
								Number:   81,
								Name:     "port,name-.non-compat!ble",
								Protocol: "http",
							},
						},
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				assert.Equal(t, "servicename-non-compatble", e.services[0].GenerateName)
				assert.NotEmpty(t, e.ProxyPool["service,name-.non-compat!ble"])
				assert.Equal(t, "port,name-.non-compat!ble", e.ProxyPool["service,name-.non-compat!ble"].Settings.Ports[0].Name)
			},
		},
		"sets command (entrypoint) and args": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace:   "default",
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: common.Image{
					Name: "test-image",
				},
				Services: common.Services{
					{
						Name:    "test-service-0",
						Command: []string{"application", "--debug"},
					},
					{
						Name:       "test-service-1",
						Entrypoint: []string{"application", "--debug"},
					},
					{
						Name:       "test-service-2",
						Entrypoint: []string{"application", "--debug"},
						Command:    []string{"argument1", "argument2"},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				require.Len(t, pod.Spec.Containers, 5)

				assert.Equal(t, "build", pod.Spec.Containers[0].Name)
				assert.Equal(t, "test-image", pod.Spec.Containers[0].Image)
				assert.Empty(t, pod.Spec.Containers[0].Command, "Build container command should be empty")
				assert.Empty(t, pod.Spec.Containers[0].Args, "Build container args should be empty")

				assert.Equal(t, "helper", pod.Spec.Containers[1].Name)
				assert.Equal(t, "custom/helper-image", pod.Spec.Containers[1].Image)
				assert.Empty(t, pod.Spec.Containers[1].Command, "Helper container command should be empty")
				assert.Empty(t, pod.Spec.Containers[1].Args, "Helper container args should be empty")

				assert.Equal(t, "svc-0", pod.Spec.Containers[2].Name)
				assert.Equal(t, "test-service-0", pod.Spec.Containers[2].Image)
				assert.Empty(t, pod.Spec.Containers[2].Command, "Service container command should be empty")
				assert.Equal(t, []string{"application", "--debug"}, pod.Spec.Containers[2].Args)

				assert.Equal(t, "svc-1", pod.Spec.Containers[3].Name)
				assert.Equal(t, "test-service-1", pod.Spec.Containers[3].Image)
				assert.Equal(t, []string{"application", "--debug"}, pod.Spec.Containers[3].Command)
				assert.Empty(t, pod.Spec.Containers[3].Args, "Service container args should be empty")

				assert.Equal(t, "svc-2", pod.Spec.Containers[4].Name)
				assert.Equal(t, "test-service-2", pod.Spec.Containers[4].Image)
				assert.Equal(t, []string{"application", "--debug"}, pod.Spec.Containers[4].Command)
				assert.Equal(t, []string{"argument1", "argument2"}, pod.Spec.Containers[4].Args)
			},
		},
		"non-DNS-1123-compatible-token": {
			RunnerConfig: common.RunnerConfig{
				RunnerCredentials: common.RunnerCredentials{
					Token: "ToK3_?OF",
				},
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				dns_test.AssertRFC1123Compatibility(t, pod.GetGenerateName())
			},
		},
		"supports pod security context": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
						PodSecurityContext: common.KubernetesPodSecurityContext{
							FSGroup:            func() *int64 { i := int64(200); return &i }(),
							RunAsGroup:         func() *int64 { i := int64(200); return &i }(),
							RunAsNonRoot:       func() *bool { i := bool(true); return &i }(),
							RunAsUser:          func() *int64 { i := int64(200); return &i }(),
							SupplementalGroups: []int64{200},
						},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Equal(t, int64(200), *pod.Spec.SecurityContext.FSGroup)
				assert.Equal(t, int64(200), *pod.Spec.SecurityContext.RunAsGroup)
				assert.Equal(t, int64(200), *pod.Spec.SecurityContext.RunAsUser)
				assert.Equal(t, true, *pod.Spec.SecurityContext.RunAsNonRoot)
				assert.Equal(t, []int64{200}, pod.Spec.SecurityContext.SupplementalGroups)
			},
		},
		"uses default security context when unspecified": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Empty(t, pod.Spec.SecurityContext, "Security context should be empty")
			},
		},
		"supports services as host aliases": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
					},
				},
			},
			Options: &kubernetesOptions{
				Services: common.Services{
					{
						Name:  "test-service",
						Alias: "svc-alias",
					},
					{
						Name: "docker:dind",
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Len(t, pod.Spec.Containers, 4)
				require.Len(t, pod.Spec.HostAliases, 1)

				expectedHostnames := []string{
					"test-service",
					"svc-alias",
					"docker",
				}
				assert.Equal(t, "127.0.0.1", pod.Spec.HostAliases[0].IP)
				assert.Equal(t, expectedHostnames, pod.Spec.HostAliases[0].Hostnames)
			},
		},
		"supports services as kubernetes host aliases with simple docker image syntax": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
					},
				},
			},
			Options: &kubernetesOptions{
				Services: common.Services{
					{
						Name: "docker:dind",
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Len(t, pod.Spec.Containers, 3)
				require.Len(t, pod.Spec.HostAliases, 1)

				assert.Equal(t, "127.0.0.1", pod.Spec.HostAliases[0].IP)
				assert.Equal(t, []string{"docker"}, pod.Spec.HostAliases[0].Hostnames)
			},
		},
		"ignores non RFC1123 aliases": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
					},
				},
			},
			Options: &kubernetesOptions{
				Services: common.Services{
					{
						Name:  "test-service",
						Alias: "INVALID_ALIAS",
					},
					{
						Name: "docker:dind",
					},
				},
			},
			VerifySetupBuildPodErrFn: func(t *testing.T, err error) {
				var expected *invalidHostAliasDNSError
				assert.True(t, errors.As(err, &expected))
				assert.True(t, expected.Is(err))
				errMsg := err.Error()
				assert.Contains(t, errMsg, "is invalid DNS")
				assert.Contains(t, errMsg, "INVALID_ALIAS")
				assert.Contains(t, errMsg, "test-service")
			},
		},
		"ignores services with ports": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
					},
				},
			},
			Options: &kubernetesOptions{
				Services: common.Services{
					{
						Name:  "test-service",
						Alias: "alias",
					},
					{
						Name: "docker:dind",
						Ports: []common.Port{{
							Number:   0,
							Protocol: "",
							Name:     "",
						}},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				// the second time this fn is called is to create the proxy service
				if pod.Kind == "Service" {
					return
				}

				require.Len(t, pod.Spec.Containers, 4)

				require.Len(t, pod.Spec.HostAliases, 1)
				assert.Equal(t, "127.0.0.1", pod.Spec.HostAliases[0].IP)
				assert.Equal(t, []string{"test-service", "alias"}, pod.Spec.HostAliases[0].Hostnames)
			},
		},
		"no host aliases when feature is not supported": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
					},
				},
			},
			Options: &kubernetesOptions{
				Services: common.Services{
					{
						Name:  "test-service",
						Alias: "alias",
					},
				},
			},
			PrepareFn: func(t *testing.T, def setupBuildPodTestDef, e *executor) {
				mockFc := &mockFeatureChecker{}
				mockFc.On("IsHostAliasSupported").Return(false, nil)
				e.featureChecker = mockFc
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Len(t, pod.Spec.Containers, 3)
				assert.Nil(t, pod.Spec.HostAliases)
			},
		},
		"check host aliases non version error": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
					},
				},
			},
			Options: &kubernetesOptions{
				Services: common.Services{
					{
						Name:  "test-service",
						Alias: "alias",
					},
				},
			},
			PrepareFn: func(t *testing.T, def setupBuildPodTestDef, e *executor) {
				mockFc := &mockFeatureChecker{}
				mockFc.On("IsHostAliasSupported").Return(false, testErr)
				e.featureChecker = mockFc
			},
			VerifySetupBuildPodErrFn: func(t *testing.T, err error) {
				assert.True(t, errors.Is(err, testErr))
			},
		},
		"check host aliases version error": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
					},
				},
			},
			Options: &kubernetesOptions{
				Services: common.Services{
					{
						Name:  "test-service",
						Alias: "alias",
					},
				},
			},
			PrepareFn: func(t *testing.T, def setupBuildPodTestDef, e *executor) {
				mockFc := &mockFeatureChecker{}
				mockFc.On("IsHostAliasSupported").Return(false, &badVersionError{})
				e.featureChecker = mockFc
			},
			VerifySetupBuildPodErrFn: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			helperImageInfo, err := helperimage.Get(common.REVISION, helperimage.Config{
				OSType:       helperimage.OSTypeLinux,
				Architecture: "amd64",
			})
			require.NoError(t, err)

			vars := test.Variables
			if vars == nil {
				vars = []common.JobVariable{}
			}

			options := test.Options
			if options == nil {
				options = &kubernetesOptions{}
			}

			rt := setupBuildPodFakeRoundTripper{
				t:    t,
				test: test,
			}

			mockFc := &mockFeatureChecker{}
			mockFc.On("IsHostAliasSupported").Return(true, nil)
			ex := executor{
				kubeClient: testKubernetesClient(version, fake.CreateHTTPClient(rt.RoundTrip)),
				options:    options,
				configMap:  fakeConfigMap(),
				AbstractExecutor: executors.AbstractExecutor{
					Config:     test.RunnerConfig,
					BuildShell: &common.ShellConfiguration{},
					Build: &common.Build{
						JobResponse: common.JobResponse{
							Variables: vars,
						},
						Runner: &test.RunnerConfig,
					},
					ProxyPool: proxy.NewPool(),
				},
				helperImageInfo: helperImageInfo,
				featureChecker:  mockFc,
			}

			if test.PrepareFn != nil {
				test.PrepareFn(t, test, &ex)
			}

			err = ex.prepareOverwrites(make(common.JobVariables, 0))
			assert.NoError(t, err, "error preparing overwrites")

			err = ex.setupBuildPod()
			if test.VerifySetupBuildPodErrFn == nil {
				assert.NoError(t, err, "error setting up build pod")
				assert.True(t, rt.executed, "RoundTrip for kubernetes client should be executed")
			} else {
				test.VerifySetupBuildPodErrFn(t, err)
			}

			if test.VerifyExecutorFn != nil {
				test.VerifyExecutorFn(t, test, &ex)
			}
		})
	}
}

func TestProcessLogs(t *testing.T) {
	mockTrace := &common.MockJobTrace{}
	defer mockTrace.AssertExpectations(t)

	mockTrace.On("Write", mock.MatchedBy(func(in []byte) bool {
		return bytes.Equal(in, []byte("line\n"))
	})).Return(0, nil).Once()

	oldLogProcessor := newLogProcessor
	defer func() {
		newLogProcessor = oldLogProcessor
	}()

	mockLogProcessor := &mockLogProcessor{}
	defer mockLogProcessor.AssertExpectations(t)

	logsCh := make(chan string)
	go func() {
		logsCh <- "line"

		exitCode := 1
		script := "script"
		status := shells.TrapCommandExitStatus{
			CommandExitCode: &exitCode,
			Script:          &script,
		}

		b, _ := json.Marshal(status)
		logsCh <- string(b)
		close(logsCh)
	}()

	var returnCh <-chan string = logsCh
	mockLogProcessor.On("Listen", mock.Anything).Return(returnCh).Once()

	newLogProcessor = func(client *kubernetes.Clientset, backoff backoff.Backoff, logger *common.BuildLogger, podCfg kubernetesLogProcessorPodConfig) logProcessor {
		return mockLogProcessor
	}

	e := executor{}
	e.remoteProcessTerminated = make(chan shells.TrapCommandExitStatus)
	e.Trace = mockTrace
	e.pod = &api.Pod{}
	e.pod.Name = "pod_name"
	e.pod.Namespace = "namespace"

	go e.processLogs(context.Background())

	exitStatus := <-e.remoteProcessTerminated
	assert.Equal(t, 1, *exitStatus.CommandExitCode)
	assert.Equal(t, "script", *exitStatus.Script)
}

func TestRunAttachCheckPodStatus(t *testing.T) {
	version, codec := testVersionAndCodec()

	respErr := errors.New("err")

	type podResponse struct {
		response *http.Response
		err      error
	}

	tests := map[string]struct {
		responses []podResponse
		verifyErr func(t *testing.T, errCh <-chan error)
	}{
		"no error": {
			responses: []podResponse{
				{
					response: &http.Response{StatusCode: http.StatusOK},
					err:      nil,
				},
			},
			verifyErr: func(t *testing.T, errCh <-chan error) {
				assert.NoError(t, <-errCh)
			},
		},
		"pod phase failed": {
			responses: []podResponse{
				{
					response: &http.Response{
						StatusCode: http.StatusOK,
						Body:       objBody(codec, execPodWithPhase(api.PodFailed)),
					},
					err: nil,
				},
			},
			verifyErr: func(t *testing.T, errCh <-chan error) {
				err := <-errCh
				require.Error(t, err)
				var phaseErr *podPhaseError
				assert.True(t, errors.As(err, &phaseErr))
				assert.Equal(t, api.PodFailed, phaseErr.phase)
			},
		},
		"pod not found": {
			responses: []podResponse{
				{
					response: nil,
					err: &kubeerrors.StatusError{
						ErrStatus: metav1.Status{
							Code: http.StatusNotFound,
						},
					},
				},
			},
			verifyErr: func(t *testing.T, errCh <-chan error) {
				err := <-errCh
				require.Error(t, err)
				var statusErr *kubeerrors.StatusError
				assert.True(t, errors.As(err, &statusErr))
				assert.Equal(t, int32(http.StatusNotFound), statusErr.ErrStatus.Code)
			},
		},
		"general error continues": {
			responses: []podResponse{
				{
					response: nil,
					err:      respErr,
				},
				{
					response: nil,
					err:      respErr,
				},
				{
					response: nil,
					err:      respErr,
				},
			},
			verifyErr: func(t *testing.T, errCh <-chan error) {
				select {
				case err, more := <-errCh:
					assert.False(t, more)
					assert.NoError(t, err)
				case <-time.After(10 * time.Second):
					require.Fail(t, "Should not get any error")
				}
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			i := 0
			fakeClient := fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case p == "/api/v1/namespaces/namespace/pods/pod" && m == http.MethodGet:
					res := tt.responses[i]
					i++
					if i == len(tt.responses) {
						cancel()
					}

					if res.response == nil {
						return nil, res.err
					}

					res.response.Header = map[string][]string{
						"Content-Type": {"application/json"},
					}
					if res.response.Body == nil {
						res.response.Body = objBody(codec, execPod())
					}

					return res.response, nil
				default:
					return nil, fmt.Errorf("unexpected request")
				}
			})

			client := testKubernetesClient(version, fakeClient)

			e := executor{}
			e.Config = common.RunnerConfig{}
			e.Config.Kubernetes = &common.KubernetesConfig{
				PollInterval: 1,
				PollTimeout:  2,
			}
			e.kubeClient = client
			e.remoteProcessTerminated = make(chan shells.TrapCommandExitStatus)
			e.Trace = &common.Trace{Writer: os.Stdout}
			e.pod = &api.Pod{}
			e.pod.Name = "pod"
			e.pod.Namespace = "namespace"

			tt.verifyErr(t, e.watchPodStatus(ctx))
		})
	}
}

func TestLimits(t *testing.T) {
	tests := []struct {
		CPU, Memory string
		Expected    api.ResourceList
	}{
		{
			CPU:    "100m",
			Memory: "100Mi",
			Expected: api.ResourceList{
				api.ResourceCPU:    resource.MustParse("100m"),
				api.ResourceMemory: resource.MustParse("100Mi"),
			},
		},
		{
			CPU: "100m",
			Expected: api.ResourceList{
				api.ResourceCPU: resource.MustParse("100m"),
			},
		},
		{
			Memory: "100Mi",
			Expected: api.ResourceList{
				api.ResourceMemory: resource.MustParse("100Mi"),
			},
		},
		{
			CPU:      "100j",
			Expected: api.ResourceList{},
		},
		{
			Memory:   "100j",
			Expected: api.ResourceList{},
		},
		{
			Expected: api.ResourceList{},
		},
	}

	for _, test := range tests {
		res, _ := limits(test.CPU, test.Memory)
		assert.Equal(t, test.Expected, res)
	}
}

func fakeConfigMap() *api.ConfigMap {
	configMap := &api.ConfigMap{}
	configMap.Name = "fake"
	return configMap
}

func fakeKubeDeleteResponse(status int) *http.Response {
	_, codec := testVersionAndCodec()

	body := objBody(codec, &metav1.Status{Code: int32(status)})
	return &http.Response{StatusCode: status, Body: body, Header: map[string][]string{
		"Content-Type": {"application/json"},
	}}
}

func setBuildFeatureFlag(build *common.Build, flag string, value bool) {
	for _, v := range build.Variables {
		if v.Key == flag {
			v.Value = fmt.Sprint(value)
			return
		}
	}

	build.Variables = append(build.Variables, common.JobVariable{
		Key:   flag,
		Value: fmt.Sprint(value),
	})
}

type FakeReadCloser struct {
	io.Reader
}

func (f FakeReadCloser) Close() error {
	return nil
}

type FakeBuildTrace struct {
	testWriter
}

func (f FakeBuildTrace) Success()                                              {}
func (f FakeBuildTrace) Fail(err error, failureReason common.JobFailureReason) {}
func (f FakeBuildTrace) Notify(func())                                         {}
func (f FakeBuildTrace) SetCancelFunc(cancelFunc context.CancelFunc)           {}
func (f FakeBuildTrace) SetFailuresCollector(fc common.FailuresCollector)      {}
func (f FakeBuildTrace) SetMasked(masked []string)                             {}
func (f FakeBuildTrace) IsStdout() bool {
	return false
}

func TestCommandTerminatedError_Is(t *testing.T) {
	tests := map[string]struct {
		err error

		expectedIsResult bool
	}{
		"nil": {
			err:              nil,
			expectedIsResult: false,
		},
		"EOF": {
			err:              io.EOF,
			expectedIsResult: false,
		},
		"commandTerminatedError": {
			err:              &commandTerminatedError{},
			expectedIsResult: true,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.Equal(t, tt.expectedIsResult, errors.Is(tt.err, new(commandTerminatedError)), "commandTerminatedError.Is incorrect result")
		})
	}
}
