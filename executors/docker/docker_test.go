//go:build !integration

package docker

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/containerd/errdefs"
	"github.com/docker/go-units"
	"github.com/hashicorp/go-version"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/image"
	"github.com/moby/moby/api/types/mount"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/system"
	"github.com/moby/moby/api/types/volume"
	"github.com/moby/moby/client"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/step-runner/schema/v1"

	"gitlab.com/gitlab-org/gitlab-runner/commands/steps"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/networks"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/prebuilt"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/pull"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/user"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/permission"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/helperimage"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/test"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

func TestParseDeviceStringOne(t *testing.T) {
	e := new(executor)

	device, err := e.parseDeviceString("/dev/kvm")

	assert.NoError(t, err)
	assert.Equal(t, "/dev/kvm", device.PathOnHost)
	assert.Equal(t, "/dev/kvm", device.PathInContainer)
	assert.Equal(t, "rwm", device.CgroupPermissions)
}

func TestParseDeviceStringTwo(t *testing.T) {
	e := new(executor)

	device, err := e.parseDeviceString("/dev/kvm:/devices/kvm")

	assert.NoError(t, err)
	assert.Equal(t, "/dev/kvm", device.PathOnHost)
	assert.Equal(t, "/devices/kvm", device.PathInContainer)
	assert.Equal(t, "rwm", device.CgroupPermissions)
}

func TestParseDeviceStringThree(t *testing.T) {
	e := new(executor)

	device, err := e.parseDeviceString("/dev/kvm:/devices/kvm:r")

	assert.NoError(t, err)
	assert.Equal(t, "/dev/kvm", device.PathOnHost)
	assert.Equal(t, "/devices/kvm", device.PathInContainer)
	assert.Equal(t, "r", device.CgroupPermissions)
}

func TestParseDeviceStringFour(t *testing.T) {
	e := new(executor)

	_, err := e.parseDeviceString("/dev/kvm:/devices/kvm:r:oops")

	assert.Error(t, err)
}

func TestBindDeviceRequests(t *testing.T) {
	tests := []struct {
		gpus                  string
		expectedDeviceRequest []container.DeviceRequest
		expectedErr           bool
	}{
		{
			gpus: "all",
			expectedDeviceRequest: []container.DeviceRequest{
				{
					Driver:       "",
					Count:        -1,
					DeviceIDs:    nil,
					Capabilities: [][]string{{"gpu"}},
					Options:      map[string]string{},
				},
			},
		},
		{
			gpus:                  "",
			expectedDeviceRequest: nil,
		},
		{
			gpus:                  "somestring=thatshouldtriggeranerror",
			expectedDeviceRequest: nil,
			expectedErr:           true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.gpus, func(t *testing.T) {
			e := executor{
				AbstractExecutor: executors.AbstractExecutor{
					Config: common.RunnerConfig{
						RunnerSettings: common.RunnerSettings{
							Docker: &common.DockerConfig{
								Gpus: tt.gpus,
							},
						},
					},
				},
			}

			err := e.bindDeviceRequests()
			if tt.expectedErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectedDeviceRequest, e.deviceRequests)
		})
	}
}

type testAllowedImageDescription struct {
	allowed       bool
	image         string
	allowedImages []string
}

var testAllowedImages = []testAllowedImageDescription{
	{true, "ruby", []string{"*"}},
	{true, "ruby:3.3", []string{"*"}},
	{true, "ruby:latest", []string{"*"}},
	{true, "library/ruby", []string{"*/*"}},
	{true, "library/ruby:3.3", []string{"*/*"}},
	{true, "library/ruby:3.3", []string{"*/*:*"}},
	{true, "my.registry.tld/library/ruby", []string{"my.registry.tld/*/*"}},
	{true, "my.registry.tld/library/ruby:3.3", []string{"my.registry.tld/*/*:*"}},
	{true, "my.registry.tld/group/subgroup/ruby", []string{"my.registry.tld/*/*/*"}},
	{true, "my.registry.tld/group/subgroup/ruby:3.3", []string{"my.registry.tld/*/*/*:*"}},
	{true, "ruby", []string{"**/*"}},
	{true, "ruby:3.3", []string{"**/*"}},
	{true, "ruby:latest", []string{"**/*"}},
	{true, "library/ruby", []string{"**/*"}},
	{true, "library/ruby:3.3", []string{"**/*"}},
	{true, "library/ruby:3.3", []string{"**/*:*"}},
	{true, "my.registry.tld/library/ruby", []string{"my.registry.tld/**/*"}},
	{true, "my.registry.tld/library/ruby:3.3", []string{"my.registry.tld/**/*:*"}},
	{true, "my.registry.tld/group/subgroup/ruby", []string{"my.registry.tld/**/*"}},
	{true, "my.registry.tld/group/subgroup/ruby:3.3", []string{"my.registry.tld/**/*:*"}},
	{false, "library/ruby", []string{"*"}},
	{false, "library/ruby:3.3", []string{"*"}},
	{false, "my.registry.tld/ruby", []string{"*"}},
	{false, "my.registry.tld/ruby:3.3", []string{"*"}},
	{false, "my.registry.tld/library/ruby", []string{"*"}},
	{false, "my.registry.tld/library/ruby:3.3", []string{"*"}},
	{false, "my.registry.tld/group/subgroup/ruby", []string{"*"}},
	{false, "my.registry.tld/group/subgroup/ruby:3.3", []string{"*"}},
	{false, "library/ruby", []string{"*/*:*"}},
	{false, "my.registry.tld/group/subgroup/ruby", []string{"my.registry.tld/*/*"}},
	{false, "my.registry.tld/group/subgroup/ruby:3.3", []string{"my.registry.tld/*/*:*"}},
	{false, "library/ruby", []string{"**/*:*"}},
}

func TestVerifyAllowedImage(t *testing.T) {
	e := new(executor)
	e.BuildLogger = buildlogger.New(nil, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})

	for _, test := range testAllowedImages {
		err := e.verifyAllowedImage(test.image, "", test.allowedImages, []string{})

		if err != nil && test.allowed {
			t.Errorf("%q must be allowed by %q", test.image, test.allowedImages)
		} else if err == nil && !test.allowed {
			t.Errorf("%q must not be allowed by %q", test.image, test.allowedImages)
		}
	}
}

func TestIsInAllowedPrivilegedImages(t *testing.T) {
	for _, test := range testAllowedImages {
		res := isInAllowedPrivilegedImages(test.image, test.allowedImages)

		if !res && test.allowed {
			t.Errorf("%q must be allowed by %q", test.image, test.allowedImages)
		} else if res && !test.allowed {
			t.Errorf("%q must not be allowed by %q", test.image, test.allowedImages)
		}
	}
}

func executorWithMockClient(c *docker.MockClient) *executor {
	mockConnector := func(ctx context.Context, options common.ExecutorPrepareOptions, e *executor) error {
		e.dockerConn = &dockerConnection{Client: c}
		e.info = system.Info{OSType: helperimage.OSTypeLinux}
		return nil
	}
	e := &executor{
		dockerConnector: mockConnector,
	}

	e.Context = context.Background()
	e.Build = new(common.Build)
	return e
}

func TestHelperImageWithVariable(t *testing.T) {
	c := docker.NewMockClient(t)
	p := pull.NewMockManager(t)

	runnerImageTag := "gitlab/gitlab-runner:" + common.AppVersion.Revision

	p.On("GetDockerImage", runnerImageTag, spec.ImageDockerOptions{}, []common.DockerPullPolicy(nil)).
		Return(&image.InspectResponse{ID: "helper-image"}, nil).
		Once()

	e := executorWithMockClient(c)
	e.pullManager = p

	e.Config = common.RunnerConfig{}
	e.Config.Docker = &common.DockerConfig{
		HelperImage: "gitlab/gitlab-runner:${CI_RUNNER_REVISION}",
	}

	img, err := e.getHelperImage()
	assert.NoError(t, err)
	require.NotNil(t, img)
	assert.Equal(t, "helper-image", img.ID)
}

func TestPrepareBuildsDir(t *testing.T) {
	tests := map[string]struct {
		dontSetupVolumeParser   bool
		rootDir                 string
		volumes                 []string
		expectedSharedBuildsDir bool
		expectedError           string
	}{
		"rootDir mounted as host based volume": {
			rootDir:                 "/build",
			volumes:                 []string{"/build:/build"},
			expectedSharedBuildsDir: true,
		},
		"rootDir mounted as container based volume": {
			rootDir:                 "/build",
			volumes:                 []string{"/build"},
			expectedSharedBuildsDir: false,
		},
		"rootDir not mounted as volume": {
			rootDir:                 "/build",
			volumes:                 []string{"/folder:/folder"},
			expectedSharedBuildsDir: false,
		},
		"rootDir's parent mounted as volume": {
			rootDir:                 "/build/other/directory",
			volumes:                 []string{"/build/:/build"},
			expectedSharedBuildsDir: true,
		},
		"rootDir is not an absolute path": {
			rootDir:       "builds",
			expectedError: "build directory needs to be an absolute path",
		},
		"rootDir is /": {
			rootDir:       "/",
			expectedError: "build directory needs to be a non-root path",
		},
		"error on volume parsing": {
			rootDir:       "/build",
			volumes:       []string{""},
			expectedError: "invalid volume specification",
		},
		"error on volume parser creation": {
			dontSetupVolumeParser: true,
			expectedError:         `missing volume parser`,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			c := common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					BuildsDir: test.rootDir,
					Docker: &common.DockerConfig{
						Volumes: test.volumes,
					},
				},
			}

			build := &common.Build{}
			build.Variables = spec.Variables{}

			options := common.ExecutorPrepareOptions{
				Config: &c,
			}

			e := &executor{
				AbstractExecutor: executors.AbstractExecutor{
					Build:  build,
					Config: c,
				},
			}
			if !test.dontSetupVolumeParser {
				e.volumeParser = parser.NewLinuxParser(e.ExpandValue)
			}

			err := e.prepareBuildsDir(options)
			if test.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.expectedSharedBuildsDir, e.SharedBuildsDir)
		})
	}
}

type volumesTestCase struct {
	volumes                  []string
	buildsDir                string
	gitStrategy              string
	adjustConfiguration      func(e *executor)
	volumesManagerAssertions func(*volumes.MockManager)
	clientAssertions         func(*docker.MockClient)
	createVolumeManager      bool
	expectedError            error
}

var (
	volumesTestsDefaultBuildsDir = "/default-builds-dir"
	volumesTestsDefaultCacheDir  = "/default-cache-dir"
)

func getExecutorForVolumesTests(t *testing.T, test volumesTestCase) *executor {
	e := &executor{}
	e.serverAPIVersion = version.Must(version.NewVersion("1.43"))

	clientMock := docker.NewMockClient(t)
	clientMock.On("Close").Return(nil).Once()
	dockerConn := &dockerConnection{Client: clientMock}
	e.dockerConn = dockerConn

	volumesManagerMock := volumes.NewMockManager(t)
	if !errors.Is(test.expectedError, errVolumesManagerUndefined) {
		volumesManagerMock.On("RemoveTemporary", mock.Anything).Return(nil).Once()
	}

	oldCreateVolumesManager := createVolumesManager

	t.Cleanup(func() {
		e.Cleanup()

		createVolumesManager = oldCreateVolumesManager
	})

	createVolumesManager = func(_ *executor) (volumes.Manager, error) {
		return volumesManagerMock, nil
	}

	if test.volumesManagerAssertions != nil {
		test.volumesManagerAssertions(volumesManagerMock)
	}

	if test.clientAssertions != nil {
		test.clientAssertions(clientMock)
	}

	c := common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "abcdef1234567890",
		},
		RunnerSettings: common.RunnerSettings{
			BuildsDir: test.buildsDir,
			Docker: &common.DockerConfig{
				Volumes: test.volumes,
			},
		},
	}

	logger, _ := logrustest.NewNullLogger()
	e.AbstractExecutor = executors.AbstractExecutor{
		BuildLogger: buildlogger.New(&common.Trace{Writer: io.Discard}, logger.WithField("test", t.Name()), buildlogger.Options{}),
		Build: &common.Build{
			ProjectRunnerID: 0,
			Runner:          &c,
			Job: spec.Job{
				JobInfo: spec.JobInfo{
					ProjectID: 0,
				},
				GitInfo: spec.GitInfo{
					RepoURL: "https://gitlab.example.com/group/project.git",
				},
			},
		},
		Config: c,
		ExecutorOptions: executors.ExecutorOptions{
			DefaultBuildsDir: volumesTestsDefaultBuildsDir,
			DefaultCacheDir:  volumesTestsDefaultCacheDir,
		},
	}
	e.dockerConn = &dockerConnection{Client: clientMock}
	e.info = system.Info{
		OSType: helperimage.OSTypeLinux,
	}

	e.Build.Variables = append(e.Build.Variables, spec.Variable{
		Key:   "GIT_STRATEGY",
		Value: test.gitStrategy,
	})

	if test.adjustConfiguration != nil {
		test.adjustConfiguration(e)
	}

	err := e.Build.StartBuild(
		e.RootDir(),
		e.CacheDir(),
		e.CustomBuildEnabled(),
		e.SharedBuildsDir,
		false,
	)
	require.NoError(t, err)

	if test.createVolumeManager {
		err = e.createVolumesManager()
		require.NoError(t, err)
	}

	return e
}

func TestCreateVolumes(t *testing.T) {
	tests := map[string]volumesTestCase{
		"volumes manager not created": {
			expectedError: errVolumesManagerUndefined,
		},
		"no volumes defined, empty buildsDir, clone strategy, no errors": {
			gitStrategy:         "clone",
			createVolumeManager: true,
		},
		"no volumes defined, defined buildsDir, clone strategy, no errors": {
			buildsDir:           "/builds",
			gitStrategy:         "clone",
			createVolumeManager: true,
		},
		"no volumes defined, defined buildsDir, fetch strategy, no errors": {
			buildsDir:           "/builds",
			gitStrategy:         "fetch",
			createVolumeManager: true,
		},
		"volumes defined, empty buildsDir, clone strategy, no errors on user volume": {
			volumes:     []string{"/volume"},
			gitStrategy: "clone",
			volumesManagerAssertions: func(vm *volumes.MockManager) {
				vm.On("Create", mock.Anything, "/volume").
					Return(nil).
					Once()
			},
			createVolumeManager: true,
		},
		"volumes defined, empty buildsDir, clone strategy, duplicated error on user volume": {
			volumes:     []string{"/volume"},
			gitStrategy: "clone",
			volumesManagerAssertions: func(vm *volumes.MockManager) {
				vm.On("Create", mock.Anything, "/volume").
					Return(volumes.NewErrVolumeAlreadyDefined("/volume")).
					Once()
			},
			createVolumeManager: true,
			expectedError:       volumes.NewErrVolumeAlreadyDefined("/volume"),
		},
		"volumes defined, empty buildsDir, clone strategy, other error on user volume": {
			volumes:     []string{"/volume"},
			gitStrategy: "clone",
			volumesManagerAssertions: func(vm *volumes.MockManager) {
				vm.On("Create", mock.Anything, "/volume").
					Return(errors.New("test-error")).
					Once()
			},
			createVolumeManager: true,
			expectedError:       errors.New("test-error"),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			e := getExecutorForVolumesTests(t, test)
			err := e.createVolumes()
			assert.Equal(t, test.expectedError, err)
		})
	}
}

func TestCreateBuildVolume(t *testing.T) {
	tests := map[string]volumesTestCase{
		"volumes manager not created": {
			expectedError: errVolumesManagerUndefined,
		},
		"git strategy clone, empty buildsDir, no error": {
			gitStrategy: "clone",
			volumesManagerAssertions: func(vm *volumes.MockManager) {
				vm.On("CreateTemporary", mock.Anything, volumesTestsDefaultBuildsDir).
					Return(nil).
					Once()
			},
			createVolumeManager: true,
		},
		"git strategy clone, empty buildsDir, duplicated error": {
			gitStrategy: "clone",
			volumesManagerAssertions: func(vm *volumes.MockManager) {
				vm.On("CreateTemporary", mock.Anything, volumesTestsDefaultBuildsDir).
					Return(volumes.NewErrVolumeAlreadyDefined(volumesTestsDefaultBuildsDir)).
					Once()
			},
			createVolumeManager: true,
		},
		"git strategy clone, empty buildsDir, other error": {
			gitStrategy: "clone",
			volumesManagerAssertions: func(vm *volumes.MockManager) {
				vm.On("CreateTemporary", mock.Anything, volumesTestsDefaultBuildsDir).
					Return(errors.New("test-error")).
					Once()
			},
			createVolumeManager: true,
			expectedError:       errors.New("test-error"),
		},
		"git strategy clone, non-empty buildsDir, no error": {
			gitStrategy: "clone",
			buildsDir:   "/builds",
			volumesManagerAssertions: func(vm *volumes.MockManager) {
				vm.On("CreateTemporary", mock.Anything, "/builds").
					Return(nil).
					Once()
			},
			createVolumeManager: true,
		},
		"git strategy clone, non-empty buildsDir, duplicated error": {
			gitStrategy: "clone",
			buildsDir:   "/builds",
			volumesManagerAssertions: func(vm *volumes.MockManager) {
				vm.On("CreateTemporary", mock.Anything, "/builds").
					Return(volumes.NewErrVolumeAlreadyDefined("/builds")).
					Once()
			},
			createVolumeManager: true,
		},
		"git strategy clone, non-empty buildsDir, other error": {
			gitStrategy: "clone",
			buildsDir:   "/builds",
			volumesManagerAssertions: func(vm *volumes.MockManager) {
				vm.On("CreateTemporary", mock.Anything, "/builds").
					Return(errors.New("test-error")).
					Once()
			},
			createVolumeManager: true,
			expectedError:       errors.New("test-error"),
		},
		"git strategy fetch, empty buildsDir, no error": {
			gitStrategy: "fetch",
			volumesManagerAssertions: func(vm *volumes.MockManager) {
				vm.On("Create", mock.Anything, volumesTestsDefaultBuildsDir).
					Return(nil).
					Once()
			},
			createVolumeManager: true,
		},
		"git strategy fetch, empty buildsDir, duplicated error": {
			gitStrategy: "fetch",
			volumesManagerAssertions: func(vm *volumes.MockManager) {
				vm.On("Create", mock.Anything, volumesTestsDefaultBuildsDir).
					Return(volumes.NewErrVolumeAlreadyDefined(volumesTestsDefaultBuildsDir)).
					Once()
			},
			createVolumeManager: true,
		},
		"git strategy fetch, empty buildsDir, other error": {
			gitStrategy: "fetch",
			volumesManagerAssertions: func(vm *volumes.MockManager) {
				vm.On("Create", mock.Anything, volumesTestsDefaultBuildsDir).
					Return(errors.New("test-error")).
					Once()
			},
			createVolumeManager: true,
			expectedError:       errors.New("test-error"),
		},
		"git strategy fetch, non-empty buildsDir, no error": {
			gitStrategy: "fetch",
			buildsDir:   "/builds",
			volumesManagerAssertions: func(vm *volumes.MockManager) {
				vm.On("Create", mock.Anything, "/builds").
					Return(nil).
					Once()
			},
			createVolumeManager: true,
		},
		"git strategy fetch, non-empty buildsDir, duplicated error": {
			gitStrategy: "fetch",
			buildsDir:   "/builds",
			volumesManagerAssertions: func(vm *volumes.MockManager) {
				vm.On("Create", mock.Anything, "/builds").
					Return(volumes.NewErrVolumeAlreadyDefined("/builds")).
					Once()
			},
			createVolumeManager: true,
		},
		"git strategy fetch, non-empty buildsDir, wrapped duplicated error": {
			gitStrategy: "fetch",
			buildsDir:   "/builds",
			volumesManagerAssertions: func(vm *volumes.MockManager) {
				vm.On("Create", mock.Anything, "/builds").
					Return(fmt.Errorf("wrap: %w", volumes.NewErrVolumeAlreadyDefined("/builds"))).
					Once()
			},
			createVolumeManager: true,
		},
		"git strategy fetch, non-empty buildsDir, other error": {
			gitStrategy: "fetch",
			buildsDir:   "/builds",
			volumesManagerAssertions: func(vm *volumes.MockManager) {
				vm.On("Create", mock.Anything, "/builds").
					Return(errors.New("test-error")).
					Once()
			},
			createVolumeManager: true,
			expectedError:       errors.New("test-error"),
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			e := getExecutorForVolumesTests(t, test)
			err := e.createBuildVolume()
			assert.Equal(t, test.expectedError, err)
		})
	}
}

func TestCreateDependencies(t *testing.T) {
	const containerID = "container-ID"
	containerNameRegex, err := regexp.Compile("runner-abcdef123-project-0-concurrent-0-[^-]+-alpine-0")
	require.NoError(t, err)

	containerNameMatcher := mock.MatchedBy(containerNameRegex.MatchString)
	testError := errors.New("test-error")

	testCase := volumesTestCase{
		buildsDir: "/builds",
		volumes:   []string{"/volume"},
		adjustConfiguration: func(e *executor) {
			e.Build.Services = append(e.Build.Services, spec.Image{
				Name: "alpine:latest",
			})

			e.BuildShell = &common.ShellConfiguration{}
		},
		volumesManagerAssertions: func(vm *volumes.MockManager) {
			binds := make([]string, 0)

			vm.On("CreateTemporary", mock.Anything, "/builds").
				Return(nil).
				Run(func(args mock.Arguments) {
					binds = append(binds, args.Get(1).(string))
				}).
				Once()
			vm.On("Create", mock.Anything, "/volume").
				Return(nil).
				Run(func(args mock.Arguments) {
					binds = append(binds, args.Get(1).(string))
				}).
				Once()
			vm.On("Binds").
				Return(func() []string {
					return binds
				}).
				Once()
		},
		clientAssertions: func(c *docker.MockClient) {
			hostConfigMatcher := mock.MatchedBy(func(conf *container.HostConfig) bool {
				return assert.Equal(t, []string{"/volume", "/builds"}, conf.Binds)
			})

			c.On("ImageInspectWithRaw", mock.Anything, "alpine:latest", mock.Anything).
				Return(image.InspectResponse{}, nil, nil).
				Once()
			c.On("NetworkList", mock.Anything, mock.Anything).
				Return(nil, nil).
				Times(2)
			c.On("ContainerRemove", mock.Anything, containerNameMatcher, mock.Anything).
				Return(nil).
				Once()
			c.On("ContainerRemove", mock.Anything, containerID, mock.Anything).
				Return(nil).
				Once()
			c.On(
				"ContainerCreate",
				mock.Anything,
				mock.Anything,
				hostConfigMatcher,
				mock.Anything,
				mock.AnythingOfType("*v1.Platform"),
				containerNameMatcher,
			).
				Return(container.CreateResponse{ID: containerID}, nil).
				Once()
			c.On("ContainerStart", mock.Anything, containerID, mock.Anything).
				Return(testError).
				Once()
		},
	}

	e := getExecutorForVolumesTests(t, testCase)
	err = e.createDependencies()
	assert.Equal(t, testError, err)
}

type containerConfigExpectations func(*testing.T, *container.Config, *container.HostConfig, *network.NetworkingConfig)

type dockerConfigurationTestFakeDockerClient struct {
	*docker.MockClient

	cce containerConfigExpectations
	t   *testing.T
}

func (c *dockerConfigurationTestFakeDockerClient) ContainerCreate(
	ctx context.Context,
	config *container.Config,
	hostConfig *container.HostConfig,
	networkingConfig *network.NetworkingConfig,
	platform *v1.Platform,
	containerName string,
) (container.CreateResponse, error) {
	c.cce(c.t, config, hostConfig, networkingConfig)
	return container.CreateResponse{ID: "abc"}, nil
}

func createExecutorForTestDockerConfiguration(
	t *testing.T,
	dockerConfig *common.DockerConfig,
	cce containerConfigExpectations,
) (*dockerConfigurationTestFakeDockerClient, *executor) {
	c := &dockerConfigurationTestFakeDockerClient{
		cce: cce,
		t:   t,
	}
	c.MockClient = docker.NewMockClient(t)

	e := new(executor)
	e.dockerConn = &dockerConnection{Client: c}
	e.info = system.Info{
		OSType:       helperimage.OSTypeLinux,
		Architecture: "amd64",
	}
	e.BuildLogger = buildlogger.New(nil, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})
	e.Config.Docker = dockerConfig
	e.Build = &common.Build{
		Runner: &common.RunnerConfig{},
	}
	e.Build.Token = "abcd123456"
	e.BuildShell = &common.ShellConfiguration{}
	var err error
	e.helperImageInfo, err = helperimage.Get(common.AppVersion.Version, helperimage.Config{
		OSType:        e.info.OSType,
		Architecture:  e.info.Architecture,
		KernelVersion: e.info.KernelVersion,
	})
	require.NoError(t, err)

	err = e.createLabeler()
	require.NoError(t, err)

	e.serverAPIVersion = version.Must(version.NewVersion("1.43"))

	return c, e
}

func prepareTestDockerConfiguration(
	t *testing.T,
	dockerConfig *common.DockerConfig,
	cce containerConfigExpectations,
	expectedInspectImage string,
	expectedPullImage string, //nolint:unparam
) (*dockerConfigurationTestFakeDockerClient, *executor) {
	c, e := createExecutorForTestDockerConfiguration(t, dockerConfig, cce)

	c.On("ImageInspectWithRaw", mock.Anything, expectedInspectImage, mock.Anything).
		Return(image.InspectResponse{ID: "123"}, []byte{}, nil).Twice()
	c.On("ImagePullBlocking", mock.Anything, expectedPullImage, mock.Anything).
		Return(nil).Once()
	c.On("NetworkList", mock.Anything, mock.Anything).
		Return([]network.Summary{}, nil).Once()
	c.On("ContainerRemove", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()

	return c, e
}

func testDockerConfigurationWithJobContainer(
	t *testing.T,
	dockerConfig *common.DockerConfig,
	cce containerConfigExpectations,
) {
	c, e := prepareTestDockerConfiguration(t, dockerConfig, cce, "alpine", "alpine:latest")
	c.On("ContainerInspect", mock.Anything, "abc").
		Return(container.InspectResponse{}, nil).Once()

	err := e.createVolumesManager()
	require.NoError(t, err)

	err = e.createPullManager()
	require.NoError(t, err)

	imageConfig := spec.Image{Name: "alpine"}
	cfgTor := newDefaultContainerConfigurator(e, buildContainerType, imageConfig, []string{"/bin/sh"}, []string{})
	_, err = e.createContainer(buildContainerType, imageConfig, []string{}, cfgTor)
	assert.NoError(t, err, "Should create container without errors")
}

func testDockerConfigurationWithPredefinedContainer(
	t *testing.T,
	dockerConfig *common.DockerConfig,
	cce containerConfigExpectations,
) {
	c, e := prepareTestDockerConfiguration(t, dockerConfig, cce, "alpine", "alpine:latest")

	c.On("ContainerInspect", mock.Anything, "abc").
		Return(container.InspectResponse{}, nil).Once()

	err := e.createVolumesManager()
	require.NoError(t, err)

	err = e.createPullManager()
	require.NoError(t, err)

	imageConfig := spec.Image{Name: "alpine"}
	cfgTor := newDefaultContainerConfigurator(e, predefinedContainerType, imageConfig, []string{"/bin/sh"}, []string{})
	_, err = e.createContainer(buildContainerType, imageConfig, []string{}, cfgTor)
	assert.NoError(t, err, "Should create container without errors")
}

func TestDockerPIDMode(t *testing.T) {
	tests := map[string]struct {
		pidMode    string
		privileged bool
	}{
		"unset":                         {pidMode: ""},
		"host":                          {pidMode: "host"},
		"container reference":           {pidMode: "container:some-other-container"},
		"host with privileged":          {pidMode: "host", privileged: true},
		"container ref with privileged": {pidMode: "container:some-other-container", privileged: true},
		"unset with privileged":         {pidMode: "", privileged: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			dockerConfig := &common.DockerConfig{
				PidMode:    tt.pidMode,
				Privileged: tt.privileged,
			}

			expectPidModeOnBuild := func(t *testing.T, _ *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
				assert.Equal(t, container.PidMode(tt.pidMode), hostConfig.PidMode)
				assert.Equal(t, tt.privileged, hostConfig.Privileged)
			}
			expectNoPidMode := func(t *testing.T, _ *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
				assert.Zero(t, hostConfig.PidMode)
				assert.Equal(t, tt.privileged, hostConfig.Privileged)
			}

			testDockerConfigurationWithJobContainer(t, dockerConfig, expectPidModeOnBuild)
			testDockerConfigurationWithPredefinedContainer(t, dockerConfig, expectNoPidMode)
			testDockerConfigurationWithServiceContainer(t, dockerConfig, expectNoPidMode)
		})
	}
}

func TestDockerMemorySetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		Memory: "42m",
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		assert.Equal(t, int64(44040192), hostConfig.Memory)
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
}

func TestDockerMemorySwapSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		MemorySwap: "2g",
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		assert.Equal(t, int64(2147483648), hostConfig.MemorySwap)
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
}

func TestDockerMemoryReservationSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		MemoryReservation: "64m",
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		assert.Equal(t, int64(67108864), hostConfig.MemoryReservation)
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
}

func TestDockerCPUSSetting(t *testing.T) {
	examples := []struct {
		cpus     string
		nanocpus int64
	}{
		{"0.5", 500000000},
		{"0.25", 250000000},
		{"1/3", 333333333},
		{"1/8", 125000000},
		{"0.0001", 100000},
	}

	for _, example := range examples {
		t.Run(example.cpus, func(t *testing.T) {
			dockerConfig := &common.DockerConfig{
				CPUS: example.cpus,
			}

			cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
				assert.Equal(t, example.nanocpus, hostConfig.NanoCPUs)
			}

			testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
		})
	}
}

func TestDockerIsolationWithCorrectValues(t *testing.T) {
	isolations := []string{"default", ""}
	if runtime.GOOS == helperimage.OSTypeWindows {
		isolations = append(isolations, "hyperv", "process")
	}

	for _, isolation := range isolations {
		t.Run(isolation, func(t *testing.T) {
			dockerConfig := &common.DockerConfig{
				Isolation: isolation,
			}

			cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
				assert.Equal(t, container.Isolation(isolation), hostConfig.Isolation)
			}

			testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
		})
	}
}

func TestDockerIsolationWithIncorrectValue(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		Isolation: "someIncorrectValue",
	}
	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
	}
	_, executor := createExecutorForTestDockerConfiguration(t, dockerConfig, cce)

	_, err := executor.createHostConfig(false, false)

	assert.Contains(t, err.Error(), `the isolation value "someIncorrectValue" is not valid`)
}

func TestDockerServiceContainerConfigIncludesDockerLabels(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		HelperImage:     "gitlab/gitlab-runner:${CI_RUNNER_REVISION}",
		ContainerLabels: map[string]string{"my.custom.dockerConfigLabel": "dockerConfigLabelValue"},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
	}
	_, executor := createExecutorForTestDockerConfiguration(t, dockerConfig, cce)

	containerConfig := executor.createServiceContainerConfig("postgres", "15-alpine", "abc123def456", spec.Image{Name: "postgres:15-alpine"})

	expectedLabels := map[string]string{
		// default labels
		"com.gitlab.gitlab-runner.job.before_sha":    "",
		"com.gitlab.gitlab-runner.job.id":            "0",
		"com.gitlab.gitlab-runner.job.ref":           "",
		"com.gitlab.gitlab-runner.job.sha":           "",
		"com.gitlab.gitlab-runner.job.timeout":       "2h0m0s",
		"com.gitlab.gitlab-runner.job.url":           "/-/jobs/0",
		"com.gitlab.gitlab-runner.managed":           "true",
		"com.gitlab.gitlab-runner.pipeline.id":       "",
		"com.gitlab.gitlab-runner.project.id":        "0",
		"com.gitlab.gitlab-runner.project.runner_id": "0",
		"com.gitlab.gitlab-runner.runner.id":         "",
		"com.gitlab.gitlab-runner.runner.local_id":   "0",
		"com.gitlab.gitlab-runner.runner.system_id":  "",
		"com.gitlab.gitlab-runner.service":           "postgres",
		"com.gitlab.gitlab-runner.service.version":   "15-alpine",
		"com.gitlab.gitlab-runner.type":              "service",
		// from user-defined config
		"my.custom.dockerConfigLabel": "dockerConfigLabelValue",
		// NOTE: this is only here for backwards-compatibility
		// see https://gitlab.com/gitlab-org/gitlab-runner/-/issues/39048
		"com.gitlab.gitlab-runner.my.custom.dockerConfigLabel": "dockerConfigLabelValue",
	}

	assert.Equal(t, expectedLabels, containerConfig.Labels)
}

func TestDockerMacAddress(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		MacAddress: "92:d0:c6:0a:29:33",
	}

	cce := func(t *testing.T, _ *container.Config, _ *container.HostConfig, netConfig *network.NetworkingConfig) {
		for _, ec := range netConfig.EndpointsConfig {
			assert.Equal(t, "92:d0:c6:0a:29:33", ec.MacAddress)
		}
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
}

func TestDockerBuildContainerNativeStepsRecoveryEnv(t *testing.T) {
	if runtime.GOOS == helperimage.OSTypeWindows {
		t.Skip("UseNativeSteps returns false on Windows")
	}

	dockerConfig := &common.DockerConfig{}

	cce := func(t *testing.T, config *container.Config, _ *container.HostConfig, _ *network.NetworkingConfig) {
		assert.Nil(t, config.Entrypoint, "image entrypoint must not be cleared")
		require.GreaterOrEqual(t, len(config.Cmd), 3)
		assert.EqualValues(t,
			[]string{bootstrappedBinary, "steps", "serve"},
			config.Cmd[:3])
		expected, err := steps.EncodeRecoveryArgv(config.Cmd[1:])
		require.NoError(t, err)
		assert.Equal(t, []string{steps.RecoveryEnvVar + "=" + expected}, config.Env)
	}

	c, e := prepareTestDockerConfiguration(t, dockerConfig, cce, "alpine", "alpine:latest")
	c.On("ContainerInspect", mock.Anything, "abc").
		Return(container.InspectResponse{}, nil).Once()

	e.Build.ExecutorFeatures.NativeStepsIntegration = true
	e.Build.Variables = append(e.Build.Variables, spec.Variable{
		Key:   featureflags.UseScriptToStepMigration,
		Value: "true",
	})

	require.NoError(t, e.createVolumesManager())
	require.NoError(t, e.createPullManager())

	imageConfig := spec.Image{Name: "alpine"}
	cfgTor := newDefaultContainerConfigurator(e, buildContainerType, imageConfig, []string{"/bin/sh"}, []string{})
	_, err := e.createContainer(buildContainerType, imageConfig, []string{}, cfgTor)
	require.NoError(t, err)
}

func TestDockerCgroupParentSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		CgroupParent: "test-docker-cgroup",
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		assert.Equal(t, "test-docker-cgroup", hostConfig.CgroupParent)
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
}

func TestDockerCPUSetCPUsSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		CPUSetCPUs: "1-3,5",
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		assert.Equal(t, "1-3,5", hostConfig.CpusetCpus)
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
}

func TestDockerCPUSetMemsSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		CPUSetMems: "1-3,5",
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		assert.Equal(t, "1-3,5", hostConfig.CpusetMems)
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
}

func TestDockerServiceSettings(t *testing.T) {
	tests := map[string]struct {
		dockerConfig common.DockerConfig
		verifyFn     func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig)
	}{
		"memory": {
			dockerConfig: common.DockerConfig{
				ServiceMemory: "42m",
			},
			verifyFn: func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
				value, err := units.RAMInBytes("42m")
				require.NoError(t, err)
				assert.Equal(t, value, hostConfig.Memory)
			},
		},
		"memory reservation": {
			dockerConfig: common.DockerConfig{
				ServiceMemoryReservation: "64m",
			},
			verifyFn: func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
				value, err := units.RAMInBytes("64m")
				require.NoError(t, err)
				assert.Equal(t, value, hostConfig.MemoryReservation)
			},
		},
		"swap": {
			dockerConfig: common.DockerConfig{
				ServiceMemorySwap: "2g",
			},
			verifyFn: func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
				value, err := units.RAMInBytes("2g")
				require.NoError(t, err)
				assert.Equal(t, value, hostConfig.MemorySwap)
			},
		},
		"CgroupParent": {
			dockerConfig: common.DockerConfig{
				ServiceCgroupParent: "test-docker-cgroup",
			},
			verifyFn: func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
				assert.Equal(t, "test-docker-cgroup", hostConfig.CgroupParent)
			},
		},
		"CPUSetCPUs": {
			dockerConfig: common.DockerConfig{
				ServiceCPUSetCPUs: "1-3,5",
			},
			verifyFn: func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
				assert.Equal(t, "1-3,5", hostConfig.CpusetCpus)
			},
		},
		"cpus_0.5": {
			dockerConfig: common.DockerConfig{
				ServiceCPUS: "0.5",
			},
			verifyFn: func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
				assert.Equal(t, int64(500000000), hostConfig.NanoCPUs)
			},
		},
		"cpus_0.25": {
			dockerConfig: common.DockerConfig{
				ServiceCPUS: "0.25",
			},
			verifyFn: func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
				assert.Equal(t, int64(250000000), hostConfig.NanoCPUs)
			},
		},
		"cpus_1/3": {
			dockerConfig: common.DockerConfig{
				ServiceCPUS: "1/3",
			},
			verifyFn: func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
				assert.Equal(t, int64(333333333), hostConfig.NanoCPUs)
			},
		},
		"cpus_1/8": {
			dockerConfig: common.DockerConfig{
				ServiceCPUS: "1/8",
			},
			verifyFn: func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
				assert.Equal(t, int64(125000000), hostConfig.NanoCPUs)
			},
		},
		"cpus_0.0001": {
			dockerConfig: common.DockerConfig{
				ServiceCPUS: "0.0001",
			},
			verifyFn: func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
				assert.Equal(t, int64(100000), hostConfig.NanoCPUs)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			testDockerConfigurationWithServiceContainer(t, &tt.dockerConfig, tt.verifyFn)
		})
	}
}

func TestDockerContainerLabelsSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		ContainerLabels: map[string]string{"my.custom.label": "my.custom.value"},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		expected := map[string]string{
			"com.gitlab.gitlab-runner.job.before_sha":    "",
			"com.gitlab.gitlab-runner.job.id":            "0",
			"com.gitlab.gitlab-runner.job.ref":           "",
			"com.gitlab.gitlab-runner.job.sha":           "",
			"com.gitlab.gitlab-runner.job.url":           "/-/jobs/0",
			"com.gitlab.gitlab-runner.job.timeout":       "2h0m0s",
			"com.gitlab.gitlab-runner.managed":           "true",
			"com.gitlab.gitlab-runner.pipeline.id":       "",
			"com.gitlab.gitlab-runner.project.id":        "0",
			"com.gitlab.gitlab-runner.project.runner_id": "0",
			"com.gitlab.gitlab-runner.runner.id":         "",
			"com.gitlab.gitlab-runner.runner.local_id":   "0",
			"com.gitlab.gitlab-runner.runner.system_id":  "",
			"com.gitlab.gitlab-runner.type":              "build",
			"my.custom.label":                            "my.custom.value",
		}

		assert.Equal(t, expected, config.Labels)
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
}

func TestDockerTmpfsSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		Tmpfs: map[string]string{
			"/tmpfs": "rw,noexec",
		},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		require.NotEmpty(t, hostConfig.Tmpfs)
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
}

func TestDockerServicesDevicesSetting(t *testing.T) {
	tests := map[string]struct {
		devices                map[string][]string
		expectedDeviceMappings []container.DeviceMapping
	}{
		"same host and container path": {
			devices: map[string][]string{
				"alpine:*": {"/dev/usb:/dev/usb:ro"},
				"alp*":     {"/dev/kvm", "/dev/dri"},
				"nomatch":  {"/dev/null"},
			},
			expectedDeviceMappings: []container.DeviceMapping{
				{
					PathOnHost:        "/dev/usb",
					PathInContainer:   "/dev/usb",
					CgroupPermissions: "ro",
				},
				{
					PathOnHost:        "/dev/kvm",
					PathInContainer:   "/dev/kvm",
					CgroupPermissions: "rwm",
				},
				{
					PathOnHost:        "/dev/dri",
					PathInContainer:   "/dev/dri",
					CgroupPermissions: "rwm",
				},
			},
		},
		"different host and container path": {
			devices: map[string][]string{
				"alpine:*": {"/dev/usb:/dev/xusb:ro"},
				"alp*":     {"/dev/kvm:/dev/xkvm", "/dev/dri"},
				"nomatch":  {"/dev/null"},
			},
			expectedDeviceMappings: []container.DeviceMapping{
				{
					PathOnHost:        "/dev/usb",
					PathInContainer:   "/dev/xusb",
					CgroupPermissions: "ro",
				},
				{
					PathOnHost:        "/dev/kvm",
					PathInContainer:   "/dev/xkvm",
					CgroupPermissions: "rwm",
				},
				{
					PathOnHost:        "/dev/dri",
					PathInContainer:   "/dev/dri",
					CgroupPermissions: "rwm",
				},
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			dockerConfig := &common.DockerConfig{
				ServicesDevices: tt.devices,
			}
			cce := func(ttt *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
				require.NotEmpty(ttt, hostConfig.Resources.Devices)
				assert.ElementsMatch(ttt, tt.expectedDeviceMappings, hostConfig.Resources.Devices)
			}
			testDockerConfigurationWithServiceContainer(t, dockerConfig, cce)
		})
	}
}

func TestDockerGetServicesDevices(t *testing.T) {
	tests := map[string]struct {
		image                  string
		devices                map[string][]string
		expectedDeviceMappings []container.DeviceMapping
		expectedErrorSubstr    string
	}{
		"matching image": {
			image: "alpine:latest",
			devices: map[string][]string{
				"alpine:*": {"/dev/null"},
			},
			expectedDeviceMappings: []container.DeviceMapping{
				{
					PathOnHost:        "/dev/null",
					PathInContainer:   "/dev/null",
					CgroupPermissions: "rwm",
				},
			},
			expectedErrorSubstr: "",
		},
		"one matching image": {
			image: "alpine:latest",
			devices: map[string][]string{
				"alpine:*": {"/dev/null"},
				"fedora:*": {"/dev/usb"},
			},
			expectedDeviceMappings: []container.DeviceMapping{
				{
					PathOnHost:        "/dev/null",
					PathInContainer:   "/dev/null",
					CgroupPermissions: "rwm",
				},
			},
			expectedErrorSubstr: "",
		},
		"multiple matching images": {
			image: "alpine:latest",
			devices: map[string][]string{
				"alpine:*":      {"/dev/null"},
				"alpine:latest": {"/dev/usb"},
			},
			expectedDeviceMappings: []container.DeviceMapping{
				{
					PathOnHost:        "/dev/null",
					PathInContainer:   "/dev/null",
					CgroupPermissions: "rwm",
				},
				{
					PathOnHost:        "/dev/usb",
					PathInContainer:   "/dev/usb",
					CgroupPermissions: "rwm",
				},
			},
			expectedErrorSubstr: "",
		},
		"no devices": {
			image: "alpine:latest",
			devices: map[string][]string{
				"alpine:*": {},
			},
			expectedDeviceMappings: nil,
			expectedErrorSubstr:    "",
		},
		"no matching image": {
			image: "alpine:latest",
			devices: map[string][]string{
				"ubuntu:*": {"/dev/null"},
			},
			expectedDeviceMappings: nil,
			expectedErrorSubstr:    "",
		},
		"devices is nil": {
			image:                  "alpine:latest",
			devices:                nil,
			expectedDeviceMappings: nil,
			expectedErrorSubstr:    "",
		},
		"multiple devices": {
			image: "private.registry:5000/emulator/OSv7:26",
			devices: map[string][]string{
				"private.registry:5000/emulator/*": {"/dev/kvm", "/dev/dri"},
			},
			expectedDeviceMappings: []container.DeviceMapping{
				{
					PathOnHost:        "/dev/kvm",
					PathInContainer:   "/dev/kvm",
					CgroupPermissions: "rwm",
				},
				{
					PathOnHost:        "/dev/dri",
					PathInContainer:   "/dev/dri",
					CgroupPermissions: "rwm",
				},
			},
			expectedErrorSubstr: "",
		},
		"parseDeviceString error": {
			image: "alpine:latest",
			devices: map[string][]string{
				"alpine:*": {"/dev/null::::"},
			},
			expectedDeviceMappings: nil,
			expectedErrorSubstr:    "too many colons",
		},
		"bad glob pattern": {
			image: "alpine:latest",
			devices: map[string][]string{
				"alpin[e:*": {"/dev/usb:/dev/usb:ro"},
			},
			expectedErrorSubstr: "invalid service device image pattern: alpin[e",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			e := &executor{}
			e.Config.Docker = &common.DockerConfig{
				ServicesDevices: tt.devices,
			}

			mappings, err := e.getServicesDevices(tt.image)
			if tt.expectedErrorSubstr != "" {
				assert.Contains(t, fmt.Sprintf("%+v", err), tt.expectedErrorSubstr)
				return
			}

			assert.ElementsMatch(t, tt.expectedDeviceMappings, mappings)
		})
	}
}

func TestDockerServicesDeviceRequestsSetting(t *testing.T) {
	tests := map[string]struct {
		gpus                   string
		expectedDeviceRequests []container.DeviceRequest
	}{
		"request all GPUs": {
			gpus: "all",
			expectedDeviceRequests: []container.DeviceRequest{
				{
					Driver:       "",
					Count:        -1,
					DeviceIDs:    nil,
					Capabilities: [][]string{{"gpu"}},
					Options:      map[string]string{},
				},
			},
		},
		"gpus is empty string": {
			gpus:                   "",
			expectedDeviceRequests: nil,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			dockerConfig := &common.DockerConfig{
				ServiceGpus: tt.gpus,
			}
			cce := func(ttt *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
				assert.ElementsMatch(ttt, tt.expectedDeviceRequests, hostConfig.Resources.DeviceRequests)
			}
			testDockerConfigurationWithServiceContainer(t, dockerConfig, cce)
		})
	}
}

func TestDockerGetServicesDeviceRequests(t *testing.T) {
	tests := map[string]struct {
		gpus                   string
		expectedDeviceRequests []container.DeviceRequest
		expectedErrorSubstr    string
	}{
		"request all GPUs": {
			gpus: "all",
			expectedDeviceRequests: []container.DeviceRequest{
				{
					Driver:       "",
					Count:        -1,
					DeviceIDs:    nil,
					Capabilities: [][]string{{"gpu"}},
					Options:      map[string]string{},
				},
			},
			expectedErrorSubstr: "",
		},
		"request GPUs by device ID": {
			gpus: "\"device=1,2\"",
			expectedDeviceRequests: []container.DeviceRequest{
				{
					Driver:       "",
					Count:        0,
					DeviceIDs:    []string{"1", "2"},
					Capabilities: [][]string{{"gpu"}},
					Options:      map[string]string{},
				},
			},
			expectedErrorSubstr: "",
		},
		"request GPUs by count": {
			gpus: "2",
			expectedDeviceRequests: []container.DeviceRequest{
				{
					Driver:       "",
					Count:        2,
					DeviceIDs:    nil,
					Capabilities: [][]string{{"gpu"}},
					Options:      map[string]string{},
				},
			},
			expectedErrorSubstr: "",
		},
		"gpus is empty string": {
			gpus:                   "",
			expectedDeviceRequests: nil,
			expectedErrorSubstr:    "",
		},
		"parse gpus string error": {
			gpus:                   "somestring=thatshouldtriggeranerror",
			expectedDeviceRequests: nil,
			expectedErrorSubstr:    "unexpected key 'somestring' in 'somestring=thatshouldtriggeranerror'",
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			e := &executor{}
			e.Config.Docker = &common.DockerConfig{
				ServiceGpus: tt.gpus,
			}

			deviceRequests, err := e.getServicesDeviceRequests()
			if tt.expectedErrorSubstr != "" {
				assert.Contains(t, fmt.Sprintf("%+v", err), tt.expectedErrorSubstr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectedDeviceRequests, deviceRequests)
		})
	}
}

func TestDockerUserSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		User: "www",
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		assert.Equal(t, "www", config.User)
	}
	ccePredefined := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		assert.Equal(t, "", config.User)
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
	testDockerConfigurationWithPredefinedContainer(t, dockerConfig, ccePredefined)
}

func TestDockerUserNSSetting(t *testing.T) {
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

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
	testDockerConfigurationWithJobContainer(t, dockerConfigWithHostUsernsMode, cceWithHostUsernsMode)
}

func TestDockerRuntimeSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		Runtime: "runc",
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		assert.Equal(t, "runc", hostConfig.Runtime)
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
}

func TestDockerSysctlsSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		SysCtls: map[string]string{
			"net.ipv4.ip_forward": "1",
		},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		assert.Equal(t, "1", hostConfig.Sysctls["net.ipv4.ip_forward"])
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
}

func TestDockerUlimitSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{}

	tests := map[string]struct {
		ulimit         map[string]string
		expectedUlimit []*units.Ulimit
		expectedError  bool
	}{
		"soft and hard values": {
			ulimit: map[string]string{
				"nofile": "1024:2048",
			},
			expectedUlimit: []*units.Ulimit{
				{
					Name: "nofile",
					Soft: 1024,
					Hard: 2048,
				},
			},
			expectedError: false,
		},
		"single limit value": {
			ulimit: map[string]string{
				"nofile": "1024",
			},
			expectedUlimit: []*units.Ulimit{
				{
					Name: "nofile",
					Soft: 1024,
					Hard: 1024,
				},
			},
			expectedError: false,
		},
		"invalid limit value": {
			ulimit: map[string]string{
				"nofile": "a",
			},
			expectedError: true,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			dockerConfig.Ulimit = test.ulimit

			ulimits, err := dockerConfig.GetUlimits()
			if test.expectedError {
				assert.Error(t, err)
				return
			}

			assert.Equal(t, ulimits, test.expectedUlimit)
		})
	}
}

type testAllowedPrivilegedJobDescription struct {
	expectedPrivileged bool
	privileged         bool
	allowedImages      []string
}

var testAllowedPrivilegedJob = []testAllowedPrivilegedJobDescription{
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

func TestDockerPrivilegedJobSetting(t *testing.T) {
	for _, test := range testAllowedPrivilegedJob {
		dockerConfig := &common.DockerConfig{
			Privileged:              test.privileged,
			AllowedPrivilegedImages: test.allowedImages,
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

		testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
	}
}

type networksTestCase struct {
	clientAssertions          func(*docker.MockClient)
	networksManagerAssertions func(*networks.MockManager)
	createNetworkManager      bool
	networkPerBuild           string
	expectedBuildError        error
	expectedCleanError        error
}

func TestDockerCreateNetwork(t *testing.T) {
	testErr := errors.New("test-err")

	tests := map[string]networksTestCase{
		"networks manager not created": {
			networkPerBuild:    "false",
			expectedBuildError: errNetworksManagerUndefined,
			expectedCleanError: errNetworksManagerUndefined,
		},
		"network not created": {
			createNetworkManager: true,
			networkPerBuild:      "false",
			networksManagerAssertions: func(nm *networks.MockManager) {
				nm.On("Create", mock.Anything, mock.Anything, mock.Anything).
					Return(container.NetworkMode("test"), nil).
					Once()
				nm.On("Inspect", mock.Anything).
					Return(network.Inspect{}, nil).
					Once()
				nm.On("Cleanup", mock.Anything).
					Return(nil).
					Once()
			},
		},
		"network created": {
			createNetworkManager: true,
			networkPerBuild:      "true",
			networksManagerAssertions: func(nm *networks.MockManager) {
				nm.On("Create", mock.Anything, mock.Anything, mock.Anything).
					Return(container.NetworkMode("test"), nil).
					Once()
				nm.On("Inspect", mock.Anything).
					Return(network.Inspect{}, nil).
					Once()
				nm.On("Cleanup", mock.Anything).
					Return(nil).
					Once()
			},
		},
		"network creation failed": {
			createNetworkManager: true,
			networkPerBuild:      "true",
			networksManagerAssertions: func(nm *networks.MockManager) {
				nm.On("Create", mock.Anything, mock.Anything, mock.Anything).
					Return(container.NetworkMode("fail"), testErr).
					Once()
			},
			expectedBuildError: testErr,
		},
		"network inspect failed": {
			createNetworkManager: true,
			networkPerBuild:      "true",
			networksManagerAssertions: func(nm *networks.MockManager) {
				nm.On("Create", mock.Anything, mock.Anything, mock.Anything).
					Return(container.NetworkMode("test"), nil).
					Once()
				nm.On("Inspect", mock.Anything).
					Return(network.Inspect{}, testErr).
					Once()
			},
			expectedCleanError: nil,
		},
		"removing container failed": {
			createNetworkManager: true,
			networkPerBuild:      "true",
			clientAssertions: func(c *docker.MockClient) {
				c.On("NetworkList", mock.Anything, mock.Anything).
					Return([]network.Summary{}, nil).
					Once()
				c.On("ContainerRemove", mock.Anything, mock.Anything, mock.Anything).
					Return(testErr).
					Once()
			},
			networksManagerAssertions: func(nm *networks.MockManager) {
				nm.On("Create", mock.Anything, mock.Anything, mock.Anything).
					Return(container.NetworkMode("test"), nil).
					Once()
				nm.On("Inspect", mock.Anything).
					Return(
						network.Inspect{
							Containers: map[string]network.EndpointResource{
								"abc": {},
							},
						},
						nil,
					).
					Once()
				nm.On("Cleanup", mock.Anything).
					Return(nil).
					Once()
			},
			expectedCleanError: nil,
		},
		"network cleanup failed": {
			createNetworkManager: true,
			networkPerBuild:      "true",
			networksManagerAssertions: func(nm *networks.MockManager) {
				nm.On("Create", mock.Anything, mock.Anything, mock.Anything).
					Return(container.NetworkMode("test"), nil).
					Once()
				nm.On("Inspect", mock.Anything).
					Return(network.Inspect{}, nil).
					Once()
				nm.On("Cleanup", mock.Anything).
					Return(testErr).
					Once()
			},
			expectedCleanError: testErr,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			e := getExecutorForNetworksTests(t, test)

			err := e.createBuildNetwork()
			assert.ErrorIs(t, err, test.expectedBuildError)

			err = e.cleanupNetwork(t.Context())
			assert.ErrorIs(t, err, test.expectedCleanError)
		})
	}
}

func getExecutorForNetworksTests(t *testing.T, test networksTestCase) *executor {
	t.Helper()

	clientMock := docker.NewMockClient(t)
	networksManagerMock := networks.NewMockManager(t)

	oldCreateNetworksManager := createNetworksManager
	t.Cleanup(func() {
		createNetworksManager = oldCreateNetworksManager
	})

	createNetworksManager = func(_ *executor) (networks.Manager, error) {
		return networksManagerMock, nil
	}

	if test.networksManagerAssertions != nil {
		test.networksManagerAssertions(networksManagerMock)
	}

	if test.clientAssertions != nil {
		test.clientAssertions(clientMock)
	}

	c := common.RunnerConfig{
		RunnerCredentials: common.RunnerCredentials{
			Token: "abcdef1234567890",
		},
	}
	c.Docker = &common.DockerConfig{
		NetworkMode: "",
	}
	e := &executor{
		AbstractExecutor: executors.AbstractExecutor{
			BuildLogger: buildlogger.New(nil, logrus.WithFields(logrus.Fields{}), buildlogger.Options{}),
			Build: &common.Build{
				ProjectRunnerID: 0,
				Runner:          &c,
				Job: spec.Job{
					JobInfo: spec.JobInfo{
						ProjectID: 0,
					},
					GitInfo: spec.GitInfo{
						RepoURL: "https://gitlab.example.com/group/project.git",
					},
				},
			},
			Config: c,
			ExecutorOptions: executors.ExecutorOptions{
				DefaultBuildsDir: volumesTestsDefaultBuildsDir,
				DefaultCacheDir:  volumesTestsDefaultCacheDir,
			},
		},
		dockerConn: &dockerConnection{Client: clientMock},
		info: system.Info{
			OSType: helperimage.OSTypeLinux,
		},
	}

	e.Context = t.Context()
	e.Build.Variables = append(e.Build.Variables, spec.Variable{
		Key:   featureflags.NetworkPerBuild,
		Value: test.networkPerBuild,
	})

	if test.createNetworkManager {
		err := e.createNetworksManager()
		require.NoError(t, err)
	}

	return e
}

func TestCheckOSType(t *testing.T) {
	cases := map[string]struct {
		dockerInfoOSType string
		expectedErr      string
	}{
		"linux type": {
			dockerInfoOSType: osTypeLinux,
		},
		"windows type": {
			dockerInfoOSType: osTypeWindows,
		},
		"freebsd type": {
			dockerInfoOSType: osTypeFreeBSD,
		},
		"unknown": {
			dockerInfoOSType: "foobar",
			expectedErr:      "unsupported os type: foobar",
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			e := executor{
				info: system.Info{
					OSType: c.dockerInfoOSType,
				},
				AbstractExecutor: executors.AbstractExecutor{},
			}

			err := validateOSType(e.info)
			if c.expectedErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.EqualError(t, err, c.expectedErr)
		})
	}
}

func TestHelperImageRegistry(t *testing.T) {
	tests := map[string]struct {
		config *common.DockerConfig
		// We only validate the name because we only care if the right image is
		// used. We don't want to end up having this test as a "spellcheck" to
		// make sure tags and commands are generated correctly since that is
		// done at a unit level already and we would be duplicating internal
		// logic and leaking abstractions.
		expectedHelperImageName string
	}{
		"Default helper image": {
			config:                  &common.DockerConfig{},
			expectedHelperImageName: helperimage.GitLabRegistryName,
		},
		"helper image overridden still use default helper image in prepare": {
			config: &common.DockerConfig{
				HelperImage: "private.registry.com/helper",
			},
			// We expect the default image to still be chosen since the check of
			// the override happens at a later stage.
			expectedHelperImageName: helperimage.GitLabRegistryName,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			e := &executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: executors.ExecutorOptions{},
				},
				newVolumePermissionSetter: func() (permission.Setter, error) {
					return nil, nil
				},
			}

			e.Build = &common.Build{}
			e.info = system.Info{
				OSType: "linux",
			}
			e.Config.Docker = tt.config

			helperImageInfo, err := e.prepareHelperImage()
			require.NoError(t, err)

			assert.Equal(t, tt.expectedHelperImageName, helperImageInfo.Name)
		})
	}
}

func TestLocalHelperImage(t *testing.T) {
	imageName := func(prefix, suffix string) string {
		return fmt.Sprintf("%s:%s%s%s", helperimage.GitLabRegistryName, prefix, "x86_64-latest", suffix)
	}

	createFakePrebuiltImages(t, "x86_64")

	tests := map[string]struct {
		jobVariables     spec.Variables
		config           helperimage.Config
		clientAssertions func(*docker.MockClient)
		expectedImage    *image.InspectResponse
	}{
		"docker import using registry.gitlab.com name": {
			config: helperimage.Config{
				Architecture: "amd64",
				OSType:       osTypeLinux,
			},
			clientAssertions: func(c *docker.MockClient) {
				c.On(
					"ImageImportBlocking",
					mock.Anything,
					mock.Anything,
					helperimage.GitLabRegistryName,
					client.ImageImportOptions{
						Tag: "x86_64-latest",
						Changes: []string{
							`ENTRYPOINT ["/usr/bin/dumb-init", "/entrypoint"]`,
						},
					},
				).Return(nil)

				imageInspect := image.InspectResponse{
					RepoTags: []string{
						imageName("", ""),
					},
				}

				c.On(
					"ImageInspectWithRaw",
					mock.Anything,
					imageName("", ""),
					mock.Anything,
				).Return(imageInspect, []byte{}, nil)
			},
			expectedImage: &image.InspectResponse{
				RepoTags: []string{
					imageName("", ""),
				},
			},
		},
		"docker import nil is returned if error": {
			config: helperimage.Config{
				Architecture: "amd64",
				OSType:       osTypeLinux,
			},
			clientAssertions: func(c *docker.MockClient) {
				c.On(
					"ImageImportBlocking",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(errors.New("error"))
			},
			expectedImage: nil,
		},
		"docker import nil is returned if error on inspect": {
			config: helperimage.Config{
				Architecture: "amd64",
				OSType:       osTypeLinux,
			},
			clientAssertions: func(c *docker.MockClient) {
				c.On(
					"ImageImportBlocking",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(nil)

				c.On(
					"ImageInspectWithRaw",
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(image.InspectResponse{}, []byte{}, errors.New("error"))
			},
			expectedImage: nil,
		},
		"powershell image is used when shell is pwsh": {
			config: helperimage.Config{
				Architecture: "amd64",
				OSType:       osTypeLinux,
				Shell:        shells.SNPwsh,
			},
			clientAssertions: func(c *docker.MockClient) {
				c.On(
					"ImageImportBlocking",
					mock.Anything,
					mock.MatchedBy(func(source client.ImageImportSource) bool {
						return assert.IsType(t, new(os.File), source.Source) &&
							assert.Equal(
								t,
								"prebuilt-alpine-x86_64-pwsh.tar.xz",
								filepath.Base((source.Source.(*os.File)).Name()),
							)
					}),
					helperimage.GitLabRegistryName,
					mock.Anything,
				).Return(nil)

				imageInspect := image.InspectResponse{
					RepoTags: []string{
						imageName("", "-pwsh"),
					},
				}

				c.On(
					"ImageInspectWithRaw",
					mock.Anything,
					imageName("", "-pwsh"),
					mock.Anything,
				).Return(imageInspect, []byte{}, nil)
			},
			expectedImage: &image.InspectResponse{
				RepoTags: []string{
					imageName("", "-pwsh"),
				},
			},
		},
		"powershell image is used when shell is pwsh and flavor ubuntu": {
			config: helperimage.Config{
				Architecture: "amd64",
				OSType:       osTypeLinux,
				Flavor:       "ubuntu",
				Shell:        shells.SNPwsh,
			},
			clientAssertions: func(c *docker.MockClient) {
				c.On(
					"ImageImportBlocking",
					mock.Anything,
					mock.MatchedBy(func(source client.ImageImportSource) bool {
						return assert.IsType(t, new(os.File), source.Source) &&
							assert.Equal(
								t,
								"prebuilt-ubuntu-x86_64-pwsh.tar.xz",
								filepath.Base((source.Source.(*os.File)).Name()),
							)
					}),
					helperimage.GitLabRegistryName,
					mock.Anything,
				).Return(nil)

				imageInspect := image.InspectResponse{
					RepoTags: []string{
						imageName("ubuntu-", "-pwsh"),
					},
				}

				c.On(
					"ImageInspectWithRaw",
					mock.Anything,
					imageName("ubuntu-", "-pwsh"),
					mock.Anything,
				).Return(imageInspect, []byte{}, nil)
			},
			expectedImage: &image.InspectResponse{
				RepoTags: []string{
					imageName("ubuntu-", "-pwsh"),
				},
			},
		},
		"docker load docker image": {
			config: helperimage.Config{
				Architecture: "amd64",
				OSType:       osTypeLinux,
				Flavor:       "ubuntu",
			},
			clientAssertions: func(c *docker.MockClient) {
				c.On(
					"ImageLoad",
					mock.Anything,
					mock.Anything,
					true,
				).Return(io.NopCloser(strings.NewReader(`{"stream": "Loaded image ID: 1234"}`)), nil)

				c.On(
					"ImageTag",
					mock.Anything,
					"1234",
					imageName("ubuntu-", ""),
				).Return(nil)

				imageInspect := image.InspectResponse{
					RepoTags: []string{
						imageName("ubuntu-", ""),
					},
				}

				c.On(
					"ImageInspectWithRaw",
					mock.Anything,
					imageName("ubuntu-", ""),
					mock.Anything,
				).Return(imageInspect, []byte{}, nil)
			},
			expectedImage: &image.InspectResponse{
				RepoTags: []string{
					imageName("ubuntu-", ""),
				},
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			c := docker.NewMockClient(t)

			info, err := helperimage.Get("", tt.config)
			require.NoError(t, err)

			e := &executor{
				AbstractExecutor: executors.AbstractExecutor{
					Build: &common.Build{
						Job: spec.Job{
							Variables: tt.jobVariables,
						},
						Runner: &common.RunnerConfig{},
					},

					Config: common.RunnerConfig{
						RunnerSettings: common.RunnerSettings{
							Shell: tt.config.Shell,
							Docker: &common.DockerConfig{
								HelperImageFlavor: tt.config.Flavor,
							},
						},
					},
				},
				dockerConn:      &dockerConnection{Client: c},
				helperImageInfo: info,
			}

			tt.clientAssertions(c)

			image := e.getLocalHelperImage()
			assert.Equal(t, tt.expectedImage, image)
		})
	}
}

func createFakePrebuiltImages(t *testing.T, architecture string) {
	t.Helper()

	// Create fake image files so that tests do not need helper images built
	tempImgDir := t.TempDir()

	prevPrebuiltImagesPaths := prebuilt.PrebuiltImagesPaths
	t.Cleanup(func() {
		prebuilt.PrebuiltImagesPaths = prevPrebuiltImagesPaths
	})

	prebuilt.PrebuiltImagesPaths = []string{tempImgDir}
	for _, fakeImgName := range []string{
		fmt.Sprintf("prebuilt-alpine-%s.tar.xz", architecture),
		fmt.Sprintf("prebuilt-alpine-%s-pwsh.tar.xz", architecture),
		fmt.Sprintf("prebuilt-ubuntu-%s.tar.xz", architecture),
		fmt.Sprintf("prebuilt-ubuntu-%s-pwsh.tar.xz", architecture),
		fmt.Sprintf("prebuilt-ubuntu-%s.docker.tar.zst", architecture),
		fmt.Sprintf("prebuilt-windows-nanoserver-ltsc2019-%s.docker.tar.zst", architecture),
	} {
		require.NoError(t, os.WriteFile(filepath.Join(tempImgDir, fakeImgName), nil, 0666))
	}
}

func TestGetUIDandGID(t *testing.T) {
	ctx := t.Context()
	testContainerID := "test-ID"
	testImageSHA := "test-SHA"
	testUID := 456
	testGID := 789

	tests := map[string]struct {
		mockInspect   func(t *testing.T, i *user.MockInspect)
		expectedError error
	}{
		"UID check returns error": {
			mockInspect: func(t *testing.T, i *user.MockInspect) {
				i.On("UID", ctx, testContainerID).Return(0, assert.AnError).Once()
			},
			expectedError: assert.AnError,
		},
		"UID check succeeds, GID check returns error": {
			mockInspect: func(t *testing.T, i *user.MockInspect) {
				i.On("UID", ctx, testContainerID).Return(testUID, nil).Once()
				i.On("GID", ctx, testContainerID).Return(0, assert.AnError).Once()
			},
			expectedError: assert.AnError,
		},
		"both checks succeed": {
			mockInspect: func(t *testing.T, i *user.MockInspect) {
				i.On("UID", ctx, testContainerID).Return(testUID, nil).Once()
				i.On("GID", ctx, testContainerID).Return(testGID, nil).Once()
			},
			expectedError: nil,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			inspectMock := user.NewMockInspect(t)

			tt.mockInspect(t, inspectMock)

			log, _ := logrustest.NewNullLogger()
			uid, gid, err := getUIDandGID(ctx, log, inspectMock, testContainerID, testImageSHA)

			if tt.expectedError != nil {
				assert.Equal(t, 0, uid)
				assert.Equal(t, 0, gid)
				assert.ErrorIs(t, err, tt.expectedError)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, testUID, uid)
			assert.Equal(t, testGID, gid)
		})
	}
}

func TestExpandingDockerImageWithImagePullPolicyAlways(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		Memory: "42m",
	}
	imageConfig := spec.Image{
		Name:         "alpine",
		PullPolicies: []spec.PullPolicy{common.PullPolicyAlways},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		assert.Equal(t, int64(44040192), hostConfig.Memory)
	}

	c, e := prepareTestDockerConfiguration(t, dockerConfig, cce, "alpine", "alpine:latest")

	c.On("ContainerInspect", mock.Anything, "abc").
		Return(container.InspectResponse{}, nil).Once()

	err := e.createVolumesManager()
	require.NoError(t, err)

	err = e.createPullManager()
	require.NoError(t, err)

	cfgTor := newDefaultContainerConfigurator(e, buildContainerType, imageConfig, []string{"/bin/sh"}, []string{})
	_, err = e.createContainer(buildContainerType, imageConfig, []string{}, cfgTor)
	assert.NoError(t, err, "Should create container without errors")
}

func TestExpandingDockerImageWithImagePullPolicyNever(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		Memory: "42m",
	}
	imageConfig := spec.Image{
		Name:         "alpine",
		PullPolicies: []spec.PullPolicy{common.PullPolicyNever},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		assert.Equal(t, int64(44040192), hostConfig.Memory)
	}

	_, e := createExecutorForTestDockerConfiguration(t, dockerConfig, cce)

	err := e.createVolumesManager()
	require.NoError(t, err)

	err = e.createPullManager()
	require.NoError(t, err)

	cfgTor := newDefaultContainerConfigurator(e, buildContainerType, imageConfig, []string{"/bin/sh"}, []string{})
	_, err = e.createContainer(buildContainerType, imageConfig, []string{}, cfgTor)
	assert.Contains(
		t,
		err.Error(),
		`invalid pull policy for image "alpine"`,
	)
	assert.Contains(
		t,
		err.Error(),
		fmt.Sprintf("pull_policy (%v) defined in %s is not one of the allowed_pull_policies (%v)", "[never]", "GitLab pipeline config", "[always]"),
	)
}

func TestDockerImageWithVariablePlatform(t *testing.T) {
	// Test with and without setting the platform to make sure that variable expansion works in both cases
	for _, platform := range []string{"linux/amd64", ""} {
		c := docker.NewMockClient(t)
		p := pull.NewMockManager(t)

		// Ensure that the pull manager gets called with the expanded platform
		p.On("GetDockerImage", mock.Anything, spec.ImageDockerOptions{Platform: platform}, mock.Anything).
			Return(nil, nil).
			Once()

		e := executorWithMockClient(c)
		e.pullManager = p

		e.Config.Docker = &common.DockerConfig{}

		imageConfig := spec.Image{
			Name: "alpine",
			ExecutorOptions: spec.ImageExecutorOptions{
				Docker: spec.ImageDockerOptions{
					Platform: "${PLATFORM}",
				},
			},
			PullPolicies: []spec.PullPolicy{common.PullPolicyAlways},
		}

		e.Build.Variables = append(e.Build.Variables, spec.Variable{
			Key:   "PLATFORM",
			Value: platform,
		})

		_, err := e.expandAndGetDockerImage(imageConfig.Name, []string{}, imageConfig.ExecutorOptions.Docker, imageConfig.PullPolicies)
		assert.NoError(t, err)
	}
}

func TestImageReferenceForCreate(t *testing.T) {
	tests := map[string]struct {
		image    image.InspectResponse
		expected string
	}{
		"repo digest present, prefer it over the (possibly platform-specific) ID": {
			image: image.InspectResponse{
				ID:          "sha256:child-manifest-digest",
				RepoDigests: []string{"postgres@sha256:index-digest"},
			},
			expected: "postgres@sha256:index-digest",
		},
		"multiple repo digests, use the first": {
			image: image.InspectResponse{
				ID:          "sha256:child-manifest-digest",
				RepoDigests: []string{"postgres@sha256:index-digest", "docker.io/library/postgres@sha256:index-digest"},
			},
			expected: "postgres@sha256:index-digest",
		},
		"no repo digest, fall back to ID": {
			image: image.InspectResponse{
				ID: "sha256:locally-built-image-id",
			},
			expected: "sha256:locally-built-image-id",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.expected, imageReferenceForCreate(&tc.image))
		})
	}
}

func TestExpandingVolumeDestination(t *testing.T) {
	dockerClient := docker.NewMockClient(t)
	executor := executorWithMockClient(dockerClient)

	executor.Build = &common.Build{
		Job: spec.Job{
			Variables: spec.Variables{
				spec.Variable{Key: "JOB_VAR_1", Value: "1"},
				spec.Variable{Key: "JOB_VAR_2", Value: "2"},
				spec.Variable{Key: "COMBINED_VAR", Value: "${JOB_VAR_1}-${JOB_VAR_2}-3"},
			},
			JobInfo: spec.JobInfo{
				ProjectID: 1234,
			},
		},
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{
				Token: "theToken",
			},
			SystemID: "some-system-id",
		},
		ProjectRunnerID: 5678,
	}
	executor.Config = common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Docker: &common.DockerConfig{
				CacheDir: "",
				Volumes: []string{
					// source should not be expanded, destination should be expanded
					"/host/${COMBINED_VAR}:/tmp/${COMBINED_VAR}",
					// a new volume for the expanded destination should be created
					"/new/cache/vol-${COMBINED_VAR}-foo",
					// expected to be passed on as is
					"/${:/tmp",
					"/host:/tmp/foo/$",
				},
			},
		},
	}

	// We need to explicitly connect, as we don't run Prepare where this would usually happen.
	// In this context, this is only used to create a connection based on the mock client, and slap that onto the executor
	// struct for later use.
	err := executor.dockerConnector.Connect(t.Context(), common.ExecutorPrepareOptions{}, executor)
	assert.NoError(t, err, "connecting connector")

	executor.volumeParser = parser.NewLinuxParser(executor.ExpandValue)
	err = executor.createLabeler()
	assert.NoError(t, err, "creating labeler")
	err = executor.createVolumesManager()
	assert.NoError(t, err, "creating volumes manager")

	// for the cache volume we expect a volume creation call
	expectedVolume := func(co client.VolumeCreateOptions) bool {
		// name build from hashed runner/build stuff & the md5sum of the (expanded) destination ("/new/cache/vol-1-2-3-foo")
		isExpected := assert.Equal(t, "runner-cb27ac1df55ad5c5857ef343b03639cf-cache-bffb7fe32becf1f1e4d6c9604d09f9d7", co.Name)

		// check for some labels, specifically the ones that moved from the volume name to metadata
		expectedLabels := map[string]string{
			"com.gitlab.gitlab-runner.project.id":        "1234",
			"com.gitlab.gitlab-runner.project.runner_id": "5678",
			"com.gitlab.gitlab-runner.runner.id":         "theToken",
			"com.gitlab.gitlab-runner.runner.system_id":  "some-system-id",
		}
		for expectedKey, expectedValue := range expectedLabels {
			actualValue, exists := co.Labels[expectedKey]
			isExpected = isExpected &&
				assert.True(t, exists, "expected volume label %q, but got none", expectedKey) &&
				assert.Equal(t, expectedValue, actualValue, "volume label %q", expectedKey)
		}

		return isExpected
	}
	dockerClient.On("VolumeCreate", mock.Anything, mock.MatchedBy(expectedVolume)).
		Return(volume.Volume{}, nil).
		Once()

	err = executor.createVolumes()
	assert.NoError(t, err, "creating volumes")

	// the volume manager is expected to have some binds set up
	expectedBinds := []string{
		// expansion only in the destination
		"/host/${COMBINED_VAR}:/tmp/1-2-3",
		// var ref in the middle of the string
		"/new/cache/vol-1-2-3-foo",
		// invalid var refs are passed on (to fail later, if really invalid)
		"/${:/tmp",
		"/host:/tmp/foo/$",
	}
	assert.ElementsMatch(t, expectedBinds, executor.volumesManager.Binds())
}

func TestDockerImageWithUser(t *testing.T) {
	tests := map[string]struct {
		jobUser          spec.StringOrInt64
		runnerUser, want string
		allowedUsers     []string
		wantErr          bool
	}{
		"no allowed users, neither specified":     {},
		"no allowed users, runner user specified": {runnerUser: "baba", want: "baba"},
		"no allowed users, job user specified":    {jobUser: "baba", want: "baba"},
		"no allowed users, both specified":        {runnerUser: "baba", jobUser: "yaga", want: "baba"},

		"ok allowed users, neither specified":     {allowedUsers: []string{"baba"}},
		"ok allowed users, runner user specified": {allowedUsers: []string{"baba"}, runnerUser: "baba", want: "baba"},
		"ok allowed users, job user specified":    {allowedUsers: []string{"baba"}, jobUser: "baba", want: "baba"},
		"ok allowed users, both specified":        {allowedUsers: []string{"baba"}, runnerUser: "baba", jobUser: "yaga", want: "baba"},
		"ok allowed users, job user as variable":  {allowedUsers: []string{"baba"}, jobUser: "${TTUSER}", want: "baba"},

		"bad allowed users, runner user specified": {allowedUsers: []string{"yaga"}, runnerUser: "baba", want: "", wantErr: true},
		"bad allowed users, job user specified":    {allowedUsers: []string{"yaga"}, jobUser: "baba", want: "", wantErr: true},
		"bad allowed users, both specified":        {allowedUsers: []string{"blammo"}, runnerUser: "baba", jobUser: "yaga", want: "", wantErr: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			dockerConfig := &common.DockerConfig{
				User:         tt.runnerUser,
				AllowedUsers: tt.allowedUsers,
			}
			imageConfig := spec.Image{
				Name: "alpine",
				ExecutorOptions: spec.ImageExecutorOptions{
					Docker: spec.ImageDockerOptions{
						User: tt.jobUser,
					},
				},
			}

			cce := func(t *testing.T, config *container.Config, _ *container.HostConfig, _ *network.NetworkingConfig) {
				assert.Equal(t, tt.want, config.User)
			}

			c, e := createExecutorForTestDockerConfiguration(t, dockerConfig, cce)
			c.On("ImageInspectWithRaw", mock.Anything, mock.Anything, mock.Anything).
				Return(image.InspectResponse{ID: "123"}, []byte{}, nil).Maybe()
			c.On("ImagePullBlocking", mock.Anything, mock.Anything, mock.Anything).
				Return(nil).Maybe()
			c.On("NetworkList", mock.Anything, mock.Anything).
				Return([]network.Summary{}, nil).Maybe()
			c.On("ContainerRemove", mock.Anything, mock.Anything, mock.Anything).
				Return(nil).Maybe()
			c.On("ContainerInspect", mock.Anything, "abc").
				Return(container.InspectResponse{}, nil).Maybe()

			e.Build.Variables = append(e.Build.Variables, spec.Variable{
				Key:   "TTUSER",
				Value: tt.want,
			})

			err := e.createVolumesManager()
			require.NoError(t, err)

			err = e.createPullManager()
			require.NoError(t, err)

			cfgTor := newDefaultContainerConfigurator(e, buildContainerType, imageConfig, []string{"/bin/sh"}, []string{})
			_, err = e.createContainer(buildContainerType, imageConfig, []string{}, cfgTor)
			if !tt.wantErr {
				require.NoError(t, err)
			} else {
				require.Contains(t, err.Error(), "is not an allowed user")
			}
		})
	}
}

func TestDockerConfigGetLogConfig(t *testing.T) {
	tests := []struct {
		name           string
		logOptions     map[string]string
		expectedConfig map[string]string
		expectedError  string
	}{
		{
			name: "empty log options",
		},
		{
			name:           "with env option",
			logOptions:     map[string]string{"env": "CI_JOB_ID,CI_JOB_NAME,CI_PROJECT_ID"},
			expectedConfig: map[string]string{"env": "CI_JOB_ID,CI_JOB_NAME,CI_PROJECT_ID"},
		},
		{
			name:           "with labels and env options",
			logOptions:     map[string]string{"labels": "com.gitlab.gitlab-runner.job.id,com.gitlab.gitlab-runner.project.id", "env": "CI_JOB_ID,CI_JOB_NAME,CI_PROJECT_ID"},
			expectedConfig: map[string]string{"labels": "com.gitlab.gitlab-runner.job.id,com.gitlab.gitlab-runner.project.id", "env": "CI_JOB_ID,CI_JOB_NAME,CI_PROJECT_ID"},
		},
		{
			name:          "invalid key",
			logOptions:    map[string]string{"foo": "bar"},
			expectedError: `creating docker log configuration: invalid log options: only ["env" "labels"] are allowed, but found: ["foo"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := docker.NewMockClient(t)
			if tt.expectedError == "" {
				mockExecutorPrepareInteraction(t, c)
			}

			e := executorWithMockClient(c)
			build := &common.Build{
				Runner: &common.RunnerConfig{
					RunnerSettings: common.RunnerSettings{
						Docker: &common.DockerConfig{
							Image:      "some-image",
							LogOptions: tt.logOptions,
						},
					},
				},
			}

			err := e.Prepare(common.ExecutorPrepareOptions{
				Context:     t.Context(),
				Build:       build,
				BuildLogger: buildlogger.New(&common.Trace{Writer: io.Discard}, logrus.WithField("test", t.Name()), buildlogger.Options{}),
				Config:      build.Runner,
			})
			if tt.expectedError != "" {
				var buildErr *common.BuildError
				assert.ErrorAs(t, err, &buildErr, "expected error to be a *common.BuildError")
				assert.Equal(t, common.ConfigurationError, buildErr.FailureReason, "expected a configuration error")
				assert.Equal(t, tt.expectedError, buildErr.Error())
				return // when prepare fails, we can bail out
			} else {
				require.NoError(t, err)
			}

			hasExpectedLogConfig := func(t *testing.T, hostConfig *container.HostConfig) {
				t.Helper()
				assert.Equal(t, "json-file", hostConfig.LogConfig.Type)
				assert.Equal(t, tt.logOptions, hostConfig.LogConfig.Config)
			}

			t.Run("build container", func(t *testing.T) {
				buildContainerHostConfig, err := e.createHostConfig(true, false)
				assert.NoError(t, err, "creating build container's host config")
				hasExpectedLogConfig(t, buildContainerHostConfig)
			})

			t.Run("service container", func(t *testing.T) {
				serviceContainerHostConfig, err := e.createHostConfigForService(false, nil, nil)
				assert.NoError(t, err, "creating service container's host config")
				hasExpectedLogConfig(t, serviceContainerHostConfig)
			})
		})
	}
}

// mockExecutorPrepareInteraction mocks out interactions the executor does with the docker client, so that Prepare can
// succeed.
func mockExecutorPrepareInteraction(t *testing.T, c *docker.MockClient) {
	waitResponseCh := make(chan container.WaitResponse)
	errCh := make(chan error)
	tCtx := t.Context()

	go func() {
		for {
			select {
			case waitResponseCh <- container.WaitResponse{}: // noop, just send out
			case errCh <- nil: // noop, just send out
			case <-tCtx.Done():
				return
			}
		}
	}()

	c.EXPECT().
		ImageInspectWithRaw(mock.Anything, mock.Anything, mock.Anything).
		Return(image.InspectResponse{}, []byte{}, nil).
		Once()
	c.EXPECT().
		VolumeCreate(mock.Anything, mock.Anything).
		Return(volume.Volume{Name: ""}, nil).
		Once()
	c.EXPECT().
		ContainerCreate(mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(container.CreateResponse{}, nil).
		Once()
	c.EXPECT().
		ContainerStart(mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Once()
	c.EXPECT().
		ContainerWait(mock.Anything, mock.Anything, mock.Anything).
		Return(waitResponseCh, errCh).
		Once()
	c.EXPECT().
		ContainerRemove(mock.Anything, mock.Anything, mock.Anything).
		Return(nil).
		Once()
}

var _ executors.Environment = (*env)(nil)

type env struct {
	client *envClient
}

var _ executors.Client = &envClient{}

type envClient struct {
	dialed bool
}

func (c *envClient) Dial(n string, addr string) (net.Conn, error) {
	c.dialed = true
	return nil, assert.AnError
}

func (c *envClient) Run(ctx context.Context, options executors.RunOptions) error {
	return nil
}

func (c *envClient) DialRun(ctx context.Context, command string) (net.Conn, error) {
	c.dialed = true
	return nil, assert.AnError
}

func (c *envClient) Close() error {
	return nil
}

func (e *env) WithContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithCancel(ctx)
}

func (e *env) Prepare(
	ctx context.Context,
	logger buildlogger.Logger,
	options common.ExecutorPrepareOptions,
) (executors.Client, error) {
	e.client = &envClient{}
	return e.client, nil
}

func TestConnectEnvironment(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)

	e := &executor{
		AbstractExecutor: executors.AbstractExecutor{
			ExecutorOptions: executors.ExecutorOptions{},
		},
	}
	e.volumeParser = parser.NewLinuxParser(e.ExpandValue)

	env := &env{}

	build := &common.Build{
		Job: spec.Job{
			Image: spec.Image{
				Name: "test",
			},
		},
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Docker: &common.DockerConfig{},
			},
		},
		ExecutorData: env,
	}

	err := e.Prepare(common.ExecutorPrepareOptions{
		Config: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				BuildsDir: "/tmp",
				CacheDir:  "/tmp",
				Shell:     "bash",
				Docker:    build.Runner.Docker,
			},
		},
		Build:   build,
		Context: t.Context(),
	})
	require.ErrorIs(t, err, assert.AnError)
	require.NotNil(t, env.client)
	require.True(t, env.client.dialed)
}

func TestTooManyServicesRequestedError(t *testing.T) {
	t.Parallel()
	t.Run(".Is()", func(t *testing.T) {
		tests := map[string]struct {
			err1 tooManyServicesRequestedError
			err2 tooManyServicesRequestedError
			want bool
		}{
			"matching errors": {
				err1: tooManyServicesRequestedError{allowed: 1, requested: 2},
				err2: tooManyServicesRequestedError{allowed: 1, requested: 2},
				want: true,
			},
			"mismatching allowed field": {
				err1: tooManyServicesRequestedError{allowed: 1, requested: 2},
				err2: tooManyServicesRequestedError{allowed: 10, requested: 2},
				want: false,
			},
			"mismatching requested field": {
				err1: tooManyServicesRequestedError{allowed: 1, requested: 2},
				err2: tooManyServicesRequestedError{allowed: 1, requested: 20},
				want: false,
			},
		}

		for testName, test := range tests {
			t.Run(testName, func(t *testing.T) {
				have := test.err1.Is(&test.err2)
				assert.Equal(t, test.want, have)
			})
		}
	})
}

func Test_bootstrap(t *testing.T) {
	type testCase struct {
		setup         func(*volumes.MockManager, *docker.MockClient, *common.Build) []string
		expectedBinds []string
		wantStage     common.ExecutorStage
	}
	tests := map[string]map[string]testCase{
		"linux": {
			"native steps enabled": {
				expectedBinds: []string{"/opt/gitlab-runner"},
				wantStage:     ExecutorStageBootstrap,
				setup: func(vm *volumes.MockManager, c *docker.MockClient, b *common.Build) []string {
					binds := make([]string, 1)
					name := "blablabla"
					b.Job.Run = []schema.Step{{Name: &name}}

					c.EXPECT().ImageInspectWithRaw(mock.Anything, mock.Anything, mock.Anything).Return(image.InspectResponse{
						ID: "helper-id",
					}, nil, nil)
					c.EXPECT().ContainerCreate(mock.Anything, &container.Config{
						Image:           "helper-id",
						Cmd:             []string{"gitlab-runner-helper", "steps", "bootstrap", bootstrappedBinary},
						Tty:             false,
						AttachStdin:     false,
						AttachStdout:    true,
						AttachStderr:    true,
						OpenStdin:       false,
						StdinOnce:       true,
						NetworkDisabled: true,
					}, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(container.CreateResponse{ID: "container-id"}, nil)

					c.EXPECT().ContainerAttach(mock.Anything, "container-id", mock.Anything).Return(client.HijackedResponse{
						Reader: bufio.NewReader(strings.NewReader("")),
						Conn:   &net.UnixConn{},
					}, nil)
					c.EXPECT().ContainerRemove(mock.Anything, "container-id", mock.Anything).Return(nil)

					bodyCh := make(chan container.WaitResponse, 1)
					bodyCh <- container.WaitResponse{StatusCode: 0}
					c.EXPECT().ContainerWait(mock.Anything, "container-id", container.WaitConditionNextExit).
						Return((<-chan container.WaitResponse)(bodyCh), nil)

					c.EXPECT().ContainerStart(mock.Anything, "container-id", mock.Anything).Return(nil)

					vm.EXPECT().CreateTemporary(mock.Anything, "/opt/gitlab-runner").
						Return(nil).
						Run(func(ctx context.Context, destination string) {
							binds[0] = destination
						}).
						Once()
					vm.EXPECT().Binds().Return(binds).Once()

					return binds
				},
			},
			"native steps not enabled": {
				setup: func(vm *volumes.MockManager, c *docker.MockClient, b *common.Build) []string {
					b.Variables = append(b.Variables, spec.Variable{
						Key:   "FF_SCRIPT_TO_STEP_MIGRATION",
						Value: "false",
					})

					return nil
				},
			},
		},
		"windows": {
			"native steps enabled":     {},
			"native steps not enabled": {},
		},
	}

	for name, tt := range tests[runtime.GOOS] {
		t.Run(name, func(t *testing.T) {
			c := docker.NewMockClient(t)
			vm := volumes.NewMockManager(t)
			e := executor{
				volumesManager: vm,
				dockerConn:     &dockerConnection{Client: c},
				AbstractExecutor: executors.AbstractExecutor{
					Context: t.Context(),
					Build: &common.Build{
						ExecutorFeatures: common.FeaturesInfo{
							NativeStepsIntegration: true,
						},
					},
					Config: common.RunnerConfig{
						RunnerSettings: common.RunnerSettings{
							Docker: &common.DockerConfig{},
						},
					},
				},
			}

			var binds []string
			if tt.setup != nil {
				binds = tt.setup(vm, c, e.Build)
			}

			assert.NoError(t, e.bootstrap())
			assert.Equal(t, tt.expectedBinds, binds)
			assert.Equal(t, tt.wantStage, e.GetCurrentStage())
		})
	}
}

// TestDockerSlotCgroupSettings verifies that slot-based cgroup settings
// are actually applied to container HostConfig when creating containers
func TestDockerSlotCgroupSettings(t *testing.T) {
	t.Run("Build container with slot cgroups enabled", func(t *testing.T) {
		runnerConfig := &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				UseSlotCgroups:     true,
				SlotCgroupTemplate: "runner/slot-${slot}",
				Docker: &common.DockerConfig{
					CgroupParent: "should-not-use-this",
				},
			},
		}

		// Verify HostConfig.CgroupParent is set to slot-based value
		cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
			assert.Equal(t, "runner/slot-5", hostConfig.CgroupParent, "HostConfig.CgroupParent should be set to slot-based value")
		}

		testDockerConfigurationWithSlotCgroups(t, runnerConfig, &mockAutoscalerExecutorData{slot: 5}, cce)
	})

	t.Run("Build container with slot cgroups enabled using default template", func(t *testing.T) {
		runnerConfig := &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				UseSlotCgroups: true,
				Docker: &common.DockerConfig{
					CgroupParent: "fallback-cgroup",
				},
			},
		}

		cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
			assert.Equal(t, "gitlab-runner/slot-10", hostConfig.CgroupParent, "HostConfig.CgroupParent should use default template")
		}

		testDockerConfigurationWithSlotCgroups(t, runnerConfig, &mockAutoscalerExecutorData{slot: 10}, cce)
	})

	t.Run("Build container with slot cgroups disabled", func(t *testing.T) {
		runnerConfig := &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				UseSlotCgroups: false,
				Docker: &common.DockerConfig{
					CgroupParent: "static-build-cgroup",
				},
			},
		}

		cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
			assert.Equal(t, "static-build-cgroup", hostConfig.CgroupParent, "HostConfig.CgroupParent should use static value when slot cgroups disabled")
		}

		testDockerConfigurationWithSlotCgroups(t, runnerConfig, &mockAutoscalerExecutorData{slot: 5}, cce)
	})

	t.Run("Build container with slot cgroups enabled but no slot available", func(t *testing.T) {
		runnerConfig := &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				UseSlotCgroups: true,
				Docker: &common.DockerConfig{
					CgroupParent: "fallback-build-cgroup",
				},
			},
		}

		cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
			assert.Equal(t, "fallback-build-cgroup", hostConfig.CgroupParent, "HostConfig.CgroupParent should fallback when no slot available")
		}

		testDockerConfigurationWithSlotCgroups(t, runnerConfig, nil, cce)
	})

	t.Run("Service container with slot cgroups enabled", func(t *testing.T) {
		runnerConfig := &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				UseSlotCgroups: true,
				Docker: &common.DockerConfig{
					ServiceCgroupParent:       "should-not-use-this",
					ServiceSlotCgroupTemplate: "runner/service-${slot}",
				},
			},
		}

		testDockerServiceContainerCgroup(t, runnerConfig, &mockAutoscalerExecutorData{slot: 7}, "runner/service-7")
	})

	t.Run("Service container with slot cgroups enabled using default template", func(t *testing.T) {
		runnerConfig := &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				UseSlotCgroups: true,
				Docker: &common.DockerConfig{
					ServiceCgroupParent: "fallback-service",
				},
			},
		}

		testDockerServiceContainerCgroup(t, runnerConfig, &mockAutoscalerExecutorData{slot: 3}, "gitlab-runner/slot-3")
	})

	t.Run("Service container with slot cgroups disabled", func(t *testing.T) {
		runnerConfig := &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				UseSlotCgroups: false,
				Docker: &common.DockerConfig{
					ServiceCgroupParent: "static-service-cgroup",
				},
			},
		}

		testDockerServiceContainerCgroup(t, runnerConfig, &mockAutoscalerExecutorData{slot: 5}, "static-service-cgroup")
	})

	t.Run("Service container with slot cgroups enabled but no slot available", func(t *testing.T) {
		runnerConfig := &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				UseSlotCgroups: true,
				Docker: &common.DockerConfig{
					ServiceCgroupParent: "fallback-service-cgroup",
				},
			},
		}

		testDockerServiceContainerCgroup(t, runnerConfig, nil, "fallback-service-cgroup")
	})
}

// Mock ExecutorData for testing slot functionality
type mockAutoscalerExecutorData struct {
	slot int
}

func (m *mockAutoscalerExecutorData) AcquisitionSlot() int {
	return m.slot
}

// testDockerConfigurationWithSlotCgroups tests that build containers are created with slot-based cgroups
func testDockerConfigurationWithSlotCgroups(
	t *testing.T,
	runnerConfig *common.RunnerConfig,
	executorData interface{},
	cce containerConfigExpectations,
) {
	c, e := prepareTestDockerConfiguration(t, runnerConfig.Docker, cce, "alpine", "alpine:latest")
	c.On("ContainerInspect", mock.Anything, "abc").
		Return(container.InspectResponse{}, nil).Once()

	// Set the executor data for slot testing
	e.Build.ExecutorData = executorData
	// Set the runner config for slot testing
	e.Config = *runnerConfig

	err := e.createVolumesManager()
	require.NoError(t, err)

	err = e.createPullManager()
	require.NoError(t, err)

	imageConfig := spec.Image{Name: "alpine"}
	cfgTor := newDefaultContainerConfigurator(e, buildContainerType, imageConfig, []string{"/bin/sh"}, []string{})
	_, err = e.createContainer(buildContainerType, imageConfig, []string{}, cfgTor)
	assert.NoError(t, err, "Should create container without errors")
}

// testDockerServiceContainerCgroup tests that service containers are created with the expected cgroup parent
func testDockerServiceContainerCgroup(
	t *testing.T,
	runnerConfig *common.RunnerConfig,
	executorData interface{},
	expectedCgroup string,
) {
	// Create mock docker client
	c := docker.NewMockClient(t)

	// Create mock volumes manager
	vm := volumes.NewMockManager(t)
	vm.On("Binds").Return([]string{})

	e := new(executor)
	e.dockerConn = &dockerConnection{Client: c}
	e.Config = *runnerConfig
	e.Build = &common.Build{
		ExecutorData: executorData,
	}
	e.volumesManager = vm

	// Call createHostConfigForService and verify the cgroup is set correctly
	hostConfig, err := e.createHostConfigForService(false, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, expectedCgroup, hostConfig.CgroupParent, "Service container HostConfig.CgroupParent should be set correctly")
}

func TestPrepareContainerEnvVariables(t *testing.T) {
	test.SkipIfGitLabCIOn(t, test.OSWindows)

	tests := map[string]struct {
		featureFlagEnabled       bool
		jobVariables             spec.Variables
		expectedVarNames         []string
		shouldHaveRunnerVarNames bool
	}{
		"feature flag disabled returns variables  unchanged": {
			featureFlagEnabled: false,
			jobVariables: spec.Variables{
				{Key: "VAR1", Value: "value1"},
				{Key: "VAR2", Value: "value2"},
			},
			shouldHaveRunnerVarNames: false,
		},
		"feature flag enabled compresses variable names": {
			featureFlagEnabled: true,
			jobVariables: spec.Variables{
				{Key: "VAR1", Value: "value1"},
				{Key: "VAR2", Value: "value2"},
				{Key: "VAR3", Value: "value3"},
			},
			expectedVarNames:         []string{"VAR1", "VAR2", "VAR3"},
			shouldHaveRunnerVarNames: true,
		},
		"feature flag enabled with empty variables": {
			featureFlagEnabled:       true,
			jobVariables:             spec.Variables{},
			shouldHaveRunnerVarNames: true,
		},
		"feature flag enabled with many variables": {
			featureFlagEnabled: true,
			jobVariables: spec.Variables{
				{Key: "LONG_VARIABLE_NAME_1", Value: "value1"},
				{Key: "LONG_VARIABLE_NAME_2", Value: "value2"},
				{Key: "LONG_VARIABLE_NAME_3", Value: "value3"},
			},
			expectedVarNames:         []string{"LONG_VARIABLE_NAME_1", "LONG_VARIABLE_NAME_2", "LONG_VARIABLE_NAME_3"},
			shouldHaveRunnerVarNames: true,
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			e := &executor{
				AbstractExecutor: executors.AbstractExecutor{
					Build: &common.Build{
						Job: spec.Job{
							Variables: test.jobVariables,
						},
					},
				},
			}

			// Set the feature flag
			if test.featureFlagEnabled {
				e.Build.ExecutorFeatures.NativeStepsIntegration = test.featureFlagEnabled
				e.Build.Variables = append(e.Build.Variables, spec.Variable{
					Key:   featureflags.UseScriptToStepMigration,
					Value: "true",
				})
			}

			result, err := e.prepareContainerEnvVariables()

			require.NoError(t, err)
			require.NotNil(t, result)

			require.Equal(t, test.shouldHaveRunnerVarNames, checkVariable(result, runnerJobVarsNames))
		})
	}
}

func checkVariable(vars spec.Variables, key string) bool {
	for i := range vars {
		if vars[i].Key == key {
			return true
		}
	}
	return false
}

func TestRemoveContainerVolumeKeep(t *testing.T) {
	tests := []struct {
		name                  string
		volumeKeep            bool
		expectedRemoveVolumes bool
	}{
		{
			name:                  "VolumeKeep=false removes volumes",
			volumeKeep:            false,
			expectedRemoveVolumes: true,
		},
		{
			name:                  "VolumeKeep=true preserves volumes",
			volumeKeep:            true,
			expectedRemoveVolumes: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := docker.NewMockClient(t)

			e := &executor{}
			e.dockerConn = &dockerConnection{Client: c}
			e.Config.Docker = &common.DockerConfig{VolumeKeep: tc.volumeKeep}
			e.BuildLogger = buildlogger.New(nil, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})

			c.On("NetworkList", mock.Anything, mock.Anything).
				Return([]network.Summary{}, nil).Once()

			expectedOptions := client.ContainerRemoveOptions{
				RemoveVolumes: tc.expectedRemoveVolumes,
				Force:         true,
			}
			c.On("ContainerRemove", mock.Anything, "test-container-id", expectedOptions).
				Return(nil).Once()

			err := e.removeContainer(t.Context(), "test-container-id")
			assert.NoError(t, err)
		})
	}
}

func TestProcessSecurityOpt(t *testing.T) {
	// Create a temporary seccomp profile file
	seccompProfile := `{"defaultAction":"SCMP_ACT_ERRNO"}`
	seccompProfilePath := filepath.Join(t.TempDir(), "seccomp-profile.json")
	require.NoError(t, os.WriteFile(seccompProfilePath, []byte(seccompProfile), 0644))

	tests := map[string]struct {
		securityOpts  []string
		expectedOpts  []string
		expectedError string
	}{
		"empty security opts": {
			securityOpts: []string{},
			expectedOpts: []string{},
		},
		"nil security opts": {
			securityOpts: nil,
			expectedOpts: nil,
		},
		"non-seccomp options pass through": {
			securityOpts: []string{"apparmor=unconfined", "no-new-privileges"},
			expectedOpts: []string{"apparmor=unconfined", "no-new-privileges"},
		},
		"seccomp=unconfined passes through": {
			securityOpts: []string{"seccomp=unconfined"},
			expectedOpts: []string{"seccomp=unconfined"},
		},
		"seccomp=builtin passes through": {
			securityOpts: []string{"seccomp=builtin"},
			expectedOpts: []string{"seccomp=builtin"},
		},
		"bare seccomp without value passes through": {
			securityOpts: []string{"seccomp"},
			expectedOpts: []string{"seccomp"},
		},
		"inline seccomp JSON passes through": {
			securityOpts: []string{`seccomp={"defaultAction":"SCMP_ACT_ERRNO"}`},
			expectedOpts: []string{`seccomp={"defaultAction":"SCMP_ACT_ERRNO"}`},
		},
		"seccomp profile path is loaded": {
			securityOpts: []string{fmt.Sprintf("seccomp=%s", seccompProfilePath)},
			expectedOpts: []string{fmt.Sprintf("seccomp=%s", seccompProfile)},
		},
		"mixed security options": {
			securityOpts: []string{
				"apparmor=unconfined",
				fmt.Sprintf("seccomp=%s", seccompProfilePath),
				"no-new-privileges",
			},
			expectedOpts: []string{
				"apparmor=unconfined",
				fmt.Sprintf("seccomp=%s", seccompProfile),
				"no-new-privileges",
			},
		},
		"non-existent file returns error": {
			securityOpts:  []string{"seccomp=/nonexistent/profile.json"},
			expectedError: "failed to read seccomp profile from /nonexistent/profile.json",
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			logger, _ := logrustest.NewNullLogger()
			e := &executor{
				AbstractExecutor: executors.AbstractExecutor{
					BuildLogger: buildlogger.New(nil, logger.WithField("test", t.Name()), buildlogger.Options{}),
				},
			}

			result, err := e.processSecurityOpt(tt.securityOpts)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expectedOpts, result)
		})
	}
}

func TestDockerSecurityOptSetting(t *testing.T) {
	// Create a temporary seccomp profile file
	seccompProfile := `{"defaultAction":"SCMP_ACT_ERRNO"}`
	seccompProfilePath := filepath.Join(t.TempDir(), "seccomp-profile.json")
	require.NoError(t, os.WriteFile(seccompProfilePath, []byte(seccompProfile), 0644))

	dockerConfig := &common.DockerConfig{
		SecurityOpt: []string{
			fmt.Sprintf("seccomp=%s", seccompProfilePath),
			"apparmor=unconfined",
		},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		expected := []string{
			fmt.Sprintf("seccomp=%s", seccompProfile),
			"apparmor=unconfined",
		}
		assert.Equal(t, expected, hostConfig.SecurityOpt)
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
}

func TestDockerServicesSecurityOptSetting(t *testing.T) {
	// Create a temporary seccomp profile file
	seccompProfile := `{"defaultAction":"SCMP_ACT_ERRNO"}`
	seccompProfilePath := filepath.Join(t.TempDir(), "seccomp-profile.json")
	require.NoError(t, os.WriteFile(seccompProfilePath, []byte(seccompProfile), 0644))

	dockerConfig := &common.DockerConfig{
		ServicesSecurityOpt: []string{
			fmt.Sprintf("seccomp=%s", seccompProfilePath),
			"apparmor=unconfined",
		},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig, _ *network.NetworkingConfig) {
		expected := []string{
			fmt.Sprintf("seccomp=%s", seccompProfile),
			"apparmor=unconfined",
		}
		assert.Equal(t, expected, hostConfig.SecurityOpt)
	}

	testDockerConfigurationWithServiceContainer(t, dockerConfig, cce)
}

func TestSuspend_noBuildContainer(t *testing.T) {
	c := docker.NewMockClient(t)
	s := &commandExecutor{executor: *executorWithMockClient(c)}

	fields, err := s.Suspend(t.Context())

	require.EqualError(t, err, "no build container to suspend")
	assert.Nil(t, fields)
	assert.False(t, s.suspended)
}

func TestSuspend_noHelperContainer(t *testing.T) {
	c := docker.NewMockClient(t)
	s := &commandExecutor{executor: *executorWithMockClient(c)}
	s.buildContainerID = "build-cid"

	fields, err := s.Suspend(t.Context())

	require.EqualError(t, err, "no helper container to suspend")
	assert.Nil(t, fields)
	assert.False(t, s.suspended)
}

func TestSuspend(t *testing.T) {
	c := docker.NewMockClient(t)
	s := &commandExecutor{executor: *executorWithMockClient(c)}
	require.NoError(t, s.dockerConnector.Connect(t.Context(), common.ExecutorPrepareOptions{}, &s.executor))
	s.buildContainerID = "build-cid"
	s.helperContainer = &container.InspectResponse{
		ID: "helper-cid",
	}
	s.services = []*serviceInfo{
		{ID: "svc-a"},
		{ID: "svc-b"},
	}

	c.On("ContainerStop", mock.Anything, "build-cid", client.ContainerStopOptions{}).
		Return(nil).Once()
	c.On("ContainerStop", mock.Anything, "helper-cid", client.ContainerStopOptions{}).
		Return(nil).Once()
	c.On("ContainerStop", mock.Anything, "svc-a", client.ContainerStopOptions{}).
		Return(nil).Once()
	c.On("ContainerStop", mock.Anything, "svc-b", client.ContainerStopOptions{}).
		Return(nil).Once()

	fields, err := s.Suspend(t.Context())
	require.NoError(t, err)

	parsed, err := parseEnvKeyFields(fields)
	require.NoError(t, err)
	assert.Equal(t, "build-cid", parsed.buildContainerID)
	assert.Equal(t, "helper-cid", parsed.helperContainerID)
	assert.Equal(t, []string{"svc-a", "svc-b"}, parsed.serviceContainerIDs)
	assert.True(t, s.suspended)
}

func TestSuspend_containerStopFails(t *testing.T) {
	c := docker.NewMockClient(t)
	s := &commandExecutor{executor: *executorWithMockClient(c)}
	require.NoError(t, s.dockerConnector.Connect(t.Context(), common.ExecutorPrepareOptions{}, &s.executor))
	s.buildContainerID = "build-cid"
	s.helperContainer = &container.InspectResponse{
		ID: "helper-cid",
	}

	c.On("ContainerStop", mock.Anything, "build-cid", client.ContainerStopOptions{}).
		Return(errors.New("dockerd boom")).Once()
	c.On("ContainerStop", mock.Anything, "helper-cid", client.ContainerStopOptions{}).
		Return(nil).Maybe()

	fields, err := s.Suspend(t.Context())

	assert.Error(t, err)
	assert.Nil(t, fields)
	assert.False(t, s.suspended)
}

func TestCleanup_skipsRemovalWhenSuspended(t *testing.T) {
	c := docker.NewMockClient(t)
	e := executorWithMockClient(c)
	err := e.dockerConnector.Connect(t.Context(), common.ExecutorPrepareOptions{}, e)
	require.NoError(t, err, "connecting mock client")
	e.suspended = true
	e.buildContainerID = "build-cid"
	e.services = []*serviceInfo{{ID: "svc-a"}, {ID: "svc-b"}}
	e.temporary = []string{"build-cid", "svc-a", "svc-b", "helper-cid"}
	e.Config = common.RunnerConfig{}
	e.Config.Docker = &common.DockerConfig{}

	c.On("Close").Return(nil).Maybe()

	e.Cleanup()

	// No ContainerRemove / NetworkRemove / VolumeRemove expectations are set.
	// Test will fail if any of them fires.
}

func TestResumeDependencies(t *testing.T) {
	c := docker.NewMockClient(t)
	e := executorWithMockClient(c)
	require.NoError(t, e.dockerConnector.Connect(t.Context(), common.ExecutorPrepareOptions{}, e))
	e.Config = common.RunnerConfig{}
	e.Config.Docker = &common.DockerConfig{WaitForServicesTimeout: -1}
	e.Build = &common.Build{
		Runner: &common.RunnerConfig{
			RunnerCredentials: common.RunnerCredentials{Token: "test-token"},
		},
	}
	e.Build.Variables = append(e.Build.Variables,
		spec.Variable{Key: featureflags.SuspendableEnvironments, Value: "true"},
	)
	e.Build.Job.SuspendOptions.EnvironmentKey =
		"1/system-id/build-container-id=build-cid&helper-id=helper-cid&service-ids=svc-a"

	buildInspect := container.InspectResponse{
		ID:         "build-cid",
		HostConfig: &container.HostConfig{NetworkMode: container.NetworkMode(network.NetworkDefault)},
		Mounts: []container.MountPoint{
			{Type: mount.TypeVolume, Name: "vol-x", Source: "/var/lib/docker/volumes/vol-x/_data", Destination: "/builds"},
		},
	}
	svcInspect := container.InspectResponse{
		ID:    "svc-a",
		Name:  "/svc-a",
		State: &container.State{Status: "running"},
		NetworkSettings: &container.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{
				"bridge": {IPAddress: netip.MustParseAddr("172.17.0.4")},
			},
		},
		Config: &container.Config{
			ExposedPorts: network.PortSet{network.MustParsePort("80/tcp"): {}},
		},
	}

	c.On("ContainerInspect", mock.Anything, "build-cid").Return(buildInspect, nil).Once()
	c.On("ContainerInspect", mock.Anything, "helper-cid").Return(container.InspectResponse{
		ID: "helper-cid",
	}, nil).Once()
	c.On("VolumeInspect", mock.Anything, "vol-x").Return(volume.Volume{Name: "vol-x"}, nil).Once()
	c.On("ContainerInspect", mock.Anything, "svc-a").Return(svcInspect, nil)
	c.On("ContainerStart", mock.Anything, "svc-a", client.ContainerStartOptions{}).Return(nil).Once()

	require.NoError(t, e.resumeDependencies())
	assert.Equal(t, "build-cid", e.buildContainerID)
	require.NotNil(t, e.buildContainer)
	assert.Equal(t, "build-cid", e.buildContainer.ID)
	require.NotNil(t, e.helperContainer)
	assert.Equal(t, "helper-cid", e.helperContainer.ID)
	assert.Contains(t, e.temporary, "build-cid")
	assert.Contains(t, e.temporary, "svc-a")
	assert.Contains(t, e.temporary, "helper-cid")
	assert.Len(t, e.services, 1)
	assert.Equal(t, "svc-a", e.services[0].ID)
	assert.Equal(t, []string{"172.17.0.4"}, e.services[0].IP)
	assert.Equal(t, []int{80}, e.services[0].Ports)
}

func TestResumeDependencies_buildContainerNotFound(t *testing.T) {
	c := docker.NewMockClient(t)
	e := executorWithMockClient(c)
	require.NoError(t, e.dockerConnector.Connect(t.Context(), common.ExecutorPrepareOptions{}, e))
	e.Config = common.RunnerConfig{}
	e.Config.Docker = &common.DockerConfig{}
	e.Build = &common.Build{Runner: &common.RunnerConfig{}}
	e.Build.Variables = append(e.Build.Variables,
		spec.Variable{Key: featureflags.SuspendableEnvironments, Value: "true"},
	)
	e.Build.Job.SuspendOptions.EnvironmentKey =
		"1/system-id/build-container-id=missing-cid&helper-id=helper-cid"

	c.On("ContainerInspect", mock.Anything, "missing-cid").
		Return(container.InspectResponse{}, errdefs.ErrNotFound).Once()

	err := e.resumeDependencies()
	require.EqualError(t, err, "build container missing-cid not found: not found")
}

func TestResumeDependencies_helperNotFound(t *testing.T) {
	c := docker.NewMockClient(t)
	e := executorWithMockClient(c)
	require.NoError(t, e.dockerConnector.Connect(t.Context(), common.ExecutorPrepareOptions{}, e))
	e.Config = common.RunnerConfig{}
	e.Config.Docker = &common.DockerConfig{}
	e.Build = &common.Build{Runner: &common.RunnerConfig{}}
	e.Build.Variables = append(e.Build.Variables,
		spec.Variable{Key: featureflags.SuspendableEnvironments, Value: "true"},
	)
	e.Build.Job.SuspendOptions.EnvironmentKey =
		"1/system-id/build-container-id=build-cid&helper-id=missing-helper"

	buildInspect := container.InspectResponse{
		ID:         "build-cid",
		HostConfig: &container.HostConfig{NetworkMode: container.NetworkMode(network.NetworkDefault)},
	}
	c.On("ContainerInspect", mock.Anything, "build-cid").Return(buildInspect, nil).Once()
	c.On("ContainerInspect", mock.Anything, "missing-helper").
		Return(container.InspectResponse{}, errdefs.ErrNotFound).Once()

	err := e.resumeDependencies()
	require.EqualError(t, err, "helper container missing-helper not found: not found")
}

func TestResumeDependencies_serviceNotFound(t *testing.T) {
	c := docker.NewMockClient(t)
	e := executorWithMockClient(c)
	require.NoError(t, e.dockerConnector.Connect(t.Context(), common.ExecutorPrepareOptions{}, e))
	e.Config = common.RunnerConfig{}
	e.Config.Docker = &common.DockerConfig{}
	e.Build = &common.Build{Runner: &common.RunnerConfig{}}
	e.Build.Variables = append(e.Build.Variables,
		spec.Variable{Key: featureflags.SuspendableEnvironments, Value: "true"},
	)
	e.Build.Job.SuspendOptions.EnvironmentKey =
		"1/system-id/build-container-id=build-cid&helper-id=helper-cid&service-ids=svc-missing"

	buildInspect := container.InspectResponse{
		ID:         "build-cid",
		HostConfig: &container.HostConfig{NetworkMode: container.NetworkMode(network.NetworkDefault)},
	}
	c.On("ContainerInspect", mock.Anything, "build-cid").Return(buildInspect, nil).Once()
	c.On("ContainerInspect", mock.Anything, "helper-cid").Return(container.InspectResponse{
		ID: "helper-cid",
	}, nil).Once()
	c.On("ContainerInspect", mock.Anything, "svc-missing").
		Return(container.InspectResponse{}, errdefs.ErrNotFound).Once()

	err := e.resumeDependencies()
	require.EqualError(t, err, "service container svc-missing not found: not found")
}

func TestResumeDependencies_serviceStartFails(t *testing.T) {
	c := docker.NewMockClient(t)
	e := executorWithMockClient(c)
	require.NoError(t, e.dockerConnector.Connect(t.Context(), common.ExecutorPrepareOptions{}, e))
	e.Config = common.RunnerConfig{}
	e.Config.Docker = &common.DockerConfig{}
	e.Build = &common.Build{Runner: &common.RunnerConfig{}}
	e.Build.Variables = append(e.Build.Variables,
		spec.Variable{Key: featureflags.SuspendableEnvironments, Value: "true"},
	)
	e.Build.Job.SuspendOptions.EnvironmentKey =
		"1/system-id/build-container-id=build-cid&helper-id=helper-cid&service-ids=svc-broken"

	buildInspect := container.InspectResponse{
		ID:         "build-cid",
		HostConfig: &container.HostConfig{NetworkMode: container.NetworkMode(network.NetworkDefault)},
	}
	c.On("ContainerInspect", mock.Anything, "build-cid").Return(buildInspect, nil).Once()
	c.On("ContainerInspect", mock.Anything, "helper-cid").Return(container.InspectResponse{
		ID: "helper-cid",
	}, nil).Once()
	c.On("ContainerInspect", mock.Anything, "svc-broken").
		Return(container.InspectResponse{
			ID: "svc-broken",
		}, nil).Once()
	c.On("ContainerStart", mock.Anything, "svc-broken", client.ContainerStartOptions{}).
		Return(errors.New("daemon error: cannot start")).Once()

	err := e.resumeDependencies()
	require.EqualError(t, err, "service container svc-broken failed to start: daemon error: cannot start")
}

func TestDisconnectNetwork(t *testing.T) {
	tests := map[string]struct {
		containerID       string
		networkContainers map[string]network.EndpointResource
		expectDisconnect  bool
	}{
		"match by container ID": {
			containerID: "abc123",
			networkContainers: map[string]network.EndpointResource{
				"abc123": {Name: "my-container"},
			},
			expectDisconnect: true,
		},
		"match by container name": {
			containerID: "my-container",
			networkContainers: map[string]network.EndpointResource{
				"abc123": {Name: "my-container"},
			},
			expectDisconnect: true,
		},
		"no match": {
			containerID: "xyz789",
			networkContainers: map[string]network.EndpointResource{
				"abc123": {Name: "my-container"},
			},
			expectDisconnect: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := docker.NewMockClient(t)
			e := executorWithMockClient(c)
			require.NoError(t, e.dockerConnector.Connect(t.Context(), common.ExecutorPrepareOptions{}, e))
			e.BuildLogger = buildlogger.New(nil, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})

			netList := []network.Summary{
				{Network: network.Network{ID: "net-123", Name: "test-network"}},
			}

			c.On("NetworkList", mock.Anything, client.NetworkListOptions{}).
				Return(netList, nil).Once()
			c.On("NetworkInspect", mock.Anything, "net-123").
				Return(network.Inspect{
					Network:    network.Network{ID: "net-123", Name: "test-network"},
					Containers: tc.networkContainers,
				}, nil).Once()

			if tc.expectDisconnect {
				c.On("NetworkDisconnect", mock.Anything, "net-123", tc.containerID, true).
					Return(nil).Once()
			}

			e.disconnectNetwork(t.Context(), tc.containerID)
		})
	}
}

func TestDisconnectNetwork_ListError(t *testing.T) {
	c := docker.NewMockClient(t)
	e := executorWithMockClient(c)
	require.NoError(t, e.dockerConnector.Connect(t.Context(), common.ExecutorPrepareOptions{}, e))
	e.BuildLogger = buildlogger.New(nil, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})

	c.On("NetworkList", mock.Anything, client.NetworkListOptions{}).
		Return(nil, errors.New("network list failed")).Once()

	e.disconnectNetwork(t.Context(), "any-container")
}

func TestDisconnectNetwork_DisconnectError(t *testing.T) {
	c := docker.NewMockClient(t)
	e := executorWithMockClient(c)
	require.NoError(t, e.dockerConnector.Connect(t.Context(), common.ExecutorPrepareOptions{}, e))
	e.BuildLogger = buildlogger.New(nil, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})

	netList := []network.Summary{
		{Network: network.Network{ID: "net-123", Name: "test-network"}},
	}

	c.On("NetworkList", mock.Anything, client.NetworkListOptions{}).
		Return(netList, nil).Once()
	c.On("NetworkInspect", mock.Anything, "net-123").
		Return(network.Inspect{
			Network: network.Network{ID: "net-123", Name: "test-network"},
			Containers: map[string]network.EndpointResource{
				"abc123": {Name: "my-container"},
			},
		}, nil).Once()
	c.On("NetworkDisconnect", mock.Anything, "net-123", "abc123", true).
		Return(errors.New("disconnect failed")).Once()

	e.disconnectNetwork(t.Context(), "abc123")
}

func TestDNSServerAddrs(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		addrs, err := dnsServerAddrs([]string{"2001:db8::1", "192.0.2.1"})
		require.NoError(t, err)
		assert.Equal(t, []netip.Addr{
			netip.MustParseAddr("2001:db8::1"),
			netip.MustParseAddr("192.0.2.1"),
		}, addrs)
	})

	t.Run("empty", func(t *testing.T) {
		addrs, err := dnsServerAddrs(nil)
		require.NoError(t, err)
		assert.Nil(t, addrs)
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := dnsServerAddrs([]string{"192.0.2.1", "not-an-ip"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid DNS server address")
	})
}

func TestParseMACAddress(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		mac, err := parseMACAddress("92:d0:c6:0a:29:33")
		require.NoError(t, err)
		assert.Equal(t, "92:d0:c6:0a:29:33", mac.String())
	})

	t.Run("empty", func(t *testing.T) {
		mac, err := parseMACAddress("")
		require.NoError(t, err)
		assert.Nil(t, mac)
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := parseMACAddress("not-a-mac")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid MAC address")
	})
}

func TestNetworkConfigLegacyMacAddressWarning(t *testing.T) {
	newExecutorWithMacAddress := func(t *testing.T, apiVersion, macAddress string) (*executor, *bytes.Buffer) {
		buffer := new(bytes.Buffer)
		logger, _ := logrustest.NewNullLogger()

		e := &executor{}
		e.serverAPIVersion = version.Must(version.NewVersion(apiVersion))
		e.AbstractExecutor.BuildLogger = buildlogger.New(
			&common.Trace{Writer: buffer}, logger.WithField("test", t.Name()), buildlogger.Options{},
		)
		e.Config.Docker = &common.DockerConfig{MacAddress: macAddress}

		return e, buffer
	}

	t.Run("pre-1.44 with mac_address configured warns", func(t *testing.T) {
		e, buffer := newExecutorWithMacAddress(t, "1.43", "92:d0:c6:0a:29:33")

		_, err := e.networkConfig(nil)
		require.NoError(t, err)
		assert.Contains(t, buffer.String(), "mac_address is configured")
	})

	t.Run("pre-1.44 without mac_address does not warn", func(t *testing.T) {
		e, buffer := newExecutorWithMacAddress(t, "1.43", "")

		_, err := e.networkConfig(nil)
		require.NoError(t, err)
		assert.NotContains(t, buffer.String(), "mac_address is configured")
	})

	t.Run("1.44 and later with mac_address does not warn since it is applied", func(t *testing.T) {
		e, buffer := newExecutorWithMacAddress(t, "1.44", "92:d0:c6:0a:29:33")

		_, err := e.networkConfig(nil)
		require.NoError(t, err)
		assert.NotContains(t, buffer.String(), "mac_address is configured")
	})
}

func TestIsContainerNotRunning(t *testing.T) {
	tests := map[string]struct {
		err  error
		want bool
	}{
		"nil":               {err: nil, want: false},
		"unrelated":         {err: errors.New("some other failure"), want: false},
		"docker daemon msg": {err: errors.New("Error response from daemon: can only create exec sessions on running containers: container state improper"), want: true},
		"podman compat msg": {err: errors.New("can only create exec sessions on running containers"), want: true},
		"wrapped":           {err: fmt.Errorf("exec failed: %w", errors.New("can only create exec sessions on running containers: container state improper")), want: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, isContainerNotRunning(tc.err))
		})
	}
}

func TestShouldIgnoreDockerError(t *testing.T) {
	preds := []func(error) bool{errdefs.IsConflict, errdefs.IsNotFound, isContainerNotRunning}

	tests := map[string]struct {
		err  error
		want bool
	}{
		"nil":                              {err: nil, want: true},
		"unrelated daemon error":           {err: errors.New("daemon: kaboom"), want: false},
		"docker conflict (errdefs typed)":  {err: fmt.Errorf("wrap: %w", errdefs.ErrConflict), want: true},
		"docker not-found (errdefs typed)": {err: fmt.Errorf("wrap: %w", errdefs.ErrNotFound), want: true},
		"podman 500 not-running":           {err: errors.New("Error response from daemon: can only create exec sessions on running containers"), want: true},
		"podman 500 not-running wrapped":   {err: fmt.Errorf("exec create: %w", errors.New("can only create exec sessions on running containers: container state improper")), want: true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, shouldIgnoreDockerError(tc.err, preds...))
		})
	}
}
