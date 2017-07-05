package kubernetes

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/net/context"
	"k8s.io/kubernetes/pkg/api"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/credentialprovider"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/executors"
)

var (
	executorOptions = executors.ExecutorOptions{
		SharedBuildsDir: false,
		Shell: common.ShellScriptInfo{
			Shell:         "bash",
			Type:          common.NormalShell,
			RunnerCommand: "/usr/bin/gitlab-runner-helper",
		},
		ShowHostname: true,
	}
)

type kubernetesOptions struct {
	Image    string   `json:"image"`
	Services []string `json:"services"`
}

type executor struct {
	executors.AbstractExecutor

	kubeClient  *client.Client
	pod         *api.Pod
	credentials *api.Secret
	options     *kubernetesOptions

	namespaceOverwrite      string
	serviceAccountOverwrite string
	buildLimits             api.ResourceList
	serviceLimits           api.ResourceList
	helperLimits            api.ResourceList
	buildRequests           api.ResourceList
	serviceRequests         api.ResourceList
	helperRequests          api.ResourceList
	pullPolicy              common.KubernetesPullPolicy
}

func (s *executor) setupResources() error {
	var err error

	// Limit
	if s.buildLimits, err = limits(s.Config.Kubernetes.CPULimit, s.Config.Kubernetes.MemoryLimit); err != nil {
		return fmt.Errorf("invalid build limits specified: %s", err.Error())
	}

	if s.serviceLimits, err = limits(s.Config.Kubernetes.ServiceCPULimit, s.Config.Kubernetes.ServiceMemoryLimit); err != nil {
		return fmt.Errorf("invalid service limits specified: %s", err.Error())
	}

	if s.helperLimits, err = limits(s.Config.Kubernetes.HelperCPULimit, s.Config.Kubernetes.HelperMemoryLimit); err != nil {
		return fmt.Errorf("invalid helper limits specified: %s", err.Error())
	}

	// Requests
	if s.buildRequests, err = limits(s.Config.Kubernetes.CPURequest, s.Config.Kubernetes.MemoryRequest); err != nil {
		return fmt.Errorf("invalid build requests specified: %s", err.Error())
	}

	if s.serviceRequests, err = limits(s.Config.Kubernetes.ServiceCPURequest, s.Config.Kubernetes.ServiceMemoryRequest); err != nil {
		return fmt.Errorf("invalid service requests specified: %s", err.Error())
	}

	if s.helperRequests, err = limits(s.Config.Kubernetes.HelperCPURequest, s.Config.Kubernetes.HelperMemoryRequest); err != nil {
		return fmt.Errorf("invalid helper requests specified: %s", err.Error())
	}
	return nil
}

func (s *executor) Prepare(options common.ExecutorPrepareOptions) (err error) {
	if err = s.AbstractExecutor.Prepare(options); err != nil {
		return err
	}

	if s.BuildShell.PassFile {
		return fmt.Errorf("kubernetes doesn't support shells that require script file")
	}

	if s.kubeClient, err = getKubeClient(options.Config.Kubernetes); err != nil {
		return fmt.Errorf("error connecting to Kubernetes: %s", err.Error())
	}

	if err = s.setupResources(); err != nil {
		return err
	}

	if s.pullPolicy, err = s.Config.Kubernetes.PullPolicy.Get(); err != nil {
		return err
	}

	if err = s.overwriteNamespace(options.Build); err != nil {
		return err
	}

	if err = s.overwriteServiceAccount(options.Build); err != nil {
		return err
	}

	s.prepareOptions(options.Build)

	if err = s.checkDefaults(); err != nil {
		return err
	}

	s.Println("Using Kubernetes executor with image", s.options.Image, "...")

	return nil
}

func (s *executor) Run(cmd common.ExecutorCommand) error {
	s.Debugln("Starting Kubernetes command...")

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

	containerName := "build"
	if cmd.Predefined {
		containerName = "helper"
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	select {
	case err := <-s.runInContainer(ctx, containerName, cmd.Script):
		if err != nil && strings.Contains(err.Error(), "executing in Docker Container") {
			return &common.BuildError{Inner: err}
		}
		return err

	case <-cmd.Context.Done():
		return fmt.Errorf("build aborted")
	}
}

func (s *executor) Cleanup() {
	if s.pod != nil {
		err := s.kubeClient.Pods(s.pod.Namespace).Delete(s.pod.Name, nil)
		if err != nil {
			s.Errorln(fmt.Sprintf("Error cleaning up pod: %s", err.Error()))
		}
	}
	if s.credentials != nil {
		err := s.kubeClient.Secrets(s.pod.Namespace).Delete(s.credentials.Name)
		if err != nil {
			s.Errorln(fmt.Sprintf("Error cleaning up secrets: %s", err.Error()))
		}
	}
	closeKubeClient(s.kubeClient)
	s.AbstractExecutor.Cleanup()
}

func (s *executor) buildContainer(name, image string, requests, limits api.ResourceList, command ...string) api.Container {
	privileged := false
	if s.Config.Kubernetes != nil {
		privileged = s.Config.Kubernetes.Privileged
	}

	return api.Container{
		Name:            name,
		Image:           image,
		ImagePullPolicy: api.PullPolicy(s.pullPolicy),
		Command:         command,
		Env:             buildVariables(s.Build.GetAllVariables().PublicOrInternal()),
		Resources: api.ResourceRequirements{
			Limits:   limits,
			Requests: requests,
		},
		VolumeMounts: s.getVolumeMounts(),
		SecurityContext: &api.SecurityContext{
			Privileged: &privileged,
		},
		Stdin: true,
	}
}

func (s *executor) getVolumeMounts() (mounts []api.VolumeMount) {
	path := strings.Split(s.Build.BuildDir, "/")
	path = path[:len(path)-1]

	mounts = append(mounts, api.VolumeMount{
		Name:      "repo",
		MountPath: strings.Join(path, "/"),
	})

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

	return
}

func (s *executor) getVolumes() (volumes []api.Volume) {
	volumes = append(volumes, api.Volume{
		Name: "repo",
		VolumeSource: api.VolumeSource{
			EmptyDir: &api.EmptyDirVolumeSource{},
		},
	})

	for _, volume := range s.Config.Kubernetes.Volumes.HostPaths {
		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				HostPath: &api.HostPathVolumeSource{
					Path: volume.HostPath,
				},
			},
		})
	}

	for _, volume := range s.Config.Kubernetes.Volumes.Secrets {
		volumes = append(volumes, api.Volume{
			Name: volume.Name,
			VolumeSource: api.VolumeSource{
				Secret: &api.SecretVolumeSource{
					SecretName: volume.Name,
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

	return
}

func (s *executor) setupCredentials() error {
	authConfigs := make(map[string]credentialprovider.DockerConfigEntry)

	for _, credentials := range s.Build.Credentials {
		if credentials.Type != "registry" {
			continue
		}

		authConfigs[credentials.URL] = credentialprovider.DockerConfigEntry{
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
	secret.Namespace = s.Config.Kubernetes.Namespace
	secret.Type = api.SecretTypeDockercfg
	secret.Data = map[string][]byte{}
	secret.Data[api.DockerConfigKey] = dockerCfgContent

	s.credentials, err = s.kubeClient.Secrets(s.Config.Kubernetes.Namespace).Create(&secret)
	if err != nil {
		return err
	}
	return nil
}

func (s *executor) setupBuildPod() error {
	services := make([]api.Container, len(s.options.Services))
	for i, image := range s.options.Services {
		resolvedImage := s.Build.GetAllVariables().ExpandValue(image)
		services[i] = s.buildContainer(fmt.Sprintf("svc-%d", i), resolvedImage, s.serviceRequests, s.serviceLimits)
	}
	labels := make(map[string]string)
	for k, v := range s.Build.Runner.Kubernetes.PodLabels {
		labels[k] = s.Build.Variables.ExpandValue(v)
	}

	var imagePullSecrets []api.LocalObjectReference
	for _, imagePullSecret := range s.Config.Kubernetes.ImagePullSecrets {
		imagePullSecrets = append(imagePullSecrets, api.LocalObjectReference{Name: imagePullSecret})
	}

	if s.credentials != nil {
		imagePullSecrets = append(imagePullSecrets, api.LocalObjectReference{Name: s.credentials.Name})
	}

	buildImage := s.Build.GetAllVariables().ExpandValue(s.options.Image)
	pod, err := s.kubeClient.Pods(s.Config.Kubernetes.Namespace).Create(&api.Pod{
		ObjectMeta: api.ObjectMeta{
			GenerateName: s.Build.ProjectUniqueName(),
			Namespace:    s.Config.Kubernetes.Namespace,
			Labels:       labels,
		},
		Spec: api.PodSpec{
			Volumes:            s.getVolumes(),
			ServiceAccountName: s.Config.Kubernetes.ServiceAccount,
			RestartPolicy:      api.RestartPolicyNever,
			NodeSelector:       s.Config.Kubernetes.NodeSelector,
			Containers: append([]api.Container{
				// TODO use the build and helper template here
				s.buildContainer("build", buildImage, s.buildRequests, s.buildLimits, s.BuildShell.DockerCommand...),
				s.buildContainer("helper", s.Config.Kubernetes.GetHelperImage(), s.helperRequests, s.helperLimits, s.BuildShell.DockerCommand...),
			}, services...),
			TerminationGracePeriodSeconds: &s.Config.Kubernetes.TerminationGracePeriodSeconds,
			ImagePullSecrets:              imagePullSecrets,
		},
	})

	if err != nil {
		return err
	}

	s.pod = pod

	return nil
}

func (s *executor) runInContainer(ctx context.Context, name, command string) <-chan error {
	errc := make(chan error, 1)
	go func() {
		defer close(errc)

		status, err := waitForPodRunning(ctx, s.kubeClient, s.pod, s.Trace, s.Config.Kubernetes)

		if err != nil {
			errc <- err
			return
		}

		if status != api.PodRunning {
			errc <- fmt.Errorf("pod failed to enter running state: %s", status)
			return
		}

		config, err := getKubeClientConfig(s.Config.Kubernetes)

		if err != nil {
			errc <- err
			return
		}

		exec := ExecOptions{
			PodName:       s.pod.Name,
			Namespace:     s.pod.Namespace,
			ContainerName: name,
			Command:       s.BuildShell.DockerCommand,
			In:            strings.NewReader(command),
			Out:           s.Trace,
			Err:           s.Trace,
			Stdin:         true,
			Config:        config,
			Client:        s.kubeClient,
			Executor:      &DefaultRemoteExecutor{},
		}

		errc <- exec.Run()
	}()

	return errc
}

func (s *executor) prepareOptions(job *common.Build) {
	s.options = &kubernetesOptions{}
	s.options.Image = job.Image.Name
	for _, service := range job.Services {
		serviceName := service.Name
		if serviceName == "" {
			continue
		}

		s.options.Services = append(s.options.Services, serviceName)
	}
}

// checkDefaults Defines the configuration for the Pod on Kubernetes
func (s *executor) checkDefaults() error {
	if s.options.Image == "" {
		if s.Config.Kubernetes.Image == "" {
			return fmt.Errorf("no image specified and no default set in config")
		}

		s.options.Image = s.Config.Kubernetes.Image
	}

	if s.Config.Kubernetes.Namespace == "" {
		s.Warningln("Namespace is empty, therefore assuming 'default'.")
		s.Config.Kubernetes.Namespace = "default"
	}

	s.Println("Using Kubernetes namespace:", s.Config.Kubernetes.Namespace)

	return nil
}

// overwriteNamespace checks for variable in order to overwrite the configured
// namespace, as long as it complies to validation regular-expression, when
// expression is empty the overwrite is disabled.
func (s *executor) overwriteNamespace(job *common.Build) error {
	if s.Config.Kubernetes.NamespaceOverwriteAllowed == "" {
		s.Debugln("Configuration entry 'namespace_overwrite_allowed' is empty, using configured namespace.")
		return nil
	}

	// looking for namespace overwrite variable, and expanding for interpolation
	s.namespaceOverwrite = job.Variables.Expand().Get("KUBERNETES_NAMESPACE_OVERWRITE")
	if s.namespaceOverwrite == "" {
		return nil
	}

	if err := overwriteRegexCheck(s.Config.Kubernetes.NamespaceOverwriteAllowed, s.namespaceOverwrite); err != nil {
		return err
	}

	s.Println("Overwritting configured namespace, from", s.Config.Kubernetes.Namespace, "to", s.namespaceOverwrite)
	s.Config.Kubernetes.Namespace = s.namespaceOverwrite

	return nil
}

// overwriteSercviceAccount checks for variable in order to overwrite the configured
// service account, as long as it complies to validation regular-expression, when
// expression is empty the overwrite is disabled.
func (s *executor) overwriteServiceAccount(job *common.Build) error {
	if s.Config.Kubernetes.ServiceAccountOverwriteAllowed == "" {
		s.Debugln("Configuration entry 'service_accunt_overwrite_allowed' is empty, disabling override.")
		return nil
	}

	s.serviceAccountOverwrite = job.Variables.Expand().Get("KUBERNETES_SERVICE_ACCOUNT_OVERWRITE")
	if s.serviceAccountOverwrite == "" {
		return nil
	}

	if err := overwriteRegexCheck(s.Config.Kubernetes.ServiceAccountOverwriteAllowed, s.serviceAccountOverwrite); err != nil {
		return err
	}

	s.Println("Overwritting configured ServiceAccount, from", s.Config.Kubernetes.ServiceAccount, "to", s.serviceAccountOverwrite)
	s.Config.Kubernetes.ServiceAccount = s.serviceAccountOverwrite

	return nil
}

//overwriteRegexCheck check if the regex provided for overwriting a config field matches the
//paramether provided, returns error if doesn't match
func overwriteRegexCheck(regex, value string) error {
	var err error
	var r *regexp.Regexp
	if r, err = regexp.Compile(regex); err != nil {
		return err
	}

	if match := r.MatchString(value); !match {
		return fmt.Errorf("Provided value %s does not match regex %s", value, regex)
	}
	return nil
}

func createFn() common.Executor {
	return &executor{
		AbstractExecutor: executors.AbstractExecutor{
			ExecutorOptions: executorOptions,
		},
	}
}

func featuresFn(features *common.FeaturesInfo) {
	features.Variables = true
	features.Image = true
	features.Services = true
	features.Artifacts = true
	features.Cache = true
}

func init() {
	common.RegisterExecutor("kubernetes", executors.DefaultExecutorProvider{
		Creator:         createFn,
		FeaturesUpdater: featuresFn,
	})
}
