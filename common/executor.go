package common

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
)

// ExecutorData is an empty interface representing free-form data
// executor will use. Meant to be casted, e.g. virtual machine details.
//
//go:generate mockery --name=ExecutorData --inpackage
type ExecutorData interface{}

// ExecutorCommand stores the script executor will run on a given stage.
// If Predefined it will try to use already allocated resources.
type ExecutorCommand struct {
	Script     string
	Stage      BuildStage
	Predefined bool
	Context    context.Context
}

// ExecutorStage represents a stage of build execution in the executor scope.
type ExecutorStage string

const (
	// ExecutorStageCreated means the executor is being initialized, i.e. created.
	ExecutorStageCreated ExecutorStage = "created"
	// ExecutorStagePrepare means the executor is preparing its environment, initializing dependencies.
	ExecutorStagePrepare ExecutorStage = "prepare"
	// ExecutorStageFinish means the executor has finished build execution.
	ExecutorStageFinish ExecutorStage = "finish"
	// ExecutorStageCleanup means the executor is cleaning up resources.
	ExecutorStageCleanup ExecutorStage = "cleanup"
)

// ExecutorPrepareOptions stores any data necessary for the executor to prepare
// the environment for running a build. This includes runner configuration, build data, etc.
type ExecutorPrepareOptions struct {
	Config  *RunnerConfig
	Build   *Build
	Trace   JobTrace
	User    string
	Context context.Context
}

type NoFreeExecutorError struct {
	Message string
}

func (e *NoFreeExecutorError) Error() string {
	return e.Message
}

// Executor represents entities responsible for build execution.
// It prepares the environment, runs the build and cleans up resources.
// See more in https://docs.gitlab.com/runner/executors/
//
//go:generate mockery --name=Executor --inpackage
type Executor interface {
	// Shell returns data about the shell and scripts this executor is bound to.
	Shell() *ShellScriptInfo
	// Prepare prepares the environment for build execution. e.g. connects to SSH, creates containers.
	Prepare(options ExecutorPrepareOptions) error
	// Run executes a command on the prepared environment.
	Run(cmd ExecutorCommand) error
	// Finish marks the build execution as finished.
	Finish(err error)
	// Cleanup cleans any resources left by build execution.
	Cleanup()
	// GetCurrentStage returns current stage of build execution.
	GetCurrentStage() ExecutorStage
	// SetCurrentStage sets the current stage of build execution.
	SetCurrentStage(stage ExecutorStage)
	// Name returns the name of the executor
	Name() string
}

type ManagedExecutorProvider interface {
	// Init initializes the executor provider.
	//
	// Some providers may require that a non-trivial setup will be done for them to work properly. They may also
	// run a goroutines handling provider's state and management layer.
	//
	// Init method is a hook allowing to add such behavior.
	//
	// Init MUST BE NON-BLOCKING!
	Init()

	// Shutdown terminates the executor provider.
	//
	// As noted above, some executor providers may require to maintain a long-running state and management
	// layer.
	//
	// Shutdown method is a hook that allows to inform the executor provider that it should terminate
	// itself.
	//
	// Shutdown MUST BE BLOCKING until termination is done or provided context is canceled.
	//
	// First argument receive a context.Context object that will be canceled when shutting down will exceed
	// configured timeout.
	Shutdown(ctx context.Context)
}

// ExecutorProvider is responsible for managing the lifetime of executors, acquiring resources,
// retrieving executor metadata, etc.
//
//go:generate mockery --name=ExecutorProvider --inpackage
type ExecutorProvider interface {
	// CanCreate returns whether the executor provider has the necessary data to create an executor.
	CanCreate() bool
	// Create creates a new executor. No resource allocation happens.
	Create() Executor
	// Acquire acquires the necessary resources for the executor to run, e.g. finds a virtual machine.
	Acquire(config *RunnerConfig) (ExecutorData, error)
	// Release releases any resources locked by Acquire.
	Release(config *RunnerConfig, data ExecutorData)
	// GetFeatures returns metadata about the features the executor supports, e.g. variables, services, shell.
	GetFeatures(features *FeaturesInfo) error
	// GetConfigInfo extracts metadata about the config the executor is using, e.g. GPUs.
	GetConfigInfo(input *RunnerConfig, output *ConfigInfo)

	// GetDefaultShell returns the name of the default shell for the executor.
	GetDefaultShell() string
}

// BuildError represents an error during build execution, not related to
// the job script, e.g. failed to create container, establish ssh connection.
type BuildError struct {
	Inner         error
	FailureReason JobFailureReason
	ExitCode      int
}

// Error implements the error interface.
func (b *BuildError) Error() string {
	if b.Inner == nil {
		return "error"
	}

	return b.Inner.Error()
}

func (b *BuildError) Is(err error) bool {
	buildErr, ok := err.(*BuildError)
	if !ok {
		return false
	}

	return buildErr.FailureReason == b.FailureReason
}

func (b *BuildError) Unwrap() error {
	return b.Inner
}

// MakeBuildError returns an new instance of BuildError.
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

	provider := executorProviders[executor]
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
