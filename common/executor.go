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

var executorProviders map[string]ExecutorProvider

func validateExecutorProvider(provider ExecutorProvider) error {
	if provider.GetDefaultShell() == "" {
		return errors.New("default shell not implemented")
	}

	if !provider.CanCreate() {
		return errors.New("cannot create executor")
	}

	if err := provider.GetFeatures(&FeaturesInfo{}); err != nil {
		return fmt.Errorf("cannot get features: %w", err)
	}

	return nil
}

// RegisterExecutorProvider maps an ExecutorProvider to an executor name, i.e. registers it.
func RegisterExecutorProvider(executor string, provider ExecutorProvider) {
	logrus.Debugln("Registering", executor, "executor...")

	if err := validateExecutorProvider(provider); err != nil {
		panic("Executor cannot be registered: " + err.Error())
	}

	if executorProviders == nil {
		executorProviders = make(map[string]ExecutorProvider)
	}
	if _, ok := executorProviders[executor]; ok {
		panic("Executor already exist: " + executor)
	}
	executorProviders[executor] = provider
}

// GetExecutorProvider returns an ExecutorProvider by name from the registered ones.
func GetExecutorProvider(executor string) ExecutorProvider {
	if executorProviders == nil {
		return nil
	}

	provider, _ := executorProviders[executor]
	return provider
}

// GetExecutorNames returns a list of all registered executor names.
func GetExecutorNames() []string {
	var names []string
	for name := range executorProviders {
		names = append(names, name)
	}
	return names
}

// GetExecutorProviders returns a list of all registered executor providers.
func GetExecutorProviders() []ExecutorProvider {
	var providers []ExecutorProvider
	for _, executorProvider := range executorProviders {
		providers = append(providers, executorProvider)
	}
	return providers
}

func NewExecutor(executor string) Executor {
	provider := GetExecutorProvider(executor)
	if provider != nil {
		return provider.Create()
	}

	return nil
}
