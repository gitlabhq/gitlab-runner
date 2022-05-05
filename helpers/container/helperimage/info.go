package helperimage

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/errors"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

const (
	OSTypeLinux   = "linux"
	OSTypeWindows = "windows"

	//nolint:lll
	// DockerHubWarningMessage is the message that is printed to the user when
	// it's using the helper image hosted in Docker Hub. It is up to the caller
	// to print this message.
	DockerHubWarningMessage = "Pulling GitLab Runner helper image from Docker Hub. " +
		"Helper image is migrating to registry.gitlab.com, " +
		"for more information see " +
		"https://docs.gitlab.com/runner/configuration/advanced-configuration.html#migrate-helper-image-to-registrygitlabcom"

	// GitLabRegistryName is the name of the helper image hosted in registry.gitlab.com.
	GitLabRegistryName = "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper"

	// DefaultFlavor is the default flavor of image we use for the helper.
	DefaultFlavor = "alpine"

	headRevision        = "HEAD"
	latestImageRevision = "latest"
)

type Info struct {
	OSType                  string
	Architecture            string
	Name                    string
	Tag                     string
	IsSupportingLocalImport bool
	Cmd                     []string
}

func (i Info) String() string {
	return fmt.Sprintf("%s:%s", i.Name, i.Tag)
}

// Config specifies details about the consumer of this package that need to be
// taken in consideration when building Container.
type Config struct {
	OSType          string
	Architecture    string
	OperatingSystem string
	Shell           string
	Flavor          string
}

type creator interface {
	Create(revision string, cfg Config) (Info, error)
}

var supportedOsTypesFactories = map[string]creator{
	OSTypeWindows: new(windowsInfo),
	OSTypeLinux:   new(linuxInfo),
}

func Get(revision string, cfg Config) (Info, error) {
	factory, ok := supportedOsTypesFactories[cfg.OSType]
	if !ok {
		return Info{}, errors.NewErrOSNotSupported(cfg.OSType)
	}

	info, err := factory.Create(imageRevision(revision), cfg)
	info.OSType = cfg.OSType

	return info, err
}

func imageRevision(revision string) string {
	if revision != headRevision {
		return revision
	}

	return latestImageRevision
}

func getPowerShellCmd(shell string) []string {
	if shell == "" {
		shell = shells.SNPowershell
	}

	return shells.PowershellDockerCmd(shell)
}
