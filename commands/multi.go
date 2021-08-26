package commands

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/kardianos/service"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/certificate"
	prometheus_helper "gitlab.com/gitlab-org/gitlab-runner/helpers/prometheus"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/sentry"
	service_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/service"
	"gitlab.com/gitlab-org/gitlab-runner/log"
	"gitlab.com/gitlab-org/gitlab-runner/network"
	"gitlab.com/gitlab-org/gitlab-runner/session"
)

var (
	concurrentDesc = prometheus.NewDesc(
		"gitlab_runner_concurrent",
		"The current value of concurrent setting",
		nil,
		nil,
	)

	limitDesc = prometheus.NewDesc(
		"gitlab_runner_limit",
		"The current value of concurrent setting",
		[]string{"runner"},
		nil,
	)
)

type RunCommand struct {
	configOptionsWithListenAddress
	network common.Network
	healthHelper

	buildsHelper buildsHelper

	ServiceName      string `short:"n" long:"service" description:"Use different names for different services"`
	WorkingDirectory string `short:"d" long:"working-directory" description:"Specify custom working directory"`
	User             string `short:"u" long:"user" description:"Use specific user to execute shell scripts"`
	Syslog           bool   `long:"syslog" description:"Log to system service logger" env:"LOG_SYSLOG"`

	sentryLogHook     sentry.LogHook
	prometheusLogHook prometheus_helper.LogHook

	failuresCollector               *prometheus_helper.FailuresCollector
	networkRequestStatusesCollector prometheus.Collector

	sessionServer *session.Server

	// abortBuilds is used to abort running builds
	abortBuilds chan os.Signal

	// runInterruptSignal is used to abort current operation (scaling workers, waiting for config)
	runInterruptSignal chan os.Signal

	// reloadSignal is used to trigger forceful config reload
	reloadSignal chan os.Signal

	// stopSignals is to catch a signals notified to process: SIGTERM, SIGQUIT, Interrupt, Kill
	stopSignals chan os.Signal

	// stopSignal is used to preserve the signal that was used to stop the
	// process In case this is SIGQUIT it makes to finish all builds and session
	// server.
	stopSignal os.Signal

	// runFinished is used to notify that run() did finish
	runFinished chan bool

	currentWorkers int
}

func (mr *RunCommand) log() *logrus.Entry {
	return logrus.WithField("builds", mr.buildsHelper.buildsCount())
}

// Start is the method implementing `github.com/kardianos/service`.`Interface`
// interface. It's responsible for a non-blocking initialization of the process. When it exits,
// the main control flow is passed to runWait() configured as service's RunWait method. Take a look
// into Execute() for details.
func (mr *RunCommand) Start(_ service.Service) error {
	mr.abortBuilds = make(chan os.Signal)
	mr.runInterruptSignal = make(chan os.Signal, 1)
	mr.reloadSignal = make(chan os.Signal, 1)
	mr.runFinished = make(chan bool, 1)
	mr.stopSignals = make(chan os.Signal)

	mr.log().Info("Starting multi-runner from ", mr.ConfigFile, "...")

	userModeWarning(false)

	if len(mr.WorkingDirectory) > 0 {
		err := os.Chdir(mr.WorkingDirectory)
		if err != nil {
			return err
		}
	}

	err := mr.loadConfig()
	if err != nil {
		return err
	}

	// Start should not block. Do the actual work async.
	go mr.run()

	return nil
}

func (mr *RunCommand) loadConfig() error {
	err := mr.configOptions.loadConfig()
	if err != nil {
		return err
	}

	// Set log level
	err = mr.updateLoggingConfiguration()
	if err != nil {
		return err
	}

	// pass user to execute scripts as specific user
	if mr.User != "" {
		mr.config.User = mr.User
	}

	mr.healthy = nil
	mr.log().Println("Configuration loaded")
	mr.log().Debugln(helpers.ToYAML(mr.config))

	// initialize sentry
	if mr.config.SentryDSN != nil {
		var err error
		mr.sentryLogHook, err = sentry.NewLogHook(*mr.config.SentryDSN)
		if err != nil {
			mr.log().WithError(err).Errorln("Sentry failure")
		}
	} else {
		mr.sentryLogHook = sentry.LogHook{}
	}

	return nil
}

func (mr *RunCommand) updateLoggingConfiguration() error {
	reloadNeeded := false

	if mr.config.LogLevel != nil && !log.Configuration().IsLevelSetWithCli() {
		err := log.Configuration().SetLevel(*mr.config.LogLevel)
		if err != nil {
			return err
		}

		reloadNeeded = true
	}

	if mr.config.LogFormat != nil && !log.Configuration().IsFormatSetWithCli() {
		err := log.Configuration().SetFormat(*mr.config.LogFormat)
		if err != nil {
			return err
		}

		reloadNeeded = true
	}

	if reloadNeeded {
		log.Configuration().ReloadConfiguration()
	}

	return nil
}

// run is the main method of RunCommand. It's started asynchronously by services support
// through `Start` method and is responsible for initializing all goroutines handling
// concurrent, multi-runner execution of jobs.
// When mr.stopSignal is broadcasted (after `Stop` is called by services support)
// this method waits for all workers to be terminated and closes the mr.runFinished
// channel, which is the signal that the command was properly terminated (this is the only
// valid, properly terminated exit flow for `gitlab-runner run`).
func (mr *RunCommand) run() {
	mr.setupMetricsAndDebugServer()
	mr.setupSessionServer()

	runners := make(chan *common.RunnerConfig)
	go mr.feedRunners(runners)

	signal.Notify(mr.stopSignals, syscall.SIGQUIT, syscall.SIGTERM, os.Interrupt)
	signal.Notify(mr.reloadSignal, syscall.SIGHUP)

	startWorker := make(chan int)
	stopWorker := make(chan bool)
	go mr.startWorkers(startWorker, stopWorker, runners)

	workerIndex := 0

	// Update number of workers and reload configuration.
	// Exits when mr.runInterruptSignal receives a signal.
	for mr.stopSignal == nil {
		signaled := mr.updateWorkers(&workerIndex, startWorker, stopWorker)
		if signaled != nil {
			break
		}

		signaled = mr.updateConfig()
		if signaled != nil {
			break
		}
	}

	// Wait for workers to shutdown
	for mr.currentWorkers > 0 {
		stopWorker <- true
		mr.currentWorkers--
	}

	mr.log().Info("All workers stopped. Can exit now")

	close(mr.runFinished)
}

func (mr *RunCommand) setupMetricsAndDebugServer() {
	listenAddress, err := mr.listenAddress()

	if err != nil {
		mr.log().Errorf("invalid listen address: %s", err.Error())
		return
	}

	if listenAddress == "" {
		mr.log().Info("listen_address not defined, metrics & debug endpoints disabled")
		return
	}

	// We separate out the listener creation here so that we can return an error if
	// the provided address is invalid or there is some other listener error.
	listener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		mr.log().WithError(err).Fatal("Failed to create listener for metrics server")
	}

	mux := http.NewServeMux()

	go func() {
		err := http.Serve(listener, mux)

		if err != nil {
			mr.log().WithError(err).Fatal("Metrics server terminated")
		}
	}()

	mr.serveMetrics(mux)
	mr.serveDebugData(mux)
	mr.servePprof(mux)

	mr.log().
		WithField("address", listenAddress).
		Info("Metrics server listening")
}

func (mr *RunCommand) serveMetrics(mux *http.ServeMux) {
	registry := prometheus.NewRegistry()
	// Metrics about the runner's business logic.
	registry.MustRegister(&mr.buildsHelper)
	registry.MustRegister(mr)
	// Metrics about API connections
	registry.MustRegister(mr.networkRequestStatusesCollector)
	// Metrics about jobs failures
	registry.MustRegister(mr.failuresCollector)
	// Metrics about catched errors
	registry.MustRegister(&mr.prometheusLogHook)
	// Metrics about the program's build version.
	registry.MustRegister(common.AppVersion.NewMetricsCollector())
	// Go-specific metrics about the process (GC stats, goroutines, etc.).
	registry.MustRegister(prometheus.NewGoCollector())
	// Go-unrelated process metrics (memory usage, file descriptors, etc.).
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	// Register all executor provider collectors
	for _, provider := range common.GetExecutorProviders() {
		if collector, ok := provider.(prometheus.Collector); ok && collector != nil {
			registry.MustRegister(collector)
		}
	}

	// restrictHTTPMethods should be used on all promhttp handlers
	// In this specific instance, the handler is unintrumented, so isn't as
	// important. But in the future, if any other promhttp handlers are added
	// they too should be wrapped and restriced.
	// https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27194
	mux.Handle(
		"/metrics",
		restrictHTTPMethods(
			promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
			http.MethodGet, http.MethodHead,
		),
	)
}

func (mr *RunCommand) serveDebugData(mux *http.ServeMux) {
	mux.HandleFunc("/debug/jobs/list", mr.buildsHelper.ListJobsHandler)
}

func (mr *RunCommand) servePprof(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
}

// restrictHTTPMethods wraps a http.Handler and returns a http.Handler that
// restricts methods only to those provided.
func restrictHTTPMethods(handler http.Handler, methods ...string) http.Handler {
	supported := map[string]struct{}{}
	for _, method := range methods {
		supported[method] = struct{}{}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := supported[r.Method]; !ok {
			http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
			return
		}

		handler.ServeHTTP(w, r)
	})
}

func (mr *RunCommand) setupSessionServer() {
	if mr.config.SessionServer.ListenAddress == "" {
		mr.log().Info("[session_server].listen_address not defined, session endpoints disabled")
		return
	}

	var err error
	mr.sessionServer, err = session.NewServer(
		session.ServerConfig{
			AdvertiseAddress: mr.config.SessionServer.AdvertiseAddress,
			ListenAddress:    mr.config.SessionServer.ListenAddress,
			ShutdownTimeout:  common.ShutdownTimeout * time.Second,
		},
		mr.log(),
		certificate.X509Generator{},
		mr.buildsHelper.findSessionByURL,
	)
	if err != nil {
		mr.log().WithError(err).Fatal("Failed to create session server")
	}

	go func() {
		err := mr.sessionServer.Start()
		if err != nil {
			mr.log().WithError(err).Fatal("Session server terminated")
		}
	}()

	mr.log().
		WithField("address", mr.config.SessionServer.ListenAddress).
		Info("Session server listening")
}

// feedRunners works until a stopSignal was saved.
// It is responsible for feeding the runners (workers) to channel, which
// asynchronously ends with job requests being made and jobs being executed
// by concurrent workers.
// This is also the place where check interval is calculated and
// applied.
func (mr *RunCommand) feedRunners(runners chan *common.RunnerConfig) {
	for mr.stopSignal == nil {
		mr.log().Debugln("Feeding runners to channel")
		config := mr.config

		// If no runners wait full interval to test again
		if len(config.Runners) == 0 {
			time.Sleep(config.GetCheckInterval())
			continue
		}

		interval := config.GetCheckInterval() / time.Duration(len(config.Runners))

		// Feed runner with waiting exact amount of time
		for _, runner := range config.Runners {
			mr.feedRunner(runner, runners)
			time.Sleep(interval)
		}
	}

	mr.log().
		WithField("StopSignal", mr.stopSignal).
		Debug("Stopping feeding runners to channel")
}

func (mr *RunCommand) feedRunner(runner *common.RunnerConfig, runners chan *common.RunnerConfig) {
	if !mr.isHealthy(runner.UniqueID()) {
		return
	}

	runners <- runner
}

// startWorkers is responsible for starting the workers (up to the number
// defined by `concurrent`) and assigning a runner processing method to them.
func (mr *RunCommand) startWorkers(startWorker chan int, stopWorker chan bool, runners chan *common.RunnerConfig) {
	for mr.stopSignal == nil {
		id := <-startWorker
		go mr.processRunners(id, stopWorker, runners)
	}
}

// processRunners is responsible for processing a Runner on a worker (when received
// a runner information sent to the channel by feedRunners) and for terminating the worker
// (when received an information on stoWorker chan - provided by updateWorkers)
func (mr *RunCommand) processRunners(id int, stopWorker chan bool, runners chan *common.RunnerConfig) {
	mr.log().
		WithField("worker", id).
		Debugln("Starting worker")

	for mr.stopSignal == nil {
		select {
		case runner := <-runners:
			err := mr.processRunner(id, runner, runners)
			if err != nil {
				logger := mr.log().WithFields(logrus.Fields{
					"runner":   runner.ShortDescription(),
					"executor": runner.Executor,
				}).WithError(err)

				var NoFreeExecutorError *common.NoFreeExecutorError
				if errors.As(err, &NoFreeExecutorError) {
					logger.Debug("Failed to process runner")
				} else {
					logger.Warn("Failed to process runner")
				}
			}

			// force GC cycle after processing build
			runtime.GC()

		case <-stopWorker:
			mr.log().
				WithField("worker", id).
				Debugln("Stopping worker")
			return
		}
	}
	<-stopWorker
}

// processRunner is responsible for handling one job on a specified runner.
// First it acquires the Build to check if `limit` was met. If it's still in the capacity
// it creates the debug session (for debug terminal), triggers a job request to configured
// GitLab instance and finally creates and finishes the job.
// To speed-up jobs handling before starting the job this method "requeues" the runner to another
// worker (by feeding the channel normally handled by feedRunners).
func (mr *RunCommand) processRunner(
	id int,
	runner *common.RunnerConfig,
	runners chan *common.RunnerConfig,
) (err error) {
	provider := common.GetExecutorProvider(runner.Executor)
	if provider == nil {
		return
	}

	executorData, err := provider.Acquire(runner)
	if err != nil {
		return fmt.Errorf("failed to update executor: %w", err)
	}
	defer provider.Release(runner, executorData)

	if !mr.buildsHelper.acquireBuild(runner) {
		logrus.WithFields(logrus.Fields{
			"runner": runner.ShortDescription(),
			"worker": id,
		}).Debug("Failed to request job, runner limit met")
		return
	}
	defer mr.buildsHelper.releaseBuild(runner)

	buildSession, sessionInfo, err := mr.createSession(provider)
	if err != nil {
		return
	}

	// Receive a new build
	trace, jobData, err := mr.requestJob(runner, sessionInfo)
	if err != nil || jobData == nil {
		return
	}
	defer func() { mr.traceOutcome(trace, err) }()

	// Create a new build
	build, err := common.NewBuild(*jobData, runner, mr.abortBuilds, executorData)
	if err != nil {
		return
	}
	build.Session = buildSession
	build.ArtifactUploader = mr.network.UploadRawArtifacts

	// Add build to list of builds to assign numbers
	mr.buildsHelper.addBuild(build)
	defer mr.buildsHelper.removeBuild(build)

	// Process the same runner by different worker again
	// to speed up taking the builds
	mr.requeueRunner(runner, runners)

	// Process a build
	return build.Run(mr.config, trace)
}

func (mr *RunCommand) traceOutcome(trace common.JobTrace, err error) {
	if err != nil {
		fmt.Fprintln(trace, err.Error())
		trace.Fail(err, common.JobFailureData{Reason: common.RunnerSystemFailure})
	} else {
		trace.Success()
	}
}

// createSession checks if debug server is supported by configured executor and if the
// debug server was configured. If both requirements are met, then it creates a debug session
// that will be assigned to newly created job.
func (mr *RunCommand) createSession(provider common.ExecutorProvider) (*session.Session, *common.SessionInfo, error) {
	var features common.FeaturesInfo

	if err := provider.GetFeatures(&features); err != nil {
		return nil, nil, err
	}

	if mr.sessionServer == nil || !features.Session {
		return nil, nil, nil
	}

	sess, err := session.NewSession(mr.log())
	if err != nil {
		return nil, nil, err
	}

	sessionInfo := &common.SessionInfo{
		URL:           mr.sessionServer.AdvertiseAddress + sess.Endpoint,
		Certificate:   string(mr.sessionServer.CertificatePublicKey),
		Authorization: sess.Token,
	}

	return sess, sessionInfo, err
}

// requestJob will check if the runner can send another concurrent request to
// GitLab, if not the return value is nil.
func (mr *RunCommand) requestJob(
	runner *common.RunnerConfig,
	sessionInfo *common.SessionInfo,
) (common.JobTrace, *common.JobResponse, error) {
	if !mr.buildsHelper.acquireRequest(runner) {
		mr.log().WithField("runner", runner.ShortDescription()).
			Debugln("Failed to request job: runner requestConcurrency meet")
		return nil, nil, nil
	}
	defer mr.buildsHelper.releaseRequest(runner)

	jobData, healthy := mr.doJobRequest(context.Background(), runner, sessionInfo)
	mr.makeHealthy(runner.UniqueID(), healthy)

	if jobData == nil {
		return nil, nil, nil
	}

	// Make sure to always close output
	jobCredentials := &common.JobCredentials{
		ID:    jobData.ID,
		Token: jobData.Token,
	}

	trace, err := mr.network.ProcessJob(*runner, jobCredentials)
	if err != nil {
		jobInfo := common.UpdateJobInfo{
			ID:            jobCredentials.ID,
			State:         common.Failed,
			FailureReason: common.RunnerSystemFailure,
		}

		// send failure once
		mr.network.UpdateJob(*runner, jobCredentials, jobInfo)
		return nil, nil, err
	}

	trace.SetFailuresCollector(mr.failuresCollector)
	return trace, jobData, nil
}

// doJobRequest will execute the request for a new job, respecting an interruption
// caused by interrupt signals or process execution finalization
func (mr *RunCommand) doJobRequest(
	ctx context.Context,
	runner *common.RunnerConfig,
	sessionInfo *common.SessionInfo,
) (*common.JobResponse, bool) {
	// Terminate opened requests to GitLab when interrupt signal
	// is broadcast.
	ctx, cancelFn := context.WithCancel(ctx)
	defer cancelFn()

	go func() {
		select {
		case <-mr.runInterruptSignal:
			cancelFn()
		case <-mr.runFinished:
			cancelFn()
		case <-ctx.Done():
		}
	}()

	return mr.network.RequestJob(ctx, *runner, sessionInfo)
}

// requeueRunner feeds the runners channel in a non-blocking way. This replicates the
// behavior of feedRunners and speeds-up jobs handling. But if the channel is full, the
// method just exits without blocking.
func (mr *RunCommand) requeueRunner(runner *common.RunnerConfig, runners chan *common.RunnerConfig) {
	runnerLog := mr.log().WithField("runner", runner.ShortDescription())

	select {
	case runners <- runner:
		runnerLog.Debugln("Requeued the runner")

	default:
		runnerLog.Debugln("Failed to requeue the runner")
	}
}

// updateWorkers, called periodically from run() is responsible for scaling the pool
// of workers. By worker we don't understand a `[[runners]]` entry, but a "slot" that will
// use one of the runners to request and handle a job.
// The size of the workers pool is controlled by `concurrent` setting. This method is responsible
// for the fact that `concurrent` defines the upper number of jobs that can be concurrently handled
// by GitLab Runner process.
func (mr *RunCommand) updateWorkers(workerIndex *int, startWorker chan int, stopWorker chan bool) os.Signal {
	concurrentLimit := mr.config.Concurrent

	if concurrentLimit < 1 {
		mr.log().Fatalln("Concurrent is less than 1 - no jobs will be processed")
	}

	for mr.currentWorkers > concurrentLimit {
		// Too many workers. Trigger stop on one of them
		// or exit if termination signal was broadcasted.
		select {
		case stopWorker <- true:
		case signaled := <-mr.runInterruptSignal:
			return signaled
		}
		mr.currentWorkers--
	}

	for mr.currentWorkers < concurrentLimit {
		// Too few workers. Trigger a creation of a new one
		// or exit if termination signal was broadcasted.
		select {
		case startWorker <- *workerIndex:
		case signaled := <-mr.runInterruptSignal:
			return signaled
		}
		mr.currentWorkers++
		*workerIndex++
	}

	return nil
}

func (mr *RunCommand) updateConfig() os.Signal {
	select {
	case <-time.After(common.ReloadConfigInterval * time.Second):
		err := mr.checkConfig()
		if err != nil {
			mr.log().Errorln("Failed to load config", err)
		}

	case <-mr.reloadSignal:
		err := mr.loadConfig()
		if err != nil {
			mr.log().Errorln("Failed to load config", err)
		}

	case signaled := <-mr.runInterruptSignal:
		return signaled
	}

	return nil
}

func (mr *RunCommand) checkConfig() (err error) {
	info, err := os.Stat(mr.ConfigFile)
	if err != nil {
		return err
	}

	if !mr.config.ModTime.Before(info.ModTime()) {
		return nil
	}

	err = mr.loadConfig()
	if err != nil {
		mr.log().Errorln("Failed to load config", err)
		// don't reload the same file
		mr.config.ModTime = info.ModTime()
		return
	}
	return nil
}

// Stop is the method implementing `github.com/kardianos/service`.`Interface`
// interface. It's responsible for triggering the process stop.
// First it starts a goroutine that starts broadcasting the interrupt signal (used to stop
// workers scaling goroutine).
// Next it triggers graceful shutdown, which will be handled only if a proper signal is used.
// At the end it triggers the forceful shutdown, which handles the forceful the process termination.
func (mr *RunCommand) Stop(_ service.Service) error {
	if mr.stopSignal == nil {
		mr.stopSignal = os.Interrupt
	}

	go mr.interruptRun()

	defer func() {
		if mr.sessionServer != nil {
			mr.sessionServer.Close()
		}
	}()

	// On Windows, we convert SIGTERM and SIGINT signals into a SIGQUIT.
	//
	// This enforces *graceful* termination on the first signal received, and a forceful shutdown
	// on the second.
	//
	// This slightly differs from other operating systems. On other systems, receiving a SIGQUIT
	// works the same way (gracefully) but receiving a SIGTERM and SIGQUIT always results
	// in an immediate forceful shutdown.
	//
	// This handling has to be different as SIGQUIT is not a signal the os/signal package translates
	// any Windows control concepts to.
	if runtime.GOOS == "windows" {
		mr.stopSignal = syscall.SIGQUIT
	}

	err := mr.handleGracefulShutdown()
	if err == nil {
		return nil
	}

	mr.log().
		WithError(err).
		Warning("Graceful shutdown not finished properly")

	err = mr.handleForcefulShutdown()
	if err == nil {
		return nil
	}

	mr.log().
		WithError(err).
		Warning("Forceful shutdown not finished properly")

	return err
}

// interruptRun broadcasts interrupt signal, which exits the workers
// scaling goroutine.
func (mr *RunCommand) interruptRun() {
	mr.log().Debug("Broadcasting interrupt signal")

	// Pump interrupt signal
	for {
		mr.runInterruptSignal <- mr.stopSignal
	}
}

// handleGracefulShutdown is responsible for handling the "graceful" strategy of exiting.
// It's executed only when specific signal is used to terminate the process.
// At this moment feedRunners() should exit and workers scaling is being terminated.
// This means that new jobs will be not requested. handleGracefulShutdown() will ensure that
// the process will not exit until `mr.runFinished` is closed, so all jobs were finished and
// all workers terminated. It may however exit if another signal - other than the gracefulShutdown
// signal - is received.
func (mr *RunCommand) handleGracefulShutdown() error {
	// We wait till we have a SIGQUIT
	for mr.stopSignal == syscall.SIGQUIT {
		mr.log().
			WithField("StopSignal", mr.stopSignal).
			Warning("Starting graceful shutdown, waiting for builds to finish")

		// Wait for other signals to finish builds
		select {
		case mr.stopSignal = <-mr.stopSignals:
			// We received a new signal
			mr.log().WithField("stop-signal", mr.stopSignal).Warning("[handleGracefulShutdown] received stop signal")

		case <-mr.runFinished:
			// Everything finished we can exit now
			return nil
		}
	}

	return fmt.Errorf("received stop signal: %v", mr.stopSignal)
}

// handleForcefulShutdown is executed if handleGracefulShutdown exited with an error
// (which means that a signal forcing shutdown was used instead of the signal
// specific for graceful shutdown).
// It calls mr.abortAllBuilds which will broadcast abort signal which finally
// ends with jobs termination.
// Next it waits for one of the following events:
// 1. Another signal was sent to process, which is handled as force exit and
//    triggers exit of the method and finally process termination without
//    waiting for anything else.
// 2. ShutdownTimeout is exceeded. If waiting for shutdown will take more than
//    defined time, the process will be forceful terminated just like in the
//    case when second signal is sent.
// 3. mr.runFinished was closed, which means that all termination was done
//    properly.
//
// After this method exits, Stop returns it error and finally the
// `github.com/kardianos/service` service mechanism will finish
// process execution.
func (mr *RunCommand) handleForcefulShutdown() error {
	mr.log().
		WithField("StopSignal", mr.stopSignal).
		Warning("Starting forceful shutdown")

	go mr.abortAllBuilds()

	// Wait for graceful shutdown or abort after timeout
	for {
		select {
		case mr.stopSignal = <-mr.stopSignals:
			mr.log().WithField("stop-signal", mr.stopSignal).Warning("[handleForcefulShutdown] received stop signal")
			return fmt.Errorf("forced exit with stop signal: %v", mr.stopSignal)

		case <-time.After(common.ShutdownTimeout * time.Second):
			return errors.New("shutdown timed out")

		case <-mr.runFinished:
			// Everything finished we can exit now
			return nil
		}
	}
}

// abortAllBuilds broadcasts abort signal, which ends with all currently executed
// jobs being interrupted and terminated.
func (mr *RunCommand) abortAllBuilds() {
	mr.log().Debug("Broadcasting job abort signal")

	// Pump signal to abort all current builds
	for {
		mr.abortBuilds <- mr.stopSignal
	}
}

func (mr *RunCommand) Execute(_ *cli.Context) {
	svcConfig := &service.Config{
		Name:        mr.ServiceName,
		DisplayName: mr.ServiceName,
		Description: defaultDescription,
		Arguments:   []string{"run"},
		Option: service.KeyValue{
			"RunWait": mr.runWait,
		},
	}

	svc, err := service_helpers.New(mr, svcConfig)
	if err != nil {
		logrus.WithError(err).
			Fatalln("Service creation failed")
	}

	if mr.Syslog {
		log.SetSystemLogger(logrus.StandardLogger(), svc)
	}

	logrus.AddHook(&mr.sentryLogHook)
	logrus.AddHook(&mr.prometheusLogHook)

	err = svc.Run()
	if err != nil {
		logrus.WithError(err).
			Fatal("Service run failed")
	}
}

// runWait is the blocking mechanism for `github.com/kardianos/service`
// service. It's started after Start exited and should block the control flow. When it exits,
// then the Stop is executed and service shutdown should be handled.
// For Runner it waits for the stopSignal to be received by the process. When it will happen,
// it's saved in mr.stopSignal and runWait() exits, triggering the shutdown handling.
func (mr *RunCommand) runWait() {
	mr.log().Debugln("Waiting for stop signal")

	// Save the stop signal and exit to execute Stop()
	stopSignal := <-mr.stopSignals
	mr.stopSignal = stopSignal
	mr.log().WithField("stop-signal", stopSignal).Warning("[runWait] received stop signal")
}

// Describe implements prometheus.Collector.
func (mr *RunCommand) Describe(ch chan<- *prometheus.Desc) {
	ch <- concurrentDesc
	ch <- limitDesc
}

// Collect implements prometheus.Collector.
func (mr *RunCommand) Collect(ch chan<- prometheus.Metric) {
	config := mr.config

	ch <- prometheus.MustNewConstMetric(
		concurrentDesc,
		prometheus.GaugeValue,
		float64(config.Concurrent),
	)

	for _, runner := range config.Runners {
		ch <- prometheus.MustNewConstMetric(
			limitDesc,
			prometheus.GaugeValue,
			float64(runner.Limit),
			runner.ShortDescription(),
		)
	}
}

func init() {
	requestStatusesCollector := network.NewAPIRequestStatusesMap()

	common.RegisterCommand2(
		"run",
		"run multi runner service",
		&RunCommand{
			ServiceName:                     defaultServiceName,
			network:                         network.NewGitLabClientWithRequestStatusesMap(requestStatusesCollector),
			networkRequestStatusesCollector: requestStatusesCollector,
			prometheusLogHook:               prometheus_helper.NewLogHook(),
			failuresCollector:               prometheus_helper.NewFailuresCollector(),
			buildsHelper:                    newBuildsHelper(),
		},
	)
}
