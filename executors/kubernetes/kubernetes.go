package kubernetes

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/jpillora/backoff"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	api "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // Register all available authentication methods

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/helperimage"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/services"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/dns"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/retry"
	"gitlab.com/gitlab-org/gitlab-runner/session/proxy"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

const (
	buildContainerName  = "build"
	helperContainerName = "helper"

	detectShellScriptName = "detect_shell_script"
)

var (
	executorOptions = executors.ExecutorOptions{
		DefaultCustomBuildsDirEnabled: true,
		DefaultBuildsDir:              "/builds",
		DefaultCacheDir:               "/cache",
		SharedBuildsDir:               false,
		Shell: common.ShellScriptInfo{
			Shell:         "bash",
			Type:          common.NormalShell,
			RunnerCommand: "/usr/bin/gitlab-runner-helper",
		},
		ShowHostname: true,
	}

	detectShellScript = shells.BashTrapShellScript + shells.BashDetectShellScript

	newLogProcessor = newKubernetesLogProcessor
)

type commandTerminatedError struct {
	exitCode int
}

func (c *commandTerminatedError) Error() string {
	return fmt.Sprintf("command terminated with exit code %d", c.exitCode)
}

func (c *commandTerminatedError) Is(err error) bool {
	_, ok := err.(*commandTerminatedError)
	return ok
}

type podPhaseError struct {
	name  string
	phase api.PodPhase
}

func (p *podPhaseError) Error() string {
	return fmt.Sprintf("pod %q status is %q", p.name, p.phase)
}

type kubernetesOptions struct {
	Image    common.Image
	Services common.Services
}

type executor struct {
	executors.AbstractExecutor

	kubeClient  *kubernetes.Clientset
	pod         *api.Pod
	configMap   *api.ConfigMap
	credentials *api.Secret
	options     *kubernetesOptions
	services    []api.Service

	configurationOverwrites *overwrites
	buildLimits             api.ResourceList
	serviceLimits           api.ResourceList
	helperLimits            api.ResourceList
	buildRequests           api.ResourceList
	serviceRequests         api.ResourceList
	helperRequests          api.ResourceList
	pullPolicy              common.KubernetesPullPolicy

	helperImageInfo helperimage.Info

	featureChecker featureChecker

	remoteProcessTerminated chan shells.TrapCommandExitStatus
	logProcessor            logProcessor
}

type serviceDeleteResponse struct {
	serviceName string
	err         error
}

type serviceCreateResponse struct {
	service *api.Service
	err     error
}

func (s *executor) setupResources() error {
	var err error

	if s.buildLimits, err = limits(s.configurationOverwrites.cpuLimit, s.configurationOverwrites.memoryLimit); err != nil {
		return fmt.Errorf("invalid build limits specified: %w", err)
	}

	if s.buildRequests, err = limits(s.configurationOverwrites.cpuRequest, s.configurationOverwrites.memoryRequest); err != nil {
		return fmt.Errorf("invalid build requests specified: %w", err)
	}

	if s.serviceLimits, err = limits(s.Config.Kubernetes.ServiceCPULimit, s.Config.Kubernetes.ServiceMemoryLimit); err != nil {
		return fmt.Errorf("invalid service limits specified: %w", err)
	}

	if s.serviceRequests, err = limits(s.Config.Kubernetes.ServiceCPURequest, s.Config.Kubernetes.ServiceMemoryRequest); err != nil {
		return fmt.Errorf("invalid service requests specified: %w", err)
	}

	if s.helperLimits, err = limits(s.Config.Kubernetes.HelperCPULimit, s.Config.Kubernetes.HelperMemoryLimit); err != nil {
		return fmt.Errorf("invalid helper limits specified: %w", err)
	}

	if s.helperRequests, err = limits(s.Config.Kubernetes.HelperCPURequest, s.Config.Kubernetes.HelperMemoryRequest); err != nil {
		return fmt.Errorf("invalid helper requests specified: %w", err)
	}

	return nil
}

func (s *executor) Prepare(options common.ExecutorPrepareOptions) (err error) {
	if err = s.AbstractExecutor.Prepare(options); err != nil {
		return fmt.Errorf("prepare AbstractExecutor: %w", err)
	}

	if s.BuildShell.PassFile {
		return fmt.Errorf("kubernetes doesn't support shells that require script file")
	}

	if err = s.prepareOverwrites(options.Build.Variables); err != nil {
		return fmt.Errorf("couldn't prepare overwrites: %w", err)
	}

	if err = s.setupResources(); err != nil {
		return fmt.Errorf("couldn't setup Kubernetes resources: %w", err)

	}

	if s.pullPolicy, err = s.Config.Kubernetes.PullPolicy.Get(); err != nil {
		return fmt.Errorf("couldn't get pull policy: %w", err)
	}

	s.prepareOptions(options.Build)

	if err = s.checkDefaults(); err != nil {
		return fmt.Errorf("check defaults error: %w", err)
	}

	if s.kubeClient, err = getKubeClient(options.Config.Kubernetes, s.configurationOverwrites); err != nil {
		return fmt.Errorf("error connecting to Kubernetes: %w", err)
	}

	s.featureChecker = &kubeClientFeatureChecker{kubeClient: s.kubeClient}

	s.Println("Using Kubernetes executor with image", s.options.Image.Name, "...")
	if !s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) {
		s.Println("Using attach strategy to execute scripts...")
	}

	return nil
}

func (s *executor) Run(cmd common.ExecutorCommand) error {
	if s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) {
		s.Debugln("Starting Kubernetes command...")
		return s.runWithExecLegacy(cmd)
	}

	s.Debugln("Starting Kubernetes command with attach...")
	return s.runWithAttach(cmd)
}

func (s *executor) runWithExecLegacy(cmd common.ExecutorCommand) error {
	if s.pod == nil {
		err := s.setupCredentials()
		if err != nil {
			return err
		}

		err = s.setupBuildPod()
		if err != nil {
			return err
		}
	}

	containerName := buildContainerName
	containerCommand := s.BuildShell.DockerCommand
	if cmd.Predefined {
		containerName = helperContainerName
		containerCommand = s.helperImageInfo.Cmd
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Debugln(fmt.Sprintf(
		"Starting in container %q the command %q with script: %s",
		containerName,
		containerCommand,
		cmd.Script,
	))

	select {
	case err := <-s.runInContainerWithExecLegacy(ctx, containerName, containerCommand, cmd.Script):
		s.Debugln(fmt.Sprintf("Container %q exited with error: %v", containerName, err))
		if err != nil && strings.Contains(err.Error(), "command terminated with exit code") {
			return &common.BuildError{Inner: err}
		}
		return err

	case <-cmd.Context.Done():
		return fmt.Errorf("build aborted")
	}
}

func (s *executor) runWithAttach(cmd common.ExecutorCommand) error {
	err := s.ensurePodsConfigured(cmd.Context)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(cmd.Context)
	defer cancel()

	containerName := buildContainerName
	// Translates to roughly "sh /detect/shell/path.sh /stage/script/path.sh"
	// which when the detect shell exits becomes something like "bash /stage/script/path.sh".
	// This works unlike "gitlab-runner-build" since the detect shell passes arguments with "$@"
	containerCommand := []string{"sh", s.scriptPath(detectShellScriptName), s.scriptPath(cmd.Stage)}
	if cmd.Predefined {
		containerName = helperContainerName
		// We use redirection here since the "gitlab-runner-build" helper doesn't pass input args
		// to the shell it executes, so we technically pass the script to the stdin of the underlying shell
		// translates roughly to "gitlab-runner-build <<< /stage/script/path.sh"
		containerCommand = append(s.helperImageInfo.Cmd, "<<<", s.scriptPath(cmd.Stage))
	}

	s.Debugln(fmt.Sprintf(
		"Starting in container %q the command %q with script: %s",
		containerName,
		containerCommand,
		cmd.Script,
	))

	podStatusCh := s.watchPodStatus(ctx)

	select {
	case err := <-s.runInContainer(containerName, containerCommand):
		s.Debugln(fmt.Sprintf("Container %q exited with error: %v", containerName, err))
		if err != nil && errors.Is(err, new(commandTerminatedError)) {
			return &common.BuildError{Inner: err}
		}

		return err
	case err := <-podStatusCh:
		return &common.BuildError{Inner: err}
	case <-ctx.Done():
		return fmt.Errorf("build aborted")
	}
}

func (s *executor) ensurePodsConfigured(ctx context.Context) error {
	if s.pod != nil {
		return nil
	}

	err := s.setupCredentials()
	if err != nil {
		return fmt.Errorf("setting up credentials: %w", err)
	}

	err = s.setupScriptsConfigMap()
	if err != nil {
		return fmt.Errorf("setting up scripts configMap: %w", err)
	}

	err = s.setupBuildPod()
	if err != nil {
		return fmt.Errorf("setting up build pod: %w", err)
	}

	status, err := waitForPodRunning(ctx, s.kubeClient, s.pod, s.Trace, s.Config.Kubernetes)
	if err != nil {
		return fmt.Errorf("waiting for pod running: %w", err)
	}

	if status != api.PodRunning {
		return fmt.Errorf("pod failed to enter running state: %s", status)
	}

	go s.processLogs(ctx)

	return nil
}

func (s *executor) processLogs(ctx context.Context) {
	processor := newLogProcessor(
		s.kubeClient,
		backoff.Backoff{Min: time.Second, Max: 30 * time.Second},
		&s.BuildLogger,
		kubernetesLogProcessorPodConfig{
			namespace:  s.pod.Namespace,
			pod:        s.pod.Name,
			containers: []string{helperContainerName, buildContainerName},
		},
	)

	logs := processor.Listen(ctx)
	for line := range logs {
		var status shells.TrapCommandExitStatus
		if status.TryUnmarshal(line) {
			s.remoteProcessTerminated <- status
			continue
		}

		_, err := s.Trace.Write([]byte(line + "\n"))
		if err != nil {
			s.Warningln(fmt.Sprintf("Error writing log line to trace: %v", err))
		}
	}
}

func (s *executor) setupScriptsConfigMap() error {
	s.Debugln("Setting up scripts config map")

	scripts := map[string]string{}
	scripts[detectShellScriptName] = detectShellScript

	// After issue https://gitlab.com/gitlab-org/gitlab-runner/issues/10342 is resolved and the legacy execution mode is removed
	// we can remove the manual construction of trapShell and just use "bash+trap"
	// in the exec options
	bashShell, ok := common.GetShell(s.Shell().Shell).(*shells.BashShell)
	if !ok {
		return fmt.Errorf("kubernetes executor incorrect shell type")
	}

	trapShell := &shells.BashTrapShell{BashShell: bashShell}
	for _, stage := range common.BuildStages {
		script, err := trapShell.GenerateScript(stage, *s.Shell())
		if err != nil {
			return fmt.Errorf("generating trap shell script: %w", err)
		}
		scripts[string(stage)] = script
	}

	configMap := &api.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-scripts", s.Build.ProjectUniqueName()),
			Namespace:    s.configurationOverwrites.namespace,
		},
		Data: scripts,
	}

	var err error
	s.configMap, err = s.kubeClient.CoreV1().ConfigMaps(s.configurationOverwrites.namespace).Create(configMap)
	if err != nil {
		return fmt.Errorf("generating scripts config map: %w", err)
	}

	return nil
}

func (s *executor) Cleanup() {
	s.cleanupResources()
	s.cleanupServices()
	closeKubeClient(s.kubeClient)
	s.AbstractExecutor.Cleanup()
}

func (s *executor) cleanupServices() {
	ch := make(chan serviceDeleteResponse)
	var wg sync.WaitGroup
	wg.Add(len(s.services))

	for _, service := range s.services {
		go s.deleteKubernetesService(service.ObjectMeta.Name, ch, &wg)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	for res := range ch {
		if res.err != nil {
			s.Errorln(fmt.Sprintf("Error cleaning up the pod service %q: %v", res.serviceName, res.err))
		}
	}
}

func (s *executor) deleteKubernetesService(serviceName string, ch chan<- serviceDeleteResponse, wg *sync.WaitGroup) {
	defer wg.Done()

	err := s.kubeClient.CoreV1().Services(s.configurationOverwrites.namespace).Delete(serviceName, &metav1.DeleteOptions{})
	ch <- serviceDeleteResponse{serviceName: serviceName, err: err}
}

func (s *executor) cleanupResources() {
	if s.pod != nil {
		err := s.kubeClient.CoreV1().Pods(s.pod.Namespace).Delete(s.pod.Name, &metav1.DeleteOptions{})
		if err != nil {
			s.Errorln(fmt.Sprintf("Error cleaning up pod: %s", err.Error()))
		}
	}
	if s.credentials != nil {
		err := s.kubeClient.CoreV1().Secrets(s.configurationOverwrites.namespace).Delete(s.credentials.Name, &metav1.DeleteOptions{})
		if err != nil {
			s.Errorln(fmt.Sprintf("Error cleaning up secrets: %s", err.Error()))
		}
	}
	if s.configMap != nil {
		err := s.kubeClient.CoreV1().ConfigMaps(s.configurationOverwrites.namespace).Delete(s.configMap.Name, &metav1.DeleteOptions{})
		if err != nil {
			s.Errorln(fmt.Sprintf("Error cleaning up configmap: %s", err.Error()))
		}
	}
}

func (s *executor) buildContainer(name, image string, imageDefinition common.Image, requests, limits api.ResourceList, containerCommand ...string) api.Container {
	privileged := false
	containerPorts := make([]api.ContainerPort, len(imageDefinition.Ports))
	proxyPorts := make([]proxy.Port, len(imageDefinition.Ports))

	for i, port := range imageDefinition.Ports {
		proxyPorts[i] = proxy.Port{Name: port.Name, Number: port.Number, Protocol: port.Protocol}
		containerPorts[i] = api.ContainerPort{ContainerPort: int32(port.Number)}
	}

	if len(proxyPorts) > 0 {
		serviceName := imageDefinition.Alias

		if serviceName == "" {
			serviceName = name
			if name != buildContainerName {
				serviceName = fmt.Sprintf("proxy-%s", name)
			}
		}

		s.ProxyPool[serviceName] = s.newProxy(serviceName, proxyPorts)
	}

	if s.Config.Kubernetes != nil {
		privileged = s.Config.Kubernetes.Privileged
	}

	command, args := s.getCommandAndArgs(imageDefinition, containerCommand...)

	return api.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: api.PullPolicy(s.pullPolicy),
		Command:         command,
		Args:            args,
		Env:             buildVariables(s.Build.GetAllVariables().PublicOrInternal()),
		Resources: api.ResourceRequirements{
			Limits:   limits,
			Requests: requests,
		},
		Ports:        containerPorts,
		VolumeMounts: s.getVolumeMounts(),
		SecurityContext: &api.SecurityContext{
			Privileged: &privileged,
		},
		Stdin: true,
	}
}

func (s *executor) getCommandAndArgs(imageDefinition common.Image, command ...string) ([]string, []string) {
	if len(command) == 0 && len(imageDefinition.Entrypoint) > 0 {
		command = imageDefinition.Entrypoint
	}

	var args []string
	if len(imageDefinition.Command) > 0 {
		args = imageDefinition.Command
	}

	return command, args
}

func (s *executor) scriptsDir() string {
	return path.Join(s.Build.TmpProjectDir(), "scripts")
}

func (s *executor) scriptPath(stage common.BuildStage) string {
	return path.Join(s.scriptsDir(), string(stage))
}

func (s *executor) getVolumeMounts() []api.VolumeMount {
	var mounts []api.VolumeMount

	mounts = append(mounts, api.VolumeMount{
		Name:      "repo",
		MountPath: s.Build.RootDir,
	})

	// The configMap is nil when using legacy execution
	if s.configMap != nil {
		mounts = append(mounts, api.VolumeMount{
			Name:      "scripts",
			MountPath: s.scriptsDir(),
		})
	}

	mounts = append(mounts, s.getVolumeMountsForConfig()...)

	return mounts
}

func (s *executor) getVolumeMountsForConfig() []api.VolumeMount {
	var mounts []api.VolumeMount

	for _, mount := range s.Config.Kubernetes.Volumes.HostPaths {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: mount.MountPath,
			ReadOnly:  mount.ReadOnly,
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.Secrets {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: mount.MountPath,
			ReadOnly:  mount.ReadOnly,
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.PVCs {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: mount.MountPath,
			ReadOnly:  mount.ReadOnly,
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.ConfigMaps {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: mount.MountPath,
			ReadOnly:  mount.ReadOnly,
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.EmptyDirs {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: mount.MountPath,
		})
	}

	return mounts
}

func (s *executor) getVolumes() []api.Volume {
	volumes := s.getVolumesForConfig()
	volumes = append(volumes, api.Volume{
		Name: "repo",
		VolumeSource: api.VolumeSource{
			EmptyDir: &api.EmptyDirVolumeSource{},
		},
	})

	// The configMap is nil when using legacy execution
	if s.configMap == nil {
		return volumes
	}

	mode := int32(0777)
	optional := false
	volumes = append(volumes, api.Volume{
		Name: "scripts",
		VolumeSource: api.VolumeSource{
			ConfigMap: &api.ConfigMapVolumeSource{
				LocalObjectReference: api.LocalObjectReference{
					Name: s.configMap.Name,
				},
				DefaultMode: &mode,
				Optional:    &optional,
			},
		},
	})

	return volumes
}

func (s *executor) getVolumesForConfig() []api.Volume {
	var volumes []api.Volume

	for _, volume := range s.Config.Kubernetes.Volumes.HostPaths {
		path := volume.HostPath
		// Make backward compatible with syntax introduced in version 9.3.0
		if path == "" {
			path = volume.MountPath
		}

		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{
					Path: path,
				},
			},
		})
	}

	for _, volume := range s.Config.Kubernetes.Volumes.Secrets {
		items := []api.KeyToPath{}
		for key, path := range volume.Items {
			items = append(items, api.KeyToPath{Key: key, Path: path})
		}

		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				Secret: &api.SecretVolumeSource{
					SecretName: volume.Name,
					Items:      items,
				},
			},
		})
	}

	for _, volume := range s.Config.Kubernetes.Volumes.PVCs {
		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				PersistentVolumeClaim: &api.PersistentVolumeClaimVolumeSource{
					ClaimName: volume.Name,
					ReadOnly:  volume.ReadOnly,
				},
			},
		})
	}

	for _, volume := range s.Config.Kubernetes.Volumes.ConfigMaps {
		items := []api.KeyToPath{}
		for key, path := range volume.Items {
			items = append(items, api.KeyToPath{Key: key, Path: path})
		}

		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				ConfigMap: &api.ConfigMapVolumeSource{
					LocalObjectReference: api.LocalObjectReference{
						Name: volume.Name,
					},
					Items: items,
				},
			},
		})
	}

	for _, volume := range s.Config.Kubernetes.Volumes.EmptyDirs {
		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				EmptyDir: &api.EmptyDirVolumeSource{
					Medium: api.StorageMedium(volume.Medium),
				},
			},
		})
	}

	return volumes
}

type dockerConfigEntry struct {
	Username, Password string
}

func (s *executor) setupCredentials() error {
	s.Debugln("Setting up secrets")

	authConfigs := make(map[string]dockerConfigEntry)

	for _, credentials := range s.Build.Credentials {
		if credentials.Type != "registry" {
			continue
		}

		authConfigs[credentials.URL] = dockerConfigEntry{
			Username: credentials.Username,
			Password: credentials.Password,
		}
	}

	if len(authConfigs) == 0 {
		return nil
	}

	dockerCfgContent, err := json.Marshal(authConfigs)
	if err != nil {
		return err
	}

	secret := api.Secret{}
	secret.GenerateName = s.Build.ProjectUniqueName()
	secret.Namespace = s.configurationOverwrites.namespace
	secret.Type = api.SecretTypeDockercfg
	secret.Data = map[string][]byte{}
	secret.Data[api.DockerConfigKey] = dockerCfgContent

	s.credentials, err = s.kubeClient.CoreV1().Secrets(s.configurationOverwrites.namespace).Create(&secret)
	if err != nil {
		return err
	}
	return nil
}

type invalidHostAliasDNSError struct {
	service common.Image
	inner   error
}

func (e *invalidHostAliasDNSError) Error() string {
	return fmt.Sprintf(
		"provided host alias %s for service %s is invalid DNS. %s",
		e.service.Alias,
		e.service.Name,
		e.inner,
	)
}

func (e *invalidHostAliasDNSError) Is(err error) bool {
	_, ok := err.(*invalidHostAliasDNSError)
	return ok
}

func (s *executor) prepareHostAlias() (*api.HostAlias, error) {
	supportsHostAliases, err := s.featureChecker.IsHostAliasSupported()
	if errors.Is(err, &badVersionError{}) {
		s.Warningln("Checking for host alias support. Host aliases will be disabled.", err)
		return nil, nil
	} else if err != nil {
		return nil, err
	} else if !supportsHostAliases {
		return nil, nil
	}

	return s.createHostAlias()
}

func (s *executor) createHostAlias() (*api.HostAlias, error) {
	servicesHostAlias := api.HostAlias{IP: "127.0.0.1"}

	for _, service := range s.options.Services {
		// Services with ports are coming from .gitlab-webide.yml
		// they are used for ports mapping and their aliases are in no way validated
		// so we ignore them. Check out https://gitlab.com/gitlab-org/gitlab-runner/merge_requests/1170
		// for details
		if len(service.Ports) > 0 {
			continue
		}

		serviceMeta := services.SplitNameAndVersion(service.Name)
		for _, alias := range serviceMeta.Aliases {
			// For backward compatibility reasons a non DNS1123 compliant alias might be generated,
			// this will be removed in https://gitlab.com/gitlab-org/gitlab-runner/issues/6100
			err := dns.ValidateDNS1123Subdomain(alias)
			if err == nil {
				servicesHostAlias.Hostnames = append(servicesHostAlias.Hostnames, alias)
			}
		}

		if service.Alias == "" {
			continue
		}

		err := dns.ValidateDNS1123Subdomain(service.Alias)
		if err != nil {
			return nil, &invalidHostAliasDNSError{service: service, inner: err}
		}

		servicesHostAlias.Hostnames = append(servicesHostAlias.Hostnames, service.Alias)
	}

	return &servicesHostAlias, nil
}

func (s *executor) setupBuildPod() error {
	s.Debugln("Setting up build pod")

	services := make([]api.Container, len(s.options.Services))

	for i, service := range s.options.Services {
		resolvedImage := s.Build.GetAllVariables().ExpandValue(service.Name)
		services[i] = s.buildContainer(fmt.Sprintf("svc-%d", i), resolvedImage, service, s.serviceRequests, s.serviceLimits)
	}

	// We set a default label to the pod. This label will be used later
	// by the services, to link each service to the pod
	labels := map[string]string{"pod": s.Build.ProjectUniqueName()}
	for k, v := range s.Build.Runner.Kubernetes.PodLabels {
		labels[k] = s.Build.Variables.ExpandValue(v)
	}

	annotations := make(map[string]string)
	for key, val := range s.configurationOverwrites.podAnnotations {
		annotations[key] = s.Build.Variables.ExpandValue(val)
	}

	var imagePullSecrets []api.LocalObjectReference
	for _, imagePullSecret := range s.Config.Kubernetes.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, api.LocalObjectReference{Name: imagePullSecret})
	}

	if s.credentials != nil {
		imagePullSecrets = append(imagePullSecrets, api.LocalObjectReference{Name: s.credentials.Name})
	}

	hostAlias, err := s.prepareHostAlias()
	if err != nil {
		return err
	}

	podConfig := s.preparePodConfig(labels, annotations, services, imagePullSecrets, hostAlias)

	s.Debugln("Creating build pod")
	pod, err := s.kubeClient.CoreV1().Pods(s.configurationOverwrites.namespace).Create(&podConfig)
	if err != nil {
		return err
	}

	s.pod = pod
	s.services, err = s.makePodProxyServices()
	if err != nil {
		return err
	}

	return nil
}

func (s *executor) preparePodConfig(labels, annotations map[string]string, services []api.Container, imagePullSecrets []api.LocalObjectReference, hostAlias *api.HostAlias) api.Pod {
	buildImage := s.Build.GetAllVariables().ExpandValue(s.options.Image.Name)

	pod := api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: s.Build.ProjectUniqueName(),
			Namespace:    s.configurationOverwrites.namespace,
			Labels:       labels,
			Annotations:  annotations,
		},
		Spec: api.PodSpec{
			Volumes:            s.getVolumes(),
			ServiceAccountName: s.configurationOverwrites.serviceAccount,
			RestartPolicy:      api.RestartPolicyNever,
			NodeSelector:       s.Config.Kubernetes.NodeSelector,
			Tolerations:        s.Config.Kubernetes.GetNodeTolerations(),
			Containers: append([]api.Container{
				// TODO use the build and helper template here
				s.buildContainer(buildContainerName, buildImage, s.options.Image, s.buildRequests, s.buildLimits, s.BuildShell.DockerCommand...),
				s.buildContainer(helperContainerName, s.getHelperImage(), common.Image{}, s.helperRequests, s.helperLimits, s.BuildShell.DockerCommand...),
			}, services...),
			TerminationGracePeriodSeconds: &s.Config.Kubernetes.TerminationGracePeriodSeconds,
			ImagePullSecrets:              imagePullSecrets,
			SecurityContext:               s.Config.Kubernetes.GetPodSecurityContext(),
		},
	}

	if hostAlias != nil {
		pod.Spec.HostAliases = []api.HostAlias{*hostAlias}
	}

	return pod
}

func (s *executor) getHelperImage() string {
	if len(s.Config.Kubernetes.HelperImage) > 0 {
		return common.AppVersion.Variables().ExpandValue(s.Config.Kubernetes.HelperImage)
	}

	return s.helperImageInfo.String()
}

func (s *executor) makePodProxyServices() ([]api.Service, error) {
	s.Debugln("Creating pod proxy services")

	ch := make(chan serviceCreateResponse)
	var wg sync.WaitGroup
	wg.Add(len(s.ProxyPool))

	for serviceName, serviceProxy := range s.ProxyPool {
		serviceName := dns.MakeRFC1123Compatible(serviceName)
		servicePorts := make([]api.ServicePort, len(serviceProxy.Settings.Ports))
		for i, port := range serviceProxy.Settings.Ports {
			// When there is more than one port Kubernetes requires a port name
			portName := fmt.Sprintf("%s-%d", serviceName, port.Number)
			servicePorts[i] = api.ServicePort{Port: int32(port.Number), TargetPort: intstr.FromInt(port.Number), Name: portName}
		}

		serviceConfig := s.prepareServiceConfig(serviceName, servicePorts)
		go s.createKubernetesService(&serviceConfig, serviceProxy.Settings, ch, &wg)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var services []api.Service
	for res := range ch {
		if res.err != nil {
			err := fmt.Errorf("error creating the proxy service %q: %w", res.service.Name, res.err)
			s.Errorln(err)

			return []api.Service{}, err
		}

		services = append(services, *res.service)
	}

	return services, nil
}

func (s *executor) prepareServiceConfig(name string, ports []api.ServicePort) api.Service {
	return api.Service{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: name,
			Namespace:    s.configurationOverwrites.namespace,
		},
		Spec: api.ServiceSpec{
			Ports:    ports,
			Selector: map[string]string{"pod": s.Build.ProjectUniqueName()},
			Type:     api.ServiceTypeClusterIP,
		},
	}
}

func (s *executor) createKubernetesService(service *api.Service, proxySettings *proxy.Settings, ch chan<- serviceCreateResponse, wg *sync.WaitGroup) {
	defer wg.Done()

	service, err := s.kubeClient.CoreV1().Services(s.pod.Namespace).Create(service)
	if err == nil {
		// Updating the internal service name reference and activating the proxy
		proxySettings.ServiceName = service.Name
	}

	ch <- serviceCreateResponse{service: service, err: err}
}

func (s *executor) watchPodStatus(ctx context.Context) <-chan error {
	// Buffer of 1 in case the context is cancelled while the timer tick case is being executed
	// and the consumer is no longer reading from the channel while we try to write to it
	ch := make(chan error, 1)

	go func() {
		defer close(ch)

		t := time.NewTicker(time.Duration(s.Config.Kubernetes.GetPollInterval()) * time.Second)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				err := s.checkPodStatus()
				if err != nil {
					ch <- err
					return
				}
			}
		}
	}()

	return ch
}

func (s *executor) checkPodStatus() error {
	pod, err := s.kubeClient.CoreV1().Pods(s.pod.Namespace).Get(s.pod.Name, metav1.GetOptions{})
	var statusErr *kubeerrors.StatusError
	if errors.As(err, &statusErr) && statusErr.ErrStatus.Code == http.StatusNotFound {
		return err
	}

	if err != nil {
		// General request failure
		s.Warningln("Getting job pod status", err)
		return nil
	}

	if pod.Status.Phase != api.PodRunning {
		return &podPhaseError{
			name:  s.pod.Name,
			phase: pod.Status.Phase,
		}
	}

	return nil
}

func (s *executor) runInContainer(name string, command []string) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)

		config, err := getKubeClientConfig(s.Config.Kubernetes, s.configurationOverwrites)
		if err != nil {
			errCh <- err
			return
		}

		attach := AttachOptions{
			PodName:       s.pod.Name,
			Namespace:     s.pod.Namespace,
			ContainerName: name,
			Command:       command,

			Config:   config,
			Client:   s.kubeClient,
			Executor: &DefaultRemoteExecutor{},
		}

		retryable := retry.New(retry.WithBuildLog(&attach, &s.BuildLogger))
		err = retryable.Run()
		if err != nil {
			errCh <- err
		}

		exitStatus := <-s.remoteProcessTerminated
		if *exitStatus.CommandExitCode == 0 {
			errCh <- nil
			return
		}

		errCh <- &commandTerminatedError{exitCode: *exitStatus.CommandExitCode}
	}()

	return errCh
}

func (s *executor) runInContainerWithExecLegacy(ctx context.Context, name string, command []string, script string) <-chan error {
	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)

		status, err := waitForPodRunning(ctx, s.kubeClient, s.pod, s.Trace, s.Config.Kubernetes)

		if err != nil {
			errCh <- err
			return
		}

		if status != api.PodRunning {
			errCh <- fmt.Errorf("pod failed to enter running state: %s", status)
			return
		}

		config, err := getKubeClientConfig(s.Config.Kubernetes, s.configurationOverwrites)

		if err != nil {
			errCh <- err
			return
		}

		exec := ExecOptions{
			PodName:       s.pod.Name,
			Namespace:     s.pod.Namespace,
			ContainerName: name,
			Command:       command,
			In:            strings.NewReader(script),
			Out:           s.Trace,
			Err:           s.Trace,
			Stdin:         true,
			Config:        config,
			Client:        s.kubeClient,
			Executor:      &DefaultRemoteExecutor{},
		}

		retryable := retry.New(retry.WithBuildLog(&exec, &s.BuildLogger))
		errCh <- retryable.Run()
	}()

	return errCh
}

func (s *executor) prepareOverwrites(variables common.JobVariables) error {
	values, err := createOverwrites(s.Config.Kubernetes, variables, s.BuildLogger)
	if err != nil {
		return err
	}

	s.configurationOverwrites = values
	return nil
}

func (s *executor) prepareOptions(build *common.Build) {
	s.options = &kubernetesOptions{}
	s.options.Image = build.Image

	s.getServices(build)
}

func (s *executor) getServices(build *common.Build) {
	for _, service := range s.Config.Kubernetes.Services {
		if service.Name == "" {
			continue
		}
		s.options.Services = append(s.options.Services, service.ToImageDefinition())
	}

	for _, service := range build.Services {
		if service.Name == "" {
			continue
		}
		s.options.Services = append(s.options.Services, service)
	}
}

// checkDefaults Defines the configuration for the Pod on Kubernetes
func (s *executor) checkDefaults() error {
	if s.options.Image.Name == "" {
		if s.Config.Kubernetes.Image == "" {
			return fmt.Errorf("no image specified and no default set in config")
		}

		s.options.Image = common.Image{
			Name: s.Config.Kubernetes.Image,
		}
	}

	if s.configurationOverwrites.namespace == "" {
		s.Warningln("Namespace is empty, therefore assuming 'default'.")
		s.configurationOverwrites.namespace = "default"
	}

	s.Println("Using Kubernetes namespace:", s.configurationOverwrites.namespace)

	return nil
}

func createFn() common.Executor {
	helperImageInfo, err := helperimage.Get(common.REVISION, helperimage.Config{
		OSType:       helperimage.OSTypeLinux,
		Architecture: "amd64",
	})
	if err != nil {
		logrus.WithError(err).Fatal("Failed to set up helper image for kubernetes executor")
	}

	return &executor{
		AbstractExecutor: executors.AbstractExecutor{
			ExecutorOptions: executorOptions,
		},
		helperImageInfo:         helperImageInfo,
		remoteProcessTerminated: make(chan shells.TrapCommandExitStatus),
	}
}

func featuresFn(features *common.FeaturesInfo) {
	features.Variables = true
	features.Image = true
	features.Services = true
	features.Artifacts = true
	features.Cache = true
	features.Session = true
	features.Terminal = true
	features.Proxy = true
}

func init() {
	common.RegisterExecutorProvider("kubernetes", executors.DefaultExecutorProvider{
		Creator:          createFn,
		FeaturesUpdater:  featuresFn,
		DefaultShellName: executorOptions.Shell.Shell,
	})
}
