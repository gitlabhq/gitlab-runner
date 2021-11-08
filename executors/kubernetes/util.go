package kubernetes

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/net/context"
	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes/internal/pull"
)

type kubeConfigProvider func() (*restclient.Config, error)

type resourceQuantityError struct {
	resource string
	value    string
	inner    error
}

func (r *resourceQuantityError) Error() string {
	return fmt.Sprintf("parsing resource %q with value %q: %q", r.resource, r.value, r.inner)
}

func (r *resourceQuantityError) Is(err error) bool {
	t, ok := err.(*resourceQuantityError)
	return ok && r.resource == t.resource && r.value == t.value && r.inner == t.inner
}

var (
	// inClusterConfig parses kubernetes configuration reading in cluster values
	inClusterConfig kubeConfigProvider = restclient.InClusterConfig
	// defaultKubectlConfig parses kubectl configuration ad loads the default cluster
	defaultKubectlConfig kubeConfigProvider = loadDefaultKubectlConfig
)

func getKubeClientConfig(
	config *common.KubernetesConfig,
	overwrites *overwrites,
) (kubeConfig *restclient.Config, err error) {
	if len(config.Host) > 0 {
		kubeConfig, err = getOutClusterClientConfig(config)
	} else {
		kubeConfig, err = guessClientConfig()
	}
	if err != nil {
		return nil, err
	}

	// apply overwrites
	if overwrites.bearerToken != "" {
		kubeConfig.BearerToken = overwrites.bearerToken
	}

	kubeConfig.UserAgent = common.AppVersion.UserAgent()

	return kubeConfig, nil
}

func getOutClusterClientConfig(config *common.KubernetesConfig) (*restclient.Config, error) {
	kubeConfig := &restclient.Config{
		Host:        config.Host,
		BearerToken: config.BearerToken,
		TLSClientConfig: restclient.TLSClientConfig{
			CAFile: config.CAFile,
		},
	}

	// certificate based auth
	if config.CertFile != "" {
		if config.KeyFile == "" || config.CAFile == "" {
			return nil, fmt.Errorf("ca file, cert file and key file must be specified when using file based auth")
		}

		kubeConfig.TLSClientConfig.CertFile = config.CertFile
		kubeConfig.TLSClientConfig.KeyFile = config.KeyFile
	}

	return kubeConfig, nil
}

func guessClientConfig() (*restclient.Config, error) {
	// Try in cluster config first
	if inClusterCfg, err := inClusterConfig(); err == nil {
		return inClusterCfg, nil
	}

	// in cluster config failed. Reading default kubectl config
	return defaultKubectlConfig()
}

func loadDefaultKubectlConfig() (*restclient.Config, error) {
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		return nil, err
	}

	return clientcmd.NewDefaultClientConfig(*config, &clientcmd.ConfigOverrides{}).ClientConfig()
}

func closeKubeClient(client *kubernetes.Clientset) bool {
	if client == nil {
		return false
	}
	rest, ok := client.CoreV1().RESTClient().(*restclient.RESTClient)
	if !ok || rest.Client == nil || rest.Client.Transport == nil {
		return false
	}
	if transport, ok := rest.Client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
		return true
	}
	return false
}

func isRunning(pod *api.Pod) (bool, error) {
	switch pod.Status.Phase {
	case api.PodRunning:
		return true, nil
	case api.PodSucceeded:
		return false, fmt.Errorf("pod already succeeded before it begins running")
	case api.PodFailed:
		return false, fmt.Errorf("pod status is failed")
	default:
		return false, nil
	}
}

type podPhaseResponse struct {
	done  bool
	phase api.PodPhase
	err   error
}

func getPodPhase(c *kubernetes.Clientset, pod *api.Pod, out io.Writer) podPhaseResponse {
	// TODO: handle the context properly with https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27932
	pod, err := c.CoreV1().Pods(pod.Namespace).Get(context.TODO(), pod.Name, metav1.GetOptions{})
	if err != nil {
		return podPhaseResponse{true, api.PodUnknown, err}
	}

	ready, err := isRunning(pod)
	if err != nil || ready {
		return podPhaseResponse{true, pod.Status.Phase, err}
	}

	// check status of containers
	for _, container := range append(pod.Status.ContainerStatuses, pod.Status.InitContainerStatuses...) {
		if container.Ready {
			continue
		}
		waiting := container.State.Waiting
		if waiting == nil {
			continue
		}

		switch waiting.Reason {
		case "InvalidImageName":
			err = &common.BuildError{Inner: fmt.Errorf("image pull failed: %s", waiting.Message)}
			return podPhaseResponse{true, api.PodUnknown, err}
		case "ErrImagePull", "ImagePullBackOff":
			msg := fmt.Sprintf("image pull failed: %s", waiting.Message)
			imagePullErr := &pull.ImagePullError{Message: msg, Image: container.Image}
			return podPhaseResponse{
				true,
				api.PodUnknown,
				&common.BuildError{Inner: imagePullErr, FailureReason: common.ScriptFailure},
			}
		}
	}

	_, _ = fmt.Fprintf(
		out,
		"Waiting for pod %s/%s to be running, status is %s\n",
		pod.Namespace,
		pod.Name,
		pod.Status.Phase,
	)

	for _, condition := range pod.Status.Conditions {
		// skip conditions with no reason, these are typically expected pod conditions
		if condition.Reason == "" {
			continue
		}

		_, _ = fmt.Fprintf(
			out,
			"\t%s: %q\n",
			condition.Reason,
			condition.Message,
		)
	}

	return podPhaseResponse{false, pod.Status.Phase, nil}
}

func triggerPodPhaseCheck(c *kubernetes.Clientset, pod *api.Pod, out io.Writer) <-chan podPhaseResponse {
	errc := make(chan podPhaseResponse)
	go func() {
		defer close(errc)
		errc <- getPodPhase(c, pod, out)
	}()
	return errc
}

// waitForPodRunning will use client c to detect when pod reaches the PodRunning
// state. It returns the final PodPhase once either PodRunning, PodSucceeded or
// PodFailed has been reached. In the case of PodRunning, it will also wait until
// all containers within the pod are also Ready.
// It returns error if the call to retrieve pod details fails or the timeout is
// reached.
// The timeout and polling values are configurable through KubernetesConfig
// parameters.
func waitForPodRunning(
	ctx context.Context,
	c *kubernetes.Clientset,
	pod *api.Pod,
	out io.Writer,
	config *common.KubernetesConfig,
) (api.PodPhase, error) {
	pollInterval := config.GetPollInterval()
	pollAttempts := config.GetPollAttempts()
	for i := 0; i <= pollAttempts; i++ {
		select {
		case r := <-triggerPodPhaseCheck(c, pod, out):
			if !r.done {
				time.Sleep(time.Duration(pollInterval) * time.Second)
				continue
			}
			return r.phase, r.err
		case <-ctx.Done():
			return api.PodUnknown, ctx.Err()
		}
	}
	return api.PodUnknown, errors.New("timed out waiting for pod to start")
}

// limits takes a string representing CPU, memory and ephemeralStorage limits,
// and returns a ResourceList with appropriately scaled Quantity
// values for Kubernetes. This allows users to write "500m" for CPU,
// "50Mi" for memory and "1Gi" for ephemeral storage (etc.)
func createResourceList(cpu, memory, ephemeralStorage string) (api.ResourceList, error) {
	var rCPU, rMem, rStor resource.Quantity
	var err error

	parse := func(s string) (resource.Quantity, error) {
		var q resource.Quantity
		if s == "" {
			return q, nil
		}
		if q, err = resource.ParseQuantity(s); err != nil {
			return q, err
		}
		return q, nil
	}

	if rCPU, err = parse(cpu); err != nil {
		return api.ResourceList{}, &resourceQuantityError{resource: "cpu", value: cpu, inner: err}
	}

	if rMem, err = parse(memory); err != nil {
		return api.ResourceList{}, &resourceQuantityError{resource: "memory", value: memory, inner: err}
	}

	if rStor, err = parse(ephemeralStorage); err != nil {
		return api.ResourceList{}, &resourceQuantityError{
			resource: "ephemeralStorage",
			value:    ephemeralStorage,
			inner:    err,
		}
	}

	l := make(api.ResourceList)

	q := resource.Quantity{}
	if rCPU != q {
		l[api.ResourceCPU] = rCPU
	}
	if rMem != q {
		l[api.ResourceMemory] = rMem
	}
	if rStor != q {
		l[api.ResourceEphemeralStorage] = rStor
	}

	return l, nil
}

// buildVariables converts a common.BuildVariables into a list of
// kubernetes EnvVar objects
func buildVariables(bv common.JobVariables) []api.EnvVar {
	e := make([]api.EnvVar, len(bv))
	for i, b := range bv {
		e[i] = api.EnvVar{
			Name:  b.Key,
			Value: b.Value,
		}
	}
	return e
}

// Sanitize labels to match Kubernetes restrictions from https://kubernetes.io/
// /docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set
//nolint:gocognit
func sanitizeLabel(value string) string {
	mapFn := func(r rune) rune {
		if r >= 'a' && r <= 'z' ||
			r >= 'A' && r <= 'Z' ||
			r >= '0' && r <= '9' ||
			r == '-' || r == '_' || r == '.' {
			return r
		}
		return '_'
	}

	// only alphanumerics, dashes (-), underscores (_), dots (.) are valid
	value = strings.Map(mapFn, value)

	// must start/end with alphanumerics only
	value = strings.Trim(value, "-_.")

	// length must be <= 63 characters
	if len(value) > 63 {
		value = value[:63]
	}

	// trim again if required after shortening
	return strings.Trim(value, "-_.")
}
