package kubernetes

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"

	api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/exec"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/retry"
	"gitlab.com/gitlab-org/gitlab-runner/session/proxy"
)

// helperBinaryName is the basename of the gitlab-runner-helper binary
// that the bootstrap init container copies into the shared scripts
// emptyDir, where it is then invoked by the build container's overridden
// command and by the dialer's proxy exec.
const helperBinaryName = "gitlab-runner-helper"

// stepsBootstrapInitContainerName is the source-of-truth name of the
// init container that copies the helper binary into the scripts
// emptyDir. It is referenced both by buildStepsBootstrapInitContainer
// (which creates the container) and by helperImageUpgradeMessage in
// steps.go (which scans for it on failure). Extracting it as a constant
// pins the two sites together; a typo at one would be a compile-time
// signal instead of a silent break in helper-too-old detection.
const stepsBootstrapInitContainerName = "init-steps-bootstrap"

// stepsRunnerBinaryPath returns the absolute path inside the pod at which
// the bootstrapped helper binary lives. The path is shared by:
//   - the init container, which writes it,
//   - the build container, whose Command is overridden to "<path> steps serve",
//   - the dialer's pods/exec invocation, which runs "<path> steps proxy".
func (s *executor) stepsRunnerBinaryPath() string {
	return path.Join(s.scriptsDir(), helperBinaryName)
}

// ensureStepsPod creates the minimal Concrete-mode pod if it does not
// already exist. It is idempotent on s.pod and safe to call from multiple
// Connect attempts on the same executor instance.
func (s *executor) ensureStepsPod(ctx context.Context) error {
	if s.pod != nil {
		return nil
	}

	if err := s.setupBuildNamespace(ctx); err != nil {
		return fmt.Errorf("setting up build namespace: %w", err)
	}

	if err := s.setupCredentials(ctx); err != nil {
		return fmt.Errorf("setting up credentials: %w", err)
	}

	initContainers, err := s.stepsInitContainers()
	if err != nil {
		return err
	}

	if err := s.setupStepsPod(ctx, initContainers); err != nil {
		return fmt.Errorf("setting up build pod: %w", err)
	}

	if s.Build.IsFeatureFlagOn(featureflags.PrintPodEvents) {
		if err := s.handlePodEvents(); err != nil {
			return err
		}
	}

	// When PrintPodEvents is on, handlePodEvents (above) streams events
	// directly to the build log, so out stays as io.Discard here to avoid
	// duplicating those events via waitForPod's internal logger. When off,
	// waitForPod's progress lines are the only signal, so we route them
	// to stderr.
	var out io.WriteCloser = buildlogger.NewNopCloser(io.Discard)
	if !s.Build.IsFeatureFlagOn(featureflags.PrintPodEvents) {
		out = s.BuildLogger.Stream(buildlogger.StreamExecutorLevel, buildlogger.Stderr)
	}
	defer out.Close()

	if err := s.waitForPod(ctx, out); err != nil {
		return err
	}
	// Flush and close the executor stream now that pod startup is done,
	// before the next phase runs — mirroring the Connect path. The defer
	// above still covers the early-return error paths.
	out.Close()

	s.captureServiceContainersLogs(ctx, s.pod.Spec.Containers)

	return nil
}

// stepsInitContainers returns the init containers for the Concrete pod.
// Concrete pods need only the bootstrap init container — no permissions
// or logs init containers, which are part of the non-steps pod shape.
func (s *executor) stepsInitContainers() ([]api.Container, error) {
	bootstrap, err := s.buildStepsBootstrapInitContainer()
	if err != nil {
		return nil, fmt.Errorf("building steps bootstrap init container: %w", err)
	}

	return []api.Container{bootstrap}, nil
}

// buildStepsBootstrapInitContainer constructs the init container that
// copies the helper binary into the shared scripts emptyDir.
//
// The container sets TerminationMessagePolicy: FallbackToLogsOnError so
// that on a non-zero exit Kubernetes copies the last bytes of stderr into
// ContainerStateTerminated.Message. This is what the Connect path
// inspects to detect a helper image that predates the `steps` subcommand
// and surface the friendly upgrade error.
func (s *executor) buildStepsBootstrapInitContainer() (api.Container, error) {
	pullPolicy, err := s.pullManager.GetPullPolicyFor(stepsBootstrapInitContainerName)
	if err != nil {
		return api.Container{}, fmt.Errorf("getting pull policy for steps bootstrap: %w", err)
	}

	return api.Container{
		Name:            stepsBootstrapInitContainerName,
		Image:           s.getHelperImage(),
		ImagePullPolicy: pullPolicy,
		// Image is the helper image, which has gitlab-runner-helper on
		// $PATH, so the bare basename suffices here. The build container
		// (which runs the user's image) cannot make the same assumption
		// and must use the absolute path of the bootstrapped binary in
		// the scripts emptyDir, hence the s.stepsRunnerBinaryPath() arg.
		Command: []string{
			helperBinaryName, "steps", "bootstrap", s.stepsRunnerBinaryPath(),
		},
		VolumeMounts: s.stepsVolumeMounts(),
		Resources:    s.initContainerResources(),
		SecurityContext: s.Config.Kubernetes.GetContainerSecurityContext(
			s.Config.Kubernetes.InitPermissionsContainerSecurityContext,
			s.defaultCapDrop()...,
		),
		TerminationMessagePolicy: api.TerminationMessageFallbackToLogsOnError,
	}, nil
}

// setupStepsPod creates the pod spec for Concrete mode and submits it to
// the kube API. It is the steps-specific equivalent of setupBuildPod.
func (s *executor) setupStepsPod(ctx context.Context, initContainers []api.Container) error {
	s.BuildLogger.Debugln("Setting up steps pod")

	// Concrete assembles its own service containers and reuses only the
	// mode-neutral pod metadata helpers. It deliberately does NOT call
	// createPodConfigPrepareOpts, whose preparePodServices builds service
	// containers via the legacy buildContainer (which mounts the logs
	// emptyDir Concrete pods do not have).
	services, err := s.stepsServiceContainers()
	if err != nil {
		return err
	}

	hostAliases, err := s.getHostAliases()
	if err != nil {
		return err
	}

	prepareOpts := podConfigPrepareOpts{
		labels:           s.buildLabels(),
		annotations:      s.buildPodAnnotations(),
		services:         services,
		initContainers:   initContainers,
		imagePullSecrets: s.prepareImagePullSecrets(),
		hostAliases:      hostAliases,
	}

	podConfig, err := s.stepsPreparePodConfig(prepareOpts)
	if err != nil {
		return err
	}

	s.BuildLogger.Debugln("Checking for ImagePullSecrets or ServiceAccount existence")
	if err := s.checkDependantResources(ctx); err != nil {
		return err
	}

	if s.Build.IsFeatureFlagOn(featureflags.UseAdvancedPodSpecConfiguration) {
		s.BuildLogger.Warningln(
			"Advanced Pod Spec configuration enabled, " +
				"merging the provided PodSpec to the generated one.")
		podConfig.Spec, err = s.applyPodSpecMerge(&podConfig.Spec)
		if err != nil {
			return err
		}
	}

	s.podWatcher.UpdatePodName(podConfig.GetName())

	s.BuildLogger.Debugln("Creating steps pod")

	s.pod, err = retry.WithValueFn(s, func() (*api.Pod, error) {
		return s.requestPodCreation(ctx, &podConfig, s.configurationOverwrites.namespace)
	}).Run()
	if err != nil {
		return err
	}

	if data, ok := s.Build.ExecutorData.(*executorData); ok {
		data.PodName = s.pod.GetName()
	}

	ownerReferences := s.buildPodReferences()
	if err := s.setOwnerReferencesForResources(ctx, ownerReferences); err != nil {
		return fmt.Errorf("error setting ownerReferences: %w", err)
	}

	// Mirror setupBuildPod: if the operator has enabled PDB protection
	// for K8s jobs, Concrete-mode pods must opt in the same way.
	// Otherwise a deliberately-configured operator feature would be
	// silently dropped for native-steps jobs.
	if s.Config.Kubernetes.GetPodDisruptionBudget() {
		s.podDisruptionBudget, err = s.createPodDisruptionBudget(ctx, ownerReferences)
		if err != nil {
			return fmt.Errorf("error creating PodDisruptionBudget: %w", err)
		}
	}

	s.services, err = s.makePodProxyServices(ctx, ownerReferences)
	return err
}

// stepsPreparePodConfig assembles the Pod spec for Concrete mode. It is
// the steps-specific equivalent of preparePodConfig. The shape is
// intentionally minimal:
//   - one build container (running `steps serve`),
//   - user-declared service containers,
//   - one init container (init-steps-bootstrap),
//   - the scripts emptyDir, optionally the default builds-dir emptyDir,
//     plus any user-configured volumes.
//
// In particular there is NO helper container and NO logs emptyDir.
func (s *executor) stepsPreparePodConfig(opts podConfigPrepareOpts) (api.Pod, error) {
	buildContainer, err := s.stepsBuildContainer()
	if err != nil {
		return api.Pod{}, err
	}

	containers := append([]api.Container{buildContainer}, opts.services...)

	return api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        generateNameForK8sResources(s.Build.ProjectUniqueName()),
			Namespace:   s.configurationOverwrites.namespace,
			Labels:      opts.labels,
			Annotations: opts.annotations,
		},
		Spec: api.PodSpec{
			Volumes:                       s.stepsVolumes(),
			SchedulerName:                 s.Config.Kubernetes.SchedulerName,
			ServiceAccountName:            s.configurationOverwrites.serviceAccount,
			AutomountServiceAccountToken:  s.Config.Kubernetes.AutomountServiceAccountToken,
			RestartPolicy:                 api.RestartPolicyNever,
			NodeSelector:                  s.configurationOverwrites.nodeSelector,
			Tolerations:                   s.getPodTolerations(),
			InitContainers:                opts.initContainers,
			Containers:                    containers,
			TerminationGracePeriodSeconds: s.Config.Kubernetes.PodTerminationGracePeriodSeconds,
			ActiveDeadlineSeconds:         s.getPodActiveDeadlineSeconds(),
			ImagePullSecrets:              opts.imagePullSecrets,
			SecurityContext:               s.Config.Kubernetes.GetPodSecurityContext(),
			HostAliases:                   opts.hostAliases,
			Affinity:                      s.Config.Kubernetes.GetAffinity(),
			DNSPolicy:                     s.getDNSPolicy(),
			DNSConfig:                     s.Config.Kubernetes.GetDNSConfig(),
			RuntimeClassName:              s.Config.Kubernetes.RuntimeClassName,
			PriorityClassName:             s.Config.Kubernetes.PriorityClassName,
			Resources:                     s.podResourcesReference(),
		},
	}, nil
}

// stepsBuildContainer constructs the build container for Concrete mode.
//
// Env is set to nil because step-runner injects environment variables
// into the user's command process tree over the bidirectional stdin/stdout
// protocol. This has no effect on service containers, which get their env
// from getServiceVariables via buildContainer(..., isServiceContainer: true).
func (s *executor) stepsBuildContainer() (api.Container, error) {
	kubernetesOptions := s.options.Image.ExecutorOptions.Kubernetes.Expand(
		s.Build.GetAllVariables())
	securityContext := s.getSecurityContextWithUIDGID(
		string(kubernetesOptions.User),
		buildContainerName,
		s.Config.Kubernetes.BuildContainerSecurityContext,
	)

	if err := s.verifyAllowedImages(containerBuildOpts{
		name:            buildContainerName,
		image:           s.options.Image.Name,
		imageDefinition: s.options.Image,
	}); err != nil {
		return api.Container{}, err
	}

	pullPolicy, err := s.pullManager.GetPullPolicyFor(buildContainerName)
	if err != nil {
		return api.Container{}, err
	}

	// The user image's own Entrypoint/Cmd are intentionally overridden:
	// the build container's purpose in Concrete mode is to host the
	// step-runner serve process, not to run the image's default command.
	command := []string{s.stepsRunnerBinaryPath(), "steps", "serve"}
	command = append(command, s.BuildShell.DockerCommand...)

	containerPorts := make([]api.ContainerPort, len(s.options.Image.Ports))
	proxyPorts := make([]proxy.Port, len(s.options.Image.Ports))
	for i, port := range s.options.Image.Ports {
		proxyPorts[i] = proxy.Port{Name: port.Name, Number: port.Number, Protocol: port.Protocol}
		containerPorts[i] = api.ContainerPort{ContainerPort: int32(port.Number)}
	}

	// Register session proxies for the build container's declared ports,
	// mirroring the standard buildContainer path so attach/legacy port
	// proxying is preserved in Concrete mode. setupStepsPod consumes the
	// ProxyPool via makePodProxyServices. The build container's name is
	// always buildContainerName, so the alias fallback is that name.
	if len(proxyPorts) > 0 {
		aliases := s.options.Image.Aliases()
		if len(aliases) == 0 {
			aliases = []string{buildContainerName}
		}
		for _, serviceName := range aliases {
			s.ProxyPool[serviceName] = s.newProxy(serviceName, proxyPorts)
		}
	}

	return api.Container{
		Name:            buildContainerName,
		Image:           s.options.Image.Name,
		ImagePullPolicy: pullPolicy,
		Command:         command,
		Env:             nil,
		Resources: api.ResourceRequirements{
			Limits:   s.configurationOverwrites.buildLimits,
			Requests: s.configurationOverwrites.buildRequests,
		},
		Ports:           containerPorts,
		VolumeMounts:    s.stepsVolumeMounts(),
		SecurityContext: securityContext,
		Lifecycle:       s.prepareLifecycleHooks(),
		Stdin:           true,
	}, nil
}

// stepsServiceContainers builds the user-declared service containers for
// a Concrete pod. It deliberately duplicates the service-relevant parts
// of buildContainer rather than calling it, so Concrete never depends on
// the legacy container builder (whose getVolumeMounts references the logs
// emptyDir Concrete pods do not have). Mode-neutral helpers (service env,
// image allow-listing, proxy registration, command/args) are still reused.
func (s *executor) stepsServiceContainers() ([]api.Container, error) {
	services := make([]api.Container, len(s.options.Services))

	for i, name := range s.options.getSortedServiceNames() {
		service := s.options.Services[name]
		kubernetesOptions := service.ExecutorOptions.Kubernetes.Expand(s.Build.GetAllVariables())
		securityContext := s.getSecurityContextWithUIDGID(
			string(kubernetesOptions.User),
			name,
			s.Config.Kubernetes.ServiceContainerSecurityContext,
		)

		c, err := s.stepsServiceContainer(name, *service, securityContext)
		if err != nil {
			return nil, err
		}
		services[i] = c
	}

	return services, nil
}

// stepsServiceContainer builds a single Concrete service container. It
// mirrors buildContainer(isServiceContainer: true) except that it mounts
// stepsVolumeMounts (no logs emptyDir) instead of getVolumeMounts.
func (s *executor) stepsServiceContainer(
	name string,
	service spec.Image,
	securityContext *api.SecurityContext,
) (api.Container, error) {
	if err := s.verifyAllowedImages(containerBuildOpts{
		name:               name,
		image:              service.Name,
		imageDefinition:    service,
		isServiceContainer: true,
	}); err != nil {
		return api.Container{}, err
	}

	containerPorts := make([]api.ContainerPort, len(service.Ports))
	proxyPorts := make([]proxy.Port, len(service.Ports))
	for i, port := range service.Ports {
		proxyPorts[i] = proxy.Port{Name: port.Name, Number: port.Number, Protocol: port.Protocol}
		containerPorts[i] = api.ContainerPort{ContainerPort: int32(port.Number)}
	}

	if len(proxyPorts) > 0 {
		aliases := service.Aliases()
		if len(aliases) == 0 {
			aliases = []string{fmt.Sprintf("proxy-%s", name)}
		}
		for _, alias := range aliases {
			s.ProxyPool[alias] = s.newProxy(alias, proxyPorts)
		}
	}

	pullPolicy, err := s.pullManager.GetPullPolicyFor(name)
	if err != nil {
		return api.Container{}, err
	}

	command, args := s.getCommandAndArgs(service)

	return api.Container{
		Name:            name,
		Image:           service.Name,
		ImagePullPolicy: pullPolicy,
		Command:         command,
		Args:            args,
		Env:             buildVariables(s.getServiceVariables(service)),
		Resources: api.ResourceRequirements{
			Limits:   s.configurationOverwrites.getServiceResourceLimits(name),
			Requests: s.configurationOverwrites.getServiceResourceRequests(name),
		},
		Ports:           containerPorts,
		VolumeMounts:    s.stepsVolumeMounts(),
		SecurityContext: securityContext,
		Lifecycle:       s.prepareLifecycleHooks(),
		Stdin:           true,
	}, nil
}

// stepsVolumes returns the pod-level volumes for Concrete mode: the
// scripts emptyDir (always), the default builds-dir emptyDir (when
// needed), and any user-configured volumes. No logs emptyDir is
// emitted — that volume exists for the non-steps helper container, which
// Concrete mode omits.
func (s *executor) stepsVolumes() []api.Volume {
	volumes := s.getVolumesForConfig()

	if s.isDefaultBuildsDirVolumeRequired() {
		volumes = append(volumes, api.Volume{
			Name: "repo",
			VolumeSource: api.VolumeSource{
				EmptyDir: &api.EmptyDirVolumeSource{},
			},
		})
	}

	volumes = append(volumes, api.Volume{
		Name: "scripts",
		VolumeSource: api.VolumeSource{
			EmptyDir: &api.EmptyDirVolumeSource{},
		},
	})

	if s.isDefaultCacheDirVolumeRequired() {
		volumes = append(volumes, api.Volume{
			Name: "cache",
			VolumeSource: api.VolumeSource{
				EmptyDir: &api.EmptyDirVolumeSource{},
			},
		})
	}

	return volumes
}

// stepsWaitForServices is the Concrete-owned equivalent of
// waitForServices. It probes the TCP health-check ports of user services
// from inside the build container, where the bootstrapped helper binary
// lives (Concrete pods have no helper container). It is intentionally a
// self-contained fork of the legacy path so removing legacy later is a
// deletion rather than conditional surgery.
func (s *executor) stepsWaitForServices(ctx context.Context) error {
	portArgs := ""
	for _, name := range s.options.getSortedServiceNames() {
		service := s.options.Services[name]
		port := service.Variables.Get("HEALTHCHECK_TCP_PORT")
		if port == "" {
			continue
		}
		portArgs += fmt.Sprintf("--port '%s' ", port)
	}
	if portArgs == "" {
		return nil
	}

	if err := s.ensureStepsPod(ctx); err != nil {
		return err
	}

	command := s.stepsRunnerBinaryPath() + " health-check " + portArgs

	podStatusCh := s.watchPodStatus(ctx, &podContainerStatusChecker{})

	stdout, stderr := s.getExecutorIoWriters()
	defer stdout.Close()
	defer stderr.Close()

	select {
	case err := <-s.runInContainerWithExec(ctx, buildContainerName, s.BuildShell.DockerCommand, command, stdout, stderr):
		s.BuildLogger.Debugln(fmt.Sprintf("Container helper exited with error: %v", err))
		var exitError exec.CodeExitError
		if err != nil && errors.As(err, &exitError) {
			return &common.BuildError{Inner: err, ExitCode: common.NormalizeExitCode(exitError.ExitStatus())}
		}
	case err := <-podStatusCh:
		s.BuildLogger.Println("Health check aborted due to error: ", err.Error())
		return err
	case <-ctx.Done():
		return fmt.Errorf("health check aborted")
	}

	return nil
}

// stepsVolumeMounts returns the volume mounts shared by the bootstrap
// init container and the build container in Concrete mode.
func (s *executor) stepsVolumeMounts() []api.VolumeMount {
	mounts := []api.VolumeMount{
		{
			Name:      "scripts",
			MountPath: s.scriptsDir(),
		},
	}

	mounts = append(mounts, s.getVolumeMountsForConfig()...)

	if s.isDefaultBuildsDirVolumeRequired() {
		mounts = append(mounts, api.VolumeMount{
			Name:      "repo",
			MountPath: s.AbstractExecutor.RootDir(),
		})
	}

	if s.isDefaultCacheDirVolumeRequired() {
		mounts = append(mounts, api.VolumeMount{
			Name:      "cache",
			MountPath: s.AbstractExecutor.CacheDir(),
		})
	}

	return mounts
}

// isDefaultCacheDirVolumeRequired reports whether the default cache
// directory must be backed by an emptyDir. Under Concrete dispatch the
// cache archive runs inside the build container as the configured
// RunAsUser/RunAsGroup, so the cache directory must sit on a writable
// volume: the pod rootfs is root-owned and a non-root build container
// cannot write the cache archive there. Skipped when a user-configured
// volume already covers the cache directory.
func (s *executor) isDefaultCacheDirVolumeRequired() bool {
	cacheDir := s.AbstractExecutor.CacheDir()
	for _, mount := range s.getVolumeMountsForConfig() {
		if mount.MountPath == cacheDir {
			return false
		}
	}

	return true
}
