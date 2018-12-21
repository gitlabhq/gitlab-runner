package commands

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	_ "net/http/pprof" // pprof package adds everything itself inside its init() function
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/ayufan/golang-kardianos-service"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/certificate"
	prometheus_helper "gitlab.com/gitlab-org/gitlab-runner/helpers/prometheus"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/sentry"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/service"
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

	// runSignal is used to abort current operation (scaling workers, waiting for config)
	runSignal chan os.Signal

	// reloadSignal is used to trigger forceful config reload
	reloadSignal chan os.Signal

	// stopSignals is to catch a signals notified to process: SIGTERM, SIGQUIT, Interrupt, Kill
	stopSignals chan os.Signal

	// stopSignal is used to preserve the signal that was used to stop the
	// process In case this is SIGQUIT it makes to finish all builds and session
	// server.
	stopSignal os.Signal

	// runFinished is used to notify that Run() did finish
	runFinished chan bool

	currentWorkers int
}

func (mr *RunCommand) log() *logrus.Entry {
	return logrus.WithField("builds", mr.buildsHelper.buildsCount())
}

func (mr *RunCommand) feedRunner(runner *common.RunnerConfig, runners chan *common.RunnerConfig) {
	if !mr.isHealthy(runner.UniqueID()) {
		return
	}

	runners <- runner
}

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
}

// requestJob will check if the runner can send another concurrent request to
// GitLab, if not the return value is nil.
func (mr *RunCommand) requestJob(runner *common.RunnerConfig, sessionInfo *common.SessionInfo) *common.JobResponse {
	if !mr.buildsHelper.acquireRequest(runner) {
		mr.log().WithField("runner", runner.ShortDescription()).
			Debugln("Failed to request job: runner requestConcurrency meet")

		return nil
	}
	defer mr.buildsHelper.releaseRequest(runner)

	jobData, healthy := mr.network.RequestJob(*runner, sessionInfo)
	mr.makeHealthy(runner.UniqueID(), healthy)

	return jobData
}

func (mr *RunCommand) processRunner(id int, runner *common.RunnerConfig, runners chan *common.RunnerConfig) (err error) {
	provider := common.GetExecutor(runner.Executor)
	if provider == nil {
		return
	}

	executorData, releaseFn, err := mr.acquireRunnerResources(provider, runner)
	if err != nil {
		return
	}
	defer releaseFn()

	var features common.FeaturesInfo
	provider.GetFeatures(&features)
	buildSession, sessionInfo, err := mr.createSession(features)
	if err != nil {
		return
	}

	// Receive a new build
	jobData := mr.requestJob(runner, sessionInfo)
	if jobData == nil {
		return
	}

	// Make sure to always close output
	jobCredentials := &common.JobCredentials{
		ID:    jobData.ID,
		Token: jobData.Token,
	}
	trace := mr.network.ProcessJob(*runner, jobCredentials)
	defer func() {
		if err != nil {
			fmt.Fprintln(trace, err.Error())
			trace.Fail(err, common.RunnerSystemFailure)
		} else {
			trace.Fail(nil, common.NoneFailure)
		}
	}()

	trace.SetFailuresCollector(mr.failuresCollector)

	// Create a new build
	build, err := common.NewBuild(*jobData, runner, mr.abortBuilds, executorData)
	if err != nil {
		return
	}
	build.Session = buildSession

	// Add build to list of builds to assign numbers
	mr.buildsHelper.addBuild(build)
	defer mr.buildsHelper.removeBuild(build)

	// Process the same runner by different worker again
	// to speed up taking the builds
	select {
	case runners <- runner:
		mr.log().WithField("runner", runner.ShortDescription()).Debugln("Requeued the runner")

	default:
		mr.log().WithField("runner", runner.ShortDescription()).Debugln("Failed to requeue the runner: ")
	}

	// Process a build
	return build.Run(mr.config, trace)
}

func (mr *RunCommand) acquireRunnerResources(provider common.ExecutorProvider, runner *common.RunnerConfig) (common.ExecutorData, func(), error) {
	executorData, err := provider.Acquire(runner)
	if err != nil {
		return nil, func() {}, fmt.Errorf("failed to update executor: %v", err)
	}

	if !mr.buildsHelper.acquireBuild(runner) {
		provider.Release(runner, executorData)
		return nil, nil, errors.New("failed to request job, runner limit met")
	}

	releaseFn := func() {
		mr.buildsHelper.releaseBuild(runner)
		provider.Release(runner, executorData)
	}

	return executorData, releaseFn, nil
}

func (mr *RunCommand) createSession(features common.FeaturesInfo) (*session.Session, *common.SessionInfo, error) {
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

func (mr *RunCommand) processRunners(id int, stopWorker chan bool, runners chan *common.RunnerConfig) {
	mr.log().WithField("worker", id).Debugln("Starting worker")
	for mr.stopSignal == nil {
		select {
		case runner := <-runners:
			err := mr.processRunner(id, runner, runners)
			if err != nil {
				mr.log().WithFields(logrus.Fields{
					"runner":   runner.ShortDescription(),
					"executor": runner.Executor,
				}).WithError(err).
					Error("Failed to process runner")
			}

			// force GC cycle after processing build
			runtime.GC()

		case <-stopWorker:
			mr.log().WithField("worker", id).Debugln("Stopping worker")
			return
		}
	}
	<-stopWorker
}

func (mr *RunCommand) startWorkers(startWorker chan int, stopWorker chan bool, runners chan *common.RunnerConfig) {
	for mr.stopSignal == nil {
		id := <-startWorker
		go mr.processRunners(id, stopWorker, runners)
	}
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

func (mr *RunCommand) Start(s service.Service) error {
	mr.abortBuilds = make(chan os.Signal)
	mr.runSignal = make(chan os.Signal, 1)
	mr.reloadSignal = make(chan os.Signal, 1)
	mr.runFinished = make(chan bool, 1)
	mr.stopSignals = make(chan os.Signal)
	mr.log().Println("Starting multi-runner from", mr.ConfigFile, "...")

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
	go mr.Run()

	return nil
}

func (mr *RunCommand) updateWorkers(workerIndex *int, startWorker chan int, stopWorker chan bool) os.Signal {
	buildLimit := mr.config.Concurrent

	if buildLimit < 1 {
		mr.log().Fatalln("Concurrent is less than 1 - no jobs will be processed")
	}

	for mr.currentWorkers > buildLimit {
		select {
		case stopWorker <- true:
		case signaled := <-mr.runSignal:
			return signaled
		}
		mr.currentWorkers--
	}

	for mr.currentWorkers < buildLimit {
		select {
		case startWorker <- *workerIndex:
		case signaled := <-mr.runSignal:
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

	case signaled := <-mr.runSignal:
		return signaled
	}
	return nil
}

func (mr *RunCommand) runWait() {
	mr.log().Debugln("Waiting for stop signal")

	// Save the stop signal and exit to execute Stop()
	mr.stopSignal = <-mr.stopSignals
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

	mux.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
}

func (mr *RunCommand) serveDebugData(mux *http.ServeMux) {
	mux.HandleFunc("/debug/jobs/list", mr.buildsHelper.ListJobsHandler)
}

func (mr *RunCommand) setupMetricsAndDebugServer() {
	listenAddress, err := mr.listenAddress()

	if err != nil {
		mr.log().Errorf("invalid listen address: %s", err.Error())
		return
	}

	if listenAddress == "" {
		mr.log().Info("Listen address not defined, metrics server disabled")
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

	mr.log().
		WithField("address", listenAddress).
		Info("Metrics server listening")
}

func (mr *RunCommand) setupSessionServer() {
	if mr.config.SessionServer.ListenAddress == "" {
		mr.log().Info("Listen address not defined, session server disabled")
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

func (mr *RunCommand) Run() {
	mr.setupMetricsAndDebugServer()
	mr.setupSessionServer()

	runners := make(chan *common.RunnerConfig)
	go mr.feedRunners(runners)

	signal.Notify(mr.stopSignals, syscall.SIGQUIT, syscall.SIGTERM, os.Interrupt, os.Kill)
	signal.Notify(mr.reloadSignal, syscall.SIGHUP)

	startWorker := make(chan int)
	stopWorker := make(chan bool)
	go mr.startWorkers(startWorker, stopWorker, runners)

	workerIndex := 0

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
	mr.log().Println("All workers stopped. Can exit now")
	mr.runFinished <- true
}

func (mr *RunCommand) interruptRun() {
	// Pump interrupt signal
	for {
		mr.runSignal <- mr.stopSignal
	}
}

func (mr *RunCommand) abortAllBuilds() {
	// Pump signal to abort all current builds
	for {
		mr.abortBuilds <- mr.stopSignal
	}
}

func (mr *RunCommand) handleGracefulShutdown() error {
	// We wait till we have a SIGQUIT
	for mr.stopSignal == syscall.SIGQUIT {
		mr.log().Warningln("Requested quit, waiting for builds to finish")

		// Wait for other signals to finish builds
		select {
		case mr.stopSignal = <-mr.stopSignals:
		// We received a new signal

		case <-mr.runFinished:
			// Everything finished we can exit now
			return nil
		}
	}

	return fmt.Errorf("received: %v", mr.stopSignal)
}

func (mr *RunCommand) handleShutdown() error {
	mr.log().Warningln("Requested service stop:", mr.stopSignal)

	go mr.abortAllBuilds()

	if mr.sessionServer != nil {
		mr.sessionServer.Close()
	}

	// Wait for graceful shutdown or abort after timeout
	for {
		select {
		case mr.stopSignal = <-mr.stopSignals:
			return fmt.Errorf("forced exit: %v", mr.stopSignal)

		case <-time.After(common.ShutdownTimeout * time.Second):
			return errors.New("shutdown timed out")

		case <-mr.runFinished:
			// Everything finished we can exit now
			return nil
		}
	}
}

func (mr *RunCommand) Stop(s service.Service) (err error) {
	go mr.interruptRun()
	err = mr.handleGracefulShutdown()
	if err == nil {
		return
	}
	err = mr.handleShutdown()
	return
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

func (mr *RunCommand) Execute(context *cli.Context) {
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
		logrus.Fatalln(err)
	}

	if mr.Syslog {
		log.SetSystemLogger(logrus.StandardLogger(), svc)
	}

	logrus.AddHook(&mr.sentryLogHook)
	logrus.AddHook(&mr.prometheusLogHook)

	err = svc.Run()
	if err != nil {
		logrus.Fatalln(err)
	}
}

func init() {
	requestStatusesCollector := network.NewAPIRequestStatusesMap()

	common.RegisterCommand2("run", "run multi runner service", &RunCommand{
		ServiceName: defaultServiceName,
		network:     network.NewGitLabClientWithRequestStatusesMap(requestStatusesCollector),
		networkRequestStatusesCollector: requestStatusesCollector,
		prometheusLogHook:               prometheus_helper.NewLogHook(),
		failuresCollector:               prometheus_helper.NewFailuresCollector(),
		buildsHelper:                    newBuildsHelper(),
	})
}
