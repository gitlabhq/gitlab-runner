//go:build !integration

package configfile

import (
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

// A nil-able field that carries a toml tag but no json tag marshals to an
// explicit null while the reflected schema types it concretely, so validation
// fails on configs that are perfectly valid. Each case leaves one such field
// unset. See https://gitlab.com/gitlab-org/gitlab-runner/-/issues/39688.
func TestValidate_UnsetOptionalFields(t *testing.T) {
	tests := map[string]func(*common.RunnerConfig){
		"empty_dir without mount_propagation": func(r *common.RunnerConfig) {
			r.Kubernetes.Volumes.EmptyDirs = []common.KubernetesEmptyDir{{
				Name:      "docker-certs",
				MountPath: "/certs/client",
			}}
		},
		"host_path without mount_propagation": func(r *common.RunnerConfig) {
			r.Kubernetes.Volumes.HostPaths = []common.KubernetesHostPath{{
				Name:      "docker-sock",
				MountPath: "/var/run/docker.sock",
			}}
		},
		"pvc without mount_propagation": func(r *common.RunnerConfig) {
			r.Kubernetes.Volumes.PVCs = []common.KubernetesPVC{{
				Name:      "cache",
				MountPath: "/cache",
			}}
		},
		"dns_config option without value": func(r *common.RunnerConfig) {
			r.Kubernetes.DNSConfig.Options = []common.KubernetesDNSConfigOption{{
				Name: "ndots",
			}}
		},
		"autoscaler policy without preemptive_mode": func(r *common.RunnerConfig) {
			r.Autoscaler = &common.AutoscalerConfig{
				Policy: []common.AutoscalerPolicyConfig{{IdleCount: 1}},
			}
		},
		"autoscaler without plugin_config": func(r *common.RunnerConfig) {
			r.Autoscaler = &common.AutoscalerConfig{Plugin: "fleeting-plugin-aws"}
		},
	}

	for name, setup := range tests {
		t.Run(name, func(t *testing.T) {
			runner := &common.RunnerConfig{
				RunnerCredentials: common.RunnerCredentials{
					URL:   "https://gitlab.example.com/",
					Token: "glrt-token",
				},
				RunnerSettings: common.RunnerSettings{
					Executor:   "kubernetes",
					Kubernetes: &common.KubernetesConfig{},
				},
			}
			setup(runner)

			require.NoError(t, Validate(&common.Config{Runners: []*common.RunnerConfig{runner}}))
		})
	}
}
