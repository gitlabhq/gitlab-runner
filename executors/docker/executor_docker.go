package docker

import (
	"bytes"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	docker_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/docker"

	"golang.org/x/net/context"
)

const (
	DockerExecutorStagePrepare common.ExecutorStage = "docker_prepare"
	DockerExecutorStageRun     common.ExecutorStage = "docker_run"
	DockerExecutorStageCleanup common.ExecutorStage = "docker_cleanup"

	DockerExecutorStageCreatingBuildVolumes common.ExecutorStage = "docker_creating_build_volumes"
	DockerExecutorStageCreatingServices     common.ExecutorStage = "docker_creating_services"
	DockerExecutorStageCreatingUserVolumes  common.ExecutorStage = "docker_creating_user_volumes"
	DockerExecutorStagePullingImage         common.ExecutorStage = "docker_pulling_image"
)

var neverRestartPolicy = container.RestartPolicy{Name: "no"}

type executor struct {
	executors.AbstractExecutor
	client      docker_helpers.Client
	failures    []string // IDs of containers that have failed in some way
	builds      []string // IDs of successfully created build containers
	services    []*types.Container
	caches      []string // IDs of cache containers
	info        types.Info
	binds       []string
	volumesFrom []string
	devices     []container.DeviceMapping
	links       []string
}

func (s *executor) getServiceVariables() []string {
	return s.Build.GetAllVariables().PublicOrInternal().StringList()
}

func (s *executor) getUserAuthConfiguration(indexName string) *types.AuthConfig {
	if s.Build == nil {
		return nil
	}

	buf := bytes.NewBufferString(s.Build.GetDockerAuthConfig())
	authConfigs, _ := docker_helpers.ReadAuthConfigsFromReader(buf)
	if authConfigs != nil {
		return docker_helpers.ResolveDockerAuthConfig(indexName, authConfigs)
	}
	return nil
}

func (s *executor) getBuildAuthConfiguration(indexName string) *types.AuthConfig {
	if s.Build == nil {
		return nil
	}

	authConfigs := make(map[string]types.AuthConfig)

	for _, credentials := range s.Build.Credentials {
		if credentials.Type != "registry" {
			continue
		}

		authConfigs[credentials.URL] = types.AuthConfig{
			Username:      credentials.Username,
			Password:      credentials.Password,
			ServerAddress: credentials.URL,
		}
	}

	if authConfigs != nil {
		return docker_helpers.ResolveDockerAuthConfig(indexName, authConfigs)
	}
	return nil
}

func (s *executor) getHomeDirAuthConfiguration(indexName string) *types.AuthConfig {
	authConfigs, _ := docker_helpers.ReadDockerAuthConfigsFromHomeDir(s.Shell().User)
	if authConfigs != nil {
		return docker_helpers.ResolveDockerAuthConfig(indexName, authConfigs)
	}
	return nil
}

func (s *executor) getAuthConfig(imageName string) *types.AuthConfig {
	indexName, _ := docker_helpers.SplitDockerImageName(imageName)

	authConfig := s.getUserAuthConfiguration(indexName)
	if authConfig == nil {
		authConfig = s.getHomeDirAuthConfiguration(indexName)
	}
	if authConfig == nil {
		authConfig = s.getBuildAuthConfiguration(indexName)
	}

	if authConfig != nil {
		s.Debugln("Using", authConfig.Username, "to connect to", authConfig.ServerAddress,
			"in order to resolve", imageName, "...")
		return authConfig
	}

	s.Debugln(fmt.Sprintf("No credentials found for %v", indexName))
	return nil
}

func (s *executor) pullDockerImage(imageName string, ac *types.AuthConfig) (*types.ImageInspect, error) {
	s.SetCurrentStage(DockerExecutorStagePullingImage)
	s.Println("Pulling docker image", imageName, "...")

	ref := imageName
	// Add :latest to limit the download results
	if !strings.ContainsAny(ref, ":@") {
		ref += ":latest"
	}

	options := types.ImagePullOptions{}
	if ac != nil {
		options.RegistryAuth, _ = docker_helpers.EncodeAuthConfig(ac)
	}

	if err := s.client.ImagePullBlocking(s.Context, ref, options); err != nil {
		return nil, err
	}

	image, _, err := s.client.ImageInspectWithRaw(s.Context, imageName)
	return &image, err
}

func (s *executor) getDockerImage(imageName string) (*types.ImageInspect, error) {
	pullPolicy, err := s.Config.Docker.PullPolicy.Get()
	if err != nil {
		return nil, err
	}

	authConfig := s.getAuthConfig(imageName)

	s.Debugln("Looking for image", imageName, "...")
	image, _, err := s.client.ImageInspectWithRaw(s.Context, imageName)

	// If never is specified then we return what inspect did return
	if pullPolicy == common.PullPolicyNever {
		return &image, err
	}

	if err == nil {
		// Don't pull image that is passed by ID
		if image.ID == imageName {
			return &image, nil
		}

		// If not-present is specified
		if pullPolicy == common.PullPolicyIfNotPresent {
			s.Println("Using locally found image version due to if-not-present pull policy")
			return &image, err
		}
	}

	newImage, err := s.pullDockerImage(imageName, authConfig)
	if err != nil {
		if docker_helpers.IsErrNotFound(err) {
			return nil, &common.BuildError{Inner: err}
		}
		return nil, err
	}
	return newImage, nil
}

func (s *executor) getArchitecture() string {
	architecture := s.info.Architecture
	switch architecture {
	case "armv6l", "armv7l", "aarch64":
		architecture = "arm"
	case "amd64":
		architecture = "x86_64"
	}

	if architecture != "" {
		return architecture
	}

	switch runtime.GOARCH {
	case "amd64":
		return "x86_64"
	default:
		return runtime.GOARCH
	}
}

func (s *executor) getPrebuiltImage() (*types.ImageInspect, error) {
	if imageNameFromConfig := s.Config.Docker.HelperImage; imageNameFromConfig != "" {
		s.Debugln("Pull configured helper_image for predefined container instead of import bundled image", imageNameFromConfig, "...")
		return s.getDockerImage(imageNameFromConfig)
	}

	architecture := s.getArchitecture()
	if architecture == "" {
		return nil, errors.New("unsupported docker architecture")
	}

	imageName := prebuiltImageName + ":" + architecture + "-" + common.REVISION
	s.Debugln("Looking for prebuilt image", imageName, "...")
	image, _, err := s.client.ImageInspectWithRaw(s.Context, imageName)
	if err == nil {
		return &image, nil
	}

	data, err := Asset("prebuilt-" + architecture + prebuiltImageExtension)
	if err != nil {
		return nil, fmt.Errorf("Unsupported architecture: %s: %q", architecture, err.Error())
	}

	s.Debugln("Loading prebuilt image...")

	ref := prebuiltImageName
	source := types.ImageImportSource{
		Source:     bytes.NewBuffer(data),
		SourceName: "-",
	}
	options := types.ImageImportOptions{
		Tag: architecture + "-" + common.REVISION,
	}

	if err := s.client.ImageImportBlocking(s.Context, source, ref, options); err != nil {
		return nil, fmt.Errorf("Failed to import image: %s", err)
	}

	image, _, err = s.client.ImageInspectWithRaw(s.Context, imageName)
	if err != nil {
		s.Debugln("Inspecting imported image", imageName, "failed:", err)
		return nil, err
	}

	return &image, err
}

func (s *executor) getAbsoluteContainerPath(dir string) string {
	if path.IsAbs(dir) {
		return dir
	}
	return path.Join(s.Build.FullProjectDir(), dir)
}

func (s *executor) addHostVolume(hostPath, containerPath string) error {
	containerPath = s.getAbsoluteContainerPath(containerPath)
	s.Debugln("Using host-based", hostPath, "for", containerPath, "...")
	s.binds = append(s.binds, fmt.Sprintf("%v:%v", hostPath, containerPath))
	return nil
}

func (s *executor) getLabels(containerType string, otherLabels ...string) map[string]string {
	labels := make(map[string]string)
	labels[dockerLabelPrefix+".job.id"] = strconv.Itoa(s.Build.ID)
	labels[dockerLabelPrefix+".job.sha"] = s.Build.GitInfo.Sha
	labels[dockerLabelPrefix+".job.before_sha"] = s.Build.GitInfo.BeforeSha
	labels[dockerLabelPrefix+".job.ref"] = s.Build.GitInfo.Ref
	labels[dockerLabelPrefix+".project.id"] = strconv.Itoa(s.Build.JobInfo.ProjectID)
	labels[dockerLabelPrefix+".runner.id"] = s.Build.Runner.ShortDescription()
	labels[dockerLabelPrefix+".runner.local_id"] = strconv.Itoa(s.Build.RunnerID)
	labels[dockerLabelPrefix+".type"] = containerType
	for _, label := range otherLabels {
		keyValue := strings.SplitN(label, "=", 2)
		if len(keyValue) == 2 {
			labels[dockerLabelPrefix+"."+keyValue[0]] = keyValue[1]
		}
	}
	return labels
}

// createCacheVolume returns the id of the created container, or an error
func (s *executor) createCacheVolume(containerName, containerPath string) (string, error) {
	// get busybox image
	cacheImage, err := s.getPrebuiltImage()
	if err != nil {
		return "", err
	}

	config := &container.Config{
		Image: cacheImage.ID,
		Cmd: []string{
			"gitlab-runner-cache", containerPath,
		},
		Volumes: map[string]struct{}{
			containerPath: {},
		},
		Labels: s.getLabels("cache", "cache.dir="+containerPath),
	}

	hostConfig := &container.HostConfig{
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
	}

	resp, err := s.client.ContainerCreate(s.Context, config, hostConfig, nil, containerName)
	if err != nil {
		if resp.ID != "" {
			s.failures = append(s.failures, resp.ID)
		}
		return "", err
	}

	s.Debugln("Starting cache container", resp.ID, "...")
	err = s.client.ContainerStart(s.Context, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		s.failures = append(s.failures, resp.ID)
		return "", err
	}

	s.Debugln("Waiting for cache container", resp.ID, "...")
	err = s.waitForContainer(resp.ID)
	if err != nil {
		s.failures = append(s.failures, resp.ID)
		return "", err
	}

	return resp.ID, nil
}

func (s *executor) addCacheVolume(containerPath string) error {
	var err error
	containerPath = s.getAbsoluteContainerPath(containerPath)

	// disable cache for automatic container cache, but leave it for host volumes (they are shared on purpose)
	if s.Config.Docker.DisableCache {
		s.Debugln("Container cache for", containerPath, " is disabled.")
		return nil
	}

	hash := md5.Sum([]byte(containerPath))

	// use host-based cache
	if cacheDir := s.Config.Docker.CacheDir; cacheDir != "" {
		hostPath := fmt.Sprintf("%s/%s/%x", cacheDir, s.Build.ProjectUniqueName(), hash)
		hostPath, err := filepath.Abs(hostPath)
		if err != nil {
			return err
		}
		s.Debugln("Using path", hostPath, "as cache for", containerPath, "...")
		s.binds = append(s.binds, fmt.Sprintf("%v:%v", filepath.ToSlash(hostPath), containerPath))
		return nil
	}

	// get existing cache container
	var containerID string
	containerName := fmt.Sprintf("%s-cache-%x", s.Build.ProjectUniqueName(), hash)
	if inspected, err := s.client.ContainerInspect(s.Context, containerName); err == nil {
		// check if we have valid cache, if not remove the broken container
		if _, ok := inspected.Config.Volumes[containerPath]; !ok {
			s.Debugln("Removing broken cache container for ", containerPath, "path")
			s.removeContainer(s.Context, inspected.ID)
		} else {
			containerID = inspected.ID
		}
	}

	// create new cache container for that project
	if containerID == "" {
		containerID, err = s.createCacheVolume(containerName, containerPath)
		if err != nil {
			return err
		}
	}

	s.Debugln("Using container", containerID, "as cache", containerPath, "...")
	s.volumesFrom = append(s.volumesFrom, containerID)
	return nil
}

func (s *executor) addVolume(volume string) error {
	var err error
	hostVolume := strings.SplitN(volume, ":", 2)
	switch len(hostVolume) {
	case 2:
		err = s.addHostVolume(hostVolume[0], hostVolume[1])

	case 1:
		// disable cache disables
		err = s.addCacheVolume(hostVolume[0])
	}

	if err != nil {
		s.Errorln("Failed to create container volume for", volume, err)
	}
	return err
}

func fakeContainer(id string, names ...string) *types.Container {
	return &types.Container{ID: id, Names: names}
}

func (s *executor) createBuildVolume() error {
	// Cache Git sources:
	// take path of the projects directory,
	// because we use `rm -rf` which could remove the mounted volume
	parentDir := path.Dir(s.Build.FullProjectDir())

	if !path.IsAbs(parentDir) && parentDir != "/" {
		return errors.New("build directory needs to be absolute and non-root path")
	}

	if s.isHostMountedVolume(s.Build.RootDir, s.Config.Docker.Volumes...) {
		return nil
	}

	if s.Build.GetGitStrategy() == common.GitFetch && !s.Config.Docker.DisableCache {
		// create persistent cache container
		return s.addVolume(parentDir)
	}

	// create temporary cache container
	id, err := s.createCacheVolume("", parentDir)
	if err != nil {
		return err
	}

	s.caches = append(s.caches, id)
	s.volumesFrom = append(s.volumesFrom, id)

	return nil
}

func (s *executor) createUserVolumes() (err error) {
	for _, volume := range s.Config.Docker.Volumes {
		err = s.addVolume(volume)
		if err != nil {
			return
		}
	}
	return nil
}

func (s *executor) isHostMountedVolume(dir string, volumes ...string) bool {
	isParentOf := func(parent string, dir string) bool {
		for dir != "/" && dir != "." {
			if dir == parent {
				return true
			}
			dir = path.Dir(dir)
		}
		return false
	}

	for _, volume := range volumes {
		hostVolume := strings.Split(volume, ":")
		if len(hostVolume) < 2 {
			continue
		}

		if isParentOf(path.Clean(hostVolume[1]), path.Clean(dir)) {
			return true
		}
	}
	return false
}

func (s *executor) parseDeviceString(deviceString string) (device container.DeviceMapping, err error) {
	// Split the device string PathOnHost[:PathInContainer[:CgroupPermissions]]
	parts := strings.Split(deviceString, ":")

	if len(parts) > 3 {
		err = fmt.Errorf("Too many colons")
		return
	}

	device.PathOnHost = parts[0]

	// Optional container path
	if len(parts) >= 2 {
		device.PathInContainer = parts[1]
	} else {
		// default: device at same path in container
		device.PathInContainer = device.PathOnHost
	}

	// Optional permissions
	if len(parts) >= 3 {
		device.CgroupPermissions = parts[2]
	} else {
		// default: rwm, just like 'docker run'
		device.CgroupPermissions = "rwm"
	}

	return
}

func (s *executor) bindDevices() (err error) {
	for _, deviceString := range s.Config.Docker.Devices {
		device, err := s.parseDeviceString(deviceString)
		if err != nil {
			err = fmt.Errorf("Failed to parse device string %q: %s", deviceString, err)
			return err
		}

		s.devices = append(s.devices, device)
	}
	return nil
}

func (s *executor) printUsedDockerImageID(imageName, imageID, containerType, containerTypeName string) {
	var line string
	if imageName == imageID {
		line = fmt.Sprintf("Using docker image %s for %s %s...", imageName, containerTypeName, containerType)
	} else {
		line = fmt.Sprintf("Using docker image %s ID=%s for %s %s...", imageName, imageID, containerTypeName, containerType)
	}
	s.Println(line)
}

func (s *executor) splitServiceAndVersion(serviceDescription string) (service, version, imageName string, linkNames []string) {
	ReferenceRegexpNoPort := regexp.MustCompile(`^(.*?)(|:[0-9]+)(|/.*)$`)
	imageName = serviceDescription
	version = "latest"

	if match := reference.ReferenceRegexp.FindStringSubmatch(serviceDescription); match != nil {
		matchService := ReferenceRegexpNoPort.FindStringSubmatch(match[1])
		service = matchService[1] + matchService[3]

		if len(match[2]) > 0 {
			version = match[2]
		} else {
			imageName = match[1] + ":" + version
		}
	} else {
		return
	}

	linkName := strings.Replace(service, "/", "__", -1)
	linkNames = append(linkNames, linkName)

	// Create alternative link name according to RFC 1123
	// Where you can use only `a-zA-Z0-9-`
	if alternativeName := strings.Replace(service, "/", "-", -1); linkName != alternativeName {
		linkNames = append(linkNames, alternativeName)
	}
	return
}

func (s *executor) createService(serviceIndex int, service, version, image string, serviceDefinition common.Image) (*types.Container, error) {
	if len(service) == 0 {
		return nil, errors.New("invalid service name")
	}

	s.Println("Starting service", service+":"+version, "...")
	serviceImage, err := s.getDockerImage(image)
	if err != nil {
		return nil, err
	}

	s.printUsedDockerImageID(image, serviceImage.ID, "service", service)

	serviceSlug := strings.Replace(service, "/", "__", -1)
	containerName := fmt.Sprintf("%s-%s-%d", s.Build.ProjectUniqueName(), serviceSlug, serviceIndex)

	// this will fail potentially some builds if there's name collision
	s.removeContainer(s.Context, containerName)

	config := &container.Config{
		Image:  serviceImage.ID,
		Labels: s.getLabels("service", "service="+service, "service.version="+version),
		Env:    s.getServiceVariables(),
	}

	if len(serviceDefinition.Command) > 0 {
		config.Cmd = serviceDefinition.Command
	}
	if len(serviceDefinition.Entrypoint) > 0 {
		config.Entrypoint = serviceDefinition.Entrypoint
	}

	hostConfig := &container.HostConfig{
		RestartPolicy: neverRestartPolicy,
		Privileged:    s.Config.Docker.Privileged,
		NetworkMode:   container.NetworkMode(s.Config.Docker.NetworkMode),
		Binds:         s.binds,
		ShmSize:       s.Config.Docker.ShmSize,
		VolumesFrom:   s.volumesFrom,
		Tmpfs:         s.Config.Docker.ServicesTmpfs,
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
	}

	s.Debugln("Creating service container", containerName, "...")
	resp, err := s.client.ContainerCreate(s.Context, config, hostConfig, nil, containerName)
	if err != nil {
		return nil, err
	}

	s.Debugln("Starting service container", resp.ID, "...")
	err = s.client.ContainerStart(s.Context, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		s.failures = append(s.failures, resp.ID)
		return nil, err
	}

	return fakeContainer(resp.ID, containerName), nil
}

func (s *executor) getServicesDefinitions() (common.Services, error) {
	serviceDefinitions := common.Services{}
	for _, service := range s.Config.Docker.Services {
		serviceDefinitions = append(serviceDefinitions, common.Image{Name: service})
	}

	for _, service := range s.Build.Services {
		serviceName := s.Build.GetAllVariables().ExpandValue(service.Name)
		err := s.verifyAllowedImage(serviceName, "services", s.Config.Docker.AllowedServices, s.Config.Docker.Services)
		if err != nil {
			return nil, err
		}

		service.Name = serviceName
		serviceDefinitions = append(serviceDefinitions, service)
	}

	return serviceDefinitions, nil
}

func (s *executor) waitForServices() {
	waitForServicesTimeout := s.Config.Docker.WaitForServicesTimeout
	if waitForServicesTimeout == 0 {
		waitForServicesTimeout = common.DefaultWaitForServicesTimeout
	}

	// wait for all services to came up
	if waitForServicesTimeout > 0 && len(s.services) > 0 {
		s.Println("Waiting for services to be up and running...")
		wg := sync.WaitGroup{}
		for _, service := range s.services {
			wg.Add(1)
			go func(service *types.Container) {
				s.waitForServiceContainer(service, time.Duration(waitForServicesTimeout)*time.Second)
				wg.Done()
			}(service)
		}
		wg.Wait()
	}
}

func (s *executor) buildServiceLinks(linksMap map[string]*types.Container) (links []string) {
	for linkName, linkee := range linksMap {
		newContainer, err := s.client.ContainerInspect(s.Context, linkee.ID)
		if err != nil {
			continue
		}
		if newContainer.State.Running {
			links = append(links, linkee.ID+":"+linkName)
		}
	}
	return
}

func (s *executor) createFromServiceDefinition(serviceIndex int, serviceDefinition common.Image, linksMap map[string]*types.Container) (err error) {
	var container *types.Container

	service, version, imageName, linkNames := s.splitServiceAndVersion(serviceDefinition.Name)

	if serviceDefinition.Alias != "" {
		linkNames = append(linkNames, serviceDefinition.Alias)
	}

	for _, linkName := range linkNames {
		if linksMap[linkName] != nil {
			s.Warningln("Service", serviceDefinition.Name, "is already created. Ignoring.")
			continue
		}

		// Create service if not yet created
		if container == nil {
			container, err = s.createService(serviceIndex, service, version, imageName, serviceDefinition)
			if err != nil {
				return
			}
			s.Debugln("Created service", serviceDefinition.Name, "as", container.ID)
			s.services = append(s.services, container)
		}
		linksMap[linkName] = container
	}
	return
}

func (s *executor) createServices() (err error) {
	servicesDefinitions, err := s.getServicesDefinitions()
	if err != nil {
		return
	}

	linksMap := make(map[string]*types.Container)

	for index, serviceDefinition := range servicesDefinitions {
		err = s.createFromServiceDefinition(index, serviceDefinition, linksMap)
		if err != nil {
			return
		}
	}

	s.waitForServices()

	s.links = s.buildServiceLinks(linksMap)
	return
}

func (s *executor) createContainer(containerType string, imageDefinition common.Image, cmd []string, allowedInternalImages []string) (*types.ContainerJSON, error) {
	imageName, err := s.expandImageName(imageDefinition.Name, allowedInternalImages)
	if err != nil {
		return nil, err
	}

	// Fetch image
	image, err := s.getDockerImage(imageName)
	if err != nil {
		return nil, err
	}

	s.printUsedDockerImageID(imageName, image.ID, "container", containerType)

	hostname := s.Config.Docker.Hostname
	if hostname == "" {
		hostname = s.Build.ProjectUniqueName()
	}

	containerName := s.Build.ProjectUniqueName() + "-" + containerType
	config := &container.Config{
		Image:        image.ID,
		Hostname:     hostname,
		Cmd:          cmd,
		Labels:       s.getLabels(containerType),
		Tty:          false,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    true,
		StdinOnce:    true,
		Env:          append(s.Build.GetAllVariables().StringList(), s.BuildShell.Environment...),
	}

	if len(imageDefinition.Entrypoint) > 0 {
		config.Entrypoint = imageDefinition.Entrypoint
	}

	nanoCPUs, err := s.Config.Docker.GetNanoCPUs()
	if err != nil {
		return nil, err
	}

	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			CpusetCpus: s.Config.Docker.CPUSetCPUs,
			NanoCPUs:   nanoCPUs,
			Devices:    s.devices,
		},
		DNS:           s.Config.Docker.DNS,
		DNSSearch:     s.Config.Docker.DNSSearch,
		Privileged:    s.Config.Docker.Privileged,
		UsernsMode:    container.UsernsMode(s.Config.Docker.UsernsMode),
		CapAdd:        s.Config.Docker.CapAdd,
		CapDrop:       s.Config.Docker.CapDrop,
		SecurityOpt:   s.Config.Docker.SecurityOpt,
		RestartPolicy: neverRestartPolicy,
		ExtraHosts:    s.Config.Docker.ExtraHosts,
		NetworkMode:   container.NetworkMode(s.Config.Docker.NetworkMode),
		Links:         append(s.Config.Docker.Links, s.links...),
		Binds:         s.binds,
		ShmSize:       s.Config.Docker.ShmSize,
		VolumeDriver:  s.Config.Docker.VolumeDriver,
		VolumesFrom:   append(s.Config.Docker.VolumesFrom, s.volumesFrom...),
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
		Tmpfs:   s.Config.Docker.Tmpfs,
		Sysctls: s.Config.Docker.SysCtls,
	}

	// this will fail potentially some builds if there's name collision
	s.removeContainer(s.Context, containerName)

	s.Debugln("Creating container", containerName, "...")
	resp, err := s.client.ContainerCreate(s.Context, config, hostConfig, nil, containerName)
	if err != nil {
		if resp.ID != "" {
			s.failures = append(s.failures, resp.ID)
		}
		return nil, err
	}

	inspect, err := s.client.ContainerInspect(s.Context, resp.ID)
	if err != nil {
		s.failures = append(s.failures, resp.ID)
		return nil, err
	}

	s.builds = append(s.builds, resp.ID)
	return &inspect, nil
}

func (s *executor) killContainer(id string, waitCh chan error) (err error) {
	for {
		s.disconnectNetwork(s.Context, id)
		s.Debugln("Killing container", id, "...")
		s.client.ContainerKill(s.Context, id, "SIGKILL")

		// Wait for signal that container were killed
		// or retry after some time
		select {
		case err = <-waitCh:
			return

		case <-time.After(time.Second):
		}
	}
}

func (s *executor) waitForContainer(id string) error {
	s.Debugln("Waiting for container", id, "...")

	retries := 0

	// Use active wait
	for {
		container, err := s.client.ContainerInspect(s.Context, id)
		if err != nil {
			if docker_helpers.IsErrNotFound(err) {
				return err
			}

			if retries > 3 {
				return err
			}

			retries++
			time.Sleep(time.Second)
			continue
		}

		// Reset retry timer
		retries = 0

		if container.State.Running {
			time.Sleep(time.Second)
			continue
		}

		if container.State.ExitCode != 0 {
			return &common.BuildError{
				Inner: fmt.Errorf("exit code %d", container.State.ExitCode),
			}
		}

		return nil
	}
}

func (s *executor) watchContainer(ctx context.Context, id string, input io.Reader) (err error) {
	options := types.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	}

	s.Debugln("Attaching to container", id, "...")
	hijacked, err := s.client.ContainerAttach(ctx, id, options)
	if err != nil {
		return
	}
	defer hijacked.Close()

	s.Debugln("Starting container", id, "...")
	err = s.client.ContainerStart(ctx, id, types.ContainerStartOptions{})
	if err != nil {
		return
	}

	s.Debugln("Waiting for attach to finish", id, "...")
	attachCh := make(chan error, 2)

	// Copy any output to the build trace
	go func() {
		_, err := stdcopy.StdCopy(s.Trace, s.Trace, hijacked.Reader)
		if err != nil {
			attachCh <- err
		}
	}()

	// Write the input to the container and close its STDIN to get it to finish
	go func() {
		_, err := io.Copy(hijacked.Conn, input)
		hijacked.CloseWrite()
		if err != nil {
			attachCh <- err
		}
	}()

	waitCh := make(chan error, 1)
	go func() {
		waitCh <- s.waitForContainer(id)
	}()

	select {
	case <-ctx.Done():
		s.killContainer(id, waitCh)
		err = errors.New("Aborted")

	case err = <-attachCh:
		s.killContainer(id, waitCh)
		s.Debugln("Container", id, "finished with", err)

	case err = <-waitCh:
		s.Debugln("Container", id, "finished with", err)
	}
	return
}

func (s *executor) removeContainer(ctx context.Context, id string) error {
	s.disconnectNetwork(ctx, id)
	options := types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}
	err := s.client.ContainerRemove(ctx, id, options)
	s.Debugln("Removed container", id, "with", err)
	return err
}

func (s *executor) disconnectNetwork(ctx context.Context, id string) error {
	netList, err := s.client.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		s.Debugln("Can't get network list. ListNetworks exited with", err)
		return err
	}

	for _, network := range netList {
		for _, pluggedContainer := range network.Containers {
			if id == pluggedContainer.Name {
				err = s.client.NetworkDisconnect(ctx, network.ID, id, true)
				if err != nil {
					s.Warningln("Can't disconnect possibly zombie container", pluggedContainer.Name, "from network", network.Name, "->", err)
				} else {
					s.Warningln("Possibly zombie container", pluggedContainer.Name, "is disconnected from network", network.Name)
				}
				break
			}
		}
	}
	return err
}

func (s *executor) verifyAllowedImage(image, optionName string, allowedImages []string, internalImages []string) error {
	for _, allowedImage := range allowedImages {
		ok, _ := filepath.Match(allowedImage, image)
		if ok {
			return nil
		}
	}

	for _, internalImage := range internalImages {
		if internalImage == image {
			return nil
		}
	}

	if len(allowedImages) != 0 {
		s.Println()
		s.Errorln("The", image, "is not present on list of allowed", optionName)
		for _, allowedImage := range allowedImages {
			s.Println("-", allowedImage)
		}
		s.Println()
	} else {
		// by default allow to override the image name
		return nil
	}

	s.Println("Please check runner's configuration: http://doc.gitlab.com/ci/docker/using_docker_images.html#overwrite-image-and-services")
	return errors.New("invalid image")
}

func (s *executor) expandImageName(imageName string, allowedInternalImages []string) (string, error) {
	if imageName != "" {
		image := s.Build.GetAllVariables().ExpandValue(imageName)
		allowedInternalImages = append(allowedInternalImages, s.Config.Docker.Image)
		err := s.verifyAllowedImage(image, "images", s.Config.Docker.AllowedImages, allowedInternalImages)
		if err != nil {
			return "", err
		}
		return image, nil
	}

	if s.Config.Docker.Image == "" {
		return "", errors.New("No Docker image specified to run the build in")
	}

	return s.Config.Docker.Image, nil
}

func (s *executor) connectDocker() (err error) {
	client, err := docker_helpers.New(s.Config.Docker.DockerCredentials, DockerAPIVersion)
	if err != nil {
		return err
	}
	s.client = client

	s.info, err = client.Info(s.Context)
	if err != nil {
		return err
	}

	return
}

func (s *executor) createDependencies() (err error) {
	err = s.bindDevices()
	if err != nil {
		return err
	}

	s.SetCurrentStage(DockerExecutorStageCreatingBuildVolumes)
	s.Debugln("Creating build volume...")
	err = s.createBuildVolume()
	if err != nil {
		return err
	}

	s.SetCurrentStage(DockerExecutorStageCreatingServices)
	s.Debugln("Creating services...")
	err = s.createServices()
	if err != nil {
		return err
	}

	s.SetCurrentStage(DockerExecutorStageCreatingUserVolumes)
	s.Debugln("Creating user-defined volumes...")
	err = s.createUserVolumes()
	if err != nil {
		return err
	}

	return
}

func (s *executor) Prepare(options common.ExecutorPrepareOptions) error {
	err := s.prepareBuildsDir(options.Config)
	if err != nil {
		return err
	}

	err = s.AbstractExecutor.Prepare(options)
	if err != nil {
		return err
	}

	if s.BuildShell.PassFile {
		return errors.New("Docker doesn't support shells that require script file")
	}

	if options.Config.Docker == nil {
		return errors.New("Missing docker configuration")
	}

	s.SetCurrentStage(DockerExecutorStagePrepare)
	imageName, err := s.expandImageName(s.Build.Image.Name, []string{})
	if err != nil {
		return err
	}

	s.Println("Using Docker executor with image", imageName, "...")

	err = s.connectDocker()
	if err != nil {
		return err
	}

	err = s.createDependencies()
	if err != nil {
		return err
	}
	return nil
}

func (s *executor) prepareBuildsDir(config *common.RunnerConfig) error {
	rootDir := config.BuildsDir
	if rootDir == "" {
		rootDir = s.DefaultBuildsDir
	}
	if s.isHostMountedVolume(rootDir, config.Docker.Volumes...) {
		s.SharedBuildsDir = true
	}
	return nil
}

func (s *executor) Cleanup() {
	s.SetCurrentStage(DockerExecutorStageCleanup)

	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(context.Background(), dockerCleanupTimeout)
	defer cancel()

	remove := func(id string) {
		wg.Add(1)
		go func() {
			s.removeContainer(ctx, id)
			wg.Done()
		}()
	}

	for _, failureID := range s.failures {
		remove(failureID)
	}

	for _, service := range s.services {
		remove(service.ID)
	}

	for _, cacheID := range s.caches {
		remove(cacheID)
	}

	for _, buildID := range s.builds {
		remove(buildID)
	}

	wg.Wait()

	if s.client != nil {
		s.client.Close()
	}

	s.AbstractExecutor.Cleanup()
}

func (s *executor) runServiceHealthCheckContainer(service *types.Container, timeout time.Duration) error {
	waitImage, err := s.getPrebuiltImage()
	if err != nil {
		return err
	}

	containerName := service.Names[0] + "-wait-for-service"

	config := &container.Config{
		Cmd:    []string{"gitlab-runner-service"},
		Image:  waitImage.ID,
		Labels: s.getLabels("wait", "wait="+service.ID),
	}
	hostConfig := &container.HostConfig{
		RestartPolicy: neverRestartPolicy,
		Links:         []string{service.Names[0] + ":" + service.Names[0]},
		NetworkMode:   container.NetworkMode(s.Config.Docker.NetworkMode),
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
	}
	s.Debugln("Waiting for service container", containerName, "to be up and running...")
	resp, err := s.client.ContainerCreate(s.Context, config, hostConfig, nil, containerName)
	if err != nil {
		return err
	}
	defer s.removeContainer(s.Context, resp.ID)
	err = s.client.ContainerStart(s.Context, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	waitResult := make(chan error, 1)
	go func() {
		waitResult <- s.waitForContainer(resp.ID)
	}()

	// these are warnings and they don't make the build fail
	select {
	case err := <-waitResult:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("service %v did timeout", containerName)
	}
}

func (s *executor) waitForServiceContainer(service *types.Container, timeout time.Duration) error {
	err := s.runServiceHealthCheckContainer(service, timeout)
	if err == nil {
		return nil
	}

	var buffer bytes.Buffer
	buffer.WriteString("\n")
	buffer.WriteString(helpers.ANSI_YELLOW + "*** WARNING:" + helpers.ANSI_RESET + " Service " + service.Names[0] + " probably didn't start properly.\n")
	buffer.WriteString("\n")
	buffer.WriteString(strings.TrimSpace(err.Error()) + "\n")

	var containerBuffer bytes.Buffer

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
	}

	hijacked, err := s.client.ContainerLogs(s.Context, service.ID, options)
	if err == nil {
		defer hijacked.Close()
		stdcopy.StdCopy(&containerBuffer, &containerBuffer, hijacked)
		if containerLog := containerBuffer.String(); containerLog != "" {
			buffer.WriteString("\n")
			buffer.WriteString(strings.TrimSpace(containerLog))
			buffer.WriteString("\n")
		}
	} else {
		buffer.WriteString(strings.TrimSpace(err.Error()) + "\n")
	}

	buffer.WriteString("\n")
	buffer.WriteString(helpers.ANSI_YELLOW + "*********" + helpers.ANSI_RESET + "\n")
	buffer.WriteString("\n")
	io.Copy(s.Trace, &buffer)
	return err
}
