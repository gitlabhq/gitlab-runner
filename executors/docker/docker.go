package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/kardianos/osext"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/labels"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/networks"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/permission"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/wait"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/helperimage"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/services"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/auth"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
)

const (
	ExecutorStagePrepare common.ExecutorStage = "docker_prepare"
	ExecutorStageRun     common.ExecutorStage = "docker_run"
	ExecutorStageCleanup common.ExecutorStage = "docker_cleanup"

	ExecutorStageCreatingBuildVolumes common.ExecutorStage = "docker_creating_build_volumes"
	ExecutorStageCreatingServices     common.ExecutorStage = "docker_creating_services"
	ExecutorStageCreatingUserVolumes  common.ExecutorStage = "docker_creating_user_volumes"
	ExecutorStagePullingImage         common.ExecutorStage = "docker_pulling_image"
)

var PrebuiltImagesPaths []string

const (
	labelServiceType = "service"
	labelWaitType    = "wait"
)

var neverRestartPolicy = container.RestartPolicy{Name: "no"}

var (
	errVolumesManagerUndefined  = errors.New("volumesManager is undefined")
	errNetworksManagerUndefined = errors.New("networksManager is undefined")
)

type executor struct {
	executors.AbstractExecutor
	client                    docker.Client
	volumeParser              parser.Parser
	newVolumePermissionSetter func() (permission.Setter, error)
	info                      types.Info
	waiter                    wait.KillWaiter

	temporary []string // IDs of containers that should be removed

	builds   []string // IDs of successfully created build containers
	services []*types.Container

	links []string

	devices []container.DeviceMapping

	helperImageInfo helperimage.Info

	usedImages     map[string]string
	usedImagesLock sync.RWMutex

	volumesManager  volumes.Manager
	networksManager networks.Manager
	labeler         labels.Labeler

	networkMode container.NetworkMode

	projectUniqRandomizedName string
}

func init() {
	runnerFolder, err := osext.ExecutableFolder()
	if err != nil {
		logrus.Errorln(
			"Docker executor: unable to detect gitlab-runner folder, "+
				"prebuilt image helpers will be loaded from DockerHub.",
			err,
		)
	}

	PrebuiltImagesPaths = []string{
		filepath.Join(runnerFolder, "helper-images"),
		filepath.Join(runnerFolder, "out/helper-images"),
	}
	if runtime.GOOS == "linux" {
		// This section covers the Linux packaged app scenario, with the binary in /usr/bin.
		// The helper images are located in /usr/lib/gitlab-runner/helper-images,
		// as part of the packaging done in the create_package function in ci/package
		PrebuiltImagesPaths = append(
			PrebuiltImagesPaths,
			filepath.Join(runnerFolder, "../lib/gitlab-runner/helper-images"),
		)
	}
}

func (e *executor) getServiceVariables() []string {
	return e.Build.GetAllVariables().PublicOrInternal().StringList()
}

func (e *executor) pullDockerImage(imageName string, ac *types.AuthConfig) (*types.ImageInspect, error) {
	e.SetCurrentStage(ExecutorStagePullingImage)
	e.Println("Pulling docker image", imageName, "...")

	ref := imageName
	// Add :latest to limit the download results
	if !strings.ContainsAny(ref, ":@") {
		ref += ":latest"
	}

	options := types.ImagePullOptions{}
	options.RegistryAuth, _ = auth.EncodeConfig(ac)

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

	e.Debugln("Looking for image", imageName, "...")
	existingImage, _, err := e.client.ImageInspectWithRaw(e.Context, imageName)

	// Return early if we already used that image
	if err == nil && e.wasImageUsed(imageName, existingImage.ID) {
		return &existingImage, nil
	}

	defer func() {
		if err == nil {
			e.markImageAsUsed(imageName, image)
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

	registryInfo, err := auth.ResolveConfigForImage(
		imageName,
		e.Build.GetDockerAuthConfig(),
		e.Shell().User,
		e.Build.Credentials,
	)
	if err != nil {
		return nil, err
	}

	if registryInfo != nil {
		e.Println(fmt.Sprintf("Authenticating with credentials from %v", registryInfo.Source))
		e.Debugln(fmt.Sprintf(
			"Using %v to connect to %v in order to resolve %v...",
			registryInfo.AuthConfig.Username,
			registryInfo.AuthConfig.ServerAddress,
			imageName,
		))
		return e.pullDockerImage(imageName, &registryInfo.AuthConfig)
	}

	e.Debugln(fmt.Sprintf("No credentials found for %v", imageName))
	return e.pullDockerImage(imageName, nil)
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

		return nil, fmt.Errorf("cannot load prebuilt image: %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	e.Debugln("Loading prebuilt image...")

	source := types.ImageImportSource{
		Source:     file,
		SourceName: "-",
	}
	options := types.ImageImportOptions{Tag: tag}

	// TODO: Remove check in 14.0 https://gitlab.com/gitlab-org/gitlab-runner/-/issues/26679
	if e.Build.IsFeatureFlagOn(featureflags.ResetHelperImageEntrypoint) {
		// NOTE: The ENTRYPOINT metadata is not preserved on export, so we need to reapply this metadata on import.
		// See https://gitlab.com/gitlab-org/gitlab-runner/-/merge_requests/2058#note_388341301
		options.Changes = append(options.Changes, `ENTRYPOINT ["/usr/bin/dumb-init", "/entrypoint"]`)
	}

	if err = e.client.ImageImportBlocking(e.Context, source, ref, options); err != nil {
		return nil, fmt.Errorf("failed to import image: %w", err)
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

		e.Debugln(
			"Pull configured helper_image for predefined container instead of import bundled image",
			imageNameFromConfig,
			"...",
		)

		return e.getDockerImage(imageNameFromConfig)
	}

	e.Debugln(fmt.Sprintf("Looking for prebuilt image %s...", e.helperImageInfo))
	image, _, err := e.client.ImageInspectWithRaw(e.Context, e.helperImageInfo.String())
	if err == nil {
		return &image, nil
	}

	// Try to load prebuilt image from local filesystem
	loadedImage := e.getLocalHelperImage()
	if loadedImage != nil {
		return loadedImage, nil
	}

	// Fallback to getting image from DockerHub
	e.Debugln(fmt.Sprintf("Loading image form registry: %s", e.helperImageInfo))
	return e.getDockerImage(e.helperImageInfo.String())
}

func (e *executor) getLocalHelperImage() *types.ImageInspect {
	if !e.helperImageInfo.IsSupportingLocalImport {
		return nil
	}

	architecture := e.helperImageInfo.Architecture
	for _, dockerPrebuiltImagesPath := range PrebuiltImagesPaths {
		dockerPrebuiltImageFilePath := filepath.Join(
			dockerPrebuiltImagesPath,
			"prebuilt-"+architecture+prebuiltImageExtension,
		)
		image, err := e.loadPrebuiltImage(dockerPrebuiltImageFilePath, prebuiltImageName, e.helperImageInfo.Tag)
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

func fakeContainer(id string, names ...string) *types.Container {
	return &types.Container{ID: id, Names: names}
}

func (e *executor) parseDeviceString(deviceString string) (device container.DeviceMapping, err error) {
	// Split the device string PathOnHost[:PathInContainer[:CgroupPermissions]]
	parts := strings.Split(deviceString, ":")

	if len(parts) > 3 {
		err = fmt.Errorf("too many colons")
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
			err = fmt.Errorf("failed to parse device string %q: %w", deviceString, err)
			return err
		}

		e.devices = append(e.devices, device)
	}
	return nil
}

func (e *executor) wasImageUsed(imageName, imageID string) bool {
	e.usedImagesLock.RLock()
	defer e.usedImagesLock.RUnlock()

	return e.usedImages[imageName] == imageID
}

func (e *executor) markImageAsUsed(imageName string, image *types.ImageInspect) {
	e.usedImagesLock.Lock()
	defer e.usedImagesLock.Unlock()

	if e.usedImages == nil {
		e.usedImages = make(map[string]string)
	}
	e.usedImages[imageName] = image.ID

	if imageName != image.ID && len(image.RepoDigests) > 0 {
		e.Println("Using docker image", image.ID, "for", imageName, "with digest", image.RepoDigests[0], "...")
	} else if imageName != image.ID {
		e.Println("Using docker image", image.ID, "for", imageName, "...")
	}
}

func (e *executor) createService(
	serviceIndex int,
	service, version, image string,
	serviceDefinition common.Image,
	linkNames []string,
) (*types.Container, error) {
	if service == "" {
		return nil, fmt.Errorf("invalid service name: %s", serviceDefinition.Name)
	}

	if e.volumesManager == nil {
		return nil, errVolumesManagerUndefined
	}

	e.Println("Starting service", service+":"+version, "...")
	serviceImage, err := e.getDockerImage(image)
	if err != nil {
		return nil, err
	}

	serviceSlug := strings.ReplaceAll(service, "/", "__")
	containerName := fmt.Sprintf("%s-%s-%d", e.getProjectUniqRandomizedName(), serviceSlug, serviceIndex)

	// this will fail potentially some builds if there's name collision
	_ = e.removeContainer(e.Context, containerName)

	labels := map[string]string{
		"type":            labelServiceType,
		"service":         service,
		"service.version": version,
	}

	config := &container.Config{
		Image:  serviceImage.ID,
		Labels: e.labeler.Labels(labels),
		Env:    append(e.getServiceVariables(), e.BuildShell.Environment...),
	}

	if len(serviceDefinition.Command) > 0 {
		config.Cmd = serviceDefinition.Command
	}
	config.Entrypoint = e.overwriteEntrypoint(&serviceDefinition)

	hostConfig := e.createHostConfigForService()
	networkConfig := e.networkConfig(linkNames)

	e.Debugln("Creating service container", containerName, "...")
	resp, err := e.client.ContainerCreate(e.Context, config, hostConfig, networkConfig, containerName)
	if err != nil {
		return nil, err
	}

	e.Debugln(fmt.Sprintf("Starting service container %s (%s)...", containerName, resp.ID))
	err = e.client.ContainerStart(e.Context, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		e.temporary = append(e.temporary, resp.ID)
		return nil, err
	}

	return fakeContainer(resp.ID, containerName), nil
}

func (e *executor) createHostConfigForService() *container.HostConfig {
	return &container.HostConfig{
		DNS:           e.Config.Docker.DNS,
		DNSSearch:     e.Config.Docker.DNSSearch,
		RestartPolicy: neverRestartPolicy,
		ExtraHosts:    e.Config.Docker.ExtraHosts,
		Privileged:    e.Config.Docker.Privileged,
		UsernsMode:    container.UsernsMode(e.Config.Docker.UsernsMode),
		NetworkMode:   e.networkMode,
		Binds:         e.volumesManager.Binds(),
		ShmSize:       e.Config.Docker.ShmSize,
		Tmpfs:         e.Config.Docker.ServicesTmpfs,
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
	}
}

func (e *executor) networkConfig(aliases []string) *network.NetworkingConfig {
	if e.networkMode.UserDefined() == "" {
		return &network.NetworkingConfig{}
	}

	return &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			e.networkMode.UserDefined(): {Aliases: aliases},
		},
	}
}

func (e *executor) getProjectUniqRandomizedName() string {
	if e.projectUniqRandomizedName == "" {
		uuid, _ := helpers.GenerateRandomUUID(8)
		e.projectUniqRandomizedName = fmt.Sprintf("%s-%s", e.Build.ProjectUniqueName(), uuid)
	}

	return e.projectUniqRandomizedName
}

func (e *executor) getServicesDefinitions() (common.Services, error) {
	var internalServiceImages []string
	serviceDefinitions := common.Services{}

	for _, service := range e.Config.Docker.Services {
		internalServiceImages = append(internalServiceImages, service.Name)
		serviceDefinitions = append(serviceDefinitions, service.ToImageDefinition())
	}

	for _, service := range e.Build.Services {
		serviceName := e.Build.GetAllVariables().ExpandValue(service.Name)
		err := e.verifyAllowedImage(serviceName, "services", e.Config.Docker.AllowedServices, internalServiceImages)
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
				_ = e.waitForServiceContainer(service, time.Duration(waitForServicesTimeout)*time.Second)
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

func (e *executor) createFromServiceDefinition(
	serviceIndex int,
	serviceDefinition common.Image,
	linksMap map[string]*types.Container,
) error {
	var container *types.Container

	serviceMeta := services.SplitNameAndVersion(serviceDefinition.Name)

	if serviceDefinition.Alias != "" {
		serviceMeta.Aliases = append(serviceMeta.Aliases, serviceDefinition.Alias)
	}

	for _, linkName := range serviceMeta.Aliases {
		if linksMap[linkName] != nil {
			e.Warningln("Service", serviceDefinition.Name, "is already created. Ignoring.")
			continue
		}

		// Create service if not yet created
		if container == nil {
			var err error
			container, err = e.createService(
				serviceIndex,
				serviceMeta.Service,
				serviceMeta.Version,
				serviceMeta.ImageName,
				serviceDefinition,
				serviceMeta.Aliases,
			)
			if err != nil {
				return err
			}

			e.Debugln("Created service", serviceDefinition.Name, "as", container.ID)
			e.services = append(e.services, container)
			e.temporary = append(e.temporary, container.ID)
		}
		linksMap[linkName] = container
	}
	return nil
}

func (e *executor) createBuildNetwork() error {
	if e.networksManager == nil {
		return errNetworksManagerUndefined
	}

	networkMode, err := e.networksManager.Create(e.Context, e.Config.Docker.NetworkMode)
	if err != nil {
		return err
	}

	e.networkMode = networkMode

	return nil
}

func (e *executor) cleanupNetwork(ctx context.Context) error {
	if e.networksManager == nil {
		return errNetworksManagerUndefined
	}

	if e.networkMode.UserDefined() == "" {
		return nil
	}

	inspectResponse, err := e.networksManager.Inspect(ctx)
	if err != nil {
		e.Errorln("network inspect returned error ", err)
		return nil
	}

	for id := range inspectResponse.Containers {
		e.Debugln("Removing Container", id, "...")
		err = e.removeContainer(ctx, id)
		if err != nil {
			e.Errorln("remove container returned error ", err)
		}
	}

	return e.networksManager.Cleanup(ctx)
}

func (e *executor) createServices() (err error) {
	e.SetCurrentStage(ExecutorStageCreatingServices)
	e.Debugln("Creating services...")

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

	if e.networkMode.IsBridge() || e.networkMode.NetworkName() == "" {
		e.Debugln("Building service links...")
		e.links = e.buildServiceLinks(linksMap)
	}

	return
}

func (e *executor) createContainer(
	containerType string,
	imageDefinition common.Image,
	cmd []string,
	allowedInternalImages []string,
) (*types.ContainerJSON, error) {
	if e.volumesManager == nil {
		return nil, errVolumesManagerUndefined
	}

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
	containerName := e.getProjectUniqRandomizedName() + "-" + containerType + "-" + strconv.Itoa(containerIndex)

	config := e.createContainerConfig(containerType, imageDefinition, image.ID, hostname, cmd)

	hostConfig, err := e.createHostConfig()
	if err != nil {
		return nil, err
	}

	aliases := []string{"build", containerName}
	networkConfig := e.networkConfig(aliases)

	// this will fail potentially some builds if there's name collision
	_ = e.removeContainer(e.Context, containerName)

	e.Debugln("Creating container", containerName, "...")
	resp, err := e.client.ContainerCreate(e.Context, config, hostConfig, networkConfig, containerName)
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

func (e *executor) createContainerConfig(
	containerType string,
	imageDefinition common.Image,
	imageID string,
	hostname string,
	cmd []string,
) *container.Config {
	config := &container.Config{
		Image:        imageID,
		Hostname:     hostname,
		Cmd:          cmd,
		Labels:       e.labeler.Labels(map[string]string{"type": containerType}),
		Tty:          false,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    true,
		StdinOnce:    true,
		Env:          append(e.Build.GetAllVariables().StringList(), e.BuildShell.Environment...),
	}
	config.Entrypoint = e.overwriteEntrypoint(&imageDefinition)

	return config
}

func (e *executor) createHostConfig() (*container.HostConfig, error) {
	nanoCPUs, err := e.Config.Docker.GetNanoCPUs()
	if err != nil {
		return nil, err
	}

	return &container.HostConfig{
		Resources: container.Resources{
			Memory:            e.Config.Docker.GetMemory(),
			MemorySwap:        e.Config.Docker.GetMemorySwap(),
			MemoryReservation: e.Config.Docker.GetMemoryReservation(),
			CpusetCpus:        e.Config.Docker.CPUSetCPUs,
			CPUShares:         e.Config.Docker.CPUShares,
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
		NetworkMode:   e.networkMode,
		Links:         append(e.Config.Docker.Links, e.links...),
		Binds:         e.volumesManager.Binds(),
		OomScoreAdj:   e.Config.Docker.OomScoreAdjust,
		ShmSize:       e.Config.Docker.ShmSize,
		VolumeDriver:  e.Config.Docker.VolumeDriver,
		VolumesFrom:   e.Config.Docker.VolumesFrom,
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
		Tmpfs:   e.Config.Docker.Tmpfs,
		Sysctls: e.Config.Docker.SysCtls,
	}, nil
}

func (e *executor) startAndWatchContainer(ctx context.Context, id string, input io.Reader) error {
	options := types.ContainerAttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	}

	e.Debugln("Attaching to container", id, "...")
	hijacked, err := e.client.ContainerAttach(ctx, id, options)
	if err != nil {
		return err
	}
	defer hijacked.Close()

	e.Debugln("Starting container", id, "...")
	err = e.client.ContainerStart(ctx, id, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	// Copy any output to the build trace
	stdoutErrCh := make(chan error)
	go func() {
		_, errCopy := stdcopy.StdCopy(e.Trace, e.Trace, hijacked.Reader)
		stdoutErrCh <- errCopy
	}()

	// Write the input to the container and close its STDIN to get it to finish
	stdinErrCh := make(chan error)
	go func() {
		_, errCopy := io.Copy(hijacked.Conn, input)
		_ = hijacked.CloseWrite()
		if errCopy != nil {
			stdinErrCh <- errCopy
		}
	}()

	// Wait until either:
	// - the job is aborted/cancelled/deadline exceeded
	// - stdin has an error
	// - stdout returns an error or nil, indicating the stream has ended and
	//   the container has exited
	select {
	case <-ctx.Done():
		err = errors.New("aborted")
	case err = <-stdinErrCh:
	case err = <-stdoutErrCh:
	}

	if err != nil {
		e.Debugln("Container", id, "finished with", err)
	}

	// Kill and wait for exit.
	// Containers are stopped so that they can be reused by the job.
	return e.waiter.KillWait(ctx, id)
}

func (e *executor) removeContainer(ctx context.Context, id string) error {
	e.Debugln("Removing container", id)

	e.disconnectNetwork(ctx, id)

	options := types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}

	err := e.client.ContainerRemove(ctx, id, options)
	if err != nil {
		e.Debugln("Removing container", id, "finished with error", err)
		return err
	}

	e.Debugln("Removed container", id)
	return nil
}

func (e *executor) disconnectNetwork(ctx context.Context, id string) {
	e.Debugln("Disconnecting container", id, "from networks")

	netList, err := e.client.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		e.Debugln("Can't get network list. ListNetworks exited with", err)
		return
	}

	for _, network := range netList {
		for _, pluggedContainer := range network.Containers {
			if id == pluggedContainer.Name {
				err = e.client.NetworkDisconnect(ctx, network.ID, id, true)
				if err != nil {
					e.Warningln(
						"Can't disconnect possibly zombie container",
						pluggedContainer.Name,
						"from network",
						network.Name,
						"->",
						err,
					)
				} else {
					e.Warningln(
						"Possibly zombie container",
						pluggedContainer.Name,
						"is disconnected from network",
						network.Name,
					)
				}
				break
			}
		}
	}
}

func (e *executor) verifyAllowedImage(image, optionName string, allowedImages, internalImages []string) error {
	for _, allowedImage := range allowedImages {
		ok, _ := doublestar.Match(allowedImage, image)
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

	e.Println(
		"Please check runner's configuration: " +
			"http://doc.gitlab.com/ci/docker/using_docker_images.html#overwrite-image-and-services")
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
		return "", errors.New("no Docker image specified to run the build in")
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

func (e *executor) connectDocker() error {
	client, err := docker.New(e.Config.Docker.Credentials, "")
	if err != nil {
		return err
	}
	e.client = client

	e.info, err = client.Info(e.Context)
	if err != nil {
		return err
	}

	err = e.validateOSType()
	if err != nil {
		return err
	}

	e.helperImageInfo, err = helperimage.Get(common.REVISION, helperimage.Config{
		OSType:          e.info.OSType,
		Architecture:    e.info.Architecture,
		OperatingSystem: e.info.OperatingSystem,
	})
	e.waiter = wait.NewDockerKillWaiter(e.client)

	return err
}

// validateOSType checks if the ExecutorOptions metadata matches with the docker
// info response.
func (e *executor) validateOSType() error {
	executorOSType := e.ExecutorOptions.Metadata[metadataOSType]
	if executorOSType == "" {
		return common.MakeBuildError("%s does not have any OSType specified", e.Config.Executor)
	}

	if executorOSType != e.info.OSType {
		return common.MakeBuildError(
			"executor requires OSType=%s, but Docker Engine supports only OSType=%s",
			executorOSType, e.info.OSType,
		)
	}

	return nil
}

func (e *executor) createDependencies() error {
	createDependenciesStrategy := []func() error{
		e.createLabeler,
		e.createNetworksManager,
		e.createBuildNetwork,
		e.bindDevices,
		e.createVolumesManager,
		e.createVolumes,
		e.createBuildVolume,
		e.createServices,
	}

	for _, setup := range createDependenciesStrategy {
		err := setup()
		if err != nil {
			return err
		}
	}

	return nil
}

func (e *executor) createVolumes() error {
	e.SetCurrentStage(ExecutorStageCreatingUserVolumes)
	e.Debugln("Creating user-defined volumes...")

	if e.volumesManager == nil {
		return errVolumesManagerUndefined
	}

	for _, volume := range e.Config.Docker.Volumes {
		err := e.volumesManager.Create(e.Context, volume)
		if errors.Is(err, volumes.ErrCacheVolumesDisabled) {
			e.Warningln(fmt.Sprintf(
				"Container based cache volumes creation is disabled. Will not create volume for %q",
				volume,
			))
			continue
		}

		if err != nil {
			return err
		}
	}

	return nil
}

func (e *executor) createBuildVolume() error {
	e.SetCurrentStage(ExecutorStageCreatingBuildVolumes)
	e.Debugln("Creating build volume...")

	if e.volumesManager == nil {
		return errVolumesManagerUndefined
	}

	jobsDir := e.Build.RootDir

	var err error

	if e.Build.GetGitStrategy() == common.GitFetch {
		err = e.volumesManager.Create(e.Context, jobsDir)
		if err == nil {
			return nil
		}

		if errors.Is(err, volumes.ErrCacheVolumesDisabled) {
			err = e.volumesManager.CreateTemporary(e.Context, jobsDir)
		}
	} else {
		err = e.volumesManager.CreateTemporary(e.Context, jobsDir)
	}

	if err != nil {
		var volDefinedErr *volumes.ErrVolumeAlreadyDefined
		if !errors.As(err, &volDefinedErr) {
			return err
		}
	}

	return nil
}

func (e *executor) Prepare(options common.ExecutorPrepareOptions) error {
	e.SetCurrentStage(ExecutorStagePrepare)

	if options.Config.Docker == nil {
		return errors.New("missing docker configuration")
	}

	e.AbstractExecutor.PrepareConfiguration(options)

	err := e.connectDocker()
	if err != nil {
		return err
	}

	err = e.prepareBuildsDir(options)
	if err != nil {
		return err
	}

	err = e.AbstractExecutor.PrepareBuildAndShell()
	if err != nil {
		return err
	}

	if e.BuildShell.PassFile {
		return errors.New("docker doesn't support shells that require script file")
	}

	imageName, err := e.expandImageName(e.Build.Image.Name, []string{})
	if err != nil {
		return err
	}

	e.Println("Using Docker executor with image", imageName, "...")

	err = e.createDependencies()
	if err != nil {
		return err
	}
	return nil
}

func (e *executor) prepareBuildsDir(options common.ExecutorPrepareOptions) error {
	if e.volumeParser == nil {
		return common.MakeBuildError("missing volume parser")
	}

	isHostMounted, err := volumes.IsHostMountedVolume(e.volumeParser, e.RootDir(), options.Config.Docker.Volumes...)
	if err != nil {
		return &common.BuildError{Inner: err}
	}

	// We need to set proper value for e.SharedBuildsDir because
	// it's required to properly start the job, what is done inside of
	// e.AbstractExecutor.Prepare()
	// And a started job is required for Volumes Manager to work, so it's
	// done before the manager is even created.
	if isHostMounted {
		e.SharedBuildsDir = true
	}

	return nil
}

func (e *executor) Cleanup() {
	e.SetCurrentStage(ExecutorStageCleanup)

	var wg sync.WaitGroup

	ctx, cancel := context.WithTimeout(context.Background(), dockerCleanupTimeout)
	defer cancel()

	remove := func(id string) {
		wg.Add(1)
		go func() {
			_ = e.removeContainer(ctx, id)
			wg.Done()
		}()
	}

	for _, temporaryID := range e.temporary {
		remove(temporaryID)
	}

	wg.Wait()

	err := e.cleanupVolume(ctx)
	if err != nil {
		volumeLogger := e.WithFields(logrus.Fields{
			"error": err,
		})

		volumeLogger.Errorln("Failed to cleanup volumes")
	}

	err = e.cleanupNetwork(ctx)
	if err != nil {
		networkLogger := e.WithFields(logrus.Fields{
			"network": e.networkMode.NetworkName(),
			"error":   err,
		})

		networkLogger.Errorln("Failed to remove network for build")
	}

	if e.client != nil {
		_ = e.client.Close()
	}

	e.AbstractExecutor.Cleanup()
}

func (e *executor) cleanupVolume(ctx context.Context) error {
	if e.volumesManager == nil {
		e.Debugln("Volumes manager is empty, skipping volumes cleanup")
		return nil
	}

	err := e.volumesManager.RemoveTemporary(ctx)
	if err != nil {
		return fmt.Errorf("remove temporary volumes: %w", err)
	}

	return nil
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
		return fmt.Errorf("getPrebuiltImage: %w", err)
	}

	containerName := service.Names[0] + "-wait-for-service"

	environment, err := e.addServiceHealthCheckEnvironment(service)
	if err != nil {
		return err
	}

	cmd := []string{"gitlab-runner-helper", "health-check"}

	config := e.createConfigForServiceHealthCheckContainer(service, cmd, waitImage, environment)
	hostConfig := e.createHostConfigForServiceHealthCheck(service)

	e.Debugln(fmt.Sprintf("Creating service healthcheck container %s...", containerName))
	resp, err := e.client.ContainerCreate(e.Context, config, hostConfig, nil, containerName)
	if err != nil {
		return fmt.Errorf("create service container: %w", err)
	}
	defer func() { _ = e.removeContainer(e.Context, resp.ID) }()

	e.Debugln(fmt.Sprintf("Starting service healthcheck container %s (%s)...", containerName, resp.ID))
	err = e.client.ContainerStart(e.Context, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return fmt.Errorf("start service container: %w", err)
	}

	ctx, cancel := context.WithTimeout(e.Context, timeout)
	defer cancel()

	err = e.waiter.Wait(ctx, resp.ID)
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		err = fmt.Errorf("service %q timeout", containerName)
	} else {
		err = fmt.Errorf("service %q health check: %w", containerName, err)
	}

	return &serviceHealthCheckError{
		Inner: err,
		Logs:  e.readContainerLogs(resp.ID),
	}
}

func (e *executor) createConfigForServiceHealthCheckContainer(
	service *types.Container,
	cmd []string,
	waitImage *types.ImageInspect,
	environment []string,
) *container.Config {
	return &container.Config{
		Cmd:    cmd,
		Image:  waitImage.ID,
		Labels: e.labeler.Labels(map[string]string{"type": labelWaitType, "wait": service.ID}),
		Env:    environment,
	}
}

func (e *executor) createHostConfigForServiceHealthCheck(service *types.Container) *container.HostConfig {
	return &container.HostConfig{
		RestartPolicy: neverRestartPolicy,
		Links:         []string{service.Names[0] + ":service"},
		NetworkMode:   e.networkMode,
		LogConfig: container.LogConfig{
			Type: "json-file",
		},
	}
}

// addServiceHealthCheckEnvironment returns environment variables mimicing
// the legacy container links networking feature of Docker, where environment
// variables are provided with the hostname and port of the linked service our
// health check is performed against.
//
// The hostname we provide is the container's short ID (the first 12 characters
// of a full container ID). The short ID, as opposed to the full ID, is
// internally resolved to the container's IP address by Docker's built-in DNS
// service.
//
// The legacy container links (https://docs.docker.com/network/links/) network
// feature is deprecated. When we remove support for links, the healthcheck
// system can be updated to no longer rely on environment variables
func (e *executor) addServiceHealthCheckEnvironment(service *types.Container) ([]string, error) {
	environment := []string{}

	if e.networkMode.UserDefined() != "" {
		environment = append(environment, "WAIT_FOR_SERVICE_TCP_ADDR="+service.ID[:12])
		ports, err := e.getContainerExposedPorts(service)
		if err != nil {
			return nil, fmt.Errorf("get container exposed ports: %v", err)
		}
		if len(ports) == 0 {
			return nil, fmt.Errorf("service %q has no exposed ports", service.Names[0])
		}

		environment = append(environment, fmt.Sprintf("WAIT_FOR_SERVICE_TCP_PORT=%d", ports[0]))
	}

	return environment, nil
}

func (e *executor) getContainerExposedPorts(container *types.Container) ([]int, error) {
	var ports []int

	inspect, err := e.client.ContainerInspect(e.Context, container.ID)
	if err != nil {
		return nil, err
	}

	for port := range inspect.Config.ExposedPorts {
		start, _, err := port.Range()
		if err == nil && port.Proto() == "tcp" {
			ports = append(ports, start)
		}
	}

	sort.Ints(ports)
	return ports, nil
}

func (e *executor) waitForServiceContainer(service *types.Container, timeout time.Duration) error {
	err := e.runServiceHealthCheckContainer(service, timeout)
	if err == nil {
		return nil
	}

	var buffer bytes.Buffer
	buffer.WriteString("\n")
	buffer.WriteString(
		helpers.ANSI_YELLOW + "*** WARNING:" + helpers.ANSI_RESET + " Service " + service.Names[0] +
			" probably didn't start properly.\n")
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
	_, _ = io.Copy(e.Trace, &buffer)
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
	defer func() { _ = hijacked.Close() }()

	_, _ = stdcopy.StdCopy(&containerBuffer, &containerBuffer, hijacked)
	containerLog := containerBuffer.String()
	return strings.TrimSpace(containerLog)
}
