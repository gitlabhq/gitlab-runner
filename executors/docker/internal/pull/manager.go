package pull

import (
	"context"
	"fmt"
	"strings"
	"sync"

	cli "github.com/docker/cli/cli/config/types"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/image"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/auth"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/pull_policies"
)

type Manager interface {
	GetDockerImage(imageName string, options common.ImageDockerOptions, imagePullPolicies []common.DockerPullPolicy,
	) (*types.ImageInspect, error)
}

type ManagerConfig struct {
	DockerConfig *common.DockerConfig
	AuthConfig   string
	ShellUser    string
	Credentials  []common.Credentials
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
	}
}

func (m *manager) GetDockerImage(
	imageName string, options common.ImageDockerOptions,
	imagePullPolicies []common.DockerPullPolicy,
) (*types.ImageInspect, error) {
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

		var img *types.ImageInspect
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

func (m *manager) markImageAsUsed(imageName string, image *types.ImageInspect) {
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
	imageName string, options common.ImageDockerOptions,
	pullPolicy common.DockerPullPolicy,
) (*types.ImageInspect, error) {
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
	registryInfo, err := auth.ResolveConfigForImage(
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

func (m *manager) pullDockerImage(imageName string, options common.ImageDockerOptions, ac *cli.AuthConfig) (*types.ImageInspect, error) {
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
		return nil, &common.BuildError{Inner: err, FailureReason: common.ImagePullFailure}
	}

	if err := m.client.ImagePullBlocking(m.context, ref, opts); err != nil {
		return nil, &common.BuildError{Inner: err, FailureReason: getImagePullFailureReason(err)}
	}

	image, _, err := m.client.ImageInspectWithRaw(m.context, imageName)
	return &image, err
}

// Return a JobFailureReason for an image pull failure. The reason can be a ConfigurationError if the image
// specification was invalid, otherwise it's an ImagePullFailure.
func getImagePullFailureReason(err error) common.JobFailureReason {
	if err == nil {
		return ""
	}

	// These error messages indicate an invalid image specification.
	if strings.Contains(err.Error(), "repository does not exist") ||
		strings.Contains(err.Error(), "manifest unknown") {
		return common.ConfigurationError
	}
	return common.ImagePullFailure
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
