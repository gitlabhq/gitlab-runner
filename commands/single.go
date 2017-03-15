package commands

import (
	"github.com/codegangsta/cli"
	"os"
	"time"

	log "github.com/Sirupsen/logrus"

	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/common"
	"gitlab.com/gitlab-org/gitlab-ci-multi-runner/network"
	"os/signal"
	"syscall"
)

type RunSingleCommand struct {
	common.RunnerConfig
	network     common.Network
	WaitTimeout int `long:"wait-timeout" description:"How long to wait in seconds before receiving the first job"`
	lastBuild   time.Time
	runForever  bool
	MaxBuilds   int `long:"max-builds" description:"How many builds to process before exiting"`
	finished    bool
}

func waitForInterrupts(finished *bool, abortSignal chan os.Signal, doneSignal chan int) {
	signals := make(chan os.Signal)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGQUIT)

	interrupt := <-signals
	if finished != nil {
		*finished = true
	}

	// request stop, but wait for force exit
	for interrupt == syscall.SIGQUIT {
		log.Warningln("Requested quit, waiting for builds to finish")
		interrupt = <-signals
	}

	log.Warningln("Requested exit:", interrupt)

	go func() {
		for {
			abortSignal <- interrupt
		}
	}()

	select {
	case newSignal := <-signals:
		log.Fatalln("forced exit:", newSignal)
	case <-time.After(common.ShutdownTimeout * time.Second):
		log.Fatalln("shutdown timedout")
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

func (r *RunSingleCommand) processBuild(data common.ExecutorData, abortSignal chan os.Signal) (err error) {
	buildData, healthy := r.network.GetBuild(r.RunnerConfig)
	if !healthy {
		log.Println("Runner is not healthy!")
		select {
		case <-time.After(common.NotHealthyCheckInterval * time.Second):
		case <-abortSignal:
		}
		return
	}

	if buildData == nil {
		select {
		case <-time.After(common.CheckInterval):
		case <-abortSignal:
		}
		return
	}

	config := common.NewConfig()

	newBuild := common.Build{
		GetBuildResponse: *buildData,
		Runner:           &r.RunnerConfig,
		SystemInterrupt:  abortSignal,
		ExecutorData:     data,
	}

	buildCredentials := &common.JobCredentials{
		ID:    buildData.ID,
		Token: buildData.Token,
	}
	trace := r.network.ProcessBuild(r.RunnerConfig, buildCredentials)
	defer trace.Fail(err)

	err = newBuild.Run(config, trace)

	r.postBuild()

	return
}

func (r *RunSingleCommand) checkFinishedConditions() {
	if r.MaxBuilds < 1 && !r.runForever {
		log.Println("This runner has processed its build limit, so now exiting")
		r.finished = true
	}
	if r.WaitTimeout > 0 && int(time.Since(r.lastBuild).Seconds()) > r.WaitTimeout {
		log.Println("This runner has not received a job in", r.WaitTimeout, "seconds, so now exiting")
		r.finished = true
	}
	return
}

func (r *RunSingleCommand) Execute(c *cli.Context) {
	if len(r.URL) == 0 {
		log.Fatalln("Missing URL")
	}
	if len(r.Token) == 0 {
		log.Fatalln("Missing Token")
	}
	if len(r.Executor) == 0 {
		log.Fatalln("Missing Executor")
	}

	executorProvider := common.GetExecutor(r.Executor)
	if executorProvider == nil {
		log.Fatalln("Unknown executor:", r.Executor)
	}

	log.Println("Starting runner for", r.URL, "with token", r.ShortDescription(), "...")

	r.finished = false
	abortSignal := make(chan os.Signal)
	doneSignal := make(chan int, 1)
	r.runForever = r.MaxBuilds == 0

	go waitForInterrupts(&r.finished, abortSignal, doneSignal)

	r.lastBuild = time.Now()

	for !r.finished {
		data, err := executorProvider.Acquire(&r.RunnerConfig)
		if err != nil {
			log.Warningln("Executor update:", err)
		}

		r.processBuild(data, abortSignal)
		r.checkFinishedConditions()
		executorProvider.Release(&r.RunnerConfig, data)
	}

	doneSignal <- 0
}

func init() {
	common.RegisterCommand2("run-single", "start single runner", &RunSingleCommand{
		network: network.NewGitLabClient(),
	})
}
