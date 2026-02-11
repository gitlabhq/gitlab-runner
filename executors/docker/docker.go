package docker

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/containerd/errdefs"
	"github.com/docker/cli/opts"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/hashicorp/go-version"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/exec"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/labels"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/networks"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/prebuilt"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/pull"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/permission"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/wait"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/helperimage"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/limitwriter"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

const (
	ExecutorStagePrepare common.ExecutorStage = "docker_prepare"
	ExecutorStageRun     common.ExecutorStage = "docker_run"
	ExecutorStageCleanup common.ExecutorStage = "docker_cleanup"

	ExecutorStageBootstrap            common.ExecutorStage = "docker_bootstrap"
	ExecutorStageCreatingBuildVolumes common.ExecutorStage = "docker_creating_build_volumes"
	ExecutorStageCreatingServices     common.ExecutorStage = "docker_creating_services"
	ExecutorStageCreatingUserVolumes  common.ExecutorStage = "docker_creating_user_volumes"
	ExecutorStagePullingImage         common.ExecutorStage = "docker_pulling_image"

	ServiceLogOutputLimit = 64 * 1024

	labelServiceType = "service"
	labelWaitType    = "wait"

	// internalFakeTunnelHostname is an internal hostname we provide the Docker client
	// when we provide a tunnelled dialer implementation. Because we're overriding
	// the dialer, this domain should never be used by the client, but we use the
	// reserved TLD ".invalid" for safety.
	internalFakeTunnelHostname = "http://internal.tunnel.invalid"

	// runnerJobVarsNames is the name used to identify the all the job variables names.
	// It is used to allow step-runner to filter these variables once the gRPC service is started
	runnerJobVarsNames = "RUNNER_JOB_VAR_NAMES"
)

var neverRestartPolicy = container.RestartPolicy{Name: "no"}

var (
	errVolumesManagerUndefined  = errors.New("volumesManager is undefined")
	errNetworksManagerUndefined = errors.New("networksManager is undefined")
)

type executor struct {
	executors.AbstractExecutor
	volumeParser              parser.Parser
	newVolumePermissionSetter func() (permission.Setter, error)
	info                      system.Info
	serverAPIVersion          *version.Version
	waiter                    wait.KillWaiter

	temporary        []string // IDs of containers that should be removed
	buildContainerID string

	services []*serviceInfo

	// links used to use docker 'links' feature, which tied containers together
	// so that their hosts would resolve.
	//
	// This feature is now deprecated, but we emulate it using ExtraHosts, and
	// therefore links is now an array of "<service name>:<service ip>" that
	// is provided to every container.
	links []string

	devices        []container.DeviceMapping
	deviceRequests []container.DeviceRequest

	helperImageInfo helperimage.Info

	volumesManager  volumes.Manager
	networksManager networks.Manager
	labeler         labels.Labeler
	pullManager     pull.Manager

	networkMode container.NetworkMode

	projectUniqRandomizedName string

	dockerConn      *dockerConnection
	dockerConnector dockerConnector

	logConfig container.LogConfig
}

type dockerConnector func(ctx context.Context, options common.ExecutorPrepareOptions, executor *executor) error

func (dc dockerConnector) Connect(ctx context.Context, options common.ExecutorPrepareOptions, executor *executor) error {
	if dc == nil {
		dc = connectDocker
	}
	return dc(ctx, options, executor)
}

type dockerTunnel struct {
	client executors.Client
	opts   []client.Opt
	creds  docker.Credentials
}

// newDockerTunnel returns a new dockerTunnel instance. IF the specified common.ExecutorData is of type executors.Environment,
// this indicates we will be connecting to a remote docker daemon instance and should tunnel docker commands though a
// executors.Client instance. In this case, the returned dockerTunnel will include a valid and initialized executors.Client
// instance, with corresponding []client.Opt and docker.Credentials to initialize a docker.Client.
//
// Otherwise the returned dockerTunnel will have a nil executor.Client and []client.Opt, and a default docker.Credentials.
func newDockerTunnel(
	ctx context.Context,
	options common.ExecutorPrepareOptions,
	build *common.Build,
	creds docker.Credentials,
	env common.ExecutorData,
	logger buildlogger.Logger,
) (*dockerTunnel, error) {
	if environment, ok := env.(executors.Environment); ok {
		tc, err := environment.Prepare(ctx, logger, options)
		if err != nil {
			return nil, fmt.Errorf("preparing environment: %w", err)
		}

		// We tunnel the docker connection for remote environments.
		//
		// To do this, we create a new dial context for Docker's client, whilst
		// also overridding the daemon hostname it would typically use (if it were to use
		// its own dialer).
		scheme, dialer, err := environmentDialContext(ctx, tc, creds.Host, build.IsFeatureFlagOn(featureflags.UseDockerAutoscalerDialStdio))
		if err != nil {
			return nil, fmt.Errorf("creating env dialer: %w", err)
		}

		// If the scheme (docker uses it to define the protocol used) is "npipe" or "unix", we
		// need to use a "fake" host, otherwise when dialing from Linux to Windows or vice-versa
		// docker will complain because it doesn't think Linux can support "npipe" and doesn't
		// think Windows can support "unix".
		switch scheme {
		case "unix", "npipe", "dial-stdio":
			creds.Host = internalFakeTunnelHostname
		}

		return &dockerTunnel{
			client: tc,
			opts:   []client.Opt{client.WithDialContext(dialer)},
			creds:  creds,
		}, nil
	}

	return &dockerTunnel{client: nil, opts: nil, creds: creds}, nil
}

type dockerConnection struct {
	docker.Client
	tunnelClient executors.Client
	cancel       func()
}

func (dc *dockerConnection) Close() error {
	if dc == nil {
		return nil
	}
	var err error
	if dc.Client != nil {
		err = dc.Client.Close()
		dc.Client = nil
	}
	if dc.tunnelClient != nil {
		err = errors.Join(err, dc.tunnelClient.Close())
		dc.tunnelClient = nil
	}
	if dc.cancel != nil {
		dc.cancel()
		dc.cancel = nil
	}
	return err
}

// newDockerConnection returns a new dockerConnection instance using the executor.Client instance and connection info
// embedded in the dockerTunnel instance returned by the factory function. If we're connecting to the local docker
// daemon, the executor.Client instance will be nil (and that's OK).
func newDockerConnection(dockerTunnel *dockerTunnel, cancel func()) (*dockerConnection, error) {
	dockerClient, err := docker.New(dockerTunnel.creds, dockerTunnel.opts...)
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}

	return &dockerConnection{Client: dockerClient, tunnelClient: dockerTunnel.client, cancel: cancel}, nil
}

// createDockerConnection creates a connection to a potentially remote docker daemon. The connection is encapsulated in
// a dockerConnection object which includes a docker.Client instance and, if connecting to a remote docker daemon, an
// executors.Client instance.
//
// Note that in the case of a remote docker daemon, we want to maintain a long-lived connection for the duration of the
// job (including during the Cleanup stage). To achieve this, we don't want the context to be cancelled when the job is
// cancelled or times out, so we create a new context here with a timeout of job-timeout + dockerCleanupTimeout. This
// fixes https://gitlab.com/gitlab-org/gitlab-runner/-/issues/38725.
func createDockerConnection(ctx context.Context, opts common.ExecutorPrepareOptions, e *executor) (*dockerConnection, error) {
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		deadline = time.Now().Add(e.Build.GetBuildTimeout())
	}
	ctx, cancel := context.WithDeadline(context.Background(), deadline.Add(dockerCleanupTimeout))

	dockerTunnel, err := newDockerTunnel(
		ctx,
		opts,
		e.Build,
		e.Config.Docker.Credentials,
		e.Build.ExecutorData,
		e.BuildLogger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("creating docker tunnel: %w", err)
	}

	return newDockerConnection(dockerTunnel, cancel)
}

var version1_44 = version.Must(version.NewVersion("1.44"))

func (e *executor) getServiceVariables(serviceDefinition spec.Image) []string {
	variables := e.Build.GetAllVariables().PublicOrInternal()
	variables = append(variables, serviceDefinition.Variables...)

	return variables.Expand().StringList()
}

func (e *executor) expandAndGetDockerImage(
	imageName string,
	allowedImages []string,
	dockerOptions spec.ImageDockerOptions,
	imagePullPolicies []common.DockerPullPolicy,
) (*image.InspectResponse, error) {
	imageName, err := e.expandImageName(imageName, allowedImages)
	if err != nil {
		return nil, err
	}

	dockerOptions = dockerOptions.Expand(e.Build.GetAllVariables())

	image, err := e.pullManager.GetDockerImage(imageName, dockerOptions, imagePullPolicies)
	if err != nil {
		return nil, err
	}

	return image, nil
}

func (e *executor) getHelperImage() (*image.InspectResponse, error) {
	if imageNameFromConfig := e.ExpandValue(e.Config.Docker.HelperImage); imageNameFromConfig != "" {
		e.BuildLogger.Debugln(
			"Pull configured helper_image for predefined container instead of import bundled image",
			imageNameFromConfig,
			"...",
		)

		e.BuildLogger.Println("Using helper image: ", imageNameFromConfig, " (overridden, default would be ", e.helperImageInfo, ")")

		return e.pullManager.GetDockerImage(imageNameFromConfig, spec.ImageDockerOptions{}, nil)
	}

	e.BuildLogger.Debugln(fmt.Sprintf("Looking for prebuilt image %s...", e.helperImageInfo))
	image, _, err := e.dockerConn.ImageInspectWithRaw(e.Context, e.helperImageInfo.String())
	if err == nil {
		return &image, nil
	}

	// Try to load prebuilt image from local filesystem
	loadedImage := e.getLocalHelperImage()
	if loadedImage != nil {
		return loadedImage, nil
	}

	e.BuildLogger.Println("Using helper image: ", e.helperImageInfo.String())

	// Fall back to getting image from registry
	e.BuildLogger.Debugln(fmt.Sprintf("Loading image form registry: %s", e.helperImageInfo))
	return e.pullManager.GetDockerImage(e.helperImageInfo.String(), spec.ImageDockerOptions{}, nil)
}

func (e *executor) getLocalHelperImage() *image.InspectResponse {
	if e.helperImageInfo.Prebuilt == "" {
		return nil
	}

	image, err := prebuilt.Get(e.Context, e.dockerConn, e.helperImageInfo)
	if err != nil {
		e.BuildLogger.Debugln("Failed to load prebuilt:", err)
	}

	return image
}

func (e *executor) getBuildImage() (*image.InspectResponse, error) {
	imageName, err := e.expandImageName(e.Build.Image.Name, []string{})
	if err != nil {
		return nil, err
	}

	dockerOptions := e.Build.Image.ExecutorOptions.Docker.Expand(e.Build.GetAllVariables())
	imagePullPolicies := e.Build.Image.PullPolicies

	// Fetch image
	image, err := e.pullManager.GetDockerImage(imageName, dockerOptions, imagePullPolicies)
	if err != nil {
		return nil, err
	}

	return image, nil
}

func fakeContainer(id string, names ...string) *container.Summary {
	return &container.Summary{ID: id, Names: names}
}

func (e *executor) parseDeviceString(deviceString string) (device container.DeviceMapping, err error) {
	// Split the device string PathOnHost[:PathInContainer[:CgroupPermissions]]
	parts := strings.Split(deviceString, ":")

	if len(parts) > 3 {
		return device, fmt.Errorf("too many colons")
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

	return device, err
}

func (e *executor) bindDevices() (err error) {
	e.devices, err = e.bindContainerDevices(e.Config.Docker.Devices)
	return err
}

func (e *executor) bindContainerDevices(devices []string) ([]container.DeviceMapping, error) {
	mapping := []container.DeviceMapping{}

	for _, deviceString := range devices {
		device, err := e.parseDeviceString(deviceString)
		if err != nil {
			return nil, fmt.Errorf("failed to parse device string %q: %w", deviceString, err)
		}

		mapping = append(mapping, device)
	}
	return mapping, nil
}

func (e *executor) bindDeviceRequests() (err error) {
	e.deviceRequests, err = e.bindContainerDeviceRequests(e.Config.Docker.Gpus)
	return err
}

func (e *executor) bindContainerDeviceRequests(gpus string) ([]container.DeviceRequest, error) {
	if strings.TrimSpace(gpus) == "" {
		return nil, nil
	}

	var gpuOpts opts.GpuOpts

	err := gpuOpts.Set(gpus)
	if err != nil {
		return nil, fmt.Errorf("parsing gpus string %q: %w", gpus, err)
	}

	return gpuOpts.Value(), nil
}

func isInAllowedPrivilegedImages(image string, allowedPrivilegedImages []string) bool {
	if len(allowedPrivilegedImages) == 0 {
		return true
	}
	for _, allowedImage := range allowedPrivilegedImages {
		ok, _ := doublestar.Match(allowedImage, image)
		if ok {
			return true
		}
	}
	return false
}

func (e *executor) isInPrivilegedServiceList(serviceDefinition spec.Image) bool {
	return isInAllowedPrivilegedImages(serviceDefinition.Name, e.Config.Docker.AllowedPrivilegedServices)
}

func (e *executor) createService(
	serviceIndex int,
	service, version, image string,
	definition spec.Image,
	linkNames []string,
) (*serviceInfo, error) {
	if service == "" {
		return nil, common.MakeBuildError("invalid service image name: %s", definition.Name)
	}

	if e.volumesManager == nil {
		return nil, errVolumesManagerUndefined
	}

	var serviceName string
	if strings.HasPrefix(version, "@sha256") {
		serviceName = fmt.Sprintf("%s%s...", service, version) // service@digest
	} else {
		serviceName = fmt.Sprintf("%s:%s...", service, version) // service:version
	}

	dockerOptions := definition.ExecutorOptions.Docker.Expand(e.Build.GetAllVariables())

	e.BuildLogger.Println("Starting service", serviceName)
	serviceImage, err := e.pullManager.GetDockerImage(image, dockerOptions, definition.PullPolicies)
	if err != nil {
		return nil, err
	}

	serviceSlug := strings.ReplaceAll(service, "/", "__")
	containerName := e.makeContainerName(fmt.Sprintf("%s-%d", serviceSlug, serviceIndex))

	// this will fail potentially some builds if there's name collision
	_ = e.removeContainer(e.Context, containerName)

	config := e.createServiceContainerConfig(service, version, serviceImage.ID, definition)

	devices, err := e.getServicesDevices(image)
	if err != nil {
		return nil, err
	}

	deviceRequests, err := e.getServicesDeviceRequests()
	if err != nil {
		return nil, err
	}

	hostConfig, err := e.createHostConfigForService(e.isInPrivilegedServiceList(definition), devices, deviceRequests)
	if err != nil {
		return nil, err
	}

	platform := platformForImage(serviceImage, definition.ExecutorOptions)
	networkConfig := e.networkConfig(linkNames)

	e.BuildLogger.Debugln("Creating service container", containerName, "...")
	resp, err := e.dockerConn.ContainerCreate(e.Context, config, hostConfig, networkConfig, platform, containerName)
	if err != nil {
		return nil, err
	}

	e.BuildLogger.Debugln(fmt.Sprintf("Starting service container %s (%s)...", containerName, resp.ID))
	err = e.dockerConn.ContainerStart(e.Context, resp.ID, container.StartOptions{})
	if err != nil {
		e.temporary = append(e.temporary, resp.ID)
		return nil, err
	}

	ip, ports, err := e.getContainerIPAndExposedPorts(resp.ID)
	if err != nil {
		return nil, fmt.Errorf("getting exposed ports: %w", err)
	}

	return &serviceInfo{
		ID:    resp.ID,
		Name:  containerName,
		IP:    ip,
		Ports: ports,
	}, nil
}

func platformForImage(image *image.InspectResponse, opts spec.ImageExecutorOptions) *v1.Platform {
	if image == nil || opts.Docker.Platform == "" {
		return nil
	}

	return &v1.Platform{
		Architecture: image.Architecture,
		OS:           image.Os,
		OSVersion:    image.OsVersion,
		Variant:      image.Variant,
	}
}

func (e *executor) createHostConfigForService(imageIsPrivileged bool, devices []container.DeviceMapping, deviceRequests []container.DeviceRequest) (*container.HostConfig, error) {
	nanoCPUs, err := e.Config.Docker.GetServiceNanoCPUs()
	if err != nil {
		return nil, fmt.Errorf("service nano cpus: %w", err)
	}

	privileged := e.Config.Docker.Privileged
	if e.Config.Docker.ServicesPrivileged != nil {
		privileged = *e.Config.Docker.ServicesPrivileged
	}
	privileged = privileged && imageIsPrivileged

	var useInit *bool
	if e.Build.IsFeatureFlagOn(featureflags.UseInitWithDockerExecutor) {
		yes := true
		useInit = &yes
	}

	return &container.HostConfig{
		Resources: container.Resources{
			Memory:            e.Config.Docker.GetServiceMemory(),
			MemorySwap:        e.Config.Docker.GetServiceMemorySwap(),
			MemoryReservation: e.Config.Docker.GetServiceMemoryReservation(),
			CgroupParent:      e.getServiceCgroupParent(),
			CpusetCpus:        e.Config.Docker.ServiceCPUSetCPUs,
			CPUShares:         e.Config.Docker.ServiceCPUShares,
			NanoCPUs:          nanoCPUs,
			Devices:           devices,
			DeviceRequests:    deviceRequests,
		},
		DNS:           e.Config.Docker.DNS,
		DNSSearch:     e.Config.Docker.DNSSearch,
		RestartPolicy: neverRestartPolicy,
		ExtraHosts:    e.Config.Docker.ExtraHosts,
		Privileged:    privileged,
		SecurityOpt:   e.Config.Docker.ServicesSecurityOpt,
		Runtime:       e.Config.Docker.Runtime,
		UsernsMode:    container.UsernsMode(e.Config.Docker.UsernsMode),
		NetworkMode:   e.networkMode,
		Binds:         e.volumesManager.Binds(),
		ShmSize:       e.Config.Docker.ShmSize,
		Tmpfs:         e.Config.Docker.ServicesTmpfs,
		LogConfig:     e.logConfig,
		Init:          useInit,
	}, nil
}

func (e *executor) createServiceContainerConfig(
	service, version, serviceImageID string,
	definition spec.Image,
) *container.Config {
	labels := e.prepareContainerLabels(map[string]string{
		"type":            labelServiceType,
		"service":         service,
		"service.version": version,
	})

	// NOTE: the follow is for backwards-compatibility.
	// See https://gitlab.com/gitlab-org/gitlab-runner/-/issues/39048
	// It adds the labels from the configuration with the gitlab-runner prefix.
	// The SSoT for the dockerLabelPrefix is the labels package, but lets avoid
	// exporting it or providing helper functions to add it.
	// The code below is an EXCEPTION and should be removed asap.
	const dockerLabelPrefix = "com.gitlab.gitlab-runner"
	for k, v := range e.Config.Docker.ContainerLabels {
		labels[fmt.Sprintf("%s.%s", dockerLabelPrefix, k)] = e.Build.Variables.ExpandValue(v)
	}

	config := &container.Config{
		Image:  serviceImageID,
		Labels: labels,
		Env:    e.getServiceVariables(definition),
	}

	if len(definition.Command) > 0 {
		config.Cmd = definition.Command
	}
	config.Entrypoint = e.overwriteEntrypoint(&definition)
	config.User = string(definition.ExecutorOptions.Docker.Expand(e.Build.GetAllVariables()).User)

	return config
}

func (e *executor) getServicesDevices(image string) ([]container.DeviceMapping, error) {
	var devices []container.DeviceMapping
	for imageGlob, deviceStrings := range e.Config.Docker.ServicesDevices {
		ok, err := doublestar.Match(imageGlob, image)
		if err != nil {
			return nil, fmt.Errorf("invalid service device image pattern: %s: %w", imageGlob, err)
		}

		if !ok {
			continue
		}

		dvs, err := e.bindContainerDevices(deviceStrings)
		if err != nil {
			return nil, err
		}
		devices = append(devices, dvs...)
	}

	return devices, nil
}

func (e *executor) getServicesDeviceRequests() ([]container.DeviceRequest, error) {
	return e.bindContainerDeviceRequests(e.Config.Docker.ServiceGpus)
}

func (e *executor) networkConfig(aliases []string) *network.NetworkingConfig {
	// setting a container's mac-address changed in API version 1.44
	if e.serverAPIVersion.LessThan(version1_44) {
		return e.networkConfigLegacy(aliases)
	}

	nm := string(e.networkMode)
	nc := network.NetworkingConfig{}

	if nm == "" {
		// docker defaults to using "bridge" network driver if none was specified.
		nc.EndpointsConfig = map[string]*network.EndpointSettings{
			network.NetworkDefault: {MacAddress: e.Config.Docker.MacAddress},
		}
		return &nc
	}

	nc.EndpointsConfig = map[string]*network.EndpointSettings{
		nm: {MacAddress: e.Config.Docker.MacAddress},
	}

	if e.networkMode.IsUserDefined() {
		nc.EndpointsConfig[nm].Aliases = aliases
	}

	return &nc
}

// Setting a container's mac-address changed in API version 1.44. This is the original/legacy/pre-1.44 way to set
// mac-address.
func (e *executor) networkConfigLegacy(aliases []string) *network.NetworkingConfig {
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

// Build and predefined container names are comprised of:
// - A runner project scoped ID (runner-<description>-project-<project_id>-concurrent-<concurrent>)
// - A unique randomized ID for each execution
// - The container's type (build, predefined, step-runner)
//
// For example: runner-linux-project-123-concurrent-2-0a1b2c3d-predefined
//
// A container of the same type is created _once_ per execution and re-used.
func (e *executor) makeContainerName(suffix string) string {
	return e.getProjectUniqRandomizedName() + "-" + suffix
}

func (e *executor) createBuildNetwork() error {
	if e.networksManager == nil {
		return errNetworksManagerUndefined
	}

	networkMode, err := e.networksManager.Create(e.Context, e.Config.Docker.NetworkMode, e.Config.Docker.EnableIPv6)
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
		e.BuildLogger.Errorln("network inspect returned error ", err)
		return nil
	}

	for id := range inspectResponse.Containers {
		e.BuildLogger.Debugln("Removing Container", id, "...")
		err = e.removeContainer(ctx, id)
		if err != nil {
			e.BuildLogger.Errorln("remove container returned error ", err)
		}
	}

	return e.networksManager.Cleanup(ctx)
}

func (e *executor) isInPrivilegedImageList(imageDefinition spec.Image) bool {
	return isInAllowedPrivilegedImages(imageDefinition.Name, e.Config.Docker.AllowedPrivilegedImages)
}

type containerConfigurator interface {
	ContainerConfig(image *image.InspectResponse) (*container.Config, error)
	HostConfig() (*container.HostConfig, error)
	NetworkConfig(aliases []string) *network.NetworkingConfig
}

type defaultContainerConfigurator struct {
	e                     *executor
	containerType         string
	imageDefinition       spec.Image
	cmd                   []string
	allowedInternalImages []string
}

var _ containerConfigurator = &defaultContainerConfigurator{}

func newDefaultContainerConfigurator(
	e *executor,
	containerType string,
	imageDefinition spec.Image,
	cmd,
	allowedInternalImages []string,
) *defaultContainerConfigurator {
	return &defaultContainerConfigurator{
		e:                     e,
		containerType:         containerType,
		imageDefinition:       imageDefinition,
		cmd:                   cmd,
		allowedInternalImages: allowedInternalImages,
	}
}

func (c *defaultContainerConfigurator) ContainerConfig(image *image.InspectResponse) (*container.Config, error) {
	hostname := c.e.Config.Docker.Hostname
	if hostname == "" {
		hostname = c.e.Build.ProjectUniqueName()
	}

	return c.e.createContainerConfig(
		c.containerType,
		c.imageDefinition,
		image,
		hostname,
		c.cmd,
	)
}

func (c *defaultContainerConfigurator) HostConfig() (*container.HostConfig, error) {
	return c.e.createHostConfig(
		c.containerType == buildContainerType,
		c.e.isInPrivilegedImageList(c.imageDefinition),
	)
}

func (c *defaultContainerConfigurator) NetworkConfig(aliases []string) *network.NetworkingConfig {
	return c.e.networkConfig(aliases)
}

func (e *executor) createContainer(
	containerType string,
	imageDefinition spec.Image,
	allowedInternalImages []string,
	cfgTor containerConfigurator,
) (*container.InspectResponse, error) {
	if e.volumesManager == nil {
		return nil, errVolumesManagerUndefined
	}

	image, err := e.expandAndGetDockerImage(
		imageDefinition.Name,
		allowedInternalImages,
		imageDefinition.ExecutorOptions.Docker,
		imageDefinition.PullPolicies,
	)
	if err != nil {
		return nil, err
	}

	containerName := e.makeContainerName(containerType)

	config, err := cfgTor.ContainerConfig(image)
	if err != nil {
		return nil, fmt.Errorf("failed to create container configuration: %w", err)
	}

	hostConfig, err := cfgTor.HostConfig()
	if err != nil {
		return nil, err
	}

	networkConfig := cfgTor.NetworkConfig([]string{"build", containerName})

	var platform *v1.Platform
	// predefined/helper container always uses native platform
	if containerType == buildContainerType {
		platform = platformForImage(image, imageDefinition.ExecutorOptions)
	}

	// this will fail potentially some builds if there's name collision
	_ = e.removeContainer(e.Context, containerName)

	e.BuildLogger.Debugln("Creating container", containerName, "...")
	resp, err := e.dockerConn.ContainerCreate(e.Context, config, hostConfig, networkConfig, platform, containerName)
	if resp.ID != "" {
		e.temporary = append(e.temporary, resp.ID)
		if containerType == buildContainerType {
			e.buildContainerID = resp.ID
		}
	}
	if err != nil {
		return nil, err
	}

	inspect, err := e.dockerConn.ContainerInspect(e.Context, resp.ID)
	return &inspect, err
}

func (e *executor) createContainerConfig(
	containerType string,
	imageDefinition spec.Image,
	image *image.InspectResponse,
	hostname string,
	cmd []string,
) (*container.Config, error) {
	labels := e.prepareContainerLabels(map[string]string{"type": containerType})
	jobVars, err := e.prepareContainerEnvVariables()
	if err != nil {
		return nil, fmt.Errorf("setting job variables: %w", err)
	}

	config := &container.Config{
		Image:        image.ID,
		Hostname:     hostname,
		Cmd:          cmd,
		Labels:       labels,
		Tty:          false,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    true,
		StdinOnce:    true,
		Entrypoint:   e.overwriteEntrypoint(&imageDefinition),
		Env:          jobVars.StringList(),
	}

	//nolint:nestif
	if containerType == buildContainerType {
		if e.Build.UseNativeSteps() {
			config.Cmd = append([]string{bootstrappedBinary, "steps", "serve"}, config.Cmd...)

			// Environment variables interferes with steps. Given this situation, when
			// native steps are enabled, we no longer add the env vars to the container.
			config.Env = nil
		}

		// user config should only be set in build containers
		if user, err := e.getBuildContainerUser(imageDefinition); err != nil {
			return nil, err
		} else {
			config.User = user
		}
	}

	// setting a container's mac-address changed in API version 1.44
	if e.serverAPIVersion.LessThan(version1_44) {
		//nolint:staticcheck
		config.MacAddress = e.Config.Docker.MacAddress
	}

	return config, nil
}

// prepareContainerEnvVariables prepares the environment variables for the build container.
// When native steps are enabled, it compresses the list of job variable names and adds them
// to the environment as RUNNER_JOB_VAR_NAMES. This allows step-runner to identify and filter
// out job variables from the OS environment, preventing environment variable size limit issues.
//
// The variable names are gzip-compressed to minimize the size of the RUNNER_JOB_VAR_NAMES
// environment variable itself, which is important on systems with strict environment limits
// (particularly Windows).
//
// For non-native step builds, the function returns the variables unchanged since step-runner
// filtering is not needed.
func (e *executor) prepareContainerEnvVariables() (spec.Variables, error) {
	vars := e.Build.GetAllVariables()

	if !e.Build.UseNativeSteps() {
		return vars, nil
	}

	names := vars.GetAllVariableNames()
	compressedVarNames, err := gzipString(names)
	if err != nil {
		return nil, fmt.Errorf("job variables names compression failed: %w", err)
	}

	v := append([]spec.Variable{}, vars...)
	v = append(v, spec.Variable{
		Key:   runnerJobVarsNames,
		Value: compressedVarNames,
	})

	return v, nil
}

// gzipString compresses a string and returns the compressed string.
func gzipString(src string) (string, error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	if _, err := gz.Write([]byte(src)); err != nil {
		return "", fmt.Errorf("writing to gzip writer: %w", err)
	}
	if err := gz.Close(); err != nil {
		return "", fmt.Errorf("closing gzip writer: %w", err)
	}

	return base64.StdEncoding.EncodeToString(b.Bytes()), nil
}

func (e *executor) getBuildContainerUser(imageDefinition spec.Image) (string, error) {
	// runner config takes precedence
	user := e.Config.Docker.User
	if user == "" {
		user = string(imageDefinition.ExecutorOptions.Docker.Expand(e.Build.GetAllVariables()).User)
	}

	if !e.Config.Docker.IsUserAllowed(user) {
		return "", fmt.Errorf("user %q is not an allowed user: %v",
			user, e.Config.Docker.AllowedUsers)
	}

	return user, nil
}

// getCgroupParent returns the cgroup parent for build containers
func (e *executor) getCgroupParent() string {
	if path := e.Config.GetSlotCgroupPath(e.Build.ExecutorData); path != "" {
		return path
	}
	return e.Config.Docker.CgroupParent
}

// getServiceCgroupParent returns the cgroup parent for service containers
func (e *executor) getServiceCgroupParent() string {
	if path := e.Config.GetServiceSlotCgroupPath(e.Build.ExecutorData); path != "" {
		return path
	}
	return e.Config.Docker.ServiceCgroupParent
}

func (e *executor) createHostConfig(isBuildContainer, imageIsPrivileged bool) (*container.HostConfig, error) {
	nanoCPUs, err := e.Config.Docker.GetNanoCPUs()
	if err != nil {
		return nil, err
	}

	isolation := container.Isolation(e.Config.Docker.Isolation)
	if !isolation.IsValid() {
		return nil, fmt.Errorf("the isolation value %q is not valid. "+
			"the valid values are: 'process', 'hyperv', 'default' and an empty string", isolation)
	}

	ulimits, err := e.Config.Docker.GetUlimits()
	if err != nil {
		return nil, err
	}

	var useInit *bool
	if isBuildContainer && e.Build.IsFeatureFlagOn(featureflags.UseInitWithDockerExecutor) {
		yes := true
		useInit = &yes
	}

	return &container.HostConfig{
		Resources: container.Resources{
			Memory:            e.Config.Docker.GetMemory(),
			MemorySwap:        e.Config.Docker.GetMemorySwap(),
			MemoryReservation: e.Config.Docker.GetMemoryReservation(),
			CgroupParent:      e.getCgroupParent(),
			CpusetCpus:        e.Config.Docker.CPUSetCPUs,
			CpusetMems:        e.Config.Docker.CPUSetMems,
			CPUShares:         e.Config.Docker.CPUShares,
			NanoCPUs:          nanoCPUs,
			Devices:           e.devices,
			DeviceRequests:    e.deviceRequests,
			OomKillDisable:    e.Config.Docker.GetOomKillDisable(),
			DeviceCgroupRules: e.Config.Docker.DeviceCgroupRules,
			Ulimits:           ulimits,
		},
		DNS:           e.Config.Docker.DNS,
		DNSSearch:     e.Config.Docker.DNSSearch,
		Runtime:       e.Config.Docker.Runtime,
		Privileged:    e.Config.Docker.Privileged && imageIsPrivileged,
		GroupAdd:      e.Config.Docker.GroupAdd,
		UsernsMode:    container.UsernsMode(e.Config.Docker.UsernsMode),
		CapAdd:        e.Config.Docker.CapAdd,
		CapDrop:       e.Config.Docker.CapDrop,
		SecurityOpt:   e.Config.Docker.SecurityOpt,
		RestartPolicy: neverRestartPolicy,
		ExtraHosts:    append(e.Config.Docker.ExtraHosts, e.links...),
		NetworkMode:   e.networkMode,
		IpcMode:       container.IpcMode(e.Config.Docker.IpcMode),
		Links:         e.Config.Docker.Links,
		Binds:         e.volumesManager.Binds(),
		OomScoreAdj:   e.Config.Docker.OomScoreAdjust,
		ShmSize:       e.Config.Docker.ShmSize,
		Isolation:     isolation,
		VolumeDriver:  e.Config.Docker.VolumeDriver,
		VolumesFrom:   e.Config.Docker.VolumesFrom,
		LogConfig:     e.logConfig,
		Tmpfs:         e.Config.Docker.Tmpfs,
		Sysctls:       e.Config.Docker.SysCtls,
		Init:          useInit,
	}, nil
}

func (e *executor) startAndWatchContainer(ctx context.Context, id string, input io.Reader) error {
	dockerExec := exec.NewDocker(e.Context, e.dockerConn, e.waiter, e.Build.Log())

	stdout := e.BuildLogger.Stream(buildlogger.StreamWorkLevel, buildlogger.Stdout)
	defer stdout.Close()

	stderr := e.BuildLogger.Stream(buildlogger.StreamWorkLevel, buildlogger.Stderr)
	defer stderr.Close()

	streams := exec.IOStreams{
		Stdin:  input,
		Stdout: stdout,
		Stderr: stderr,
	}

	var gracefulExitFunc wait.GracefulExitFunc
	if id == e.buildContainerID && e.helperImageInfo.OSType != helperimage.OSTypeWindows {
		// send SIGTERM to all processes in the build container.
		gracefulExitFunc = e.sendSIGTERMToContainerProcs
	}

	err := dockerExec.Exec(ctx, id, streams, gracefulExitFunc)

	// if the context is canceled we attempt to remove the container,
	// as Exec making calls such as ContainerAttach that are canceled
	// can leave the container in a state that cannot easily be recovered
	// from.
	if ctx.Err() != nil {
		_ = e.removeContainer(e.Context, id)
	}

	return err
}

func (e *executor) removeContainer(ctx context.Context, id string) error {
	e.BuildLogger.Debugln("Removing container", id)

	e.disconnectNetwork(ctx, id)

	options := container.RemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}

	err := e.dockerConn.ContainerRemove(ctx, id, options)
	if docker.IsErrNotFound(err) {
		return nil
	}
	if err != nil {
		e.BuildLogger.Debugln("Removing container", id, "finished with error", err)
		return fmt.Errorf("removing container: %w", err)
	}

	e.BuildLogger.Debugln("Removed container", id)
	return nil
}

func (e *executor) disconnectNetwork(ctx context.Context, id string) {
	e.BuildLogger.Debugln("Disconnecting container", id, "from networks")

	netList, err := e.dockerConn.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		e.BuildLogger.Debugln("Can't get network list. ListNetworks exited with", err)
		return
	}

	for _, network := range netList {
		for _, pluggedContainer := range network.Containers {
			if id == pluggedContainer.Name {
				err = e.dockerConn.NetworkDisconnect(ctx, network.ID, id, true)
				if err != nil {
					e.BuildLogger.Warningln(
						"Can't disconnect possibly zombie container",
						pluggedContainer.Name,
						"from network",
						network.Name,
						"->",
						err,
					)
				} else {
					e.BuildLogger.Warningln(
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
	options := common.VerifyAllowedImageOptions{
		Image:          image,
		OptionName:     optionName,
		AllowedImages:  allowedImages,
		InternalImages: internalImages,
	}
	return common.VerifyAllowedImage(options, e.BuildLogger)
}

func (e *executor) expandImageName(imageName string, allowedInternalImages []string) (string, error) {
	defaultDockerImage := e.ExpandValue(e.Config.Docker.Image)
	if imageName != "" {
		image := e.ExpandValue(imageName)
		allowedInternalImages = append(allowedInternalImages, defaultDockerImage)
		err := e.verifyAllowedImage(image, "images", e.Config.Docker.AllowedImages, allowedInternalImages)
		if err != nil {
			return "", err
		}
		return image, nil
	}

	if defaultDockerImage == "" {
		return "", errors.New("no Docker image specified to run the build in")
	}

	e.BuildLogger.
		WithFields(logrus.Fields{
			"executor": "docker",
			"image":    defaultDockerImage,
		}).
		Infoln("Using default image")

	return defaultDockerImage, nil
}

func (e *executor) overwriteEntrypoint(image *spec.Image) []string {
	if len(image.Entrypoint) > 0 {
		if !e.Config.Docker.DisableEntrypointOverwrite {
			return image.Entrypoint
		}

		e.BuildLogger.Warningln("Entrypoint override disabled")
	}

	return nil
}

func connectDocker(ctx context.Context, options common.ExecutorPrepareOptions, e *executor) error {
	_ = e.dockerConn.Close()

	dockerConnection, err := createDockerConnection(ctx, options, e)
	if err != nil {
		return fmt.Errorf("creating docker connection: %w", err)
	}

	info, err := dockerConnection.Info(ctx)
	if err != nil {
		return fmt.Errorf("getting docker info: %w", err)
	}

	serverVersion, err := dockerConnection.ServerVersion(ctx)
	if err != nil {
		return fmt.Errorf("getting server version info: %w", err)
	}

	serverAPIVersion, err := version.NewVersion(serverVersion.APIVersion)
	if err != nil {
		return fmt.Errorf("parsing server API version %q: %w", serverVersion.APIVersion, err)
	}

	if err := validateOSType(info); err != nil {
		return err
	}

	e.BuildLogger.Debugln(fmt.Sprintf(
		"Connected to docker daemon (client version: %s, server version: %s, api version: %s, kernel: %s, os: %s/%s)",
		dockerConnection.ClientVersion(),
		info.ServerVersion,
		serverVersion.APIVersion,
		info.KernelVersion,
		info.OSType,
		info.Architecture,
	))

	e.dockerConn = dockerConnection
	e.info = info
	e.serverAPIVersion = serverAPIVersion
	e.waiter = wait.NewDockerKillWaiter(dockerConnection)

	return nil
}

type contextDialerFunc = func(ctx context.Context, network, addr string) (net.Conn, error)

func environmentDialContext(
	ctx context.Context,
	executorClient executors.Client,
	host string,
	useDockerAutoscalerDialStdio bool,
) (string, contextDialerFunc, error) {
	systemHost := host == ""
	if host == "" {
		host = os.Getenv("DOCKER_HOST")
	}
	if host == "" {
		host = client.DefaultDockerHost
	}

	u, err := client.ParseHostURL(host)
	if err != nil {
		return "", nil, fmt.Errorf("parsing docker host: %w", err)
	}

	if !useDockerAutoscalerDialStdio {
		return u.Scheme, func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := executorClient.Dial(u.Scheme, u.Host)
			if err != nil {
				return nil, fmt.Errorf("dialing environment connection: %w", err)
			}

			return conn, nil
		}, nil
	}

	return "dial-stdio", func(_ context.Context, network, addr string) (net.Conn, error) {
		// DialRun doesn't want just a context for dialing, but one for a long-lived connection, including cleanup.
		// We don't want this context to be cancelled when the job is cancelled or times out since that would prevent
		// cleanup.

		// if the host was explicit, we try to use this even with dial-stdio
		cmd := fmt.Sprintf("docker -H %s system dial-stdio", host)

		// rather than use this system's host, we use the remote system's default
		if systemHost {
			cmd = "docker system dial-stdio"
		}
		return executorClient.DialRun(ctx, cmd)
	}, nil
}

// validateOSType checks if the ExecutorOptions metadata matches with the docker
// info response.
func validateOSType(info system.Info) error {
	switch info.OSType {
	case osTypeLinux, osTypeWindows, osTypeFreeBSD:
		return nil
	}

	return fmt.Errorf("unsupported os type: %s", info.OSType)
}

func (e *executor) createDependencies() error {
	createDependenciesStrategy := []func() error{
		e.createLabeler,
		e.createNetworksManager,
		e.createBuildNetwork,
		e.createPullManager,
		e.bindDevices,
		e.bindDeviceRequests,
		e.createVolumesManager,
		e.createVolumes,
		e.createBuildVolume,
		e.bootstrap,
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
	e.BuildLogger.Debugln("Creating user-defined volumes...")

	if e.volumesManager == nil {
		return errVolumesManagerUndefined
	}

	for _, volume := range e.Config.Docker.Volumes {
		err := e.volumesManager.Create(e.Context, volume)
		if errors.Is(err, volumes.ErrCacheVolumesDisabled) {
			e.BuildLogger.Warningln(fmt.Sprintf(
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
	e.BuildLogger.Debugln("Creating build volume...")

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

	var err error
	e.logConfig, err = options.Config.Docker.GetLogConfig()
	if err != nil {
		return &common.BuildError{
			Inner:         fmt.Errorf("creating docker log configuration: %w", err),
			FailureReason: common.RunnerSystemFailure,
		}
	}

	err = e.dockerConnector.Connect(e.Context, options, e)
	if err != nil {
		return err
	}

	e.helperImageInfo, err = e.prepareHelperImage()
	if err != nil {
		return err
	}

	// setup default executor options based on OS type
	e.setupDefaultExecutorOptions(e.helperImageInfo.OSType)

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

	e.BuildLogger.Println("Using Docker executor with image", imageName, "...")

	err = e.createDependencies()
	if err != nil {
		return err
	}
	return nil
}

func (e *executor) setupDefaultExecutorOptions(os string) {
	switch os {
	case helperimage.OSTypeWindows:
		e.DefaultBuildsDir = `C:\builds`
		e.DefaultCacheDir = `C:\cache`

		e.ExecutorOptions.Shell.Shell = shells.SNPowershell
		e.ExecutorOptions.Shell.RunnerCommand = "gitlab-runner-helper"

		if e.volumeParser == nil {
			e.volumeParser = parser.NewWindowsParser(e.ExpandValue)
		}

		if e.newVolumePermissionSetter == nil {
			e.newVolumePermissionSetter = func() (permission.Setter, error) {
				return permission.NewDockerWindowsSetter(), nil
			}
		}

	default:
		e.DefaultBuildsDir = `/builds`
		e.DefaultCacheDir = `/cache`

		e.ExecutorOptions.Shell.Shell = "bash"
		e.ExecutorOptions.Shell.RunnerCommand = "/usr/bin/gitlab-runner-helper"

		if e.volumeParser == nil {
			e.volumeParser = parser.NewLinuxParser(e.ExpandValue)
		}

		if e.newVolumePermissionSetter == nil {
			e.newVolumePermissionSetter = func() (permission.Setter, error) {
				helperImage, err := e.getHelperImage()
				if err != nil {
					return nil, err
				}

				return permission.NewDockerLinuxSetter(e.dockerConn, e.Build.Log(), helperImage), nil
			}
		}
	}
}

func (e *executor) prepareHelperImage() (helperimage.Info, error) {
	return helperimage.Get(common.AppVersion.Version, helperimage.Config{
		OSType:        e.info.OSType,
		Architecture:  e.info.Architecture,
		KernelVersion: e.info.KernelVersion,
		Shell:         e.Config.Shell,
		Flavor:        e.ExpandValue(e.Config.Docker.HelperImageFlavor),
		ProxyExec:     e.Config.IsProxyExec(),
	})
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
	if e.Config.Docker == nil {
		// if there's no Docker config, we got here because Prepare() failed
		// and there's nothing to cleanup.
		return
	}

	e.SetCurrentStage(ExecutorStageCleanup)

	var wg sync.WaitGroup

	// create a new context for cleanup in case the main context has expired or been cancelled.
	ctx, cancel := context.WithTimeout(context.Background(), dockerCleanupTimeout)
	defer cancel()

	defer func() {
		if err := e.dockerConn.Close(); err != nil {
			e.BuildLogger.WithFields(logrus.Fields{"error": err}).Debugln("Failed to close the client")
		}
	}()

	remove := func(id string) {
		wg.Add(1)
		go func() {
			if err := e.removeContainer(ctx, id); err != nil {
				e.BuildLogger.WithFields(logrus.Fields{"error": err}).Errorln("Failed to remove container", id)
			}
			wg.Done()
		}()
	}

	for _, temporaryID := range e.temporary {
		remove(temporaryID)
	}

	wg.Wait()

	if err := e.cleanupVolume(ctx); err != nil {
		e.BuildLogger.WithFields(logrus.Fields{"error": err}).Errorln("Failed to cleanup volumes")
	}

	if err := e.cleanupNetwork(ctx); err != nil {
		e.BuildLogger.WithFields(logrus.Fields{
			"network": e.networkMode.NetworkName(),
			"error":   err,
		}).Errorln("Failed to remove network for build")
	}

	e.AbstractExecutor.Cleanup()
}

// sendSIGTERMToContainerProcs exec's into the specified container and executes the script
// shells.sendSIGTERMToContainerProcs, which (unsurprisingly) sends SIGTERM to all processes in the container. This
// Effectively gives the processes in a the container a chance to exit gracefully (if they listen for SIGTERM).
func (e *executor) sendSIGTERMToContainerProcs(ctx context.Context, containerID string) error {
	e.BuildLogger.Debugln("Emitting SIGTERM to processes in container", containerID)
	return e.execScriptOnContainer(ctx, containerID, shells.ContainerSigTermScriptForLinux)
}

// Because docker error types are in fact interfaces with a unique identifying method, it's not possible to use
// errors.Is or errors.As on them. And because we wrap those errors as they are returned up the chain, we can't use
// errdefs directly. Do this instead.
func shouldIgnoreDockerError(err error, isFuncs ...func(error) bool) bool {
	if err == nil {
		return true
	}
	for e := err; e != nil; e = errors.Unwrap(e) {
		for _, is := range isFuncs {
			if is(e) {
				return true
			}
		}
	}
	return false
}

func (e *executor) execScriptOnContainer(ctx context.Context, containerID string, script ...string) (err error) {
	action := ""
	execConfig := container.ExecOptions{
		Tty:          false,
		AttachStderr: true,
		AttachStdout: true,
		Cmd:          append([]string{"sh", "-c"}, script...),
	}

	defer func() {
		if !shouldIgnoreDockerError(err, errdefs.IsConflict, errdefs.IsNotFound) {
			e.Config.Log().WithFields(logrus.Fields{"error": err}).Warningln(action, err)
		}
	}()

	exec, err := e.dockerConn.ContainerExecCreate(ctx, containerID, execConfig)
	if err != nil {
		action = "Failed to exec create to container:"
		return err
	}

	resp, err := e.dockerConn.ContainerExecAttach(ctx, exec.ID, container.ExecStartOptions{})
	if err != nil {
		action = "Failed to exec attach to container:"
		return err
	}
	defer resp.Close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		<-ctx.Done()
		resp.Close()
	}()

	// Copy any output generated by running the script (typically there will be none) to runner's stdout/stderr...
	_, err = stdcopy.StdCopy(os.Stdout, os.Stderr, resp.Reader)
	if err != nil {
		action = "Failed to read from attached container:"
		return err
	}

	return nil
}

func (e *executor) cleanupVolume(ctx context.Context) error {
	if e.volumesManager == nil {
		e.BuildLogger.Debugln("Volumes manager is empty, skipping volumes cleanup")
		return nil
	}

	err := e.volumesManager.RemoveTemporary(ctx)
	if err != nil {
		return fmt.Errorf("remove temporary volumes: %w", err)
	}

	return nil
}

func (e *executor) createHostConfigForServiceHealthCheck(service *serviceInfo) *container.HostConfig {
	var extraHosts []string

	// we only get a service IP from the default network, for other networks, Docker
	// already provides DNS entries
	for _, ip := range service.IP {
		extraHosts = []string{service.ID[:min(12, len(service.ID))] + ":" + ip}
	}

	return &container.HostConfig{
		RestartPolicy: neverRestartPolicy,
		ExtraHosts:    extraHosts,
		NetworkMode:   e.networkMode,
		LogConfig:     e.logConfig,
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
func (e *executor) addServiceHealthCheckEnvironment(service *serviceInfo) ([]string, error) {
	environment := []string{}

	if len(service.Ports) == 0 {
		return environment, fmt.Errorf("service %q has no exposed ports", service.Name)
	}

	environment = append(environment, "WAIT_FOR_SERVICE_TCP_ADDR="+service.ID[:12])
	for _, port := range service.Ports {
		environment = append(environment, fmt.Sprintf("WAIT_FOR_SERVICE_%d_TCP_PORT=%d", port, port))
	}

	return environment, nil
}

//nolint:gocognit
func (e *executor) getContainerIPAndExposedPorts(id string) ([]string, []int, error) {
	// We either wait for the user's provided timeout, or our default, whichever is larger.
	//
	// The reason we don't wait for the smaller timeout is because users often set WaitForServicesTimeout=-1,
	// or a low number, to indicate they want to skip the healthcheck. In this scenario, we're not using
	// it for the healthcheck, but the wait for the container to come up.
	timeout := max(e.Config.Docker.WaitForServicesTimeout, common.DefaultWaitForServicesTimeout)

	var inspect container.InspectResponse
	start := time.Now()
	for {
		if time.Since(start) > time.Duration(timeout)*time.Second {
			return nil, nil, fmt.Errorf("service failed to start after %v", time.Since(start))
		}

		var err error
		inspect, err = e.dockerConn.ContainerInspect(e.Context, id)
		if err != nil {
			return nil, nil, err
		}

		if inspect.State.Status != container.StateCreated {
			break
		}
		time.Sleep(time.Second)
	}

	var ip []string
	if inspect.NetworkSettings.IPAddress != "" { //nolint:staticcheck
		ip = append(ip, inspect.NetworkSettings.IPAddress) //nolint:staticcheck
	}
	if inspect.NetworkSettings.GlobalIPv6Address != "" { //nolint:staticcheck
		ip = append(ip, inspect.NetworkSettings.GlobalIPv6Address) //nolint:staticcheck
	}

	for _, env := range inspect.Config.Env {
		key, val, ok := strings.Cut(env, "=")
		if !ok {
			continue
		}

		if strings.EqualFold(key, "HEALTHCHECK_TCP_PORT") {
			port, err := strconv.ParseInt(val, 10, 32)
			if err != nil {
				return nil, nil, fmt.Errorf("invalid health check tcp port: %v", val)
			}

			return ip, []int{int(port)}, nil
		}
	}

	// maxPortsCheck is the maximum number of ports that we'll check to see
	// if a service is running
	const maxPortsCheck = 20

	var ports []int
	for port := range inspect.Config.ExposedPorts {
		start, end, err := port.Range()
		if err == nil && port.Proto() == "tcp" {
			for i := start; i <= end && len(ports) < maxPortsCheck; i++ {
				ports = append(ports, i)
			}
		}
	}

	sort.Ints(ports)

	return ip, ports, nil
}

func (e *executor) readContainerLogs(containerID string) string {
	var buf bytes.Buffer

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
	}

	hijacked, err := e.dockerConn.ContainerLogs(e.Context, containerID, options)
	if err != nil {
		return strings.TrimSpace(err.Error())
	}
	defer func() { _ = hijacked.Close() }()

	// limit how much data we read from the container log to
	// avoid memory exhaustion
	w := limitwriter.New(&buf, ServiceLogOutputLimit)

	_, _ = stdcopy.StdCopy(w, w, hijacked)
	return strings.TrimSpace(buf.String())
}

// prepareContainerLabels returns a map of the default labels combined with the passed otherLabels
// and the docker labels from the config.
func (e *executor) prepareContainerLabels(otherLabels map[string]string) map[string]string {
	l := e.labeler.Labels(otherLabels)

	for k, v := range e.Config.Docker.ContainerLabels {
		l[k] = e.Build.Variables.ExpandValue(v)
	}

	return l
}
