package common

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheS3Config_ShouldUseIAMCredentials(t *testing.T) {
	tests := map[string]struct {
		s3                     CacheS3Config
		shouldUseIAMCredential bool
	}{
		"Everything is empty": {
			s3: CacheS3Config{
				ServerAddress:  "",
				AccessKey:      "",
				SecretKey:      "",
				BucketName:     "name",
				BucketLocation: "us-east-1a",
			},
			shouldUseIAMCredential: true,
		},
		"Both AccessKey & SecretKey are empty": {
			s3: CacheS3Config{
				ServerAddress:  "s3.amazonaws.com",
				AccessKey:      "",
				SecretKey:      "",
				BucketName:     "name",
				BucketLocation: "us-east-1a",
			},
			shouldUseIAMCredential: true,
		},
		"SecretKey is empty": {
			s3: CacheS3Config{
				ServerAddress:  "s3.amazonaws.com",
				AccessKey:      "TOKEN",
				SecretKey:      "",
				BucketName:     "name",
				BucketLocation: "us-east-1a",
			},
			shouldUseIAMCredential: true,
		},
		"AccessKey is empty": {
			s3: CacheS3Config{
				ServerAddress:  "s3.amazonaws.com",
				AccessKey:      "",
				SecretKey:      "TOKEN",
				BucketName:     "name",
				BucketLocation: "us-east-1a",
			},
			shouldUseIAMCredential: true,
		},
		"ServerAddress is empty": {
			s3: CacheS3Config{
				ServerAddress:  "",
				AccessKey:      "TOKEN",
				SecretKey:      "TOKEN",
				BucketName:     "name",
				BucketLocation: "us-east-1a",
			},
			shouldUseIAMCredential: true,
		},
		"ServerAddress & AccessKey are empty": {
			s3: CacheS3Config{
				ServerAddress:  "",
				AccessKey:      "",
				SecretKey:      "TOKEN",
				BucketName:     "name",
				BucketLocation: "us-east-1a",
			},
			shouldUseIAMCredential: true,
		},
		"ServerAddress & SecretKey are empty": {
			s3: CacheS3Config{
				ServerAddress:  "",
				AccessKey:      "TOKEN",
				SecretKey:      "",
				BucketName:     "name",
				BucketLocation: "us-east-1a",
			},
			shouldUseIAMCredential: true,
		},
		"Nothing is empty": {
			s3: CacheS3Config{
				ServerAddress:  "s3.amazonaws.com",
				AccessKey:      "TOKEN",
				SecretKey:      "TOKEN",
				BucketName:     "name",
				BucketLocation: "us-east-1a",
			},
			shouldUseIAMCredential: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.shouldUseIAMCredential, tt.s3.ShouldUseIAMCredentials())
		})
	}
}

func TestConfigParse(t *testing.T) {
	tests := map[string]struct {
		config         string
		validateConfig func(t *testing.T, config *Config)
		expectedErr    string
	}{
		"parse Service as table with only name": {
			config: `
				[[runners]]
				[[runners.docker.services]]
				name = "svc1"
				[[runners.docker.services]]
				name = "svc2"
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Equal(t, 1, len(config.Runners))
				require.Equal(t, 2, len(config.Runners[0].Docker.Services))
				assert.Equal(t, "svc1", config.Runners[0].Docker.Services[0].Name)
				assert.Equal(t, "", config.Runners[0].Docker.Services[0].Alias)
				assert.Equal(t, "svc2", config.Runners[0].Docker.Services[1].Name)
				assert.Equal(t, "", config.Runners[0].Docker.Services[1].Alias)
			},
		},
		"parse Service as table with only alias": {
			config: `
				[[runners]]
				[[runners.docker.services]]
				alias = "svc1"
				[[runners.docker.services]]
				alias = "svc2"
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Equal(t, 1, len(config.Runners))
				require.Equal(t, 2, len(config.Runners[0].Docker.Services))
				assert.Equal(t, "", config.Runners[0].Docker.Services[0].Name)
				assert.Equal(t, "svc1", config.Runners[0].Docker.Services[0].Alias)
				assert.Equal(t, "", config.Runners[0].Docker.Services[1].Name)
				assert.Equal(t, "svc2", config.Runners[0].Docker.Services[1].Alias)
			},
		},
		"parse Service as table": {
			config: `
				[[runners]]
				[[runners.docker.services]]
				name = "svc1"
				alias = "svc1_alias"
				[[runners.docker.services]]
				name = "svc2"
				alias = "svc2_alias"
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Equal(t, 1, len(config.Runners))
				require.Equal(t, 2, len(config.Runners[0].Docker.Services))
				assert.Equal(t, "svc1", config.Runners[0].Docker.Services[0].Name)
				assert.Equal(t, "svc1_alias", config.Runners[0].Docker.Services[0].Alias)
				assert.Equal(t, "svc2", config.Runners[0].Docker.Services[1].Name)
				assert.Equal(t, "svc2_alias", config.Runners[0].Docker.Services[1].Alias)
			},
		},
		"parse Service as table int value name": {
			config: `
				[[runners]]
				[[runners.docker.services]]
				name = 5
			`,
			expectedErr: "toml: cannot load TOML value of type int64 into a Go string",
		},
		"parse Service as table int value alias": {
			config: `
				[[runners]]
				[[runners.docker.services]]
				name = "svc1"
				alias = 5
			`,
			expectedErr: "toml: cannot load TOML value of type int64 into a Go string",
		},
		"parse Service runners.docker and runners.docker.services": {
			config: `
				[[runners]]
				[runners.docker]
				image = "image"
				[[runners.docker.services]]
				name = "svc1"
				[[runners.docker.services]]
				name = "svc2"
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Equal(t, 1, len(config.Runners))
				require.Equal(t, 2, len(config.Runners[0].Docker.Services))
				assert.Equal(t, "image", config.Runners[0].Docker.Image)
			},
		},
		//nolint:lll
		"check node affinities": {
			config: `
				[[runners]]
					[runners.kubernetes]
						[runners.kubernetes.affinity]
							[runners.kubernetes.affinity.node_affinity]
								[[runners.kubernetes.affinity.node_affinity.preferred_during_scheduling_ignored_during_execution]]
									weight = 100
									[runners.kubernetes.affinity.node_affinity.preferred_during_scheduling_ignored_during_execution.preference]
										[[runners.kubernetes.affinity.node_affinity.preferred_during_scheduling_ignored_during_execution.preference.match_expressions]]
											key = "cpu_speed"
											operator = "In"
											values = ["fast"]
								[[runners.kubernetes.affinity.node_affinity.preferred_during_scheduling_ignored_during_execution]]
									weight = 50
									[runners.kubernetes.affinity.node_affinity.preferred_during_scheduling_ignored_during_execution.preference]
										[[runners.kubernetes.affinity.node_affinity.preferred_during_scheduling_ignored_during_execution.preference.match_expressions]]
											key = "core_count"
											operator = "In"
											values = ["heigh", "32"]
										[[runners.kubernetes.affinity.node_affinity.preferred_during_scheduling_ignored_during_execution.preference.match_expressions]]
											key = "cpu_type"
											operator = "In"
											values = ["x86, arm", "i386"]
								[[runners.kubernetes.affinity.node_affinity.preferred_during_scheduling_ignored_during_execution]]
									weight = 20
									[runners.kubernetes.affinity.node_affinity.preferred_during_scheduling_ignored_during_execution.preference]
										[[runners.kubernetes.affinity.node_affinity.preferred_during_scheduling_ignored_during_execution.preference.match_fields]]
											key = "zone"
											operator = "In"
											values = ["us-east"]
								[runners.kubernetes.affinity.node_affinity.required_during_scheduling_ignored_during_execution]
									[[runners.kubernetes.affinity.node_affinity.required_during_scheduling_ignored_during_execution.node_selector_terms]]
										[[runners.kubernetes.affinity.node_affinity.required_during_scheduling_ignored_during_execution.node_selector_terms.match_expressions]]
											key = "kubernetes.io/e2e-az-name"
											operator = "In"
											values = [
												"e2e-az1",
												"e2e-az2"
											]
										[[runners.kubernetes.affinity.node_affinity.required_during_scheduling_ignored_during_execution.node_selector_terms]]
											[[runners.kubernetes.affinity.node_affinity.required_during_scheduling_ignored_during_execution.node_selector_terms.match_fields]]
												 key = "kubernetes.io/e2e-az-name/field"
												 operator = "In"
												 values = [
												   "e2e-az1"
												 ]

			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Len(t, config.Runners, 1)
				require.NotNil(t, config.Runners[0].Kubernetes.Affinity)
				require.NotNil(t, config.Runners[0].Kubernetes.Affinity.NodeAffinity)

				nodeAffinity := config.Runners[0].Kubernetes.Affinity.NodeAffinity

				require.Len(t, nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution, 3)
				assert.Equal(t, int32(100), nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Weight)
				require.NotNil(t, nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference)
				require.Len(t, nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions, 1)
				assert.Equal(t, "In", nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Operator)
				assert.Equal(t, "cpu_speed", nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Key)
				assert.Equal(t, "fast", nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0].Preference.MatchExpressions[0].Values[0])

				assert.Equal(t, int32(50), nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].Weight)
				require.NotNil(t, nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].Preference)
				require.Len(t, nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].Preference.MatchExpressions, 2)
				assert.Equal(t, "In", nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].Preference.MatchExpressions[0].Operator)
				assert.Equal(t, "core_count", nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].Preference.MatchExpressions[0].Key)
				assert.Equal(t, []string{"heigh", "32"}, nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].Preference.MatchExpressions[0].Values)
				assert.Equal(t, "In", nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].Preference.MatchExpressions[1].Operator)
				assert.Equal(t, "cpu_type", nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].Preference.MatchExpressions[1].Key)
				assert.Equal(t, []string{"x86, arm", "i386"}, nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].Preference.MatchExpressions[1].Values)

				assert.Equal(t, int32(20), nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[2].Weight)
				require.NotNil(t, nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[2].Preference)
				require.Len(t, nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[2].Preference.MatchFields, 1)
				assert.Equal(t, "zone", nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[2].Preference.MatchFields[0].Key)
				assert.Equal(t, "In", nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[2].Preference.MatchFields[0].Operator)
				assert.Equal(t, []string{"us-east"}, nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[2].Preference.MatchFields[0].Values)

				require.NotNil(t, nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
				require.Len(t, nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, 2)
				require.Len(t, nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions, 1)
				require.Len(t, nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchFields, 0)
				assert.Equal(t, "kubernetes.io/e2e-az-name", nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Key)
				assert.Equal(t, "In", nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Operator)
				assert.Equal(t, []string{"e2e-az1", "e2e-az2"}, nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values)

				assert.Equal(t, []string{"e2e-az1", "e2e-az2"}, nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values)
				assert.Equal(t, "kubernetes.io/e2e-az-name/field", nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[1].MatchFields[0].Key)
				assert.Equal(t, "In", nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[1].MatchFields[0].Operator)
				assert.Equal(t, []string{"e2e-az1"}, nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[1].MatchFields[0].Values)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			cfg := NewConfig()
			_, err := toml.Decode(tt.config, cfg)
			if tt.expectedErr != "" {
				assert.EqualError(t, err, tt.expectedErr)
				return
			}

			assert.NoError(t, err)
			if tt.validateConfig != nil {
				tt.validateConfig(t, cfg)
			}
		})
	}
}

func TestService_ToImageDefinition(t *testing.T) {
	tests := map[string]struct {
		service       Service
		expectedImage Image
	}{
		"empty service": {
			service:       Service{},
			expectedImage: Image{},
		},
		"only name": {
			service:       Service{Name: "name"},
			expectedImage: Image{Name: "name"},
		},
		"only alias": {
			service:       Service{Alias: "alias"},
			expectedImage: Image{Alias: "alias"},
		},
		"name and alias": {
			service:       Service{Name: "name", Alias: "alias"},
			expectedImage: Image{Name: "name", Alias: "alias"},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.Equal(t, tt.expectedImage, tt.service.ToImageDefinition())
		})
	}
}

func TestDockerMachine(t *testing.T) {
	timeNow := func() time.Time {
		return time.Date(2020, 05, 05, 20, 00, 00, 0, time.Local)
	}
	activeTimePeriod := []string{fmt.Sprintf("* * %d * * * *", timeNow().Hour())}
	inactiveTimePeriod := []string{fmt.Sprintf("* * %d * * * *", timeNow().Add(2*time.Hour).Hour())}
	invalidTimePeriod := []string{"invalid period"}

	oldPeriodTimer := periodTimer
	defer func() {
		periodTimer = oldPeriodTimer
	}()
	periodTimer = timeNow

	tests := map[string]struct {
		config            *DockerMachine
		expectedIdleCount int
		expectedIdleTime  int
		expectedErr       error
	}{
		"global config only": {
			config:            &DockerMachine{IdleCount: 1, IdleTime: 1000},
			expectedIdleCount: 1,
			expectedIdleTime:  1000,
		},
		"offpeak active": {
			config: &DockerMachine{
				IdleCount:        1,
				IdleTime:         1000,
				OffPeakPeriods:   activeTimePeriod,
				OffPeakIdleCount: 2,
				OffPeakIdleTime:  2000,
			},
			expectedIdleCount: 2,
			expectedIdleTime:  2000,
		},
		"offpeak inactive": {
			config: &DockerMachine{
				IdleCount:        1,
				IdleTime:         1000,
				OffPeakPeriods:   inactiveTimePeriod,
				OffPeakIdleCount: 2,
				OffPeakIdleTime:  2000,
			},
			expectedIdleCount: 1,
			expectedIdleTime:  1000,
		},
		"offpeak invalid format": {
			config: &DockerMachine{
				IdleCount:        1,
				IdleTime:         1000,
				OffPeakPeriods:   invalidTimePeriod,
				OffPeakIdleCount: 2,
				OffPeakIdleTime:  2000,
			},
			expectedIdleCount: 0,
			expectedIdleTime:  0,
			expectedErr:       new(InvalidTimePeriodsError),
		},
		"autoscaling config active": {
			config: &DockerMachine{
				IdleCount: 1,
				IdleTime:  1000,
				AutoscalingConfigs: []*DockerMachineAutoscaling{
					{
						Periods:   activeTimePeriod,
						IdleCount: 2,
						IdleTime:  2000,
					},
				},
			},
			expectedIdleCount: 2,
			expectedIdleTime:  2000,
		},
		"autoscaling config inactive": {
			config: &DockerMachine{
				IdleCount: 1,
				IdleTime:  1000,
				AutoscalingConfigs: []*DockerMachineAutoscaling{
					{
						Periods:   inactiveTimePeriod,
						IdleCount: 2,
						IdleTime:  2000,
					},
				},
			},
			expectedIdleCount: 1,
			expectedIdleTime:  1000,
		},
		"last matching autoscaling config is selected": {
			config: &DockerMachine{
				IdleCount: 1,
				IdleTime:  1000,
				AutoscalingConfigs: []*DockerMachineAutoscaling{
					{
						Periods:   activeTimePeriod,
						IdleCount: 2,
						IdleTime:  2000,
					},
					{
						Periods:   activeTimePeriod,
						IdleCount: 3,
						IdleTime:  3000,
					},
				},
			},
			expectedIdleCount: 3,
			expectedIdleTime:  3000,
		},
		"autoscaling overrides offpeak config": {
			config: &DockerMachine{
				IdleCount:        1,
				IdleTime:         1000,
				OffPeakPeriods:   activeTimePeriod,
				OffPeakIdleCount: 2,
				OffPeakIdleTime:  2000,
				AutoscalingConfigs: []*DockerMachineAutoscaling{
					{
						Periods:   activeTimePeriod,
						IdleCount: 3,
						IdleTime:  3000,
					},
					{
						Periods:   activeTimePeriod,
						IdleCount: 4,
						IdleTime:  4000,
					},
					{
						Periods:   inactiveTimePeriod,
						IdleCount: 5,
						IdleTime:  5000,
					},
				},
			},
			expectedIdleCount: 4,
			expectedIdleTime:  4000,
		},
		"autoscaling invalid period config": {
			config: &DockerMachine{
				IdleCount: 1,
				IdleTime:  1000,
				AutoscalingConfigs: []*DockerMachineAutoscaling{
					{
						Periods:   []string{"invalid period"},
						IdleCount: 3,
						IdleTime:  3000,
					},
				},
			},
			expectedIdleCount: 0,
			expectedIdleTime:  0,
			expectedErr:       new(InvalidTimePeriodsError),
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			err := tt.config.CompilePeriods()
			if tt.expectedErr != nil {
				assert.True(t, errors.Is(err, tt.expectedErr))
				return
			}
			assert.NoError(t, err, "should not return err on good period compile")
			assert.Equal(t, tt.expectedIdleCount, tt.config.GetIdleCount())
			assert.Equal(t, tt.expectedIdleTime, tt.config.GetIdleTime())
		})
	}
}
