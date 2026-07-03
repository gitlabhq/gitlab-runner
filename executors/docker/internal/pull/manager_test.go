//go:build !integration

package pull

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/errdefs"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

func TestNewDefaultManager(t *testing.T) {
	m := NewManager(t.Context(), newLoggerMock(t), ManagerConfig{}, docker.NewMockClient(t), nil)
	assert.IsType(t, &manager{}, m)
}

func TestDockerForNamedImage(t *testing.T) {
	c := docker.NewMockClient(t)
	validSHA := "real@sha256:b5bb9d8014a0f9b1d61e21e796d78dccdf1352f23cd32812f4850b878ae4944c"

	dockerConfig := &common.DockerConfig{}
	dockerOptions := spec.ImageDockerOptions{}

	m := newDefaultTestManager(t, c, dockerConfig)

	c.On("ImagePullBlocking", m.context, "test:latest", mock.AnythingOfType("image.PullOptions")).
		Return(os.ErrNotExist).
		Once()

	c.On("ImagePullBlocking", m.context, "tagged:tag", mock.AnythingOfType("image.PullOptions")).
		Return(os.ErrNotExist).
		Once()

	c.On("ImagePullBlocking", m.context, validSHA, mock.AnythingOfType("image.PullOptions")).
		Return(os.ErrNotExist).
		Once()

	image, err := m.pullDockerImage("test", dockerOptions, nil)
	assert.Error(t, err)
	assert.Nil(t, image)

	image, err = m.pullDockerImage("tagged:tag", dockerOptions, nil)
	assert.Error(t, err)
	assert.Nil(t, image)

	image, err = m.pullDockerImage(validSHA, dockerOptions, nil)
	assert.Error(t, err)
	assert.Nil(t, image)
}

func TestDockerForImagePullFailures(t *testing.T) {
	c := docker.NewMockClient(t)
	errTest := errors.New("this is a test")

	dockerConfig := &common.DockerConfig{}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)

	tests := map[string]struct {
		imageName string
		initMock  func(c *docker.MockClient, imageName string)
		assert    func(t *testing.T, m *manager, imageName string)
	}{
		"ImagePullBlocking unwrapped system failure": {
			imageName: "unwrapped-system:failure",
			initMock: func(c *docker.MockClient, imageName string) {
				c.On("ImagePullBlocking", m.context, imageName, mock.AnythingOfType("image.PullOptions")).
					Return(errdefs.System(errTest)).
					Once()
			},
			assert: func(t *testing.T, m *manager, imageName string) {
				var buildError *common.BuildError
				image, err := m.pullDockerImage(imageName, dockerOptions, nil)
				assert.Nil(t, image)
				assert.Error(t, err)
				require.ErrorAs(t, err, &buildError)
				assert.Equal(t, buildError.FailureReason, common.ImagePullFailure)
			},
		},
		"ImagePullBlocking wrapped system failure": {
			imageName: "wrapped-system:failure",
			initMock: func(c *docker.MockClient, imageName string) {
				c.On("ImagePullBlocking", m.context, imageName, mock.AnythingOfType("image.PullOptions")).
					Return(fmt.Errorf("wrapped error: %w", errdefs.System(errTest))).
					Once()
			},
			assert: func(t *testing.T, m *manager, imageName string) {
				var buildError *common.BuildError
				image, err := m.pullDockerImage(imageName, dockerOptions, nil)
				assert.Nil(t, image)
				assert.Error(t, err)
				require.ErrorAs(t, err, &buildError)
				assert.Equal(t, buildError.FailureReason, common.ImagePullFailure)
			},
		},
		"ImagePullBlocking two level wrapped system failure": {
			imageName: "two-level-wrapped-system:failure",
			initMock: func(c *docker.MockClient, imageName string) {
				c.On("ImagePullBlocking", m.context, imageName, mock.AnythingOfType("image.PullOptions")).
					Return(fmt.Errorf("wrapped error: %w", fmt.Errorf("wrapped error: %w", errdefs.System(errTest)))).
					Once()
			},
			assert: func(t *testing.T, m *manager, imageName string) {
				var buildError *common.BuildError
				image, err := m.pullDockerImage(imageName, dockerOptions, nil)
				assert.Nil(t, image)
				assert.Error(t, err)
				require.ErrorAs(t, err, &buildError)
				assert.Equal(t, buildError.FailureReason, common.ImagePullFailure)
			},
		},
		"ImagePullBlocking wrapped transient failure is retried then fails": {
			imageName: "wrapped-request-timeout:failure",
			initMock: func(c *docker.MockClient, imageName string) {
				c.On("ImagePullBlocking", m.context, imageName, mock.AnythingOfType("image.PullOptions")).
					Return(fmt.Errorf(
						"wrapped error: %w", errdefs.System(errors.New(
							"request canceled while waiting for connection",
						)))).
					Times(defaultPullMaxAttempts)
			},
			assert: func(t *testing.T, m *manager, imageName string) {
				var buildError *common.BuildError
				image, err := m.pullDockerImage(imageName, dockerOptions, nil)
				assert.Nil(t, image)
				assert.Error(t, err)
				require.ErrorAs(t, err, &buildError)
				assert.Equal(t, buildError.FailureReason, common.RunnerExternalDependencyFailure)
			},
		},
		"ImagePullBlocking two level wrapped transient failure is retried then fails": {
			imageName: "lwo-level-wrapped-request-timeout:failure",
			initMock: func(c *docker.MockClient, imageName string) {
				c.On("ImagePullBlocking", m.context, imageName, mock.AnythingOfType("image.PullOptions")).
					Return(fmt.Errorf(
						"wrapped error: %w", fmt.Errorf(
							"wrapped error: %w", errdefs.System(errors.New(
								"request canceled while waiting for connection",
							))))).
					Times(defaultPullMaxAttempts)
			},
			assert: func(t *testing.T, m *manager, imageName string) {
				var buildError *common.BuildError
				image, err := m.pullDockerImage(imageName, dockerOptions, nil)
				assert.Nil(t, image)
				assert.Error(t, err)
				require.ErrorAs(t, err, &buildError)
				assert.Equal(t, buildError.FailureReason, common.RunnerExternalDependencyFailure)
			},
		},
		"ImagePullBlocking unwrapped script failure": {
			imageName: "unwrapped-script:failure",
			initMock: func(c *docker.MockClient, imageName string) {
				c.On("ImagePullBlocking", m.context, imageName, mock.AnythingOfType("image.PullOptions")).
					Return(errdefs.NotFound(errTest)).
					Once()
			},
			assert: func(t *testing.T, m *manager, imageName string) {
				var buildError *common.BuildError
				image, err := m.pullDockerImage(imageName, dockerOptions, nil)
				assert.Nil(t, image)
				assert.Error(t, err)
				require.ErrorAs(t, err, &buildError)
				assert.Equal(t, buildError.FailureReason, common.ImagePullFailure)
			},
		},
		"ImagePullBlocking wrapped script failure": {
			imageName: "wrapped-script:failure",
			initMock: func(c *docker.MockClient, imageName string) {
				c.On("ImagePullBlocking", m.context, imageName, mock.AnythingOfType("image.PullOptions")).
					Return(fmt.Errorf("wrapped error: %w", errdefs.NotFound(errTest))).
					Once()
			},
			assert: func(t *testing.T, m *manager, imageName string) {
				var buildError *common.BuildError
				image, err := m.pullDockerImage(imageName, dockerOptions, nil)
				assert.Nil(t, image)
				assert.Error(t, err)
				require.ErrorAs(t, err, &buildError)
				assert.Equal(t, buildError.FailureReason, common.ImagePullFailure)
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			tc.initMock(c, tc.imageName)
			tc.assert(t, m, tc.imageName)
		})
	}
}

func TestDockerPullRetriesTransientFailureThenSucceeds(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)

	// First pull fails with a transient registry/auth timeout, second succeeds.
	c.On("ImagePullBlocking", m.context, "flaky:latest", mock.AnythingOfType("image.PullOptions")).
		Return(errors.New(
			`Get "https://gitlab.com/jwt/auth": context deadline exceeded ` +
				`(Client.Timeout exceeded while awaiting headers)`,
		)).
		Once()
	c.On("ImagePullBlocking", m.context, "flaky:latest", mock.AnythingOfType("image.PullOptions")).
		Return(nil).
		Once()
	c.On("ImageInspectWithRaw", m.context, "flaky").
		Return(image.InspectResponse{ID: "image-id"}, nil, nil).
		Once()

	img, err := m.pullDockerImage("flaky", dockerOptions, nil)
	assert.NoError(t, err)
	require.NotNil(t, img)
	assert.Equal(t, "image-id", img.ID)
}

func TestDockerPullDoesNotRetryNonTransientFailure(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)

	// Auth-denied is not transient and must not be retried (single call only).
	c.On("ImagePullBlocking", m.context, "denied:latest", mock.AnythingOfType("image.PullOptions")).
		Return(errors.New("pull access denied, repository does not exist or may require authorization")).
		Once()

	var buildError *common.BuildError
	img, err := m.pullDockerImage("denied", dockerOptions, nil)
	assert.Nil(t, img)
	require.ErrorAs(t, err, &buildError)
	assert.Equal(t, common.ConfigurationError, buildError.FailureReason)
}

func TestDockerPullStopsRetryingWhenContextCancelled(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)

	ctx, cancel := context.WithCancel(m.context)
	m.context = ctx

	// A transient failure would normally be retried. Cancelling the build
	// context during the first attempt must stop the loop. The Once() is the
	// assertion: a second pull means cancellation was ignored.
	c.On("ImagePullBlocking", m.context, "flaky:latest", mock.AnythingOfType("image.PullOptions")).
		Run(func(mock.Arguments) { cancel() }).
		Return(errors.New(
			`Get "https://gitlab.com/jwt/auth": context deadline exceeded ` +
				`(Client.Timeout exceeded while awaiting headers)`,
		)).
		Once()

	img, err := m.pullDockerImage("flaky", dockerOptions, nil)
	assert.Nil(t, img)

	// Cancellation during a pull attempt is classified as JobCanceled (not an
	// image-pull failure) and a JobCanceled reason is never retried, so the
	// loop stops after the single attempt asserted by Once() above.
	var buildError *common.BuildError
	require.ErrorAs(t, err, &buildError)
	assert.Equal(t, common.JobCanceled, buildError.FailureReason)
}

func TestDockerForExistingImage(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)
	c.On("ImagePullBlocking", m.context, "existing:latest", mock.AnythingOfType("image.PullOptions")).
		Return(nil).
		Once()

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(image.InspectResponse{ID: "image-id"}, nil, nil).
		Once()

	image, err := m.pullDockerImage("existing", dockerOptions, nil)
	assert.NoError(t, err)
	assert.NotNil(t, image)
}

func TestDockerInspectAfterPullFailure(t *testing.T) {
	dockerConfig := &common.DockerConfig{}
	dockerOptions := spec.ImageDockerOptions{}

	tests := map[string]struct {
		inspectErr            error
		expectedFailureReason spec.JobFailureReason
	}{
		"network-shaped error maps to RunnerExternalDependencyFailure": {
			inspectErr:            errors.New("dial tcp: lookup registry: no such host"),
			expectedFailureReason: common.RunnerExternalDependencyFailure,
		},
		"default error maps to ImagePullFailure": {
			inspectErr:            errors.New("daemon unavailable"),
			expectedFailureReason: common.ImagePullFailure,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			c := docker.NewMockClient(t)
			m := newDefaultTestManager(t, c, dockerConfig)

			c.On("ImagePullBlocking", m.context, "post-pull-inspect:latest", mock.AnythingOfType("image.PullOptions")).
				Return(nil).
				Once()
			c.On("ImageInspectWithRaw", m.context, "post-pull-inspect").
				Return(image.InspectResponse{}, nil, tc.inspectErr).
				Once()

			img, err := m.pullDockerImage("post-pull-inspect", dockerOptions, nil)
			assert.Nil(t, img)
			assert.Error(t, err)

			var buildError *common.BuildError
			require.ErrorAs(t, err, &buildError)
			assert.Equal(t, tc.expectedFailureReason, buildError.FailureReason)
		})
	}
}

func TestDockerPullContextCancellation(t *testing.T) {
	dockerConfig := &common.DockerConfig{}
	dockerOptions := spec.ImageDockerOptions{}

	tests := map[string]struct {
		setupContext          func(t *testing.T) context.Context
		expectedFailureReason spec.JobFailureReason
	}{
		"canceled context produces JobCanceled": {
			setupContext: func(t *testing.T) context.Context {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				return ctx
			},
			expectedFailureReason: common.JobCanceled,
		},
		"deadline-exceeded context produces JobExecutionTimeout": {
			setupContext: func(t *testing.T) context.Context {
				ctx, cancel := context.WithDeadline(t.Context(), time.Unix(0, 0))
				t.Cleanup(cancel)
				return ctx
			},
			expectedFailureReason: common.JobExecutionTimeout,
		},
		"WithDeadlineCause + non-DeadlineExceeded custom cause still maps to JobExecutionTimeout": {
			setupContext: func(t *testing.T) context.Context {
				ctx, cancel := context.WithDeadlineCause(
					t.Context(),
					time.Unix(0, 0),
					errors.New("custom timeout cause that does not wrap context.DeadlineExceeded"),
				)
				t.Cleanup(cancel)
				return ctx
			},
			expectedFailureReason: common.JobExecutionTimeout,
		},
	}

	for tn, tc := range tests {
		t.Run("pull fails: "+tn, func(t *testing.T) {
			c := docker.NewMockClient(t)
			m := newDefaultTestManager(t, c, dockerConfig)
			m.context = tc.setupContext(t)

			c.On("ImagePullBlocking", m.context, "canceled-pull:latest", mock.AnythingOfType("image.PullOptions")).
				Return(context.Canceled).
				Once()

			img, err := m.pullDockerImage("canceled-pull", dockerOptions, nil)
			assert.Nil(t, img)
			assert.Error(t, err)

			var buildError *common.BuildError
			require.ErrorAs(t, err, &buildError)
			assert.Equal(t, tc.expectedFailureReason, buildError.FailureReason)
		})

		t.Run("inspect fails: "+tn, func(t *testing.T) {
			c := docker.NewMockClient(t)
			m := newDefaultTestManager(t, c, dockerConfig)
			m.context = tc.setupContext(t)

			c.On("ImagePullBlocking", m.context, "canceled-inspect:latest", mock.AnythingOfType("image.PullOptions")).
				Return(nil).
				Once()
			c.On("ImageInspectWithRaw", m.context, "canceled-inspect").
				Return(image.InspectResponse{}, nil, context.Canceled).
				Once()

			img, err := m.pullDockerImage("canceled-inspect", dockerOptions, nil)
			assert.Nil(t, img)
			assert.Error(t, err)

			var buildError *common.BuildError
			require.ErrorAs(t, err, &buildError)
			assert.Equal(t, tc.expectedFailureReason, buildError.FailureReason)
		})
	}
}

func TestDockerGetImageById(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{}
	dockerOptions := spec.ImageDockerOptions{}

	m := newDefaultTestManager(t, c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	c.On("ImageInspectWithRaw", m.context, "ID").
		Return(image.InspectResponse{ID: "ID"}, nil, nil).
		Once()

	image, err := m.GetDockerImage("ID", dockerOptions, nil)
	assert.NoError(t, err)
	assert.NotNil(t, image)
	assert.Equal(t, "ID", image.ID)
}

func TestDockerUnknownPolicyMode(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{PullPolicy: []string{"unknown"}}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)

	_, err := m.GetDockerImage("not-existing", dockerOptions, nil)
	assert.Error(t, err)
}

func TestDockerPolicyModeNever(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyNever}}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(image.InspectResponse{ID: "existing"}, nil, nil).
		Once()

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(image.InspectResponse{}, nil, os.ErrNotExist).
		Once()

	image, err := m.GetDockerImage("existing", dockerOptions, nil)
	assert.NoError(t, err)
	assert.Equal(t, "existing", image.ID)

	_, err = m.GetDockerImage("not-existing", dockerOptions, nil)
	assert.Error(t, err)
}

func TestDockerPolicyModeIfNotPresentForExistingImage(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyIfNotPresent}}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(image.InspectResponse{ID: "image-id"}, nil, nil).
		Once()

	image, err := m.GetDockerImage("existing", dockerOptions, nil)
	assert.NoError(t, err)
	assert.NotNil(t, image)
}

func TestDockerPolicyModeIfNotPresentForNotExistingImage(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyIfNotPresent}}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)

	pullImageHookCalled := false
	m.onPullImageHookFunc = func() { pullImageHookCalled = true }

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(image.InspectResponse{}, nil, os.ErrNotExist).
		Once()

	c.On("ImagePullBlocking", m.context, "not-existing:latest", mock.AnythingOfType("image.PullOptions")).
		Return(nil).
		Once()

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(image.InspectResponse{ID: "image-id"}, nil, nil).
		Once()

	img, err := m.GetDockerImage("not-existing", dockerOptions, nil)
	assert.NoError(t, err)
	assert.NotNil(t, img)
	assert.True(t, pullImageHookCalled)

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(image.InspectResponse{ID: "image-id"}, nil, nil).
		Once()

	// It shouldn't execute the pull for second time
	img, err = m.GetDockerImage("not-existing", dockerOptions, nil)
	assert.NoError(t, err)
	assert.NotNil(t, img)
}

func TestDockerPolicyModeAlwaysForExistingImage(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyAlways}}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)

	pullImageHookCalled := false
	m.onPullImageHookFunc = func() { pullImageHookCalled = true }

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(image.InspectResponse{ID: "image-id"}, nil, nil).
		Once()

	c.On("ImagePullBlocking", m.context, "existing:latest", mock.AnythingOfType("image.PullOptions")).
		Return(nil).
		Once()

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(image.InspectResponse{ID: "image-id"}, nil, nil).
		Once()

	image, err := m.GetDockerImage("existing", dockerOptions, nil)
	assert.NoError(t, err)
	assert.NotNil(t, image)
	assert.True(t, pullImageHookCalled)
}

func TestDockerPolicyModeAlwaysForLocalOnlyImage(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyAlways}}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)

	pullImageHookCalled := false
	m.onPullImageHookFunc = func() { pullImageHookCalled = true }

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(image.InspectResponse{ID: "image-id"}, nil, nil).
		Once()

	c.On("ImagePullBlocking", m.context, "existing:latest", mock.AnythingOfType("image.PullOptions")).
		Return(fmt.Errorf("not found")).
		Once()

	image, err := m.GetDockerImage("existing", dockerOptions, nil)
	assert.Error(t, err)
	assert.Nil(t, image)
	assert.True(t, pullImageHookCalled)
}

func TestDockerGetExistingDockerImageIfPullFails(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyAlways}}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)

	c.On("ImageInspectWithRaw", m.context, "to-pull").
		Return(image.InspectResponse{ID: "image-id"}, nil, nil).
		Once()

	c.On("ImagePullBlocking", m.context, "to-pull:latest", mock.AnythingOfType("image.PullOptions")).
		Return(os.ErrNotExist).
		Once()

	img, err := m.GetDockerImage("to-pull", dockerOptions, nil)
	assert.Error(t, err)
	assert.Nil(t, img, "Forces to authorize pulling")

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(image.InspectResponse{}, nil, os.ErrNotExist).
		Once()

	c.On("ImagePullBlocking", m.context, "not-existing:latest", mock.AnythingOfType("image.PullOptions")).
		Return(os.ErrNotExist).
		Once()

	img, err = m.GetDockerImage("not-existing", dockerOptions, nil)
	assert.Error(t, err)
	assert.Nil(t, img, "No existing image")
}

func TestCombinedDockerPolicyModesAlwaysAndIfNotPresentForExistingImage(t *testing.T) {
	c := docker.NewMockClient(t)

	logger, _ := logrustest.NewNullLogger()
	output := bytes.NewBufferString("")
	buildLogger := buildlogger.New(&common.Trace{Writer: output}, logger.WithField("test", t.Name()), buildlogger.Options{})

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyAlways, common.PullPolicyIfNotPresent}}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)
	m.logger = &buildLogger

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(image.InspectResponse{ID: "image-id"}, nil, nil).
		Once()

	c.On("ImagePullBlocking", m.context, "existing:latest", mock.AnythingOfType("image.PullOptions")).
		Return(errors.New("received unexpected HTTP status: 502 Bad Gateway")).
		Once()

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(image.InspectResponse{ID: "local-image-id"}, nil, nil).
		Once()

	image, err := m.GetDockerImage("existing", dockerOptions, nil)
	assert.NoError(t, err)
	assert.Contains(t, output.String(), `WARNING: Failed to pull image with policy "always": `+
		`received unexpected HTTP status: 502 Bad Gateway`)
	assert.Contains(t, output.String(), `Attempt #2: Trying "if-not-present" pull policy`)
	assert.Contains(t, output.String(), `Using locally found image version due to "if-not-present" pull policy`)
	require.NotNil(t, image)
	assert.Equal(t, "local-image-id", image.ID)
}

func TestCombinedDockerPolicyModeAlwaysAndIfNotPresentForNonExistingImage(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyAlways, common.PullPolicyIfNotPresent}}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(image.InspectResponse{}, nil, os.ErrNotExist).
		Once()

	c.On("ImagePullBlocking", m.context, "not-existing:latest", mock.AnythingOfType("image.PullOptions")).
		Return(os.ErrNotExist).
		Twice()

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(image.InspectResponse{}, nil, os.ErrNotExist).
		Once()

	image, err := m.GetDockerImage("not-existing", dockerOptions, nil)
	assert.Error(t, err)
	assert.Nil(t, image, "No existing image")
}

func TestPullPolicyWhenAlwaysIsSet(t *testing.T) {
	remoteImage := "registry.domain.tld:5005/image/name:version"
	gitlabImage := "registry.gitlab.tld:1234/image/name:version"

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyAlways}}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, nil, dockerConfig)

	testGetDockerImage(t, m, remoteImage, dockerOptions, addPullsRemoteImageExpectations)
	testDeniesDockerImage(t, m, remoteImage, dockerOptions, addDeniesPullExpectations)

	testGetDockerImage(t, m, gitlabImage, dockerOptions, addPullsRemoteImageExpectations)
	testDeniesDockerImage(t, m, gitlabImage, dockerOptions, addDeniesPullExpectations)
}

func TestPullPolicyWhenIfNotPresentIsSet(t *testing.T) {
	remoteImage := "registry.domain.tld:5005/image/name:version"
	gitlabImage := "registry.gitlab.tld:1234/image/name:version"

	dockerConfig := &common.DockerConfig{PullPolicy: []string{common.PullPolicyIfNotPresent}}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, nil, dockerConfig)

	testGetDockerImage(t, m, remoteImage, dockerOptions, addFindsLocalImageExpectations)
	testGetDockerImage(t, m, gitlabImage, dockerOptions, addFindsLocalImageExpectations)
}

func TestPullPolicyPassedAsIfNotPresentForExistingAndConfigAlways(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{
		PullPolicy:          []string{common.PullPolicyAlways},
		AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways, common.PullPolicyIfNotPresent},
	}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(image.InspectResponse{ID: "image-id"}, nil, nil).
		Once()

	imagePullPolicies := []common.DockerPullPolicy{common.PullPolicyIfNotPresent}
	image, err := m.GetDockerImage("existing", dockerOptions, imagePullPolicies)
	assert.NoError(t, err)
	assert.NotNil(t, image)
}

func TestPullPolicyPassedAsIfNotPresentForNonExistingAndConfigAlways(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{
		PullPolicy:          []string{common.PullPolicyAlways},
		AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways, common.PullPolicyIfNotPresent},
	}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)

	pullImageHookCalled := false
	m.onPullImageHookFunc = func() { pullImageHookCalled = true }

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(image.InspectResponse{}, nil, os.ErrNotExist).
		Once()

	c.On("ImagePullBlocking", m.context, "not-existing:latest", mock.AnythingOfType("image.PullOptions")).
		Return(nil).
		Once()

	c.On("ImageInspectWithRaw", m.context, "not-existing").
		Return(image.InspectResponse{ID: "image-id"}, nil, nil).
		Once()

	imagePullPolicies := []common.DockerPullPolicy{common.PullPolicyIfNotPresent}
	image, err := m.GetDockerImage("not-existing", dockerOptions, imagePullPolicies)
	assert.NoError(t, err)
	assert.NotNil(t, image)
	assert.True(t, pullImageHookCalled, "image should have been pulled")
}

func TestPullPolicyPassedAsIfNotPresentButNotAllowedDefault(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	imagePullPolicies := []common.DockerPullPolicy{common.PullPolicyIfNotPresent}
	_, err := m.GetDockerImage("existing", dockerOptions, imagePullPolicies)
	assert.Contains(
		t,
		err.Error(),
		`invalid pull policy for image "existing"`,
	)
	assert.Regexp(t, regexp.MustCompile(`if-not-present.* GitLab pipeline config .*always`), err.Error())
}

func TestPullPolicyPassedAsIfNotPresentButNotAllowed(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{
		AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyNever},
	}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	imagePullPolicies := []common.DockerPullPolicy{common.PullPolicyIfNotPresent}
	_, err := m.GetDockerImage("existing", dockerOptions, imagePullPolicies)
	assert.Contains(
		t,
		err.Error(),
		`invalid pull policy for image "existing"`,
	)
	assert.Regexp(t, regexp.MustCompile(`if-not-present.* GitLab pipeline config .*never`), err.Error())
}

func TestPullPolicyWhenConfigIsNotAllowed(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{
		PullPolicy:          []string{common.PullPolicyNever, common.PullPolicyIfNotPresent},
		AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways},
	}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	_, err := m.GetDockerImage("existing", dockerOptions, nil)
	assert.Contains(
		t,
		err.Error(),
		`invalid pull policy for image "existing"`,
	)
	assert.Regexp(t, regexp.MustCompile(`never if-not-present.* Runner config .*always`), err.Error())
}

func TestPullPolicyWhenConfigIsAllowed(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{
		PullPolicy:          []string{common.PullPolicyNever},
		AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent, common.PullPolicyNever},
	}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	c.On("ImageInspectWithRaw", m.context, "existing").
		Return(image.InspectResponse{ID: "existing"}, nil, nil).
		Once()

	image, err := m.GetDockerImage("existing", dockerOptions, nil)
	assert.NoError(t, err)
	assert.Equal(t, "existing", image.ID)
}

func TestPullPolicyWhenConfigPullPolicyIsInvalid(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{
		PullPolicy:          []string{"invalid"},
		AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways},
	}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	_, err := m.GetDockerImage("existing", dockerOptions, nil)
	assert.EqualError(
		t,
		err,
		"unsupported pull_policy config: \"invalid\"",
	)
}

func TestPullPolicyWhenConfigAllowedPullPoliciesIsInvalid(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{
		PullPolicy:          []string{common.PullPolicyAlways},
		AllowedPullPolicies: []common.DockerPullPolicy{"invalid"},
	}
	dockerOptions := spec.ImageDockerOptions{}
	m := newDefaultTestManager(t, c, dockerConfig)
	m.onPullImageHookFunc = func() { assert.Fail(t, "image should not be pulled") }

	_, err := m.GetDockerImage("existing", dockerOptions, nil)
	assert.EqualError(
		t,
		err,
		"unsupported allowed_pull_policies config: \"invalid\"",
	)
}

func newLoggerMock(t *testing.T) *mockPullLogger {
	loggerMock := newMockPullLogger(t)
	loggerMock.On(
		"Debugln",
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
		mock.AnythingOfType("string"),
	).Maybe()
	loggerMock.On("Infoln", mock.AnythingOfType("string")).Maybe()
	loggerMock.On("Warningln", mock.AnythingOfType("string")).Maybe()
	loggerMock.On("Println", mock.AnythingOfType("string"), mock.Anything).Maybe()
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

func newDefaultTestManager(t *testing.T, client *docker.MockClient, dockerConfig *common.DockerConfig) *manager {
	// Create a unique context value that can be later compared with to ensure
	// that the production code is passing it to the mocks
	ctx := context.WithValue(t.Context(), new(struct{}), "unique context")

	return &manager{
		context: ctx,
		logger:  newLoggerMock(t),
		config: ManagerConfig{
			DockerConfig: dockerConfig,
		},
		client: client,
		// Mirror the production retry behaviour but with a negligible backoff so
		// retry-exercising tests don't sleep.
		pullMaxAttempts:     defaultPullMaxAttempts,
		pullRetryMinBackoff: time.Millisecond,
		pullRetryMaxBackoff: time.Millisecond,
	}
}

func testGetDockerImage(
	t *testing.T,
	m *manager,
	imageName string,
	dockerOptions spec.ImageDockerOptions,
	setClientExpectations func(c *docker.MockClient, imageName string),
) {
	t.Run("get:"+imageName, func(t *testing.T) {
		c := docker.NewMockClient(t)

		m.client = c

		setClientExpectations(c, imageName)

		image, err := m.GetDockerImage(imageName, dockerOptions, nil)
		assert.NoError(t, err, "Should not generate error")
		assert.Equal(t, "this-image", image.ID, "Image ID")
	})
}

func testDeniesDockerImage(
	t *testing.T,
	m *manager,
	imageName string,
	dockerOptions spec.ImageDockerOptions,
	setClientExpectations func(c *docker.MockClient, imageName string),
) {
	t.Run("deny:"+imageName, func(t *testing.T) {
		c := docker.NewMockClient(t)

		m.client = c

		setClientExpectations(c, imageName)

		_, err := m.GetDockerImage(imageName, dockerOptions, nil)
		assert.Error(t, err, "Should generate error")
	})
}

func addFindsLocalImageExpectations(c *docker.MockClient, imageName string) {
	c.On("ImageInspectWithRaw", mock.Anything, imageName).
		Return(image.InspectResponse{ID: "this-image"}, nil, nil).
		Once()
}

func addPullsRemoteImageExpectations(c *docker.MockClient, imageName string) {
	c.On("ImageInspectWithRaw", mock.Anything, imageName).
		Return(image.InspectResponse{ID: "not-this-image"}, nil, nil).
		Once()

	c.On("ImagePullBlocking", mock.Anything, imageName, mock.AnythingOfType("image.PullOptions")).
		Return(nil).
		Once()

	c.On("ImageInspectWithRaw", mock.Anything, imageName).
		Return(image.InspectResponse{ID: "this-image"}, nil, nil).
		Once()
}

func addDeniesPullExpectations(c *docker.MockClient, imageName string) {
	c.On("ImageInspectWithRaw", mock.Anything, imageName).
		Return(image.InspectResponse{ID: "image"}, nil, nil).
		Once()

	c.On("ImagePullBlocking", mock.Anything, imageName, mock.AnythingOfType("image.PullOptions")).
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

func TestDockerGetImagePlatformSuccess(t *testing.T) {
	c := docker.NewMockClient(t)

	dockerConfig := &common.DockerConfig{}
	dockerOptions := spec.ImageDockerOptions{}
	dockerOptions.Platform = "arm64/v8"

	m := newDefaultTestManager(t, c, dockerConfig)

	c.On("ImagePullBlocking", m.context, "test:latest", mock.AnythingOfType("image.PullOptions")).
		Return(nil).
		Once()

	c.On("ImageInspectWithRaw", m.context, "test").
		Return(image.InspectResponse{Architecture: "arm64/v8"}, nil, nil).
		Once()

	image, err := m.pullDockerImage("test", dockerOptions, nil)
	assert.NoError(t, err)
	assert.NotNil(t, image)
	assert.Equal(t, "arm64/v8", image.Architecture)
}

func TestGetDockerImageWithPlatform(t *testing.T) {
	remoteImage := "registry.domain.tld:5005/image/name:version"

	dockerConfig := &common.DockerConfig{}
	dockerOptions := spec.ImageDockerOptions{Platform: "foo/bar"}
	m := newDefaultTestManager(t, nil, dockerConfig)

	testGetDockerImage(t, m, remoteImage, dockerOptions, addPullsRemoteImageExpectations)
}

func TestResolveAuthConfigForImageErrorsOnPathTraversal(t *testing.T) {
	loggerMock := newMockPullLogger(t)
	loggerMock.On("Debugln", mock.Anything, mock.Anything, mock.Anything).Maybe()

	m := &manager{
		context: t.Context(),
		logger:  loggerMock,
		config: ManagerConfig{
			DockerConfig: &common.DockerConfig{},
			AuthConfig:   `{"credsStore": "../../usr/bin/sudo"}`,
		},
	}

	authConfig, err := m.resolveAuthConfigForImage("registry.domain.tld:5005/image/name:version")
	assert.ErrorContains(t, err, "path traversal")
	assert.Nil(t, authConfig)
}

func TestResolveAuthConfigForImageWarnsMissingCredentialHelper(t *testing.T) {
	loggerMock := newMockPullLogger(t)
	loggerMock.On("Debugln", mock.Anything, mock.Anything, mock.Anything).Maybe()
	loggerMock.On("Warningln", mock.MatchedBy(func(msg string) bool {
		return strings.Contains(msg, "$DOCKER_AUTH_CONFIG") &&
			strings.Contains(msg, "Credentials from this source will not be used")
	})).Once()

	m := &manager{
		context: t.Context(),
		logger:  loggerMock,
		config: ManagerConfig{
			DockerConfig: &common.DockerConfig{},
			AuthConfig:   `{"credsStore": "nonexistent-helper"}`,
		},
	}

	authConfig, err := m.resolveAuthConfigForImage("registry.domain.tld:5005/image/name:version")
	assert.NoError(t, err)
	assert.Nil(t, authConfig)
}
