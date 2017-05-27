package scaling

import (
	"errors"

	"github.com/Sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"

	// Force to load docker executor
	"github.com/mohae/deepcopy"
	_ "gitlab.com/gitlab-org/gitlab-ci-multi-runner/executors/docker"
	"fmt"
)

type instanceExecutor struct {
	provider *provider

	manager  *instanceManager
	machine  *instanceDetails
	executor common.Executor

	build  *common.Build
	config common.RunnerConfig

	currentStage common.ExecutorStage
}

func (e *instanceExecutor) log() (log *logrus.Entry) {
	log = e.build.Log()

	if e.machine != nil {
		log = log.WithField("name", e.machine.GetName())
		log = log.WithField("ip", e.machine.GetIP())
	}

	if e.config.Docker != nil {
		log = log.WithField("docker", e.config.Docker.Host)
	}
	return
}

func (e *instanceExecutor) Shell() *common.ShellScriptInfo {
	if e.executor == nil {
		return nil
	}
	return e.executor.Shell()
}

func (e *instanceExecutor) Prepare(options common.ExecutorPrepareOptions) (err error) {
	e.build = options.Build

	e.manager = e.provider.instanceManager(options.Config)
	if e.manager == nil {
		return errors.New("cannot find instance manager")
	}

	e.machine, err = e.manager.Allocate()
	if err != nil {
		return err
	}

	options.Config = deepcopy.Copy(options.Config).(*common.RunnerConfig)
	options.Config.Docker.DockerCredentials.Host = fmt.Sprintf("tcp://%s:2376", e.machine.GetIP())
	options.Build.Hostname = e.machine.GetName()

	// TODO: Currently the docker-machine doesn't support multiple builds
	e.build.ProjectRunnerID = 0

	e.log().Infoln("Starting docker-machine build...")

	// Create original executor
	e.executor = e.provider.executorProvider.Create()
	if e.executor == nil {
		return errors.New("failed to create an executor")
	}

	return e.executor.Prepare(options)
}

func (e *instanceExecutor) Run(cmd common.ExecutorCommand) error {
	if e.executor == nil {
		return errors.New("missing executor")
	}
	return e.executor.Run(cmd)
}

func (e *instanceExecutor) Finish(err error) {
	if e.executor != nil {
		e.executor.Finish(err)
	}
	e.log().Infoln("Finished scaling build:", err)
}

func (e *instanceExecutor) Cleanup() {
	if e.executor != nil {
		e.executor.Cleanup()
	}

	if e.machine != nil {
		e.manager.Free(e.machine)
	}
}

func (e *instanceExecutor) GetCurrentStage() common.ExecutorStage {
	if e.executor == nil {
		return common.ExecutorStage("")
	}

	return e.executor.GetCurrentStage()
}

func (e *instanceExecutor) SetCurrentStage(stage common.ExecutorStage) {
	if e.executor == nil {
		e.currentStage = stage
		return
	}

	e.executor.SetCurrentStage(stage)
}
