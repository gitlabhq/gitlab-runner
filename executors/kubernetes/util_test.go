package kubernetes

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/net/context"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"

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
			name: "Complete cert based auth take precendece over in cluster config",
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
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inClusterConfig = test.inClusterConfig
			defaultKubectlConfig = test.defaultKubectlConfig

			rcConf, err := getKubeClientConfig(test.config, test.overwrites)

			if err != nil && !test.error {
				t.Errorf("expected error, but instead received: %v", rcConf)
				return
			}

			if !reflect.DeepEqual(rcConf, test.expected) {
				t.Errorf("expected: '%v', got: '%v'", test.expected, rcConf)
			}
		})
	}
}

func TestWaitForPodRunning(t *testing.T) {
	version, codec := testVersionAndCodec()
	retries := 0

	tests := []struct {
		Name         string
		Pod          *apiv1.Pod
		Config       *common.KubernetesConfig
		ClientFunc   func(*http.Request) (*http.Response, error)
		PodEndPhase  apiv1.PodPhase
		Retries      int
		Error        bool
		ExactRetries bool
	}{
		{
			Name: "ensure function retries until ready",
			Pod: &apiv1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
				},
			},
			Config: &common.KubernetesConfig{},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case p == "/api/"+version+"/namespaces/test-ns/pods/test-pod" && m == "GET":
					pod := &apiv1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-pod",
							Namespace: "test-ns",
						},
						Status: apiv1.PodStatus{
							Phase: apiv1.PodPending,
						},
					}

					if retries > 1 {
						pod.Status.Phase = apiv1.PodRunning
						pod.Status.ContainerStatuses = []apiv1.ContainerStatus{
							{
								Ready: false,
							},
						}
					}

					if retries > 2 {
						pod.Status.Phase = apiv1.PodRunning
						pod.Status.ContainerStatuses = []apiv1.ContainerStatus{
							{
								Ready: true,
							},
						}
					}
					retries++
					return &http.Response{StatusCode: http.StatusOK, Body: objBody(codec, pod), Header: map[string][]string{
						"Content-Type": []string{"application/json"},
					}}, nil
				default:
					// Ensures no GET is performed when deleting by name
					t.Errorf("unexpected request: %s %#v\n%#v", req.Method, req.URL, req)
					return nil, fmt.Errorf("unexpected request")
				}
			},
			PodEndPhase: apiv1.PodRunning,
			Retries:     2,
		},
		{
			Name: "ensure function errors if pod already succeeded",
			Pod: &apiv1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
				},
			},
			Config: &common.KubernetesConfig{},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case p == "/api/"+version+"/namespaces/test-ns/pods/test-pod" && m == "GET":
					pod := &apiv1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-pod",
							Namespace: "test-ns",
						},
						Status: apiv1.PodStatus{
							Phase: apiv1.PodSucceeded,
						},
					}
					return &http.Response{StatusCode: http.StatusOK, Body: objBody(codec, pod), Header: map[string][]string{
						"Content-Type": []string{"application/json"},
					}}, nil
				default:
					// Ensures no GET is performed when deleting by name
					t.Errorf("unexpected request: %s %#v\n%#v", req.Method, req.URL, req)
					return nil, fmt.Errorf("unexpected request")
				}
			},
			Error:       true,
			PodEndPhase: apiv1.PodSucceeded,
		},
		{
			Name: "ensure function returns error if pod unknown",
			Pod: &apiv1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "test-ns",
				},
			},
			Config: &common.KubernetesConfig{},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("error getting pod")
			},
			PodEndPhase: apiv1.PodUnknown,
			Error:       true,
		},
		{
			Name: "ensure poll parameters work correctly",
			Pod: &apiv1.Pod{
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
				case p == "/api/"+version+"/namespaces/test-ns/pods/test-pod" && m == "GET":
					pod := &apiv1.Pod{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "test-pod",
							Namespace: "test-ns",
						},
					}
					if retries > 3 {
						t.Errorf("Too many retries for the given poll parameters. (Expected 3)")
					}
					retries++
					return &http.Response{StatusCode: http.StatusOK, Body: objBody(codec, pod), Header: map[string][]string{
						"Content-Type": []string{"application/json"},
					}}, nil
				default:
					// Ensures no GET is performed when deleting by name
					t.Errorf("unexpected request: %s %#v\n%#v", req.Method, req.URL, req)
					return nil, fmt.Errorf("unexpected request")
				}
			},
			PodEndPhase:  apiv1.PodUnknown,
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

type testWriter struct {
	call func([]byte) (int, error)
}

func (t testWriter) Write(b []byte) (int, error) {
	return t.call(b)
}

func objBody(codec runtime.Codec, obj runtime.Object) io.ReadCloser {
	return ioutil.NopCloser(bytes.NewReader([]byte(runtime.EncodeOrDie(codec, obj))))
}

func testKubernetesClient(version string, httpClient *http.Client) *kubernetes.Clientset {
	conf := restclient.Config{
		ContentConfig: restclient.ContentConfig{
			GroupVersion: &schema.GroupVersion{Version: version},
		},
	}
	kube := kubernetes.NewForConfigOrDie(&conf)
	fakeClient := fake.RESTClient{Client: httpClient}
	kube.Core().RESTClient().(*restclient.RESTClient).Client = fakeClient.Client
	kube.Extensions().RESTClient().(*restclient.RESTClient).Client = fakeClient.Client

	return kube
}

// minimal port from k8s.io/kubernetes/pkg/testapi
func testVersionAndCodec() (version string, codec runtime.Codec) {
	scheme := runtime.NewScheme()

	scheme.AddIgnoredConversionType(&metav1.TypeMeta{}, &metav1.TypeMeta{})
	scheme.AddKnownTypes(
		apiv1.SchemeGroupVersion,
		&apiv1.Pod{},
		&metav1.Status{},
	)

	codecs := runtimeserializer.NewCodecFactory(scheme)
	codec = codecs.LegacyCodec(apiv1.SchemeGroupVersion)
	version = apiv1.SchemeGroupVersion.Version

	return
}
