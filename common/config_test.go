//go:build !integration

package common

import (
	"flag"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	api "k8s.io/api/core/v1"

	clihelpers "gitlab.com/gitlab-org/golang-cli-helpers"

	"gitlab.com/gitlab-org/gitlab-runner/cache/cacheconfig"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/process"
)

func TestConfigParse(t *testing.T) {
	httpHeaders := []KubernetesLifecycleHTTPGetHeader{
		{Name: "header_name_1", Value: "header_value_1"},
		{Name: "header_name_2", Value: "header_value_2"},
	}

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
			expectedErr: "incompatible types: TOML value has type int64; destination has type string",
		},
		"parse Service as table int value alias": {
			config: `
				[[runners]]
				[[runners.docker.services]]
				name = "svc1"
				alias = 5
			`,
			expectedErr: "incompatible types: TOML value has type int64; destination has type string",
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
		"parse Service runners.docker.services environment": {
			config: `
				[[runners]]
				[runners.docker]
				[[runners.docker.services]]
				name = "svc1"
				environment = ["ENV1=value1", "ENV2=value2"]
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Equal(t, 1, len(config.Runners))
				require.Equal(t, 1, len(config.Runners[0].Docker.Services))
				require.Equal(t, 2, len(config.Runners[0].Docker.Services[0].Environment))
				assert.Equal(t, "ENV1=value1", config.Runners[0].Docker.Services[0].Environment[0])
				assert.Equal(t, "ENV2=value2", config.Runners[0].Docker.Services[0].Environment[1])
			},
		},
		"parse Docker Container Labels with string key and value": {
			config: `
                        [[runners]]
                                [runners.docker]
                                        image = "image"
                                        [runners.docker.container_labels]
                                                "my.docker.TestContainerlabel1" = "TestContainerlabel-1"
                `,
			validateConfig: func(t *testing.T, config *Config) {
				require.Equal(t, 1, len(config.Runners))

				runner := config.Runners[0]
				require.NotNil(t, runner.RunnerSettings.Docker.ContainerLabels)
				require.NotNil(t, runner.RunnerSettings.Docker.ContainerLabels["my.docker.TestContainerlabel1"])
				require.Equal(
					t,
					"TestContainerlabel-1",
					runner.RunnerSettings.Docker.ContainerLabels["my.docker.TestContainerlabel1"],
				)
			},
		},
		"parse Docker Container Labels with integer key and value": {
			config: `
                        [[runners]]
                                [runners.docker]
                                        image = "image"
                                        [runners.docker.container_labels]
                                                5 = 5
                `,
			expectedErr: "incompatible types: TOML value has type int64; destination has type string",
		},
		"parse Docker Container Labels with integer value": {
			config: `
                        [[runners]]
                                [runners.docker]
                                        image = "image"
                                        [runners.docker.container_labels]
                                                "my.docker.TestContainerlabel1" = 5
                `,
			expectedErr: "incompatible types: TOML value has type int64; destination has type string",
		},
		"parse Docker Container Labels with integer key": {
			config: `
                        [[runners]]
                                [runners.docker]
                                        image = "image"
                                        [runners.docker.container_labels]
                                                5 = "TestContainerlabel-1"
                `,
		},
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
											values = ["high", "32"]
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
				assert.Equal(t, []string{"high", "32"}, nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution[1].Preference.MatchExpressions[0].Values)
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

				assert.Equal(t, "kubernetes.io/e2e-az-name/field", nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[1].MatchFields[0].Key)
				assert.Equal(t, "In", nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[1].MatchFields[0].Operator)
				assert.Equal(t, []string{"e2e-az1"}, nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[1].MatchFields[0].Values)
			},
		},
		"check pod affinities": {
			config: `
				[[runners]]
					[runners.kubernetes]
						[runners.kubernetes.affinity]
							[runners.kubernetes.affinity.pod_affinity]
								[[runners.kubernetes.affinity.pod_affinity.required_during_scheduling_ignored_during_execution]]
									topology_key = "failure-domain.beta.kubernetes.io/zone"
									namespaces = ["namespace_1", "namespace_2"]
									[runners.kubernetes.affinity.pod_affinity.required_during_scheduling_ignored_during_execution.label_selector]
										[[runners.kubernetes.affinity.pod_affinity.required_during_scheduling_ignored_during_execution.label_selector.match_expressions]]
											key = "security"
											operator = "In"
											values = ["S1"]
									[runners.kubernetes.affinity.pod_affinity.required_during_scheduling_ignored_during_execution.namespace_selector]
										[[runners.kubernetes.affinity.pod_affinity.required_during_scheduling_ignored_during_execution.namespace_selector.match_expressions]]
											key = "security"
											operator = "In"
											values = ["S1"]

								[[runners.kubernetes.affinity.pod_affinity.preferred_during_scheduling_ignored_during_execution]]
								weight = 100
								[runners.kubernetes.affinity.pod_affinity.preferred_during_scheduling_ignored_during_execution.pod_affinity_term]
									topology_key = "failure-domain.beta.kubernetes.io/zone"
									[runners.kubernetes.affinity.pod_affinity.preferred_during_scheduling_ignored_during_execution.pod_affinity_term.label_selector]
										[[runners.kubernetes.affinity.pod_affinity.preferred_during_scheduling_ignored_during_execution.pod_affinity_term.label_selector.match_expressions]]
											key = "security_2"
											operator = "In"
											values = ["S2"]
									[runners.kubernetes.affinity.pod_affinity.preferred_during_scheduling_ignored_during_execution.pod_affinity_term.namespace_selector]
										[[runners.kubernetes.affinity.pod_affinity.preferred_during_scheduling_ignored_during_execution.pod_affinity_term.namespace_selector.match_expressions]]
											key = "security_2"
											operator = "In"
											values = ["S2"]
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Len(t, config.Runners, 1)
				require.NotNil(t, config.Runners[0].Kubernetes.Affinity)
				require.NotNil(t, config.Runners[0].Kubernetes.Affinity.PodAffinity)

				podAffinity := config.Runners[0].Kubernetes.Affinity.PodAffinity
				require.Len(t, podAffinity.RequiredDuringSchedulingIgnoredDuringExecution, 1)
				required := podAffinity.RequiredDuringSchedulingIgnoredDuringExecution

				assert.Equal(t, "failure-domain.beta.kubernetes.io/zone", required[0].TopologyKey)
				assert.Equal(t, []string{"namespace_1", "namespace_2"}, required[0].Namespaces)

				require.NotNil(t, required[0].LabelSelector)
				require.Len(t, required[0].LabelSelector.MatchExpressions, 1)
				requiredMatchExp := required[0].LabelSelector.MatchExpressions[0]
				assert.Equal(t, "security", requiredMatchExp.Key)
				assert.Equal(t, "In", requiredMatchExp.Operator)
				assert.Equal(t, []string{"S1"}, requiredMatchExp.Values)

				require.NotNil(t, required[0].NamespaceSelector)
				require.Len(t, required[0].NamespaceSelector.MatchExpressions, 1)
				requiredMatchExp = required[0].NamespaceSelector.MatchExpressions[0]
				assert.Equal(t, "security", requiredMatchExp.Key)
				assert.Equal(t, "In", requiredMatchExp.Operator)
				assert.Equal(t, []string{"S1"}, requiredMatchExp.Values)

				require.Len(t, podAffinity.PreferredDuringSchedulingIgnoredDuringExecution, 1)
				preferred := podAffinity.PreferredDuringSchedulingIgnoredDuringExecution
				assert.Equal(t, int32(100), preferred[0].Weight)
				assert.Empty(t, preferred[0].PodAffinityTerm.Namespaces)
				assert.Equal(t, "failure-domain.beta.kubernetes.io/zone", preferred[0].PodAffinityTerm.TopologyKey)

				require.NotNil(t, preferred[0].PodAffinityTerm.LabelSelector)
				require.Len(t, preferred[0].PodAffinityTerm.LabelSelector.MatchExpressions, 1)
				preferredMatchExp := preferred[0].PodAffinityTerm.LabelSelector.MatchExpressions[0]
				assert.Equal(t, "security_2", preferredMatchExp.Key)
				assert.Equal(t, "In", preferredMatchExp.Operator)
				assert.Equal(t, []string{"S2"}, preferredMatchExp.Values)

				require.NotNil(t, preferred[0].PodAffinityTerm.NamespaceSelector)
				require.Len(t, preferred[0].PodAffinityTerm.NamespaceSelector.MatchExpressions, 1)
				preferredMatchExp = preferred[0].PodAffinityTerm.NamespaceSelector.MatchExpressions[0]
				assert.Equal(t, "security_2", preferredMatchExp.Key)
				assert.Equal(t, "In", preferredMatchExp.Operator)
				assert.Equal(t, []string{"S2"}, preferredMatchExp.Values)
			},
		},
		"check pod anti affinities": {
			config: `
				[[runners]]
					[runners.kubernetes]
						[runners.kubernetes.affinity]
							[runners.kubernetes.affinity.pod_anti_affinity]
								[[runners.kubernetes.affinity.pod_anti_affinity.required_during_scheduling_ignored_during_execution]]
									topology_key = "failure-domain.beta.kubernetes.io/zone"
									namespaces = ["namespace_1", "namespace_2"]
									[runners.kubernetes.affinity.pod_anti_affinity.required_during_scheduling_ignored_during_execution.label_selector]
										[[runners.kubernetes.affinity.pod_anti_affinity.required_during_scheduling_ignored_during_execution.label_selector.match_expressions]]
											key = "security"
											operator = "In"
											values = ["S1"]
									[runners.kubernetes.affinity.pod_anti_affinity.required_during_scheduling_ignored_during_execution.namespace_selector]
										[[runners.kubernetes.affinity.pod_anti_affinity.required_during_scheduling_ignored_during_execution.namespace_selector.match_expressions]]
											key = "security"
											operator = "In"
											values = ["S1"]

								[[runners.kubernetes.affinity.pod_anti_affinity.preferred_during_scheduling_ignored_during_execution]]
								weight = 100
								[runners.kubernetes.affinity.pod_anti_affinity.preferred_during_scheduling_ignored_during_execution.pod_affinity_term]
									topology_key = "failure-domain.beta.kubernetes.io/zone"
									[runners.kubernetes.affinity.pod_anti_affinity.preferred_during_scheduling_ignored_during_execution.pod_affinity_term.label_selector]
										[[runners.kubernetes.affinity.pod_anti_affinity.preferred_during_scheduling_ignored_during_execution.pod_affinity_term.label_selector.match_expressions]]
											key = "security_2"
											operator = "In"
											values = ["S2"]
									[runners.kubernetes.affinity.pod_anti_affinity.preferred_during_scheduling_ignored_during_execution.pod_affinity_term.namespace_selector]
										[[runners.kubernetes.affinity.pod_anti_affinity.preferred_during_scheduling_ignored_during_execution.pod_affinity_term.namespace_selector.match_expressions]]
											key = "security_2"
											operator = "In"
											values = ["S2"]
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Len(t, config.Runners, 1)
				require.NotNil(t, config.Runners[0].Kubernetes.Affinity)
				require.NotNil(t, config.Runners[0].Kubernetes.Affinity.PodAntiAffinity)

				podAntiAffinity := config.Runners[0].Kubernetes.Affinity.PodAntiAffinity
				require.Len(t, podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution, 1)
				required := podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0]

				assert.Equal(t, "failure-domain.beta.kubernetes.io/zone", required.TopologyKey)
				assert.Equal(t, []string{"namespace_1", "namespace_2"}, required.Namespaces)

				require.NotNil(t, required.LabelSelector)
				require.Len(t, required.LabelSelector.MatchExpressions, 1)
				requiredMatchExp := required.LabelSelector.MatchExpressions[0]
				assert.Equal(t, "security", requiredMatchExp.Key)
				assert.Equal(t, "In", requiredMatchExp.Operator)
				assert.Equal(t, []string{"S1"}, requiredMatchExp.Values)

				require.NotNil(t, required.NamespaceSelector)
				require.Len(t, required.NamespaceSelector.MatchExpressions, 1)
				requiredMatchExp = required.NamespaceSelector.MatchExpressions[0]
				assert.Equal(t, "security", requiredMatchExp.Key)
				assert.Equal(t, "In", requiredMatchExp.Operator)
				assert.Equal(t, []string{"S1"}, requiredMatchExp.Values)

				require.Len(t, podAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution, 1)
				preferred := podAntiAffinity.PreferredDuringSchedulingIgnoredDuringExecution[0]
				assert.Equal(t, int32(100), preferred.Weight)
				assert.Empty(t, preferred.PodAffinityTerm.Namespaces)
				assert.Equal(t, "failure-domain.beta.kubernetes.io/zone", preferred.PodAffinityTerm.TopologyKey)

				require.NotNil(t, preferred.PodAffinityTerm.LabelSelector)
				require.Len(t, preferred.PodAffinityTerm.LabelSelector.MatchExpressions, 1)
				preferredMatchExp := preferred.PodAffinityTerm.LabelSelector.MatchExpressions[0]
				assert.Equal(t, "security_2", preferredMatchExp.Key)
				assert.Equal(t, "In", preferredMatchExp.Operator)
				assert.Equal(t, []string{"S2"}, preferredMatchExp.Values)

				require.NotNil(t, preferred.PodAffinityTerm.NamespaceSelector)
				require.Len(t, preferred.PodAffinityTerm.NamespaceSelector.MatchExpressions, 1)
				preferredMatchExp = preferred.PodAffinityTerm.NamespaceSelector.MatchExpressions[0]
				assert.Equal(t, "security_2", preferredMatchExp.Key)
				assert.Equal(t, "In", preferredMatchExp.Operator)
				assert.Equal(t, []string{"S2"}, preferredMatchExp.Values)
			},
		},
		"check that GracefulKillTimeout and ForceKillTimeout can't be set": {
			config: `
				[[runners]]
					GracefulKillTimeout = 30
					ForceKillTimeout = 10
			`,
			validateConfig: func(t *testing.T, config *Config) {
				assert.Nil(t, config.Runners[0].GracefulKillTimeout)
				assert.Nil(t, config.Runners[0].ForceKillTimeout)
			},
		},
		"setting DNS policy to none": {
			config: `
				[[runners]]
					[runners.kubernetes]
						dns_policy = 'none'
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Len(t, config.Runners, 1)

				dnsPolicy, err := config.Runners[0].Kubernetes.DNSPolicy.Get()
				assert.NoError(t, err)
				assert.Equal(t, api.DNSNone, dnsPolicy)
			},
		},
		"setting DNS policy to default": {
			config: `
				[[runners]]
					[runners.kubernetes]
						dns_policy = 'default'
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Len(t, config.Runners, 1)

				dnsPolicy, err := config.Runners[0].Kubernetes.DNSPolicy.Get()
				assert.NoError(t, err)
				assert.Equal(t, api.DNSDefault, dnsPolicy)
			},
		},
		"setting DNS policy to cluster-first": {
			config: `
				[[runners]]
					[runners.kubernetes]
						dns_policy = 'cluster-first'
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Len(t, config.Runners, 1)

				dnsPolicy, err := config.Runners[0].Kubernetes.DNSPolicy.Get()
				assert.NoError(t, err)
				assert.Equal(t, api.DNSClusterFirst, dnsPolicy)
			},
		},
		"setting DNS policy to cluster-first-with-host-net": {
			config: `
				[[runners]]
					[runners.kubernetes]
						dns_policy = 'cluster-first-with-host-net'
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Len(t, config.Runners, 1)

				dnsPolicy, err := config.Runners[0].Kubernetes.DNSPolicy.Get()
				assert.NoError(t, err)
				assert.Equal(t, api.DNSClusterFirstWithHostNet, dnsPolicy)
			},
		},
		"fail setting DNS policy to invalid value": {
			config: `
				[[runners]]
					[runners.kubernetes]
						dns_policy = 'some-invalid-policy'
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Len(t, config.Runners, 1)

				dnsPolicy, err := config.Runners[0].Kubernetes.DNSPolicy.Get()
				assert.Error(t, err)
				assert.Empty(t, dnsPolicy)
			},
		},
		"fail setting DNS policy to empty value returns default value": {
			config: `
				[[runners]]
					[runners.kubernetes]
						dns_policy = ''
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Len(t, config.Runners, 1)

				dnsPolicy, err := config.Runners[0].Kubernetes.DNSPolicy.Get()
				assert.NoError(t, err)
				assert.Equal(t, api.DNSClusterFirst, dnsPolicy)
			},
		},
		"check empty container lifecycle": {
			config: `
				[[runners]]
					[runners.kubernetes]
						namespace = "default"
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Len(t, config.Runners, 1)

				lifecycleCfg := config.Runners[0].Kubernetes.GetContainerLifecycle()
				assert.Nil(t, lifecycleCfg.PostStart)
				assert.Nil(t, lifecycleCfg.PreStop)
			},
		},
		"check postStart execAction configuration": {
			config: `
				[[runners]]
					[runners.kubernetes]
						namespace = "default"
						[runners.kubernetes.container_lifecycle.post_start.exec]
							command = ["ls", "-l"]
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Len(t, config.Runners, 1)

				lifecycleCfg := config.Runners[0].Kubernetes.GetContainerLifecycle()
				assert.NotNil(t, lifecycleCfg.PostStart)

				assert.Equal(t, []string{"ls", "-l"}, lifecycleCfg.PostStart.Exec.Command)
				assert.Nil(t, nil, lifecycleCfg.PostStart.HTTPGet)
				assert.Nil(t, nil, lifecycleCfg.PostStart.TCPSocket)
			},
		},
		"check postStart httpGetAction configuration": {
			config: `
				[[runners]]
					[runners.kubernetes]
						namespace = "default"
						[runners.kubernetes.container_lifecycle.post_start.http_get]
							port = 8080
							host = "localhost"
							path = "/test"
							[[runners.kubernetes.container_lifecycle.post_start.http_get.http_headers]]
								name = "header_name_1"
								value = "header_value_1"
							[[runners.kubernetes.container_lifecycle.post_start.http_get.http_headers]]
								name = "header_name_2"
								value = "header_value_2"
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Len(t, config.Runners, 1)

				lifecycleCfg := config.Runners[0].Kubernetes.GetContainerLifecycle()
				assert.NotNil(t, lifecycleCfg.PostStart)

				assert.Equal(t, 8080, lifecycleCfg.PostStart.HTTPGet.Port)
				assert.Equal(t, "localhost", lifecycleCfg.PostStart.HTTPGet.Host)
				assert.Equal(t, "/test", lifecycleCfg.PostStart.HTTPGet.Path)
				assert.Equal(t, httpHeaders, lifecycleCfg.PostStart.HTTPGet.HTTPHeaders)
			},
		},
		"check postStart tcpSocketAction configuration": {
			config: `
				[[runners]]
					[runners.kubernetes]
						namespace = "default"
						[runners.kubernetes.container_lifecycle.post_start.tcp_socket]
							port = 8080
							host = "localhost"
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Len(t, config.Runners, 1)

				lifecycleCfg := config.Runners[0].Kubernetes.GetContainerLifecycle()
				assert.NotNil(t, lifecycleCfg.PostStart)

				assert.Equal(t, 8080, lifecycleCfg.PostStart.TCPSocket.Port)
				assert.Equal(t, "localhost", lifecycleCfg.PostStart.TCPSocket.Host)
			},
		},
		"check preStop execAction configuration": {
			config: `
				[[runners]]
					[runners.kubernetes]
						namespace = "default"
						[runners.kubernetes.container_lifecycle.pre_stop.exec]
							command = ["ls", "-l"]
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Len(t, config.Runners, 1)

				lifecycleCfg := config.Runners[0].Kubernetes.GetContainerLifecycle()
				assert.NotNil(t, lifecycleCfg.PreStop)

				assert.Equal(t, []string{"ls", "-l"}, lifecycleCfg.PreStop.Exec.Command)
				assert.Nil(t, nil, lifecycleCfg.PreStop.HTTPGet)
				assert.Nil(t, nil, lifecycleCfg.PreStop.TCPSocket)
			},
		},
		"check preStop httpGetAction configuration": {
			config: `
				[[runners]]
					[runners.kubernetes]
						namespace = "default"
						[runners.kubernetes.container_lifecycle.pre_stop.http_get]
						port = 8080
						host = "localhost"
						path = "/test"
						[[runners.kubernetes.container_lifecycle.pre_stop.http_get.http_headers]]
							name = "header_name_1"
							value = "header_value_1"
						[[runners.kubernetes.container_lifecycle.pre_stop.http_get.http_headers]]
							name = "header_name_2"
							value = "header_value_2"
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Len(t, config.Runners, 1)

				lifecycleCfg := config.Runners[0].Kubernetes.GetContainerLifecycle()
				assert.NotNil(t, lifecycleCfg.PreStop)

				assert.Equal(t, 8080, lifecycleCfg.PreStop.HTTPGet.Port)
				assert.Equal(t, "localhost", lifecycleCfg.PreStop.HTTPGet.Host)
				assert.Equal(t, "/test", lifecycleCfg.PreStop.HTTPGet.Path)
				assert.Equal(t, httpHeaders, lifecycleCfg.PreStop.HTTPGet.HTTPHeaders)
			},
		},
		"check preStop tcpSocketAction configuration": {
			config: `
				[[runners]]
					[runners.kubernetes]
						namespace = "default"
						[runners.kubernetes.container_lifecycle.pre_stop.tcp_socket]
							port = 8080
							host = "localhost"
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Len(t, config.Runners, 1)

				lifecycleCfg := config.Runners[0].Kubernetes.GetContainerLifecycle()
				assert.NotNil(t, lifecycleCfg.PreStop)

				assert.Equal(t, 8080, lifecycleCfg.PreStop.TCPSocket.Port)
				assert.Equal(t, "localhost", lifecycleCfg.PreStop.TCPSocket.Host)
			},
		},
		"setting Priority Class to priority-1": {
			config: `
				[[runners]]
					[runners.kubernetes]
						priority_class_name = 'priority-1'
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Len(t, config.Runners, 1)

				priorityClassName := config.Runners[0].Kubernetes.PriorityClassName
				assert.Equal(t, "priority-1", priorityClassName)
			},
		},
		"setting scheduler_name to foobar": {
			config: `
				[[runners]]
					[runners.kubernetes]
						scheduler_name = 'foobar'
			`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Len(t, config.Runners, 1)

				schedulerName := config.Runners[0].Kubernetes.SchedulerName
				assert.Equal(t, "foobar", schedulerName)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			cfg := NewConfig()
			_, err := toml.Decode(tt.config, cfg)
			if tt.expectedErr != "" {
				assert.ErrorContains(t, err, tt.expectedErr)
				return
			}

			assert.NoError(t, err)
			if tt.validateConfig != nil {
				tt.validateConfig(t, cfg)
			}
		})
	}
}

func TestKubernetesHostAliases(t *testing.T) {
	tests := map[string]struct {
		config              KubernetesConfig
		expectedHostAliases []api.HostAlias
	}{
		"parse Kubernetes HostAliases with empty list": {
			config:              KubernetesConfig{},
			expectedHostAliases: nil,
		},
		"parse Kubernetes HostAliases with unique ips": {
			config: KubernetesConfig{
				HostAliases: []KubernetesHostAliases{
					{
						IP:        "127.0.0.1",
						Hostnames: []string{"web1", "web2"},
					},
					{
						IP:        "192.168.1.1",
						Hostnames: []string{"web14", "web15"},
					},
				},
			},
			expectedHostAliases: []api.HostAlias{
				{
					IP:        "127.0.0.1",
					Hostnames: []string{"web1", "web2"},
				},
				{
					IP:        "192.168.1.1",
					Hostnames: []string{"web14", "web15"},
				},
			},
		},
		"parse Kubernetes HostAliases with duplicated ip": {
			config: KubernetesConfig{
				HostAliases: []KubernetesHostAliases{
					{
						IP:        "127.0.0.1",
						Hostnames: []string{"web1", "web2"},
					},
					{
						IP:        "127.0.0.1",
						Hostnames: []string{"web14", "web15"},
					},
				},
			},
			expectedHostAliases: []api.HostAlias{
				{
					IP:        "127.0.0.1",
					Hostnames: []string{"web1", "web2"},
				},
				{
					IP:        "127.0.0.1",
					Hostnames: []string{"web14", "web15"},
				},
			},
		},
		"parse Kubernetes HostAliases with duplicated hostname": {
			config: KubernetesConfig{
				HostAliases: []KubernetesHostAliases{
					{
						IP:        "127.0.0.1",
						Hostnames: []string{"web1", "web1", "web2"},
					},
					{
						IP:        "127.0.0.1",
						Hostnames: []string{"web1", "web15"},
					},
				},
			},
			expectedHostAliases: []api.HostAlias{
				{
					IP:        "127.0.0.1",
					Hostnames: []string{"web1", "web1", "web2"},
				},
				{
					IP:        "127.0.0.1",
					Hostnames: []string{"web1", "web15"},
				},
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.Equal(t, tt.expectedHostAliases, tt.config.GetHostAliases())
		})
	}
}

func TestService_ToImageDefinition(t *testing.T) {
	tests := map[string]struct {
		service       Service
		expectedImage spec.Image
	}{
		"empty service": {
			service:       Service{},
			expectedImage: spec.Image{},
		},
		"only name": {
			service:       Service{Name: "name"},
			expectedImage: spec.Image{Name: "name"},
		},
		"only alias": {
			service:       Service{Alias: "alias"},
			expectedImage: spec.Image{Alias: "alias"},
		},
		"name and alias": {
			service:       Service{Name: "name", Alias: "alias"},
			expectedImage: spec.Image{Name: "name", Alias: "alias"},
		},
		"only aliases": {
			service:       Service{Alias: "alias-1 alias-2"},
			expectedImage: spec.Image{Alias: "alias-1 alias-2"},
		},
		"name and aliases": {
			service:       Service{Name: "name", Alias: "alias-1 alias-2"},
			expectedImage: spec.Image{Name: "name", Alias: "alias-1 alias-2"},
		},
		"command specified": {
			service:       Service{Name: "name", Command: []string{"executable", "param1", "param2"}},
			expectedImage: spec.Image{Name: "name", Command: []string{"executable", "param1", "param2"}},
		},
		"entrypoint specified": {
			service:       Service{Name: "name", Entrypoint: []string{"executable", "param3", "param4"}},
			expectedImage: spec.Image{Name: "name", Entrypoint: []string{"executable", "param3", "param4"}},
		},
		"command and entrypoint specified": {
			service: Service{
				Name:       "name",
				Command:    []string{"executable", "param1", "param2"},
				Entrypoint: []string{"executable", "param3", "param4"},
			},
			expectedImage: spec.Image{
				Name:       "name",
				Command:    []string{"executable", "param1", "param2"},
				Entrypoint: []string{"executable", "param3", "param4"},
			},
		},
		"environment specified": {
			service: Service{Name: "name", Environment: []string{"ENV1=value1", "ENV2=value2"}},
			expectedImage: spec.Image{Name: "name", Variables: spec.Variables{
				{Key: "ENV1", Value: "value1", Internal: true},
				{Key: "ENV2", Value: "value2", Internal: true},
			}},
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
		"offpeak active ignored": {
			config: &DockerMachine{
				IdleCount:        1,
				IdleTime:         1000,
				OffPeakPeriods:   activeTimePeriod,
				OffPeakIdleCount: 2,
				OffPeakIdleTime:  2000,
			},
			expectedIdleCount: 1,
			expectedIdleTime:  1000,
		},
		"offpeak inactive ignored": {
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
		"offpeak invalid format ignored": {
			config: &DockerMachine{
				OffPeakPeriods:   invalidTimePeriod,
				OffPeakIdleCount: 2,
				OffPeakIdleTime:  2000,
			},
			expectedIdleCount: 0,
			expectedIdleTime:  0,
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
				assert.ErrorIs(t, err, tt.expectedErr)
				return
			}
			assert.NoError(t, err, "should not return err on good period compile")
			assert.Equal(t, tt.expectedIdleCount, tt.config.GetIdleCount())
			assert.Equal(t, tt.expectedIdleTime, tt.config.GetIdleTime())
		})
	}
}

func TestRunnerSettings_GetGracefulKillTimeout_GetForceKillTimeout(t *testing.T) {
	tests := map[string]struct {
		config                      RunnerSettings
		expectedGracefulKillTimeout time.Duration
		expectedForceKillTimeout    time.Duration
	}{
		"undefined": {
			config:                      RunnerSettings{},
			expectedGracefulKillTimeout: process.GracefulTimeout,
			expectedForceKillTimeout:    process.KillTimeout,
		},
		"timeouts lower than 0": {
			config: RunnerSettings{
				GracefulKillTimeout: func(i int) *int { return &i }(-10),
				ForceKillTimeout:    func(i int) *int { return &i }(-10),
			},
			expectedGracefulKillTimeout: process.GracefulTimeout,
			expectedForceKillTimeout:    process.KillTimeout,
		},
		"timeouts greater than 0": {
			config: RunnerSettings{
				GracefulKillTimeout: func(i int) *int { return &i }(30),
				ForceKillTimeout:    func(i int) *int { return &i }(15),
			},
			expectedGracefulKillTimeout: 30 * time.Second,
			expectedForceKillTimeout:    15 * time.Second,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.Equal(t, tt.expectedGracefulKillTimeout, tt.config.GetGracefulKillTimeout())
			assert.Equal(t, tt.expectedForceKillTimeout, tt.config.GetForceKillTimeout())
		})
	}
}

func TestDockerConfig_GetPullPolicies(t *testing.T) {
	tests := map[string]struct {
		config               DockerConfig
		expectedPullPolicies []DockerPullPolicy
		expectedErr          bool
	}{
		"nil pull_policy": {
			config:               DockerConfig{},
			expectedPullPolicies: []DockerPullPolicy{PullPolicyAlways},
			expectedErr:          false,
		},
		"empty pull_policy": {
			config:               DockerConfig{PullPolicy: StringOrArray{}},
			expectedPullPolicies: []DockerPullPolicy{PullPolicyAlways},
			expectedErr:          false,
		},
		"empty string pull_policy": {
			config:      DockerConfig{PullPolicy: StringOrArray{""}},
			expectedErr: true,
		},
		"known elements in pull_policy": {
			config: DockerConfig{
				PullPolicy: StringOrArray{PullPolicyAlways, PullPolicyIfNotPresent, PullPolicyNever},
			},
			expectedPullPolicies: []DockerPullPolicy{PullPolicyAlways, PullPolicyIfNotPresent, PullPolicyNever},
			expectedErr:          false,
		},
		"invalid pull_policy": {
			config:      DockerConfig{PullPolicy: StringOrArray{"invalid"}},
			expectedErr: true,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			policies, err := tt.config.GetPullPolicies()

			if tt.expectedErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedPullPolicies, policies)
		})
	}
}

func TestDockerConfig_GetAllowedPullPolicies(t *testing.T) {
	tests := map[string]struct {
		config               DockerConfig
		expectedPullPolicies []DockerPullPolicy
		expectedErr          bool
	}{
		"nil allowed_pull_policies": {
			config:               DockerConfig{},
			expectedPullPolicies: []DockerPullPolicy{PullPolicyAlways},
			expectedErr:          false,
		},
		"empty allowed_pull_policies": {
			config:               DockerConfig{AllowedPullPolicies: []DockerPullPolicy{}},
			expectedPullPolicies: []DockerPullPolicy{PullPolicyAlways},
			expectedErr:          false,
		},
		"empty string allowed_pull_policies": {
			config:      DockerConfig{AllowedPullPolicies: []DockerPullPolicy{""}},
			expectedErr: true,
		},
		"known elements in allowed_pull_policies": {
			config: DockerConfig{
				AllowedPullPolicies: []DockerPullPolicy{PullPolicyAlways, PullPolicyNever},
			},
			expectedPullPolicies: []DockerPullPolicy{PullPolicyAlways, PullPolicyNever},
			expectedErr:          false,
		},
		"invalid allowed_pull_policies": {
			config:      DockerConfig{AllowedPullPolicies: []DockerPullPolicy{"invalid"}},
			expectedErr: true,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			policies, err := tt.config.GetAllowedPullPolicies()

			if tt.expectedErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedPullPolicies, policies)
		})
	}
}

func TestKubernetesConfig_GetAllowedPullPolicies(t *testing.T) {
	tests := map[string]struct {
		config               KubernetesConfig
		expectedPullPolicies []api.PullPolicy
		expectedErr          bool
	}{
		"nil allowed_pull_policies": {
			config:               KubernetesConfig{},
			expectedPullPolicies: []api.PullPolicy{""},
			expectedErr:          false,
		},
		"empty allowed_pull_policies": {
			config: KubernetesConfig{
				AllowedPullPolicies: []DockerPullPolicy{},
			},
			expectedPullPolicies: []api.PullPolicy{""},
			expectedErr:          false,
		},
		"empty string allowed_pull_policies": {
			config: KubernetesConfig{
				AllowedPullPolicies: []DockerPullPolicy{""},
			},
			expectedPullPolicies: []api.PullPolicy{""},
			expectedErr:          false,
		},
		"known elements in allowed_pull_policies": {
			config: KubernetesConfig{
				AllowedPullPolicies: []DockerPullPolicy{PullPolicyAlways, PullPolicyNever},
			},
			expectedPullPolicies: []api.PullPolicy{api.PullAlways, api.PullNever},
			expectedErr:          false,
		},
		"invalid allowed_pull_policies": {
			config: KubernetesConfig{
				AllowedPullPolicies: []DockerPullPolicy{"invalid"},
			},
			expectedErr: true,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			policies, err := tt.config.GetAllowedPullPolicies()

			if tt.expectedErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedPullPolicies, policies)
		})
	}
}

func TestKubernetesConfig_GetPullPolicies(t *testing.T) {
	tests := map[string]struct {
		config               KubernetesConfig
		expectedPullPolicies []api.PullPolicy
		expectedErr          bool
	}{
		"nil pull_policy": {
			config:               KubernetesConfig{},
			expectedPullPolicies: []api.PullPolicy{""},
			expectedErr:          false,
		},
		"empty pull_policy": {
			config:               KubernetesConfig{PullPolicy: StringOrArray{}},
			expectedPullPolicies: []api.PullPolicy{""},
			expectedErr:          false,
		},
		"empty string pull_policy": {
			config:               KubernetesConfig{PullPolicy: StringOrArray{""}},
			expectedPullPolicies: []api.PullPolicy{""},
			expectedErr:          false,
		},
		"known elements in pull_policy": {
			config: KubernetesConfig{
				PullPolicy: StringOrArray{PullPolicyAlways, PullPolicyIfNotPresent, PullPolicyNever},
			},
			expectedPullPolicies: []api.PullPolicy{api.PullAlways, api.PullIfNotPresent, api.PullNever},
			expectedErr:          false,
		},
		"invalid pull_policy": {
			config:      KubernetesConfig{PullPolicy: StringOrArray{"invalid"}},
			expectedErr: true,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			policies, err := tt.config.GetPullPolicies()

			if tt.expectedErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedPullPolicies, policies)
		})
	}
}

func TestKubernetesConfig_ConvertFromDockerPullPolicy(t *testing.T) {
	tests := map[string]struct {
		config               KubernetesConfig
		dockerPullPolicies   []DockerPullPolicy
		expectedPullPolicies []api.PullPolicy
		expectedErr          bool
	}{
		"valid list": {
			config:               KubernetesConfig{},
			dockerPullPolicies:   []DockerPullPolicy{PullPolicyAlways, PullPolicyIfNotPresent, PullPolicyNever},
			expectedPullPolicies: []api.PullPolicy{api.PullAlways, api.PullIfNotPresent, api.PullNever},
			expectedErr:          false,
		},
		"has an invalid pull policy": {
			config:               KubernetesConfig{},
			dockerPullPolicies:   []DockerPullPolicy{PullPolicyAlways, "invalid"},
			expectedPullPolicies: []api.PullPolicy{""},
			expectedErr:          true,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			policies, err := tt.config.ConvertFromDockerPullPolicy(tt.dockerPullPolicies)

			if tt.expectedErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedPullPolicies, policies)
		})
	}
}

func TestStringOrArray_UnmarshalTOML(t *testing.T) {
	tests := map[string]struct {
		toml           string
		expectedResult StringOrArray
		expectedErr    bool
	}{
		"no fields": {
			toml:           "",
			expectedResult: nil,
			expectedErr:    false,
		},
		"empty string_or_array": {
			toml:           `string_or_array = ""`,
			expectedResult: StringOrArray{""},
			expectedErr:    false,
		},
		"string": {
			toml:           `string_or_array = "always"`,
			expectedResult: StringOrArray{"always"},
			expectedErr:    false,
		},
		"slice with invalid single value": {
			toml:        `string_or_array = 10`,
			expectedErr: true,
		},
		"valid slice with multiple values": {
			toml:           `string_or_array = ["unknown", "always"]`,
			expectedResult: StringOrArray{"unknown", "always"},
			expectedErr:    false,
		},
		"slice with mixed values": {
			toml:        `string_or_array = ["unknown", 10]`,
			expectedErr: true,
		},
		"slice with invalid values": {
			toml:        `string_or_array = [true, false]`,
			expectedErr: true,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			type Config struct {
				StringOrArray StringOrArray `toml:"string_or_array"`
			}

			var result Config
			_, err := toml.Decode(tt.toml, &result)

			if tt.expectedErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.expectedResult, result.StringOrArray)
		})
	}
}

func TestAutoscalerPolicyConfig_PreemptiveModeEnabled(t *testing.T) {
	tests := map[string]struct {
		internalValue *bool
		idleCount     int
		expectedValue bool
	}{
		"should return enabled when flag is true": {
			internalValue: ptr(true),
			expectedValue: true,
		},
		"should return turned off when flag is false": {
			internalValue: ptr(false),
			expectedValue: false,
		},
		"should return turned off when flag is false and idle count is greater than zero": {
			idleCount:     10,
			internalValue: ptr(false),
			expectedValue: false,
		},
		"should return turned off when value is not set and the idle count is zero": {
			idleCount:     0,
			internalValue: nil,
			expectedValue: false,
		},
		"should return enabled when value is not set and the idle count is greater than zero": {
			idleCount:     10,
			internalValue: nil,
			expectedValue: true,
		},
	}
	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			config := AutoscalerPolicyConfig{
				PreemptiveMode: tt.internalValue,
				IdleCount:      tt.idleCount,
			}

			result := config.PreemptiveModeEnabled()

			assert.Equal(t, tt.expectedValue, result)
		})
	}
}

func TestRunnerSettings_IsFeatureFlagOn(t *testing.T) {
	tests := map[string]struct {
		featureFlags  map[string]bool
		name          string
		expectedValue bool
	}{
		"feature flag not configured": {
			featureFlags:  map[string]bool{},
			name:          t.Name(),
			expectedValue: false,
		},
		"feature flag not configured but feature flag default is true": {
			featureFlags:  map[string]bool{},
			name:          featureflags.UseDirectDownload,
			expectedValue: true,
		},
		"feature flag on": {
			featureFlags: map[string]bool{
				t.Name(): true,
			},
			name:          t.Name(),
			expectedValue: true,
		},
		"feature flag off": {
			featureFlags: map[string]bool{
				featureflags.UseDirectDownload: false,
			},
			name:          t.Name(),
			expectedValue: false,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			cfg := RunnerConfig{
				RunnerSettings: RunnerSettings{
					FeatureFlags: tt.featureFlags,
				},
			}

			on := cfg.IsFeatureFlagOn(tt.name)
			assert.Equal(t, tt.expectedValue, on)
		})
	}
}

func TestEffectivePrivilege(t *testing.T) {
	tests := map[string]struct {
		pod       bool
		container bool
		expected  bool
	}{
		"pod and container privileged": {
			pod:       true,
			container: true,
			expected:  true,
		},
		"pod privileged": {
			pod:       true,
			container: false,
			expected:  false,
		},
		"container privileged": {
			pod:       false,
			container: true,
			expected:  true,
		},
		"all unprivileged": {
			pod:       false,
			container: false,
			expected:  false,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			effectivePrivileged := getContainerSecurityContextEffectiveFlagValue(&tt.container, &tt.pod)
			require.NotNil(t, effectivePrivileged)
			assert.Equal(t, tt.expected, *effectivePrivileged)
		})
	}
}

func TestContainerSecurityContext(t *testing.T) {
	tests := map[string]struct {
		getSecurityContext                  func(c *KubernetesConfig) *api.SecurityContext
		getExpectedContainerSecurityContext func() *api.SecurityContext
	}{
		"no container security context": {
			getSecurityContext: func(c *KubernetesConfig) *api.SecurityContext {
				return c.GetContainerSecurityContext(KubernetesContainerSecurityContext{})
			},
			getExpectedContainerSecurityContext: func() *api.SecurityContext {
				return &api.SecurityContext{}
			},
		},
		"run as user - container security context": {
			getSecurityContext: func(c *KubernetesConfig) *api.SecurityContext {
				return c.GetContainerSecurityContext(KubernetesContainerSecurityContext{
					RunAsUser: Int64Ptr(1000),
				})
			},
			getExpectedContainerSecurityContext: func() *api.SecurityContext {
				runAsUser := int64(1000)
				return &api.SecurityContext{
					RunAsUser: &runAsUser,
				}
			},
		},
		"privileged - container security context": {
			getSecurityContext: func(c *KubernetesConfig) *api.SecurityContext {
				return c.GetContainerSecurityContext(KubernetesContainerSecurityContext{
					Privileged: ptr(true),
				})
			},
			getExpectedContainerSecurityContext: func() *api.SecurityContext {
				return &api.SecurityContext{
					Privileged: ptr(true),
				}
			},
		},
		"container privileged override - container security context": {
			getSecurityContext: func(c *KubernetesConfig) *api.SecurityContext {
				c.Privileged = ptr(true)
				return c.GetContainerSecurityContext(KubernetesContainerSecurityContext{
					Privileged: ptr(false),
					RunAsUser:  Int64Ptr(65535),
				})
			},
			getExpectedContainerSecurityContext: func() *api.SecurityContext {
				runAsUser := int64(65535)
				return &api.SecurityContext{
					Privileged: ptr(false),
					RunAsUser:  &runAsUser,
				}
			},
		},
		"allow privilege escalation - not set on container security context": {
			getSecurityContext: func(c *KubernetesConfig) *api.SecurityContext {
				return c.GetContainerSecurityContext(KubernetesContainerSecurityContext{
					AllowPrivilegeEscalation: ptr(true),
				})
			},
			getExpectedContainerSecurityContext: func() *api.SecurityContext {
				return &api.SecurityContext{
					AllowPrivilegeEscalation: ptr(true),
				}
			},
		},
		"allow privilege escalation - set on container security context": {
			getSecurityContext: func(c *KubernetesConfig) *api.SecurityContext {
				c.AllowPrivilegeEscalation = ptr(true)
				return c.GetContainerSecurityContext(KubernetesContainerSecurityContext{
					AllowPrivilegeEscalation: ptr(false),
				})
			},
			getExpectedContainerSecurityContext: func() *api.SecurityContext {
				return &api.SecurityContext{
					AllowPrivilegeEscalation: ptr(false),
				}
			},
		},
		"SELinux type label - container security context": {
			getSecurityContext: func(c *KubernetesConfig) *api.SecurityContext {
				return c.GetContainerSecurityContext(KubernetesContainerSecurityContext{
					SELinuxType: "spc_t",
				})
			},
			getExpectedContainerSecurityContext: func() *api.SecurityContext {
				return &api.SecurityContext{
					SELinuxOptions: &api.SELinuxOptions{Type: "spc_t"},
				}
			},
		},
		"proc mount - blank": {
			getSecurityContext: func(c *KubernetesConfig) *api.SecurityContext {
				return c.GetContainerSecurityContext(KubernetesContainerSecurityContext{
					ProcMount: "",
				})
			},
			getExpectedContainerSecurityContext: func() *api.SecurityContext {
				return &api.SecurityContext{
					ProcMount: nil,
				}
			},
		},
		"proc mount - invalid": {
			getSecurityContext: func(c *KubernetesConfig) *api.SecurityContext {
				return c.GetContainerSecurityContext(KubernetesContainerSecurityContext{
					ProcMount: "invalid",
				})
			},
			getExpectedContainerSecurityContext: func() *api.SecurityContext {
				return &api.SecurityContext{
					ProcMount: nil,
				}
			},
		},
		"proc mount - default": {
			getSecurityContext: func(c *KubernetesConfig) *api.SecurityContext {
				return c.GetContainerSecurityContext(KubernetesContainerSecurityContext{
					ProcMount: "default",
				})
			},
			getExpectedContainerSecurityContext: func() *api.SecurityContext {
				pm := api.DefaultProcMount

				return &api.SecurityContext{
					ProcMount: &pm,
				}
			},
		},
		"proc mount - unmasked": {
			getSecurityContext: func(c *KubernetesConfig) *api.SecurityContext {
				return c.GetContainerSecurityContext(KubernetesContainerSecurityContext{
					ProcMount: "unmasked",
				})
			},
			getExpectedContainerSecurityContext: func() *api.SecurityContext {
				pm := api.UnmaskedProcMount

				return &api.SecurityContext{
					ProcMount: &pm,
				}
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			config := new(KubernetesConfig)
			scExpected := tt.getExpectedContainerSecurityContext()
			scActual := tt.getSecurityContext(config)
			assert.Equal(t, scExpected, scActual)
		})
	}
}

func TestKubernetesPodSpecContents(t *testing.T) {
	tests := map[string]struct {
		patchPath     string
		patchContents string
		patchType     KubernetesPodSpecPatchType

		expectedContents string
		expectedType     KubernetesPodSpecPatchType
		expectedErr      error
	}{
		"yaml to json": {
			patchContents:    `hostname: "test"`,
			expectedContents: `{"hostname":"test"}`,
			expectedType:     PatchTypeStrategicMergePatchType,
		},
		"json without format to json": {
			patchContents:    `{"hostname":"test"}`,
			expectedContents: `{"hostname":"test"}`,
			expectedType:     PatchTypeStrategicMergePatchType,
		},
		"json to json": {
			patchContents:    `{"hostname": {"test": "value"}}`,
			expectedContents: `{"hostname":{"test":"value"}}`,
			expectedType:     PatchTypeStrategicMergePatchType,
		},
		"invalid json": {
			patchContents: `{"hostname": {{}"test": "value"}}`,
			expectedType:  PatchTypeStrategicMergePatchType,
			expectedErr:   errPatchConversion,
		},
		"invalid yaml": {
			patchContents: `[invalid yaml`,
			expectedErr:   errPatchConversion,
		},
		"missing file": {
			patchPath:   "missing/file",
			expectedErr: errPatchFileFail,
		},
		"patch_path and patch ambiguous": {
			patchPath:     "missing/file",
			patchContents: `{"hostname": {"test": "value"}}`,
			expectedErr:   errPatchAmbiguous,
		},
		"explicit patch type": {
			patchContents:    `hostname: "test"`,
			patchType:        PatchTypeMergePatchType,
			expectedContents: `{"hostname":"test"}`,
			expectedType:     PatchTypeMergePatchType,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			s := KubernetesPodSpec{
				PatchPath: tc.patchPath,
				Patch:     tc.patchContents,
				PatchType: tc.patchType,
			}
			patchBytes, patchType, err := s.PodSpecPatch()
			if tc.expectedErr != nil {
				require.ErrorIs(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.expectedContents, string(patchBytes))
				assert.Equal(t, tc.expectedType, patchType)
			}
		})
	}
}

func TestContainerSecurityCapabilities(t *testing.T) {
	tests := map[string]struct {
		getCapabilitiesFn    func(c *KubernetesConfig) *api.Capabilities
		expectedCapabilities *api.Capabilities
	}{
		"container add": {
			getCapabilitiesFn: func(c *KubernetesConfig) *api.Capabilities {
				return c.GetContainerSecurityContext(KubernetesContainerSecurityContext{
					Capabilities: &KubernetesContainerCapabilities{
						Add: []api.Capability{"SYS_TIME"},
					},
				}).Capabilities
			},
			expectedCapabilities: &api.Capabilities{
				Add:  []api.Capability{"SYS_TIME"},
				Drop: nil,
			},
		},
		"container drop": {
			getCapabilitiesFn: func(c *KubernetesConfig) *api.Capabilities {
				return c.GetContainerSecurityContext(KubernetesContainerSecurityContext{
					Capabilities: &KubernetesContainerCapabilities{
						Drop: []api.Capability{"SYS_TIME"},
					},
				}).Capabilities
			},
			expectedCapabilities: &api.Capabilities{
				Add:  nil,
				Drop: []api.Capability{"SYS_TIME"},
			},
		},
		"container add and drop": {
			getCapabilitiesFn: func(c *KubernetesConfig) *api.Capabilities {
				return c.GetContainerSecurityContext(KubernetesContainerSecurityContext{
					Capabilities: &KubernetesContainerCapabilities{
						Add:  []api.Capability{"SYS_TIME"},
						Drop: []api.Capability{"SYS_TIME"},
					},
				}).Capabilities
			},
			expectedCapabilities: &api.Capabilities{
				Add:  []api.Capability{"SYS_TIME"},
				Drop: []api.Capability{"SYS_TIME"},
			},
		},
		"container empty": {
			getCapabilitiesFn: func(c *KubernetesConfig) *api.Capabilities {
				return c.GetContainerSecurityContext(KubernetesContainerSecurityContext{}).Capabilities
			},
		},
		"container when capAdd and capDrop exist": {
			getCapabilitiesFn: func(c *KubernetesConfig) *api.Capabilities {
				c.CapAdd = []string{"add"}
				c.CapDrop = []string{"drop"}
				return c.GetContainerSecurityContext(KubernetesContainerSecurityContext{}).Capabilities
			},
			expectedCapabilities: &api.Capabilities{
				Add:  []api.Capability{"add"},
				Drop: []api.Capability{"drop"},
			},
		},
		"container when capAdd and container capabilities exist": {
			getCapabilitiesFn: func(c *KubernetesConfig) *api.Capabilities {
				c.CapAdd = []string{"add"}
				c.CapDrop = []string{"drop"}
				return c.GetContainerSecurityContext(KubernetesContainerSecurityContext{
					Capabilities: &KubernetesContainerCapabilities{
						Add: []api.Capability{"add container"},
					},
				}).Capabilities
			},
			expectedCapabilities: &api.Capabilities{
				Add:  []api.Capability{"add container"},
				Drop: []api.Capability{"drop"},
			},
		},
		"container when capDrop and container capabilities exist": {
			getCapabilitiesFn: func(c *KubernetesConfig) *api.Capabilities {
				c.CapAdd = []string{"add"}
				c.CapDrop = []string{"drop"}
				return c.GetContainerSecurityContext(KubernetesContainerSecurityContext{
					Capabilities: &KubernetesContainerCapabilities{
						Drop: []api.Capability{"drop container"},
					},
				}).Capabilities
			},
			expectedCapabilities: &api.Capabilities{
				Add:  []api.Capability{"add"},
				Drop: []api.Capability{"drop container"},
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			config := new(KubernetesConfig)
			c := tt.getCapabilitiesFn(config)
			assert.Equal(t, tt.expectedCapabilities, c)
		})
	}
}

func TestGetCapabilities(t *testing.T) {
	tests := map[string]struct {
		defaultCapDrop     []string
		capAdd             []string
		capDrop            []string
		assertCapabilities func(t *testing.T, a *api.Capabilities)
	}{
		"no data provided": {
			assertCapabilities: func(t *testing.T, a *api.Capabilities) {
				assert.Nil(t, a)
			},
		},
		"only default_cap_drop provided": {
			defaultCapDrop: []string{"CAP_1", "CAP_2"},
			assertCapabilities: func(t *testing.T, a *api.Capabilities) {
				require.NotNil(t, a)
				assert.Empty(t, a.Add)
				assert.Len(t, a.Drop, 2)
				assert.Contains(t, a.Drop, api.Capability("CAP_1"))
				assert.Contains(t, a.Drop, api.Capability("CAP_2"))
			},
		},
		"only custom cap_add provided": {
			capAdd: []string{"CAP_1", "CAP_2"},
			assertCapabilities: func(t *testing.T, a *api.Capabilities) {
				require.NotNil(t, a)
				assert.Len(t, a.Add, 2)
				assert.Contains(t, a.Add, api.Capability("CAP_1"))
				assert.Contains(t, a.Add, api.Capability("CAP_2"))
				assert.Empty(t, a.Drop)
			},
		},
		"only custom cap_drop provided": {
			capDrop: []string{"CAP_1", "CAP_2"},
			assertCapabilities: func(t *testing.T, a *api.Capabilities) {
				require.NotNil(t, a)
				assert.Empty(t, a.Add)
				assert.Len(t, a.Drop, 2)
				assert.Contains(t, a.Drop, api.Capability("CAP_1"))
				assert.Contains(t, a.Drop, api.Capability("CAP_2"))
			},
		},
		"default_cap_drop and custom cap_drop sums": {
			defaultCapDrop: []string{"CAP_1", "CAP_2"},
			capDrop:        []string{"CAP_3", "CAP_4"},
			assertCapabilities: func(t *testing.T, a *api.Capabilities) {
				require.NotNil(t, a)
				assert.Empty(t, a.Add)
				assert.Len(t, a.Drop, 4)
				assert.Contains(t, a.Drop, api.Capability("CAP_1"))
				assert.Contains(t, a.Drop, api.Capability("CAP_2"))
				assert.Contains(t, a.Drop, api.Capability("CAP_3"))
				assert.Contains(t, a.Drop, api.Capability("CAP_4"))
			},
		},
		"default_cap_drop and custom cap_drop duplicate": {
			defaultCapDrop: []string{"CAP_1", "CAP_2"},
			capDrop:        []string{"CAP_2", "CAP_3"},
			assertCapabilities: func(t *testing.T, a *api.Capabilities) {
				require.NotNil(t, a)
				assert.Empty(t, a.Add)
				assert.Len(t, a.Drop, 3)
				assert.Contains(t, a.Drop, api.Capability("CAP_1"))
				assert.Contains(t, a.Drop, api.Capability("CAP_2"))
				assert.Contains(t, a.Drop, api.Capability("CAP_3"))
			},
		},
		"default_cap_drop and custom cap_add intersect": {
			defaultCapDrop: []string{"CAP_1", "CAP_2"},
			capAdd:         []string{"CAP_2", "CAP_3"},
			assertCapabilities: func(t *testing.T, a *api.Capabilities) {
				require.NotNil(t, a)
				assert.Len(t, a.Add, 2)
				assert.Contains(t, a.Add, api.Capability("CAP_2"))
				assert.Contains(t, a.Add, api.Capability("CAP_3"))
				assert.Len(t, a.Drop, 1)
				assert.Contains(t, a.Drop, api.Capability("CAP_1"))
			},
		},
		"default_cap_drop and custom cap_add intersect and cap_drop forces": {
			defaultCapDrop: []string{"CAP_1", "CAP_2"},
			capAdd:         []string{"CAP_2", "CAP_3"},
			capDrop:        []string{"CAP_2", "CAP_4"},
			assertCapabilities: func(t *testing.T, a *api.Capabilities) {
				require.NotNil(t, a)
				assert.Len(t, a.Add, 1)
				assert.Contains(t, a.Add, api.Capability("CAP_3"))
				assert.Len(t, a.Drop, 3)
				assert.Contains(t, a.Drop, api.Capability("CAP_1"))
				assert.Contains(t, a.Drop, api.Capability("CAP_2"))
				assert.Contains(t, a.Drop, api.Capability("CAP_4"))
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			c := KubernetesConfig{
				CapAdd:  tt.capAdd,
				CapDrop: tt.capDrop,
			}

			tt.assertCapabilities(t, c.getCapabilities(tt.defaultCapDrop))
		})
	}
}

func TestKubernetesTerminationPeriod(t *testing.T) {
	tests := map[string]struct {
		cfg                                      KubernetesConfig
		expectedPodTerminationGracePeriodSeconds *int64
		expectedCleanupGracePeriodSeconds        *int64
	}{
		"all default values": {
			cfg:                                      KubernetesConfig{},
			expectedPodTerminationGracePeriodSeconds: nil,
			expectedCleanupGracePeriodSeconds:        nil,
		},
		"all specified": {
			cfg: KubernetesConfig{
				PodTerminationGracePeriodSeconds: Int64Ptr(3),
				CleanupGracePeriodSeconds:        Int64Ptr(5),
			},
			expectedPodTerminationGracePeriodSeconds: Int64Ptr(3),
			expectedCleanupGracePeriodSeconds:        Int64Ptr(5),
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			assert.EqualValues(
				t,
				tt.expectedPodTerminationGracePeriodSeconds,
				tt.cfg.PodTerminationGracePeriodSeconds,
			)
			assert.EqualValues(
				t,
				tt.expectedCleanupGracePeriodSeconds,
				tt.cfg.CleanupGracePeriodSeconds,
			)
		})
	}
}

func TestConfig_SaveConfig(t *testing.T) {
	const (
		configFileName = "config-file"
	)

	oldTime := time.Now().Add(-1 * time.Hour)

	cs := NewMockConfigSaver(t)
	cs.On("Save", configFileName, mock.Anything).Return(nil).Once()

	c := new(Config)
	c.ModTime = oldTime
	c.ConfigSaver = cs

	err := c.SaveConfig(configFileName)
	require.NoError(t, err)

	assert.NotEqual(t, oldTime, c.ModTime, "Expected ModTime field of Config struct to be updated")
}

func TestConfig_Masked(t *testing.T) {
	tests := map[string]struct {
		input    *Config
		expected *Config
	}{
		"nil runner": {
			input: &Config{
				Runners: nil,
			},
			expected: &Config{
				Runners: nil,
			},
		},
		"runner token": {
			input: &Config{
				Runners: []*RunnerConfig{
					nil,
					{
						RunnerCredentials: RunnerCredentials{
							Token: "some token",
						},
					},
				},
			},
			expected: &Config{
				Runners: []*RunnerConfig{
					nil,
					{
						RunnerCredentials: RunnerCredentials{
							Token: "[MASKED]",
						},
					},
				},
			},
		},
		"kubernetes bearer token": {
			input: &Config{
				Runners: []*RunnerConfig{
					nil,
					{
						RunnerSettings: RunnerSettings{
							Kubernetes: nil,
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Kubernetes: &KubernetesConfig{
								BearerToken: "some bearer token",
							},
						},
					},
				},
			},
			expected: &Config{
				Runners: []*RunnerConfig{
					nil,
					{
						RunnerSettings: RunnerSettings{
							Kubernetes: nil,
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Kubernetes: &KubernetesConfig{
								BearerToken: "[MASKED]",
							},
						},
					},
				},
			},
		},
		"cache s3 access key": {
			input: &Config{
				Runners: []*RunnerConfig{
					nil,
					{
						RunnerSettings: RunnerSettings{
							Cache: nil,
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								S3: nil,
							},
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								S3: &cacheconfig.CacheS3Config{
									AccessKey: "some access key",
								},
							},
						},
					},
				},
			},
			expected: &Config{
				Runners: []*RunnerConfig{
					nil,
					{
						RunnerSettings: RunnerSettings{
							Cache: nil,
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								S3: nil,
							},
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								S3: &cacheconfig.CacheS3Config{
									AccessKey: "[MASKED]",
								},
							},
						},
					},
				},
			},
		},
		"cache s3 secret key": {
			input: &Config{
				Runners: []*RunnerConfig{
					nil,
					{
						RunnerSettings: RunnerSettings{
							Cache: nil,
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								S3: nil,
							},
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								S3: &cacheconfig.CacheS3Config{
									SecretKey: "some secret key",
								},
							},
						},
					},
				},
			},
			expected: &Config{
				Runners: []*RunnerConfig{
					nil,
					{
						RunnerSettings: RunnerSettings{
							Cache: nil,
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								S3: nil,
							},
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								S3: &cacheconfig.CacheS3Config{
									SecretKey: "[MASKED]",
								},
							},
						},
					},
				},
			},
		},
		"cache s3 session token": {
			input: &Config{
				Runners: []*RunnerConfig{
					nil,
					{
						RunnerSettings: RunnerSettings{
							Cache: nil,
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								S3: nil,
							},
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								S3: &cacheconfig.CacheS3Config{
									SessionToken: "some session token",
								},
							},
						},
					},
				},
			},
			expected: &Config{
				Runners: []*RunnerConfig{
					nil,
					{
						RunnerSettings: RunnerSettings{
							Cache: nil,
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								S3: nil,
							},
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								S3: &cacheconfig.CacheS3Config{
									SessionToken: "[MASKED]",
								},
							},
						},
					},
				},
			},
		},
		"cache gcs private key": {
			input: &Config{
				Runners: []*RunnerConfig{
					nil,
					{
						RunnerSettings: RunnerSettings{
							Cache: nil,
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								GCS: nil,
							},
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								GCS: &cacheconfig.CacheGCSConfig{
									CacheGCSCredentials: cacheconfig.CacheGCSCredentials{
										PrivateKey: "some private key",
									},
								},
							},
						},
					},
				},
			},
			expected: &Config{
				Runners: []*RunnerConfig{
					nil,
					{
						RunnerSettings: RunnerSettings{
							Cache: nil,
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								S3: nil,
							},
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								GCS: &cacheconfig.CacheGCSConfig{
									CacheGCSCredentials: cacheconfig.CacheGCSCredentials{
										PrivateKey: "[MASKED]",
									},
								},
							},
						},
					},
				},
			},
		},
		"cache gcs universe domain": {
			input: &Config{
				Runners: []*RunnerConfig{
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								GCS: &cacheconfig.CacheGCSConfig{
									BucketName:     "test-bucket",
									UniverseDomain: "googleapis.com",
								},
							},
						},
					},
				},
			},
			expected: &Config{
				Runners: []*RunnerConfig{
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								GCS: &cacheconfig.CacheGCSConfig{
									BucketName:     "test-bucket",
									UniverseDomain: "googleapis.com",
								},
							},
						},
					},
				},
			},
		},
		"cache azure account key": {
			input: &Config{
				Runners: []*RunnerConfig{
					nil,
					{
						RunnerSettings: RunnerSettings{
							Cache: nil,
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								S3: nil,
							},
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								Azure: &cacheconfig.CacheAzureConfig{
									CacheAzureCredentials: cacheconfig.CacheAzureCredentials{
										AccountKey: "some account key",
									},
								},
							},
						},
					},
				},
			},
			expected: &Config{
				Runners: []*RunnerConfig{
					nil,
					{
						RunnerSettings: RunnerSettings{
							Cache: nil,
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								S3: nil,
							},
						},
					},
					{
						RunnerSettings: RunnerSettings{
							Cache: &cacheconfig.Config{
								Azure: &cacheconfig.CacheAzureConfig{
									CacheAzureCredentials: cacheconfig.CacheAzureCredentials{
										AccountKey: "[MASKED]",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := tt.input.Masked()
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestConfig_GetCleanupResourcesTimeout(t *testing.T) {
	tests := map[string]struct {
		config      string
		expected    time.Duration
		expectError bool
	}{
		"negative value": {
			config: `
[[runners]]
	name = "negative value"
	executor = "kubernetes"
	[runners.kubernetes]
		cleanup_resources_timeout = "-5m"`,
			expected: KubernetesCleanupResourcesTimeout,
		},
		"zero value": {
			config: `
[[runners]]
	name = "zero value"
	executor = "kubernetes"
	[runners.kubernetes]
		cleanup_resources_timeout = "0m"`,
			expected: KubernetesCleanupResourcesTimeout,
		},
		"no value": {
			config: `
[[runners]]
	name = "no value"
	executor = "kubernetes"
	[runners.kubernetes]`,
			expected: KubernetesCleanupResourcesTimeout,
		},
		"valid value": {
			config: `
[[runners]]
	name = "valid value"
	executor = "kubernetes"
	[runners.kubernetes]
		cleanup_resources_timeout = "3m"`,
			expected: 3 * time.Minute,
		},
		"invalid value": {
			config: `
[[runners]]
	name = "invalid value"
	executor = "kubernetes"
	[runners.kubernetes]
		cleanup_resources_timeout = "nothing"`,
			expected:    KubernetesCleanupResourcesTimeout,
			expectError: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := NewConfig()
			_, e := toml.Decode(tt.config, cfg)
			if tt.expectError {
				assert.Error(t, e)
				return
			}
			assert.Equal(t, tt.expected.Seconds(), cfg.Runners[0].Kubernetes.GetCleanupResourcesTimeout().Seconds())
		})
	}
}

func Test_Docker_UserIsAllowed(t *testing.T) {
	tests := map[string]struct {
		user, runnerUser string
		allowedUsers     []string
		want             bool
	}{
		"no allowed users, neither specified":     {want: true},
		"no allowed users, runner user specified": {runnerUser: "baba", want: true},
		"no allowed users, job user specified":    {user: "baba", want: true},
		"no allowed users, both specified":        {runnerUser: "baba", user: "yaga", want: false},

		"allowed users, neither specified":     {allowedUsers: []string{"baba"}, want: true},
		"allowed users, runner user specified": {allowedUsers: []string{"baba"}, runnerUser: "yaga", want: true},
		"allowed users, job user specified":    {allowedUsers: []string{"baba"}, runnerUser: "yaga", user: "baba", want: true},
		"allowed users, both specified":        {allowedUsers: []string{"baba"}, runnerUser: "yaga", user: "yaga", want: false},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := DockerConfig{
				User:         tt.runnerUser,
				AllowedUsers: tt.allowedUsers,
			}

			assert.Equal(t, tt.want, cfg.IsUserAllowed(tt.user))
		})
	}
}

func Test_Kubernetes_GroupIsAllowed(t *testing.T) {
	tests := map[string]struct {
		group         string
		allowedGroups []string
		expectError   bool
	}{
		"no allowed groups":             {group: "1000", allowedGroups: nil, expectError: false},
		"exact match":                   {group: "1000", allowedGroups: []string{"1000"}, expectError: false},
		"exact match fails":             {group: "1000", allowedGroups: []string{"1001"}, expectError: true},
		"multiple groups":               {group: "1000", allowedGroups: []string{"1000", "1001"}, expectError: false},
		"string group exact match":      {group: "wheel", allowedGroups: []string{"wheel"}, expectError: false},
		"string group fails":            {group: "wheel", allowedGroups: []string{"docker"}, expectError: true},
		"empty group allowed":           {group: "", allowedGroups: []string{"1000"}, expectError: false},
		"root group blocked by default": {group: "0", expectError: true},
		"root group explicitly allowed": {group: "0", allowedGroups: []string{"0", "1000"}, expectError: false},
		"root group explicitly blocked": {group: "0", allowedGroups: []string{"1000", "1001"}, expectError: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := KubernetesConfig{
				AllowedGroups: tt.allowedGroups,
			}

			err := cfg.IsGroupAllowed(tt.group)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_Kubernetes_UserIsAllowed(t *testing.T) {
	tests := map[string]struct {
		user         string
		allowedUsers []string
		expectError  bool
	}{
		"empty user":                   {user: "", expectError: false},
		"no allowed users specified":   {user: "1000", expectError: false},
		"user in allowed list":         {user: "1000", allowedUsers: []string{"1000", "1001"}, expectError: false},
		"user not in allowed list":     {user: "1002", allowedUsers: []string{"1000", "1001"}, expectError: true},
		"single user allowed list":     {user: "1000", allowedUsers: []string{"1000"}, expectError: false},
		"single user not in list":      {user: "1001", allowedUsers: []string{"1000"}, expectError: true},
		"root user blocked by default": {user: "0", expectError: true},
		"root user explicitly allowed": {user: "0", allowedUsers: []string{"0", "1000"}, expectError: false},
		"root user explicitly blocked": {user: "0", allowedUsers: []string{"1000", "1001"}, expectError: true},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := KubernetesConfig{
				AllowedUsers: tt.allowedUsers,
			}

			err := cfg.IsUserAllowed(tt.user)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	tests := map[string]struct {
		config         string
		validateConfig func(t *testing.T, config *Config)
		assertError    func(t *testing.T, err error)
	}{
		"parse defaults": {
			config: ``,
			validateConfig: func(t *testing.T, config *Config) {
				require.Equal(t, 0, len(config.Runners))
				require.Equal(t, 15*time.Minute, *config.ConnectionMaxAge)
			},
		},
		"connection max age set": {
			config: `connection_max_age = "1s"`,
			validateConfig: func(t *testing.T, config *Config) {
				require.Equal(t, 0, len(config.Runners))
				require.Equal(t, 1*time.Second, *config.ConnectionMaxAge)
			},
		},
		"invalid labels": {
			config: `[labels]  # Global defaults
  "invalid/key" = "valid_value"

[[runners]]
  name = "labels-test"
  [runners.labels]  # Runner-specific data
    env = "prod"
`,
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrInvalidLabelKey)
			},
		},
		"valid labels": {
			config: `
concurrent = 1

[labels]
  "env" = "prod"
  test = "value"
  test_label = "value"
  test-label = "value"
  "test.label" = "value"

[[runners]]
  name = "labels-test"

  [runners.labels]
    "shard" = "default"
    test = "override"
    test_label = "override"
    "test-label" = "override"
    "test.label" = "override"
`,
			validateConfig: func(t *testing.T, config *Config) {
				globalLabels := Labels{
					"env":        "prod",
					"test":       "value",
					"test_label": "value",
					"test.label": "value",
					"test-label": "value",
				}

				runnerLabels := Labels{
					"shard":      "default",
					"test":       "override",
					"test_label": "override",
					"test.label": "override",
					"test-label": "override",
				}

				computedLabels := Labels{
					"env":        "prod",
					"shard":      "default",
					"test":       "override",
					"test_label": "override",
					"test.label": "override",
					"test-label": "override",
				}

				assert.Equal(t, globalLabels, config.Labels)
				if assert.GreaterOrEqual(t, len(config.Runners), 1) {
					assert.Equal(t, runnerLabels, config.Runners[0].Labels)
					assert.Equal(t, computedLabels, config.Runners[0].ComputedLabels())
				}
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			tempFile, err := os.CreateTemp(t.TempDir(), "test_config")
			require.NoError(t, err)
			defer tempFile.Close()

			_, err = tempFile.WriteString(tt.config)
			require.NoError(t, err)

			cfg := NewConfig()
			err = cfg.LoadConfig(tempFile.Name())

			if tt.assertError != nil {
				tt.assertError(t, err)
				return
			}

			assert.NoError(t, err)

			if tt.validateConfig != nil {
				tt.validateConfig(t, cfg)
			}
		})
	}
}

func Test_CommandLineFlags(t *testing.T) {
	tests := map[string]struct {
		args          []string
		expectedError bool
		verifyArgs    func(t *testing.T, config *RunnerConfig)
	}{
		"Kubernetes host aliases": {
			args: []string{
				"--request-concurrency",
				"10",
				"--kubernetes-host_aliases",
				`[{"ip":"192.168.1.100","hostnames":["myservice.local"]},{"ip":"192.168.1.101","hostnames":["otherservice.local"]}]`,
			},
			verifyArgs: func(t *testing.T, config *RunnerConfig) {
				assert.Equal(t, 10, config.RequestConcurrency)
				assert.Len(t, config.Kubernetes.HostAliases, 2)
				assert.Equal(t, "192.168.1.100", config.Kubernetes.HostAliases[0].IP)
				assert.Len(t, config.Kubernetes.HostAliases[0].Hostnames, 1)
				assert.Equal(t, "myservice.local", config.Kubernetes.HostAliases[0].Hostnames[0])
				assert.Len(t, config.Kubernetes.HostAliases[1].Hostnames, 1)
				assert.Equal(t, "otherservice.local", config.Kubernetes.HostAliases[1].Hostnames[0])
			},
		},
		"Bad Kubernetes host aliases": {
			args: []string{
				"--kubernetes-host_aliases",
				"{ bad",
			},
			expectedError: true,
		},
	}

	// Loop across tests
	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			config := &RunnerConfig{}
			flags := clihelpers.GetFlagsFromStruct(config)
			flagSet := flag.NewFlagSet("test-flags", flag.ContinueOnError)
			for _, f := range flags {
				f.Apply(flagSet)
			}

			err := flagSet.Parse(tt.args)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.verifyArgs != nil {
					tt.verifyArgs(t, config)
				}
			}
		})
	}
}

func TestConfig_SaveConfig_CustomBuildDir(t *testing.T) {
	tests := map[string]struct {
		customBuildDir    CustomBuildDir
		expectedTomlRE    string
		notExpectedTomlRE string
	}{
		"not explicitly set": {
			customBuildDir:    CustomBuildDir{},
			notExpectedTomlRE: "custom_build_dir",
		},
		"explicitly enabled": {
			customBuildDir: CustomBuildDir{Enabled: ptr(true)},
			expectedTomlRE: `(?m)\[runners\.custom_build_dir\]\n\s+enabled = true\n`,
		},
		"explicitly disabled": {
			customBuildDir: CustomBuildDir{Enabled: ptr(false)},
			expectedTomlRE: `(?m)\[runners\.custom_build_dir\]\n\s+enabled = false\n`,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			cs := NewMockConfigSaver(t)
			cs.On("Save", "", mock.MatchedBy(func(b []byte) bool {
				tomlBlob := string(b)
				if e := test.expectedTomlRE; e != "" {
					assert.Regexp(t, e, tomlBlob)
				}
				if ne := test.notExpectedTomlRE; ne != "" {
					assert.NotRegexp(t, ne, tomlBlob)
				}
				return true
			})).Return(nil).Once()

			c := &Config{
				ConfigSaver: cs,
				Runners: []*RunnerConfig{
					{
						Name: name,
						RunnerSettings: RunnerSettings{
							CustomBuildDir: test.customBuildDir,
						},
					},
				},
			}

			err := c.SaveConfig("")
			require.NoError(t, err)
		})
	}
}

func ptr[T any](v T) *T {
	return &v
}

func TestRunnerByName(t *testing.T) {
	examples := map[string]struct {
		runners       []*RunnerConfig
		runnerName    string
		expectedIndex int
		expectedError error
	}{
		"finds runner by name": {
			runners: []*RunnerConfig{
				{
					Name: "runner1",
				},
				{
					Name: "runner2",
				},
			},
			runnerName:    "runner2",
			expectedIndex: 1,
		},
		"does not find non-existent runner": {
			runners: []*RunnerConfig{
				{
					Name: "runner1",
				},
				{
					Name: "runner2",
				},
			},
			runnerName:    "runner3",
			expectedIndex: -1,
			expectedError: fmt.Errorf("could not find a runner with the name 'runner3'"),
		},
	}

	for tn, tt := range examples {
		t.Run(tn, func(t *testing.T) {
			config := &Config{
				Runners: tt.runners,
			}

			runner, err := config.RunnerByName(tt.runnerName)
			if tt.expectedIndex == -1 {
				assert.Nil(t, runner)
			} else {
				assert.Equal(t, tt.runners[tt.expectedIndex], runner)
			}
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestRunnerByToken(t *testing.T) {
	examples := map[string]struct {
		runners       []*RunnerConfig
		runnerToken   string
		expectedIndex int
		expectedError error
	}{
		"finds runner by token": {
			runners: []*RunnerConfig{
				{
					RunnerCredentials: RunnerCredentials{
						Token: "runner1",
					},
				},
				{
					RunnerCredentials: RunnerCredentials{
						Token: "runner2",
					},
				},
			},
			runnerToken:   "runner2",
			expectedIndex: 1,
		},
		"does not find non-existent runner authentication token": {
			runners: []*RunnerConfig{
				{
					RunnerCredentials: RunnerCredentials{
						Token: "runner1",
					},
				},
				{
					RunnerCredentials: RunnerCredentials{
						Token: "runner2",
					},
				},
			},
			runnerToken:   "runner3",
			expectedIndex: -1,
			expectedError: fmt.Errorf("could not find a runner with the token 'runner3'"),
		},
	}

	for tn, tt := range examples {
		t.Run(tn, func(t *testing.T) {
			config := &Config{
				Runners: tt.runners,
			}

			runner, err := config.RunnerByToken(tt.runnerToken)
			if tt.expectedIndex == -1 {
				assert.Nil(t, runner)
			} else {
				assert.Equal(t, tt.runners[tt.expectedIndex], runner)
			}
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestRunnerByURLAndID(t *testing.T) {
	examples := map[string]struct {
		runners       []*RunnerConfig
		runnerURL     string
		runnerID      int64
		expectedIndex int
		expectedError error
	}{
		"finds runner by name": {
			runners: []*RunnerConfig{
				{
					RunnerCredentials: RunnerCredentials{
						ID:  1,
						URL: "https://gitlab1.example.com/",
					},
				},
				{
					RunnerCredentials: RunnerCredentials{
						ID:  2,
						URL: "https://gitlab1.example.com/",
					},
				},
			},
			runnerURL:     "https://gitlab1.example.com/",
			runnerID:      1,
			expectedIndex: 0,
		},
		"does not find runner with wrong ID": {
			runners: []*RunnerConfig{
				{
					RunnerCredentials: RunnerCredentials{
						ID:  1,
						URL: "https://gitlab1.example.com/",
					},
				},
				{
					RunnerCredentials: RunnerCredentials{
						ID:  2,
						URL: "https://gitlab1.example.com/",
					},
				},
			},
			runnerURL:     "https://gitlab1.example.com/",
			runnerID:      3,
			expectedIndex: -1,
			expectedError: fmt.Errorf(`could not find a runner with the URL "https://gitlab1.example.com/" and ID 3`),
		},
		"does not find runner with wrong URL": {
			runners: []*RunnerConfig{
				{
					RunnerCredentials: RunnerCredentials{
						ID:  1,
						URL: "https://gitlab1.example.com/",
					},
				},
				{
					RunnerCredentials: RunnerCredentials{
						ID:  2,
						URL: "https://gitlab1.example.com/",
					},
				},
			},
			runnerURL:     "https://gitlab2.example.com/",
			runnerID:      1,
			expectedIndex: -1,
			expectedError: fmt.Errorf(`could not find a runner with the URL "https://gitlab2.example.com/" and ID 1`),
		},
	}

	for tn, tt := range examples {
		t.Run(tn, func(t *testing.T) {
			config := &Config{
				Runners: tt.runners,
			}

			runner, err := config.RunnerByURLAndID(tt.runnerURL, tt.runnerID)
			if tt.expectedIndex == -1 {
				assert.Nil(t, runner)
			} else {
				assert.Equal(t, tt.runners[tt.expectedIndex], runner)
			}
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestRunnerByNameAndToken(t *testing.T) {
	examples := map[string]struct {
		runners       []*RunnerConfig
		runnerName    string
		runnerToken   string
		expectedIndex int
		expectedError error
	}{
		"finds runner by name and token": {
			runners: []*RunnerConfig{
				{
					Name: "runner1",
					RunnerCredentials: RunnerCredentials{
						Token: "token1",
					},
				},
				{
					Name: "runner2",
					RunnerCredentials: RunnerCredentials{
						Token: "token2",
					},
				},
			},
			runnerName:    "runner1",
			runnerToken:   "token1",
			expectedIndex: 0,
		},
		"does not find runner with wrong name": {
			runners: []*RunnerConfig{
				{
					Name: "runner1",
					RunnerCredentials: RunnerCredentials{
						Token: "token1",
					},
				},
				{
					Name: "runner2",
					RunnerCredentials: RunnerCredentials{
						Token: "token2",
					},
				},
			},
			runnerName:    "runner3",
			runnerToken:   "token1",
			expectedIndex: -1,
			expectedError: fmt.Errorf(`could not find a runner with the Name 'runner3' and Token 'token1'`),
		},
		"does not find runner with wrong token": {
			runners: []*RunnerConfig{
				{
					Name: "runner1",
					RunnerCredentials: RunnerCredentials{
						Token: "token1",
					},
				},
				{
					Name: "runner2",
					RunnerCredentials: RunnerCredentials{
						Token: "token2",
					},
				},
			},
			runnerName:    "runner1",
			runnerToken:   "token3",
			expectedIndex: -1,
			expectedError: fmt.Errorf(`could not find a runner with the Name 'runner1' and Token 'token3'`),
		},
	}

	for tn, tt := range examples {
		t.Run(tn, func(t *testing.T) {
			config := &Config{
				Runners: tt.runners,
			}

			runner, err := config.RunnerByNameAndToken(tt.runnerName, tt.runnerToken)
			if tt.expectedIndex == -1 {
				assert.Nil(t, runner)
			} else {
				assert.Equal(t, tt.runners[tt.expectedIndex], runner)
			}
			assert.Equal(t, tt.expectedError, err)
		})
	}
}

func TestRunnerSettings_ComputeLabels(t *testing.T) {
	tests := map[string]struct {
		runnerWorkerLabels        Labels
		initialRunnerWorkerLabels Labels
		globalDefaults            Labels
		expectedResult            Labels
	}{
		"nil labels and nil computed with empty global defaults": {
			runnerWorkerLabels: nil,
			globalDefaults:     Labels{},
			expectedResult:     Labels{},
		},
		"nil labels and nil computed with global defaults": {
			runnerWorkerLabels: nil,
			globalDefaults:     Labels{"env": "prod", "team": "backend"},
			expectedResult:     Labels{"env": "prod", "team": "backend"},
		},
		"empty labels with global defaults": {
			runnerWorkerLabels: Labels{},
			globalDefaults:     Labels{"env": "prod", "team": "backend"},
			expectedResult:     Labels{"env": "prod", "team": "backend"},
		},
		"runner labels override global defaults": {
			runnerWorkerLabels: Labels{"env": "staging", "region": "us-west"},
			globalDefaults:     Labels{"env": "prod", "team": "backend"},
			expectedResult:     Labels{"env": "staging", "team": "backend", "region": "us-west"},
		},
		"runner labels only, no global defaults": {
			runnerWorkerLabels: Labels{"custom": "value", "runner": "specific"},
			expectedResult:     Labels{"custom": "value", "runner": "specific"},
		},
		"existing computed labels are overwritten": {
			runnerWorkerLabels:        Labels{"env": "staging"},
			initialRunnerWorkerLabels: Labels{"old": "value", "env": "dev"},
			globalDefaults:            Labels{"team": "backend"},
			expectedResult:            Labels{"env": "staging", "team": "backend"},
		},
		"nil global defaults with existing labels": {
			runnerWorkerLabels: Labels{"runner": "test"},
			globalDefaults:     nil,
			expectedResult:     Labels{"runner": "test"},
		},
		"complex scenario with multiple overrides": {
			runnerWorkerLabels: Labels{"env": "staging", "version": "1.2.3", "team": "frontend"},
			globalDefaults:     Labels{"env": "prod", "team": "backend", "region": "us-east", "cost-center": "eng"},
			expectedResult:     Labels{"env": "staging", "version": "1.2.3", "team": "frontend", "region": "us-east", "cost-center": "eng"},
		},
		"empty string values in labels": {
			runnerWorkerLabels: Labels{"empty": "", "normal": "value"},
			globalDefaults:     Labels{"global": "default", "empty": "global-value"},
			expectedResult:     Labels{"global": "default", "empty": "", "normal": "value"},
		},
		"labels with special characters in key": {
			runnerWorkerLabels: Labels{"key-with-dashes": "value1", "key_with_underscores": "value2", "key.with.dots": "value3"},
			globalDefaults:     Labels{"key-with_different.characters": "value4"},
			expectedResult:     Labels{"key-with-dashes": "value1", "key_with_underscores": "value2", "key.with.dots": "value3", "key-with_different.characters": "value4"},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			r := &RunnerSettings{
				Labels: tt.runnerWorkerLabels,
				labels: tt.initialRunnerWorkerLabels,
			}

			r.ComputeLabels(tt.globalDefaults)

			assert.Equal(t, tt.runnerWorkerLabels, r.Labels)
			assert.Equal(t, tt.expectedResult, r.labels, "computed labels should match expected result")
		})
	}
}

func TestRunnerSettings_ComputedLabels(t *testing.T) {
	tests := map[string]struct {
		computedLabels Labels
		expected       Labels
	}{
		"nil computed labels": {
			computedLabels: nil,
			expected:       nil,
		},
		"empty computed labels": {
			computedLabels: Labels{},
			expected:       Labels{},
		},
		"single label": {
			computedLabels: Labels{"env": "prod"},
			expected:       Labels{"env": "prod"},
		},
		"multiple labels": {
			computedLabels: Labels{"env": "prod", "team": "backend", "region": "us-west"},
			expected:       Labels{"env": "prod", "team": "backend", "region": "us-west"},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			r := &RunnerSettings{
				labels: tt.computedLabels,
			}

			assert.Equal(t, tt.expected, r.ComputedLabels(), "ComputedLabels should return the labels field")
		})
	}
}

func TestRunnerSettings_CombineLabels_MultipleCalls(t *testing.T) {
	t.Run("multiple calls to ComputeLabels", func(t *testing.T) {
		r := &RunnerSettings{
			Labels: Labels{"runner": "test"},
		}

		// First call
		r.ComputeLabels(Labels{"env": "prod", "team": "backend"})
		expected1 := Labels{"env": "prod", "team": "backend", "runner": "test"}
		assert.Equal(t, expected1, r.ComputedLabels())

		// Second call with different global defaults
		r.ComputeLabels(Labels{"env": "staging", "region": "us-east"})
		expected2 := Labels{"env": "staging", "region": "us-east", "runner": "test"}
		assert.Equal(t, expected2, r.ComputedLabels())
	})
}

func TestConfig_Validate(t *testing.T) {
	tests := map[string]struct {
		globalLabels Labels
		runnerLabels Labels
		assertError  func(t *testing.T, err error)
	}{
		"all labels are valid": {
			globalLabels: Labels{
				"env": "production",
			},
			runnerLabels: Labels{
				"privileged": "true",
			},
		},
		"invalid global label key": {
			globalLabels: Labels{
				"test/key": "test_value",
			},
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrInvalidLabelKey)
				assert.Contains(t, err.Error(), "lobal labels")
			},
		},
		"invalid global label value": {
			globalLabels: Labels{
				"test_key": "test/value",
			},
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrInvalidLabelValue)
				assert.Contains(t, err.Error(), "lobal labels")
			},
		},
		"invalid runner label key": {
			runnerLabels: Labels{
				"test/key": "test_value",
			},
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrInvalidLabelKey)
				assert.Contains(t, err.Error(), "runner-tested")
			},
		},
		"invalid runner label value": {
			runnerLabels: Labels{
				"test_key": "test/value",
			},
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrInvalidLabelValue)
				assert.Contains(t, err.Error(), "runner-tested")
			},
		},
		"too many labels": {
			globalLabels: Labels{
				"one":       "1",
				"two":       "2",
				"three":     "3",
				"four":      "4",
				"five":      "5",
				"six":       "6",
				"seven":     "7",
				"eight":     "8",
				"nine":      "9",
				"ten":       "10",
				"eleven":    "11",
				"twelve":    "12",
				"thirteen":  "13",
				"fourteen":  "14",
				"fifteen":   "15",
				"sixteen":   "16",
				"seventeen": "17",
			},
			runnerLabels: Labels{
				"eighteen":     "18",
				"nineteen":     "19",
				"twenty":       "20",
				"twenty-one":   "21",
				"twenty-two":   "22",
				"twenty-three": "23",
				"twenty-four":  "24",
				"twenty-five":  "25",
				"twenty-six":   "26",
				"twenty-seven": "27",
				"twenty-eight": "28",
				"twenty-nine":  "29",
				"thirty":       "30",
				"thirty-one":   "31",
				"thirty-two":   "32",
				"thirty-three": "33",
				"thirty-four":  "34",
			},
			assertError: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, ErrLabelsCountExceeded)
				assert.Contains(t, err.Error(), "runner-tested")
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			c := &Config{
				Labels: tc.globalLabels,
				Runners: []*RunnerConfig{
					{
						Name: "runner-always-valid",
						RunnerSettings: RunnerSettings{
							Labels: Labels{
								"runner": "name",
							},
						},
					},
					{
						Name: "runner-tested",
						RunnerSettings: RunnerSettings{
							Labels: tc.runnerLabels,
						},
					},
				},
			}

			for _, r := range c.Runners {
				r.ComputeLabels(c.Labels)
			}

			assert.NoError(t, c.Runners[0].Validate())

			err := c.Validate()
			if tc.assertError == nil {
				assert.NoError(t, err)
				return
			}

			tc.assertError(t, err)
		})
	}
}

func TestRunnerConfig_ValidateMachineOptionsWithName(t *testing.T) {
	tests := map[string]struct {
		options      []string
		expectError  bool
		errorMessage string
	}{
		"valid options with %s": {
			options:     []string{"--option=%s", "--another=%s-suffix"},
			expectError: false,
		},
		"empty options": {
			options:     []string{},
			expectError: false,
		},
		"nil options": {
			options:     nil,
			expectError: false,
		},
		"nil machine config": {
			options:     nil,
			expectError: false,
		},
		"invalid option without %s": {
			options:      []string{"--option=value"},
			expectError:  true,
			errorMessage: `machine option with name "--option=value" must contain %s placeholder`,
		},
		"mixed valid and invalid": {
			options:      []string{"--valid=%s", "--invalid=value"},
			expectError:  true,
			errorMessage: `machine option with name "--invalid=value" must contain %s placeholder`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			config := &RunnerConfig{
				RunnerSettings: RunnerSettings{
					Machine: &DockerMachine{
						MachineOptionsWithName: tc.options,
					},
				},
			}

			err := config.Validate()

			if tc.expectError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.errorMessage)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseVariable(t *testing.T) {
	v, err := parseVariable("key=value=value2")
	assert.NoError(t, err)
	assert.Equal(t, spec.Variable{Key: "key", Value: "value=value2"}, v)
}

func TestInvalidParseVariable(t *testing.T) {
	_, err := parseVariable("some_other_key")
	assert.Error(t, err)
}

func TestRunnerCredentials_SameAs(t *testing.T) {
	tests := map[string]struct {
		c      *RunnerCredentials
		other  *RunnerCredentials
		result bool
	}{
		"same token and same URL": {
			c: &RunnerCredentials{
				URL:   "https://gitlab.com",
				Token: "token123",
			},
			other: &RunnerCredentials{
				URL:   "https://gitlab.com",
				Token: "token123",
			},
			result: true,
		},
		"same token but different URL": {
			c: &RunnerCredentials{
				URL:   "https://gitlab.com",
				Token: "token123",
			},
			other: &RunnerCredentials{
				URL:   "https://gitlab.example.com",
				Token: "token123",
			},
			result: false,
		},
		"different token but same URL": {
			c: &RunnerCredentials{
				URL:   "https://gitlab.com",
				Token: "token123",
			},
			other: &RunnerCredentials{
				URL:   "https://gitlab.com",
				Token: "token456",
			},
			result: false,
		},
		"different token and different URL": {
			c: &RunnerCredentials{
				URL:   "https://gitlab.com",
				Token: "token123",
			},
			other: &RunnerCredentials{
				URL:   "https://gitlab.example.com",
				Token: "token456",
			},
			result: false,
		},
		"same token, first URL is wildcard *": {
			c: &RunnerCredentials{
				URL:   "*",
				Token: "token123",
			},
			other: &RunnerCredentials{
				URL:   "https://gitlab.com",
				Token: "token123",
			},
			result: true,
		},
		"same token, second URL is wildcard *": {
			c: &RunnerCredentials{
				URL:   "https://gitlab.com",
				Token: "token123",
			},
			other: &RunnerCredentials{
				URL:   "*",
				Token: "token123",
			},
			result: true,
		},
		"same token, both URLs are wildcard *": {
			c: &RunnerCredentials{
				URL:   "*",
				Token: "token123",
			},
			other: &RunnerCredentials{
				URL:   "*",
				Token: "token123",
			},
			result: true,
		},
		"same token, first URL is empty": {
			c: &RunnerCredentials{
				URL:   "",
				Token: "token123",
			},
			other: &RunnerCredentials{
				URL:   "https://gitlab.com",
				Token: "token123",
			},
			result: true,
		},
		"same token, second URL is empty": {
			c: &RunnerCredentials{
				URL:   "https://gitlab.com",
				Token: "token123",
			},
			other: &RunnerCredentials{
				URL:   "",
				Token: "token123",
			},
			result: true,
		},
		"same token, both URLs are empty": {
			c: &RunnerCredentials{
				URL:   "",
				Token: "token123",
			},
			other: &RunnerCredentials{
				URL:   "",
				Token: "token123",
			},
			result: true,
		},
		"same token, empty and wildcard *": {
			c: &RunnerCredentials{
				URL:   "",
				Token: "token123",
			},
			other: &RunnerCredentials{
				URL:   "*",
				Token: "token123",
			},
			result: true,
		},
		"different token, first URL is wildcard *": {
			c: &RunnerCredentials{
				URL:   "*",
				Token: "token123",
			},
			other: &RunnerCredentials{
				URL:   "https://gitlab.com",
				Token: "token456",
			},
			result: false,
		},
		"different token, second URL is wildcard *": {
			c: &RunnerCredentials{
				URL:   "https://gitlab.com",
				Token: "token123",
			},
			other: &RunnerCredentials{
				URL:   "*",
				Token: "token456",
			},
			result: false,
		},
		"same token, URLs differ only by trailing slash": {
			c: &RunnerCredentials{
				URL:   "https://gitlab.com",
				Token: "token123",
			},
			other: &RunnerCredentials{
				URL:   "https://gitlab.com/",
				Token: "token123",
			},
			result: false,
		},
		"same token, URLs differ by protocol": {
			c: &RunnerCredentials{
				URL:   "http://gitlab.com",
				Token: "token123",
			},
			other: &RunnerCredentials{
				URL:   "https://gitlab.com",
				Token: "token123",
			},
			result: false,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			result := tt.c.SameAs(tt.other)
			assert.Equal(t, tt.result, result, "SameAs should return %v for this case", tt.result)
		})
	}
}
