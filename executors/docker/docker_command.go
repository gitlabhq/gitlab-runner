package docker

import (
	"bytes"
	"errors"
	"fmt"
	"sync"

	"github.com/docker/docker/api/types"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/permission"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/docker"
)

type commandExecutor struct {
	executor
	buildContainer *types.ContainerJSON
	lock           sync.Mutex
}

func (s *commandExecutor) getBuildContainer() *types.ContainerJSON {
	s.lock.Lock()
	defer s.lock.Unlock()

	return s.buildContainer
}

func (s *commandExecutor) Prepare(options common.ExecutorPrepareOptions) error {
	err := s.executor.Prepare(options)
	if err != nil {
		return err
	}

	s.Debugln("Starting Docker command...")

	if len(s.BuildShell.DockerCommand) == 0 {
		return errors.New("script is not compatible with Docker")
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

	containerJSON, err := s.createContainer("predefined", buildImage, s.helperImageInfo.Cmd, []string{prebuildImage.ID})
	if err != nil {
		return nil, err
	}

	return containerJSON, err
}

func (s *commandExecutor) requestBuildContainer() (*types.ContainerJSON, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.buildContainer != nil {
		_, inspectErr := s.client.ContainerInspect(s.Context, s.buildContainer.ID)
		if inspectErr == nil {
			return s.buildContainer, nil
		}

		if !docker.IsErrNotFound(inspectErr) {
			s.Warningln("Failed to inspect build container", s.buildContainer.ID, inspectErr.Error())
		}
	}

	var err error
	s.buildContainer, err = s.createContainer("build", s.Build.Image, s.BuildShell.DockerCommand, []string{})
	if err != nil {
		return nil, err
	}

	return s.buildContainer, nil
}

func (s *commandExecutor) Run(cmd common.ExecutorCommand) error {
	maxAttempts, err := s.Build.GetExecutorJobSectionAttempts()
	if err != nil {
		return fmt.Errorf("getting job section attempts: %w", err)
	}

	var runErr error
	for attempts := 1; attempts <= maxAttempts; attempts++ {
		if attempts > 1 {
			s.Infoln(fmt.Sprintf("Retrying %s", cmd.Stage))
		}

		ctr, err := s.getContainer(cmd)
		if err != nil {
			return err
		}

		s.Debugln("Executing on", ctr.Name, "the", cmd.Script)
		s.SetCurrentStage(ExecutorStageRun)

		runErr = s.startAndWatchContainer(cmd.Context, ctr.ID, bytes.NewBufferString(cmd.Script))
		if !docker.IsErrNotFound(runErr) {
			return runErr
		}

		s.Errorln(fmt.Sprintf("Container %q not found or removed. Will retry...", ctr.ID))
	}

	if runErr != nil && maxAttempts > 1 {
		s.Errorln("Execution attempts exceeded")
	}

	return runErr
}

func (s *commandExecutor) getContainer(cmd common.ExecutorCommand) (*types.ContainerJSON, error) {
	if cmd.Predefined {
		return s.requestNewPredefinedContainer()
	}

	return s.requestBuildContainer()
}

func (s *commandExecutor) GetMetricsSelector() string {
	return fmt.Sprintf("instance=%q", s.executor.info.Name)
}

func init() {
	options := executors.ExecutorOptions{
		DefaultCustomBuildsDirEnabled: true,
		DefaultBuildsDir:              "/builds",
		DefaultCacheDir:               "/cache",
		SharedBuildsDir:               false,
		Shell: common.ShellScriptInfo{
			Shell:         "bash",
			Type:          common.NormalShell,
			RunnerCommand: "/usr/bin/gitlab-runner-helper",
		},
		ShowHostname: true,
		Metadata: map[string]string{
			metadataOSType: osTypeLinux,
		},
	}

	creator := func() common.Executor {
		e := &commandExecutor{
			executor: executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: options,
				},
				volumeParser: parser.NewLinuxParser(),
			},
		}

		e.newVolumePermissionSetter = func() (permission.Setter, error) {
			helperImage, err := e.getPrebuiltImage()
			if err != nil {
				return nil, err
			}

			return permission.NewDockerLinuxSetter(e.client, e.Build.Log(), helperImage), nil
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

	common.RegisterExecutorProvider("docker", executors.DefaultExecutorProvider{
		Creator:          creator,
		FeaturesUpdater:  featuresUpdater,
		DefaultShellName: options.Shell.Shell,
	})
}
