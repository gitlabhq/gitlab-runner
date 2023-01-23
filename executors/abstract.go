package executors

import (
	"context"
	"os"
	"sync"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/session/proxy"
)

type ExecutorOptions struct {
	DefaultCustomBuildsDirEnabled bool
	DefaultBuildsDir              string
	DefaultCacheDir               string
	SharedBuildsDir               bool
	Shell                         common.ShellScriptInfo
	ShowHostname                  bool
	Metadata                      map[string]string
}

type AbstractExecutor struct {
	ExecutorOptions
	common.BuildLogger
	Config       common.RunnerConfig
	Build        *common.Build
	Trace        common.JobTrace
	BuildShell   *common.ShellConfiguration
	currentStage common.ExecutorStage
	Context      context.Context
	ProxyPool    proxy.Pool

	stageLock sync.RWMutex
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
	info.PreGetSourcesScript = e.Config.GetPreGetSourcesScript()
	info.PostGetSourcesScript = e.Config.GetPostGetSourcesScript()
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

	return e.Build.StartBuild(
		e.RootDir(),
		e.CacheDir(),
		e.CustomBuildEnabled(),
		e.SharedBuildsDir,
	)
}

func (e *AbstractExecutor) RootDir() string {
	if e.Config.BuildsDir != "" {
		return e.Config.BuildsDir
	}

	return e.DefaultBuildsDir
}

func (e *AbstractExecutor) CacheDir() string {
	if e.Config.CacheDir != "" {
		return e.Config.CacheDir
	}

	return e.DefaultCacheDir
}

func (e *AbstractExecutor) CustomBuildEnabled() bool {
	if e.Config.CustomBuildDir != nil {
		return e.Config.CustomBuildDir.Enabled
	}

	return e.DefaultCustomBuildsDirEnabled
}

func (e *AbstractExecutor) Shell() *common.ShellScriptInfo {
	return &e.ExecutorOptions.Shell
}

func (e *AbstractExecutor) Prepare(options common.ExecutorPrepareOptions) error {
	e.PrepareConfiguration(options)

	return e.PrepareBuildAndShell()
}

func (e *AbstractExecutor) PrepareConfiguration(options common.ExecutorPrepareOptions) {
	e.SetCurrentStage(common.ExecutorStagePrepare)
	e.Context = options.Context
	e.Config = *options.Config
	e.Build = options.Build
	e.Trace = options.Trace
	e.BuildLogger = common.NewBuildLogger(options.Trace, options.Build.Log())
	e.ProxyPool = proxy.NewPool()
}

func (e *AbstractExecutor) PrepareBuildAndShell() error {
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
	e.SetCurrentStage(common.ExecutorStageFinish)
}

func (e *AbstractExecutor) Cleanup() {
	e.SetCurrentStage(common.ExecutorStageCleanup)
}

func (e *AbstractExecutor) GetCurrentStage() common.ExecutorStage {
	e.stageLock.RLock()
	defer e.stageLock.RUnlock()

	return e.currentStage
}

func (e *AbstractExecutor) SetCurrentStage(stage common.ExecutorStage) {
	e.stageLock.Lock()
	defer e.stageLock.Unlock()

	e.currentStage = stage
}
