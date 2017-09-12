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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	_ "k8s.io/client-go/pkg/api/install"
	apiv1 "k8s.io/client-go/pkg/api/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestGetKubeClientConfig(t *testing.T) {
	tests := []struct {
		CertFile, KeyFile, CAFile, Host string
		Error                           bool
		Expected                        *restclient.Config
	}{
		{
			CertFile: "test",
			Error:    true,
		},
		{
			CertFile: "crt",
			KeyFile:  "key",
			CAFile:   "ca",
			Host:     "host",
			Expected: &restclient.Config{
				Host: "host",
				TLSClientConfig: restclient.TLSClientConfig{
					CertFile: "crt",
					KeyFile:  "key",
					CAFile:   "ca",
				},
			},
		},
		{
			Host: "host",
			Expected: &restclient.Config{
				Host: "host",
			},
		},
	}
	for _, test := range tests {
		rcConf, err := getKubeClientConfig(&common.KubernetesConfig{
			Host:     test.Host,
			CertFile: test.CertFile,
			KeyFile:  test.KeyFile,
			CAFile:   test.CAFile,
		})

		if err != nil && !test.Error {
			t.Errorf("expected error, but instead received: %v", rcConf)
			continue
		}

		if !reflect.DeepEqual(rcConf, test.Expected) {
			t.Errorf("expected: '%v', got: '%v'", test.Expected, rcConf)
			continue
		}
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
			c := kubernetes.NewForConfigOrDie(&restclient.Config{ContentConfig: restclient.ContentConfig{GroupVersion: &schema.GroupVersion{Version: version}}})
			fakeClient := fake.RESTClient{
				// Codec:  codec,
				Client: fake.CreateHTTPClient(test.ClientFunc),
			}
			c.CoreV1().RESTClient().(*restclient.RESTClient).Client = fakeClient.Client
			// c.Client = fakeClient.Client
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
	versionGroup := schema.GroupVersion{Group: api.GroupName, Version: api.Registry.GroupOrDie(api.GroupName).GroupVersion.Version}
	version = versionGroup.Version

	serializer := runtime.SerializerInfo{}
	if serializer.Serializer == nil {
		codec = api.Codecs.LegacyCodec(versionGroup)
	} else {
		codec = api.Codecs.CodecForVersions(serializer.Serializer, api.Codecs.UniversalDeserializer(), schema.GroupVersions{versionGroup}, nil)
	}

	return
}
