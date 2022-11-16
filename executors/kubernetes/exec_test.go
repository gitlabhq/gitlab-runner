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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	api "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/retry"
)

type fakeRemoteExecutor struct {
	method  string
	url     *url.URL
	execErr error
}

func (f *fakeRemoteExecutor) Execute(
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
					"Content-Type": {"application/json"},
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

		bufOut := bytes.NewBuffer([]byte{})
		bufErr := bytes.NewBuffer([]byte{})
		bufIn := bytes.NewBuffer([]byte{})

		params := &ExecOptions{
			PodName:       "foo",
			ContainerName: "bar",
			Namespace:     "test",
			Command:       []string{"command"},
			In:            bufIn,
			Out:           bufOut,
			Err:           bufErr,
			Stdin:         true,
			Executor:      ex,
			Client:        c,
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

func TestExecOptions_ShouldRetry(t *testing.T) {
	testCommandShouldRetry(t, &ExecOptions{})
}

func TestAttachOptions_ShouldRetry(t *testing.T) {
	testCommandShouldRetry(t, &AttachOptions{})
}

func testCommandShouldRetry(t *testing.T, retryable retry.Retryable) {
	tests := map[string]struct {
		err                 error
		tries               int
		expectedShouldRetry bool
	}{
		"no error, shouldn't retry": {
			err:                 nil,
			expectedShouldRetry: false,
		},
		"different error, shouldn't retry": {
			err:                 errors.New("err"),
			expectedShouldRetry: false,
		},
		"empty status error, shouldn't retry": {
			err:                 &kubeerrors.StatusError{},
			expectedShouldRetry: false,
		},
		"status error different code, shouldn't retry": {
			err: &kubeerrors.StatusError{
				ErrStatus: metav1.Status{Message: "error dialing backend: not found", Code: http.StatusNotFound},
			},
			expectedShouldRetry: false,
		},
		"status error different message, shouldn't retry": {
			err: &kubeerrors.StatusError{
				ErrStatus: metav1.Status{Message: "random", Code: http.StatusInternalServerError},
			},
			expectedShouldRetry: false,
		},
		"status error matching message, should retry": {
			err: &kubeerrors.StatusError{
				ErrStatus: metav1.Status{Message: "error dialing backend: EOF", Code: http.StatusInternalServerError},
			},
			expectedShouldRetry: true,
		},
		"status error matching code and message, over max tries limit": {
			err: &kubeerrors.StatusError{
				ErrStatus: metav1.Status{Message: "error dialing backend: EOF", Code: http.StatusInternalServerError},
			},
			tries:               commandConnectFailureMaxTries + 1,
			expectedShouldRetry: false,
		},
		"no error, over max tries limit": {
			err:                 nil,
			tries:               commandConnectFailureMaxTries + 1,
			expectedShouldRetry: false,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.Equal(t, tt.expectedShouldRetry, retryable.ShouldRetry(tt.tries, tt.err))
		})
	}
}

func TestAttach(t *testing.T) {
	version, codec := testVersionAndCodec()

	fakeClient := fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
		switch p, m := req.URL.Path, req.Method; {
		case p == "/api/v1/namespaces/test-resource/pods/test-resource" && m == http.MethodGet:
			body := objBody(codec, execPod())
			return &http.Response{StatusCode: http.StatusOK, Body: body, Header: map[string][]string{
				"Content-Type": {"application/json"},
			}}, nil

		default:
			return nil, fmt.Errorf("unexpected request")
		}
	})

	client := testKubernetesClient(version, fakeClient)
	clientConfig := &restclient.Config{}

	mockExecutor := &MockRemoteExecutor{}
	defer mockExecutor.AssertExpectations(t)

	urlMatcher := mock.MatchedBy(func(url *url.URL) bool {
		return url.Path == "/api/v1/namespaces/test/pods/foo/attach"
	})
	stdinMatcher := mock.MatchedBy(func(stdin io.Reader) bool {
		b, _ := io.ReadAll(stdin)
		return string(b) == "sleep 1\n"
	})
	mockExecutor.
		On("Execute", http.MethodPost, urlMatcher, clientConfig, stdinMatcher, nil, nil, false).
		Return(nil).
		Once()

	opts := &AttachOptions{
		Namespace:     "test-resource",
		PodName:       "test-resource",
		ContainerName: "test-resource",
		Command:       []string{"sleep", "1"},
		Executor:      mockExecutor,
		Client:        client,
		Config:        clientConfig,
	}

	assert.Nil(t, opts.Run())
}

func TestAttachErrorGettingPod(t *testing.T) {
	err := errors.New("error")

	version, _ := testVersionAndCodec()

	fakeClient := fake.CreateHTTPClient(func(*http.Request) (*http.Response, error) {
		return nil, err
	})

	client := testKubernetesClient(version, fakeClient)
	clientConfig := &restclient.Config{}

	opts := &AttachOptions{
		Namespace:     "test-resource",
		PodName:       "test-resource",
		ContainerName: "test-resource",
		Client:        client,
		Config:        clientConfig,
	}

	assert.ErrorIs(t, opts.Run(), err)
}

func TestAttachPodNotRunning(t *testing.T) {
	version, codec := testVersionAndCodec()

	fakeClient := fake.CreateHTTPClient(func(*http.Request) (*http.Response, error) {
		body := objBody(codec, execPodWithPhase(api.PodUnknown))
		return &http.Response{StatusCode: http.StatusOK, Body: body, Header: map[string][]string{
			"Content-Type": {"application/json"},
		}}, nil
	})

	client := testKubernetesClient(version, fakeClient)
	clientConfig := &restclient.Config{}

	opts := &AttachOptions{
		Namespace:     "test-resource",
		PodName:       "test-resource",
		ContainerName: "test-resource",
		Client:        client,
		Config:        clientConfig,
	}

	err := opts.Run()
	require.Error(t, err)
	assert.Contains(t, err.Error(), api.PodUnknown)
}
