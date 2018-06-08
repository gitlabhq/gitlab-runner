package machine

import (
	"errors"
	"time"

	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-runner/common"

	// Force to load docker executor
	_ "gitlab.com/gitlab-org/gitlab-runner/executors/docker"
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
			"usedcount": details.UsedCount,
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
	options.Config.Docker.DockerCredentials = e.config.Docker.DockerCredentials

	// TODO: Currently the docker-machine doesn't support multiple builds
	e.build.ProjectRunnerID = 0
	if details, _ := options.Build.ExecutorData.(*machineDetails); details != nil {
		options.Build.Hostname = details.Name
	} else if details, _ := e.data.(*machineDetails); details != nil {
		options.Build.Hostname = details.Name
	}

	e.log().Infoln("Starting docker-machine build...")

	// Create original executor
	e.executor = e.provider.provider.Create()
	if e.executor == nil {
		return errors.New("failed to create an executor")
	}

	return e.executor.Prepare(options)
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
	e.log().Infoln("Finished docker-machine build:", err)
}

func (e *machineExecutor) Cleanup() {
	// Cleanup executor if were created
	if e.executor != nil {
		e.executor.Cleanup()
	}

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

func init() {
	common.RegisterExecutor("docker+machine", newMachineProvider("docker+machine", "docker"))
	common.RegisterExecutor("docker-ssh+machine", newMachineProvider("docker-ssh+machine", "docker-ssh"))
}
