//go:build !integration

package kubernetes

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"golang.org/x/net/context"

	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"
	k8stesting "k8s.io/client-go/testing"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestGetKubeClientConfig(t *testing.T) {
	originalInClusterConfig := inClusterConfig
	originalDefaultKubectlConfig := defaultKubectlConfig
	defer func() {
		inClusterConfig = originalInClusterConfig
		defaultKubectlConfig = originalDefaultKubectlConfig
	}()

	completeConfig := &restclient.Config{
		Host:        "host",
		BearerToken: "token",
		TLSClientConfig: restclient.TLSClientConfig{
			CAFile: "ca",
		},
		UserAgent: common.AppVersion.UserAgent(),
	}

	noConfigAvailable := func() (*restclient.Config, error) {
		return nil, fmt.Errorf("config not available")
	}

	aConfig := func() (*restclient.Config, error) {
		config := *completeConfig
		return &config, nil
	}

	tests := []struct {
		name                 string
		config               *common.KubernetesConfig
		overwrites           *overwrites
		inClusterConfig      kubeConfigProvider
		defaultKubectlConfig kubeConfigProvider
		error                bool
		expected             *restclient.Config
	}{
		{
			name: "Incomplete cert based auth outside cluster",
			config: &common.KubernetesConfig{
				Host:     "host",
				CertFile: "test",
			},
			inClusterConfig:      noConfigAvailable,
			defaultKubectlConfig: noConfigAvailable,
			overwrites:           &overwrites{},
			error:                true,
		},
		{
			name: "Complete cert based auth take precedence over in cluster config",
			config: &common.KubernetesConfig{
				CertFile: "crt",
				KeyFile:  "key",
				CAFile:   "ca",
				Host:     "another_host",
			},
			overwrites:           &overwrites{},
			inClusterConfig:      aConfig,
			defaultKubectlConfig: aConfig,
			expected: &restclient.Config{
				Host: "another_host",
				TLSClientConfig: restclient.TLSClientConfig{
					CertFile: "crt",
					KeyFile:  "key",
					CAFile:   "ca",
				},
				UserAgent: common.AppVersion.UserAgent(),
			},
		},
		{
			name: "User provided configuration take precedence",
			config: &common.KubernetesConfig{
				Host:   "another_host",
				CAFile: "ca",
			},
			overwrites: &overwrites{
				bearerToken: "another_token",
			},
			inClusterConfig:      aConfig,
			defaultKubectlConfig: aConfig,
			expected: &restclient.Config{
				Host:        "another_host",
				BearerToken: "another_token",
				TLSClientConfig: restclient.TLSClientConfig{
					CAFile: "ca",
				},
				UserAgent: common.AppVersion.UserAgent(),
			},
		},
		{
			name:                 "InCluster config",
			config:               &common.KubernetesConfig{},
			overwrites:           &overwrites{},
			inClusterConfig:      aConfig,
			defaultKubectlConfig: noConfigAvailable,
			expected:             completeConfig,
		},
		{
			name:                 "Default cluster config",
			config:               &common.KubernetesConfig{},
			overwrites:           &overwrites{},
			inClusterConfig:      noConfigAvailable,
			defaultKubectlConfig: aConfig,
			expected:             completeConfig,
		},
		{
			name:   "Overwrites works also in cluster",
			config: &common.KubernetesConfig{},
			overwrites: &overwrites{
				bearerToken: "bearerToken",
			},
			inClusterConfig:      aConfig,
			defaultKubectlConfig: noConfigAvailable,
			expected: &restclient.Config{
				Host:        "host",
				BearerToken: "bearerToken",
				TLSClientConfig: restclient.TLSClientConfig{
					CAFile: "ca",
				},
				UserAgent: common.AppVersion.UserAgent(),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inClusterConfig = test.inClusterConfig
			defaultKubectlConfig = test.defaultKubectlConfig

			rcConf, err := getKubeClientConfig(test.config, test.overwrites)

			if test.error {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, test.expected, rcConf)
		})
	}
}

func TestWaitForPodRunning(t *testing.T) {
	version, codec := testVersionAndCodec()
	retries := 0

	tests := []struct {
		Name         string
		Pod          *api.Pod
		Config       *common.KubernetesConfig
		ClientFunc   func(*http.Request) (*http.Response, error)
		PodEndPhase  api.PodPhase
		Retries      int
		Error        bool
		ExactRetries bool
	}{
		{
			Name: "ensure function retries until ready",
			Pod: &api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
				},
			},
			Config: &common.KubernetesConfig{},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case p == "/api/"+version+"/namespaces/test-ns/pods/test-pod" && m == http.MethodGet:
					pod := &api.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-pod",
							Namespace: "test-ns",
						},
						Status: api.PodStatus{
							Phase: api.PodPending,
						},
					}

					if retries > 1 {
						pod.Status.Phase = api.PodRunning
						pod.Status.ContainerStatuses = []api.ContainerStatus{
							{
								Ready: false,
							},
						}
					}

					if retries > 2 {
						pod.Status.ContainerStatuses = []api.ContainerStatus{
							{
								Ready: true,
							},
						}
					}

					retries++
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       objBody(codec, pod),
						Header:     map[string][]string{common.ContentType: {"application/json"}},
					}, nil
				default:
					// Ensures no GET is performed when deleting by name
					t.Errorf("unexpected request: %s %#v\n%#v", req.Method, req.URL, req)
					return nil, fmt.Errorf("unexpected request")
				}
			},
			PodEndPhase: api.PodRunning,
			Retries:     2,
		},
		{
			Name: "ensure function errors if pod already succeeded",
			Pod: &api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
				},
			},
			Config: &common.KubernetesConfig{},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case p == "/api/"+version+"/namespaces/test-ns/pods/test-pod" && m == http.MethodGet:
					pod := &api.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-pod",
							Namespace: "test-ns",
						},
						Status: api.PodStatus{
							Phase: api.PodSucceeded,
						},
					}
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       objBody(codec, pod),
						Header:     map[string][]string{common.ContentType: {"application/json"}},
					}, nil
				default:
					// Ensures no GET is performed when deleting by name
					t.Errorf("unexpected request: %s %#v\n%#v", req.Method, req.URL, req)
					return nil, fmt.Errorf("unexpected request")
				}
			},
			Error:       true,
			PodEndPhase: api.PodSucceeded,
		},
		{
			Name: "ensure function returns error if pod unknown",
			Pod: &api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
				},
			},
			Config: &common.KubernetesConfig{},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("error getting pod")
			},
			PodEndPhase: api.PodUnknown,
			Error:       true,
		},
		{
			Name: "ensure poll parameters work correctly",
			Pod: &api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
				},
			},
			// Will result in 3 attempts at 0, 3, and 6 seconds
			Config: &common.KubernetesConfig{
				PollInterval: 0, // Should get changed to default of 3 by GetPollInterval()
				PollTimeout:  6,
			},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case p == "/api/"+version+"/namespaces/test-ns/pods/test-pod" && m == http.MethodGet:
					pod := &api.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-pod",
							Namespace: "test-ns",
						},
					}
					if retries > 3 {
						t.Errorf("Too many retries for the given poll parameters. (Expected 3)")
					}
					retries++
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       objBody(codec, pod),
						Header:     map[string][]string{common.ContentType: {"application/json"}},
					}, nil
				default:
					// Ensures no GET is performed when deleting by name
					t.Errorf("unexpected request: %s %#v\n%#v", req.Method, req.URL, req)
					return nil, fmt.Errorf("unexpected request")
				}
			},
			PodEndPhase:  api.PodUnknown,
			Retries:      3,
			Error:        true,
			ExactRetries: true,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			retries = 0
			c := testKubernetesClient(version, fake.CreateHTTPClient(test.ClientFunc))

			fw := testWriter{
				call: func(b []byte) (int, error) {
					if retries < test.Retries {
						if !strings.Contains(string(b), "Waiting for pod") {
							t.Errorf("[%s] Expected to continue waiting for pod. Got: '%s'", test.Name, string(b))
						}
					}
					return len(b), nil
				},
			}
			phase, err := waitForPodRunning(context.Background(), c, test.Pod, fw, test.Config)

			if err != nil && !test.Error {
				t.Errorf("[%s] Expected success. Got: %s", test.Name, err.Error())
				return
			}

			if phase != test.PodEndPhase {
				t.Errorf("[%s] Invalid end state. Expected '%v', got: '%v'", test.Name, test.PodEndPhase, phase)
				return
			}

			if test.ExactRetries && retries < test.Retries {
				t.Errorf("[%s] Not enough retries. Expected: %d, got: %d", test.Name, test.Retries, retries)
				return
			}
		})
	}
}

func TestCreateResourceList(t *testing.T) {
	mustGetParseError := func(t *testing.T, s string) error {
		_, err := resource.ParseQuantity(s)
		require.Error(t, err)
		return err
	}

	tests := []struct {
		Name             string
		CPU              string
		Memory           string
		EphemeralStorage string
		Expected         api.ResourceList
		Error            error
	}{
		{
			Name:     "empty values",
			Expected: api.ResourceList{},
		},
		{
			Name:             "cpu and memory",
			CPU:              "500m",
			Memory:           "1024Mi",
			EphemeralStorage: "2048Mi",
			Expected: api.ResourceList{
				api.ResourceCPU:              resource.MustParse("500m"),
				api.ResourceMemory:           resource.MustParse("1024Mi"),
				api.ResourceEphemeralStorage: resource.MustParse("2048Mi"),
			},
		},
		{
			Name: "only cpu",
			CPU:  "500m",
			Expected: api.ResourceList{
				api.ResourceCPU: resource.MustParse("500m"),
			},
		},
		{
			Name:   "only memory",
			Memory: "1024Mi",
			Expected: api.ResourceList{
				api.ResourceMemory: resource.MustParse("1024Mi"),
			},
		},
		{
			Name:             "only ephemeral storage",
			EphemeralStorage: "3024Mi",
			Expected: api.ResourceList{
				api.ResourceEphemeralStorage: resource.MustParse("3024Mi"),
			},
		},
		{
			Name:     "invalid cpu",
			CPU:      "100j",
			Expected: api.ResourceList{},
			Error: &resourceQuantityError{
				resource: "cpu",
				value:    "100j",
				inner:    mustGetParseError(t, "100j"),
			},
		},
		{
			Name:     "invalid memory",
			Memory:   "200j",
			Expected: api.ResourceList{},
			Error: &resourceQuantityError{
				resource: "memory",
				value:    "200j",
				inner:    mustGetParseError(t, "200j"),
			},
		},
		{
			Name:             "invalid ephemeral storage",
			EphemeralStorage: "200j",
			Expected:         api.ResourceList{},
			Error: &resourceQuantityError{
				resource: "ephemeralStorage",
				value:    "200j",
				inner:    mustGetParseError(t, "200j"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			actual, err := createResourceList(test.CPU, test.Memory, test.EphemeralStorage)
			assert.Equal(t, test.Error, err)
			assert.Equal(t, test.Expected, actual)
		})
	}
}

type testWriter struct {
	call func([]byte) (int, error)
}

func (t testWriter) Write(b []byte) (int, error) {
	return t.call(b)
}

func objBody(codec runtime.Codec, obj runtime.Object) io.ReadCloser {
	return io.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(codec, obj))))
}

func testKubernetesClient(version string, httpClient *http.Client) *kubernetes.Clientset {
	conf := restclient.Config{
		ContentConfig: restclient.ContentConfig{
			GroupVersion: &schema.GroupVersion{Version: version},
		},
	}
	kube := kubernetes.NewForConfigOrDie(&conf)
	fakeClient := fake.RESTClient{Client: httpClient}
	kube.RESTClient().(*restclient.RESTClient).Client = fakeClient.Client
	kube.CoreV1().RESTClient().(*restclient.RESTClient).Client = fakeClient.Client
	kube.ExtensionsV1beta1().RESTClient().(*restclient.RESTClient).Client = fakeClient.Client

	return kube
}

// minimal port from k8s.io/kubernetes/pkg/testapi
func testVersionAndCodec() (version string, codec runtime.Codec) {
	scheme := runtime.NewScheme()

	_ = scheme.AddIgnoredConversionType(&metav1.TypeMeta{}, &metav1.TypeMeta{})
	scheme.AddKnownTypes(
		api.SchemeGroupVersion,
		&api.Pod{},
		&api.ServiceAccount{},
		&api.Secret{},
		&metav1.Status{},
	)

	codecs := runtimeserializer.NewCodecFactory(scheme)
	codec = codecs.LegacyCodec(api.SchemeGroupVersion)
	version = api.SchemeGroupVersion.Version

	return
}

func TestSanitizeLabel(t *testing.T) {
	tests := []struct {
		Name string
		In   string
		Out  string
	}{
		{
			Name: "valid label",
			In:   "label",
			Out:  "label",
		},
		{
			Name: "invalid label",
			In:   "label++@",
			Out:  "label",
		},
		{
			Name: "invalid label start end character",
			In:   "--label-",
			Out:  "label",
		},
		{
			Name: "invalid label too long",
			In:   "labellabellabellabellabellabellabellabellabellabellabellabellabel",
			Out:  "labellabellabellabellabellabellabellabellabellabellabellabellab",
		},
		{
			Name: "invalid characters",
			In:   "a\xc5z",
			Out:  "a_z",
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			assert.Equal(t, sanitizeLabel(test.In), test.Out)
		})
	}
}

func TestBuildVariables(t *testing.T) {
	bv := []common.JobVariable{
		{Key: "k", Value: "__nope__"},
		{Key: "filetype", Value: "secret-stuff", File: true},
		{Key: "k", Value: "v"},
		{Key: "RUNNER_TEMP_PROJECT_DIR", Value: "/foo/bar", Public: true, Internal: true},
	}

	validate := func(t *testing.T, envVars []api.EnvVar) {
		assert.Len(t, envVars, 3)
		assert.Equal(t, envVars[0], api.EnvVar{Name: "k", Value: "v"})
		assert.Equal(t, envVars[1], api.EnvVar{Name: "filetype", Value: "/foo/bar/filetype"})
		assert.Equal(t, envVars[2], api.EnvVar{Name: "RUNNER_TEMP_PROJECT_DIR", Value: "/foo/bar"})
	}

	envVars := buildVariables(bv)
	validate(t, envVars)
}

func TestGetContainerStatus(t *testing.T) {
	const containerName = "some-container"

	testCases := map[string]struct {
		statuses []api.ContainerStatus
		expectOK bool
	}{
		"no statuses": {},
		"container has status": {
			statuses: []api.ContainerStatus{
				{Name: containerName},
			},
			expectOK: true,
		},
		"container has no status": {
			statuses: []api.ContainerStatus{
				{Name: "some-other-container"},
			},
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			status, ok := getContainerStatus(tc.statuses, containerName)

			if tc.expectOK {
				assert.True(t, ok)
				assert.NotEqual(t, api.ContainerStatus{}, status, "expected not to receive the zero-value")
			} else {
				assert.False(t, ok)
			}
		})
	}
}

func TestWaitForRunningContainer(t *testing.T) {
	const (
		podName       = "some-pod"
		podNamespace  = "some-namespace"
		containerName = "some-container"
	)
	podTemplate := func(podName, containerName string) *api.Pod {
		return &api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      podName,
				Namespace: podNamespace,
			},
			Status: api.PodStatus{
				ContainerStatuses: []api.ContainerStatus{
					{Name: containerName},
				},
			},
		}
	}
	gvr := schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	ctx := context.TODO()

	tests := map[string]struct {
		waiter     func(*testing.T, context.Context, kubernetes.Interface, chan struct{})
		podUpdater func(*testing.T, context.Context, k8stesting.ObjectTracker, chan struct{})
	}{
		"container not found": {
			waiter: func(
				t *testing.T,
				ctx context.Context,
				fakeKubeClient kubernetes.Interface,
				returnChan chan struct{},
			) {
				err := waitForRunningContainer(ctx, fakeKubeClient, 0, podNamespace, podName, containerName)
				assert.Error(t, err, "expected error from the container waiter")
				assert.ErrorContains(t, err, "container status for \"some-container\" not found")
				close(returnChan)
			},
			podUpdater: func(
				t *testing.T,
				ctx context.Context,
				tracker k8stesting.ObjectTracker,
				watcherChan chan struct{},
			) {
				<-watcherChan

				pod := podTemplate(podName, "another-container")
				err := tracker.Add(pod)
				require.NoError(t, err, "adding the pod to the tracker")

				pod.Status.ContainerStatuses[0].State.Waiting = &api.ContainerStateWaiting{}
				err = tracker.Update(gvr, pod, podNamespace)
				require.NoError(t, err, "updating container to waiting")

				pod.Status.ContainerStatuses[0].State.Running = &api.ContainerStateRunning{}
				err = tracker.Update(gvr, pod, podNamespace)
				require.NoError(t, err, "updating container to running")
			},
		},
		"no failure": {
			waiter: func(
				t *testing.T,
				ctx context.Context,
				fakeKubeClient kubernetes.Interface,
				returnChan chan struct{},
			) {
				err := waitForRunningContainer(ctx, fakeKubeClient, 0, podNamespace, podName, containerName)
				assert.NoError(t, err, "expected no error from the container waiter")
				close(returnChan)
			},
			podUpdater: func(
				t *testing.T,
				ctx context.Context,
				tracker k8stesting.ObjectTracker,
				watcherChan chan struct{},
			) {
				<-watcherChan

				pod := podTemplate(podName, containerName)
				err := tracker.Add(pod)
				require.NoError(t, err, "adding the pod to the tracker")

				pod.Status.ContainerStatuses[0].State.Waiting = &api.ContainerStateWaiting{}
				err = tracker.Update(gvr, pod, podNamespace)
				require.NoError(t, err, "updating container to waiting")

				pod.Status.ContainerStatuses[0].State.Running = &api.ContainerStateRunning{}
				err = tracker.Update(gvr, pod, podNamespace)
				require.NoError(t, err, "updating container to running")
			},
		},
		"container terminated with non-zero status": {
			waiter: func(
				t *testing.T,
				ctx context.Context,
				fakeKubeClient kubernetes.Interface,
				returnChan chan struct{},
			) {
				err := waitForRunningContainer(ctx, fakeKubeClient, 0, podNamespace, podName, containerName)
				assert.Error(t, err, "expected error from the container waiter")
				assert.ErrorContains(t, err, "container \"some-container\" terminated with non-zero status: 1")
				close(returnChan)
			},
			podUpdater: func(
				t *testing.T,
				ctx context.Context,
				tracker k8stesting.ObjectTracker,
				watcherChan chan struct{},
			) {
				<-watcherChan

				pod := podTemplate(podName, containerName)
				err := tracker.Add(pod)
				require.NoError(t, err, "adding the pod to the tracker")

				pod.Status.ContainerStatuses[0].State.Waiting = &api.ContainerStateWaiting{}
				err = tracker.Update(gvr, pod, podNamespace)
				require.NoError(t, err, "updating container to waiting")

				pod.Status.ContainerStatuses[0].State.Terminated = &api.ContainerStateTerminated{ExitCode: 1}
				err = tracker.Update(gvr, pod, podNamespace)
				require.NoError(t, err, "updating container to terminated")
			},
		},
		"container terminated with zero status": {
			waiter: func(
				t *testing.T,
				ctx context.Context,
				fakeKubeClient kubernetes.Interface,
				returnChan chan struct{},
			) {
				err := waitForRunningContainer(ctx, fakeKubeClient, 0, podNamespace, podName, containerName)
				assert.NoError(t, err, "no error from the container waiter")
				close(returnChan)
			},
			podUpdater: func(
				t *testing.T,
				ctx context.Context,
				tracker k8stesting.ObjectTracker,
				watcherChan chan struct{},
			) {
				<-watcherChan

				pod := podTemplate(podName, containerName)
				err := tracker.Add(pod)
				require.NoError(t, err, "adding the pod to the tracker")

				pod.Status.ContainerStatuses[0].State.Waiting = &api.ContainerStateWaiting{}
				err = tracker.Update(gvr, pod, podNamespace)
				require.NoError(t, err, "updating container to waiting")

				pod.Status.ContainerStatuses[0].State.Terminated = &api.ContainerStateTerminated{ExitCode: 0}
				err = tracker.Update(gvr, pod, podNamespace)
				require.NoError(t, err, "updating container to terminated")
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			// Channels to synchronize the test steps
			watcherChan := make(chan struct{})
			returnChan := make(chan struct{})

			fakeKubeClient := testclient.NewSimpleClientset()
			tracker := fakeKubeClient.Tracker()

			fakeKubeClient.PrependWatchReactor("pods", func(action k8stesting.Action) (handled bool, ret watch.Interface, err error) {
				watchAction, ok := action.(k8stesting.WatchAction)
				require.True(t, ok, "action is not a WatchAction, action: %+v", action)

				assert.Equal(t, podNamespace, watchAction.GetNamespace())
				assert.Equal(t, gvr, watchAction.GetResource())
				assert.Equal(t, "metadata.name="+podName+",status.phase=Running", watchAction.GetWatchRestrictions().Fields.String())

				watch, err := tracker.Watch(watchAction.GetResource(), watchAction.GetNamespace())
				if err != nil {
					return false, nil, err
				}

				// Start the process by signaling to add the pod
				close(watcherChan)

				return true, watch, nil
			})

			go tt.waiter(t, ctx, fakeKubeClient, returnChan)
			go tt.podUpdater(t, ctx, tracker, watcherChan)

			select {
			case <-returnChan:
			case <-time.After(5 * time.Second):
				obj, err := tracker.Get(gvr, podNamespace, podName)
				assert.NoError(t, err, "getting current pod state")
				require.FailNowf(t, "container waiter did not return in time", "current object state: %+v", obj)
			}
		})
	}
}

func TestIsRunning(t *testing.T) {
	tests := map[string]struct {
		statuses   []api.ContainerStatus
		phase      api.PodPhase
		containers []string
		isRunning  bool
	}{
		"pod running no containers checked": {
			statuses: []api.ContainerStatus{
				{Name: buildContainerName, Ready: true},
			},
			phase:      api.PodRunning,
			containers: nil,
			isRunning:  true,
		},
		"pod running build container checked": {
			statuses: []api.ContainerStatus{
				{Name: buildContainerName, Ready: true},
			},
			phase:      api.PodRunning,
			containers: []string{buildContainerName},
			isRunning:  true,
		},
		"pod running build container not ready": {
			statuses: []api.ContainerStatus{
				{Name: buildContainerName, Ready: false},
			},
			phase:      api.PodRunning,
			containers: []string{buildContainerName},
			isRunning:  false,
		},
		"pod failed build container ready": {
			statuses: []api.ContainerStatus{
				{Name: buildContainerName, Ready: true},
			},
			phase:      api.PodFailed,
			containers: []string{buildContainerName},
			isRunning:  false,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			pod := &api.Pod{
				Status: api.PodStatus{
					Phase:             tt.phase,
					ContainerStatuses: tt.statuses,
				},
			}

			result, _ := isRunning(pod, tt.containers...)
			assert.Equal(t, tt.isRunning, result)
		})
	}
}
