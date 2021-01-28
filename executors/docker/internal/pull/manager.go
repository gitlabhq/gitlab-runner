package pull

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	cli "github.com/docker/cli/cli/config/types"
	"github.com/docker/docker/api/types"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/auth"
)

type Manager interface {
	GetDockerImage(imageName string) (*types.ImageInspect, error)
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
	usedImagesLock sync.RWMutex

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

func (m *manager) GetDockerImage(imageName string) (*types.ImageInspect, error) {
	pullPolicies, err := m.config.DockerConfig.GetPullPolicies()
	if err != nil {
		return nil, err
	}

	var imageErr error
	for idx, pullPolicy := range pullPolicies {
		attempt := 1 + idx
		if attempt > 1 {
			m.logger.Infoln(fmt.Sprintf("Attempt #%d: Trying %q pull policy", attempt, pullPolicy))
		}

		var img *types.ImageInspect
		img, imageErr = m.getImageUsingPullPolicy(imageName, pullPolicy)
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
	m.usedImagesLock.RLock()
	defer m.usedImagesLock.RUnlock()

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
	imageName string,
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

	return m.pullDockerImage(imageName, authConfig)
}

func (m *manager) resolveAuthConfigForImage(imageName string) (*cli.AuthConfig, error) {
	registryInfo, err := auth.ResolveConfigForImage(
		imageName,
		m.config.AuthConfig,
		m.config.ShellUser,
		m.config.Credentials,
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

func (m *manager) pullDockerImage(imageName string, ac *cli.AuthConfig) (*types.ImageInspect, error) {
	if m.onPullImageHookFunc != nil {
		m.onPullImageHookFunc()
	}
	m.logger.Println("Pulling docker image", imageName, "...")

	ref := imageName
	// Add :latest to limit the download results
	if !strings.ContainsAny(ref, ":@") {
		ref += ":latest"
	}

	options := types.ImagePullOptions{}
	options.RegistryAuth, _ = auth.EncodeConfig(ac)

	errorRegexp := regexp.MustCompile("(repository does not exist|not found)")
	if err := m.client.ImagePullBlocking(m.context, ref, options); err != nil {
		if errorRegexp.MatchString(err.Error()) {
			return nil, &common.BuildError{Inner: err}
		}
		return nil, err
	}

	image, _, err := m.client.ImageInspectWithRaw(m.context, imageName)
	return &image, err
}
