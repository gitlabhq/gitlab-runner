package autoscaler

import (
	"context"
	"fmt"
	"os"
	"sync"

	"gitlab.com/gitlab-org/fleeting/fleeting"
	"gitlab.com/gitlab-org/gitlab-runner/common"

	fleetingprovider "gitlab.com/gitlab-org/fleeting/fleeting/provider"
	"gitlab.com/gitlab-org/fleeting/taskscaler"
)

type provider struct {
	common.ExecutorProvider

	mu      sync.Mutex
	scalers map[string]*taskscaler.Taskscaler
}

func New(ep common.ExecutorProvider) common.ExecutorProvider {
	return &provider{
		ExecutorProvider: ep,
		scalers:          make(map[string]*taskscaler.Taskscaler),
	}
}

//nolint:funlen
func (p *provider) init(ctx context.Context, config *common.RunnerConfig) (*taskscaler.Taskscaler, bool, error) {
	if config.Autoscaler == nil {
		return nil, false, fmt.Errorf("executor requires autoscaler config")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	scaler, ok := p.scalers[config.GetToken()]
	if ok {
		return scaler, false, nil
	}

	pluginCfg, err := config.Autoscaler.PluginConfig.JSON()
	if err != nil {
		return nil, false, fmt.Errorf("marshaling plugin config: %w", err)
	}

	runner, err := fleeting.RunPlugin(config.Autoscaler.Plugin, pluginCfg)
	if err != nil {
		return nil, false, fmt.Errorf("running autoscaler plugin: %w", err)
	}
	// todo:
	// The plugin can be killed/unloaded with runner.Kill().
	// There's no mechanism in place to do this at the moment, as executor
	// providers have no shutdown routine.

	instanceConnectConfig := fleetingprovider.ConnectorConfig{
		OS:                   config.Autoscaler.InstanceGroupSettings.OS,
		Arch:                 config.Autoscaler.InstanceGroupSettings.Arch,
		Protocol:             fleetingprovider.Protocol(config.Autoscaler.InstanceGroupSettings.Protocol),
		Username:             config.Autoscaler.InstanceGroupSettings.Username,
		Password:             config.Autoscaler.InstanceGroupSettings.Password,
		UseStaticCredentials: config.Autoscaler.InstanceGroupSettings.UseStaticCredentials,
		Keepalive:            config.Autoscaler.InstanceGroupSettings.Keepalive,
		Timeout:              config.Autoscaler.InstanceGroupSettings.Timeout,
	}

	if config.Autoscaler.InstanceGroupSettings.KeyPathname != "" {
		key, err := os.ReadFile(config.Autoscaler.InstanceGroupSettings.KeyPathname)
		if err != nil {
			return nil, false, fmt.Errorf("reading instance group connector key: %w", err)
		}
		instanceConnectConfig.Key = key
	}

	options := []taskscaler.Option{
		taskscaler.WithCapacityPerInstance(config.Autoscaler.CapacityPerInstance),
		taskscaler.WithMaxUseCount(config.Autoscaler.MaxUseCount),
		taskscaler.WithMaxInstances(config.Autoscaler.MaxInstances),
		taskscaler.WithInstanceGroupSettings(fleetingprovider.Settings{
			ConnectorConfig: instanceConnectConfig,
		}),
	}

	scaler, err = taskscaler.New(ctx, runner.InstanceGroup(), options...)
	if err != nil {
		return nil, false, fmt.Errorf("creating taskscaler: %w", err)
	}

	p.scalers[config.GetToken()] = scaler

	return scaler, true, nil
}

func (p *provider) Acquire(config *common.RunnerConfig) (common.ExecutorData, error) {
	scaler, fresh, err := p.init(context.Background(), config)
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

	if potential <= 0 || available <= 0 {
		return nil, fmt.Errorf("already at capacity, cannot accept")
	}

	if scaler.Schedule().IdleCount > 0 && available <= 0 {
		return nil, fmt.Errorf("already at capacity, cannot accept, allow on demand is disabled")
	}

	return &acqusitionRef{}, nil
}

func (p *provider) Release(config *common.RunnerConfig, data common.ExecutorData) {
	acq, ok := data.(*acqusitionRef)
	if !ok {
		return
	}

	p.getRunnerTaskscaler(config).Release(acq.get())
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

func (p *provider) getRunnerTaskscaler(config *common.RunnerConfig) *taskscaler.Taskscaler {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.scalers[config.GetToken()]
}
