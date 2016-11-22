package kubernetes

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/restclient"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/fake"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/executors"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers"
)

var (
	TRUE = true
)

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

func TestCleanup(t *testing.T) {
	version := testapi.Default.GroupVersion().Version
	codec := testapi.Default.Codec()

	tests := []struct {
		Pod        *api.Pod
		ClientFunc func(*http.Request) (*http.Response, error)
		Error      bool
	}{
		{
			Pod: &api.Pod{
				ObjectMeta: api.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
				},
			},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == "DELETE" && p == "/api/"+version+"/namespaces/test-ns/pods/test-pod":
					return &http.Response{StatusCode: 200, Body: FakeReadCloser{
						Reader: strings.NewReader(""),
					}}, nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
		},
		{
			Pod: &api.Pod{
				ObjectMeta: api.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
				},
			},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("delete failed")
			},
			Error: true,
		},
	}

	for _, test := range tests {
		c := client.NewOrDie(&restclient.Config{ContentConfig: restclient.ContentConfig{GroupVersion: &unversioned.GroupVersion{Version: version}}})
		fakeClient := fake.RESTClient{
			Codec:  codec,
			Client: fake.CreateHTTPClient(test.ClientFunc),
		}
		c.Client = fakeClient.Client

		ex := executor{
			kubeClient: c,
			pod:        test.Pod,
		}
		errored := false
		buildTrace := FakeBuildTrace{
			testWriter{
				call: func(b []byte) (int, error) {
					if test.Error && !errored {
						if s := string(b); strings.Contains(s, "Error cleaning up") {
							errored = true
						} else {
							t.Errorf("expected failure. got: '%s'", string(b))
						}
					}
					return len(b), nil
				},
			},
		}
		ex.AbstractExecutor.BuildTrace = buildTrace
		ex.AbstractExecutor.BuildLogger = common.NewBuildLogger(buildTrace, logrus.WithFields(logrus.Fields{}))
		ex.Cleanup()
		if test.Error && !errored {
			t.Errorf("expected cleanup to error but it didn't")
		}
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
				GetBuildResponse: common.GetBuildResponse{
					Sha: "1234567890",
					Options: common.BuildOptions{
						"image": "test-image",
					},
					Variables: []common.BuildVariable{
						common.BuildVariable{Key: "privileged", Value: "true"},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: "test-image",
				},
				namespaceOverwrite: "",
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
						Host:                      "test-server",
						Namespace:                 "namespace",
						NamespaceOverwriteAllowed: "^n.*?e$",
						ServiceCPULimit:           "100m",
						ServiceMemoryLimit:        "200Mi",
						CPULimit:                  "1.5",
						MemoryLimit:               "4Gi",
						HelperCPULimit:            "50m",
						HelperMemoryLimit:         "100Mi",
						ServiceCPURequest:         "99m",
						ServiceMemoryRequest:      "5Mi",
						CPURequest:                "1",
						MemoryRequest:             "1.5Gi",
						HelperCPURequest:          "0.5m",
						HelperMemoryRequest:       "42Mi",
						Privileged:                false,
					},
				},
			},
			Build: &common.Build{
				GetBuildResponse: common.GetBuildResponse{
					Sha: "1234567890",
					Options: common.BuildOptions{
						"image": "test-image",
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: "test-image",
				},
				namespaceOverwrite: "",
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
						Host:               "test-server",
						ServiceCPUs:        "100m",
						ServiceMemory:      "200Mi",
						ServiceMemoryLimit: "202Mi",
						CPUs:               "1.5",
						Memory:             "4Gi",
						HelperCPUs:         "50m",
						HelperCPULimit:     "1m",
						HelperMemory:       "100Mi",
						Privileged:         true,
					},
				},
			},
			Build: &common.Build{
				GetBuildResponse: common.GetBuildResponse{
					Sha: "1234567890",
					Options: common.BuildOptions{
						"image": "test-image",
					},
					Variables: []common.BuildVariable{
						common.BuildVariable{Key: "privileged", Value: "true"},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: "test-image",
				},
				namespaceOverwrite: "",
				serviceLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("100m"),
					api.ResourceMemory: resource.MustParse("202Mi"),
				},
				buildLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("1.5"),
					api.ResourceMemory: resource.MustParse("4Gi"),
				},
				helperLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("1m"),
					api.ResourceMemory: resource.MustParse("100Mi"),
				},
				serviceRequests: api.ResourceList{},
				buildRequests:   api.ResourceList{},
				helperRequests:  api.ResourceList{},
			},
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
				GetBuildResponse: common.GetBuildResponse{
					Sha: "1234567890",
					Options: common.BuildOptions{
						"image": "test-image",
					},
					Variables: []common.BuildVariable{
						common.BuildVariable{Key: "KUBERNETES_NAMESPACE_OVERWRITE", Value: "namespace"},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: "test-image",
				},
				namespaceOverwrite: "namespace",
				serviceLimits:      api.ResourceList{},
				buildLimits:        api.ResourceList{},
				helperLimits:       api.ResourceList{},
				serviceRequests:    api.ResourceList{},
				buildRequests:      api.ResourceList{},
				helperRequests:     api.ResourceList{},
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

			err := e.Prepare(test.GlobalConfig, test.RunnerConfig, test.Build)

			if err != nil {
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
			assert.Equal(t, test.Expected, e)
		})
	}
}

func TestSetupBuildPod(t *testing.T) {
	version := testapi.Default.GroupVersion().Version
	codec := testapi.Default.Codec()

	type testDef struct {
		RunnerConfig common.RunnerConfig
		VerifyFn     func(*testing.T, testDef, *api.Pod)
	}
	tests := []testDef{
		{
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
			VerifyFn: func(t *testing.T, test testDef, pod *api.Pod) {
				assert.Equal(t, test.RunnerConfig.RunnerSettings.Kubernetes.NodeSelector, pod.Spec.NodeSelector)
			},
		},
		{
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
					},
				},
			},
			VerifyFn: func(t *testing.T, test testDef, pod *api.Pod) {
				hasHelper := false
				for _, c := range pod.Spec.Containers {
					if c.Name == "helper" {
						hasHelper = true
					}
				}
				assert.True(t, hasHelper)
			},
		},
		{
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace:   "default",
						HelperImage: "custom/helper-image",
					},
				},
			},
			VerifyFn: func(t *testing.T, test testDef, pod *api.Pod) {
				for _, c := range pod.Spec.Containers {
					if c.Name == "helper" {
						assert.Equal(t, test.RunnerConfig.RunnerSettings.Kubernetes.HelperImage, c.Image)
					}
				}
			},
		},
	}

	fakeClientRoundTripper := func(test testDef) func(req *http.Request) (*http.Response, error) {
		return func(req *http.Request) (resp *http.Response, err error) {
			podBytes, err := ioutil.ReadAll(req.Body)

			if err != nil {
				t.Errorf("failed to read request body: %s", err.Error())
				return
			}

			p := new(api.Pod)

			err = json.Unmarshal(podBytes, p)

			if err != nil {
				t.Errorf("error decoding pod: %s", err.Error())
				return
			}

			test.VerifyFn(t, test, p)

			resp = &http.Response{StatusCode: 200, Body: FakeReadCloser{
				Reader: bytes.NewBuffer(podBytes),
			}}
			resp.Header = make(http.Header)
			resp.Header.Add("Content-Type", "application/json")

			return
		}
	}

	for _, test := range tests {
		c := client.NewOrDie(&restclient.Config{ContentConfig: restclient.ContentConfig{GroupVersion: &unversioned.GroupVersion{Version: version}}})
		fakeClient := fake.RESTClient{
			Codec:  codec,
			Client: fake.CreateHTTPClient(fakeClientRoundTripper(test)),
		}
		c.Client = fakeClient.Client

		ex := executor{
			kubeClient: c,
			options:    &kubernetesOptions{},
			AbstractExecutor: executors.AbstractExecutor{
				Config:     test.RunnerConfig,
				BuildShell: &common.ShellConfiguration{},
				Build: &common.Build{
					GetBuildResponse: common.GetBuildResponse{
						Variables: []common.BuildVariable{},
					},
					Runner: &common.RunnerConfig{},
				},
			},
		}

		err := ex.setupBuildPod()

		if err != nil {
			t.Errorf("error setting up build pod: %s", err.Error())
		}
	}
}

func TestKubernetesSuccessRun(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	assert.NoError(t, err)
	successfulBuild.Options = map[string]interface{}{
		"image": "docker:git",
	}
	build := &common.Build{
		GetBuildResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor:   "kubernetes",
				Kubernetes: &common.KubernetesConfig{},
			},
		},
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func TestKubernetesBuildFail(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	failedBuild, err := common.GetRemoteFailedBuild()
	assert.NoError(t, err)
	build := &common.Build{
		GetBuildResponse: failedBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor:   "kubernetes",
				Kubernetes: &common.KubernetesConfig{},
			},
		},
	}
	build.Options = map[string]interface{}{
		"image": "docker:git",
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err, "error")
	assert.IsType(t, err, &common.BuildError{})
	assert.Contains(t, err.Error(), "Error executing in Docker Container: 1")
}

func TestKubernetesMissingImage(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	failedBuild, err := common.GetRemoteFailedBuild()
	assert.NoError(t, err)
	build := &common.Build{
		GetBuildResponse: failedBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor:   "kubernetes",
				Kubernetes: &common.KubernetesConfig{},
			},
		},
	}
	build.Options = map[string]interface{}{
		"image": "some/non-existing/image",
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.IsType(t, err, &common.BuildError{})
	assert.Contains(t, err.Error(), "image pull failed")
}

func TestKubernetesMissingTag(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	failedBuild, err := common.GetRemoteFailedBuild()
	assert.NoError(t, err)
	build := &common.Build{
		GetBuildResponse: failedBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor:   "kubernetes",
				Kubernetes: &common.KubernetesConfig{},
			},
		},
	}
	build.Options = map[string]interface{}{
		"image": "docker:missing-tag",
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.IsType(t, err, &common.BuildError{})
	assert.Contains(t, err.Error(), "image pull failed")
}

func TestKubernetesBuildAbort(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	failedBuild, err := common.GetRemoteFailedBuild()
	assert.NoError(t, err)
	build := &common.Build{
		GetBuildResponse: failedBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor:   "kubernetes",
				Kubernetes: &common.KubernetesConfig{},
			},
		},
		SystemInterrupt: make(chan os.Signal, 1),
	}
	build.Options = map[string]interface{}{
		"image": "docker:git",
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

func TestKubernetesBuildCancel(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	failedBuild, err := common.GetRemoteFailedBuild()
	assert.NoError(t, err)
	build := &common.Build{
		GetBuildResponse: failedBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor:   "kubernetes",
				Kubernetes: &common.KubernetesConfig{},
			},
		},
		SystemInterrupt: make(chan os.Signal, 1),
	}
	build.Options = map[string]interface{}{
		"image": "docker:git",
	}

	trace := &common.Trace{Writer: os.Stdout, Abort: make(chan interface{}, 1)}

	abortTimer := time.AfterFunc(time.Second, func() {
		t.Log("Interrupt")
		trace.Abort <- true
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

func TestOverwriteNamespaceNotMatch(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	build := &common.Build{
		GetBuildResponse: common.GetBuildResponse{
			Sha: "1234567890",
			Options: common.BuildOptions{
				"image": "test-image",
			},
			Variables: []common.BuildVariable{
				common.BuildVariable{Key: "KUBERNETES_NAMESPACE_OVERWRITE", Value: "namespace"},
			},
		},
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					NamespaceOverwriteAllowed: "^not_a_match$",
				},
			},
		},
		SystemInterrupt: make(chan os.Signal, 1),
	}
	build.Options = map[string]interface{}{
		"image": "docker:git",
	}

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
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

func (f FakeBuildTrace) Success()      {}
func (f FakeBuildTrace) Fail(error)    {}
func (f FakeBuildTrace) Notify(func()) {}
func (f FakeBuildTrace) Aborted() chan interface{} {
	return make(chan interface{})
}
func (f FakeBuildTrace) IsStdout() bool {
	return false
}
