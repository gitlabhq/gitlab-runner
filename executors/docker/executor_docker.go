package docker

import (
	"bytes"
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/kardianos/osext"
	"github.com/mattn/go-zglob"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	docker_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/helperimage"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
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

var DockerPrebuiltImagesPaths []string

var neverRestartPolicy = container.RestartPolicy{Name: "no"}

type executor struct {
	executors.AbstractExecutor
	client docker_helpers.Client
	info   types.Info

	temporary []string // IDs of containers that should be removed

	builds   []string // IDs of successfully created build containers
	services []*types.Container
	caches   []string // IDs of cache containers

	binds []string
	links []string

	devices []container.DeviceMapping

	usedImages     map[string]string
	usedImagesLock sync.RWMutex
}

func init() {
	runnerFolder, err := osext.ExecutableFolder()
	if err != nil {
		logrus.Errorln("Docker executor: unable to detect gitlab-runner folder, prebuilt image helpers will be loaded from DockerHub.", err)
	}

	DockerPrebuiltImagesPaths = []string{
		filepath.Join(runnerFolder, "helper-images"),
		filepath.Join(runnerFolder, "out/helper-images"),
	}
}

func (e *executor) getServiceVariables() []string {
	return e.Build.GetAllVariables().PublicOrInternal().StringList()
}

func (e *executor) getUserAuthConfiguration(indexName string) *types.AuthConfig {
	if e.Build == nil {
		return nil
	}

	buf := bytes.NewBufferString(e.Build.GetDockerAuthConfig())
	authConfigs, _ := docker_helpers.ReadAuthConfigsFromReader(buf)
	if authConfigs != nil {
		return docker_helpers.ResolveDockerAuthConfig(indexName, authConfigs)
	}
	return nil
}

func (e *executor) getBuildAuthConfiguration(indexName string) *types.AuthConfig {
	if e.Build == nil {
		return nil
	}

	authConfigs := make(map[string]types.AuthConfig)

	for _, credentials := range e.Build.Credentials {
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

func (e *executor) getHomeDirAuthConfiguration(indexName string) *types.AuthConfig {
	authConfigs, _ := docker_helpers.ReadDockerAuthConfigsFromHomeDir(e.Shell().User)
	if authConfigs != nil {
		return docker_helpers.ResolveDockerAuthConfig(indexName, authConfigs)
	}
	return nil
}

func (e *executor) getAuthConfig(imageName string) *types.AuthConfig {
	indexName, _ := docker_helpers.SplitDockerImageName(imageName)

	authConfig := e.getUserAuthConfiguration(indexName)
	if authConfig == nil {
		authConfig = e.getHomeDirAuthConfiguration(indexName)
	}
	if authConfig == nil {
		authConfig = e.getBuildAuthConfiguration(indexName)
	}

	if authConfig != nil {
		e.Debugln("Using", authConfig.Username, "to connect to", authConfig.ServerAddress,
			"in order to resolve", imageName, "...")
		return authConfig
	}

	e.Debugln(fmt.Sprintf("No credentials found for %v", indexName))
	return nil
}

func (e *executor) pullDockerImage(imageName string, ac *types.AuthConfig) (*types.ImageInspect, error) {
	e.SetCurrentStage(DockerExecutorStagePullingImage)
	e.Println("Pulling docker image", imageName, "...")

	ref := imageName
	// Add :latest to limit the download results
	if !strings.ContainsAny(ref, ":@") {
		ref += ":latest"
	}

	options := types.ImagePullOptions{}
	if ac != nil {
		options.RegistryAuth, _ = docker_helpers.EncodeAuthConfig(ac)
	}

	errorRegexp := regexp.MustCompile("(repository does not exist|not found)")
	if err := e.client.ImagePullBlocking(e.Context, ref, options); err != nil {
		if errorRegexp.MatchString(err.Error()) {
			return nil, &common.BuildError{Inner: err}
		}
		return nil, err
	}

	image, _, err := e.client.ImageInspectWithRaw(e.Context, imageName)
	return &image, err
}

func (e *executor) getDockerImage(imageName string) (image *types.ImageInspect, err error) {
	pullPolicy, err := e.Config.Docker.PullPolicy.Get()
	if err != nil {
		return nil, err
	}

	authConfig := e.getAuthConfig(imageName)

	e.Debugln("Looking for image", imageName, "...")
	existingImage, _, err := e.client.ImageInspectWithRaw(e.Context, imageName)

	// Return early if we already used that image
	if err == nil && e.wasImageUsed(imageName, existingImage.ID) {
		return &existingImage, nil
	}

	defer func() {
		if err == nil {
			e.markImageAsUsed(imageName, image.ID)
		}
	}()

	// If never is specified then we return what inspect did return
	if pullPolicy == common.PullPolicyNever {
		return &existingImage, err
	}

	if err == nil {
		// Don't pull image that is passed by ID
		if existingImage.ID == imageName {
			return &existingImage, nil
		}

		// If not-present is specified
		if pullPolicy == common.PullPolicyIfNotPresent {
			e.Println("Using locally found image version due to if-not-present pull policy")
			return &existingImage, err
		}
	}

	return e.pullDockerImage(imageName, authConfig)
}

func (e *executor) expandAndGetDockerImage(imageName string, allowedImages []string) (*types.ImageInspect, error) {
	imageName, err := e.expandImageName(imageName, allowedImages)
	if err != nil {
		return nil, err
	}

	image, err := e.getDockerImage(imageName)
	if err != nil {
		return nil, err
	}

	return image, nil
}

func (e *executor) loadPrebuiltImage(path, ref, tag string) (*types.ImageInspect, error) {
	file, err := os.OpenFile(path, os.O_RDONLY, 0600)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, err
		}

		return nil, fmt.Errorf("Cannot load prebuilt image: %s: %q", path, err.Error())
	}
	defer file.Close()

	e.Debugln("Loading prebuilt image...")

	source := types.ImageImportSource{
		Source:     file,
		SourceName: "-",
	}
	options := types.ImageImportOptions{Tag: tag}

	if err := e.client.ImageImportBlocking(e.Context, source, ref, options); err != nil {
		return nil, fmt.Errorf("Failed to import image: %s", err)
	}

	image, _, err := e.client.ImageInspectWithRaw(e.Context, ref+":"+tag)
	if err != nil {
		e.Debugln("Inspecting imported image", ref, "failed:", err)
		return nil, err
	}

	return &image, err
}

func (e *executor) getPrebuiltImage() (*types.ImageInspect, error) {
	if imageNameFromConfig := e.Config.Docker.HelperImage; imageNameFromConfig != "" {
		imageNameFromConfig = common.AppVersion.Variables().ExpandValue(imageNameFromConfig)

		e.Debugln("Pull configured helper_image for predefined container instead of import bundled image", imageNameFromConfig, "...")
		if !e.Build.IsFeatureFlagOn(featureflags.FFDockerHelperImageV2) {
			e.Warningln("DEPRECATION: With gitlab-runner 12.0 we will change some tools inside the helper image, please make sure your image is compliant with the new API. https://gitlab.com/gitlab-org/gitlab-runner/issues/4013")
		}

		return e.getDockerImage(imageNameFromConfig)
	}

	helperImageInfo, err := helperimage.GetInfo(e.info)
	if err != nil {
		return nil, err
	}

	revision := "latest"
	if common.REVISION != "HEAD" {
		revision = common.REVISION
	}

	tag, err := helperImageInfo.Tag(revision)
	if err != nil {
		return nil, err
	}

	// Try to find already loaded prebuilt image
	imageName := fmt.Sprintf("%s:%s", prebuiltImageName, tag)
	e.Debugln("Looking for prebuilt image", imageName, "...")
	image, _, err := e.client.ImageInspectWithRaw(e.Context, imageName)
	if err == nil {
		return &image, nil
	}

	// Try to load prebuilt image from local filesystem
	loadedImage := e.getLocalDockerImage(helperImageInfo, tag)
	if loadedImage != nil {
		return loadedImage, nil
	}

	// Fallback to getting image from DockerHub
	e.Debugln("Loading image from registry:", imageName)
	return e.getDockerImage(imageName)
}

func (e *executor) getLocalDockerImage(helperImageInfo helperimage.Info, tag string) *types.ImageInspect {
	if !helperImageInfo.IsSupportingLocalImport() {
		return nil
	}

	architecture := helperImageInfo.Architecture()
	for _, dockerPrebuiltImagesPath := range DockerPrebuiltImagesPaths {
		dockerPrebuiltImageFilePath := filepath.Join(dockerPrebuiltImagesPath, "prebuilt-"+architecture+prebuiltImageExtension)
		image, err := e.loadPrebuiltImage(dockerPrebuiltImageFilePath, prebuiltImageName, tag)
		if err != nil {
			e.Debugln("Failed to load prebuilt image from:", dockerPrebuiltImageFilePath, "error:", err)
			continue
		}

		return image
	}

	return nil
}

func (e *executor) getBuildImage() (*types.ImageInspect, error) {
	imageName, err := e.expandImageName(e.Build.Image.Name, []string{})
	if err != nil {
		return nil, err
	}

	// Fetch image
	image, err := e.getDockerImage(imageName)
	if err != nil {
		return nil, err
	}

	return image, nil
}

func (e *executor) getAbsoluteContainerPath(dir string) string {
	if path.IsAbs(dir) {
		return dir
	}
	return path.Join(e.Build.FullProjectDir(), dir)
}

func (e *executor) addHostVolume(hostPath, containerPath string) error {
	containerPath = e.getAbsoluteContainerPath(containerPath)
	e.Debugln("Using host-based", hostPath, "for", containerPath, "...")
	e.binds = append(e.binds, fmt.Sprintf("%v:%v", hostPath, containerPath))
	return nil
}

func (e *executor) getLabels(containerType string, otherLabels ...string) map[string]string {
	labels := make(map[string]string)
	labels[dockerLabelPrefix+".job.id"] = strconv.Itoa(e.Build.ID)
	labels[dockerLabelPrefix+".job.sha"] = e.Build.GitInfo.Sha
	labels[dockerLabelPrefix+".job.before_sha"] = e.Build.GitInfo.BeforeSha
	labels[dockerLabelPrefix+".job.ref"] = e.Build.GitInfo.Ref
	labels[dockerLabelPrefix+".project.id"] = strconv.Itoa(e.Build.JobInfo.ProjectID)
	labels[dockerLabelPrefix+".runner.id"] = e.Build.Runner.ShortDescription()
	labels[dockerLabelPrefix+".runner.local_id"] = strconv.Itoa(e.Build.RunnerID)
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
func (e *executor) createCacheVolume(containerName, containerPath string) (string, error) {
	cacheImage, err := e.getPrebuiltImage()
	if err != nil {
		return "", err
	}

	cmd := []string{"gitlab-runner-helper", "cache-init", containerPath}
	// TODO: Remove in 12.0 to start using the command from `gitlab-runner-helper`
	if e.checkOutdatedHelperImage() {
		e.Debugln(featureflags.FFDockerHelperImageV2, "is not set, falling back to old command")
		cmd = []string{"gitlab-runner-cache", containerPath}
	}

	config := &container.Config{
		Image: cacheImage.ID,
		Cmd:   cmd,
		Volumes: map[string]struct{}{
			containerPath: {},
		},
		Labels: e.getLabels("cache", "cache.dir="+containerPath),
	}

	hostConfig := &container.HostConfig{
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
	}

	resp, err := e.client.ContainerCreate(e.Context, config, hostConfig, nil, containerName)
	if err != nil {
		if resp.ID != "" {
			e.temporary = append(e.temporary, resp.ID)
		}
		return "", err
	}

	e.Debugln("Starting cache container", resp.ID, "...")
	err = e.client.ContainerStart(e.Context, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		e.temporary = append(e.temporary, resp.ID)
		return "", err
	}

	e.Debugln("Waiting for cache container", resp.ID, "...")
	err = e.waitForContainer(e.Context, resp.ID)
	if err != nil {
		e.temporary = append(e.temporary, resp.ID)
		return "", err
	}

	return resp.ID, nil
}

func (e *executor) addCacheVolume(containerPath string) error {
	var err error
	containerPath = e.getAbsoluteContainerPath(containerPath)

	// disable cache for automatic container cache, but leave it for host volumes (they are shared on purpose)
	if e.Config.Docker.DisableCache {
		e.Debugln("Container cache for", containerPath, " is disabled.")
		return nil
	}

	hash := md5.Sum([]byte(containerPath))

	// use host-based cache
	if cacheDir := e.Config.Docker.CacheDir; cacheDir != "" {
		hostPath := fmt.Sprintf("%s/%s/%x", cacheDir, e.Build.ProjectUniqueName(), hash)
		hostPath, err := filepath.Abs(hostPath)
		if err != nil {
			return err
		}
		e.Debugln("Using path", hostPath, "as cache for", containerPath, "...")
		e.binds = append(e.binds, fmt.Sprintf("%v:%v", filepath.ToSlash(hostPath), containerPath))
		return nil
	}

	// get existing cache container
	var containerID string
	containerName := fmt.Sprintf("%s-cache-%x", e.Build.ProjectUniqueName(), hash)
	if inspected, err := e.client.ContainerInspect(e.Context, containerName); err == nil {
		// check if we have valid cache, if not remove the broken container
		if _, ok := inspected.Config.Volumes[containerPath]; !ok {
			e.Debugln("Removing broken cache container for ", containerPath, "path")
			e.removeContainer(e.Context, inspected.ID)
		} else {
			containerID = inspected.ID
		}
	}

	// create new cache container for that project
	if containerID == "" {
		containerID, err = e.createCacheVolume(containerName, containerPath)
		if err != nil {
			return err
		}
	}

	e.Debugln("Using container", containerID, "as cache", containerPath, "...")
	e.caches = append(e.caches, containerID)
	return nil
}

func (e *executor) addVolume(volume string) error {
	var err error
	hostVolume := strings.SplitN(volume, ":", 2)
	switch len(hostVolume) {
	case 2:
		err = e.addHostVolume(hostVolume[0], hostVolume[1])

	case 1:
		// disable cache disables
		err = e.addCacheVolume(hostVolume[0])
	}

	if err != nil {
		e.Errorln("Failed to create container volume for", volume, err)
	}
	return err
}

func fakeContainer(id string, names ...string) *types.Container {
	return &types.Container{ID: id, Names: names}
}

func (e *executor) createBuildVolume() error {
	parentDir := e.Build.RootDir

	if e.Build.IsFeatureFlagOn(featureflags.FFUseLegacyBuildsDirForDocker) {
		// Cache Git sources:
		// take path of the projects directory,
		// because we use `rm -rf` which could remove the mounted volume
		parentDir = path.Dir(e.Build.FullProjectDir())
	}

	if !path.IsAbs(parentDir) && parentDir != "/" {
		return common.MakeBuildError("build directory needs to be absolute and non-root path")
	}

	if e.isHostMountedVolume(e.Build.RootDir, e.Config.Docker.Volumes...) {
		return nil
	}

	if e.Build.GetGitStrategy() == common.GitFetch && !e.Config.Docker.DisableCache {
		// create persistent cache container
		return e.addVolume(parentDir)
	}

	// create temporary cache container
	id, err := e.createCacheVolume("", parentDir)
	if err != nil {
		return err
	}

	e.caches = append(e.caches, id)
	e.temporary = append(e.temporary, id)

	return nil
}

func (e *executor) createUserVolumes() (err error) {
	for _, volume := range e.Config.Docker.Volumes {
		err = e.addVolume(volume)
		if err != nil {
			return
		}
	}
	return nil
}

func (e *executor) isHostMountedVolume(dir string, volumes ...string) bool {
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

func (e *executor) parseDeviceString(deviceString string) (device container.DeviceMapping, err error) {
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

func (e *executor) bindDevices() (err error) {
	for _, deviceString := range e.Config.Docker.Devices {
		device, err := e.parseDeviceString(deviceString)
		if err != nil {
			err = fmt.Errorf("Failed to parse device string %q: %s", deviceString, err)
			return err
		}

		e.devices = append(e.devices, device)
	}
	return nil
}

func (e *executor) wasImageUsed(imageName, imageID string) bool {
	e.usedImagesLock.RLock()
	defer e.usedImagesLock.RUnlock()

	if e.usedImages[imageName] == imageID {
		return true
	}
	return false
}

func (e *executor) markImageAsUsed(imageName, imageID string) {
	e.usedImagesLock.Lock()
	defer e.usedImagesLock.Unlock()

	if e.usedImages == nil {
		e.usedImages = make(map[string]string)
	}
	e.usedImages[imageName] = imageID

	if imageName != imageID {
		e.Println("Using docker image", imageID, "for", imageName, "...")
	}
}

func (e *executor) splitServiceAndVersion(serviceDescription string) (service, version, imageName string, linkNames []string) {
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

func (e *executor) createService(serviceIndex int, service, version, image string, serviceDefinition common.Image) (*types.Container, error) {
	if len(service) == 0 {
		return nil, errors.New("invalid service name")
	}

	e.Println("Starting service", service+":"+version, "...")
	serviceImage, err := e.getDockerImage(image)
	if err != nil {
		return nil, err
	}

	serviceSlug := strings.Replace(service, "/", "__", -1)
	containerName := fmt.Sprintf("%s-%s-%d", e.Build.ProjectUniqueName(), serviceSlug, serviceIndex)

	// this will fail potentially some builds if there's name collision
	e.removeContainer(e.Context, containerName)

	config := &container.Config{
		Image:  serviceImage.ID,
		Labels: e.getLabels("service", "service="+service, "service.version="+version),
		Env:    e.getServiceVariables(),
	}

	if len(serviceDefinition.Command) > 0 {
		config.Cmd = serviceDefinition.Command
	}
	config.Entrypoint = e.overwriteEntrypoint(&serviceDefinition)

	hostConfig := &container.HostConfig{
		DNS:           e.Config.Docker.DNS,
		DNSSearch:     e.Config.Docker.DNSSearch,
		RestartPolicy: neverRestartPolicy,
		ExtraHosts:    e.Config.Docker.ExtraHosts,
		Privileged:    e.Config.Docker.Privileged,
		NetworkMode:   container.NetworkMode(e.Config.Docker.NetworkMode),
		Binds:         e.binds,
		ShmSize:       e.Config.Docker.ShmSize,
		VolumesFrom:   e.caches,
		Tmpfs:         e.Config.Docker.ServicesTmpfs,
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
	}

	e.Debugln("Creating service container", containerName, "...")
	resp, err := e.client.ContainerCreate(e.Context, config, hostConfig, nil, containerName)
	if err != nil {
		return nil, err
	}

	e.Debugln("Starting service container", resp.ID, "...")
	err = e.client.ContainerStart(e.Context, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		e.temporary = append(e.temporary, resp.ID)
		return nil, err
	}

	return fakeContainer(resp.ID, containerName), nil
}

func (e *executor) getServicesDefinitions() (common.Services, error) {
	serviceDefinitions := common.Services{}
	for _, service := range e.Config.Docker.Services {
		serviceDefinitions = append(serviceDefinitions, common.Image{Name: service})
	}

	for _, service := range e.Build.Services {
		serviceName := e.Build.GetAllVariables().ExpandValue(service.Name)
		err := e.verifyAllowedImage(serviceName, "services", e.Config.Docker.AllowedServices, e.Config.Docker.Services)
		if err != nil {
			return nil, err
		}

		service.Name = serviceName
		serviceDefinitions = append(serviceDefinitions, service)
	}

	return serviceDefinitions, nil
}

func (e *executor) waitForServices() {
	waitForServicesTimeout := e.Config.Docker.WaitForServicesTimeout
	if waitForServicesTimeout == 0 {
		waitForServicesTimeout = common.DefaultWaitForServicesTimeout
	}

	// wait for all services to came up
	if waitForServicesTimeout > 0 && len(e.services) > 0 {
		e.Println("Waiting for services to be up and running...")
		wg := sync.WaitGroup{}
		for _, service := range e.services {
			wg.Add(1)
			go func(service *types.Container) {
				e.waitForServiceContainer(service, time.Duration(waitForServicesTimeout)*time.Second)
				wg.Done()
			}(service)
		}
		wg.Wait()
	}
}

func (e *executor) buildServiceLinks(linksMap map[string]*types.Container) (links []string) {
	for linkName, linkee := range linksMap {
		newContainer, err := e.client.ContainerInspect(e.Context, linkee.ID)
		if err != nil {
			continue
		}
		if newContainer.State.Running {
			links = append(links, linkee.ID+":"+linkName)
		}
	}
	return
}

func (e *executor) createFromServiceDefinition(serviceIndex int, serviceDefinition common.Image, linksMap map[string]*types.Container) (err error) {
	var container *types.Container

	service, version, imageName, linkNames := e.splitServiceAndVersion(serviceDefinition.Name)

	if serviceDefinition.Alias != "" {
		linkNames = append(linkNames, serviceDefinition.Alias)
	}

	for _, linkName := range linkNames {
		if linksMap[linkName] != nil {
			e.Warningln("Service", serviceDefinition.Name, "is already created. Ignoring.")
			continue
		}

		// Create service if not yet created
		if container == nil {
			container, err = e.createService(serviceIndex, service, version, imageName, serviceDefinition)
			if err != nil {
				return
			}
			e.Debugln("Created service", serviceDefinition.Name, "as", container.ID)
			e.services = append(e.services, container)
			e.temporary = append(e.temporary, container.ID)
		}
		linksMap[linkName] = container
	}
	return
}

func (e *executor) createServices() (err error) {
	servicesDefinitions, err := e.getServicesDefinitions()
	if err != nil {
		return
	}

	linksMap := make(map[string]*types.Container)

	for index, serviceDefinition := range servicesDefinitions {
		err = e.createFromServiceDefinition(index, serviceDefinition, linksMap)
		if err != nil {
			return
		}
	}

	e.waitForServices()

	e.links = e.buildServiceLinks(linksMap)
	return
}

func (e *executor) getValidContainers(containers []string) []string {
	var newContainers []string

	for _, container := range containers {
		if _, err := e.client.ContainerInspect(e.Context, container); err == nil {
			newContainers = append(newContainers, container)
		}
	}

	return newContainers
}

func (e *executor) createContainer(containerType string, imageDefinition common.Image, cmd []string, allowedInternalImages []string) (*types.ContainerJSON, error) {
	image, err := e.expandAndGetDockerImage(imageDefinition.Name, allowedInternalImages)
	if err != nil {
		return nil, err
	}

	hostname := e.Config.Docker.Hostname
	if hostname == "" {
		hostname = e.Build.ProjectUniqueName()
	}

	// Always create unique, but sequential name
	containerIndex := len(e.builds)
	containerName := e.Build.ProjectUniqueName() + "-" +
		containerType + "-" + strconv.Itoa(containerIndex)

	config := &container.Config{
		Image:        image.ID,
		Hostname:     hostname,
		Cmd:          cmd,
		Labels:       e.getLabels(containerType),
		Tty:          false,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    true,
		StdinOnce:    true,
		Env:          append(e.Build.GetAllVariables().StringList(), e.BuildShell.Environment...),
	}

	config.Entrypoint = e.overwriteEntrypoint(&imageDefinition)

	nanoCPUs, err := e.Config.Docker.GetNanoCPUs()
	if err != nil {
		return nil, err
	}

	// By default we use caches container,
	// but in later phases we hook to previous build container
	volumesFrom := e.caches
	if len(e.builds) > 0 {
		volumesFrom = []string{
			e.builds[len(e.builds)-1],
		}
	}

	hostConfig := &container.HostConfig{
		Resources: container.Resources{
			Memory:            e.Config.Docker.GetMemory(),
			MemorySwap:        e.Config.Docker.GetMemorySwap(),
			MemoryReservation: e.Config.Docker.GetMemoryReservation(),
			CpusetCpus:        e.Config.Docker.CPUSetCPUs,
			NanoCPUs:          nanoCPUs,
			Devices:           e.devices,
			OomKillDisable:    e.Config.Docker.GetOomKillDisable(),
		},
		DNS:           e.Config.Docker.DNS,
		DNSSearch:     e.Config.Docker.DNSSearch,
		Runtime:       e.Config.Docker.Runtime,
		Privileged:    e.Config.Docker.Privileged,
		UsernsMode:    container.UsernsMode(e.Config.Docker.UsernsMode),
		CapAdd:        e.Config.Docker.CapAdd,
		CapDrop:       e.Config.Docker.CapDrop,
		SecurityOpt:   e.Config.Docker.SecurityOpt,
		RestartPolicy: neverRestartPolicy,
		ExtraHosts:    e.Config.Docker.ExtraHosts,
		NetworkMode:   container.NetworkMode(e.Config.Docker.NetworkMode),
		Links:         append(e.Config.Docker.Links, e.links...),
		Binds:         e.binds,
		ShmSize:       e.Config.Docker.ShmSize,
		VolumeDriver:  e.Config.Docker.VolumeDriver,
		VolumesFrom:   append(e.Config.Docker.VolumesFrom, volumesFrom...),
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
		Tmpfs:   e.Config.Docker.Tmpfs,
		Sysctls: e.Config.Docker.SysCtls,
	}

	// this will fail potentially some builds if there's name collision
	e.removeContainer(e.Context, containerName)

	e.Debugln("Creating container", containerName, "...")
	resp, err := e.client.ContainerCreate(e.Context, config, hostConfig, nil, containerName)
	if err != nil {
		if resp.ID != "" {
			e.temporary = append(e.temporary, resp.ID)
		}
		return nil, err
	}

	inspect, err := e.client.ContainerInspect(e.Context, resp.ID)
	if err != nil {
		e.temporary = append(e.temporary, resp.ID)
		return nil, err
	}

	e.builds = append(e.builds, resp.ID)
	e.temporary = append(e.temporary, resp.ID)
	return &inspect, nil
}

func (e *executor) killContainer(id string, waitCh chan error) (err error) {
	for {
		e.disconnectNetwork(e.Context, id)
		e.Debugln("Killing container", id, "...")
		e.client.ContainerKill(e.Context, id, "SIGKILL")

		// Wait for signal that container were killed
		// or retry after some time
		select {
		case err = <-waitCh:
			return

		case <-time.After(time.Second):
		}
	}
}

func (e *executor) waitForContainer(ctx context.Context, id string) error {
	e.Debugln("Waiting for container", id, "...")

	retries := 0

	// Use active wait
	for ctx.Err() == nil {
		container, err := e.client.ContainerInspect(ctx, id)
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

	return ctx.Err()
}

func (e *executor) watchContainer(ctx context.Context, id string, input io.Reader) (err error) {
	options := types.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	}

	e.Debugln("Attaching to container", id, "...")
	hijacked, err := e.client.ContainerAttach(ctx, id, options)
	if err != nil {
		return
	}
	defer hijacked.Close()

	e.Debugln("Starting container", id, "...")
	err = e.client.ContainerStart(ctx, id, types.ContainerStartOptions{})
	if err != nil {
		return
	}

	e.Debugln("Waiting for attach to finish", id, "...")
	attachCh := make(chan error, 2)

	// Copy any output to the build trace
	go func() {
		_, err := stdcopy.StdCopy(e.Trace, e.Trace, hijacked.Reader)
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
		waitCh <- e.waitForContainer(e.Context, id)
	}()

	select {
	case <-ctx.Done():
		e.killContainer(id, waitCh)
		err = errors.New("Aborted")

	case err = <-attachCh:
		e.killContainer(id, waitCh)
		e.Debugln("Container", id, "finished with", err)

	case err = <-waitCh:
		e.Debugln("Container", id, "finished with", err)
	}
	return
}

func (e *executor) removeContainer(ctx context.Context, id string) error {
	e.disconnectNetwork(ctx, id)
	options := types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}
	err := e.client.ContainerRemove(ctx, id, options)
	e.Debugln("Removed container", id, "with", err)
	return err
}

func (e *executor) disconnectNetwork(ctx context.Context, id string) error {
	netList, err := e.client.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		e.Debugln("Can't get network list. ListNetworks exited with", err)
		return err
	}

	for _, network := range netList {
		for _, pluggedContainer := range network.Containers {
			if id == pluggedContainer.Name {
				err = e.client.NetworkDisconnect(ctx, network.ID, id, true)
				if err != nil {
					e.Warningln("Can't disconnect possibly zombie container", pluggedContainer.Name, "from network", network.Name, "->", err)
				} else {
					e.Warningln("Possibly zombie container", pluggedContainer.Name, "is disconnected from network", network.Name)
				}
				break
			}
		}
	}
	return err
}

func (e *executor) verifyAllowedImage(image, optionName string, allowedImages []string, internalImages []string) error {
	for _, allowedImage := range allowedImages {
		ok, _ := zglob.Match(allowedImage, image)
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
		e.Println()
		e.Errorln("The", image, "is not present on list of allowed", optionName)
		for _, allowedImage := range allowedImages {
			e.Println("-", allowedImage)
		}
		e.Println()
	} else {
		// by default allow to override the image name
		return nil
	}

	e.Println("Please check runner's configuration: http://doc.gitlab.com/ci/docker/using_docker_images.html#overwrite-image-and-services")
	return errors.New("invalid image")
}

func (e *executor) expandImageName(imageName string, allowedInternalImages []string) (string, error) {
	if imageName != "" {
		image := e.Build.GetAllVariables().ExpandValue(imageName)
		allowedInternalImages = append(allowedInternalImages, e.Config.Docker.Image)
		err := e.verifyAllowedImage(image, "images", e.Config.Docker.AllowedImages, allowedInternalImages)
		if err != nil {
			return "", err
		}
		return image, nil
	}

	if e.Config.Docker.Image == "" {
		return "", errors.New("No Docker image specified to run the build in")
	}

	return e.Config.Docker.Image, nil
}

func (e *executor) overwriteEntrypoint(image *common.Image) []string {
	if len(image.Entrypoint) > 0 {
		if !e.Config.Docker.DisableEntrypointOverwrite {
			return image.Entrypoint
		}

		e.Warningln("Entrypoint override disabled")
	}

	return nil
}

func (e *executor) connectDocker() (err error) {
	client, err := docker_helpers.New(e.Config.Docker.DockerCredentials, "")
	if err != nil {
		return err
	}
	e.client = client

	e.info, err = client.Info(e.Context)
	if err != nil {
		return err
	}

	return
}

func (e *executor) createDependencies() (err error) {
	err = e.bindDevices()
	if err != nil {
		return err
	}

	e.SetCurrentStage(DockerExecutorStageCreatingBuildVolumes)
	e.Debugln("Creating build volume...")
	err = e.createBuildVolume()
	if err != nil {
		return err
	}

	e.SetCurrentStage(DockerExecutorStageCreatingServices)
	e.Debugln("Creating services...")
	err = e.createServices()
	if err != nil {
		return err
	}

	e.SetCurrentStage(DockerExecutorStageCreatingUserVolumes)
	e.Debugln("Creating user-defined volumes...")
	err = e.createUserVolumes()
	if err != nil {
		return err
	}

	return
}

func (e *executor) Prepare(options common.ExecutorPrepareOptions) error {
	err := e.prepareBuildsDir(options.Config)
	if err != nil {
		return err
	}

	err = e.AbstractExecutor.Prepare(options)
	if err != nil {
		return err
	}

	if e.BuildShell.PassFile {
		return errors.New("Docker doesn't support shells that require script file")
	}

	if options.Config.Docker == nil {
		return errors.New("Missing docker configuration")
	}

	e.SetCurrentStage(DockerExecutorStagePrepare)
	imageName, err := e.expandImageName(e.Build.Image.Name, []string{})
	if err != nil {
		return err
	}

	e.Println("Using Docker executor with image", imageName, "...")

	err = e.connectDocker()
	if err != nil {
		return err
	}

	err = e.createDependencies()
	if err != nil {
		return err
	}
	return nil
}

func (e *executor) prepareBuildsDir(config *common.RunnerConfig) error {
	rootDir := config.BuildsDir
	if rootDir == "" {
		rootDir = e.DefaultBuildsDir
	}
	if e.isHostMountedVolume(rootDir, config.Docker.Volumes...) {
		e.SharedBuildsDir = true
	}
	return nil
}

func (e *executor) Cleanup() {
	e.SetCurrentStage(DockerExecutorStageCleanup)

	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(context.Background(), dockerCleanupTimeout)
	defer cancel()

	remove := func(id string) {
		wg.Add(1)
		go func() {
			e.removeContainer(ctx, id)
			wg.Done()
		}()
	}

	for _, temporaryID := range e.temporary {
		remove(temporaryID)
	}

	wg.Wait()

	if e.client != nil {
		e.client.Close()
	}

	e.AbstractExecutor.Cleanup()
}

type serviceHealthCheckError struct {
	Inner error
	Logs  string
}

func (e *serviceHealthCheckError) Error() string {
	if e.Inner == nil {
		return "serviceHealthCheckError"
	}

	return e.Inner.Error()
}

func (e *executor) runServiceHealthCheckContainer(service *types.Container, timeout time.Duration) error {
	waitImage, err := e.getPrebuiltImage()
	if err != nil {
		return fmt.Errorf("getPrebuiltImage: %v", err)
	}

	containerName := service.Names[0] + "-wait-for-service"

	cmd := []string{"gitlab-runner-helper", "health-check"}
	// TODO: Remove in 12.0 to start using the command from `gitlab-runner-helper`
	if e.checkOutdatedHelperImage() {
		e.Debugln(featureflags.FFDockerHelperImageV2, "is not set, falling back to old command")
		cmd = []string{"gitlab-runner-service"}
	}

	config := &container.Config{
		Cmd:    cmd,
		Image:  waitImage.ID,
		Labels: e.getLabels("wait", "wait="+service.ID),
	}
	hostConfig := &container.HostConfig{
		RestartPolicy: neverRestartPolicy,
		Links:         []string{service.Names[0] + ":service"},
		NetworkMode:   container.NetworkMode(e.Config.Docker.NetworkMode),
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
	}
	e.Debugln("Waiting for service container", containerName, "to be up and running...")
	resp, err := e.client.ContainerCreate(e.Context, config, hostConfig, nil, containerName)
	if err != nil {
		return fmt.Errorf("ContainerCreate: %v", err)
	}
	defer e.removeContainer(e.Context, resp.ID)
	err = e.client.ContainerStart(e.Context, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("ContainerStart: %v", err)
	}

	waitResult := make(chan error, 1)
	go func() {
		waitResult <- e.waitForContainer(e.Context, resp.ID)
	}()

	// these are warnings and they don't make the build fail
	select {
	case err := <-waitResult:
		if err == nil {
			return nil
		}

		return &serviceHealthCheckError{
			Inner: err,
			Logs:  e.readContainerLogs(resp.ID),
		}
	case <-time.After(timeout):
		return &serviceHealthCheckError{
			Inner: fmt.Errorf("service %q timeout", containerName),
			Logs:  e.readContainerLogs(resp.ID),
		}
	}
}

func (e *executor) waitForServiceContainer(service *types.Container, timeout time.Duration) error {
	err := e.runServiceHealthCheckContainer(service, timeout)
	if err == nil {
		return nil
	}

	var buffer bytes.Buffer
	buffer.WriteString("\n")
	buffer.WriteString(helpers.ANSI_YELLOW + "*** WARNING:" + helpers.ANSI_RESET + " Service " + service.Names[0] + " probably didn't start properly.\n")
	buffer.WriteString("\n")
	buffer.WriteString("Health check error:\n")
	buffer.WriteString(strings.TrimSpace(err.Error()))
	buffer.WriteString("\n")

	if healtCheckErr, ok := err.(*serviceHealthCheckError); ok {
		buffer.WriteString("\n")
		buffer.WriteString("Health check container logs:\n")
		buffer.WriteString(healtCheckErr.Logs)
		buffer.WriteString("\n")
	}

	buffer.WriteString("\n")
	buffer.WriteString("Service container logs:\n")
	buffer.WriteString(e.readContainerLogs(service.ID))
	buffer.WriteString("\n")

	buffer.WriteString("\n")
	buffer.WriteString(helpers.ANSI_YELLOW + "*********" + helpers.ANSI_RESET + "\n")
	buffer.WriteString("\n")
	io.Copy(e.Trace, &buffer)
	return err
}

func (e *executor) readContainerLogs(containerID string) string {
	var containerBuffer bytes.Buffer

	options := types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
	}

	hijacked, err := e.client.ContainerLogs(e.Context, containerID, options)
	if err != nil {
		return strings.TrimSpace(err.Error())
	}
	defer hijacked.Close()

	stdcopy.StdCopy(&containerBuffer, &containerBuffer, hijacked)
	containerLog := containerBuffer.String()
	return strings.TrimSpace(containerLog)
}

func (e *executor) checkOutdatedHelperImage() bool {
	return !e.Build.IsFeatureFlagOn(featureflags.FFDockerHelperImageV2) && e.Config.Docker.HelperImage != ""
}
