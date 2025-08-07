package kubernetes

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
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
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes/internal/watchers"
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
	if config.Host != "" {
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

func getContainerStatus(containerStatuses []api.ContainerStatus, containerName string) (api.ContainerStatus, bool) {
	for i := range containerStatuses {
		if containerStatuses[i].Name == containerName {
			return containerStatuses[i], true
		}
	}
	return api.ContainerStatus{}, false
}

func waitForRunningContainer(ctx context.Context, client kubernetes.Interface, timeoutSeconds int, namespace, pod, container string) error {
	// kubeAPI: pods, watch, FF_KUBERNETES_HONOR_ENTRYPOINT=true,FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY=false
	watcher, err := client.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		FieldSelector:  "status.phase=Running,metadata.name=" + pod,
		TimeoutSeconds: common.Int64Ptr(int64(timeoutSeconds)),
	})
	if err != nil {
		return err
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		pod, ok := event.Object.(*api.Pod)
		if !ok {
			return fmt.Errorf("event object is not a pod: %v", event.Object)
		}

		containerStatus, ok := getContainerStatus(pod.Status.ContainerStatuses, container)
		if !ok {
			return fmt.Errorf("container status for %q not found", container)
		}

		if terminated := containerStatus.State.Terminated; terminated != nil {
			if terminated.ExitCode != 0 {
				return fmt.Errorf("container %q terminated with non-zero status: %d", container, terminated.ExitCode)
			}
			return nil
		}

		if running := containerStatus.State.Running; running != nil {
			break
		}
	}

	return nil
}

func closeKubeClient(client kubernetes.Interface) bool {
	if client == nil {
		return false
	}
	// kubeAPI: ignore
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

func isRunning(pod *api.Pod, containers ...string) (bool, error) {
	switch pod.Status.Phase {
	case api.PodRunning:
		var readyCount int
		for _, c := range containers {
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.Name == c && cs.Ready {
					readyCount++
				}
			}
		}

		return readyCount == len(containers), nil
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

func getPodPhase(ctx context.Context, client kubernetes.Interface, pod *api.Pod, out io.Writer, containers ...string) (pf podPhaseResponse) {
	// kubeAPI: pods, get
	pod, err := client.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
	if err != nil {
		return podPhaseResponse{true, api.PodUnknown, err}
	}

	ready, err := isRunning(pod, containers...)
	if err != nil || ready {
		return podPhaseResponse{true, pod.Status.Phase, err}
	}

	containerStatuses := slices.Concat(pod.Status.ContainerStatuses, pod.Status.InitContainerStatuses)
	if err := watchers.CheckTerminalContainerErrors(containerStatuses); err != nil {
		return podPhaseResponse{true, api.PodUnknown, err}
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

func triggerPodPhaseCheck(ctx context.Context, c kubernetes.Interface, pod *api.Pod, out io.Writer, containers ...string) <-chan podPhaseResponse {
	errc := make(chan podPhaseResponse)
	go func() {
		defer close(errc)
		errc <- getPodPhase(ctx, c, pod, out, containers...)
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
// The containers parameter is optional and can be used to wait for a specific containers' readiness
func waitForPodRunning(
	ctx context.Context,
	c kubernetes.Interface,
	pod *api.Pod,
	out io.Writer,
	config *common.KubernetesConfig,
	containers ...string,
) (api.PodPhase, error) {
	pollInterval := config.GetPollInterval()
	pollAttempts := config.GetPollAttempts()
	for i := 0; i <= pollAttempts; i++ {
		select {
		case r := <-triggerPodPhaseCheck(ctx, c, pod, out, containers...):
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

func getPodLog(ctx context.Context, client kubernetes.Interface, pod *api.Pod) error {
	count := int64(10)
	podLogOptions := api.PodLogOptions{
		Container: "helper",
		Follow:    false,
		TailLines: &count,
	}

	//nolint:gocritic
	// kubeAPI: pods/log, get, FF_WAIT_FOR_POD_TO_BE_REACHABLE=true
	req := client.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &podLogOptions)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return fmt.Errorf("failed to open log stream for %s: %w", pod.Name, err)
	}
	defer podLogs.Close()

	return nil
}

func triggerPodReachableCheck(ctx context.Context, c kubernetes.Interface, pod *api.Pod) <-chan error {
	errc := make(chan error)
	go func() {
		defer close(errc)
		errc <- getPodLog(ctx, c, pod)
	}()
	return errc
}

func WaitForPodReachable(
	ctx context.Context,
	c kubernetes.Interface,
	pod *api.Pod,
	config *common.KubernetesConfig,
) error {
	pollInterval := config.GetPollInterval()
	pollAttempts := config.GetPollAttempts()
	for i := 0; i <= pollAttempts; i++ {
		select {
		case r := <-triggerPodReachableCheck(ctx, c, pod):
			if r != nil {
				time.Sleep(time.Duration(pollInterval) * time.Second)
				continue
			}

			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return errors.New("timed out waiting for pod to become attachable")
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

// buildVariables converts a common.JobVariables into a list of
// kubernetes EnvVar objects
// The order of keys is preserved, but duplicate elements (with the same name/key) will be deduped, the last one in
// the list wins.
func buildVariables(bv common.JobVariables) []api.EnvVar {
	idx := map[string]int{}
	envs := make([]api.EnvVar, 0, len(bv))

	i := 0
	for _, b := range bv {
		// For file-type secrets, substitute the path to the secret
		// for the secret value.
		if b.File {
			b.Value = bv.Get(b.Key)
		}
		e := api.EnvVar{
			Name:  b.Key,
			Value: b.Value,
		}
		if j, ok := idx[e.Name]; ok {
			envs[j] = e
			continue
		}

		envs = append(envs, e)
		idx[e.Name] = i
		i++
	}

	return slices.Clip(envs)
}

// Sanitize labels to match Kubernetes restrictions from https://kubernetes.io/
// /docs/concepts/overview/working-with-objects/labels/#syntax-and-character-set
//
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
