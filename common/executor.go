package common

import (
	log "github.com/Sirupsen/logrus"
	"context"
)

type ExecutorData interface{}

type ExecutorCommand struct {
	Script     string
	Predefined bool
	Context    context.Context
}

type ExecutorStage string

const (
	ExecutorStageCreated ExecutorStage = "created"
	ExecutorStagePrepare ExecutorStage = "prepare"
	ExecutorStageFinish  ExecutorStage = "finish"
	ExecutorStageCleanup ExecutorStage = "cleanup"
)

type ExecutorPrepareOptions struct {
	Config  *RunnerConfig
	Build   *Build
	Trace   JobTrace
	User    string
	Context context.Context
}

type Executor interface {
	Shell() *ShellScriptInfo
	Prepare(options ExecutorPrepareOptions) error
	Run(cmd ExecutorCommand) error
	Finish(err error)
	Cleanup()
	GetCurrentStage() ExecutorStage
	SetCurrentStage(stage ExecutorStage)
}

type ExecutorProvider interface {
	CanCreate() bool
	Create() Executor
	Acquire(config *RunnerConfig) (ExecutorData, error)
	Release(config *RunnerConfig, data ExecutorData) error
	GetFeatures(features *FeaturesInfo)
}

type BuildError struct {
	Inner error
}

func (b *BuildError) Error() string {
	if b.Inner == nil {
		return "error"
	}

	return b.Inner.Error()
}

var executors map[string]ExecutorProvider

func RegisterExecutor(executor string, provider ExecutorProvider) {
	log.Debugln("Registering", executor, "executor...")

	if executors == nil {
		executors = make(map[string]ExecutorProvider)
	}
	if _, ok := executors[executor]; ok {
		panic("Executor already exist: " + executor)
	}
	executors[executor] = provider
}

func GetExecutor(executor string) ExecutorProvider {
	if executors == nil {
		return nil
	}

	provider, _ := executors[executor]
	return provider
}

func GetExecutors() []string {
	names := []string{}
	if executors != nil {
		for name := range executors {
			names = append(names, name)
		}
	}
	return names
}

func GetExecutorProviders() (providers []ExecutorProvider) {
	if executors != nil {
		for _, executorProvider := range executors {
			providers = append(providers, executorProvider)
		}
	}
	return
}

func NewExecutor(executor string) Executor {
	provider := GetExecutor(executor)
	if provider != nil {
		return provider.Create()
	}

	return nil
}
