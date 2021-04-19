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

	"github.com/docker/cli/cli/config/types"
	"github.com/jpillora/backoff"
	"golang.org/x/net/context"
	api "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // Register all available authentication methods
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/util/exec"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes/internal/pull"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/helperimage"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/dns"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/auth"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/retry"
	"gitlab.com/gitlab-org/gitlab-runner/session/proxy"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

const (
	buildContainerName  = "build"
	helperContainerName = "helper"

	detectShellScriptName = "detect_shell_script"

	waitLogFileTimeout = time.Minute
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

	detectShellScript = shells.BashDetectShellScript
)

// GetDefaultCapDrop returns the default capabilities that should be dropped
// from a build container.
func GetDefaultCapDrop() []string {
	return []string{
		// Reasons for disabling NET_RAW by default were
		// discussed in https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26833
		"NET_RAW",
	}
}

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
	kubeConfig  *restclient.Config
	pod         *api.Pod
	configMap   *api.ConfigMap
	credentials *api.Secret
	options     *kubernetesOptions
	services    []api.Service

	configurationOverwrites *overwrites
	pullManager             pull.Manager

	helperImageInfo helperimage.Info

	featureChecker featureChecker

	newLogProcessor func() logProcessor

	remoteProcessTerminated chan shells.TrapCommandExitStatus
}

type serviceDeleteResponse struct {
	serviceName string
	err         error
}

type serviceCreateResponse struct {
	service *api.Service
	err     error
}

func (s *executor) Prepare(options common.ExecutorPrepareOptions) (err error) {
	if err = s.AbstractExecutor.Prepare(options); err != nil {
		return fmt.Errorf("prepare AbstractExecutor: %w", err)
	}

	if s.BuildShell.PassFile {
		return fmt.Errorf("kubernetes doesn't support shells that require script file")
	}

	if err = s.prepareOverwrites(options.Build.GetAllVariables()); err != nil {
		return fmt.Errorf("couldn't prepare overwrites: %w", err)
	}

	var pullPolicies []api.PullPolicy
	if pullPolicies, err = s.Config.Kubernetes.GetPullPolicies(); err != nil {
		return fmt.Errorf("couldn't get pull policy: %w", err)
	}
	s.pullManager = pull.NewPullManager(pullPolicies, &s.BuildLogger)

	s.prepareOptions(options.Build)

	if err = s.checkDefaults(); err != nil {
		return fmt.Errorf("check defaults error: %w", err)
	}

	s.kubeConfig, err = getKubeClientConfig(s.Config.Kubernetes, s.configurationOverwrites)
	if err != nil {
		return fmt.Errorf("getting Kubernetes config: %w", err)
	}

	s.kubeClient, err = kubernetes.NewForConfig(s.kubeConfig)
	if err != nil {
		return fmt.Errorf("connecting to Kubernetes: %w", err)
	}

	s.helperImageInfo, err = s.prepareHelperImage()
	if err != nil {
		return fmt.Errorf("prepare helper image: %w", err)
	}

	s.featureChecker = &kubeClientFeatureChecker{kubeClient: s.kubeClient}

	imageName := s.Build.GetAllVariables().ExpandValue(s.options.Image.Name)

	s.Println("Using Kubernetes executor with image", imageName, "...")
	if !s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) {
		s.Println("Using attach strategy to execute scripts...")
	}

	return nil
}

func (s *executor) prepareHelperImage() (helperimage.Info, error) {
	return helperimage.Get(common.REVISION, helperimage.Config{
		OSType:         helperimage.OSTypeLinux,
		Architecture:   "amd64",
		GitLabRegistry: s.Build.IsFeatureFlagOn(featureflags.GitLabRegistryHelperImage),
		Shell:          s.Config.Shell,
	})
}

func (s *executor) Run(cmd common.ExecutorCommand) error {
	for attempt := 1; ; attempt++ {
		var err error

		if s.Build.IsFeatureFlagOn(featureflags.UseLegacyKubernetesExecutionStrategy) {
			s.Debugln("Starting Kubernetes command...")
			err = s.runWithExecLegacy(cmd)
		} else {
			s.Debugln("Starting Kubernetes command with attach...")
			err = s.runWithAttach(cmd)
		}

		var imagePullErr *pull.ImagePullError
		if errors.As(err, &imagePullErr) {
			if s.pullManager.UpdatePolicyForImage(attempt, imagePullErr) {
				s.cleanupResources()
				s.pod = nil
				continue
			}
		}
		return err
	}
}

func (s *executor) runWithExecLegacy(cmd common.ExecutorCommand) error {
	if s.pod == nil {
		err := s.setupCredentials()
		if err != nil {
			return err
		}

		err = s.setupBuildPod(nil)
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
		var exitError exec.CodeExitError
		if err != nil && errors.As(err, &exitError) {
			return &common.BuildError{Inner: err, ExitCode: exitError.ExitStatus()}
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
	containerCommand := []string{
		"sh",
		s.scriptPath(detectShellScriptName),
		s.buildCommandForStage(cmd.Stage),
	}
	if cmd.Predefined {
		containerName = helperContainerName
		// We use redirection here since the "gitlab-runner-build" helper doesn't pass input args
		// to the shell it executes, so we technically pass the script to the stdin of the underlying shell
		// translates roughly to "gitlab-runner-build <<< /stage/script/path.sh"
		containerCommand = append(
			s.helperImageInfo.Cmd,
			"<<<",
			s.buildCommandForStage(cmd.Stage),
		)
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
		var terminatedError *commandTerminatedError
		if err != nil && errors.As(err, &terminatedError) {
			return &common.BuildError{Inner: err, ExitCode: terminatedError.exitCode}
		}

		return err
	case err := <-podStatusCh:
		if isKubernetesPodNotFoundError(err) {
			return err
		}

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

	permissionsInitContainer, err := s.buildLogPermissionsInitContainer()
	if err != nil {
		return fmt.Errorf("building log permissions init container: %w", err)
	}
	err = s.setupBuildPod([]api.Container{permissionsInitContainer})
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

func (s *executor) buildLogPermissionsInitContainer() (api.Container, error) {
	// We need to create the log file in which all scripts will append their output.
	// The log file is created with the current user. There are 3 different scenarios for the user:
	// 1. The user in all images and containers is root, in that case the chmod is redundant since they
	// will all have permissions to the file.
	// 2. The user of the helper image is root, however the build image's user is not root.
	// In that case we need to allow the build user to write to the log file from inside the
	// build container. That's where the chmod comes into play.
	// 3. No user is root but all containers have the same user ID. In that case create the file.
	// It will have the same user and group owner across all containers. This is the case for Kubernetes
	// where the PodSecurityContext is set manually or for Openshift where each pod has a different user ID.
	// *4. We don't allow setting different user IDs across containers, if that ever becomes the case
	// we might need to try and chown the log file for the group only.
	logFile := s.logFile()
	chmod := fmt.Sprintf("touch %s && (chmod 777 %s || exit 0)", logFile, logFile)

	pullPolicy, err := s.pullManager.GetPullPolicyFor(s.getHelperImage())
	if err != nil {
		return api.Container{}, fmt.Errorf("getting pull policy for log permissions init container: %w", err)
	}

	return api.Container{
		Name:            "init-logs",
		Image:           s.getHelperImage(),
		Command:         []string{"sh", "-c", chmod},
		VolumeMounts:    s.getVolumeMounts(),
		ImagePullPolicy: pullPolicy,
	}, nil
}

func (s *executor) buildCommandForStage(stage common.BuildStage) string {
	return fmt.Sprintf("%s 2>&1 | tee -a %s", s.scriptPath(stage), s.logFile())
}

func (s *executor) processLogs(ctx context.Context) {
	processor := s.newLogProcessor()
	logsCh := processor.Process(ctx)

	for line := range logsCh {
		var status shells.TrapCommandExitStatus
		if status.TryUnmarshal(line) {
			s.remoteProcessTerminated <- status
			continue
		}

		_, err := s.Trace.Write(append([]byte(line), '\n'))
		if err != nil {
			s.Warningln(fmt.Sprintf("Error writing log line to trace: %v", err))
		}
	}
}

func (s *executor) setupScriptsConfigMap() error {
	s.Debugln("Setting up scripts config map")

	// After issue https://gitlab.com/gitlab-org/gitlab-runner/issues/10342 is resolved and
	// the legacy execution mode is removed we can remove the manual construction of trapShell and just use "bash+trap"
	// in the exec options
	bashShell, ok := common.GetShell(s.Shell().Shell).(*shells.BashShell)
	if !ok {
		return fmt.Errorf("kubernetes executor incorrect shell type")
	}

	trapShell := &shells.BashTrapShell{BashShell: bashShell, LogFile: s.logFile()}
	scripts, err := s.generateScripts(trapShell)
	if err != nil {
		return err
	}

	configMap := &api.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-scripts", s.Build.ProjectUniqueName()),
			Namespace:    s.configurationOverwrites.namespace,
		},
		Data: scripts,
	}

	s.configMap, err = s.kubeClient.CoreV1().ConfigMaps(s.configurationOverwrites.namespace).Create(configMap)
	if err != nil {
		return fmt.Errorf("generating scripts config map: %w", err)
	}

	return nil
}

func (s *executor) generateScripts(shell common.Shell) (map[string]string, error) {
	scripts := map[string]string{}
	scripts[detectShellScriptName] = detectShellScript

	for _, stage := range s.Build.BuildStages() {
		script, err := shell.GenerateScript(stage, *s.Shell())
		if errors.Is(err, common.ErrSkipBuildStage) {
			continue
		} else if err != nil {
			return nil, fmt.Errorf("generating trap shell script: %w", err)
		}

		scripts[string(stage)] = script
	}

	return scripts, nil
}

func (s *executor) Finish(err error) {
	if isKubernetesPodNotFoundError(err) {
		// Avoid an additional error message when trying to
		// cleanup a pod that we know no longer exists
		s.pod = nil
	}

	s.AbstractExecutor.Finish(err)
}

func (s *executor) Cleanup() {
	s.cleanupResources()
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

	err := s.kubeClient.CoreV1().
		Services(s.configurationOverwrites.namespace).
		Delete(serviceName, &metav1.DeleteOptions{})
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
		err := s.kubeClient.CoreV1().
			Secrets(s.configurationOverwrites.namespace).
			Delete(s.credentials.Name, &metav1.DeleteOptions{})
		if err != nil {
			s.Errorln(fmt.Sprintf("Error cleaning up secrets: %s", err.Error()))
		}
	}
	if s.configMap != nil {
		err := s.kubeClient.CoreV1().
			ConfigMaps(s.configurationOverwrites.namespace).
			Delete(s.configMap.Name, &metav1.DeleteOptions{})
		if err != nil {
			s.Errorln(fmt.Sprintf("Error cleaning up configmap: %s", err.Error()))
		}
	}

	s.cleanupServices()
}

//nolint:funlen
func (s *executor) buildContainer(
	name, image string,
	imageDefinition common.Image,
	requests, limits api.ResourceList,
	containerCommand ...string,
) (api.Container, error) {
	privileged := false
	var allowPrivilegeEscalation *bool
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
		allowPrivilegeEscalation = s.Config.Kubernetes.AllowPrivilegeEscalation
	}

	pullPolicy, err := s.pullManager.GetPullPolicyFor(image)
	if err != nil {
		return api.Container{}, err
	}

	command, args := s.getCommandAndArgs(imageDefinition, containerCommand...)

	return api.Container{
			Name:            name,
			Image:           image,
			ImagePullPolicy: pullPolicy,
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
				Privileged:               &privileged,
				AllowPrivilegeEscalation: allowPrivilegeEscalation,
				Capabilities: getCapabilities(
					GetDefaultCapDrop(),
					s.Config.Kubernetes.CapAdd,
					s.Config.Kubernetes.CapDrop,
				),
			},
			Stdin: true,
		},
		nil
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

func (s *executor) logFile() string {
	return path.Join(s.logsDir(), "output.log")
}

func (s *executor) logsDir() string {
	return fmt.Sprintf("/logs-%d-%d", s.Build.JobInfo.ProjectID, s.Build.JobResponse.ID)
}

func (s *executor) scriptsDir() string {
	return fmt.Sprintf("/scripts-%d-%d", s.Build.JobInfo.ProjectID, s.Build.JobResponse.ID)
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
		// These volume mounts **MUST NOT** be mounted inside another volume mount.
		// E.g. mounting them inside the "repo" volume mount will cause the whole volume
		// to be owned by root instead of the current user of the image. Something similar
		// is explained here https://github.com/kubernetes/kubernetes/issues/2630#issuecomment-64679120
		// where the first container determines the ownership of a volume. However, it seems like
		// when mounting a volume inside another volume the first container or the first point of contact
		// becomes root, regardless of SecurityContext or Image settings changing the user ID of the container.
		// This causes builds to stop working in environments such as OpenShift where there's no root access
		// resulting in an inability to modify anything inside the parent volume.
		mounts = append(
			mounts,
			api.VolumeMount{
				Name:      "scripts",
				MountPath: s.scriptsDir(),
			},
			api.VolumeMount{
				Name:      "logs",
				MountPath: s.logsDir(),
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
			SubPath:   mount.SubPath,
			ReadOnly:  mount.ReadOnly,
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.Secrets {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: mount.MountPath,
			SubPath:   mount.SubPath,
			ReadOnly:  mount.ReadOnly,
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.PVCs {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: mount.MountPath,
			SubPath:   mount.SubPath,
			ReadOnly:  mount.ReadOnly,
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.ConfigMaps {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: mount.MountPath,
			SubPath:   mount.SubPath,
			ReadOnly:  mount.ReadOnly,
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.EmptyDirs {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: mount.MountPath,
			SubPath:   mount.SubPath,
		})
	}

	for _, mount := range s.Config.Kubernetes.Volumes.CSIs {
		mounts = append(mounts, api.VolumeMount{
			Name:      mount.Name,
			MountPath: mount.MountPath,
			SubPath:   mount.SubPath,
			ReadOnly:  mount.ReadOnly,
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
	volumes = append(
		volumes,
		api.Volume{
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
		},
		api.Volume{
			Name: "logs",
			VolumeSource: api.VolumeSource{
				EmptyDir: &api.EmptyDirVolumeSource{},
			},
		})

	return volumes
}

func (s *executor) getVolumesForConfig() []api.Volume {
	var volumes []api.Volume

	volumes = append(volumes, s.getVolumesForHostPaths()...)
	volumes = append(volumes, s.getVolumesForSecrets()...)
	volumes = append(volumes, s.getVolumesForPVCs()...)
	volumes = append(volumes, s.getVolumesForConfigMaps()...)
	volumes = append(volumes, s.getVolumesForEmptyDirs()...)
	volumes = append(volumes, s.getVolumesForCSIs()...)

	return volumes
}

func (s *executor) getVolumesForHostPaths() []api.Volume {
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

	return volumes
}

func (s *executor) getVolumesForSecrets() []api.Volume {
	var volumes []api.Volume

	for _, volume := range s.Config.Kubernetes.Volumes.Secrets {
		var items []api.KeyToPath
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

	return volumes
}

func (s *executor) getVolumesForPVCs() []api.Volume {
	var volumes []api.Volume

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

	return volumes
}

func (s *executor) getVolumesForConfigMaps() []api.Volume {
	var volumes []api.Volume

	for _, volume := range s.Config.Kubernetes.Volumes.ConfigMaps {
		var items []api.KeyToPath
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

	return volumes
}

func (s *executor) getVolumesForEmptyDirs() []api.Volume {
	var volumes []api.Volume

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

func (s *executor) getVolumesForCSIs() []api.Volume {
	var volumes []api.Volume

	for _, volume := range s.Config.Kubernetes.Volumes.CSIs {
		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				CSI: &api.CSIVolumeSource{
					Driver:           volume.Driver,
					FSType:           &volume.FSType,
					ReadOnly:         &volume.ReadOnly,
					VolumeAttributes: volume.VolumeAttributes,
				},
			},
		})
	}
	return volumes
}

func (s *executor) setupCredentials() error {
	s.Debugln("Setting up secrets")

	authConfigs, err := auth.ResolveConfigs(s.Build.GetDockerAuthConfig(), s.Shell().User, s.Build.Credentials)
	if err != nil {
		return err
	}

	if len(authConfigs) == 0 {
		return nil
	}

	dockerCfgs := make(map[string]types.AuthConfig)
	for registry, registryInfo := range authConfigs {
		dockerCfgs[registry] = registryInfo.AuthConfig
	}

	dockerCfgContent, err := json.Marshal(dockerCfgs)
	if err != nil {
		return err
	}

	secret := api.Secret{}
	secret.GenerateName = s.Build.ProjectUniqueName()
	secret.Namespace = s.configurationOverwrites.namespace
	secret.Type = api.SecretTypeDockercfg
	secret.Data = map[string][]byte{}
	secret.Data[api.DockerConfigKey] = dockerCfgContent

	creds, err := s.kubeClient.CoreV1().Secrets(s.configurationOverwrites.namespace).Create(&secret)
	if err != nil {
		return err
	}

	s.credentials = creds
	return nil
}

func (s *executor) getHostAliases() ([]api.HostAlias, error) {
	supportsHostAliases, err := s.featureChecker.IsHostAliasSupported()
	switch {
	case errors.Is(err, &badVersionError{}):
		s.Warningln("Checking for host alias support. Host aliases will be disabled.", err)
		return nil, nil
	case err != nil:
		return nil, err
	case !supportsHostAliases:
		return nil, nil
	}

	return createHostAliases(s.options.Services, s.Config.Kubernetes.GetHostAliases())
}

//nolint:funlen
func (s *executor) setupBuildPod(initContainers []api.Container) error {
	s.Debugln("Setting up build pod")

	podServices := make([]api.Container, len(s.options.Services))

	for i, service := range s.options.Services {
		resolvedImage := s.Build.GetAllVariables().ExpandValue(service.Name)
		var err error
		podServices[i], err = s.buildContainer(
			fmt.Sprintf("svc-%d", i),
			resolvedImage,
			service,
			s.configurationOverwrites.serviceRequests,
			s.configurationOverwrites.serviceLimits,
		)
		if err != nil {
			return err
		}
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

	hostAliases, err := s.getHostAliases()
	if err != nil {
		return err
	}

	podConfig, err :=
		s.preparePodConfig(labels, annotations, podServices, imagePullSecrets, hostAliases, initContainers)
	if err != nil {
		return err
	}

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

//nolint:funlen
func (s *executor) preparePodConfig(
	labels, annotations map[string]string,
	services []api.Container,
	imagePullSecrets []api.LocalObjectReference,
	hostAliases []api.HostAlias,
	initContainers []api.Container,
) (api.Pod, error) {
	buildImage := s.Build.GetAllVariables().ExpandValue(s.options.Image.Name)

	buildContainer, err := s.buildContainer(
		buildContainerName,
		buildImage,
		s.options.Image,
		s.configurationOverwrites.buildRequests,
		s.configurationOverwrites.buildLimits,
		s.BuildShell.DockerCommand...,
	)
	if err != nil {
		return api.Pod{}, fmt.Errorf("building build container: %w", err)
	}

	helperContainer, err := s.buildContainer(
		helperContainerName,
		s.getHelperImage(),
		common.Image{},
		s.configurationOverwrites.helperRequests,
		s.configurationOverwrites.helperLimits,
		s.BuildShell.DockerCommand...,
	)
	if err != nil {
		return api.Pod{}, fmt.Errorf("building helper container: %w", err)
	}

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
			InitContainers:     initContainers,
			Containers: append([]api.Container{
				buildContainer,
				helperContainer,
			}, services...),
			TerminationGracePeriodSeconds: &s.Config.Kubernetes.TerminationGracePeriodSeconds,
			ImagePullSecrets:              imagePullSecrets,
			SecurityContext:               s.Config.Kubernetes.GetPodSecurityContext(),
			HostAliases:                   hostAliases,
			Affinity:                      s.Config.Kubernetes.GetAffinity(),
			DNSPolicy:                     s.getDNSPolicy(),
			DNSConfig:                     s.Config.Kubernetes.GetDNSConfig(),
		},
	}

	return pod, nil
}

func (s *executor) getDNSPolicy() api.DNSPolicy {
	dnsPolicy, err := s.Config.Kubernetes.DNSPolicy.Get()
	if err != nil {
		s.Warningln(fmt.Sprintf("falling back to cluster's default policy: %v", err))
	}
	return dnsPolicy
}

func (s *executor) getHelperImage() string {
	if len(s.Config.Kubernetes.HelperImage) > 0 {
		return common.AppVersion.Variables().ExpandValue(s.Config.Kubernetes.HelperImage)
	}

	if !s.Build.IsFeatureFlagOn(featureflags.GitLabRegistryHelperImage) {
		s.Warningln(helperimage.DockerHubWarningMessage)
	}

	return s.helperImageInfo.String()
}

func (s *executor) makePodProxyServices() ([]api.Service, error) {
	s.Debugln("Creating pod proxy services")

	ch := make(chan serviceCreateResponse)
	var wg sync.WaitGroup
	wg.Add(len(s.ProxyPool))

	for serviceName, serviceProxy := range s.ProxyPool {
		serviceName = dns.MakeRFC1123Compatible(serviceName)
		servicePorts := make([]api.ServicePort, len(serviceProxy.Settings.Ports))
		for i, port := range serviceProxy.Settings.Ports {
			// When there is more than one port Kubernetes requires a port name
			portName := fmt.Sprintf("%s-%d", serviceName, port.Number)
			servicePorts[i] = api.ServicePort{
				Port:       int32(port.Number),
				TargetPort: intstr.FromInt(port.Number),
				Name:       portName,
			}
		}

		serviceConfig := s.prepareServiceConfig(serviceName, servicePorts)
		go s.createKubernetesService(&serviceConfig, serviceProxy.Settings, ch, &wg)
	}

	go func() {
		wg.Wait()
		close(ch)
	}()

	var proxyServices []api.Service
	for res := range ch {
		if res.err != nil {
			err := fmt.Errorf("error creating the proxy service %q: %w", res.service.Name, res.err)
			s.Errorln(err)

			return []api.Service{}, err
		}

		proxyServices = append(proxyServices, *res.service)
	}

	return proxyServices, nil
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

func (s *executor) createKubernetesService(
	service *api.Service,
	proxySettings *proxy.Settings,
	ch chan<- serviceCreateResponse,
	wg *sync.WaitGroup,
) {
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
	if isKubernetesPodNotFoundError(err) {
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

		attach := AttachOptions{
			PodName:       s.pod.Name,
			Namespace:     s.pod.Namespace,
			ContainerName: name,
			Command:       command,

			Config:   s.kubeConfig,
			Client:   s.kubeClient,
			Executor: &DefaultRemoteExecutor{},
		}

		retryable := retry.New(retry.WithBuildLog(&attach, &s.BuildLogger))
		err := retryable.Run()
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

func (s *executor) runInContainerWithExecLegacy(
	ctx context.Context,
	name string,
	command []string,
	script string,
) <-chan error {
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

		exec := ExecOptions{
			PodName:       s.pod.Name,
			Namespace:     s.pod.Namespace,
			ContainerName: name,
			Command:       command,
			In:            strings.NewReader(script),
			Out:           s.Trace,
			Err:           s.Trace,
			Stdin:         true,
			Config:        s.kubeConfig,
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

func isKubernetesPodNotFoundError(err error) bool {
	var statusErr *kubeerrors.StatusError
	return errors.As(err, &statusErr) &&
		statusErr.ErrStatus.Code == http.StatusNotFound &&
		statusErr.ErrStatus.Details != nil &&
		statusErr.ErrStatus.Details.Kind == "pods"
}

func newExecutor() *executor {
	e := &executor{
		AbstractExecutor: executors.AbstractExecutor{
			ExecutorOptions: executorOptions,
		},
		remoteProcessTerminated: make(chan shells.TrapCommandExitStatus),
	}

	e.newLogProcessor = func() logProcessor {
		return newKubernetesLogProcessor(
			e.kubeClient,
			e.kubeConfig,
			&backoff.Backoff{Min: time.Second, Max: 30 * time.Second},
			e.Build.Log(),
			kubernetesLogProcessorPodConfig{
				namespace:          e.pod.Namespace,
				pod:                e.pod.Name,
				container:          helperContainerName,
				logPath:            e.logFile(),
				waitLogFileTimeout: waitLogFileTimeout,
			},
		)
	}

	return e
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
		Creator: func() common.Executor {
			return newExecutor()
		},
		FeaturesUpdater:  featuresFn,
		DefaultShellName: executorOptions.Shell.Shell,
	})
}
