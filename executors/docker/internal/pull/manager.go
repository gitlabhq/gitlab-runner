package pull

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	cli "github.com/docker/cli/cli/config/types"
	"github.com/docker/docker/api/types/image"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/auth"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/pull_policies"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/retry"
)

const (
	// defaultPullMaxAttempts bounds how many times a single pull-policy attempt
	// retries the underlying docker pull when it fails with a transient
	// (network/registry) error. Non-transient failures (missing image, auth
	// denied) are not retried.
	defaultPullMaxAttempts     = 3
	defaultPullRetryMinBackoff = 2 * time.Second
	defaultPullRetryMaxBackoff = 10 * time.Second
)

type Manager interface {
	GetDockerImage(imageName string, options spec.ImageDockerOptions, imagePullPolicies []common.DockerPullPolicy,
	) (*image.InspectResponse, error)
}

type ManagerConfig struct {
	DockerConfig *common.DockerConfig
	AuthConfig   string
	ShellUser    string
	Credentials  []spec.Credentials
}

type pullLogger interface {
	Debugln(args ...interface{})
	Infoln(args ...interface{})
	Warningln(args ...interface{})
	Println(args ...interface{})
}

type manager struct {
	usedImages     map[string]string
	usedImagesLock sync.Mutex

	context             context.Context
	config              ManagerConfig
	client              docker.Client
	onPullImageHookFunc func()

	pullMaxAttempts     int
	pullRetryMinBackoff time.Duration
	pullRetryMaxBackoff time.Duration

	logger pullLogger
}

func NewManager(
	ctx context.Context,
	logger pullLogger,
	config ManagerConfig,
	client docker.Client,
	onPullImageHookFunc func(),
) Manager {
	return &manager{
		context:             ctx,
		client:              client,
		config:              config,
		logger:              logger,
		onPullImageHookFunc: onPullImageHookFunc,
		pullMaxAttempts:     defaultPullMaxAttempts,
		pullRetryMinBackoff: defaultPullRetryMinBackoff,
		pullRetryMaxBackoff: defaultPullRetryMaxBackoff,
	}
}

func (m *manager) GetDockerImage(
	imageName string, options spec.ImageDockerOptions,
	imagePullPolicies []common.DockerPullPolicy,
) (*image.InspectResponse, error) {
	pullPolicies, err := m.getPullPolicies(imagePullPolicies)
	if err != nil {
		return nil, err
	}

	allowedPullPolicies, err := m.config.DockerConfig.GetAllowedPullPolicies()
	if err != nil {
		return nil, err
	}

	pullPolicies, err = pull_policies.ComputeEffectivePullPolicies(
		pullPolicies, allowedPullPolicies, imagePullPolicies, m.config.DockerConfig.PullPolicy)
	if err != nil {
		return nil, &common.BuildError{
			Inner:         fmt.Errorf("invalid pull policy for image %q: %w", imageName, err),
			FailureReason: common.ConfigurationError,
		}
	}

	m.logger.Println(fmt.Sprintf("Using effective pull policy of %s for container %s", pullPolicies, imageName))

	var imageErr error
	for idx, pullPolicy := range pullPolicies {
		attempt := 1 + idx
		if attempt > 1 {
			m.logger.Infoln(fmt.Sprintf("Attempt #%d: Trying %q pull policy", attempt, pullPolicy))
		}

		var img *image.InspectResponse
		img, imageErr = m.getImageUsingPullPolicy(imageName, options, pullPolicy)
		if imageErr != nil {
			m.logger.Warningln(fmt.Sprintf("Failed to pull image with policy %q: %v", pullPolicy, imageErr))
			continue
		}

		m.markImageAsUsed(imageName, img)

		return img, nil
	}

	return nil, fmt.Errorf(
		"failed to pull image %q with specified policies %v: %w",
		imageName,
		pullPolicies,
		imageErr,
	)
}

func (m *manager) wasImageUsed(imageName, imageID string) bool {
	m.usedImagesLock.Lock()
	defer m.usedImagesLock.Unlock()

	return m.usedImages[imageName] == imageID
}

func (m *manager) markImageAsUsed(imageName string, image *image.InspectResponse) {
	m.usedImagesLock.Lock()
	defer m.usedImagesLock.Unlock()

	if m.usedImages == nil {
		m.usedImages = make(map[string]string)
	}
	m.usedImages[imageName] = image.ID

	if imageName == image.ID {
		return
	}

	if len(image.RepoDigests) > 0 {
		m.logger.Println("Using docker image", image.ID, "for", imageName, "with digest", image.RepoDigests[0], "...")
	} else {
		m.logger.Println("Using docker image", image.ID, "for", imageName, "...")
	}
}

func (m *manager) getImageUsingPullPolicy(
	imageName string, options spec.ImageDockerOptions,
	pullPolicy common.DockerPullPolicy,
) (*image.InspectResponse, error) {
	m.logger.Debugln("Looking for image", imageName, "...")
	existingImage, _, err := m.client.ImageInspectWithRaw(m.context, imageName)

	// Return early if we already used that image
	if err == nil && m.wasImageUsed(imageName, existingImage.ID) {
		return &existingImage, nil
	}

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
			m.logger.Println(fmt.Sprintf("Using locally found image version due to %q pull policy", pullPolicy))
			return &existingImage, err
		}
	}

	authConfig, err := m.resolveAuthConfigForImage(imageName)
	if err != nil {
		return nil, err
	}

	return m.pullDockerImage(imageName, options, authConfig)
}

func (m *manager) resolveAuthConfigForImage(imageName string) (*cli.AuthConfig, error) {
	registryInfo, err := auth.Resolver{}.ConfigForImage(
		imageName,
		m.config.AuthConfig,
		m.config.ShellUser,
		m.config.Credentials,
		m.logger,
	)
	if err != nil {
		return nil, err
	}

	if registryInfo == nil {
		m.logger.Debugln(fmt.Sprintf("No credentials found for %v", imageName))
		return nil, nil
	}

	authConfig := &registryInfo.AuthConfig
	m.logger.Println(fmt.Sprintf("Authenticating with credentials from %v", registryInfo.Source))
	m.logger.Debugln(fmt.Sprintf(
		"Using %v to connect to %v in order to resolve %v...",
		authConfig.Username,
		authConfig.ServerAddress,
		imageName,
	))
	return authConfig, nil
}

func (m *manager) pullDockerImage(imageName string, options spec.ImageDockerOptions, ac *cli.AuthConfig) (*image.InspectResponse, error) {
	if m.onPullImageHookFunc != nil {
		m.onPullImageHookFunc()
	}
	msg := "Pulling docker image %s ..."
	if options.Platform == "" {
		msg = fmt.Sprintf(msg, imageName)
	} else {
		msg = fmt.Sprintf(msg, imageName+" for platform "+options.Platform)
	}
	m.logger.Println(msg)

	ref := imageName
	// Add :latest to limit the download results
	if !strings.ContainsAny(ref, ":@") {
		ref += ":latest"
	}

	opts := image.PullOptions{
		Platform: options.Platform,
	}

	var err error
	if opts.RegistryAuth, err = auth.EncodeConfig(ac); err != nil {
		return nil, &common.BuildError{Inner: err, FailureReason: common.RunnerSystemFailure}
	}

	// Retry the pull on transient (network/registry) failures such as HTTP
	// client timeouts talking to the registry auth endpoint. Non-transient
	// failures (missing image, auth denied) are returned immediately so they
	// still fail fast and fall through to the next configured pull policy. A
	// cancelled or timed-out build context is classified inside the run func
	// below and is never retried, so the loop stops on cancellation.
	pullErr := retry.NewNoValue(
		retry.New().
			WithBackoff(m.pullRetryMinBackoff, m.pullRetryMaxBackoff).
			WithBackoffJitter().
			WithCheck(func(tries int, err error) bool {
				return m.shouldRetryImagePull(imageName, tries, err)
			}),
		func() error {
			return m.imagePullOnce(ref, opts)
		},
	).Run()
	if pullErr != nil {
		return nil, pullErr
	}

	image, _, err := m.client.ImageInspectWithRaw(m.context, imageName)
	if err != nil {
		if cancelErr := contextCancellationBuildError(m.context); cancelErr != nil {
			return nil, cancelErr
		}
		return nil, &common.BuildError{
			Inner:         fmt.Errorf("inspecting image %q after pull: %w", imageName, err),
			FailureReason: common.ClassifyImagePullFailure(err.Error()),
		}
	}
	return &image, nil
}

// shouldRetryImagePull decides whether a failed pull attempt should be retried.
// Only transient (network/registry) failures are retried, and only up to
// pullMaxAttempts; missing-image, auth-denied and cancellation failures are not.
func (m *manager) shouldRetryImagePull(imageName string, tries int, err error) bool {
	var buildErr *common.BuildError
	if tries >= m.pullMaxAttempts ||
		!errors.As(err, &buildErr) ||
		buildErr.FailureReason != common.RunnerExternalDependencyFailure {
		return false
	}

	m.logger.Warningln(fmt.Sprintf(
		"Failed to pull image %q (attempt #%d), retrying: %v", imageName, tries, err))
	return true
}

// imagePullOnce performs a single docker pull and classifies any failure. A
// cancelled or timed-out build context must not be misclassified as an
// image-pull failure (and must not be retried), so surface the cancellation
// reason directly.
func (m *manager) imagePullOnce(ref string, opts image.PullOptions) error {
	if err := m.client.ImagePullBlocking(m.context, ref, opts); err != nil {
		if cancelErr := contextCancellationBuildError(m.context); cancelErr != nil {
			return cancelErr
		}
		return &common.BuildError{Inner: err, FailureReason: common.ClassifyImagePullFailure(err.Error())}
	}
	return nil
}

// contextCancellationBuildError returns a BuildError describing the cancellation
// if ctx has been canceled or its deadline exceeded; otherwise it returns nil.
// This mirrors the semantics of (*Build).runtimeStateAndError so deep call sites
// don't misclassify cancellations as image-pull or system failures.
//
// Both ctx.Err() and context.Cause(ctx) are checked for DeadlineExceeded so
// that a deadline created with context.WithDeadlineCause is still classified
// as a timeout even when its custom cause does not wrap context.DeadlineExceeded.
func contextCancellationBuildError(ctx context.Context) error {
	if ctx.Err() == nil {
		return nil
	}
	cause := context.Cause(ctx)
	if errors.Is(cause, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return &common.BuildError{
			Inner:         cause,
			FailureReason: common.JobExecutionTimeout,
		}
	}
	return &common.BuildError{
		Inner:         common.ErrJobCanceled,
		FailureReason: common.JobCanceled,
	}
}

// getPullPolicies selects the pull_policy configurations originating from
// either gitlab-ci.yaml or config.toml. If present, the pull_policies in
// gitlab-ci.yaml take precedence over those in config.toml.
func (m *manager) getPullPolicies(imagePullPolicies []common.DockerPullPolicy) ([]common.DockerPullPolicy, error) {
	if len(imagePullPolicies) != 0 {
		return imagePullPolicies, nil
	}
	return m.config.DockerConfig.GetPullPolicies()
}
