package machine

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/docker" // Force to load docker executor
	"gitlab.com/gitlab-org/gitlab-runner/referees"
	"gitlab.com/gitlab-org/gitlab-runner/session/terminal"
	"gitlab.com/gitlab-org/gitlab-runner/steps"
)

var (
	_ terminal.InteractiveTerminal = (*machineExecutor)(nil)
	_ steps.Connector              = (*machineExecutor)(nil)
)

const (
	DockerMachineExecutorStageUseMachine     common.ExecutorStage = "docker_machine_use_machine"
	DockerMachineExecutorStageReleaseMachine common.ExecutorStage = "docker_machine_release_machine"
)

type machineExecutor struct {
	provider *machineProvider
	executor common.Executor
	build    *common.Build
	data     common.ExecutorData
	config   common.RunnerConfig

	currentStage common.ExecutorStage
}

func (e *machineExecutor) log() (log *logrus.Entry) {
	log = e.build.Log()

	details, _ := e.build.ExecutorData.(*machineDetails)
	if details == nil {
		details, _ = e.data.(*machineDetails)
	}
	if details != nil {
		log = log.WithFields(logrus.Fields{
			"name":      details.Name,
			"usedCount": details.UsedCount,
			"created":   details.Created,
			"now":       time.Now(),
		})
	}
	if e.config.Docker != nil {
		log = log.WithField("docker", e.config.Docker.Host)
	}

	return
}

func (e *machineExecutor) Shell() *common.ShellScriptInfo {
	if e.executor == nil {
		return nil
	}
	return e.executor.Shell()
}

func (e *machineExecutor) Prepare(options common.ExecutorPrepareOptions) (err error) {
	e.build = options.Build

	if options.Config.Docker == nil {
		options.Config.Docker = &common.DockerConfig{}
	}

	// Use the machine
	e.SetCurrentStage(DockerMachineExecutorStageUseMachine)
	e.config, e.data, err = e.provider.Use(options.Config, options.Build.ExecutorData)
	if err != nil {
		return err
	}
	options.Config.Docker.Credentials = e.config.Docker.Credentials

	// TODO: Currently the docker-machine doesn't support multiple builds
	e.build.ProjectRunnerID = 0
	if details, _ := options.Build.ExecutorData.(*machineDetails); details != nil {
		options.Build.Hostname = details.Name
	} else if details, _ := e.data.(*machineDetails); details != nil {
		options.Build.Hostname = details.Name
	}

	// e.data is only set if the docker-machine created is new
	if e.data == nil {
		e.log().Infoln("Using existing docker-machine")
	} else {
		e.log().Infoln("Created docker-machine")
	}

	// Create original executor
	e.executor = e.provider.provider.Create()
	if e.executor == nil {
		return errors.New("failed to create an executor")
	}

	if err = e.executor.Prepare(options); err != nil {
		e.log().WithError(err).Errorln("Preparing docker-machine wrapped executor failed")
		return err
	}

	e.log().Infoln("Starting docker-machine build...")

	return nil
}

func (e *machineExecutor) Run(cmd common.ExecutorCommand) error {
	if e.executor == nil {
		return errors.New("missing executor")
	}
	return e.executor.Run(cmd)
}

func (e *machineExecutor) Finish(err error) {
	if e.executor != nil {
		e.executor.Finish(err)
	}

	if err == nil {
		e.log().Infoln("Finished docker-machine build")
	} else {
		e.log().Warningln("Finished docker-machine build with error:", err)
	}
}

func (e *machineExecutor) Cleanup() {
	// Cleanup executor if were created
	if e.executor != nil {
		e.executor.Cleanup()
	}

	e.log().Infoln("Cleaned up docker-machine")

	// Release allocated machine
	if e.data != nil {
		e.SetCurrentStage(DockerMachineExecutorStageReleaseMachine)
		e.provider.Release(&e.config, e.data)
		e.data = nil
	}
}

func (e *machineExecutor) GetCurrentStage() common.ExecutorStage {
	if e.executor == nil {
		return common.ExecutorStage("")
	}

	return e.executor.GetCurrentStage()
}

func (e *machineExecutor) SetCurrentStage(stage common.ExecutorStage) {
	if e.executor == nil {
		e.currentStage = stage
		return
	}

	e.executor.SetCurrentStage(stage)
}

func (e *machineExecutor) GetMetricsSelector() string {
	refereed, ok := e.executor.(referees.MetricsExecutor)
	if !ok {
		return ""
	}

	return refereed.GetMetricsSelector()
}

func (s *machineExecutor) Connect(ctx context.Context) (io.ReadWriteCloser, error) {
	if connector, ok := s.executor.(steps.Connector); ok {
		return connector.Connect(ctx)
	}

	return nil, common.ExecutorStepRunnerConnectNotSupported
}

func (e *machineExecutor) TerminalConnect() (terminal.Conn, error) {
	if connector, ok := e.executor.(terminal.InteractiveTerminal); ok {
		return connector.TerminalConnect()
	}

	return nil, errors.New("executor does not have terminal")
}

func init() {
	common.RegisterExecutorProvider("docker+machine", newMachineProvider())
}
