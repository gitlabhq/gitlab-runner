//go:build !integration

package autoscaler

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	fleetingprovider "gitlab.com/gitlab-org/fleeting/fleeting/provider"
	"gitlab.com/gitlab-org/fleeting/taskscaler"
	"gitlab.com/gitlab-org/fleeting/taskscaler/mocks"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func TestInit(t *testing.T) {
	tokenTaskscaler := mocks.NewTaskscaler(t) // for comparing by memory address

	tests := map[string]struct {
		scalers              map[string]taskscaler.Taskscaler
		config               *common.RunnerConfig
		newTaskscalerErr     bool
		fleetingRunPluginErr bool
		wantTaskscaler       taskscaler.Taskscaler
		wantCreated          bool
		wantErr              bool
	}{
		"nil autoscaler config return error": {
			config:  common.NewTestRunnerConfig().RunnerConfig,
			wantErr: true,
		},
		"taskscaler exists and is not created": {
			config: common.NewTestRunnerConfig().
				WithAutoscalerConfig(
					common.NewTestAutoscalerConfig().AutoscalerConfig,
				).
				RunnerConfig,
			scalers: map[string]taskscaler.Taskscaler{
				"": tokenTaskscaler,
			},
			wantTaskscaler: tokenTaskscaler,
			wantCreated:    false,
		},
		"taskscaler does not exist and is created": {
			config: common.NewTestRunnerConfig().
				WithAutoscalerConfig(
					common.NewTestAutoscalerConfig().AutoscalerConfig,
				).
				RunnerConfig,
			wantTaskscaler: tokenTaskscaler,
			wantCreated:    true,
		},
		"error creating fleeting plugin": {
			config: common.NewTestRunnerConfig().
				WithAutoscalerConfig(
					common.NewTestAutoscalerConfig().AutoscalerConfig,
				).
				RunnerConfig,
			fleetingRunPluginErr: true,
			wantErr:              true,
		},
		"error creating new taskscaler": {
			config: common.NewTestRunnerConfig().
				WithAutoscalerConfig(
					common.NewTestAutoscalerConfig().AutoscalerConfig,
				).
				RunnerConfig,
			newTaskscalerErr: true,
			wantErr:          true,
		},
		"returns correct taskscaler": {
			config: common.NewTestRunnerConfig().
				WithAutoscalerConfig(
					common.NewTestAutoscalerConfig().AutoscalerConfig,
				).
				WithToken("right").
				RunnerConfig,
			scalers: map[string]taskscaler.Taskscaler{
				"wrong": nil,
				"":      nil, // also wrong
				"right": tokenTaskscaler,
			},
			wantTaskscaler: tokenTaskscaler,
			wantCreated:    false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ep := &common.MockExecutorProvider{}
			p := New(ep, Config{}).(*provider)
			p.taskscalerNew = mockTaskscalerNew(tokenTaskscaler, tt.newTaskscalerErr)
			p.fleetingRunPlugin = mockFleetingRunPlugin(tt.fleetingRunPluginErr)
			for k, v := range tt.scalers {
				p.scalers[k] = scaler{
					internal: v,
					shutdown: func(_ context.Context) {},
				}
			}

			ts, created, err := p.init(tt.config)

			assert.Equal(t, tt.wantTaskscaler, ts)
			assert.Equal(t, tt.wantCreated, created)
			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func TestAcquire(t *testing.T) {
	const (
		afterInit      = "Init"
		afterConfigure = "Configure"
		afterReserve   = "Reserve"
	)

	tests := map[string]struct {
		idleCount          int
		canReserve         bool
		wantEarlyReturn    string
		wantAcquisitionRef bool
		wantErr            bool
	}{
		"failed init": {
			wantEarlyReturn: afterInit,
			wantErr:         true,
		},
		"failed configure schedule": {
			wantEarlyReturn: afterConfigure,
			wantErr:         true,
		},
		"has capacity, no idle count": {
			idleCount:          0,
			canReserve:         true,
			wantAcquisitionRef: true,
		},
		"no capacity, no idle count": {
			idleCount:       0,
			canReserve:      false,
			wantEarlyReturn: afterReserve,
			wantErr:         true,
		},
		"has capacity, has idle count": {
			idleCount:          1,
			canReserve:         true,
			wantAcquisitionRef: true,
		},
		"no capacity, has idle count": {
			idleCount:       1,
			canReserve:      false,
			wantEarlyReturn: afterReserve,
			wantErr:         true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			const acqRefKey = "abcdefgh"

			config := common.NewTestRunnerConfig().
				WithAutoscalerConfig(
					common.NewTestAutoscalerConfig().
						WithPolicies(
							common.AutoscalerPolicyConfig{
								IdleCount: tt.idleCount,
							},
						).AutoscalerConfig,
				).RunnerConfig
			schedule := taskscaler.Schedule{
				IdleCount:      tt.idleCount,
				PreemptiveMode: tt.idleCount > 0,
			}
			ts := mocks.NewTaskscaler(t)
			ep := &common.MockExecutorProvider{}
			p := New(ep, Config{}).(*provider)
			p.taskscalerNew = mockTaskscalerNew(ts /* wantErr */, false)
			p.fleetingRunPlugin = mockFleetingRunPlugin( /* wantErr */ false)
			p.generateUniqueID = func() (string, error) {
				return acqRefKey, nil
			}

			switch tt.wantEarlyReturn {
			case afterInit:
				// Init fails to create new taskscaler.
				p.taskscalerNew = mockTaskscalerNew(nil /* wantErr */, true)
			case afterConfigure:
				ts.EXPECT().ConfigureSchedule(schedule).Return(fmt.Errorf("test error"))
			case afterReserve:
				ts.EXPECT().ConfigureSchedule(schedule).Return(nil)
				ts.EXPECT().Reserve(acqRefKey).Return(taskscaler.ErrNoCapacity)
			default:
				ts.EXPECT().ConfigureSchedule(schedule).Return(nil)
				ts.EXPECT().Reserve(acqRefKey).Return(nil)
			}

			ar, err := p.Acquire(config)

			if tt.wantAcquisitionRef {
				if assert.IsType(t, &acquisitionRef{}, ar) {
					assert.Equal(t, acqRefKey, ar.(*acquisitionRef).key)
				}
			} else {
				assert.Nil(t, ar)
			}
			if tt.wantErr {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func mockTaskscalerNew(
	newTaskscaler taskscaler.Taskscaler,
	newTaskscalerErr bool,
) func(context.Context, fleetingprovider.InstanceGroup, ...taskscaler.Option) (taskscaler.Taskscaler, error) {
	return func(context.Context, fleetingprovider.InstanceGroup, ...taskscaler.Option) (taskscaler.Taskscaler, error) {
		if newTaskscalerErr {
			return nil, fmt.Errorf("test error")
		}
		return newTaskscaler, nil
	}
}

func mockFleetingRunPlugin(wantErr bool) func(string, []byte) (fleetingPlugin, error) {
	return func(string, []byte) (fleetingPlugin, error) {
		if wantErr {
			return nil, fmt.Errorf("test error")
		}

		return new(fakeFleetingPlugin), nil
	}
}

type fakeFleetingPlugin struct{}

func (f *fakeFleetingPlugin) InstanceGroup() fleetingprovider.InstanceGroup { return nil }

func (f *fakeFleetingPlugin) Kill() {}
