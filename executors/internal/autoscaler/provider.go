package autoscaler

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"gitlab.com/gitlab-org/fleeting/fleeting"
	flprometheus "gitlab.com/gitlab-org/fleeting/fleeting/metrics/prometheus"
	fleetingprovider "gitlab.com/gitlab-org/fleeting/fleeting/provider"
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
}

func New(ep common.ExecutorProvider) common.ExecutorProvider {
	return &provider{
		ExecutorProvider: ep,
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

	options := []taskscaler.Option{
		taskscaler.WithCapacityPerInstance(config.Autoscaler.CapacityPerInstance),
		taskscaler.WithMaxUseCount(config.Autoscaler.MaxUseCount),
		taskscaler.WithMaxInstances(config.Autoscaler.MaxInstances),
		taskscaler.WithInstanceGroupSettings(fleetingprovider.Settings{
			ConnectorConfig: instanceConnectConfig,
		}),
		taskscaler.WithMetricsCollector(tsMC),
		taskscaler.WithFleetingMetricsCollector(flMC),
	}

	ctx, cancelFn := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancelFn()

	ts, err := p.taskscalerNew(ctx, runner.InstanceGroup(), options...)
	if err != nil {
		runner.Kill()

		return nil, false, fmt.Errorf("creating taskscaler: %w", err)
	}

	s = scaler{
		internal: ts,
		shutdown: func(ctx context.Context) {
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
			})
		}
		if err := scaler.ConfigureSchedule(schedules...); err != nil {
			return nil, fmt.Errorf("configuring taskscaler schedules: %w", err)
		}
	}

	available, potential := scaler.Capacity()

	if potential <= 0 && available <= 0 {
		return nil, fmt.Errorf("already at capacity, cannot accept")
	}

	if scaler.Schedule().IdleCount > 0 && available <= 0 {
		return nil, fmt.Errorf("already at capacity, cannot accept, allow on demand is disabled")
	}

	// generate key for acquisition
	key, err := p.generateUniqueID()
	if err != nil {
		return nil, fmt.Errorf("generating unique id for task acquisition: %w", err)
	}
	key = helpers.ShortenToken(config.Token) + key

	// todo: allow configuration of how long we're willing to wait for. Do we have something like this already?
	// todo: it would be good if Acquire() was provided a context, so we could stop when shutting down.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	acq, err := scaler.Acquire(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("unable to acquire instance: %w", err)
	}

	return &acquisitionRef{key: key, acq: acq}, nil
}

func (p *provider) Release(config *common.RunnerConfig, data common.ExecutorData) {
	acq, ok := data.(*acquisitionRef)
	if !ok {
		return
	}

	p.getRunnerTaskscaler(config).Release(acq.key)
}

func (p *provider) Create() common.Executor {
	return p.ExecutorProvider.Create()
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
