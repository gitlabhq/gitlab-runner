package autoscaler

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"

	"gitlab.com/gitlab-org/fleeting/fleeting"
	"gitlab.com/gitlab-org/fleeting/fleeting/connector"
	flprometheus "gitlab.com/gitlab-org/fleeting/fleeting/metrics/prometheus"
	fleetingprovider "gitlab.com/gitlab-org/fleeting/fleeting/provider"
	"gitlab.com/gitlab-org/fleeting/nesting/api"
	"gitlab.com/gitlab-org/fleeting/taskscaler"
	tsprometheus "gitlab.com/gitlab-org/fleeting/taskscaler/metrics/prometheus"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

var (
	_ prometheus.Collector           = &provider{}
	_ common.ManagedExecutorProvider = &provider{}
)

type fleetingPlugin interface {
	InstanceGroup() fleetingprovider.InstanceGroup
	Kill()
}

type provider struct {
	common.ExecutorProvider
	cfg Config

	mu      sync.Mutex
	scalers map[string]scaler

	// Testing hooks
	//nolint:lll
	taskscalerNew     func(context.Context, fleetingprovider.InstanceGroup, ...taskscaler.Option) (taskscaler.Taskscaler, error)
	fleetingRunPlugin func(string, []byte) (fleetingPlugin, error)
	generateUniqueID  func() (string, error)
}

type scaler struct {
	internal taskscaler.Taskscaler
	shutdown func(context.Context)
	cancel   func()
}

type Config struct {
	// mapJobImageToVMImage allows the job defined image to control the VM
	// image used.
	//
	// Examples:
	// - For "instance" executor and VM Isolation enabled: the job image defines
	//   what nested VM is used on the host. We want to map the job image to
	//   the VM image.
	// - For "docker" executor and VM Isolation enabled: the job image defines what
	//   container is used, inside the nested VM, on the host. We *don't* want
	//   to map the job image to the VM image.
	MapJobImageToVMImage bool
}

func New(ep common.ExecutorProvider, cfg Config) common.ExecutorProvider {
	return &provider{
		ExecutorProvider: ep,
		cfg:              cfg,
		scalers:          make(map[string]scaler),
		taskscalerNew:    taskscaler.New,
		fleetingRunPlugin: func(name string, config []byte) (fleetingPlugin, error) {
			return fleeting.RunPlugin(name, config)
		},
		generateUniqueID: func() (string, error) {
			return helpers.GenerateRandomUUID(8)
		},
	}
}

func (p *provider) Init() {}

func (p *provider) Shutdown(ctx context.Context) {
	wg := new(sync.WaitGroup)
	for _, s := range p.scalers {
		wg.Add(1)
		go func(sc scaler) {
			defer wg.Done()
			sc.shutdown(ctx)
		}(s)
	}

	wg.Wait()
}

//nolint:funlen
func (p *provider) init(config *common.RunnerConfig) (taskscaler.Taskscaler, bool, error) {
	if config.Autoscaler == nil {
		return nil, false, fmt.Errorf("executor requires autoscaler config")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	s, ok := p.scalers[config.GetToken()]
	if ok {
		return s.internal, false, nil
	}

	pluginCfg, err := config.Autoscaler.PluginConfig.JSON()
	if err != nil {
		return nil, false, fmt.Errorf("marshaling plugin config: %w", err)
	}

	runner, err := p.fleetingRunPlugin(config.Autoscaler.Plugin, pluginCfg)
	if err != nil {
		return nil, false, fmt.Errorf("running autoscaler plugin: %w", err)
	}

	instanceConnectConfig := fleetingprovider.ConnectorConfig{
		OS:                   config.Autoscaler.ConnectorConfig.OS,
		Arch:                 config.Autoscaler.ConnectorConfig.Arch,
		Protocol:             fleetingprovider.Protocol(config.Autoscaler.ConnectorConfig.Protocol),
		Username:             config.Autoscaler.ConnectorConfig.Username,
		Password:             config.Autoscaler.ConnectorConfig.Password,
		UseStaticCredentials: config.Autoscaler.ConnectorConfig.UseStaticCredentials,
		Keepalive:            config.Autoscaler.ConnectorConfig.Keepalive,
		Timeout:              config.Autoscaler.ConnectorConfig.Timeout,
	}

	if config.Autoscaler.ConnectorConfig.KeyPathname != "" {
		key, err := os.ReadFile(config.Autoscaler.ConnectorConfig.KeyPathname)
		if err != nil {
			runner.Kill()

			return nil, false, fmt.Errorf("reading instance group connector key: %w", err)
		}
		instanceConnectConfig.Key = key
	}

	constLabels := prometheus.Labels{
		"runner":      config.ShortDescription(),
		"runner_name": config.Name,
	}

	tsMC := tsprometheus.New(constLabels)
	flMC := flprometheus.New(
		constLabels,
		config.Autoscaler.InstanceOperationTimeBuckets,
		config.Autoscaler.InstanceOperationTimeBuckets,
	)

	shutdownCtx, shutdownFn := context.WithCancel(context.Background())

	options := []taskscaler.Option{
		taskscaler.WithReservations(),
		taskscaler.WithCapacityPerInstance(config.Autoscaler.CapacityPerInstance),
		taskscaler.WithMaxUseCount(config.Autoscaler.MaxUseCount),
		taskscaler.WithMaxInstances(config.Autoscaler.MaxInstances),
		taskscaler.WithInstanceGroupSettings(fleetingprovider.Settings{
			ConnectorConfig: instanceConnectConfig,
		}),
		taskscaler.WithMetricsCollector(tsMC),
		taskscaler.WithFleetingMetricsCollector(flMC),
		taskscaler.WithInstanceUpFunc(instanceReadyUp(shutdownCtx, config)),
	}

	ctx, cancelFn := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancelFn()

	ts, err := p.taskscalerNew(ctx, runner.InstanceGroup(), options...)
	if err != nil {
		shutdownFn()
		runner.Kill()

		return nil, false, fmt.Errorf("creating taskscaler: %w", err)
	}

	s = scaler{
		internal: ts,
		shutdown: func(ctx context.Context) {
			shutdownFn()
			ts.Shutdown(ctx)
			runner.Kill()
		},
	}

	p.scalers[config.GetToken()] = s

	return s.internal, true, nil
}

//nolint:gocognit
func (p *provider) Acquire(config *common.RunnerConfig) (common.ExecutorData, error) {
	scaler, fresh, err := p.init(config)
	if err != nil {
		return nil, fmt.Errorf("initializing taskscaler: %w", err)
	}

	if fresh /* || todo: detect config updates - based on last modified timestamp? */ {
		var schedules []taskscaler.Schedule
		for _, schedule := range config.Autoscaler.Policy {
			schedules = append(schedules, taskscaler.Schedule{
				Periods:          schedule.Periods,
				Timezone:         schedule.Timezone,
				IdleCount:        schedule.IdleCount,
				IdleTime:         schedule.IdleTime,
				ScaleFactor:      schedule.ScaleFactor,
				ScaleFactorLimit: schedule.ScaleFactorLimit,
				PreemptiveMode:   schedule.IdleCount > 0,
			})
		}
		if err := scaler.ConfigureSchedule(schedules...); err != nil {
			return nil, fmt.Errorf("configuring taskscaler schedules: %w", err)
		}
	}

	// generate key for acquisition
	key, err := p.generateUniqueID()
	if err != nil {
		return nil, fmt.Errorf("generating unique id for task acquisition: %w", err)
	}
	key = helpers.ShortenToken(config.Token) + key

	if err := scaler.Reserve(key); err != nil {
		return nil, fmt.Errorf("reserving capacity: %w", err)
	}

	logrus.WithField("key", key).Trace("Reserved capacity...")

	return &acquisitionRef{
		key:                  key,
		mapJobImageToVMImage: p.cfg.MapJobImageToVMImage,
	}, nil
}

func (p *provider) Release(config *common.RunnerConfig, data common.ExecutorData) {
	acqRef, ok := data.(*acquisitionRef)
	if !ok {
		return
	}

	if acqRef.acq != nil {
		p.getRunnerTaskscaler(config).Release(acqRef.key)
		logrus.WithField("key", acqRef.key).Trace("Released capacity...")
		acqRef.acq = nil
		return
	}

	p.getRunnerTaskscaler(config).Unreserve(acqRef.key)
	logrus.WithField("key", acqRef.key).Trace("Unreserved capacity...")
}

func (p *provider) Create() common.Executor {
	e := p.ExecutorProvider.Create()
	if e == nil {
		return nil
	}

	return &executor{
		provider: p,
		Executor: e,
	}
}

func (p *provider) getRunnerTaskscaler(config *common.RunnerConfig) taskscaler.Taskscaler {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.scalers[config.GetToken()].internal
}

func (p *provider) Describe(ch chan<- *prometheus.Desc) {
	for _, scaler := range p.scalers {
		c, ok := scaler.internal.MetricsCollector().(prometheus.Collector)
		if ok {
			c.Describe(ch)
		}

		c, ok = scaler.internal.FleetingMetricsCollector().(prometheus.Collector)
		if ok {
			c.Describe(ch)
		}
	}
}

func (p *provider) Collect(ch chan<- prometheus.Metric) {
	for _, scaler := range p.scalers {
		c, ok := scaler.internal.MetricsCollector().(prometheus.Collector)
		if ok {
			c.Collect(ch)
		}

		c, ok = scaler.internal.FleetingMetricsCollector().(prometheus.Collector)
		if ok {
			c.Collect(ch)
		}
	}
}

//nolint:gocognit
func instanceReadyUp(ctx context.Context, config *common.RunnerConfig) taskscaler.UpFunc {
	if !config.Autoscaler.VMIsolation.Enabled {
		return nil
	}

	//nolint:lll
	return func(id string, info fleetingprovider.ConnectInfo, cause fleeting.Cause) (keys []string, used int, err error) {
		// dial host
		dialer, err := connector.Dial(ctx, info, connector.DialOptions{
			// todo: make this configurable
			UseExternalAddr: true,
		})
		if err != nil {
			return nil, 0, fmt.Errorf("dialing host: %w", err)
		}
		defer dialer.Close()

		//nolint:lll
		conn, err := api.NewClientConn(config.Autoscaler.VMIsolation.NestingHost, func(ctx context.Context, network, address string) (net.Conn, error) {
			return dialer.Dial(network, address)
		})
		if err != nil {
			return nil, 0, fmt.Errorf("dialing nesting daemon: %w", err)
		}

		nc := api.New(conn)
		defer nc.Close()

		nestingInitCfg, err := config.Autoscaler.VMIsolation.NestingConfig.JSON()
		if err != nil {
			return nil, 0, fmt.Errorf("converting nesting init config to json: %w", err)
		}

		err = nc.Init(ctx, nestingInitCfg)
		if err != nil && !errors.Is(err, api.ErrAlreadyInitialized) {
			return nil, 0, fmt.Errorf("initializing nesting: %w", err)
		}

		vms, err := nc.List(ctx)
		if err != nil {
			return nil, 0, fmt.Errorf("listing existing vms: %w", err)
		}

		keys = make([]string, len(vms))
		for i, vm := range vms {
			keys[i] = vm.GetId()
		}

		return keys, 0, nil
	}
}
