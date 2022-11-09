package docker

import (
	"errors"

	"github.com/docker/docker/api/types"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/docker/internal/volumes/parser"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/ssh"
)

// DEPRECATED
// TODO: Remove in 16.0. For more details read https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29406
type sshExecutor struct {
	executor
	sshCommand ssh.Client
}

func (s *sshExecutor) Prepare(options common.ExecutorPrepareOptions) error {
	err := s.executor.Prepare(options)
	if err != nil {
		return err
	}

	s.Warningln(
		"Since GitLab Runner 10.0 docker-ssh and docker-ssh+machine executors " +
			"are marked as DEPRECATED and will be removed with GitLab Runner 16.0")

	if s.Config.SSH == nil {
		return errors.New("missing SSH configuration")
	}

	s.Debugln("Starting SSH command...")

	// Start build container which will run actual build
	container, err := s.createContainer("build", s.Build.Image, []string{}, []string{})
	if err != nil {
		return err
	}

	s.Debugln("Starting container", container.ID, "...")
	err = s.client.ContainerStart(s.Context, container.ID, types.ContainerStartOptions{})
	if err != nil {
		return err
	}

	containerData, err := s.client.ContainerInspect(s.Context, container.ID)
	if err != nil {
		return err
	}

	// Create SSH command
	s.sshCommand = ssh.Client{
		Config: *s.Config.SSH,
		Stdout: s.Trace,
		Stderr: s.Trace,
	}
	s.sshCommand.Host = containerData.NetworkSettings.IPAddress

	s.Debugln("Connecting to SSH server...")
	err = s.sshCommand.Connect()
	if err != nil {
		return err
	}
	return nil
}

func (s *sshExecutor) Run(cmd common.ExecutorCommand) error {
	s.SetCurrentStage(ExecutorStageRun)

	err := s.sshCommand.Run(cmd.Context, ssh.Command{
		Command: s.BuildShell.CmdLine,
		Stdin:   cmd.Script,
	})
	if exitError, ok := err.(*ssh.ExitError); ok {
		exitCode := exitError.ExitCode()
		err = &common.BuildError{Inner: err, ExitCode: exitCode}
	}
	return err
}

func (s *sshExecutor) Cleanup() {
	s.sshCommand.Cleanup()
	s.executor.Cleanup()
}

// TODO: Remove in 16.0. For more details read https://gitlab.com/gitlab-org/gitlab-runner/-/issues/29406
func init() {
	options := executors.ExecutorOptions{
		DefaultCustomBuildsDirEnabled: true,
		DefaultBuildsDir:              "builds",
		DefaultCacheDir:               "cache",
		SharedBuildsDir:               false,
		Shell: common.ShellScriptInfo{
			Shell:         "bash",
			Type:          common.LoginShell,
			RunnerCommand: "gitlab-runner",
		},
		ShowHostname: true,
		Metadata: map[string]string{
			metadataOSType: osTypeLinux,
		},
	}

	creator := func() common.Executor {
		e := &sshExecutor{
			executor: executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: options,
				},
				volumeParser: parser.NewLinuxParser(),
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

	common.RegisterExecutorProvider("docker-ssh", executors.DefaultExecutorProvider{
		Creator:          creator,
		FeaturesUpdater:  featuresUpdater,
		ConfigUpdater:    configUpdater,
		DefaultShellName: options.Shell.Shell,
	})
}
