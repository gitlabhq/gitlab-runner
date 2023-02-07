package commands

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/tevino/abool"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/network"
)

//nolint:lll
type RunSingleCommand struct {
	common.RunnerConfig
	network          common.Network
	WaitTimeout      int `long:"wait-timeout" description:"How long to wait in seconds before receiving the first job"`
	lastBuild        time.Time
	runForever       bool
	MaxBuilds        int `long:"max-builds" description:"How many builds to process before exiting"`
	finished         *abool.AtomicBool
	interruptSignals chan os.Signal

	shutdownTimeout int `long:"shutdown-timeout" description:"Number of seconds after which the forceful shutdown operation will timeout and process will exit"`
}

func waitForInterrupts(
	finished *abool.AtomicBool,
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
		finished.Set()
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

func (r *RunSingleCommand) processBuild(data common.ExecutorData, abortSignal chan os.Signal) error {
	jobData, healthy := r.network.RequestJob(context.Background(), r.RunnerConfig, nil)
	if !healthy {
		logrus.Println("Runner is not healthy!")
		select {
		case <-time.After(common.NotHealthyCheckInterval * time.Second):
		case <-abortSignal:
		}
		return nil
	}

	if jobData == nil {
		select {
		case <-time.After(common.CheckInterval):
		case <-abortSignal:
		}
		return nil
	}

	config := common.NewConfig()
	newBuild, err := common.NewBuild(*jobData, &r.RunnerConfig, abortSignal, data)
	if err != nil {
		return err
	}

	jobCredentials := &common.JobCredentials{
		ID:    jobData.ID,
		Token: jobData.Token,
	}
	trace, err := r.network.ProcessJob(r.RunnerConfig, jobCredentials)
	if err != nil {
		return err
	}

	trace.SetDebugModeEnabled(newBuild.IsDebugModeEnabled())

	defer trace.Success()

	err = newBuild.Run(config, trace)

	r.postBuild()

	return err
}

func (r *RunSingleCommand) checkFinishedConditions() {
	if r.MaxBuilds < 1 && !r.runForever {
		logrus.Println("This runner has processed its build limit, so now exiting")
		r.finished.Set()
	}
	if r.WaitTimeout > 0 && int(time.Since(r.lastBuild).Seconds()) > r.WaitTimeout {
		logrus.Println("This runner has not received a job in", r.WaitTimeout, "seconds, so now exiting")
		r.finished.Set()
	}
}

func (r *RunSingleCommand) Execute(c *cli.Context) {
	if r.URL == "" {
		logrus.Fatalln("Missing URL")
	}
	if r.Token == "" {
		logrus.Fatalln("Missing Token")
	}
	if r.Executor == "" {
		logrus.Fatalln("Missing Executor")
	}

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

	r.finished = abool.New()
	abortSignal := make(chan os.Signal)
	doneSignal := make(chan int, 1)
	r.runForever = r.MaxBuilds == 0

	go waitForInterrupts(r.finished, abortSignal, doneSignal, r.interruptSignals, r.getShutdownTimeout())

	r.lastBuild = time.Now()

	for !r.finished.IsSet() {
		data, err := executorProvider.Acquire(&r.RunnerConfig)
		if err != nil {
			logrus.Warningln("Executor update:", err)
		}

		pErr := r.processBuild(data, abortSignal)
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
		network: network.NewGitLabClient(),
	})
}
