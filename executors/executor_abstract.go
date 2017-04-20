package executors

import (
	"os"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
)

type ExecutorOptions struct {
	DefaultBuildsDir string
	DefaultCacheDir  string
	SharedBuildsDir  bool
	Shell            common.ShellScriptInfo
	ShowHostname     bool
}

type AbstractExecutor struct {
	ExecutorOptions
	common.BuildLogger
	Config     common.RunnerConfig
	Build      *common.Build
	BuildTrace common.JobTrace
	BuildShell *common.ShellConfiguration

	currentStage common.ExecutorStage
}

func (e *AbstractExecutor) updateShell() error {
	script := e.Shell()
	script.Build = e.Build
	if e.Config.Shell != "" {
		script.Shell = e.Config.Shell
	}
	return nil
}

func (e *AbstractExecutor) generateShellConfiguration() error {
	info := e.Shell()
	info.PreCloneScript = e.Config.PreCloneScript
	info.PreBuildScript = e.Config.PreBuildScript
	info.PostBuildScript = e.Config.PostBuildScript
	shellConfiguration, err := common.GetShellConfiguration(*info)
	if err != nil {
		return err
	}
	e.BuildShell = shellConfiguration
	e.Debugln("Shell configuration:", shellConfiguration)
	return nil
}

func (e *AbstractExecutor) startBuild() error {
	// Save hostname
	if e.ShowHostname && e.Build.Hostname == "" {
		e.Build.Hostname, _ = os.Hostname()
	}

	// Start actual build
	rootDir := e.Config.BuildsDir
	if rootDir == "" {
		rootDir = e.DefaultBuildsDir
	}
	cacheDir := e.Config.CacheDir
	if cacheDir == "" {
		cacheDir = e.DefaultCacheDir
	}
	e.Build.StartBuild(rootDir, cacheDir, e.SharedBuildsDir)
	return nil
}

func (e *AbstractExecutor) Shell() *common.ShellScriptInfo {
	return &e.ExecutorOptions.Shell
}

func (e *AbstractExecutor) Prepare(options common.ExecutorPrepareOptions) error {
	e.currentStage = common.ExecutorStagePrepare
	e.Config = *options.Config
	e.Build = options.Build
	e.BuildTrace = options.Trace
	e.BuildLogger = common.NewBuildLogger(options.Build.Trace, options.Build.Log())

	err := e.startBuild()
	if err != nil {
		return err
	}

	err = e.updateShell()
	if err != nil {
		return err
	}

	err = e.generateShellConfiguration()
	if err != nil {
		return err
	}
	return nil
}

func (e *AbstractExecutor) Finish(err error) {
	e.currentStage = common.ExecutorStageFinish
}

func (e *AbstractExecutor) Cleanup() {
	e.currentStage = common.ExecutorStageCleanup
}

func (e *AbstractExecutor) GetCurrentStage() common.ExecutorStage {
	return e.currentStage
}

func (e *AbstractExecutor) SetCurrentStage(stage common.ExecutorStage) {
	e.currentStage = stage
}
