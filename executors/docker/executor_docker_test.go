package docker

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"path"

	"github.com/fsouza/go-dockerclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/helpers/docker"
)

func TestParseDeviceStringOne(t *testing.T) {
	e := executor{}

	device, err := e.parseDeviceString("/dev/kvm")

	assert.NoError(t, err)
	assert.Equal(t, device.PathOnHost, "/dev/kvm")
	assert.Equal(t, device.PathInContainer, "/dev/kvm")
	assert.Equal(t, device.CgroupPermissions, "rwm")
}

func TestParseDeviceStringTwo(t *testing.T) {
	e := executor{}

	device, err := e.parseDeviceString("/dev/kvm:/devices/kvm")

	assert.NoError(t, err)
	assert.Equal(t, device.PathOnHost, "/dev/kvm")
	assert.Equal(t, device.PathInContainer, "/devices/kvm")
	assert.Equal(t, device.CgroupPermissions, "rwm")
}

func TestParseDeviceStringThree(t *testing.T) {
	e := executor{}

	device, err := e.parseDeviceString("/dev/kvm:/devices/kvm:r")

	assert.NoError(t, err)
	assert.Equal(t, device.PathOnHost, "/dev/kvm")
	assert.Equal(t, device.PathInContainer, "/devices/kvm")
	assert.Equal(t, device.CgroupPermissions, "r")
}

func TestParseDeviceStringFour(t *testing.T) {
	e := executor{}

	_, err := e.parseDeviceString("/dev/kvm:/devices/kvm:r:oops")

	assert.Error(t, err)
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

	e := executor{client: &c}
	ac, _ := e.getAuthConfig(imageName)

	e.Config = common.RunnerConfig{}
	e.Config.Docker = &common.DockerConfig{}
	e.Build = &common.Build{
		ProjectRunnerID: 0,
		Runner:          &common.RunnerConfig{},
	}
	e.Build.ProjectID = 0
	e.Build.Runner.Token = "abcdef1234567890"

	c.On("PullImage", docker.PullImageOptions{Repository: imageName}, ac).
		Return(nil).
		Once()

	c.On("InspectImage", imageName).
		Return(&docker.Image{}, nil).
		Twice()

	c.On("RemoveContainer", mock.Anything).
		Return(nil).
		Once()

	containerName := fmt.Sprintf("runner-abcdef12-project-0-concurrent-0-%s", strings.Replace(serviceName, "/", "__", -1))
	networkID := "network-id"

	networkContainersMap := make(map[string]docker.Endpoint)
	networkContainersMap["1"] = docker.Endpoint{Name: containerName}

	c.On("ListNetworks").
		Return([]docker.Network{docker.Network{ID: networkID, Name: "network-name", Containers: networkContainersMap}}, nil).
		Once()

	c.On("DisconnectNetwork", networkID, docker.NetworkConnectionOptions{Container: containerName}).
		Return(nil).
		Once()

	c.On("CreateContainer", mock.Anything).
		Return(&docker.Container{}, nil).
		Once()

	c.On("StartContainer", mock.Anything, mock.Anything).
		Return(nil).
		Once()

	linksMap := make(map[string]*docker.Container)
	err := e.createFromServiceDescription(description, linksMap)
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

	e := executor{client: &c}
	ac, _ := e.getAuthConfig("test")

	c.On("PullImage", docker.PullImageOptions{Repository: "test:latest"}, ac).
		Return(os.ErrNotExist).
		Once()

	c.On("PullImage", docker.PullImageOptions{Repository: "tagged:tag"}, ac).
		Return(os.ErrNotExist).
		Once()

	c.On("PullImage", docker.PullImageOptions{Repository: "real@sha"}, ac).
		Return(os.ErrNotExist).
		Once()

	image, err := e.pullDockerImage("test")
	assert.Error(t, err)
	assert.Nil(t, image)

	image, err = e.pullDockerImage("tagged:tag")
	assert.Error(t, err)
	assert.Nil(t, image)

	image, err = e.pullDockerImage("real@sha")
	assert.Error(t, err)
	assert.Nil(t, image)
}

func TestDockerForExistingImage(t *testing.T) {
	var c docker_helpers.MockClient
	defer c.AssertExpectations(t)

	e := executor{client: &c}
	ac, _ := e.getAuthConfig("existing")

	c.On("PullImage", docker.PullImageOptions{Repository: "existing:latest"}, ac).
		Return(nil).
		Once()
	c.On("InspectImage", "existing").
		Return(&docker.Image{}, nil).
		Once()

	image, err := e.pullDockerImage("existing")
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

	c.On("InspectImage", "ID").
		Return(&docker.Image{ID: "ID"}, nil).
		Once()

	// Use default policy
	e := executor{client: &c}
	e.setPolicyMode("")

	image, err := e.getDockerImage("ID")
	assert.NoError(t, err)
	assert.NotNil(t, image)
	assert.Equal(t, "ID", image.ID)
}

func TestDockerUnknownPolicyMode(t *testing.T) {
	var c docker_helpers.MockClient
	defer c.AssertExpectations(t)

	e := executor{client: &c}
	e.setPolicyMode("unknown")

	_, err := e.getDockerImage("not-existing")
	assert.Error(t, err)
}

func TestDockerPolicyModeNever(t *testing.T) {
	var c docker_helpers.MockClient
	defer c.AssertExpectations(t)

	c.On("InspectImage", "existing").
		Return(&docker.Image{}, nil).
		Once()

	c.On("InspectImage", "not-existing").
		Return(nil, os.ErrNotExist).
		Once()

	e := executor{client: &c}
	e.setPolicyMode(common.PullPolicyNever)

	image, err := e.getDockerImage("existing")
	assert.NoError(t, err)
	assert.NotNil(t, image)

	image, err = e.getDockerImage("not-existing")
	assert.Error(t, err)
	assert.Nil(t, image)
}

func TestDockerPolicyModeIfNotPresentForExistingImage(t *testing.T) {
	var c docker_helpers.MockClient
	defer c.AssertExpectations(t)

	e := executor{client: &c}
	e.setPolicyMode(common.PullPolicyIfNotPresent)

	c.On("InspectImage", "existing").
		Return(&docker.Image{}, nil).
		Once()

	image, err := e.getDockerImage("existing")
	assert.NoError(t, err)
	assert.NotNil(t, image)
}

func TestDockerPolicyModeIfNotPresentForNotExistingImage(t *testing.T) {
	var c docker_helpers.MockClient
	defer c.AssertExpectations(t)

	e := executor{client: &c}
	e.setPolicyMode(common.PullPolicyIfNotPresent)

	c.On("InspectImage", "not-existing").
		Return(nil, os.ErrNotExist).
		Once()

	ac, _ := e.getAuthConfig("not-existing")
	c.On("PullImage", docker.PullImageOptions{Repository: "not-existing:latest"}, ac).
		Return(nil).
		Once()

	c.On("InspectImage", "not-existing").
		Return(&docker.Image{}, nil).
		Once()

	image, err := e.getDockerImage("not-existing")
	assert.NoError(t, err)
	assert.NotNil(t, image)

	c.On("InspectImage", "not-existing").
		Return(&docker.Image{}, nil).
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
	e.setPolicyMode(common.PullPolicyAlways)

	c.On("InspectImage", "existing").
		Return(&docker.Image{}, nil).
		Once()

	ac, _ := e.getAuthConfig("existing")
	c.On("PullImage", docker.PullImageOptions{Repository: "existing:latest"}, ac).
		Return(nil).
		Once()

	c.On("InspectImage", "existing").
		Return(&docker.Image{}, nil).
		Once()

	image, err := e.getDockerImage("existing")
	assert.NoError(t, err)
	assert.NotNil(t, image)
}

func TestDockerGetExistingDockerImageIfPullFails(t *testing.T) {
	var c docker_helpers.MockClient
	defer c.AssertExpectations(t)

	e := executor{client: &c}
	e.setPolicyMode(common.PullPolicyAlways)

	c.On("InspectImage", "to-pull").
		Return(&docker.Image{}, nil).
		Once()

	ac, _ := e.getAuthConfig("to-pull")
	c.On("PullImage", docker.PullImageOptions{Repository: "to-pull:latest"}, ac).
		Return(os.ErrNotExist).
		Once()

	image, err := e.getDockerImage("to-pull")
	assert.NoError(t, err)
	assert.NotNil(t, image, "Returns existing image")

	c.On("InspectImage", "not-existing").
		Return(nil, os.ErrNotExist).
		Once()

	c.On("PullImage", docker.PullImageOptions{Repository: "not-existing:latest"}, ac).
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

func getAuthConfigTestExecutor(precreateConfigFile bool) executor {
	tempHomeDir, _ := ioutil.TempDir("", "docker-auth-configs-test")
	docker_helpers.ResolveHomeDir = func(userName string) (string, error) {
		return tempHomeDir, nil
	}

	if precreateConfigFile {
		dockerConfigFile := path.Join(tempHomeDir, ".dockercfg")
		ioutil.WriteFile(dockerConfigFile, []byte(testFileAuthConfigs), 0600)
	}

	e := executor{}
	e.Build = &common.Build{
		Runner: &common.RunnerConfig{},
	}

	return e
}

func testAuthConfig(t *testing.T, prepareExecutor func(exec *executor)) {
	e := getAuthConfigTestExecutor(true)

	prepareExecutor(&e)

	ac, err := e.getAuthConfig("registry.domain.tld:5005/image/name:version")
	assert.NoError(t, err)
	assert.Equal(t, "https://registry.domain.tld:5005/v1/", ac.ServerAddress, "Docker auth server address")
	assert.Equal(t, "test_user", ac.Username, "Docker auth username")
	assert.Equal(t, "test_password", ac.Password, "Docker auth password")

	ac, err = e.getAuthConfig("registry2.domain.tld:5005/image/name:version")
	assert.NoError(t, err)
	assert.Equal(t, "registry2.domain.tld:5005", ac.ServerAddress, "Docker auth server address")
	assert.Equal(t, "test_user", ac.Username, "Docker auth username")
	assert.Equal(t, "test_password", ac.Password, "Docker auth password")
}

func TestGetRemoteVariableAuthConfig(t *testing.T) {
	testAuthConfig(t, func(e *executor) {
		e.Build.Variables = common.BuildVariables{
			common.BuildVariable{
				Key:   "DOCKER_AUTH_CONFIG",
				Value: testVariableAuthConfigs,
			},
		}
	})
}

func TestGetLocalVariableAuthConfig(t *testing.T) {
	testAuthConfig(t, func(e *executor) {
		e.Build.Runner.Environment = []string{
			"DOCKER_AUTH_CONFIG=" + testVariableAuthConfigs,
		}
	})
}

func TestGetDefaultAuthConfig(t *testing.T) {
	e := getAuthConfigTestExecutor(false)

	ac, err := e.getAuthConfig("docker:dind")
	assert.NoError(t, err)
	assert.Empty(t, ac.ServerAddress, "Docker auth server address")
	assert.Empty(t, ac.Username, "Docker auth username")
	assert.Empty(t, ac.Password, "Docker auth password")

	ac, err = e.getAuthConfig("registry.domain.tld:5005/image/name:version")
	assert.NoError(t, err)
	assert.Empty(t, ac.ServerAddress, "Docker auth server address")
	assert.Empty(t, ac.Username, "Docker auth username")
	assert.Empty(t, ac.Password, "Docker auth password")
}
