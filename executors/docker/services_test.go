//go:build !integration

package docker

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/system"
	"github.com/docker/go-connections/nat"
	"github.com/hashicorp/go-version"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/pull"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/helperimage"
	service_test "gitlab.com/gitlab-org/gitlab-runner/helpers/container/services/test"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	service_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/service"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/trace"
)

func testServiceFromNamedImage(t *testing.T, description, imageName, serviceName string) {
	c := docker.NewMockClient(t)
	p := pull.NewMockManager(t)

	servicePart := fmt.Sprintf("-%s-0", strings.ReplaceAll(serviceName, "/", "__"))
	containerNameRegex, err := regexp.Compile("runner-abcdef123-project-0-concurrent-0-[^-]+" + servicePart)
	require.NoError(t, err)

	containerNameMatcher := mock.MatchedBy(containerNameRegex.MatchString)
	networkID := "network-id"

	e := &executor{
		dockerConn: &dockerConnection{Client: c},
		info: system.Info{
			OSType:       helperimage.OSTypeLinux,
			Architecture: "amd64",
		},
		pullManager: p,
	}

	e.Config = common.RunnerConfig{}
	e.Config.Docker = &common.DockerConfig{}
	e.Build = &common.Build{
		ProjectRunnerID: 0,
		Runner:          &common.RunnerConfig{},
	}
	e.Build.JobInfo.ProjectID = 0
	e.Build.Runner.Token = "abcdef1234567890"
	e.Context = t.Context()

	e.helperImageInfo, err = helperimage.Get(common.AppVersion.Version, helperimage.Config{
		OSType:        e.info.OSType,
		Architecture:  e.info.Architecture,
		KernelVersion: e.info.KernelVersion,
	})
	require.NoError(t, err)

	e.serverAPIVersion = version.Must(version.NewVersion("1.43"))

	err = e.createLabeler()
	require.NoError(t, err)

	e.BuildShell = &common.ShellConfiguration{}

	realServiceContainerName := e.getProjectUniqRandomizedName() + servicePart
	options := common.ImageDockerOptions{}

	p.On("GetDockerImage", imageName, options, []common.DockerPullPolicy(nil)).
		Return(&types.ImageInspect{ID: "helper-image"}, nil).
		Once()

	c.On(
		"ContainerRemove",
		e.Context,
		containerNameMatcher,
		container.RemoveOptions{RemoveVolumes: true, Force: true},
	).
		Return(nil).
		Once()

	networkContainersMap := map[string]network.EndpointResource{
		"1": {Name: realServiceContainerName},
	}

	c.On("NetworkList", e.Context, network.ListOptions{}).
		Return([]network.Summary{{ID: networkID, Name: "network-name", Containers: networkContainersMap}}, nil).
		Once()

	c.On("NetworkDisconnect", e.Context, networkID, containerNameMatcher, true).
		Return(nil).
		Once()

	c.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(container.CreateResponse{ID: realServiceContainerName}, nil).
		Once()

	c.On("ContainerStart", e.Context, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	err = e.createVolumesManager()
	require.NoError(t, err)

	linksMap := make(map[string]*types.Container)
	err = e.createFromServiceDefinition(0, common.Image{Name: description}, linksMap)
	assert.NoError(t, err)
}

func TestServiceFromNamedImage(t *testing.T) {
	for _, test := range service_test.Services {
		t.Run(test.Description, func(t *testing.T) {
			testServiceFromNamedImage(t, test.Description, test.Image, test.Service)
		})
	}
}

func testDockerConfigurationWithServiceContainer(
	t *testing.T,
	dockerConfig *common.DockerConfig,
	cce containerConfigExpectations,
) {
	c, e := prepareTestDockerConfiguration(t, dockerConfig, cce, "alpine:latest", "alpine:latest")

	c.On("ContainerStart", mock.Anything, "abc", mock.Anything).
		Return(nil).Once()

	err := e.createVolumesManager()
	require.NoError(t, err)

	err = e.createPullManager()
	require.NoError(t, err)

	_, err = e.createService(
		0,
		"alpine",
		"latest",
		"alpine:latest",
		common.Image{Name: "alpine", Command: []string{"/bin/sh"}},
		nil,
	)
	assert.NoError(t, err, "Should create service container without errors")
}

func TestDockerServicesTmpfsSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		ServicesTmpfs: map[string]string{
			"/tmpfs": "rw,noexec",
		},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		require.NotEmpty(t, hostConfig.Tmpfs)
	}

	testDockerConfigurationWithServiceContainer(t, dockerConfig, cce)
}

func TestDockerServicesDNSSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		DNS: []string{"2001:db8::1", "192.0.2.1"},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		require.Equal(t, dockerConfig.DNS, hostConfig.DNS)
	}

	testDockerConfigurationWithServiceContainer(t, dockerConfig, cce)
}

func TestDockerServicesDNSSearchSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		DNSSearch: []string{"mydomain.example"},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		require.Equal(t, dockerConfig.DNSSearch, hostConfig.DNSSearch)
	}

	testDockerConfigurationWithServiceContainer(t, dockerConfig, cce)
}

func TestDockerServicesExtraHostsSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		ExtraHosts: []string{"foo.example:2001:db8::1", "bar.example:192.0.2.1"},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		require.Equal(t, dockerConfig.ExtraHosts, hostConfig.ExtraHosts)
	}

	testDockerConfigurationWithServiceContainer(t, dockerConfig, cce)
}

func TestDockerServiceUserNSSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{}
	dockerConfigWithHostUsernsMode := &common.DockerConfig{
		UsernsMode: "host",
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		assert.Equal(t, container.UsernsMode(""), hostConfig.UsernsMode)
	}
	cceWithHostUsernsMode := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		assert.Equal(t, container.UsernsMode("host"), hostConfig.UsernsMode)
	}

	testDockerConfigurationWithServiceContainer(t, dockerConfig, cce)
	testDockerConfigurationWithServiceContainer(t, dockerConfigWithHostUsernsMode, cceWithHostUsernsMode)
}

type testAllowedPrivilegedServiceDescription struct {
	expectedPrivileged bool
	privileged         bool
	allowedImages      []string
}

var testAllowedPrivilegedService = []testAllowedPrivilegedServiceDescription{
	{true, true, []string{}},
	{true, true, []string{"*"}},
	{false, true, []string{"*:*"}},
	{false, true, []string{"*/*"}},
	{false, true, []string{"*/*:*"}},
	{true, true, []string{"**/*"}},
	{false, true, []string{"**/*:*"}},
	{true, true, []string{"alpine"}},
	{false, true, []string{"debian"}},
	{true, true, []string{"alpi*"}},
	{true, true, []string{"*alpi*"}},
	{true, true, []string{"*alpi*"}},
	{true, true, []string{"debian", "alpine"}},
	{true, true, []string{"debian", "*"}},
	{false, false, []string{}},
	{false, false, []string{"*"}},
	{false, false, []string{"*:*"}},
	{false, false, []string{"*/*"}},
	{false, false, []string{"*/*:*"}},
	{false, false, []string{"**/*"}},
	{false, false, []string{"**/*:*"}},
	{false, false, []string{"alpine"}},
	{false, false, []string{"debian"}},
	{false, false, []string{"alpi*"}},
	{false, false, []string{"*alpi*"}},
	{false, false, []string{"*alpi*"}},
	{false, false, []string{"debian", "alpine"}},
	{false, false, []string{"debian", "*"}},
}

func TestDockerServicePrivilegedSetting(t *testing.T) {
	for _, test := range testAllowedPrivilegedService {
		dockerConfigWithoutServicePrivileged := &common.DockerConfig{
			Privileged:                test.privileged,
			ServicesPrivileged:        nil,
			AllowedPrivilegedServices: test.allowedImages,
		}
		dockerConfigWithPrivileged := &common.DockerConfig{
			Privileged:                true,
			ServicesPrivileged:        &test.privileged,
			AllowedPrivilegedServices: test.allowedImages,
		}
		dockerConfigWithoutPrivileged := &common.DockerConfig{
			Privileged:                false,
			ServicesPrivileged:        &test.privileged,
			AllowedPrivilegedServices: test.allowedImages,
		}

		cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
			var message string
			if test.expectedPrivileged {
				message = "%q must be allowed by %q"
			} else {
				message = "%q must not be allowed by %q"
			}
			assert.Equal(t, test.expectedPrivileged, hostConfig.Privileged, message, "alpine", test.allowedImages)
		}

		testDockerConfigurationWithServiceContainer(t, dockerConfigWithoutServicePrivileged, cce)
		testDockerConfigurationWithServiceContainer(t, dockerConfigWithPrivileged, cce)
		testDockerConfigurationWithServiceContainer(t, dockerConfigWithoutPrivileged, cce)
	}
}

func TestDockerWithNoDockerConfigAndWithServiceImagePullPolicyAlways(t *testing.T) {
	dockerConfig := &common.DockerConfig{}
	serviceConfig := common.Image{
		Name:         "alpine",
		PullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
	}

	c, e := prepareTestDockerConfiguration(t, dockerConfig, cce, "alpine:latest", "alpine:latest")

	c.On("ContainerStart", mock.Anything, "abc", mock.Anything).
		Return(nil).Once()

	err := e.createVolumesManager()
	require.NoError(t, err)

	err = e.createPullManager()
	require.NoError(t, err)

	_, err = e.createService(
		0,
		"alpine",
		"latest",
		"alpine:latest",
		serviceConfig,
		nil,
	)
	assert.NoError(t, err, "Should create service container without errors")
}

func TestDockerWithDockerConfigAlwaysAndIfNotPresentAndWithServiceImagePullPolicyIfNotPresent(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		PullPolicy: common.StringOrArray{common.PullPolicyAlways, common.PullPolicyIfNotPresent},
	}
	serviceConfig := common.Image{
		Name:         "alpine",
		PullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
	}

	c, e := createExecutorForTestDockerConfiguration(t, dockerConfig, cce)

	c.On("ImageInspectWithRaw", mock.Anything, "alpine:latest").
		Return(types.ImageInspect{ID: "123"}, []byte{}, nil).Once()
	c.On("NetworkList", mock.Anything, mock.Anything).
		Return([]network.Summary{}, nil).Once()
	c.On("ContainerRemove", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	c.On("ContainerStart", mock.Anything, "abc", mock.Anything).
		Return(nil).Once()

	err := e.createVolumesManager()
	require.NoError(t, err)

	err = e.createPullManager()
	require.NoError(t, err)

	_, err = e.createService(
		0,
		"alpine",
		"latest",
		"alpine:latest",
		serviceConfig,
		nil,
	)
	assert.NoError(t, err, "Should create service container without errors")
}

func TestDockerWithDockerConfigAlwaysButNotAllowedAndWithNoServiceImagePullPolicy(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		PullPolicy:          common.StringOrArray{common.PullPolicyAlways},
		AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
	}
	serviceConfig := common.Image{Name: "alpine"}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
	}
	_, e := createExecutorForTestDockerConfiguration(t, dockerConfig, cce)

	err := e.createVolumesManager()
	require.NoError(t, err)

	err = e.createPullManager()
	require.NoError(t, err)

	_, err = e.createService(
		0,
		"alpine",
		"latest",
		"alpine:latest",
		serviceConfig,
		nil,
	)
	assert.Contains(
		t,
		err.Error(),
		`invalid pull policy for image "alpine:latest"`,
	)
	assert.Regexp(t, regexp.MustCompile(`always.* Runner config .*if-not-present`), err.Error())
}

func TestDockerWithDockerConfigAlwaysAndWithServiceImagePullPolicyIfNotPresent(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		PullPolicy:          common.StringOrArray{common.PullPolicyAlways},
		AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways},
	}
	serviceConfig := common.Image{
		Name:         "alpine",
		PullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
	}
	_, e := createExecutorForTestDockerConfiguration(t, dockerConfig, cce)

	err := e.createVolumesManager()
	require.NoError(t, err)

	err = e.createPullManager()
	require.NoError(t, err)

	_, err = e.createService(
		0,
		"alpine",
		"latest",
		"alpine:latest",
		serviceConfig,
		nil,
	)
	assert.Contains(
		t,
		err.Error(),
		`invalid pull policy for image "alpine:latest"`,
	)
	assert.Regexp(t, regexp.MustCompile(`if-not-present.* GitLab pipeline config .*always`), err.Error())
}

func TestGetServiceDefinitions(t *testing.T) {
	e := new(executor)
	e.Build = &common.Build{
		Runner: &common.RunnerConfig{},
	}
	e.Config = common.RunnerConfig{}
	e.Config.Docker = &common.DockerConfig{}

	testServicesLimit := func(i int) *int {
		return &i
	}

	tests := map[string]struct {
		services         []common.Service
		servicesLimit    *int
		buildServices    []common.Image
		allowedServices  []string
		expectedServices common.Services
		expectedErr      string
	}{
		"all services with proper name and alias": {
			services: []common.Service{
				{
					Name:       "name",
					Alias:      "alias",
					Command:    []string{"executable", "param1", "param2"},
					Entrypoint: []string{"executable", "param3", "param4"},
				},
				{
					Name:    "name2",
					Alias:   "alias2",
					Command: []string{"executable", "param1", "param2"},
				},
				{
					Name:       "name3",
					Alias:      "alias3",
					Entrypoint: []string{"executable", "param3", "param4"},
				},
			},
			expectedServices: common.Services{
				{
					Name:       "name",
					Alias:      "alias",
					Command:    []string{"executable", "param1", "param2"},
					Entrypoint: []string{"executable", "param3", "param4"},
				},
				{
					Name:    "name2",
					Alias:   "alias2",
					Command: []string{"executable", "param1", "param2"},
				},
				{
					Name:       "name3",
					Alias:      "alias3",
					Entrypoint: []string{"executable", "param3", "param4"},
				},
			},
		},
		"build service not in internal images but empty allowed services": {
			services: []common.Service{
				{
					Name:  "name",
					Alias: "alias",
				},
			},
			buildServices: []common.Image{
				{
					Name: "name_not_in_internal",
				},
			},
			expectedServices: common.Services{
				{
					Name:  "name",
					Alias: "alias",
				},
				{
					Name: "name_not_in_internal",
				},
			},
		},
		"build service not in internal images": {
			services: []common.Service{
				{
					Name: "name",
				},
			},
			buildServices: []common.Image{
				{
					Name: "name_not_in_internal",
				},
			},
			allowedServices: []string{"name"},
			expectedErr:     "disallowed image",
		},
		"build service not in allowed services but in internal images": {
			services: []common.Service{
				{
					Name: "name",
				},
			},
			buildServices: []common.Image{
				{
					Name: "name",
				},
			},
			allowedServices: []string{"allowed_name"},
			expectedServices: common.Services{
				{
					Name: "name",
				},
				{
					Name: "name",
				},
			},
		},
		"empty service name": {
			services: []common.Service{
				{
					Name: "",
				},
			},
			buildServices: []common.Image{},
			expectedServices: common.Services{
				{
					Name: "",
				},
			},
		},
		"requested 1 service, max 0": {
			services: []common.Service{
				{
					Name: "name",
				},
			},
			servicesLimit: testServicesLimit(0),
			expectedErr:   (&tooManyServicesRequestedError{requested: 1, allowed: 0}).Error(),
		},
		"requested 1 service, max 1": {
			services: []common.Service{
				{
					Name: "name",
				},
			},
			servicesLimit: testServicesLimit(1),
			expectedServices: common.Services{
				{
					Name: "name",
				},
			},
		},
		"requested 2 services, max 1": {
			services: []common.Service{
				{
					Name: "name",
				},
				{
					Name: "name",
				},
			},
			servicesLimit: testServicesLimit(1),
			expectedErr:   (&tooManyServicesRequestedError{requested: 2, allowed: 1}).Error(),
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			e.Config.Docker.Services = tt.services
			e.Config.Docker.AllowedServices = tt.allowedServices
			e.Config.Docker.ServicesLimit = tt.servicesLimit
			e.Build.Services = tt.buildServices

			svcs, err := e.getServicesDefinitions()
			if tt.expectedErr != "" {
				assert.EqualError(t, err, tt.expectedErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedServices, svcs)
		})
	}
}

func TestAddServiceHealthCheck(t *testing.T) {
	tests := map[string]struct {
		networkMode            string
		dockerClientAssertions func(*docker.MockClient)
		expectedEnvironment    []string
		expectedErr            string
	}{
		"network mode not defined": {
			expectedEnvironment: []string{},
		},
		"get ports via environment": {
			networkMode: "test",
			dockerClientAssertions: func(c *docker.MockClient) {
				c.On("ContainerInspect", mock.Anything, mock.Anything).
					Return(types.ContainerJSON{
						Config: &container.Config{
							ExposedPorts: nat.PortSet{
								"1000/tcp": {},
							},
						},
					}, nil).
					Once()
			},
			expectedEnvironment: []string{
				"WAIT_FOR_SERVICE_TCP_ADDR=000000000000",
				"WAIT_FOR_SERVICE_1000_TCP_PORT=1000",
			},
		},
		"get port from many": {
			networkMode: "test",
			dockerClientAssertions: func(c *docker.MockClient) {
				c.On("ContainerInspect", mock.Anything, mock.Anything).
					Return(types.ContainerJSON{
						Config: &container.Config{
							ExposedPorts: nat.PortSet{
								"1000/tcp":  {},
								"500/udp":   {},
								"600/tcp":   {},
								"1500/tcp":  {},
								"1600-1601": {},
								"1700-1705": {},
							},
						},
					}, nil).
					Once()
			},
			expectedEnvironment: []string{
				"WAIT_FOR_SERVICE_TCP_ADDR=000000000000",
				"WAIT_FOR_SERVICE_600_TCP_PORT=600",
				"WAIT_FOR_SERVICE_1000_TCP_PORT=1000",
				"WAIT_FOR_SERVICE_1500_TCP_PORT=1500",
				"WAIT_FOR_SERVICE_1600_TCP_PORT=1600",
				"WAIT_FOR_SERVICE_1601_TCP_PORT=1601",
				"WAIT_FOR_SERVICE_1700_TCP_PORT=1700",
				"WAIT_FOR_SERVICE_1701_TCP_PORT=1701",
				"WAIT_FOR_SERVICE_1702_TCP_PORT=1702",
				"WAIT_FOR_SERVICE_1703_TCP_PORT=1703",
				"WAIT_FOR_SERVICE_1704_TCP_PORT=1704",
				"WAIT_FOR_SERVICE_1705_TCP_PORT=1705",
			},
		},
		"get port from many (limited to 20)": {
			networkMode: "test",
			dockerClientAssertions: func(c *docker.MockClient) {
				c.On("ContainerInspect", mock.Anything, mock.Anything).
					Return(types.ContainerJSON{
						Config: &container.Config{
							ExposedPorts: nat.PortSet{
								"1000-1100": {},
							},
						},
					}, nil).
					Once()
			},
			expectedEnvironment: []string{
				"WAIT_FOR_SERVICE_TCP_ADDR=000000000000",
				"WAIT_FOR_SERVICE_1000_TCP_PORT=1000",
				"WAIT_FOR_SERVICE_1001_TCP_PORT=1001",
				"WAIT_FOR_SERVICE_1002_TCP_PORT=1002",
				"WAIT_FOR_SERVICE_1003_TCP_PORT=1003",
				"WAIT_FOR_SERVICE_1004_TCP_PORT=1004",
				"WAIT_FOR_SERVICE_1005_TCP_PORT=1005",
				"WAIT_FOR_SERVICE_1006_TCP_PORT=1006",
				"WAIT_FOR_SERVICE_1007_TCP_PORT=1007",
				"WAIT_FOR_SERVICE_1008_TCP_PORT=1008",
				"WAIT_FOR_SERVICE_1009_TCP_PORT=1009",
				"WAIT_FOR_SERVICE_1010_TCP_PORT=1010",
				"WAIT_FOR_SERVICE_1011_TCP_PORT=1011",
				"WAIT_FOR_SERVICE_1012_TCP_PORT=1012",
				"WAIT_FOR_SERVICE_1013_TCP_PORT=1013",
				"WAIT_FOR_SERVICE_1014_TCP_PORT=1014",
				"WAIT_FOR_SERVICE_1015_TCP_PORT=1015",
				"WAIT_FOR_SERVICE_1016_TCP_PORT=1016",
				"WAIT_FOR_SERVICE_1017_TCP_PORT=1017",
				"WAIT_FOR_SERVICE_1018_TCP_PORT=1018",
				"WAIT_FOR_SERVICE_1019_TCP_PORT=1019",
			},
		},
		"get port from container variable": {
			networkMode: "test",
			dockerClientAssertions: func(c *docker.MockClient) {
				c.On("ContainerInspect", mock.Anything, mock.Anything).
					Return(types.ContainerJSON{
						Config: &container.Config{
							ExposedPorts: nat.PortSet{
								"1000/tcp": {},
							},
							Env: []string{
								"HEALTHCHECK_TCP_PORT=2000",
							},
						},
					}, nil).
					Once()
			},
			expectedEnvironment: []string{
				"WAIT_FOR_SERVICE_TCP_ADDR=000000000000",
				"WAIT_FOR_SERVICE_2000_TCP_PORT=2000",
			},
		},
		"get port from container variable - case insensitive": {
			networkMode: "test",
			dockerClientAssertions: func(c *docker.MockClient) {
				c.On("ContainerInspect", mock.Anything, mock.Anything).
					Return(types.ContainerJSON{
						Config: &container.Config{
							ExposedPorts: nat.PortSet{
								"1000/tcp": {},
							},
							Env: []string{
								"healthcheck_TCP_PORT=2000",
							},
						},
					}, nil).
					Once()
			},
			expectedEnvironment: []string{
				"WAIT_FOR_SERVICE_TCP_ADDR=000000000000",
				"WAIT_FOR_SERVICE_2000_TCP_PORT=2000",
			},
		},
		"get port from container variable (invalid)": {
			networkMode: "test",
			dockerClientAssertions: func(c *docker.MockClient) {
				c.On("ContainerInspect", mock.Anything, mock.Anything).
					Return(types.ContainerJSON{
						Config: &container.Config{
							ExposedPorts: nat.PortSet{
								"1000/tcp": {},
							},
							Env: []string{
								"HEALTHCHECK_TCP_PORT=hello",
							},
						},
					}, nil).
					Once()
			},
			expectedErr: fmt.Sprintf("get container exposed ports: invalid health check tcp port: %v", "hello"),
		},
		"no ports defined": {
			networkMode: "test",
			dockerClientAssertions: func(c *docker.MockClient) {
				c.On("ContainerInspect", mock.Anything, mock.Anything).
					Return(types.ContainerJSON{
						Config: &container.Config{
							ExposedPorts: nat.PortSet{},
						},
					}, nil).
					Once()
			},
			expectedErr: fmt.Sprintf("service %q has no exposed ports", "default"),
		},
		"container inspect error": {
			networkMode: "test",
			dockerClientAssertions: func(c *docker.MockClient) {
				c.On("ContainerInspect", mock.Anything, mock.Anything).
					Return(types.ContainerJSON{}, fmt.Errorf("%v", "test error")).
					Once()
			},
			expectedErr: fmt.Sprintf("get container exposed ports: %v", "test error"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			client := docker.NewMockClient(t)

			if test.dockerClientAssertions != nil {
				test.dockerClientAssertions(client)
			}

			executor := &executor{
				networkMode: container.NetworkMode(test.networkMode),
				dockerConn:  &dockerConnection{Client: client},
			}

			service := &types.Container{
				ID:    "0000000000000000000000000000000000000000000000000000000000000000",
				Names: []string{"default"},
			}

			environment, err := executor.addServiceHealthCheckEnvironment(service)

			if test.expectedErr != "" {
				assert.Error(t, err)
				assert.ErrorContains(t, err, test.expectedErr)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedEnvironment, environment)
			}
		})
	}
}

func Test_Executor_captureContainerLogs(t *testing.T) {
	const (
		cID   = "some container"
		cName = cID
		msg   = "pretend this is a log generated by a process in a container"
	)

	tests := map[string]struct {
		header  []byte
		wantLog string
		wantErr error
	}{
		"success": {
			// for header spec see https://pkg.go.dev/github.com/moby/moby/client#Client.ContainerLogs
			header:  []byte{1, 0, 0, 0, 0, 0, 0, byte(len(msg))},
			wantLog: msg,
		},
		"read error": {
			wantLog: "error streaming logs for container some container: Unrecognized input header:",
		},
		"connect error": {
			wantErr: errors.New("blammo"),
			wantLog: "failed to open log stream for container " + cName + ": blammo",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			c := docker.NewMockClient(t)
			e := &executor{}
			e.dockerConn = &dockerConnection{Client: c}

			buf, err := trace.New()
			require.NoError(t, err)
			defer buf.Close()

			trace := &common.Trace{Writer: buf}
			e.BuildLogger = buildlogger.New(trace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})

			isw := service_helpers.NewInlineServiceLogWriter(cName, trace)

			// we'll write into pw, which will be copied to pr and simulate a process in
			// a container writing to stdout.
			pr, pw := io.Pipe()
			defer pw.Close() // ... for the failure case

			ctx := t.Context()
			c.On("ContainerLogs", ctx, cID, mock.Anything).Return(pr, tt.wantErr).Once()
			err = e.captureContainerLogs(ctx, cID, cName, isw)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantLog)
				return
			}

			require.NoError(t, err)

			// this will be copied to pr...
			_, err = pw.Write(append(tt.header, msg...))
			require.NoError(t, err)
			pw.Close() // this will also close pr

			assert.Eventually(t, func() bool {
				contents, err := buf.Bytes(0, math.MaxInt64)
				require.NoError(t, err)

				return assert.Contains(t, string(contents), tt.wantLog)
			}, time.Millisecond*500, time.Millisecond+10)
		})
	}
}

func Test_Executor_captureContainersLogs(t *testing.T) {
	containers := []*types.Container{
		{
			ID:    "000000000000000000000000000000000",
			Names: []string{"some container"},
			Image: "some container",
		},
		{
			ID:    "111111111111111111111111111111111",
			Names: []string{"some other container"},
			Image: "some other container",
		},
	}

	linksMap := map[string]*types.Container{
		"one":       containers[0],
		"two":       containers[1],
		"two-alias": containers[1],
	}

	logs := bytes.Buffer{}
	lentry := logrus.New()
	lentry.Out = &logs

	stop := errors.New("don't actually try to stream the container's logs")
	c := docker.NewMockClient(t)

	e := &executor{services: containers}
	e.dockerConn = &dockerConnection{Client: c}
	e.BuildLogger = buildlogger.New(&common.Trace{Writer: &logs}, logrus.NewEntry(lentry), buildlogger.Options{})
	e.Build = &common.Build{}

	ctx := t.Context()

	tests := map[string]struct {
		debugServicePolicy string
		expect             func()
		assert             func(t *testing.T)
	}{
		"enabled": {
			debugServicePolicy: "true",
			expect: func() {
				for _, cont := range containers {
					// have the call to ContainerLogs return an error so we
					// don't have to mock more behaviour. that functionality is
					// tested elsewhere.
					c.On("ContainerLogs", ctx, cont.ID, mock.Anything).Return(nil, stop).Once()
				}
			},
			assert: func(t *testing.T) {
				for _, c := range containers {
					assert.Contains(t, logs.String(), "WARNING: failed to open log stream for container "+
						c.Names[0]+": "+stop.Error())
				}
			},
		},
		"disabled": {
			debugServicePolicy: "false",
			expect:             func() {},
			assert:             func(t *testing.T) { assert.Empty(t, logs.String()) },
		},
		"bogus": {
			debugServicePolicy: "blammo",
			expect:             func() {},
			assert:             func(t *testing.T) { assert.Empty(t, logs.String()) },
		},
	}

	for name, tt := range tests {
		logs.Reset()
		t.Run(name, func(t *testing.T) {
			e.Build = &common.Build{}
			e.Build.Variables = common.JobVariables{
				{Key: "CI_DEBUG_SERVICES", Value: tt.debugServicePolicy, Public: true},
			}

			tt.expect()
			e.captureContainersLogs(ctx, linksMap)
			tt.assert(t)
		})
	}
}
