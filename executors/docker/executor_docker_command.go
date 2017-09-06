package docker

import (
	"bytes"
	"errors"
	"time"

	"github.com/docker/docker/api/types"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/executors"
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

	_, err = s.getPrebuiltImage()
	if err != nil {
		return err
	}
	return nil
}

func (s *commandExecutor) createPrebuiltContainer() (*types.ContainerJSON, error) {
	prebuildImage, err := s.getPrebuiltImage()
	if err != nil {
		return nil, err
	}

	return s.createContainer("predefined", prebuildImage.ID, []string{}, []string{"gitlab-runner-build"})
}

func (s *commandExecutor) createBuildContainer() (*types.ContainerJSON, error) {
	buildImage, err := s.getBuildImage()
	if err != nil {
		return nil, err
	}

	return s.createContainer("build", buildImage.ID, s.Build.Image.Entrypoint, s.BuildShell.DockerCommand)
}

func (s *commandExecutor) Run(cmd common.ExecutorCommand) error {
	s.SetCurrentStage(DockerExecutorStageRun)

	var runOn *types.ContainerJSON
	var err error

	for i := 0; i < 3; i++ {
		if cmd.Predefined {
			runOn, err = s.createPrebuiltContainer()
		} else {
			runOn, err = s.createBuildContainer()
		}
		if err == nil {
			break
		}
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		return err
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
