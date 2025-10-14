//go:build !integration

/*
Copyright 2014 The Kubernetes Authors All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package kubernetes

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	testclient "k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"
	k8stesting "k8s.io/client-go/testing"
)

type fakeRemoteExecutor struct {
	method  string
	url     *url.URL
	execErr error
}

func (f *fakeRemoteExecutor) Execute(
	ctx context.Context,
	method string,
	url *url.URL,
	config *restclient.Config,
	stdin io.Reader,
	stdout, stderr io.Writer,
	tty bool,
) error {
	f.method = method
	f.url = url
	return f.execErr
}

func TestExec(t *testing.T) {
	version, codec := testVersionAndCodec()
	tests := []struct {
		name, version, podPath, execPath string
		pod                              *api.Pod
		tty, execErr                     bool
	}{
		{
			name:     "pod exec",
			version:  version,
			podPath:  "/api/" + version + "/namespaces/test/pods/foo",
			execPath: "/api/" + version + "/namespaces/test/pods/foo/exec",
			pod:      execPod(),
		},
		{
			name:     "pod exec with tty",
			version:  version,
			podPath:  "/api/" + version + "/namespaces/test/pods/foo",
			execPath: "/api/" + version + "/namespaces/test/pods/foo/exec",
			pod:      execPod(),
			tty:      true,
		},
		{
			name:     "pod exec error",
			version:  version,
			podPath:  "/api/" + version + "/namespaces/test/pods/foo",
			execPath: "/api/" + version + "/namespaces/test/pods/foo/exec",
			pod:      execPod(),
			execErr:  true,
		},
	}

	for _, test := range tests {
		// Create a fake kubeClient
		fakeClient := fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
			switch p, m := req.URL.Path, req.Method; {
			case p == test.podPath && m == http.MethodGet:
				body := objBody(codec, test.pod)
				return &http.Response{StatusCode: http.StatusOK, Body: body, Header: map[string][]string{
					common.ContentType: {"application/json"},
				}}, nil
			default:
				// Ensures no GET is performed when deleting by name
				t.Errorf("%s: unexpected request: %s %#v\n%#v", test.name, req.Method, req.URL, req)
				return nil, fmt.Errorf("unexpected request")
			}
		})
		c := testKubernetesClient(version, fakeClient)

		ex := &fakeRemoteExecutor{}
		if test.execErr {
			ex.execErr = fmt.Errorf("exec error")
		}

		params := &ExecOptions{
			PodName:       "foo",
			ContainerName: "bar",
			Namespace:     "test",
			Command:       []string{"command"},
			In:            bytes.NewBuffer([]byte{}),
			Out:           bytes.NewBuffer([]byte{}),
			Err:           bytes.NewBuffer([]byte{}),
			Stdin:         true,
			Executor:      ex,
			KubeClient:    c,
			Context:       t.Context(),
		}
		err := params.Run()
		if test.execErr && err != ex.execErr {
			t.Errorf("%s: Unexpected exec error: %v", test.name, err)
			continue
		}
		if !test.execErr && err != nil {
			t.Errorf("%s: Unexpected error: %v", test.name, err)
			continue
		}
		if test.execErr {
			continue
		}
		if ex.url.Path != test.execPath {
			t.Errorf("%s: Did not get expected path for exec request", test.name)
			continue
		}
		if ex.method != http.MethodPost {
			t.Errorf("%s: Did not get method for exec request: %s", test.name, ex.method)
		}
	}
}

func execPod() *api.Pod {
	return execPodWithPhase(api.PodRunning)
}

func execPodWithPhase(phase api.PodPhase) *api.Pod {
	return &api.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "test", ResourceVersion: "10"},
		Spec: api.PodSpec{
			RestartPolicy: api.RestartPolicyAlways,
			DNSPolicy:     api.DNSClusterFirst,
			Containers: []api.Container{
				{
					Name: "bar",
				},
			},
		},
		Status: api.PodStatus{
			Phase: phase,
		},
	}
}

func TestAttach(t *testing.T) {
	const (
		testPodNameRunning = "running"
		testPodNamePending = "pending"
		testNamespace      = "someNamespace"
		testContainerName  = "someContainer"

		testKubeHost         = "some-host:123"
		testScheme           = "some-scheme"
		testBasePath         = "basePath"
		testVersionedAPIPath = "versionedAPI"
	)

	testPods := func() []runtime.Object {
		return []runtime.Object{
			&api.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: testPodNameRunning, Namespace: testNamespace},
				Spec: api.PodSpec{
					Containers: []api.Container{{Name: testContainerName}},
				},
				Status: api.PodStatus{Phase: api.PodRunning},
			},
			&api.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: testPodNamePending, Namespace: testNamespace},
				Spec: api.PodSpec{
					Containers: []api.Container{{Name: testContainerName}},
				},
				Status: api.PodStatus{Phase: api.PodPending},
			},
		}
	}

	tests := []struct {
		name          string
		attachTo      string
		executeErr    error
		kubeAPIErr    error
		expectExecute bool
		expectedErr   string
	}{
		{
			name:          "pod attach",
			attachTo:      testPodNameRunning,
			expectExecute: true,
		},
		{
			name:        "pod does not exist",
			attachTo:    "doesNotExist",
			expectedErr: "not found",
		},
		{
			name:        "pod not running",
			attachTo:    testPodNamePending,
			expectedErr: "is not running and cannot execute commands",
		},
		{
			name:          "execute error bubbles up",
			attachTo:      testPodNameRunning,
			executeErr:    fmt.Errorf("some error on execute"),
			expectExecute: true,
			expectedErr:   "some error on execute",
		},
		{
			name:        "kube API error bubbles up",
			kubeAPIErr:  fmt.Errorf("some kube API error"),
			expectedErr: "some kube API error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClientSet := testclient.NewSimpleClientset(testPods()...)
			clientConfig := &restclient.Config{}

			fakeRESTRequest := restclient.NewRequestWithClient(
				&url.URL{Host: testKubeHost, Scheme: testScheme, Path: testBasePath},
				testVersionedAPIPath,
				restclient.ClientContentConfig{
					GroupVersion: schema.GroupVersion{Group: "", Version: "v1"},
				},
				nil,
			)
			fakeRESTClient := &FakeRESTClient{fakePostRequest: fakeRESTRequest}
			fakeClient := &FakeClient{
				FakeCoreV1: &FakeCoreV1{
					CoreV1Interface: fakeClientSet.CoreV1(),
					FakeRESTClient:  fakeRESTClient,
				},
			}

			if err := test.kubeAPIErr; err != nil {
				fakeClientSet.PrependReactor("*", "*", func(action k8stesting.Action) (bool, runtime.Object, error) {
					return true, nil, err
				})
			}

			mockExecutor := NewMockRemoteExecutor(t)

			if test.expectExecute {
				stdinMatcher := mock.MatchedBy(func(stdin io.Reader) bool {
					b, err := io.ReadAll(stdin)
					require.NoError(t, err, "reading stdin")
					return string(b) == "sleep 1\n"
				})

				expectedURL := &url.URL{
					Scheme:   testScheme,
					Host:     testKubeHost,
					Path:     fmt.Sprintf("/%s/%s/namespaces/%s/pods/%s/attach", testBasePath, testVersionedAPIPath, testNamespace, test.attachTo),
					RawQuery: fmt.Sprintf("container=%s&stdin=true", testContainerName),
				}

				mockExecutor.
					On("Execute", t.Context(), http.MethodPost, expectedURL, clientConfig, stdinMatcher, nil, nil, false).
					Return(test.executeErr).
					Once()
			}

			opts := &AttachOptions{
				Namespace:     testNamespace,
				PodName:       test.attachTo,
				ContainerName: testContainerName,
				Command:       []string{"sleep", "1"},
				Executor:      mockExecutor,
				KubeClient:    fakeClient,
				Config:        clientConfig,
				Context:       t.Context(),
			}
			err := opts.Run()
			if test.expectedErr != "" {
				assert.ErrorContains(t, err, test.expectedErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
