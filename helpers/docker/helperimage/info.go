package helperimage

import (
	"fmt"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker/errors"
)

const (
	OSTypeLinux   = "linux"
	OSTypeWindows = "windows"

	name = "gitlab/gitlab-runner-helper"

	headRevision        = "HEAD"
	latestImageRevision = "latest"
)

type Info struct {
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

	return factory.Create(imageRevision(revision), cfg)
}

func imageRevision(revision string) string {
	if revision != headRevision {
		return revision
	}

	return latestImageRevision
}
