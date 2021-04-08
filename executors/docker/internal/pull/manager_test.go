package pull

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

func TestNewDefaultManager(t *testing.T) {
	m := NewManager(context.Background(), newLoggerMock(), ManagerConfig{}, &docker.MockClient{}, nil)
	assert.IsType(t, &manager{}, m)
}

func TestDockerForNamedImage(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)
	validSHA := "real@sha256:b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c"

	m := newDefaultTestManager(c)
	options := buildImagePullOptions()

	c.On("ImagePullBlocking", m.context, "test:latest", options).
		Return(os.ErrNotExist).
		Once()

	c.On("ImagePullBlocking", m.context, "tagged:tag", options).
		Return(os.ErrNotExist).
		Once()

	c.On("ImagePullBlocking", m.context, validSHA, options).
		Return(os.ErrNotExist).
		Once()

	image, err := m.pullDockerImage("test", nil)
	assert.Error(t, err)
	assert.Nil(t, image)

	image, err = m.pullDockerImage("tagged:tag", nil)
	assert.Error(t, err)
	assert.Nil(t, image)

	image, err = m.pullDockerImage(validSHA, nil)
	assert.Error(t, err)
	assert.Nil(t, image)
}

func TestDockerForExistingImage(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	m := newDefaultTestManager(c)
	c.On("ImagePullBlocking", m.context, "existing:latest", buildImagePullOptions()).
		Return(nil).
		Once()

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	image, err := m.pullDockerImage("existing", nil)
	assert.NoError(t, err)
	assert.NotNil(t, image)
}

func TestDockerGetImageById(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	// Use default policy
	m := newDefaultTestManager(c)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	c.On("ImageInspectWithRaw", m.context, "ID").
		Return(types.ImageInspect{ID: "ID"}, nil, nil).
		Once()

	image, err := m.GetDockerImage("ID")
	assert.NoError(t, err)
	assert.NotNil(t, image)
	assert.Equal(t, "ID", image.ID)
}

func TestDockerUnknownPolicyMode(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	m := newDefaultTestManager(c, "unknown")

	_, err := m.GetDockerImage("not-existing")
	assert.Error(t, err)
}

func TestDockerPolicyModeNever(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	m := newDefaultTestManager(c, common.PullPolicyNever)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(types.ImageInspect{ID: "existing"}, nil, nil).
		Once()

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(types.ImageInspect{}, nil, os.ErrNotExist).
		Once()

	image, err := m.GetDockerImage("existing")
	assert.NoError(t, err)
	assert.Equal(t, "existing", image.ID)

	_, err = m.GetDockerImage("not-existing")
	assert.Error(t, err)
}

func TestDockerPolicyModeIfNotPresentForExistingImage(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	m := newDefaultTestManager(c, common.PullPolicyIfNotPresent)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	image, err := m.GetDockerImage("existing")
	assert.NoError(t, err)
	assert.NotNil(t, image)
}

func TestDockerPolicyModeIfNotPresentForNotExistingImage(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	m := newDefaultTestManager(c, common.PullPolicyIfNotPresent)

	pullImageHookCalled := false
	m.onPullImageHookFunc = func() { pullImageHookCalled = true }

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(types.ImageInspect{}, nil, os.ErrNotExist).
		Once()

	c.On("ImagePullBlocking", m.context, "not-existing:latest", buildImagePullOptions()).
		Return(nil).
		Once()

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	image, err := m.GetDockerImage("not-existing")
	assert.NoError(t, err)
	assert.NotNil(t, image)
	assert.True(t, pullImageHookCalled)

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	// It shouldn't execute the pull for second time
	image, err = m.GetDockerImage("not-existing")
	assert.NoError(t, err)
	assert.NotNil(t, image)
}

func TestDockerPolicyModeAlwaysForExistingImage(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	m := newDefaultTestManager(c, common.PullPolicyAlways)

	pullImageHookCalled := false
	m.onPullImageHookFunc = func() { pullImageHookCalled = true }

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	c.On("ImagePullBlocking", m.context, "existing:latest", buildImagePullOptions()).
		Return(nil).
		Once()

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	image, err := m.GetDockerImage("existing")
	assert.NoError(t, err)
	assert.NotNil(t, image)
	assert.True(t, pullImageHookCalled)
}

func TestDockerPolicyModeAlwaysForLocalOnlyImage(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	m := newDefaultTestManager(c, common.PullPolicyAlways)

	pullImageHookCalled := false
	m.onPullImageHookFunc = func() { pullImageHookCalled = true }

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	c.On("ImagePullBlocking", m.context, "existing:latest", buildImagePullOptions()).
		Return(fmt.Errorf("not found")).
		Once()

	image, err := m.GetDockerImage("existing")
	assert.Error(t, err)
	assert.Nil(t, image)
	assert.True(t, pullImageHookCalled)
}

func TestDockerGetExistingDockerImageIfPullFails(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	m := newDefaultTestManager(c, common.PullPolicyAlways)

	c.On("ImageInspectWithRaw", m.context, "to-pull").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	options := buildImagePullOptions()
	c.On("ImagePullBlocking", m.context, "to-pull:latest", options).
		Return(os.ErrNotExist).
		Once()

	image, err := m.GetDockerImage("to-pull")
	assert.Error(t, err)
	assert.Nil(t, image, "Forces to authorize pulling")

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(types.ImageInspect{}, nil, os.ErrNotExist).
		Once()

	c.On("ImagePullBlocking", m.context, "not-existing:latest", options).
		Return(os.ErrNotExist).
		Once()

	image, err = m.GetDockerImage("not-existing")
	assert.Error(t, err)
	assert.Nil(t, image, "No existing image")
}

func TestCombinedDockerPolicyModesAlwaysAndIfNotPresentForExistingImage(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	logger, _ := logrustest.NewNullLogger()
	output := bytes.NewBufferString("")
	buildLogger := common.NewBuildLogger(&common.Trace{Writer: output}, logger.WithField("test", t.Name()))

	m := newDefaultTestManager(c, common.PullPolicyAlways, common.PullPolicyIfNotPresent)
	m.logger = &buildLogger

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	c.On("ImagePullBlocking", m.context, "existing:latest", buildImagePullOptions()).
		Return(errors.New("received unexpected HTTP status: 502 Bad Gateway")).
		Once()

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(types.ImageInspect{ID: "local-image-id"}, nil, nil).
		Once()

	image, err := m.GetDockerImage("existing")
	assert.NoError(t, err)
	assert.Contains(t, output.String(), `WARNING: Failed to pull image with policy "always": `+
		`received unexpected HTTP status: 502 Bad Gateway`)
	assert.Contains(t, output.String(), `Attempt #2: Trying "if-not-present" pull policy`)
	assert.Contains(t, output.String(), `Using locally found image version due to "if-not-present" pull policy`)
	require.NotNil(t, image)
	assert.Equal(t, "local-image-id", image.ID)
}

func TestCombinedDockerPolicyModeAlwaysAndIfNotPresentForNonExistingImage(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	m := newDefaultTestManager(c, common.PullPolicyAlways, common.PullPolicyIfNotPresent)

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(types.ImageInspect{}, nil, os.ErrNotExist).
		Once()

	c.On("ImagePullBlocking", m.context, "not-existing:latest", buildImagePullOptions()).
		Return(os.ErrNotExist).
		Twice()

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(types.ImageInspect{}, nil, os.ErrNotExist).
		Once()

	image, err := m.GetDockerImage("not-existing")
	assert.Error(t, err)
	assert.Nil(t, image, "No existing image")
}

func TestPullPolicyWhenAlwaysIsSet(t *testing.T) {
	remoteImage := "registry.domain.tld:5005/image/name:version"
	gitlabImage := "registry.gitlab.tld:1234/image/name:version"

	m := newDefaultTestManager(nil, common.PullPolicyAlways)

	testGetDockerImage(t, m, remoteImage, addPullsRemoteImageExpectations)
	testDeniesDockerImage(t, m, remoteImage, addDeniesPullExpectations)

	testGetDockerImage(t, m, gitlabImage, addPullsRemoteImageExpectations)
	testDeniesDockerImage(t, m, gitlabImage, addDeniesPullExpectations)
}

func TestPullPolicyWhenIfNotPresentIsSet(t *testing.T) {
	remoteImage := "registry.domain.tld:5005/image/name:version"
	gitlabImage := "registry.gitlab.tld:1234/image/name:version"

	m := newDefaultTestManager(nil, common.PullPolicyIfNotPresent)

	testGetDockerImage(t, m, remoteImage, addFindsLocalImageExpectations)
	testGetDockerImage(t, m, gitlabImage, addFindsLocalImageExpectations)
}

// ImagePullOptions contains the RegistryAuth which is inferred from the docker
// configuration for the user, so just mock it out here.
func buildImagePullOptions() mock.AnythingOfTypeArgument {
	return mock.AnythingOfType("ImagePullOptions")
}

func newLoggerMock() *mockPullLogger {
	loggerMock := new(mockPullLogger)
	loggerMock.On(
		"Debugln",
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
	).Maybe()
	loggerMock.On("Infoln", mock.AnythingOfType("string")).Maybe()
	loggerMock.On("Warningln", mock.AnythingOfType("string")).Maybe()
	loggerMock.On(
		"Println",
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
	).Maybe()
	loggerMock.On(
		"Println",
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
	).Maybe()

	return loggerMock
}

func newDefaultTestManager(client *docker.MockClient, pullPolicy ...string) *manager {
	// Create a unique context value that can be later compared with to ensure
	// that the production code is passing it to the mocks
	ctx := context.WithValue(context.Background(), new(struct{}), "unique context")

	return &manager{
		context: ctx,
		logger:  newLoggerMock(),
		config: ManagerConfig{
			DockerConfig: &common.DockerConfig{PullPolicy: pullPolicy},
		},
		client: client,
	}
}

func testGetDockerImage(
	t *testing.T,
	m *manager,
	imageName string,
	setClientExpectations func(c *docker.MockClient, imageName string),
) {
	t.Run("get:"+imageName, func(t *testing.T) {
		c := new(docker.MockClient)
		defer c.AssertExpectations(t)

		m.client = c

		setClientExpectations(c, imageName)

		image, err := m.GetDockerImage(imageName)
		assert.NoError(t, err, "Should not generate error")
		assert.Equal(t, "this-image", image.ID, "Image ID")
	})
}

func testDeniesDockerImage(
	t *testing.T,
	m *manager,
	imageName string,
	setClientExpectations func(c *docker.MockClient, imageName string),
) {
	t.Run("deny:"+imageName, func(t *testing.T) {
		c := new(docker.MockClient)
		defer c.AssertExpectations(t)

		m.client = c

		setClientExpectations(c, imageName)

		_, err := m.GetDockerImage(imageName)
		assert.Error(t, err, "Should generate error")
	})
}

func addFindsLocalImageExpectations(c *docker.MockClient, imageName string) {
	c.On("ImageInspectWithRaw", mock.Anything, imageName).
		Return(types.ImageInspect{ID: "this-image"}, nil, nil).
		Once()
}

func addPullsRemoteImageExpectations(c *docker.MockClient, imageName string) {
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

func addDeniesPullExpectations(c *docker.MockClient, imageName string) {
	c.On("ImageInspectWithRaw", mock.Anything, imageName).
		Return(types.ImageInspect{ID: "image"}, nil, nil).
		Once()

	c.On("ImagePullBlocking", mock.Anything, imageName, mock.AnythingOfType("types.ImagePullOptions")).
		Return(fmt.Errorf("deny pulling")).
		Once()
}
