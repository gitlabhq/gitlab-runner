//go:build !integration

package docker

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/pull"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/helperimage"
	service_test "gitlab.com/gitlab-org/gitlab-runner/helpers/container/services/test"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	service_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/service"
)

func testServiceFromNamedImage(t *testing.T, description, imageName, serviceName string) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	p := new(pull.MockManager)
	defer p.AssertExpectations(t)

	servicePart := fmt.Sprintf("-%s-0", strings.ReplaceAll(serviceName, "/", "__"))
	containerNameRegex, err := regexp.Compile("runner-abcdef12-project-0-concurrent-0-[^-]+" + servicePart)
	require.NoError(t, err)

	containerNameMatcher := mock.MatchedBy(containerNameRegex.MatchString)
	networkID := "network-id"

	e := &executor{
		client: c,
		info: types.Info{
			OSType:       helperimage.OSTypeLinux,
			Architecture: "amd64",
		},
		volumeParser: parser.NewLinuxParser(),
		pullManager:  p,
	}

	e.Config = common.RunnerConfig{}
	e.Config.Docker = &common.DockerConfig{}
	e.Build = &common.Build{
		ProjectRunnerID: 0,
		Runner:          &common.RunnerConfig{},
	}
	e.Build.JobInfo.ProjectID = 0
	e.Build.Runner.Token = "abcdef1234567890"
	e.Context = context.Background()

	e.helperImageInfo, err = helperimage.Get(common.REVISION, helperimage.Config{
		OSType:          e.info.OSType,
		Architecture:    e.info.Architecture,
		OperatingSystem: e.info.OperatingSystem,
	})
	require.NoError(t, err)

	err = e.createLabeler()
	require.NoError(t, err)

	e.BuildShell = &common.ShellConfiguration{}

	realServiceContainerName := e.getProjectUniqRandomizedName() + servicePart

	p.On("GetDockerImage", imageName, []common.DockerPullPolicy(nil)).
		Return(&types.ImageInspect{ID: "helper-image"}, nil).
		Once()

	c.On(
		"ContainerRemove",
		e.Context,
		containerNameMatcher,
		types.ContainerRemoveOptions{RemoveVolumes: true, Force: true},
	).
		Return(nil).
		Once()

	networkContainersMap := map[string]types.EndpointResource{
		"1": {Name: realServiceContainerName},
	}

	c.On("NetworkList", e.Context, types.NetworkListOptions{}).
		Return([]types.NetworkResource{{ID: networkID, Name: "network-name", Containers: networkContainersMap}}, nil).
		Once()

	c.On("NetworkDisconnect", e.Context, networkID, containerNameMatcher, true).
		Return(nil).
		Once()

	c.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
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
	c, e := prepareTestDockerConfiguration(t, dockerConfig, cce)
	defer c.AssertExpectations(t)

	c.On("ContainerStart", mock.Anything, "abc", mock.Anything).
		Return(nil).Once()

	err := e.createVolumesManager()
	require.NoError(t, err)

	err = e.createPullManager()
	require.NoError(t, err)

	_, err = e.createService(
		0,
		"build",
		"latest",
		"alpine",
		common.Image{Command: []string{"/bin/sh"}},
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

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {
		require.NotEmpty(t, hostConfig.Tmpfs)
	}

	testDockerConfigurationWithServiceContainer(t, dockerConfig, cce)
}

func TestDockerServicesDNSSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		DNS: []string{"2001:db8::1", "192.0.2.1"},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {
		require.Equal(t, dockerConfig.DNS, hostConfig.DNS)
	}

	testDockerConfigurationWithServiceContainer(t, dockerConfig, cce)
}

func TestDockerServicesDNSSearchSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		DNSSearch: []string{"mydomain.example"},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {
		require.Equal(t, dockerConfig.DNSSearch, hostConfig.DNSSearch)
	}

	testDockerConfigurationWithServiceContainer(t, dockerConfig, cce)
}

func TestDockerServicesExtraHostsSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		ExtraHosts: []string{"foo.example:2001:db8::1", "bar.example:192.0.2.1"},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {
		require.Equal(t, dockerConfig.ExtraHosts, hostConfig.ExtraHosts)
	}

	testDockerConfigurationWithServiceContainer(t, dockerConfig, cce)
}

func TestDockerServiceUserNSSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{}
	dockerConfigWithHostUsernsMode := &common.DockerConfig{
		UsernsMode: "host",
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {
		assert.Equal(t, container.UsernsMode(""), hostConfig.UsernsMode)
	}
	cceWithHostUsernsMode := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {
		assert.Equal(t, container.UsernsMode("host"), hostConfig.UsernsMode)
	}

	testDockerConfigurationWithServiceContainer(t, dockerConfig, cce)
	testDockerConfigurationWithServiceContainer(t, dockerConfigWithHostUsernsMode, cceWithHostUsernsMode)
}

func TestDockerWithNoDockerConfigAndWithServiceImagePullPolicyAlways(t *testing.T) {
	dockerConfig := &common.DockerConfig{}
	serviceConfig := common.Image{
		PullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {}

	c, e := prepareTestDockerConfiguration(t, dockerConfig, cce)
	defer c.AssertExpectations(t)

	c.On("ContainerStart", mock.Anything, "abc", mock.Anything).
		Return(nil).Once()

	err := e.createVolumesManager()
	require.NoError(t, err)

	err = e.createPullManager()
	require.NoError(t, err)

	_, err = e.createService(
		0,
		"build",
		"latest",
		"alpine",
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
		PullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {}

	c, e := createExecutorForTestDockerConfiguration(t, dockerConfig, cce)

	c.On("ImageInspectWithRaw", mock.Anything, "alpine").
		Return(types.ImageInspect{ID: "123"}, []byte{}, nil).Once()
	c.On("NetworkList", mock.Anything, mock.Anything).
		Return([]types.NetworkResource{}, nil).Once()
	c.On("ContainerRemove", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()
	c.On("ContainerStart", mock.Anything, "abc", mock.Anything).
		Return(nil).Once()

	defer c.AssertExpectations(t)

	err := e.createVolumesManager()
	require.NoError(t, err)

	err = e.createPullManager()
	require.NoError(t, err)

	_, err = e.createService(
		0,
		"build",
		"latest",
		"alpine",
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
	serviceConfig := common.Image{}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {}
	_, e := createExecutorForTestDockerConfiguration(t, dockerConfig, cce)

	err := e.createVolumesManager()
	require.NoError(t, err)

	err = e.createPullManager()
	require.NoError(t, err)

	_, err = e.createService(
		0,
		"build",
		"latest",
		"alpine",
		serviceConfig,
		nil,
	)
	assert.Contains(
		t,
		err.Error(),
		"failed to pull image 'alpine'",
	)
	assert.Contains(
		t,
		err.Error(),
		fmt.Sprintf(common.IncompatiblePullPolicy, "[always]", "Runner config", "[if-not-present]"),
	)
}

func TestDockerWithDockerConfigAlwaysAndWithServiceImagePullPolicyIfNotPresent(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		PullPolicy:          common.StringOrArray{common.PullPolicyAlways},
		AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways},
	}
	serviceConfig := common.Image{
		PullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {}
	_, e := createExecutorForTestDockerConfiguration(t, dockerConfig, cce)

	err := e.createVolumesManager()
	require.NoError(t, err)

	err = e.createPullManager()
	require.NoError(t, err)

	_, err = e.createService(
		0,
		"build",
		"latest",
		"alpine",
		serviceConfig,
		nil,
	)
	assert.Contains(
		t,
		err.Error(),
		"failed to pull image 'alpine'",
	)
	assert.Contains(
		t,
		err.Error(),
		fmt.Sprintf(common.IncompatiblePullPolicy, "[if-not-present]", "GitLab pipeline config", "[always]"),
	)
}

func TestGetServiceDefinitions(t *testing.T) {
	e := new(executor)
	e.Build = &common.Build{
		Runner: &common.RunnerConfig{},
	}
	e.Config = common.RunnerConfig{}
	e.Config.Docker = &common.DockerConfig{}

	tests := map[string]struct {
		services         []common.Service
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
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			e.Config.Docker.Services = tt.services
			e.Config.Docker.AllowedServices = tt.allowedServices
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
		expectedErr            error
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
				"WAIT_FOR_SERVICE_TCP_PORT=1000",
			},
		},
		"get port from many": {
			networkMode: "test",
			dockerClientAssertions: func(c *docker.MockClient) {
				c.On("ContainerInspect", mock.Anything, mock.Anything).
					Return(types.ContainerJSON{
						Config: &container.Config{
							ExposedPorts: nat.PortSet{
								"1000/tcp": {},
								"500/udp":  {},
								"600/tcp":  {},
								"1500/tcp": {},
							},
						},
					}, nil).
					Once()
			},
			expectedEnvironment: []string{
				"WAIT_FOR_SERVICE_TCP_ADDR=000000000000",
				"WAIT_FOR_SERVICE_TCP_PORT=600",
			},
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
			expectedErr: fmt.Errorf("service %q has no exposed ports", "default"),
		},
		"container inspect error": {
			networkMode: "test",
			dockerClientAssertions: func(c *docker.MockClient) {
				c.On("ContainerInspect", mock.Anything, mock.Anything).
					Return(types.ContainerJSON{}, fmt.Errorf("%v", "test error")).
					Once()
			},
			expectedErr: fmt.Errorf("get container exposed ports: %v", "test error"),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			client := new(docker.MockClient)

			if test.dockerClientAssertions != nil {
				test.dockerClientAssertions(client)
			}
			defer client.AssertExpectations(t)

			executor := &executor{
				networkMode: container.NetworkMode(test.networkMode),
				client:      client,
			}

			service := &types.Container{
				ID:    "0000000000000000000000000000000000000000000000000000000000000000",
				Names: []string{"default"},
			}

			environment, err := executor.addServiceHealthCheckEnvironment(service)

			assert.Equal(t, test.expectedEnvironment, environment)

			assert.Equal(t, test.expectedErr, err)
		})
	}
}

func Test_Executor_captureContainerLogs(t *testing.T) {
	cID := "some container"
	cName := cID
	msg := []byte("pretend this is a log generated by a process in a container")
	ctx := context.Background()
	logs := bytes.Buffer{}

	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	e := &executor{}
	e.client = c
	lentry := logrus.New()
	lentry.Out = &logs
	e.BuildLogger = common.NewBuildLogger(&common.Trace{Writer: &logs}, logrus.NewEntry(lentry))

	isw := service_helpers.NewInlineServiceLogWriter(cName, &logs)

	tests := map[string]struct {
		header  []byte
		wantLog string
		wantErr error
	}{
		"success": {
			// for header spec see https://pkg.go.dev/github.com/moby/moby/client#Client.ContainerLogs
			header:  []byte{1, 0, 0, 0, 0, 0, 0, byte(len(msg))},
			wantLog: string(msg),
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
			logs.Reset()
			// we'll write into pw, which will be copied to pr and simulate a process in
			// a container writing to stdout.
			pr, pw := io.Pipe()
			defer pw.Close() // ... for the failure case

			c.On("ContainerLogs", ctx, cID, mock.Anything).Return(pr, tt.wantErr).Once()
			err := e.captureContainerLogs(ctx, cID, cName, isw)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantLog)
			} else {
				require.NoError(t, err)

				// this will be copied to pr...
				_, err := pw.Write(append(tt.header, msg...))
				require.NoError(t, err)
				pw.Close() // this will also close pr

				assert.Eventually(t, func() bool {
					return assert.Contains(t, logs.String(), tt.wantLog)
				}, time.Millisecond*500, time.Millisecond+10)
			}
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
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	e := &executor{services: containers}
	e.client = c
	e.BuildLogger = common.NewBuildLogger(&common.Trace{Writer: &logs}, logrus.NewEntry(lentry))
	e.Build = &common.Build{}

	ctx := context.Background()

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
