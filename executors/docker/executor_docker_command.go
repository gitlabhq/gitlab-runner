package docker

import (
	"bytes"
	"errors"

	"github.com/docker/docker/api/types"
	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
)

type commandExecutor struct {
	executor
	predefinedContainer *types.ContainerJSON
	buildContainer      *types.ContainerJSON
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

	prebuildImage, err := s.getPrebuiltImage()
	if err != nil {
		return err
	}

	buildImage := common.Image{
		Name: prebuildImage.ID,
	}

	// Start pre-build container which will git clone changes
	s.predefinedContainer, err = s.createContainer("predefined", buildImage, []string{"gitlab-runner-build"}, []string{prebuildImage.ID})
	if err != nil {
		return err
	}

	// Start build container which will run actual build
	s.buildContainer, err = s.createContainer("build", s.Build.Image, s.BuildShell.DockerCommand, []string{})
	if err != nil {
		return err
	}
	return nil
}

func (s *commandExecutor) Run(cmd common.ExecutorCommand) error {
	s.SetCurrentStage(DockerExecutorStageRun)

	var runOn *types.ContainerJSON
	if cmd.Predefined {
		runOn = s.predefinedContainer
	} else {
		runOn = s.buildContainer
	}

	s.Debugln("Executing on", runOn.Name, "the", cmd.Script)

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
	}

	common.RegisterExecutor("docker", executors.DefaultExecutorProvider{
		Creator:         creator,
		FeaturesUpdater: featuresUpdater,
	})
}
