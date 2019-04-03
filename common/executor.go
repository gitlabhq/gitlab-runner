package common

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
)

type ExecutorData interface{}

type ExecutorCommand struct {
	Script     string
	Stage      BuildStage
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
	Release(config *RunnerConfig, data ExecutorData)
	GetFeatures(features *FeaturesInfo) error
	GetDefaultShell() string
}

type BuildError struct {
	Inner         error
	FailureReason JobFailureReason
}

func (b *BuildError) Error() string {
	if b.Inner == nil {
		return "error"
	}

	return b.Inner.Error()
}

func MakeBuildError(format string, args ...interface{}) error {
	return &BuildError{
		Inner: fmt.Errorf(format, args...),
	}
}

var executors map[string]ExecutorProvider

func validateExecutorProvider(provider ExecutorProvider) error {
	if provider.GetDefaultShell() == "" {
		return errors.New("default shell not implemented")
	}

	if !provider.CanCreate() {
		return errors.New("cannot create executor")
	}

	if err := provider.GetFeatures(&FeaturesInfo{}); err != nil {
		return fmt.Errorf("cannot get features: %v", err)
	}

	return nil
}

func RegisterExecutor(executor string, provider ExecutorProvider) {
	logrus.Debugln("Registering", executor, "executor...")

	if err := validateExecutorProvider(provider); err != nil {
		panic("Executor cannot be registered: " + err.Error())
	}

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
