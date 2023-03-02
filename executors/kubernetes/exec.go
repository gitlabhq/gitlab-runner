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

This file was modified by James Munnelly (https://gitlab.com/u/munnerz)
*/

package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
	api "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

const (
	// errorDialingBackendMessage is an error prefix that is encountered when
	// connectivity to a Pod fails. This can happen for a number of reasons,
	// such as the Pod or Node still being configured.
	errorDialingBackendMessage = "error dialing backend"

	// commandConnectFailureMaxTries is the number of attempts we retry when
	// the connection to a Pod fails. There's an exponential backoff, which
	// maxes out at 5 seconds.
	commandConnectFailureMaxTries = 30
)

// RemoteExecutor defines the interface accepted by the Exec command - provided for test stubbing
//
//go:generate mockery --name=RemoteExecutor --inpackage
type RemoteExecutor interface {
	Execute(
		method string,
		url *url.URL,
		config *restclient.Config,
		stdin io.Reader,
		stdout, stderr io.Writer,
		tty bool,
	) error
}

// DefaultRemoteExecutor is the standard implementation of remote command execution
type DefaultRemoteExecutor struct{}

func (*DefaultRemoteExecutor) Execute(
	method string,
	url *url.URL,
	config *restclient.Config,
	stdin io.Reader,
	stdout, stderr io.Writer,
	tty bool,
) error {
	exec, err := remotecommand.NewSPDYExecutor(config, method, url)
	if err != nil {
		return err
	}

	return exec.StreamWithContext(
		context.TODO(),
		remotecommand.StreamOptions{
			Stdin:  stdin,
			Stdout: stdout,
			Stderr: stderr,
			Tty:    tty,
		})
}

// AttachOptions declare the arguments accepted by the Attach command
type AttachOptions struct {
	Namespace     string
	PodName       string
	ContainerName string
	Command       []string

	Executor RemoteExecutor
	Client   *kubernetes.Clientset
	Config   *restclient.Config
}

// Run executes a validated remote execution against a pod.
func (p *AttachOptions) Run() error {
	// TODO: handle the context properly with https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27932
	pod, err := p.Client.CoreV1().Pods(p.Namespace).Get(context.TODO(), p.PodName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("couldn't get pod details: %w", err)
	}

	if pod.Status.Phase != api.PodRunning {
		return fmt.Errorf(
			"pod %q (on namespace %q) is not running and cannot execute commands; current phase is %q",
			p.PodName, p.Namespace, pod.Status.Phase,
		)
	}

	// Ending with a newline is important to actually run the script
	stdin := strings.NewReader(strings.Join(p.Command, " ") + "\n")

	req := p.Client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("attach").
		VersionedParams(&api.PodAttachOptions{
			Container: p.ContainerName,
			Stdin:     true,
			Stdout:    false,
			Stderr:    false,
			TTY:       false,
		}, scheme.ParameterCodec)

	return p.Executor.Execute(http.MethodPost, req.URL(), p.Config, stdin, nil, nil, false)
}

func (p *AttachOptions) ShouldRetry(times int, err error) bool {
	return shouldRetryKubernetesError(times, err)
}

func shouldRetryKubernetesError(times int, err error) bool {
	var statusError *kubeerrors.StatusError

	if times < commandConnectFailureMaxTries &&
		errors.As(err, &statusError) &&
		statusError.ErrStatus.Code == http.StatusInternalServerError &&
		strings.HasPrefix(statusError.ErrStatus.Message, errorDialingBackendMessage) {
		return true
	}

	return false
}

// ExecOptions declare the arguments accepted by the Exec command
type ExecOptions struct {
	Namespace     string
	PodName       string
	ContainerName string
	Stdin         bool
	Command       []string

	In  io.Reader
	Out io.Writer
	Err io.Writer

	Executor RemoteExecutor
	Client   *kubernetes.Clientset
	Config   *restclient.Config
}

// Run executes a validated remote execution against a pod.
func (p *ExecOptions) Run() error {
	// TODO: handle the context properly with https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27932
	pod, err := p.Client.CoreV1().Pods(p.Namespace).Get(context.TODO(), p.PodName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("couldn't get pod details: %w", err)
	}

	if pod.Status.Phase != api.PodRunning {
		return fmt.Errorf(
			"pod %q (on namespace '%s') is not running and cannot execute commands; current phase is %q",
			p.PodName, p.Namespace, pod.Status.Phase,
		)
	}

	if p.ContainerName == "" {
		logrus.Infof("defaulting container name to '%s'", pod.Spec.Containers[0].Name)
		p.ContainerName = pod.Spec.Containers[0].Name
	}

	return p.executeRequest()
}

func (p *ExecOptions) executeRequest() error {
	req := p.Client.CoreV1().RESTClient().Post().
		Resource("pods").
		Name(p.PodName).
		Namespace(p.Namespace).
		SubResource("exec").
		Param("container", p.ContainerName)

	var stdin io.Reader
	if p.Stdin {
		stdin = p.In
	}

	req.VersionedParams(&api.PodExecOptions{
		Container: p.ContainerName,
		Command:   p.Command,
		Stdin:     stdin != nil,
		Stdout:    p.Out != nil,
		Stderr:    p.Err != nil,
	}, scheme.ParameterCodec)

	return p.Executor.Execute(http.MethodPost, req.URL(), p.Config, stdin, p.Out, p.Err, false)
}

func (p *ExecOptions) ShouldRetry(times int, err error) bool {
	return shouldRetryKubernetesError(times, err)
}

func init() {
	runtime.ErrorHandlers = append(runtime.ErrorHandlers, func(err error) {
		logrus.WithError(err).Error("K8S stream error")
	})

	runtime.PanicHandlers = append(runtime.PanicHandlers, func(r interface{}) {
		logrus.Errorf("K8S stream panic: %v", r)
	})
}
