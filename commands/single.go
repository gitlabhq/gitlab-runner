package commands

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

type RunSingleCommand struct {
	common.RunnerConfig

	network       common.Network
	traceProvider common.JobTraceProviderFunc

	WaitTimeout      int `long:"wait-timeout" description:"How long to wait in seconds before receiving the first job"`
	lastBuild        time.Time
	runForever       bool
	MaxBuilds        int `long:"max-builds" description:"How many builds to process before exiting"`
	finished         atomic.Bool
	interruptSignals chan os.Signal

	configOptions
	RunnerName string `short:"r" long:"runner" description:"Runner name from the config file to use instead of command-line arguments"`

	shutdownTimeout int `long:"shutdown-timeout" description:"Number of seconds after which the forceful shutdown operation will timeout and process will exit"`
}

func waitForInterrupts(
	finished *atomic.Bool,
	abortSignal chan os.Signal,
	doneSignal chan int,
	interruptSignals chan os.Signal,
	shutdownTimeout time.Duration,
) {
	if interruptSignals == nil {
		interruptSignals = make(chan os.Signal)
	}
	signal.Notify(interruptSignals, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	interrupt := <-interruptSignals
	if finished != nil {
		finished.Store(true)
	}

	// request stop, but wait for force exit
	for interrupt == syscall.SIGQUIT {
		logrus.Warningln("Requested quit, waiting for builds to finish")
		interrupt = <-interruptSignals
	}

	logrus.Warningln("Requested exit:", interrupt)

	go func() {
		for {
			abortSignal <- interrupt
		}
	}()

	select {
	case newSignal := <-interruptSignals:
		logrus.Fatalln("forced exit:", newSignal)
	case <-time.After(shutdownTimeout):
		logrus.Fatalln("shutdown timed out")
	case <-doneSignal:
	}
}

// Things to do after a build
func (r *RunSingleCommand) postBuild() {
	if r.MaxBuilds > 0 {
		r.MaxBuilds--
	}
	r.lastBuild = time.Now()
}

func (r *RunSingleCommand) processBuild(data common.ExecutorData, provider common.ExecutorProvider, abortSignal chan os.Signal) error {
	logrus.Debugln("[FT-DEBUG] processBuild: Starting build processing")
	
	store, err := provider.GetStore(&r.RunnerConfig)
	if err != nil {
		logrus.WithError(err).Errorln("[FT-DEBUG] processBuild: Failed to get store from provider")
		return err
	}
	
	logrus.WithField("store_type", reflect.TypeOf(store)).Debugln("[FT-DEBUG] processBuild: Got store from provider")

	manager := common.NewStatefulJobManager(
		r.network,
		store,
		r.traceProvider,
		&r.RunnerConfig,
	)
	
	logrus.Debugln("[FT-DEBUG] processBuild: Created StatefulJobManager, requesting job")
	
	job, healthy := manager.RequestJob(context.Background(), nil)
	if !healthy {
		logrus.Println("[FT-DEBUG] processBuild: Runner is not healthy!")
		select {
		case <-time.After(common.NotHealthyCheckInterval * time.Second):
		case <-abortSignal:
		}

		return nil
	}

	if job == nil {
		logrus.Debugln("[FT-DEBUG] processBuild: No job available")
		select {
		case <-time.After(common.CheckInterval):
		case <-abortSignal:
		}

		return nil
	}

	logrus.WithFields(logrus.Fields{
		"job_id": job.ID,
		"is_resumed": job.State.IsResumed(),
		"stage": job.State.GetStage(),
		"retries": job.State.GetRetries(),
	}).Infoln("[FT-DEBUG] processBuild: Got job")

	config := common.NewConfig()
	// Attempt to perform a deep copy of the RunnerConfig
	runnerConfigCopy, err := r.RunnerConfig.DeepCopy()
	if err != nil {
		logrus.WithError(err).Errorln("[FT-DEBUG] processBuild: Failed to deep copy runner config")
		return fmt.Errorf("deep copy of runner config failed: %w", err)
	}

	build := common.NewBuild(job, runnerConfigCopy)
	build.JobStore = store
	build.SystemInterrupt = abortSignal
	build.ExecutorData = data
	
	logrus.WithFields(logrus.Fields{
		"job_id": build.ID,
		"store_type": reflect.TypeOf(store),
	}).Debugln("[FT-DEBUG] processBuild: Created build with job store")

	jobCredentials := &common.JobCredentials{
		ID:    job.ID,
		Token: job.Token,
	}

	trace, err := manager.ProcessJob(jobCredentials)
	if err != nil {
		return err
	}

	trace.SetDebugModeEnabled(build.IsDebugModeEnabled())

	defer func() {
		err := trace.Success()
		logTerminationError(logrus.StandardLogger(), "Success", err)
	}()

	err = build.Run(config, trace)

	r.postBuild()

	return err
}

func (r *RunSingleCommand) checkFinishedConditions() {
	if r.MaxBuilds < 1 && !r.runForever {
		logrus.Println("This runner has processed its build limit, so now exiting")
		r.finished.Store(true)
	}
	if r.WaitTimeout > 0 && int(time.Since(r.lastBuild).Seconds()) > r.WaitTimeout {
		logrus.Println("This runner has not received a job in", r.WaitTimeout, "seconds, so now exiting")
		r.finished.Store(true)
	}
}

func (r *RunSingleCommand) HandleArgs() {
	if r.RunnerName != "" {
		fileConfig := &configOptions{ConfigFile: r.ConfigFile}
		err := fileConfig.loadConfig()
		if err != nil {
			logrus.Fatalf("Error loading config: %v", err)
		}
		runner, err := fileConfig.RunnerByName(r.RunnerName)
		if err != nil {
			logrus.Fatalf("Error loading runner by name: %v", err)
		}
		r.RunnerConfig = *runner
	}
	if r.URL == "" {
		logrus.Fatalln("Missing URL")
	}
	if r.Token == "" {
		logrus.Fatalln("Missing Token")
	}
	if r.Executor == "" {
		logrus.Fatalln("Missing Executor")
	}
}

func (r *RunSingleCommand) Execute(c *cli.Context) {
	r.HandleArgs()

	executorProvider := common.GetExecutorProvider(r.Executor)
	if executorProvider == nil {
		logrus.Fatalln("Unknown executor:", r.Executor)
	}

	managedProvider, ok := executorProvider.(common.ManagedExecutorProvider)
	if ok {
		managedProvider.Init()
	}

	r.RunnerConfig.SystemIDState = common.NewSystemIDState()
	if err := r.RunnerConfig.SystemIDState.EnsureSystemID(); err != nil {
		logrus.WithError(err).Fatal("Failed to generate random system ID")
	}

	logrus.Println("Starting runner for", r.URL, "with token", r.ShortDescription(), "...")

	abortSignal := make(chan os.Signal)
	doneSignal := make(chan int, 1)
	r.runForever = r.MaxBuilds == 0

	go waitForInterrupts(&r.finished, abortSignal, doneSignal, r.interruptSignals, r.getShutdownTimeout())

	r.lastBuild = time.Now()

	for !r.finished.Load() {
		data, err := executorProvider.Acquire(&r.RunnerConfig)
		if err != nil {
			logrus.Warningln("Executor update:", err)
		}

		pErr := r.processBuild(data, executorProvider, abortSignal)
		if pErr != nil {
			logrus.WithError(pErr).Error("Failed to process build")
		}

		r.checkFinishedConditions()
		executorProvider.Release(&r.RunnerConfig, data)
	}

	doneSignal <- 0

	providerShutdownCtx, shutdownProvider := context.WithTimeout(context.Background(), r.getShutdownTimeout())
	defer shutdownProvider()

	if managedProvider != nil {
		managedProvider.Shutdown(providerShutdownCtx)
	}
}

func (r *RunSingleCommand) getShutdownTimeout() time.Duration {
	if r.shutdownTimeout > 0 {
		return time.Duration(r.shutdownTimeout) * time.Second
	}

	return common.DefaultShutdownTimeout
}

func init() {
	common.RegisterCommand2("run-single", "start single runner", &RunSingleCommand{
		network:       network.NewGitLabClient(),
		traceProvider: network.ClientJobTraceProviderFunc,
	})
}
