package docker

import (
	"bytes"
	"errors"

	"github.com/fsouza/go-dockerclient"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/executors"
)

type commandExecutor struct {
	executor

	containerOptions *docker.CreateContainerOptions

	predefinedContainer *docker.Container
	predefinedImage     *docker.Image

	buildContainer *docker.Container
	buildImage     string

	preparedServices servicesLinks
}

func (s *commandExecutor) Prepare(globalConfig *common.Config, config *common.RunnerConfig, build *common.Build) error {
	err := s.executor.Prepare(globalConfig, config, build)
	if err != nil {
		return err
	}

	s.Debugln("Starting Docker command...")

	if len(s.BuildScript.DockerCommand) == 0 {
		return errors.New("Script is not compatible with Docker")
	}

	s.buildImage, err = s.getImageName()
	if err != nil {
		return err
	}

	s.containerOptions, err = s.prepareBuildContainer()
	if err != nil {
		return err
	}

	s.preparedServices, err = s.startServices()
	if err != nil {
		return err
	}

	s.predefinedImage, err = s.getPrebuiltImage("build")
	if err != nil {
		return err
	}
	return nil
}

func (s *commandExecutor) getOrCreatePredefinedContainer() (container *docker.Container, err error) {
	if s.predefinedContainer == nil {
		// Create pre-build container which will git clone changes
		s.predefinedContainer, err = s.createContainer("predefined", s.predefinedImage.ID, nil, *s.containerOptions)
		if err != nil {
			return
		}
	}

	return s.predefinedContainer, nil
}

func (s *commandExecutor) getOrCreateBuildContainer() (container *docker.Container, err error) {
	if s.predefinedContainer == nil {
		// Verify the state of services
		links, err := s.finishServices(s.preparedServices)
		if err != nil {
			return nil, err
		}
		s.containerOptions.HostConfig.Links = append(s.containerOptions.HostConfig.Links, links...)

		// Create build container which will run actual builds
		s.buildContainer, err = s.createContainer("build", s.buildImage, s.BuildScript.DockerCommand, *s.containerOptions)
		if err != nil {
			return err
		}
	}

	return s.predefinedContainer, nil
}

func (s *commandExecutor) Run(cmd common.ExecutorCommand) (err error) {
	var container *docker.Container

	if cmd.Predefined {
		container, err = s.getOrCreatePredefinedContainer()
	} else {
		container, err = s.getOrCreateBuildContainer()
	}

	if err != nil {
		return
	}

	s.Debugln("Executing on", container.Name, "the", cmd.Script)

	return s.watchContainer(container, bytes.NewBufferString(cmd.Script), cmd.Abort)
}

func init() {
	options := executors.ExecutorOptions{
		DefaultBuildsDir: "/builds",
		DefaultCacheDir:  "/cache",
		SharedBuildsDir:  false,
		Shell: common.ShellScriptInfo{
			Shell:         "bash",
			Type:          common.NormalShell,
			RunnerCommand: "/usr/bin/gitlab-runner-helper",
		},
		ShowHostname:     true,
		SupportedOptions: []string{"image", "services"},
	}

	creator := func() common.Executor {
		return &commandExecutor{
			executor: executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: options,
				},
			},
		}
	}

	featuresUpdater := func(features *common.FeaturesInfo) {
		features.Variables = true
		features.Image = true
		features.Services = true
	}

	common.RegisterExecutor("docker", executors.DefaultExecutorProvider{
		Creator:         creator,
		FeaturesUpdater: featuresUpdater,
	})
}
