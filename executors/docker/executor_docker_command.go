package docker

import (
	"bytes"
	"errors"
	"sync"

	"github.com/docker/docker/api/types"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
)

type commandExecutor struct {
	executor
	buildContainer *types.ContainerJSON
	sync.Mutex
}

func (s *commandExecutor) getBuildContainer() *types.ContainerJSON {
	s.Lock()
	defer s.Unlock()

	return s.buildContainer
}

func (s *commandExecutor) Prepare(options common.ExecutorPrepareOptions) error {
	err := s.executor.Prepare(options)
	if err != nil {
		return err
	}

	s.Debugln("Starting Docker command...")

	if len(s.BuildShell.DockerCommand) == 0 {
		return errors.New("Script is not compatible with Docker")
	}

	_, err = s.getPrebuiltImage()
	if err != nil {
		return err
	}

	_, err = s.getBuildImage()
	if err != nil {
		return err
	}
	return nil
}

func (s *commandExecutor) requestNewPredefinedContainer() (*types.ContainerJSON, error) {
	prebuildImage, err := s.getPrebuiltImage()
	if err != nil {
		return nil, err
	}

	buildImage := common.Image{
		Name: prebuildImage.ID,
	}

	containerJSON, err := s.createContainer("predefined", buildImage, common.ContainerCommandBuild, []string{prebuildImage.ID})
	if err != nil {
		return nil, err
	}

	return containerJSON, err
}

func (s *commandExecutor) requestBuildContainer() (*types.ContainerJSON, error) {
	s.Lock()
	defer s.Unlock()
	if s.buildContainer == nil {
		var err error

		// Start build container which will run actual build
		s.buildContainer, err = s.createContainer("build", s.Build.Image, s.BuildShell.DockerCommand, []string{})
		if err != nil {
			return nil, err
		}
	}

	return s.buildContainer, nil
}

func (s *commandExecutor) Run(cmd common.ExecutorCommand) error {
	var runOn *types.ContainerJSON
	var err error
	if cmd.Predefined {
		runOn, err = s.requestNewPredefinedContainer()
	} else {
		runOn, err = s.requestBuildContainer()
	}
	if err != nil {
		return err
	}

	s.Debugln("Executing on", runOn.Name, "the", cmd.Script)

	s.SetCurrentStage(DockerExecutorStageRun)

	return s.watchContainer(cmd.Context, runOn.ID, bytes.NewBufferString(cmd.Script))
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
		ShowHostname: true,
	}

	creator := func() common.Executor {
		e := &commandExecutor{
			executor: executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: options,
				},
			},
		}
		e.SetCurrentStage(common.ExecutorStageCreated)
		return e
	}

	featuresUpdater := func(features *common.FeaturesInfo) {
		features.Variables = true
		features.Image = true
		features.Services = true
		features.Session = true
		features.Terminal = true
	}

	common.RegisterExecutor("docker", executors.DefaultExecutorProvider{
		Creator:          creator,
		FeaturesUpdater:  featuresUpdater,
		DefaultShellName: options.Shell.Shell,
	})
}
