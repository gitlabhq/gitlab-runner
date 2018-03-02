package docker

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"

	"golang.org/x/net/context"
)

// ImagePullOptions contains the RegistryAuth which is inferred from the docker
// configuration for the user, so just mock it out here.
func buildImagePullOptions(e executor, configName string) mock.AnythingOfTypeArgument {
	return mock.AnythingOfType("ImagePullOptions")
}

func TestParseDeviceStringOne(t *testing.T) {
	e := executor{}

	device, err := e.parseDeviceString("/dev/kvm")

	assert.NoError(t, err)
	assert.Equal(t, "/dev/kvm", device.PathOnHost)
	assert.Equal(t, "/dev/kvm", device.PathInContainer)
	assert.Equal(t, "rwm", device.CgroupPermissions)
}

func TestParseDeviceStringTwo(t *testing.T) {
	e := executor{}

	device, err := e.parseDeviceString("/dev/kvm:/devices/kvm")

	assert.NoError(t, err)
	assert.Equal(t, "/dev/kvm", device.PathOnHost)
	assert.Equal(t, "/devices/kvm", device.PathInContainer)
	assert.Equal(t, "rwm", device.CgroupPermissions)
}

func TestParseDeviceStringThree(t *testing.T) {
	e := executor{}

	device, err := e.parseDeviceString("/dev/kvm:/devices/kvm:r")

	assert.NoError(t, err)
	assert.Equal(t, "/dev/kvm", device.PathOnHost)
	assert.Equal(t, "/devices/kvm", device.PathInContainer)
	assert.Equal(t, "r", device.CgroupPermissions)
}

func TestParseDeviceStringFour(t *testing.T) {
	e := executor{}

	_, err := e.parseDeviceString("/dev/kvm:/devices/kvm:r:oops")

	assert.Error(t, err)
}

type testAllowedImageDescription struct {
	allowed        bool
	image          string
	allowed_images []string
}

var testAllowedImages = []testAllowedImageDescription{
	{true, "ruby", []string{"*"}},
	{true, "ruby:2.1", []string{"*"}},
	{true, "ruby:latest", []string{"*"}},
	{true, "library/ruby", []string{"*/*"}},
	{true, "library/ruby:2.1", []string{"*/*"}},
	{true, "library/ruby:2.1", []string{"*/*:*"}},
	{true, "my.registry.tld/library/ruby", []string{"my.registry.tld/*/*"}},
	{true, "my.registry.tld/library/ruby:2.1", []string{"my.registry.tld/*/*:*"}},
	{true, "my.registry.tld/group/subgroup/ruby", []string{"my.registry.tld/*/*/*"}},
	{true, "my.registry.tld/group/subgroup/ruby:2.1", []string{"my.registry.tld/*/*/*:*"}},
	{true, "ruby", []string{"**/*"}},
	{true, "ruby:2.1", []string{"**/*"}},
	{true, "ruby:latest", []string{"**/*"}},
	{true, "library/ruby", []string{"**/*"}},
	{true, "library/ruby:2.1", []string{"**/*"}},
	{true, "library/ruby:2.1", []string{"**/*:*"}},
	{true, "my.registry.tld/library/ruby", []string{"my.registry.tld/**/*"}},
	{true, "my.registry.tld/library/ruby:2.1", []string{"my.registry.tld/**/*:*"}},
	{true, "my.registry.tld/group/subgroup/ruby", []string{"my.registry.tld/**/*"}},
	{true, "my.registry.tld/group/subgroup/ruby:2.1", []string{"my.registry.tld/**/*:*"}},
	{false, "library/ruby", []string{"*"}},
	{false, "library/ruby:2.1", []string{"*"}},
	{false, "my.registry.tld/ruby", []string{"*"}},
	{false, "my.registry.tld/ruby:2.1", []string{"*"}},
	{false, "my.registry.tld/library/ruby", []string{"*"}},
	{false, "my.registry.tld/library/ruby:2.1", []string{"*"}},
	{false, "my.registry.tld/group/subgroup/ruby", []string{"*"}},
	{false, "my.registry.tld/group/subgroup/ruby:2.1", []string{"*"}},
	{false, "library/ruby", []string{"*/*:*"}},
	{false, "my.registry.tld/group/subgroup/ruby", []string{"my.registry.tld/*/*"}},
	{false, "my.registry.tld/group/subgroup/ruby:2.1", []string{"my.registry.tld/*/*:*"}},
	{false, "library/ruby", []string{"**/*:*"}},
}

func TestVerifyAllowedImage(t *testing.T) {
	e := executor{}

	for _, test := range testAllowedImages {
		err := e.verifyAllowedImage(test.image, "", test.allowed_images, []string{})

		if err != nil && test.allowed {
			t.Errorf("%q must be allowed by %q", test.image, test.allowed_images)
		} else if err == nil && !test.allowed {
			t.Errorf("%q must not be allowed by %q", test.image, test.allowed_images)
		}
	}
}

type testServiceDescription struct {
	description string
	image       string
	service     string
	version     string
	alias       string
	alternative string
}

var testServices = []testServiceDescription{
	{"service", "service:latest", "service", "latest", "service", ""},
	{"service:version", "service:version", "service", "version", "service", ""},
	{"namespace/service", "namespace/service:latest", "namespace/service", "latest", "namespace__service", "namespace-service"},
	{"namespace/service:version", "namespace/service:version", "namespace/service", "version", "namespace__service", "namespace-service"},
	{"domain.tld/service", "domain.tld/service:latest", "domain.tld/service", "latest", "domain.tld__service", "domain.tld-service"},
	{"domain.tld/service:version", "domain.tld/service:version", "domain.tld/service", "version", "domain.tld__service", "domain.tld-service"},
	{"domain.tld/namespace/service", "domain.tld/namespace/service:latest", "domain.tld/namespace/service", "latest", "domain.tld__namespace__service", "domain.tld-namespace-service"},
	{"domain.tld/namespace/service:version", "domain.tld/namespace/service:version", "domain.tld/namespace/service", "version", "domain.tld__namespace__service", "domain.tld-namespace-service"},
	{"domain.tld:8080/service", "domain.tld:8080/service:latest", "domain.tld/service", "latest", "domain.tld__service", "domain.tld-service"},
	{"domain.tld:8080/service:version", "domain.tld:8080/service:version", "domain.tld/service", "version", "domain.tld__service", "domain.tld-service"},
	{"domain.tld:8080/namespace/service", "domain.tld:8080/namespace/service:latest", "domain.tld/namespace/service", "latest", "domain.tld__namespace__service", "domain.tld-namespace-service"},
	{"domain.tld:8080/namespace/service:version", "domain.tld:8080/namespace/service:version", "domain.tld/namespace/service", "version", "domain.tld__namespace__service", "domain.tld-namespace-service"},
	{"subdomain.domain.tld:8080/service", "subdomain.domain.tld:8080/service:latest", "subdomain.domain.tld/service", "latest", "subdomain.domain.tld__service", "subdomain.domain.tld-service"},
	{"subdomain.domain.tld:8080/service:version", "subdomain.domain.tld:8080/service:version", "subdomain.domain.tld/service", "version", "subdomain.domain.tld__service", "subdomain.domain.tld-service"},
	{"subdomain.domain.tld:8080/namespace/service", "subdomain.domain.tld:8080/namespace/service:latest", "subdomain.domain.tld/namespace/service", "latest", "subdomain.domain.tld__namespace__service", "subdomain.domain.tld-namespace-service"},
	{"subdomain.domain.tld:8080/namespace/service:version", "subdomain.domain.tld:8080/namespace/service:version", "subdomain.domain.tld/namespace/service", "version", "subdomain.domain.tld__namespace__service", "subdomain.domain.tld-namespace-service"},
}

func testSplitService(t *testing.T, test testServiceDescription) {
	e := executor{}
	service, version, imageName, linkNames := e.splitServiceAndVersion(test.description)

	assert.Equal(t, test.service, service, "service for "+test.description)
	assert.Equal(t, test.version, version, "version for "+test.description)
	assert.Equal(t, test.image, imageName, "image for "+test.description)
	assert.Equal(t, test.alias, linkNames[0], "alias for "+test.description)
	if test.alternative != "" {
		assert.Len(t, linkNames, 2, "linkNames len for "+test.description)
		assert.Equal(t, test.alternative, linkNames[1], "alternative for "+test.description)
	} else {
		assert.Len(t, linkNames, 1, "linkNames len for "+test.description)
	}
}

func TestSplitService(t *testing.T) {
	for _, test := range testServices {
		t.Run(test.description, func(t *testing.T) {
			testSplitService(t, test)
		})
	}
}

func testServiceFromNamedImage(t *testing.T, description, imageName, serviceName string) {
	var c docker_helpers.MockClient
	defer c.AssertExpectations(t)

	containerName := fmt.Sprintf("runner-abcdef12-project-0-concurrent-0-%s-0", strings.Replace(serviceName, "/", "__", -1))
	networkID := "network-id"

	e := executor{client: &c}
	options := buildImagePullOptions(e, imageName)
	e.Config = common.RunnerConfig{}
	e.Config.Docker = &common.DockerConfig{}
	e.Build = &common.Build{
		ProjectRunnerID: 0,
		Runner:          &common.RunnerConfig{},
	}
	e.Build.JobInfo.ProjectID = 0
	e.Build.Runner.Token = "abcdef1234567890"
	e.Context = context.Background()

	c.On("ImagePullBlocking", e.Context, imageName, options).
		Return(nil).
		Once()

	c.On("ImageInspectWithRaw", e.Context, imageName).
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Twice()

	c.On("ContainerRemove", e.Context, containerName, types.ContainerRemoveOptions{RemoveVolumes: true, Force: true}).
		Return(nil).
		Once()

	networkContainersMap := map[string]types.EndpointResource{
		"1": {Name: containerName},
	}

	c.On("NetworkList", e.Context, types.NetworkListOptions{}).
		Return([]types.NetworkResource{{ID: networkID, Name: "network-name", Containers: networkContainersMap}}, nil).
		Once()

	c.On("NetworkDisconnect", e.Context, networkID, containerName, true).
		Return(nil).
		Once()

	c.On("ContainerCreate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(container.ContainerCreateCreatedBody{ID: containerName}, nil).
		Once()

	c.On("ContainerStart", e.Context, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	linksMap := make(map[string]*types.Container)
	err := e.createFromServiceDefinition(0, common.Image{Name: description}, linksMap)
	assert.NoError(t, err)
}

func TestServiceFromNamedImage(t *testing.T) {
	for _, test := range testServices {
		t.Run(test.description, func(t *testing.T) {
			testServiceFromNamedImage(t, test.description, test.image, test.service)
		})
	}
}

func TestDockerForNamedImage(t *testing.T) {
	var c docker_helpers.MockClient
	defer c.AssertExpectations(t)
	validSHA := "real@sha256:b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c"

	e := executor{client: &c}
	e.Context = context.Background()
	options := buildImagePullOptions(e, "test")

	c.On("ImagePullBlocking", e.Context, "test:latest", options).
		Return(os.ErrNotExist).
		Once()

	c.On("ImagePullBlocking", e.Context, "tagged:tag", options).
		Return(os.ErrNotExist).
		Once()

	c.On("ImagePullBlocking", e.Context, validSHA, options).
		Return(os.ErrNotExist).
		Once()

	image, err := e.pullDockerImage("test", nil)
	assert.Error(t, err)
	assert.Nil(t, image)

	image, err = e.pullDockerImage("tagged:tag", nil)
	assert.Error(t, err)
	assert.Nil(t, image)

	image, err = e.pullDockerImage(validSHA, nil)
	assert.Error(t, err)
	assert.Nil(t, image)
}

func TestDockerForExistingImage(t *testing.T) {
	var c docker_helpers.MockClient
	defer c.AssertExpectations(t)

	e := executor{client: &c}
	e.Context = context.Background()
	options := buildImagePullOptions(e, "existing")

	c.On("ImagePullBlocking", e.Context, "existing:latest", options).
		Return(nil).
		Once()

	c.On("ImageInspectWithRaw", e.Context, "existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	image, err := e.pullDockerImage("existing", nil)
	assert.NoError(t, err)
	assert.NotNil(t, image)
}

func (e *executor) setPolicyMode(pullPolicy common.DockerPullPolicy) {
	e.Config = common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Docker: &common.DockerConfig{
				PullPolicy: pullPolicy,
			},
		},
	}
}

func TestDockerGetImageById(t *testing.T) {
	var c docker_helpers.MockClient
	defer c.AssertExpectations(t)

	// Use default policy
	e := executor{client: &c}
	e.Context = context.Background()
	e.setPolicyMode("")

	c.On("ImageInspectWithRaw", e.Context, "ID").
		Return(types.ImageInspect{ID: "ID"}, nil, nil).
		Once()

	image, err := e.getDockerImage("ID")
	assert.NoError(t, err)
	assert.NotNil(t, image)
	assert.Equal(t, "ID", image.ID)
}

func TestDockerUnknownPolicyMode(t *testing.T) {
	var c docker_helpers.MockClient
	defer c.AssertExpectations(t)

	e := executor{client: &c}
	e.Context = context.Background()
	e.setPolicyMode("unknown")

	_, err := e.getDockerImage("not-existing")
	assert.Error(t, err)
}

func TestDockerPolicyModeNever(t *testing.T) {
	var c docker_helpers.MockClient
	defer c.AssertExpectations(t)

	e := executor{client: &c}
	e.Context = context.Background()
	e.setPolicyMode(common.PullPolicyNever)

	c.On("ImageInspectWithRaw", e.Context, "existing").
		Return(types.ImageInspect{ID: "existing"}, nil, nil).
		Once()

	c.On("ImageInspectWithRaw", e.Context, "not-existing").
		Return(types.ImageInspect{}, nil, os.ErrNotExist).
		Once()

	image, err := e.getDockerImage("existing")
	assert.NoError(t, err)
	assert.Equal(t, "existing", image.ID)

	image, err = e.getDockerImage("not-existing")
	assert.Error(t, err)
}

func TestDockerPolicyModeIfNotPresentForExistingImage(t *testing.T) {
	var c docker_helpers.MockClient
	defer c.AssertExpectations(t)

	e := executor{client: &c}
	e.Context = context.Background()
	e.setPolicyMode(common.PullPolicyIfNotPresent)

	c.On("ImageInspectWithRaw", e.Context, "existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	image, err := e.getDockerImage("existing")
	assert.NoError(t, err)
	assert.NotNil(t, image)
}

func TestDockerPolicyModeIfNotPresentForNotExistingImage(t *testing.T) {
	var c docker_helpers.MockClient
	defer c.AssertExpectations(t)

	e := executor{client: &c}
	e.Context = context.Background()
	e.setPolicyMode(common.PullPolicyIfNotPresent)

	c.On("ImageInspectWithRaw", e.Context, "not-existing").
		Return(types.ImageInspect{}, nil, os.ErrNotExist).
		Once()

	options := buildImagePullOptions(e, "not-existing")
	c.On("ImagePullBlocking", e.Context, "not-existing:latest", options).
		Return(nil).
		Once()

	c.On("ImageInspectWithRaw", e.Context, "not-existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	image, err := e.getDockerImage("not-existing")
	assert.NoError(t, err)
	assert.NotNil(t, image)

	c.On("ImageInspectWithRaw", e.Context, "not-existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	// It shouldn't execute the pull for second time
	image, err = e.getDockerImage("not-existing")
	assert.NoError(t, err)
	assert.NotNil(t, image)
}

func TestDockerPolicyModeAlwaysForExistingImage(t *testing.T) {
	var c docker_helpers.MockClient
	defer c.AssertExpectations(t)

	e := executor{client: &c}
	e.Context = context.Background()
	e.setPolicyMode(common.PullPolicyAlways)

	c.On("ImageInspectWithRaw", e.Context, "existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	options := buildImagePullOptions(e, "existing:latest")
	c.On("ImagePullBlocking", e.Context, "existing:latest", options).
		Return(nil).
		Once()

	c.On("ImageInspectWithRaw", e.Context, "existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	image, err := e.getDockerImage("existing")
	assert.NoError(t, err)
	assert.NotNil(t, image)
}

func TestDockerPolicyModeAlwaysForLocalOnlyImage(t *testing.T) {
	var c docker_helpers.MockClient
	defer c.AssertExpectations(t)

	e := executor{client: &c}
	e.Context = context.Background()
	e.setPolicyMode(common.PullPolicyAlways)

	c.On("ImageInspectWithRaw", e.Context, "existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	options := buildImagePullOptions(e, "existing:lastest")
	c.On("ImagePullBlocking", e.Context, "existing:latest", options).
		Return(fmt.Errorf("not found")).
		Once()

	image, err := e.getDockerImage("existing")
	assert.Error(t, err)
	assert.Nil(t, image)
}

func TestDockerGetExistingDockerImageIfPullFails(t *testing.T) {
	var c docker_helpers.MockClient
	defer c.AssertExpectations(t)

	e := executor{client: &c}
	e.Context = context.Background()
	e.setPolicyMode(common.PullPolicyAlways)

	c.On("ImageInspectWithRaw", e.Context, "to-pull").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	options := buildImagePullOptions(e, "to-pull")
	c.On("ImagePullBlocking", e.Context, "to-pull:latest", options).
		Return(os.ErrNotExist).
		Once()

	image, err := e.getDockerImage("to-pull")
	assert.Error(t, err)
	assert.Nil(t, image, "Forces to authorize pulling")

	c.On("ImageInspectWithRaw", e.Context, "not-existing").
		Return(types.ImageInspect{}, nil, os.ErrNotExist).
		Once()

	c.On("ImagePullBlocking", e.Context, "not-existing:latest", options).
		Return(os.ErrNotExist).
		Once()

	image, err = e.getDockerImage("not-existing")
	assert.Error(t, err)
	assert.Nil(t, image, "No existing image")
}

func TestHostMountedBuildsDirectory(t *testing.T) {
	tests := []struct {
		path    string
		volumes []string
		result  bool
	}{
		{"/build", []string{"/build:/build"}, true},
		{"/build", []string{"/build/:/build"}, true},
		{"/build", []string{"/build"}, false},
		{"/build", []string{"/folder:/folder"}, false},
		{"/build", []string{"/folder"}, false},
		{"/build/other/directory", []string{"/build/:/build"}, true},
		{"/build/other/directory", []string{}, false},
	}

	for _, i := range tests {
		c := common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				BuildsDir: i.path,
				Docker: &common.DockerConfig{
					Volumes: i.volumes,
				},
			},
		}
		e := &executor{}

		t.Log("Testing", i.path, "if volumes are configured to:", i.volumes, "...")
		assert.Equal(t, i.result, e.isHostMountedVolume(i.path, i.volumes...))

		e.prepareBuildsDir(&c)
		assert.Equal(t, i.result, e.SharedBuildsDir)
	}
}

var testFileAuthConfigs = `{"auths":{"https://registry.domain.tld:5005/v1/":{"auth":"aW52YWxpZF91c2VyOmludmFsaWRfcGFzc3dvcmQ="},"registry2.domain.tld:5005":{"auth":"dGVzdF91c2VyOnRlc3RfcGFzc3dvcmQ="}}}`
var testVariableAuthConfigs = `{"auths":{"https://registry.domain.tld:5005/v1/":{"auth":"dGVzdF91c2VyOnRlc3RfcGFzc3dvcmQ="}}}`

func getAuthConfigTestExecutor(t *testing.T, precreateConfigFile bool) executor {
	tempHomeDir, err := ioutil.TempDir("", "docker-auth-configs-test")
	require.NoError(t, err)

	if precreateConfigFile {
		dockerConfigFile := path.Join(tempHomeDir, ".dockercfg")
		ioutil.WriteFile(dockerConfigFile, []byte(testFileAuthConfigs), 0600)
		docker_helpers.HomeDirectory = tempHomeDir
	} else {
		docker_helpers.HomeDirectory = ""
	}

	e := executor{}
	e.Build = &common.Build{
		Runner: &common.RunnerConfig{},
	}

	e.Build.Token = "abcd123456"

	e.Config = common.RunnerConfig{}
	e.Config.Docker = &common.DockerConfig{
		PullPolicy: common.PullPolicyAlways,
	}

	return e
}

func addGitLabRegistryCredentials(e *executor) {
	e.Build.Credentials = []common.Credentials{
		{
			Type:     "registry",
			URL:      "registry.gitlab.tld:1234",
			Username: "gitlab-ci-token",
			Password: e.Build.Token,
		},
	}
}

func addRemoteVariableCredentials(e *executor) {
	e.Build.Variables = common.JobVariables{
		common.JobVariable{
			Key:   "DOCKER_AUTH_CONFIG",
			Value: testVariableAuthConfigs,
		},
	}
}

func addLocalVariableCredentials(e *executor) {
	e.Build.Runner.Environment = []string{
		"DOCKER_AUTH_CONFIG=" + testVariableAuthConfigs,
	}
}

func assertEmptyCredentials(t *testing.T, ac *types.AuthConfig, messageElements ...string) {
	if ac != nil {
		assert.Empty(t, ac.ServerAddress, "ServerAddress for %v", messageElements)
		assert.Empty(t, ac.Username, "Username for %v", messageElements)
		assert.Empty(t, ac.Password, "Password for %v", messageElements)
	}
}

func assertCredentials(t *testing.T, serverAddress, username, password string, ac *types.AuthConfig, messageElements ...string) {
	assert.Equal(t, serverAddress, ac.ServerAddress, "ServerAddress for %v", messageElements)
	assert.Equal(t, username, ac.Username, "Username for %v", messageElements)
	assert.Equal(t, password, ac.Password, "Password for %v", messageElements)
}

func getTestAuthConfig(t *testing.T, e executor, imageName string) *types.AuthConfig {
	ac := e.getAuthConfig(imageName)

	return ac
}

func testVariableAuthConfig(t *testing.T, e executor) {
	t.Run("withoutGitLabRegistry", func(t *testing.T) {
		ac := getTestAuthConfig(t, e, "registry.domain.tld:5005/image/name:version")
		assertCredentials(t, "https://registry.domain.tld:5005/v1/", "test_user", "test_password", ac, "registry.domain.tld:5005/image/name:version")

		ac = getTestAuthConfig(t, e, "registry2.domain.tld:5005/image/name:version")
		assertCredentials(t, "registry2.domain.tld:5005", "test_user", "test_password", ac, "registry2.domain.tld:5005/image/name:version")

		ac = getTestAuthConfig(t, e, "registry.gitlab.tld:1234/image/name:version")
		assertEmptyCredentials(t, ac, "registry.gitlab.tld:1234")
	})

	t.Run("withGitLabRegistry", func(t *testing.T) {
		addGitLabRegistryCredentials(&e)

		ac := getTestAuthConfig(t, e, "registry.domain.tld:5005/image/name:version")
		assertCredentials(t, "https://registry.domain.tld:5005/v1/", "test_user", "test_password", ac, "registry.domain.tld:5005/image/name:version")

		ac = getTestAuthConfig(t, e, "registry2.domain.tld:5005/image/name:version")
		assertCredentials(t, "registry2.domain.tld:5005", "test_user", "test_password", ac, "registry2.domain.tld:5005/image/name:version")

		ac = getTestAuthConfig(t, e, "registry.gitlab.tld:1234/image/name:version")
		assertCredentials(t, "registry.gitlab.tld:1234", "gitlab-ci-token", "abcd123456", ac, "registry.gitlab.tld:1234")
	})
}

func TestGetRemoteVariableAuthConfig(t *testing.T) {
	e := getAuthConfigTestExecutor(t, true)
	addRemoteVariableCredentials(&e)

	testVariableAuthConfig(t, e)
}

func TestGetLocalVariableAuthConfig(t *testing.T) {
	e := getAuthConfigTestExecutor(t, true)
	addLocalVariableCredentials(&e)

	testVariableAuthConfig(t, e)
}

func TestGetDefaultAuthConfig(t *testing.T) {
	t.Run("withoutGitLabRegistry", func(t *testing.T) {
		e := getAuthConfigTestExecutor(t, false)

		ac := getTestAuthConfig(t, e, "docker:dind")
		assertEmptyCredentials(t, ac, "docker:dind")

		ac = getTestAuthConfig(t, e, "registry.gitlab.tld:1234/image/name:version")
		assertEmptyCredentials(t, ac, "registry.gitlab.tld:1234")

		ac = getTestAuthConfig(t, e, "registry.domain.tld:5005/image/name:version")
		assertEmptyCredentials(t, ac, "registry.domain.tld:5005/image/name:version")
	})

	t.Run("withGitLabRegistry", func(t *testing.T) {
		e := getAuthConfigTestExecutor(t, false)
		addGitLabRegistryCredentials(&e)

		ac := getTestAuthConfig(t, e, "docker:dind")
		assertEmptyCredentials(t, ac, "docker:dind")

		ac = getTestAuthConfig(t, e, "registry.domain.tld:5005/image/name:version")
		assertEmptyCredentials(t, ac, "registry.domain.tld:5005/image/name:version")

		ac = getTestAuthConfig(t, e, "registry.gitlab.tld:1234/image/name:version")
		assertCredentials(t, "registry.gitlab.tld:1234", "gitlab-ci-token", "abcd123456", ac, "registry.gitlab.tld:1234")
	})
}

func TestAuthConfigOverwritingOrder(t *testing.T) {
	testVariableAuthConfigs = `{"auths":{"registry.gitlab.tld:1234":{"auth":"ZnJvbV92YXJpYWJsZTpwYXNzd29yZA=="}}}`
	testFileAuthConfigs = `{"auths":{"registry.gitlab.tld:1234":{"auth":"ZnJvbV9maWxlOnBhc3N3b3Jk"}}}`

	imageName := "registry.gitlab.tld:1234/image/name:latest"

	t.Run("gitlabRegistryOnly", func(t *testing.T) {
		e := getAuthConfigTestExecutor(t, false)
		addGitLabRegistryCredentials(&e)

		ac := getTestAuthConfig(t, e, imageName)
		assertCredentials(t, "registry.gitlab.tld:1234", "gitlab-ci-token", e.Build.Token, ac, imageName)
	})

	t.Run("withConfigFromRemoteVariable", func(t *testing.T) {
		e := getAuthConfigTestExecutor(t, false)
		addGitLabRegistryCredentials(&e)
		addRemoteVariableCredentials(&e)

		ac := getTestAuthConfig(t, e, imageName)
		assertCredentials(t, "registry.gitlab.tld:1234", "from_variable", "password", ac, imageName)
	})

	t.Run("withConfigFromLocalVariable", func(t *testing.T) {
		e := getAuthConfigTestExecutor(t, false)
		addGitLabRegistryCredentials(&e)
		addLocalVariableCredentials(&e)

		ac := getTestAuthConfig(t, e, imageName)
		assertCredentials(t, "registry.gitlab.tld:1234", "from_variable", "password", ac, imageName)
	})

	t.Run("withConfigFromFile", func(t *testing.T) {
		e := getAuthConfigTestExecutor(t, true)
		addGitLabRegistryCredentials(&e)

		ac := getTestAuthConfig(t, e, imageName)
		assertCredentials(t, "registry.gitlab.tld:1234", "from_file", "password", ac, imageName)
	})

	t.Run("withConfigFromVariableAndFromFile", func(t *testing.T) {
		e := getAuthConfigTestExecutor(t, true)
		addGitLabRegistryCredentials(&e)
		addRemoteVariableCredentials(&e)

		ac := getTestAuthConfig(t, e, imageName)
		assertCredentials(t, "registry.gitlab.tld:1234", "from_variable", "password", ac, imageName)
	})

	t.Run("withConfigFromLocalAndRemoteVariable", func(t *testing.T) {
		e := getAuthConfigTestExecutor(t, true)
		addGitLabRegistryCredentials(&e)
		addRemoteVariableCredentials(&e)
		testVariableAuthConfigs = `{"auths":{"registry.gitlab.tld:1234":{"auth":"ZnJvbV9sb2NhbF92YXJpYWJsZTpwYXNzd29yZA=="}}}`
		addLocalVariableCredentials(&e)

		ac := getTestAuthConfig(t, e, imageName)
		assertCredentials(t, "registry.gitlab.tld:1234", "from_variable", "password", ac, imageName)
	})
}

func testGetDockerImage(t *testing.T, e executor, imageName string, setClientExpectations func(c *docker_helpers.MockClient, imageName string)) {
	t.Run("get:"+imageName, func(t *testing.T) {
		var c docker_helpers.MockClient
		defer c.AssertExpectations(t)

		e.client = &c

		setClientExpectations(&c, imageName)

		image, err := e.getDockerImage(imageName)
		assert.NoError(t, err, "Should not generate error")
		assert.Equal(t, "this-image", image.ID, "Image ID")
	})
}

func testDeniesDockerImage(t *testing.T, e executor, imageName string, setClientExpectations func(c *docker_helpers.MockClient, imageName string)) {
	t.Run("deny:"+imageName, func(t *testing.T) {
		var c docker_helpers.MockClient
		defer c.AssertExpectations(t)

		e.client = &c

		setClientExpectations(&c, imageName)

		_, err := e.getDockerImage(imageName)
		assert.Error(t, err, "Should generate error")
	})
}

func addFindsLocalImageExpectations(c *docker_helpers.MockClient, imageName string) {
	c.On("ImageInspectWithRaw", mock.Anything, imageName).
		Return(types.ImageInspect{ID: "this-image"}, nil, nil).
		Once()
}

func addPullsRemoteImageExpectations(c *docker_helpers.MockClient, imageName string) {
	c.On("ImageInspectWithRaw", mock.Anything, imageName).
		Return(types.ImageInspect{ID: "not-this-image"}, nil, nil).
		Once()

	c.On("ImagePullBlocking", mock.Anything, imageName, mock.AnythingOfType("types.ImagePullOptions")).
		Return(nil).
		Once()

	c.On("ImageInspectWithRaw", mock.Anything, imageName).
		Return(types.ImageInspect{ID: "this-image"}, nil, nil).
		Once()
}

func addDeniesPullExpectations(c *docker_helpers.MockClient, imageName string) {
	c.On("ImageInspectWithRaw", mock.Anything, imageName).
		Return(types.ImageInspect{ID: "image"}, nil, nil).
		Once()

	c.On("ImagePullBlocking", mock.Anything, imageName, mock.AnythingOfType("types.ImagePullOptions")).
		Return(fmt.Errorf("deny pulling")).
		Once()
}

func TestPullPolicyWhenAlwaysIsSet(t *testing.T) {
	remoteImage := "registry.domain.tld:5005/image/name:version"
	gitlabImage := "registry.gitlab.tld:1234/image/name:version"

	e := getAuthConfigTestExecutor(t, false)
	e.Context = context.Background()
	e.Config.Docker.PullPolicy = common.PullPolicyAlways

	testGetDockerImage(t, e, remoteImage, addPullsRemoteImageExpectations)
	testDeniesDockerImage(t, e, remoteImage, addDeniesPullExpectations)

	testGetDockerImage(t, e, gitlabImage, addPullsRemoteImageExpectations)
	testDeniesDockerImage(t, e, gitlabImage, addDeniesPullExpectations)
}

func TestPullPolicyWhenIfNotPresentIsSet(t *testing.T) {
	remoteImage := "registry.domain.tld:5005/image/name:version"
	gitlabImage := "registry.gitlab.tld:1234/image/name:version"

	e := getAuthConfigTestExecutor(t, false)
	e.Context = context.Background()
	e.Config.Docker.PullPolicy = common.PullPolicyIfNotPresent

	testGetDockerImage(t, e, remoteImage, addFindsLocalImageExpectations)
	testGetDockerImage(t, e, gitlabImage, addFindsLocalImageExpectations)
}

func TestDockerWatchOn_1_12_4(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "docker", "info") {
		return
	}

	e := executor{}
	e.Context = context.Background()
	e.Build = &common.Build{
		Runner: &common.RunnerConfig{},
	}
	e.Build.Token = "abcd123456"
	e.BuildShell = &common.ShellConfiguration{
		Environment: []string{},
	}

	e.Config = common.RunnerConfig{}
	e.Config.Docker = &common.DockerConfig{
		PullPolicy: common.PullPolicyAlways,
	}

	e.Trace = &common.Trace{Writer: os.Stdout}

	err := e.connectDocker()
	assert.NoError(t, err)

	container, err := e.createContainer("build", common.Image{Name: "alpine"}, []string{"/bin/sh"}, []string{})
	assert.NoError(t, err)
	assert.NotNil(t, container)

	input := bytes.NewBufferString("echo 'script'")

	finished := make(chan bool, 1)
	wg := &sync.WaitGroup{}
	wg.Add(1) // Avoid a race where assert.NoError() is called too late in the goroutine
	go func() {
		err = e.watchContainer(e.Context, container.ID, input)
		assert.NoError(t, err)
		t.Log(err)
		finished <- true
		wg.Done()
	}()

	select {
	case <-finished:
	case <-time.After(15 * time.Second):
		t.Error("Container script not finished")
	}

	err = e.removeContainer(e.Context, container.ID)
	assert.NoError(t, err)
	wg.Wait()
}

type containerConfigExpectations func(*testing.T, *container.Config, *container.HostConfig)

type dockerConfigurationTestFakeDockerClient struct {
	docker_helpers.MockClient

	cce containerConfigExpectations
	t   *testing.T
}

func (c *dockerConfigurationTestFakeDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *network.NetworkingConfig, containerName string) (container.ContainerCreateCreatedBody, error) {
	c.cce(c.t, config, hostConfig)
	return container.ContainerCreateCreatedBody{ID: "abc"}, nil
}

func prepareTestDockerConfiguration(t *testing.T, dockerConfig *common.DockerConfig, cce containerConfigExpectations) (*dockerConfigurationTestFakeDockerClient, *executor) {
	c := &dockerConfigurationTestFakeDockerClient{
		cce: cce,
		t:   t,
	}

	e := &executor{}
	e.client = c
	e.Config.Docker = dockerConfig
	e.Build = &common.Build{
		Runner: &common.RunnerConfig{},
	}
	e.Build.Token = "abcd123456"
	e.BuildShell = &common.ShellConfiguration{
		Environment: []string{},
	}

	c.On("ImageInspectWithRaw", mock.Anything, "alpine").
		Return(types.ImageInspect{ID: "123"}, []byte{}, nil).Twice()
	c.On("ImagePullBlocking", mock.Anything, "alpine:latest", mock.Anything).
		Return(nil).Once()
	c.On("NetworkList", mock.Anything, mock.Anything).
		Return([]types.NetworkResource{}, nil).Once()
	c.On("ContainerRemove", mock.Anything, mock.Anything, mock.Anything).
		Return(nil).Once()

	return c, e
}

func testDockerConfigurationWithJobContainer(t *testing.T, dockerConfig *common.DockerConfig, cce containerConfigExpectations) {
	c, e := prepareTestDockerConfiguration(t, dockerConfig, cce)
	defer c.AssertExpectations(t)

	c.On("ContainerInspect", mock.Anything, "abc").
		Return(types.ContainerJSON{}, nil).Once()

	_, err := e.createContainer("build", common.Image{Name: "alpine"}, []string{"/bin/sh"}, []string{})
	assert.NoError(t, err, "Should create container without errors")
}

func testDockerConfigurationWithServiceContainer(t *testing.T, dockerConfig *common.DockerConfig, cce containerConfigExpectations) {
	c, e := prepareTestDockerConfiguration(t, dockerConfig, cce)
	defer c.AssertExpectations(t)

	c.On("ContainerStart", mock.Anything, "abc", mock.Anything).
		Return(nil).Once()

	_, err := e.createService(0, "build", "latest", "alpine", common.Image{Command: []string{"/bin/sh"}})
	assert.NoError(t, err, "Should create service container without errors")
}

func TestDockerMemorySetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		Memory: "42m",
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {
		assert.Equal(t, int64(44040192), hostConfig.Memory)
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
}

func TestDockerMemorySwapSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		MemorySwap: "2g",
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {
		assert.Equal(t, int64(2147483648), hostConfig.MemorySwap)
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
}

func TestDockerMemoryReservationSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		MemoryReservation: "64m",
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {
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

			cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {
				assert.Equal(t, int64(example.nanocpus), hostConfig.NanoCPUs)
			}

			testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
		})
	}
}

func TestDockerCPUSetCPUsSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		CPUSetCPUs: "1-3,5",
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {
		assert.Equal(t, "1-3,5", hostConfig.CpusetCpus)
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
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
func TestDockerTmpfsSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		Tmpfs: map[string]string{
			"/tmpfs": "rw,noexec",
		},
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {
		require.NotEmpty(t, hostConfig.Tmpfs)
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
}
func TestDockerUserNSSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		UsernsMode: "host",
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {
		assert.Equal(t, container.UsernsMode("host"), hostConfig.UsernsMode)
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)

}

func TestDockerRuntimeSetting(t *testing.T) {
	dockerConfig := &common.DockerConfig{
		Runtime: "runc",
	}

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {
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

	cce := func(t *testing.T, config *container.Config, hostConfig *container.HostConfig) {
		assert.Equal(t, "1", hostConfig.Sysctls["net.ipv4.ip_forward"])
	}

	testDockerConfigurationWithJobContainer(t, dockerConfig, cce)
}

func init() {
	docker_helpers.HomeDirectory = ""
}
