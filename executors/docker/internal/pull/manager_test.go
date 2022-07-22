//go:build !integration

package pull

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
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

	dockerConfig := &common.DockerConfig{}
	m := newDefaultTestManager(c, dockerConfig)
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

func TestDockerForImagePullFailures(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)
	errTest := errors.New("this is a test")

	dockerConfig := &common.DockerConfig{}
	m := newDefaultTestManager(c, dockerConfig)
	options := buildImagePullOptions()

	tests := map[string]struct {
		imageName string
		initMock  func(c *docker.MockClient, imageName string, options mock.AnythingOfTypeArgument)
		assert    func(m *manager, imageName string)
	}{
		"ImagePullBlocking unwrapped system failure": {
			imageName: "unwrapped-system:failure",
			initMock: func(c *docker.MockClient, imageName string, options mock.AnythingOfTypeArgument) {
				c.On("ImagePullBlocking", m.context, imageName, options).
					Return(errdefs.System(errTest)).
					Once()
			},
			assert: func(m *manager, imageName string) {
				var buildError *common.BuildError
				image, err := m.pullDockerImage(imageName, nil)
				assert.Nil(t, image)
				assert.Error(t, err)
				require.ErrorAs(t, err, &buildError)
				assert.Equal(t, buildError.FailureReason, common.ScriptFailure)
			},
		},
		"ImagePullBlocking wrapped system failure": {
			imageName: "wrapped-system:failure",
			initMock: func(c *docker.MockClient, imageName string, options mock.AnythingOfTypeArgument) {
				c.On("ImagePullBlocking", m.context, imageName, options).
					Return(fmt.Errorf("wrapped error: %w", errdefs.System(errTest))).
					Once()
			},
			assert: func(m *manager, imageName string) {
				var buildError *common.BuildError
				image, err := m.pullDockerImage(imageName, nil)
				assert.Nil(t, image)
				assert.Error(t, err)
				require.ErrorAs(t, err, &buildError)
				assert.Equal(t, buildError.FailureReason, common.ScriptFailure)
			},
		},
		"ImagePullBlocking two level wrapped system failure": {
			imageName: "two-level-wrapped-system:failure",
			initMock: func(c *docker.MockClient, imageName string, options mock.AnythingOfTypeArgument) {
				c.On("ImagePullBlocking", m.context, imageName, options).
					Return(fmt.Errorf("wrapped error: %w", fmt.Errorf("wrapped error: %w", errdefs.System(errTest)))).
					Once()
			},
			assert: func(m *manager, imageName string) {
				var buildError *common.BuildError
				image, err := m.pullDockerImage(imageName, nil)
				assert.Nil(t, image)
				assert.Error(t, err)
				require.ErrorAs(t, err, &buildError)
				assert.Equal(t, buildError.FailureReason, common.ScriptFailure)
			},
		},
		"ImagePullBlocking wrapped request timeout failure": {
			imageName: "wrapped-request-timeout:failure",
			initMock: func(c *docker.MockClient, imageName string, options mock.AnythingOfTypeArgument) {
				c.On("ImagePullBlocking", m.context, imageName, options).
					Return(fmt.Errorf(
						"wrapped error: %w", errdefs.System(errors.New(
							"request canceled while waiting for connection",
						)))).
					Once()
			},
			assert: func(m *manager, imageName string) {
				var buildError *common.BuildError
				image, err := m.pullDockerImage(imageName, nil)
				assert.Nil(t, image)
				assert.Error(t, err)
				require.ErrorAs(t, err, &buildError)
				assert.Equal(t, buildError.FailureReason, common.ScriptFailure)
			},
		},
		"ImagePullBlocking two level wrapped request timeout failure": {
			imageName: "lwo-level-wrapped-request-timeout:failure",
			initMock: func(c *docker.MockClient, imageName string, options mock.AnythingOfTypeArgument) {
				c.On("ImagePullBlocking", m.context, imageName, options).
					Return(fmt.Errorf(
						"wrapped error: %w", fmt.Errorf(
							"wrapped error: %w", errdefs.System(errors.New(
								"request canceled while waiting for connection",
							))))).
					Once()
			},
			assert: func(m *manager, imageName string) {
				var buildError *common.BuildError
				image, err := m.pullDockerImage(imageName, nil)
				assert.Nil(t, image)
				assert.Error(t, err)
				require.ErrorAs(t, err, &buildError)
				assert.Equal(t, buildError.FailureReason, common.ScriptFailure)
			},
		},
		"ImagePullBlocking unwrapped script failure": {
			imageName: "unwrapped-script:failure",
			initMock: func(c *docker.MockClient, imageName string, options mock.AnythingOfTypeArgument) {
				c.On("ImagePullBlocking", m.context, imageName, options).
					Return(errdefs.NotFound(errTest)).
					Once()
			},
			assert: func(m *manager, imageName string) {
				var buildError *common.BuildError
				image, err := m.pullDockerImage(imageName, nil)
				assert.Nil(t, image)
				assert.Error(t, err)
				require.ErrorAs(t, err, &buildError)
				assert.Equal(t, buildError.FailureReason, common.ScriptFailure)
			},
		},
		"ImagePullBlocking wrapped script failure": {
			imageName: "wrapped-script:failure",
			initMock: func(c *docker.MockClient, imageName string, options mock.AnythingOfTypeArgument) {
				c.On("ImagePullBlocking", m.context, imageName, options).
					Return(fmt.Errorf("wrapped error: %w", errdefs.NotFound(errTest))).
					Once()
			},
			assert: func(m *manager, imageName string) {
				var buildError *common.BuildError
				image, err := m.pullDockerImage(imageName, nil)
				assert.Nil(t, image)
				assert.Error(t, err)
				require.ErrorAs(t, err, &buildError)
				assert.Equal(t, buildError.FailureReason, common.ScriptFailure)
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			tc.initMock(c, tc.imageName, options)
			tc.assert(m, tc.imageName)
		})
	}
}

func TestDockerForExistingImage(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	dockerConfig := &common.DockerConfig{}
	m := newDefaultTestManager(c, dockerConfig)
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

	dockerConfig := &common.DockerConfig{}
	m := newDefaultTestManager(c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	c.On("ImageInspectWithRaw", m.context, "ID").
		Return(types.ImageInspect{ID: "ID"}, nil, nil).
		Once()

	image, err := m.GetDockerImage("ID", nil)
	assert.NoError(t, err)
	assert.NotNil(t, image)
	assert.Equal(t, "ID", image.ID)
}

func TestDockerUnknownPolicyMode(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	dockerConfig := &common.DockerConfig{PullPolicy: []string{"unknown"}}
	m := newDefaultTestManager(c, dockerConfig)

	_, err := m.GetDockerImage("not-existing", nil)
	assert.Error(t, err)
}

func TestDockerPolicyModeNever(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyNever}}
	m := newDefaultTestManager(c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(types.ImageInspect{ID: "existing"}, nil, nil).
		Once()

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(types.ImageInspect{}, nil, os.ErrNotExist).
		Once()

	image, err := m.GetDockerImage("existing", nil)
	assert.NoError(t, err)
	assert.Equal(t, "existing", image.ID)

	_, err = m.GetDockerImage("not-existing", nil)
	assert.Error(t, err)
}

func TestDockerPolicyModeIfNotPresentForExistingImage(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyIfNotPresent}}
	m := newDefaultTestManager(c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	image, err := m.GetDockerImage("existing", nil)
	assert.NoError(t, err)
	assert.NotNil(t, image)
}

func TestDockerPolicyModeIfNotPresentForNotExistingImage(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyIfNotPresent}}
	m := newDefaultTestManager(c, dockerConfig)

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

	image, err := m.GetDockerImage("not-existing", nil)
	assert.NoError(t, err)
	assert.NotNil(t, image)
	assert.True(t, pullImageHookCalled)

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	// It shouldn't execute the pull for second time
	image, err = m.GetDockerImage("not-existing", nil)
	assert.NoError(t, err)
	assert.NotNil(t, image)
}

func TestDockerPolicyModeAlwaysForExistingImage(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyAlways}}
	m := newDefaultTestManager(c, dockerConfig)

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

	image, err := m.GetDockerImage("existing", nil)
	assert.NoError(t, err)
	assert.NotNil(t, image)
	assert.True(t, pullImageHookCalled)
}

func TestDockerPolicyModeAlwaysForLocalOnlyImage(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyAlways}}
	m := newDefaultTestManager(c, dockerConfig)

	pullImageHookCalled := false
	m.onPullImageHookFunc = func() { pullImageHookCalled = true }

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	c.On("ImagePullBlocking", m.context, "existing:latest", buildImagePullOptions()).
		Return(fmt.Errorf("not found")).
		Once()

	image, err := m.GetDockerImage("existing", nil)
	assert.Error(t, err)
	assert.Nil(t, image)
	assert.True(t, pullImageHookCalled)
}

func TestDockerGetExistingDockerImageIfPullFails(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyAlways}}
	m := newDefaultTestManager(c, dockerConfig)

	c.On("ImageInspectWithRaw", m.context, "to-pull").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	options := buildImagePullOptions()
	c.On("ImagePullBlocking", m.context, "to-pull:latest", options).
		Return(os.ErrNotExist).
		Once()

	image, err := m.GetDockerImage("to-pull", nil)
	assert.Error(t, err)
	assert.Nil(t, image, "Forces to authorize pulling")

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(types.ImageInspect{}, nil, os.ErrNotExist).
		Once()

	c.On("ImagePullBlocking", m.context, "not-existing:latest", options).
		Return(os.ErrNotExist).
		Once()

	image, err = m.GetDockerImage("not-existing", nil)
	assert.Error(t, err)
	assert.Nil(t, image, "No existing image")
}

func TestCombinedDockerPolicyModesAlwaysAndIfNotPresentForExistingImage(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	logger, _ := logrustest.NewNullLogger()
	output := bytes.NewBufferString("")
	buildLogger := common.NewBuildLogger(&common.Trace{Writer: output}, logger.WithField("test", t.Name()))

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyAlways, common.PullPolicyIfNotPresent}}
	m := newDefaultTestManager(c, dockerConfig)
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

	image, err := m.GetDockerImage("existing", nil)
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

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyAlways, common.PullPolicyIfNotPresent}}
	m := newDefaultTestManager(c, dockerConfig)

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(types.ImageInspect{}, nil, os.ErrNotExist).
		Once()

	c.On("ImagePullBlocking", m.context, "not-existing:latest", buildImagePullOptions()).
		Return(os.ErrNotExist).
		Twice()

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(types.ImageInspect{}, nil, os.ErrNotExist).
		Once()

	image, err := m.GetDockerImage("not-existing", nil)
	assert.Error(t, err)
	assert.Nil(t, image, "No existing image")
}

func TestPullPolicyWhenAlwaysIsSet(t *testing.T) {
	remoteImage := "registry.domain.tld:5005/image/name:version"
	gitlabImage := "registry.gitlab.tld:1234/image/name:version"

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyAlways}}
	m := newDefaultTestManager(nil, dockerConfig)

	testGetDockerImage(t, m, remoteImage, addPullsRemoteImageExpectations)
	testDeniesDockerImage(t, m, remoteImage, addDeniesPullExpectations)

	testGetDockerImage(t, m, gitlabImage, addPullsRemoteImageExpectations)
	testDeniesDockerImage(t, m, gitlabImage, addDeniesPullExpectations)
}

func TestPullPolicyWhenIfNotPresentIsSet(t *testing.T) {
	remoteImage := "registry.domain.tld:5005/image/name:version"
	gitlabImage := "registry.gitlab.tld:1234/image/name:version"

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyIfNotPresent}}
	m := newDefaultTestManager(nil, dockerConfig)

	testGetDockerImage(t, m, remoteImage, addFindsLocalImageExpectations)
	testGetDockerImage(t, m, gitlabImage, addFindsLocalImageExpectations)
}

func TestPullPolicyPassedAsIfNotPresentForExistingAndConfigAlways(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	dockerConfig := &common.DockerConfig{
		PullPolicy:          []string{common.PullPolicyAlways},
		AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways, common.PullPolicyIfNotPresent},
	}
	m := newDefaultTestManager(c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(types.ImageInspect{ID: "image-id"}, nil, nil).
		Once()

	imagePullPolicies := []common.DockerPullPolicy{common.PullPolicyIfNotPresent}
	image, err := m.GetDockerImage("existing", imagePullPolicies)
	assert.NoError(t, err)
	assert.NotNil(t, image)
}

func TestPullPolicyPassedAsIfNotPresentForNonExistingAndConfigAlways(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	dockerConfig := &common.DockerConfig{
		PullPolicy:          []string{common.PullPolicyAlways},
		AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways, common.PullPolicyIfNotPresent},
	}
	m := newDefaultTestManager(c, dockerConfig)

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

	imagePullPolicies := []common.DockerPullPolicy{common.PullPolicyIfNotPresent}
	image, err := m.GetDockerImage("not-existing", imagePullPolicies)
	assert.NoError(t, err)
	assert.NotNil(t, image)
	assert.True(t, pullImageHookCalled, "image should have been pulled")
}

func TestPullPolicyPassedAsIfNotPresentButNotAllowedDefault(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	dockerConfig := &common.DockerConfig{}
	m := newDefaultTestManager(c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	imagePullPolicies := []common.DockerPullPolicy{common.PullPolicyIfNotPresent}
	_, err := m.GetDockerImage("existing", imagePullPolicies)
	assert.Contains(
		t,
		err.Error(),
		"failed to pull image 'existing'",
	)
	assert.Contains(
		t,
		err.Error(),
		fmt.Sprintf(common.IncompatiblePullPolicy, "[if-not-present]", "GitLab pipeline config", "[always]"),
	)
}

func TestPullPolicyPassedAsIfNotPresentButNotAllowed(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	dockerConfig := &common.DockerConfig{
		AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyNever},
	}
	m := newDefaultTestManager(c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	imagePullPolicies := []common.DockerPullPolicy{common.PullPolicyIfNotPresent}
	_, err := m.GetDockerImage("existing", imagePullPolicies)
	assert.Contains(
		t,
		err.Error(),
		"failed to pull image 'existing'",
	)
	assert.Contains(
		t,
		err.Error(),
		fmt.Sprintf(common.IncompatiblePullPolicy, "[if-not-present]", "GitLab pipeline config", "[never]"),
	)
}

func TestPullPolicyWhenConfigIsNotAllowed(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	dockerConfig := &common.DockerConfig{
		PullPolicy:          []string{common.PullPolicyNever, common.PullPolicyIfNotPresent},
		AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways},
	}
	m := newDefaultTestManager(c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	_, err := m.GetDockerImage("existing", nil)
	assert.Contains(
		t,
		err.Error(),
		"failed to pull image 'existing'",
	)
	assert.Contains(
		t,
		err.Error(),
		fmt.Sprintf(common.IncompatiblePullPolicy, "[never if-not-present]", "Runner config", "[always]"),
	)
}

func TestPullPolicyWhenConfigIsAllowed(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	dockerConfig := &common.DockerConfig{
		PullPolicy:          []string{common.PullPolicyNever},
		AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent, common.PullPolicyNever},
	}
	m := newDefaultTestManager(c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(types.ImageInspect{ID: "existing"}, nil, nil).
		Once()

	image, err := m.GetDockerImage("existing", nil)
	assert.NoError(t, err)
	assert.Equal(t, "existing", image.ID)
}

func TestPullPolicyWhenConfigPullPolicyIsInvalid(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	dockerConfig := &common.DockerConfig{
		PullPolicy:          []string{"invalid"},
		AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways},
	}
	m := newDefaultTestManager(c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	_, err := m.GetDockerImage("existing", nil)
	assert.EqualError(
		t,
		err,
		"unsupported pull_policy config: \"invalid\"",
	)
}

func TestPullPolicyWhenConfigAllowedPullPoliciesIsInvalid(t *testing.T) {
	c := new(docker.MockClient)
	defer c.AssertExpectations(t)

	dockerConfig := &common.DockerConfig{
		PullPolicy:          []string{common.PullPolicyAlways},
		AllowedPullPolicies: []common.DockerPullPolicy{"invalid"},
	}
	m := newDefaultTestManager(c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	_, err := m.GetDockerImage("existing", nil)
	assert.EqualError(
		t,
		err,
		"unsupported allowed_pull_policies config: \"invalid\"",
	)
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

func newDefaultTestManager(client *docker.MockClient, dockerConfig *common.DockerConfig) *manager {
	// Create a unique context value that can be later compared with to ensure
	// that the production code is passing it to the mocks
	ctx := context.WithValue(context.Background(), new(struct{}), "unique context")

	return &manager{
		context: ctx,
		logger:  newLoggerMock(),
		config: ManagerConfig{
			DockerConfig: dockerConfig,
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

		image, err := m.GetDockerImage(imageName, nil)
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

		_, err := m.GetDockerImage(imageName, nil)
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

func Test_manager_getPullPolicies(t *testing.T) {
	m := manager{
		config: ManagerConfig{
			DockerConfig: &common.DockerConfig{},
		},
	}

	tests := map[string]struct {
		imagePullPolicies []common.DockerPullPolicy
		pullPolicy        common.StringOrArray
		want              []common.DockerPullPolicy
	}{
		"gitlab-ci.yaml only": {
			imagePullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
			pullPolicy:        common.StringOrArray{},
			want:              []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
		},
		"config.toml only": {
			imagePullPolicies: []common.DockerPullPolicy{},
			pullPolicy:        common.StringOrArray{common.PullPolicyIfNotPresent},
			want:              []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
		},
		"both": {
			imagePullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
			pullPolicy:        common.StringOrArray{common.PullPolicyNever},
			want:              []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
		},
		"not configured": {
			imagePullPolicies: []common.DockerPullPolicy{},
			pullPolicy:        common.StringOrArray{},
			want:              []common.DockerPullPolicy{common.PullPolicyAlways},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			m.config.DockerConfig.PullPolicy = tt.pullPolicy
			got, err := m.getPullPolicies(tt.imagePullPolicies)

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}
