//go:build !integration

package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/jpillora/backoff"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	api "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	testclient "k8s.io/client-go/kubernetes/fake"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/rest/fake"
	k8stesting "k8s.io/client-go/testing"
	"k8s.io/client-go/util/exec"

	"gitlab.com/gitlab-org/gitlab-runner/helpers/retry"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildlogger"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/common/spec"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes/internal/pull"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes/internal/watchers"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/helperimage"
	dns_test "gitlab.com/gitlab-org/gitlab-runner/helpers/dns/test"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	service_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/service"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/trace"
	"gitlab.com/gitlab-org/gitlab-runner/session/proxy"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

type featureFlagTest func(t *testing.T, flagName string, flagValue bool)

func mustCreateResourceList(t *testing.T, cpu, memory, ephemeralStorage string) api.ResourceList {
	resources, err := createResourceList(cpu, memory, ephemeralStorage)
	require.NoError(t, err)

	return resources
}

func containsLabels(t *testing.T, actual, expected map[string]string) {
	for key, expectedValue := range expected {
		actualValue, exists := actual[key]
		if assert.True(t, exists, "Key %q is missing!", key) {
			assert.Equal(t, expectedValue, actualValue, "Value for key %q does not match expected value!", key)
		}
	}
}

func notContainsLabels(t *testing.T, actual, unexpected map[string]string) {
	for key := range unexpected {
		_, exists := actual[key]
		assert.False(t, exists, "Key %q is present when it should not be!", key)
	}
}

func TestRunTestsWithFeatureFlag(t *testing.T) {
	tests := map[string]featureFlagTest{
		"testVolumeMounts":                      testVolumeMountsFeatureFlag,
		"testVolumes":                           testVolumesFeatureFlag,
		"testSetupBuildPodServiceCreationError": testSetupBuildPodServiceCreationErrorFeatureFlag,
		"testSetupBuildPodFailureGetPullPolicy": testSetupBuildPodFailureGetPullPolicyFeatureFlag,
		"testGetPodActiveDeadlineSeconds":       testGetPodActiveDeadlineSecondsFeatureFlag,
	}

	featureFlags := []string{
		featureflags.UseLegacyKubernetesExecutionStrategy,
	}

	for tn, tt := range tests {
		for _, ff := range featureFlags {
			t.Run(fmt.Sprintf("%s %s true", tn, ff), func(t *testing.T) {
				tt(t, ff, true)
			})

			t.Run(fmt.Sprintf("%s %s false", tn, ff), func(t *testing.T) {
				tt(t, ff, false)
			})
		}
	}
}

func testVolumeMountsFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	addScriptsAndLogVolumeMounts := func(featureFlagValue bool, volumeMounts []api.VolumeMount) []api.VolumeMount {
		if featureFlagValue {
			return volumeMounts
		}

		vm := []api.VolumeMount{
			{Name: "scripts", MountPath: "/scripts-0-0"},
			{Name: "logs", MountPath: "/logs-0-0"},
		}

		return append(
			vm,
			volumeMounts...,
		)
	}

	tests := map[string]struct {
		GlobalConfig *common.Config
		RunnerConfig common.RunnerConfig
		Build        *common.Build

		Expected func(featureFlagValue bool) []api.VolumeMount
	}{
		"no custom volumes": {
			GlobalConfig: &common.Config{},
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: func(featureFlagValue bool) []api.VolumeMount {
				volumeMounts := []api.VolumeMount{
					{Name: "repo", MountPath: "/builds"},
				}

				return addScriptsAndLogVolumeMounts(featureFlagValue, volumeMounts)
			},
		},
		"custom volumes": {
			GlobalConfig: &common.Config{},
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Volumes: common.KubernetesVolumes{
							HostPaths: []common.KubernetesHostPath{
								{Name: "docker", MountPath: "/var/run/docker.sock", HostPath: "/var/run/docker.sock"},
								{Name: "host-path", MountPath: "/path/two", HostPath: "/path/one"},
								{
									Name:      "host-subpath",
									MountPath: "/subpath",
									HostPath:  "/path/one",
									SubPath:   "subpath",
								},
							},
							Secrets: []common.KubernetesSecret{
								{Name: "Secret", MountPath: "/path/to/whatever"},
								{
									Name:      "Secret-subpath",
									MountPath: "/path/to/whatever",
									SubPath:   "secret-subpath",
								},
							},
							PVCs: []common.KubernetesPVC{
								{Name: "PVC", MountPath: "/path/to/whatever"},
								{
									Name:      "PVC-subpath",
									MountPath: "/path/to/whatever/1",
									SubPath:   "PVC-subpath-1",
								},
								{
									Name:      "PVC-subpath",
									MountPath: "/path/to/whatever/2",
									SubPath:   "PVC-subpath-2",
								},
							},
							ConfigMaps: []common.KubernetesConfigMap{
								{Name: "ConfigMap", MountPath: "/path/to/whatever"},
								{
									Name:      "ConfigMap-subpath",
									MountPath: "/path/to/whatever",
									SubPath:   "ConfigMap-subpath",
								},
							},
							EmptyDirs: []common.KubernetesEmptyDir{
								{Name: "emptyDir", MountPath: "/path/to/empty/dir"},
								{
									Name:      "emptyDir-subpath",
									MountPath: "/subpath",
									SubPath:   "empty-subpath",
								},
							},
							CSIs: []common.KubernetesCSI{
								{Name: "csi", MountPath: "/path/to/csi/volume", Driver: "some-driver"},
								{
									Name:      "csi-subpath",
									MountPath: "/path/to/csi/volume",
									Driver:    "some-driver",
									SubPath:   "subpath",
								},
							},
						},
					},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: func(featureFlagValue bool) []api.VolumeMount {
				volumeMounts := []api.VolumeMount{
					{Name: "docker", MountPath: "/var/run/docker.sock"},
					{Name: "host-path", MountPath: "/path/two"},
					{Name: "host-subpath", MountPath: "/subpath", SubPath: "subpath"},
					{Name: "Secret", MountPath: "/path/to/whatever"},
					{Name: "Secret-subpath", MountPath: "/path/to/whatever", SubPath: "secret-subpath"},
					{Name: "PVC", MountPath: "/path/to/whatever"},
					{Name: "PVC-subpath", MountPath: "/path/to/whatever/1", SubPath: "PVC-subpath-1"},
					{Name: "PVC-subpath", MountPath: "/path/to/whatever/2", SubPath: "PVC-subpath-2"},
					{Name: "ConfigMap", MountPath: "/path/to/whatever"},
					{Name: "ConfigMap-subpath", MountPath: "/path/to/whatever", SubPath: "ConfigMap-subpath"},
					{Name: "emptyDir", MountPath: "/path/to/empty/dir"},
					{Name: "emptyDir-subpath", MountPath: "/subpath", SubPath: "empty-subpath"},
					{Name: "csi", MountPath: "/path/to/csi/volume"},
					{Name: "csi-subpath", MountPath: "/path/to/csi/volume", SubPath: "subpath"},
					{Name: "repo", MountPath: "/builds"},
				}

				return addScriptsAndLogVolumeMounts(featureFlagValue, volumeMounts)
			},
		},
		"custom volumes with read-only settings": {
			GlobalConfig: &common.Config{},
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Volumes: common.KubernetesVolumes{
							HostPaths: []common.KubernetesHostPath{
								{
									Name:      "test",
									MountPath: "/opt/test/readonly",
									ReadOnly:  true,
									HostPath:  "/opt/test/rw",
								},
								{Name: "docker", MountPath: "/var/run/docker.sock"},
							},
							ConfigMaps: []common.KubernetesConfigMap{
								{Name: "configMap", MountPath: "/path/to/configmap", ReadOnly: true},
							},
							Secrets: []common.KubernetesSecret{
								{Name: "secret", MountPath: "/path/to/secret", ReadOnly: true},
							},
							CSIs: []common.KubernetesCSI{
								{Name: "csi", MountPath: "/path/to/csi/volume", Driver: "some-driver", ReadOnly: true},
							},
						},
					},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: func(featureFlagValue bool) []api.VolumeMount {
				volumeMounts := []api.VolumeMount{
					{Name: "test", MountPath: "/opt/test/readonly", ReadOnly: true},
					{Name: "docker", MountPath: "/var/run/docker.sock"},
					{Name: "secret", MountPath: "/path/to/secret", ReadOnly: true},
					{Name: "configMap", MountPath: "/path/to/configmap", ReadOnly: true},
					{Name: "csi", MountPath: "/path/to/csi/volume", ReadOnly: true},
					{Name: "repo", MountPath: "/builds"},
				}

				return addScriptsAndLogVolumeMounts(featureFlagValue, volumeMounts)
			},
		},
		"default volume with build dir": {
			GlobalConfig: &common.Config{},
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					BuildsDir: "/path/to/builds/dir",
					Kubernetes: &common.KubernetesConfig{
						Volumes: common.KubernetesVolumes{},
					},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: func(featureFlagValue bool) []api.VolumeMount {
				volumeMounts := []api.VolumeMount{
					{
						Name:      "repo",
						MountPath: "/path/to/builds/dir",
					},
				}

				return addScriptsAndLogVolumeMounts(featureFlagValue, volumeMounts)
			},
		},
		"user-provided volume with build dir": {
			GlobalConfig: &common.Config{},
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					BuildsDir: "/path/to/builds/dir",
					Kubernetes: &common.KubernetesConfig{
						Volumes: common.KubernetesVolumes{
							HostPaths: []common.KubernetesHostPath{
								{Name: "user-provided", MountPath: "/path/to/builds/dir"},
							},
						},
					},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: func(featureFlagValue bool) []api.VolumeMount {
				volumeMounts := []api.VolumeMount{
					{Name: "user-provided", MountPath: "/path/to/builds/dir"},
				}

				return addScriptsAndLogVolumeMounts(featureFlagValue, volumeMounts)
			},
		},
		"volumes with variables inside mountPath and subPath": {
			GlobalConfig: &common.Config{},
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Volumes: common.KubernetesVolumes{
							HostPaths: []common.KubernetesHostPath{
								{Name: "docker", MountPath: "${DOCKER_SOCKET}", HostPath: "/var/run/docker.sock"},
								{Name: "host-path", MountPath: "${PATH_TWO}", HostPath: "/path/one"},
								{
									Name:      "host-subpath",
									MountPath: "/${SUB_PATH}",
									HostPath:  "/path/one",
									SubPath:   "${SUB_PATH}",
								},
							},
							Secrets: []common.KubernetesSecret{
								{Name: "Secret", MountPath: "/${PATH_TO_WHATEVER}"},
								{
									Name:      "Secret-subpath",
									MountPath: "/${PATH_TO_WHATEVER}",
									SubPath:   "secret-subpath",
								},
							},
							PVCs: []common.KubernetesPVC{
								{Name: "PVC", MountPath: "/${PATH_TO_WHATEVER}"},
								{
									Name:      "PVC-subpath",
									MountPath: "/path/to/whatever/1",
									SubPath:   "PVC-subpath-1",
								},
								{
									Name:      "PVC-subpath",
									MountPath: "/path/to/whatever/2",
									SubPath:   "PVC-subpath-2",
								},
								{
									Name:      "PVC-${CI_CONCURRENT_ID}",
									MountPath: "/path/to/whatever/3",
								},
							},
							ConfigMaps: []common.KubernetesConfigMap{
								{Name: "ConfigMap", MountPath: "/path/to/whatever"},
								{
									Name:      "ConfigMap-subpath",
									MountPath: "/${PATH_TO_WHATEVER}",
									SubPath:   "ConfigMap-subpath",
								},
							},
							EmptyDirs: []common.KubernetesEmptyDir{
								{Name: "emptyDir", MountPath: "/path/to/empty/dir"},
								{
									Name:      "emptyDir-subpath",
									MountPath: "/${SUB_PATH}",
									SubPath:   "empty-${SUB_PATH}",
								},
							},
							CSIs: []common.KubernetesCSI{
								{Name: "csi", MountPath: "/path/to/${KEYWORD_CSI}/volume", Driver: "some-driver"},
								{
									Name:      "csi-subpath",
									MountPath: "/path/to/${KEYWORD_CSI}/volume",
									Driver:    "some-driver",
									SubPath:   "${SUB_PATH}",
								},
							},
						},
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					Variables: []spec.Variable{
						{Key: "DOCKER_SOCKET", Value: "/var/run/docker.sock"},
						{Key: "PATH_TWO", Value: "/path/two"},
						{Key: "SUB_PATH", Value: "subpath"},
						{Key: "PATH_TO_WHATEVER", Value: "path/to/whatever"},
						{Key: "KEYWORD_CSI", Value: "csi"},
						{Key: "CI_CONCURRENT_ID", Value: "54"},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: func(featureFlagValue bool) []api.VolumeMount {
				volumeMounts := []api.VolumeMount{
					{Name: "docker", MountPath: "/var/run/docker.sock"},
					{Name: "host-path", MountPath: "/path/two"},
					{Name: "host-subpath", MountPath: "/subpath", SubPath: "subpath"},
					{Name: "Secret", MountPath: "/path/to/whatever"},
					{Name: "Secret-subpath", MountPath: "/path/to/whatever", SubPath: "secret-subpath"},
					{Name: "PVC", MountPath: "/path/to/whatever"},
					{Name: "PVC-subpath", MountPath: "/path/to/whatever/1", SubPath: "PVC-subpath-1"},
					{Name: "PVC-subpath", MountPath: "/path/to/whatever/2", SubPath: "PVC-subpath-2"},
					{Name: "PVC-54", MountPath: "/path/to/whatever/3"},
					{Name: "ConfigMap", MountPath: "/path/to/whatever"},
					{Name: "ConfigMap-subpath", MountPath: "/path/to/whatever", SubPath: "ConfigMap-subpath"},
					{Name: "emptyDir", MountPath: "/path/to/empty/dir"},
					{Name: "emptyDir-subpath", MountPath: "/subpath", SubPath: "empty-subpath"},
					{Name: "csi", MountPath: "/path/to/csi/volume"},
					{Name: "csi-subpath", MountPath: "/path/to/csi/volume", SubPath: "subpath"},
					{Name: "repo", MountPath: "/builds"},
				}

				return addScriptsAndLogVolumeMounts(featureFlagValue, volumeMounts)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			e := newExecutor()
			e.AbstractExecutor.Build = tt.Build
			e.AbstractExecutor.Config = tt.RunnerConfig

			buildtest.SetBuildFeatureFlag(e.Build, featureFlagName, featureFlagValue)
			assert.Equal(t, tt.Expected(featureFlagValue), e.getVolumeMounts())
		})
	}
}

func testVolumesFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	addScriptsAndLogVolumes := func(featureFlagValue bool, v []api.Volume) []api.Volume {
		if featureFlagValue {
			return v
		}

		return append(
			v,

			api.Volume{
				Name: "scripts",
				VolumeSource: api.VolumeSource{
					EmptyDir: &api.EmptyDirVolumeSource{},
				},
			},
			api.Volume{
				Name: "logs",
				VolumeSource: api.VolumeSource{
					EmptyDir: &api.EmptyDirVolumeSource{},
				},
			},
		)
	}
	csiVolFSType := "ext4"
	csiVolReadOnly := false
	oneGig, _ := resource.ParseQuantity("1G")
	tests := map[string]struct {
		GlobalConfig *common.Config
		RunnerConfig common.RunnerConfig
		Build        *common.Build

		Expected func(featureFlagValue bool) []api.Volume
	}{
		"no custom volumes": {
			GlobalConfig: &common.Config{},
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: func(featureFlagValue bool) []api.Volume {
				v := []api.Volume{
					{Name: "repo", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{}}},
				}

				return addScriptsAndLogVolumes(featureFlagValue, v)
			},
		},
		"custom volumes": {
			GlobalConfig: &common.Config{},
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Volumes: common.KubernetesVolumes{
							HostPaths: []common.KubernetesHostPath{
								{Name: "docker", MountPath: "/var/run/docker.sock"},
								{Name: "host-path", MountPath: "/path/two", HostPath: "/path/one"},
								{
									Name:      "host-subpath",
									MountPath: "/subpath",
									HostPath:  "/path/one",
									SubPath:   "subpath",
								},
							},
							PVCs: []common.KubernetesPVC{
								{Name: "PVC", MountPath: "/path/to/whatever"},
								{
									Name:      "PVC-subpath",
									MountPath: "/subpath1",
									SubPath:   "subpath1",
								},
								{
									Name:      "PVC-subpath",
									MountPath: "/subpath2",
									SubPath:   "subpath2",
								},
							},
							ConfigMaps: []common.KubernetesConfigMap{
								{Name: "ConfigMap", MountPath: "/path/to/config", Items: map[string]string{"key_1": "/path/to/key_1"}},
								{
									Name:      "ConfigMap-subpath",
									MountPath: "/subpath",
									Items:     map[string]string{"key_1": "/path/to/key_1"},
									SubPath:   "subpath",
								},
							},
							Secrets: []common.KubernetesSecret{
								{Name: "secret", MountPath: "/path/to/secret", ReadOnly: true, Items: map[string]string{"secret_1": "/path/to/secret_1"}},
								{
									Name:      "secret-subpath",
									MountPath: "/subpath",
									ReadOnly:  true,
									Items:     map[string]string{"secret_1": "/path/to/secret_1"},
									SubPath:   "subpath",
								},
							},
							EmptyDirs: []common.KubernetesEmptyDir{
								{Name: "emptyDirWithoutSize", MountPath: "/path/to/empty/dir", Medium: "Memory"},
								{Name: "emptyDirWithSpaceSize", MountPath: "/path/to/empty/dir", Medium: "Memory", SizeLimit: "  "},
								{Name: "emptyDir", MountPath: "/path/to/empty/dir", Medium: "Memory", SizeLimit: "1G"},
								{
									Name:      "emptyDir-subpath",
									MountPath: "/subpath",
									Medium:    "Memory",
									SizeLimit: "1G",
									SubPath:   "subpath",
								},
							},
							CSIs: []common.KubernetesCSI{
								{
									Name:             "csi",
									MountPath:        "/path/to/csi/volume",
									Driver:           "some-driver",
									FSType:           csiVolFSType,
									ReadOnly:         csiVolReadOnly,
									VolumeAttributes: map[string]string{"key": "value"},
								},
							},
						},
					},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: func(featureFlagValue bool) []api.Volume {
				v := []api.Volume{
					{Name: "docker", VolumeSource: api.VolumeSource{HostPath: &api.HostPathVolumeSource{Path: "/var/run/docker.sock"}}},
					{Name: "host-path", VolumeSource: api.VolumeSource{HostPath: &api.HostPathVolumeSource{Path: "/path/one"}}},
					{Name: "host-subpath", VolumeSource: api.VolumeSource{HostPath: &api.HostPathVolumeSource{Path: "/path/one"}}},
					{
						Name: "secret",
						VolumeSource: api.VolumeSource{
							Secret: &api.SecretVolumeSource{
								SecretName: "secret",
								Items:      []api.KeyToPath{{Key: "secret_1", Path: "/path/to/secret_1"}},
							},
						},
					},
					{
						Name: "secret-subpath",
						VolumeSource: api.VolumeSource{
							Secret: &api.SecretVolumeSource{
								SecretName: "secret-subpath",
								Items:      []api.KeyToPath{{Key: "secret_1", Path: "/path/to/secret_1"}},
							},
						},
					},
					{Name: "PVC", VolumeSource: api.VolumeSource{PersistentVolumeClaim: &api.PersistentVolumeClaimVolumeSource{ClaimName: "PVC"}}},
					{Name: "PVC-subpath", VolumeSource: api.VolumeSource{PersistentVolumeClaim: &api.PersistentVolumeClaimVolumeSource{ClaimName: "PVC-subpath"}}},
					{
						Name: "ConfigMap",
						VolumeSource: api.VolumeSource{
							ConfigMap: &api.ConfigMapVolumeSource{
								LocalObjectReference: api.LocalObjectReference{Name: "ConfigMap"},
								Items:                []api.KeyToPath{{Key: "key_1", Path: "/path/to/key_1"}},
							},
						},
					},
					{
						Name: "ConfigMap-subpath",
						VolumeSource: api.VolumeSource{
							ConfigMap: &api.ConfigMapVolumeSource{
								LocalObjectReference: api.LocalObjectReference{Name: "ConfigMap-subpath"},
								Items:                []api.KeyToPath{{Key: "key_1", Path: "/path/to/key_1"}},
							},
						},
					},
					{Name: "emptyDirWithoutSize", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{Medium: "Memory"}}},
					{Name: "emptyDirWithSpaceSize", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{Medium: "Memory"}}},
					{Name: "emptyDir", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{Medium: "Memory", SizeLimit: &oneGig}}},
					{Name: "emptyDir-subpath", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{Medium: "Memory", SizeLimit: &oneGig}}},
					{
						Name: "csi",
						VolumeSource: api.VolumeSource{
							CSI: &api.CSIVolumeSource{
								Driver:           "some-driver",
								FSType:           &csiVolFSType,
								ReadOnly:         &csiVolReadOnly,
								VolumeAttributes: map[string]string{"key": "value"},
							},
						},
					},
					{Name: "repo", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{}}},
				}

				return addScriptsAndLogVolumes(featureFlagValue, v)
			},
		},
		"default volume with build dir": {
			GlobalConfig: &common.Config{},
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					BuildsDir: "/path/to/builds/dir",
					Kubernetes: &common.KubernetesConfig{
						Volumes: common.KubernetesVolumes{},
					},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: func(featureFlagValue bool) []api.Volume {
				v := []api.Volume{
					{Name: "repo", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{}}},
				}

				return addScriptsAndLogVolumes(featureFlagValue, v)
			},
		},
		"user-provided volume with build dir": {
			GlobalConfig: &common.Config{},
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					BuildsDir: "/path/to/builds/dir",
					Kubernetes: &common.KubernetesConfig{
						Volumes: common.KubernetesVolumes{
							HostPaths: []common.KubernetesHostPath{
								{Name: "user-provided", MountPath: "/path/to/builds/dir"},
							},
						},
					},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: func(featureFlagValue bool) []api.Volume {
				v := []api.Volume{
					{Name: "user-provided", VolumeSource: api.VolumeSource{HostPath: &api.HostPathVolumeSource{Path: "/path/to/builds/dir"}}},
				}

				return addScriptsAndLogVolumes(featureFlagValue, v)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			e := newExecutor()
			e.AbstractExecutor.Build = tt.Build
			e.AbstractExecutor.Config = tt.RunnerConfig

			buildtest.SetBuildFeatureFlag(e.Build, featureFlagName, featureFlagValue)
			assert.Equal(t, tt.Expected(featureFlagValue), e.getVolumes())
		})
	}
}

func testSetupBuildPodServiceCreationErrorFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	version, _ := testVersionAndCodec()

	runnerConfig := common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Kubernetes: &common.KubernetesConfig{
				Namespace:   "default",
				HelperImage: "custom/helper-image",
			},
		},
	}

	fakeRoundTripper := func(req *http.Request) (*http.Response, error) {
		body, errRT := io.ReadAll(req.Body)
		if !assert.NoError(t, errRT, "failed to read request body") {
			return nil, errRT
		}

		p := new(api.Pod)
		errRT = json.Unmarshal(body, p)
		if !assert.NoError(t, errRT, "failed to read request body") {
			return nil, errRT
		}

		if req.URL.Path == "/api/v1/namespaces/default/services" {
			return nil, fmt.Errorf("foobar")
		}

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body: FakeReadCloser{
				Reader: bytes.NewBuffer(body),
			},
		}
		resp.Header = make(http.Header)
		resp.Header.Add(common.ContentType, "application/json")

		return resp, nil
	}

	mockFc := newMockFeatureChecker(t)
	mockFc.On("IsHostAliasSupported").Return(true, nil)
	mockPullManager := pull.NewMockManager(t)

	mockPodWatcher := newMockPodWatcher(t)
	mockPodWatcher.On("UpdatePodName", mock.AnythingOfType("string")).Once()

	ex := newExecutor()
	ex.kubeClient = testKubernetesClient(version, fake.CreateHTTPClient(fakeRoundTripper))
	ex.podWatcher = mockPodWatcher

	ex.options = &kubernetesOptions{
		Image: spec.Image{
			Name:  "test-image",
			Ports: []spec.Port{{Number: 80}},
		},
		Services: map[string]*spec.Image{
			"test-service": {
				Name:  "test-service",
				Alias: "custom_name",
				Ports: []spec.Port{
					{
						Number:   81,
						Name:     "custom_port_name",
						Protocol: "http",
					},
				},
			},
		},
	}
	ex.AbstractExecutor.Config = runnerConfig
	ex.AbstractExecutor.BuildShell = &common.ShellConfiguration{}
	ex.AbstractExecutor.Build = &common.Build{
		Runner: &runnerConfig,
	}
	ex.AbstractExecutor.ProxyPool = proxy.NewPool()
	ex.featureChecker = mockFc
	ex.pullManager = mockPullManager

	buildtest.SetBuildFeatureFlag(ex.Build, featureFlagName, featureFlagValue)

	mockPullManager.On("GetPullPolicyFor", "test-service").
		Return(api.PullAlways, nil).
		Once()
	mockPullManager.On("GetPullPolicyFor", buildContainerName).
		Return(api.PullAlways, nil).
		Once()
	mockPullManager.On("GetPullPolicyFor", helperContainerName).
		Return(api.PullAlways, nil).
		Once()

	err := ex.prepareOverwrites(make(spec.Variables, 0))
	assert.NoError(t, err)

	err = ex.setupBuildPod(t.Context(), nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error creating the proxy service")
}

func testSetupBuildPodFailureGetPullPolicyFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	for _, failOnContainer := range []string{
		"svc-0",
		buildContainerName,
		helperContainerName,
	} {
		t.Run(failOnContainer, func(t *testing.T) {
			runnerConfig := common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "test-helper",
					},
				},
			}

			mockFc := newMockFeatureChecker(t)
			mockFc.On("IsHostAliasSupported").Return(true, nil).Maybe()

			mockPullManager := pull.NewMockManager(t)

			e := newExecutor()
			e.options = &kubernetesOptions{
				Image: spec.Image{
					Name: "test-build",
				},
				Services: map[string]*spec.Image{
					"svc-0": {
						Name: "test-service",
					},
				},
			}
			e.AbstractExecutor.Config = runnerConfig
			e.AbstractExecutor.BuildShell = &common.ShellConfiguration{}
			e.AbstractExecutor.Build = &common.Build{
				Job:    spec.Job{},
				Runner: &runnerConfig,
			}
			e.featureChecker = mockFc
			e.pullManager = mockPullManager

			buildtest.SetBuildFeatureFlag(e.Build, featureFlagName, featureFlagValue)

			mockPullManager.On("GetPullPolicyFor", failOnContainer).
				Return(api.PullAlways, assert.AnError).
				Once()

			mockPullManager.On("GetPullPolicyFor", mock.Anything).
				Return(api.PullAlways, nil).
				Maybe()

			err := e.prepareOverwrites(make(spec.Variables, 0))
			assert.NoError(t, err)

			err = e.setupBuildPod(t.Context(), nil)
			assert.ErrorIs(t, err, assert.AnError)
			assert.Error(t, err)
		})
	}
}

func testGetPodActiveDeadlineSecondsFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	tests := map[string]struct {
		featureFlagValue bool
		timeoutSeconds   int
		expectedTimeout  func(int) *int64
	}{
		"FF_USE_POD_ACTIVE_DEADLINE_SECONDS disabled": {
			timeoutSeconds: 30,
		},
		"FF_USE_POD_ACTIVE_DEADLINE_SECONDS enabled": {
			featureFlagValue: true,
			timeoutSeconds:   30,
			expectedTimeout: func(timeout int) *int64 {
				t := int64(timeout + 1)
				return &t
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			e := newExecutor()
			e.AbstractExecutor.Build = &common.Build{
				Job: spec.Job{
					RunnerInfo: spec.RunnerInfo{
						Timeout: tc.timeoutSeconds,
					},
				},
			}

			buildtest.SetBuildFeatureFlag(e.Build, featureFlagName, featureFlagValue)
			buildtest.SetBuildFeatureFlag(e.Build, "FF_USE_POD_ACTIVE_DEADLINE_SECONDS", tc.featureFlagValue)

			if !tc.featureFlagValue {
				assert.Nil(t, e.getPodActiveDeadlineSeconds())
				return
			}

			assert.EqualValues(t, *tc.expectedTimeout(tc.timeoutSeconds), *e.getPodActiveDeadlineSeconds())
		})
	}
}

func TestCleanup(t *testing.T) {
	version, _ := testVersionAndCodec()
	objectMeta := metav1.ObjectMeta{Name: "test-resource", Namespace: "test-ns"}
	podsEndpointURI := "/api/" + version + "/namespaces/" + objectMeta.Namespace + "/pods/" + objectMeta.Name
	servicesEndpointURI := "/api/" + version + "/namespaces/" + objectMeta.Namespace + "/services/" + objectMeta.Name
	secretsEndpointURI := "/api/" + version + "/namespaces/" + objectMeta.Namespace + "/secrets/" + objectMeta.Name
	configMapsEndpointURI := "/api/" + version + "/namespaces/" + objectMeta.Namespace + "/configmaps/" + objectMeta.Name

	tests := []struct {
		Name        string
		Pod         *api.Pod
		ConfigMap   *api.ConfigMap
		Credentials *api.Secret
		ClientFunc  func(*testing.T, *http.Request) (*http.Response, error)
		Services    []api.Service
		Config      *common.KubernetesConfig
		Error       bool
	}{
		{
			Name: "Proper Cleanup",
			Pod:  &api.Pod{ObjectMeta: objectMeta},
			ClientFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == http.MethodDelete && p == podsEndpointURI:
					return fakeKubeDeleteResponse(http.StatusOK), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
		},
		{
			Name: "Delete failure",
			Pod:  &api.Pod{ObjectMeta: objectMeta},
			ClientFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("delete failed")
			},
			Error: true,
		},
		{
			Name: "POD already deleted",
			Pod:  &api.Pod{ObjectMeta: objectMeta},
			ClientFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == http.MethodDelete && p == podsEndpointURI:
					return fakeKubeDeleteResponse(http.StatusNotFound), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
			Error: true,
		},
		{
			Name:        "POD creation failed, Secrets provided",
			Pod:         nil, // a failed POD create request will cause a nil Pod
			Credentials: &api.Secret{ObjectMeta: objectMeta},
			ClientFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == http.MethodDelete && p == secretsEndpointURI:
					return fakeKubeDeleteResponse(http.StatusNotFound), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
			Error: true,
		},
		{
			Name:     "POD created, Services created",
			Pod:      &api.Pod{ObjectMeta: objectMeta},
			Services: []api.Service{{ObjectMeta: objectMeta}},
			ClientFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == http.MethodDelete && ((p == servicesEndpointURI) || (p == podsEndpointURI)):
					return fakeKubeDeleteResponse(http.StatusOK), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
		},
		{
			Name:     "POD created, Services creation failed",
			Pod:      &api.Pod{ObjectMeta: objectMeta},
			Services: []api.Service{{ObjectMeta: objectMeta}},
			ClientFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == http.MethodDelete && p == podsEndpointURI:
					return fakeKubeDeleteResponse(http.StatusOK), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
			Error: false,
		},
		{
			Name:     "POD creation failed, Services created",
			Pod:      nil, // a failed POD create request will cause a nil Pod
			Services: []api.Service{{ObjectMeta: objectMeta}},
			ClientFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == http.MethodDelete && p == servicesEndpointURI:
					return fakeKubeDeleteResponse(http.StatusOK), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
		},
		{
			Name:     "POD creation failed, Services cleanup failed",
			Pod:      nil, // a failed POD create request will cause a nil Pod
			Services: []api.Service{{ObjectMeta: objectMeta}},
			ClientFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				p, m := req.URL.Path, req.Method
				return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
			},
			Error: false,
		},
		{
			Name:      "ConfigMap cleanup",
			ConfigMap: &api.ConfigMap{ObjectMeta: objectMeta},
			ClientFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == http.MethodDelete && p == configMapsEndpointURI:
					return fakeKubeDeleteResponse(http.StatusOK), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
		},
		{
			Name: "Pod cleanup specifies GracePeriodSeconds with CleanupGracePeriodSeconds set",
			Config: &common.KubernetesConfig{
				CleanupGracePeriodSeconds: common.Int64Ptr(10),
			},
			ClientFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == http.MethodDelete && p == podsEndpointURI:
					defer req.Body.Close()
					b, err := io.ReadAll(req.Body)
					if err != nil {
						return nil, err
					}

					var opts metav1.DeleteOptions
					err = json.Unmarshal(b, &opts)
					if err != nil {
						return nil, err
					}

					assert.EqualValues(t, common.Int64Ptr(10), opts.GracePeriodSeconds)
					return fakeKubeDeleteResponse(http.StatusOK), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
			Pod: &api.Pod{ObjectMeta: objectMeta},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			ex := newExecutor()
			ex.AbstractExecutor.Context = t.Context()
			ex.kubeClient = testKubernetesClient(
				version,
				fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
					return test.ClientFunc(t, req)
				}),
			)
			ex.pod = test.Pod
			ex.credentials = test.Credentials
			ex.services = test.Services
			ex.configurationOverwrites = &overwrites{namespace: "test-ns"}

			errored := false
			buildTrace := FakeBuildTrace{
				testWriter: testWriter{
					call: func(b []byte) (int, error) {
						if !errored {
							if s := string(b); strings.Contains(s, "Error cleaning up") {
								errored = true
							} else if test.Error {
								t.Errorf("expected failure. got: '%s'", string(b))
							}
						}
						return len(b), nil
					},
				},
			}
			ex.AbstractExecutor.BuildLogger = buildlogger.New(buildTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})

			if test.Config == nil {
				test.Config = &common.KubernetesConfig{}
			}
			ex.AbstractExecutor.Config = common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: test.Config,
				},
			}

			ex.Cleanup()

			if test.Error && !errored {
				t.Errorf("expected cleanup to fail but it didn't")
			} else if !test.Error && errored {
				t.Errorf("expected cleanup not to fail but it did")
			}
		})
	}
}

// TestPrepare tests the prepare step.
// They do so by running the Prepare step and then run certain assertions against the internal state of the executor,
// most prominently comparing it to a artificially created, expected executor. To make this work, before we do that
// comparison:
//
//   - we nil out some parts of the actually created executor
//
//   - we don't use the constructor function to create the executor, but use the struct directly, which gives us a
//     "smaller"/"emptier" executor, which makes it easier to compare it with the actual one.
func TestPrepare(t *testing.T) {
	helperImageTag := "latest"
	if common.AppVersion.Version != "development version" {
		helperImageTag = helperimage.Version(common.AppVersion.Version)
	}

	defaultOverwrites := &overwrites{
		namespace:       "default",
		serviceLimits:   api.ResourceList{},
		buildLimits:     api.ResourceList{},
		helperLimits:    api.ResourceList{},
		serviceRequests: api.ResourceList{},
		buildRequests:   api.ResourceList{},
		helperRequests:  api.ResourceList{},
		podRequests:     api.ResourceList{},
		podLimits:       api.ResourceList{},
	}

	defaultHelperImage := helperimage.Info{
		Architecture: "x86_64",
		OSType:       helperimage.OSTypeLinux,
		Name:         helperimage.GitLabRegistryName,
		Tag:          fmt.Sprintf("x86_64-%s", helperImageTag),
		Prebuilt:     "prebuilt-alpine-x86_64",
		Cmd:          []string{"gitlab-runner-build"},
	}

	osType := helperimage.OSTypeLinux
	os := ""
	nodeSelector := map[string]string{}
	if runtime.GOOS == helperimage.OSTypeWindows {
		os = "10.0.20348"
		osType = helperimage.OSTypeWindows
		nodeSelector = map[string]string{
			api.LabelArchStable:           "amd64",
			api.LabelOSStable:             "windows",
			nodeSelectorWindowsBuildLabel: os,
		}
	}
	pwshHelperImage, err := helperimage.Get(common.AppVersion.Version, helperimage.Config{
		Architecture:  "x86_64",
		OSType:        osType,
		Shell:         shells.SNPwsh,
		KernelVersion: os,
	})
	require.NoError(t, err)

	getExecutorForHelperAutoset := func() *executor {
		hi := helperimage.Info{
			Architecture: "x86_64",
			OSType:       helperimage.OSTypeLinux,
			Name:         helperimage.GitLabRegistryName,
			Tag:          fmt.Sprintf("x86_64-%s", helperImageTag),
			Prebuilt:     "prebuilt-alpine-x86_64",
			Cmd:          []string{"gitlab-runner-build"},
		}
		if !strings.Contains(runtime.GOARCH, "amd") {
			hi.Architecture = runtime.GOARCH
			hi.Tag = fmt.Sprintf("%s-%s", hi.Architecture, helperImageTag)
			hi.Prebuilt = "prebuilt-alpine-" + hi.Architecture
		}
		if runtime.GOOS == helperimage.OSTypeWindows {
			hi.OSType = helperimage.OSTypeWindows
			hi.Name = "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper"
			hi.Architecture = "x86_64"
			hi.Tag = "x86_64-latest-servercore1809"
			hi.Prebuilt = "prebuilt-windows-servercore-ltsc2019-x86_64"
			hi.Cmd = []string{
				"powershell", "-NoProfile", "-NoLogo", "-InputFormat", "text", "-OutputFormat", "text", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", "-",
			}
		}

		return &executor{
			options: &kubernetesOptions{
				Image: spec.Image{
					Name: "test-image",
				},
				Services: map[string]*spec.Image{},
			},
			configurationOverwrites: defaultOverwrites,
			helperImageInfo:         hi,
		}
	}

	tests := []struct {
		Name    string
		ErrorRE *regexp.Regexp

		// if Precondition is set and returns false, the test-case is skipped with the message provided
		Precondition func() (bool, string)

		// Note: this RunnerConfig will be added to the Build before we run the test, there are not 2 different
		// RunnerConfigs at play, this split is there to ease the preparation of the test cases.
		RunnerConfig               *common.RunnerConfig
		Build                      *common.Build
		WindowsKernelVersionGetter func() string

		Expected                *executor
		ExpectedPullPolicy      api.PullPolicy
		ExpectedSharedBuildsDir bool
	}{
		{
			Name: "all with limits",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host:                         "test-server",
						ServiceCPULimit:              "100m",
						ServiceMemoryLimit:           "200Mi",
						ServiceEphemeralStorageLimit: "1Gi",
						CPULimit:                     "1.5",
						MemoryLimit:                  "4Gi",
						EphemeralStorageLimit:        "6Gi",
						HelperCPULimit:               "50m",
						HelperMemoryLimit:            "100Mi",
						HelperEphemeralStorageLimit:  "200Mi",
						PodCPULimit:                  "1.8",
						PodMemoryLimit:               "5Gi",
						Privileged:                   func(b bool) *bool { return &b }(true),
						PullPolicy:                   common.StringOrArray{common.PullPolicyIfNotPresent},
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
					Image: spec.Image{
						Name: "test-image",
					},
					Variables: []spec.Variable{
						{Key: "privileged", Value: "true"},
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: &overwrites{
					namespace:       "default",
					buildLimits:     mustCreateResourceList(t, "1.5", "4Gi", "6Gi"),
					serviceLimits:   mustCreateResourceList(t, "100m", "200Mi", "1Gi"),
					helperLimits:    mustCreateResourceList(t, "50m", "100Mi", "200Mi"),
					podLimits:       mustCreateResourceList(t, "1.8", "5Gi", ""),
					buildRequests:   api.ResourceList{},
					serviceRequests: api.ResourceList{},
					helperRequests:  api.ResourceList{},
					podRequests:     api.ResourceList{},
				},
				helperImageInfo: defaultHelperImage,
			},
			ExpectedPullPolicy: api.PullIfNotPresent,
		},
		{
			Name: "all with limits and requests",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host:                           "test-server",
						ServiceAccount:                 "default",
						ServiceAccountOverwriteAllowed: ".*",
						BearerTokenOverwriteAllowed:    true,
						ServiceCPULimit:                "100m",
						ServiceMemoryLimit:             "200Mi",
						ServiceEphemeralStorageLimit:   "2Gi",
						CPULimit:                       "1.5",
						MemoryLimit:                    "4Gi",
						EphemeralStorageLimit:          "3Gi",
						HelperCPULimit:                 "50m",
						HelperMemoryLimit:              "100Mi",
						HelperEphemeralStorageLimit:    "300Mi",
						PodCPULimit:                    "1.8",
						PodMemoryLimit:                 "5Gi",
						ServiceCPURequest:              "99m",
						ServiceMemoryRequest:           "5Mi",
						ServiceEphemeralStorageRequest: "200Mi",
						CPURequest:                     "1",
						MemoryRequest:                  "1.5Gi",
						EphemeralStorageRequest:        "1.3Gi",
						HelperCPURequest:               "0.5m",
						HelperMemoryRequest:            "42Mi",
						HelperEphemeralStorageRequest:  "99Mi",
						PodCPURequest:                  "1.5",
						PodMemoryRequest:               "2Gi",
						Privileged:                     func(b bool) *bool { return &b }(false),
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
					Image: spec.Image{
						Name: "test-image",
					},
					Variables: []spec.Variable{
						{Key: ServiceAccountOverwriteVariableName, Value: "not-default"},
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: &overwrites{
					namespace:       "default",
					serviceAccount:  "not-default",
					buildLimits:     mustCreateResourceList(t, "1.5", "4Gi", "3Gi"),
					buildRequests:   mustCreateResourceList(t, "1", "1.5Gi", "1.3Gi"),
					serviceLimits:   mustCreateResourceList(t, "100m", "200Mi", "2Gi"),
					serviceRequests: mustCreateResourceList(t, "99m", "5Mi", "200Mi"),
					helperLimits:    mustCreateResourceList(t, "50m", "100Mi", "300Mi"),
					helperRequests:  mustCreateResourceList(t, "0.5m", "42Mi", "99Mi"),
					podLimits:       mustCreateResourceList(t, "1.8", "5Gi", ""),
					podRequests:     mustCreateResourceList(t, "1.5", "2Gi", ""),
				},
				helperImageInfo: defaultHelperImage,
			},
		},
		{
			Name: "unmatched service account",
			ErrorRE: regexp.MustCompile(regexp.QuoteMeta(
				`couldn't prepare overwrites: provided value "not-default" does not match "allowed-.*"`,
			)),
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host:                           "test-server",
						ServiceAccount:                 "default",
						ServiceAccountOverwriteAllowed: "allowed-.*",
						ServiceCPULimit:                "100m",
						ServiceMemoryLimit:             "200Mi",
						ServiceEphemeralStorageLimit:   "300Mi",
						CPULimit:                       "1.5",
						MemoryLimit:                    "4Gi",
						EphemeralStorageLimit:          "5Gi",
						HelperCPULimit:                 "50m",
						HelperMemoryLimit:              "100Mi",
						HelperEphemeralStorageLimit:    "200Mi",
						PodCPULimit:                    "1.8",
						PodMemoryLimit:                 "5Gi",
						ServiceCPURequest:              "99m",
						ServiceMemoryRequest:           "5Mi",
						ServiceEphemeralStorageRequest: "50Mi",
						CPURequest:                     "1",
						MemoryRequest:                  "1.5Gi",
						EphemeralStorageRequest:        "40Mi",
						HelperCPURequest:               "0.5m",
						HelperMemoryRequest:            "42Mi",
						HelperEphemeralStorageRequest:  "52Mi",
						PodCPURequest:                  "1.5",
						PodMemoryRequest:               "2Gi",
						Privileged:                     func(b bool) *bool { return &b }(false),
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
					Image: spec.Image{
						Name: "test-image",
					},
					Variables: []spec.Variable{
						{Key: ServiceAccountOverwriteVariableName, Value: "not-default"},
					},
				},
			},
		},
		{
			Name: "regexp match on service account and namespace",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host:                           "test-server",
						Namespace:                      "namespace",
						ServiceAccount:                 "a_service_account",
						ServiceAccountOverwriteAllowed: ".*",
						NamespaceOverwriteAllowed:      "^n.*?e$",
						ServiceCPULimit:                "100m",
						ServiceMemoryLimit:             "200Mi",
						ServiceEphemeralStorageLimit:   "300Mi",
						CPULimit:                       "1.5",
						MemoryLimit:                    "4Gi",
						EphemeralStorageLimit:          "5Gi",
						HelperCPULimit:                 "50m",
						HelperMemoryLimit:              "100Mi",
						HelperEphemeralStorageLimit:    "300Mi",
						PodCPULimit:                    "1.8",
						PodMemoryLimit:                 "5Gi",
						ServiceCPURequest:              "99m",
						ServiceMemoryRequest:           "5Mi",
						ServiceEphemeralStorageRequest: "15Mi",
						CPURequest:                     "1",
						MemoryRequest:                  "1.5Gi",
						EphemeralStorageRequest:        "1.7Gi",
						HelperCPURequest:               "0.5m",
						HelperMemoryRequest:            "42Mi",
						HelperEphemeralStorageRequest:  "32Mi",
						PodCPURequest:                  "1.5",
						PodMemoryRequest:               "2Gi",
						Privileged:                     func(b bool) *bool { return &b }(false),
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
					Image: spec.Image{
						Name: "test-image",
					},
					Variables: []spec.Variable{
						{Key: NamespaceOverwriteVariableName, Value: "new-namespace-name"},
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: &overwrites{
					namespace:       "new-namespace-name",
					serviceAccount:  "a_service_account",
					buildLimits:     mustCreateResourceList(t, "1.5", "4Gi", "5Gi"),
					buildRequests:   mustCreateResourceList(t, "1", "1.5Gi", "1.7Gi"),
					serviceLimits:   mustCreateResourceList(t, "100m", "200Mi", "300Mi"),
					serviceRequests: mustCreateResourceList(t, "99m", "5Mi", "15Mi"),
					helperLimits:    mustCreateResourceList(t, "50m", "100Mi", "300Mi"),
					helperRequests:  mustCreateResourceList(t, "0.5m", "42Mi", "32Mi"),
					podLimits:       mustCreateResourceList(t, "1.8", "5Gi", ""),
					podRequests:     mustCreateResourceList(t, "1.5", "2Gi", ""),
				},
				helperImageInfo: defaultHelperImage,
			},
		},
		{
			Name: "regexp match on namespace",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace:                 "namespace",
						Host:                      "test-server",
						NamespaceOverwriteAllowed: "^namespace-[0-9]$",
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
					Image: spec.Image{
						Name: "test-image",
					},
					Variables: []spec.Variable{
						{Key: NamespaceOverwriteVariableName, Value: "namespace-$CI_CONCURRENT_ID"},
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: &overwrites{
					namespace:       "namespace-0",
					serviceLimits:   api.ResourceList{},
					buildLimits:     api.ResourceList{},
					helperLimits:    api.ResourceList{},
					serviceRequests: api.ResourceList{},
					buildRequests:   api.ResourceList{},
					helperRequests:  api.ResourceList{},
					podRequests:     api.ResourceList{},
					podLimits:       api.ResourceList{},
				},
				helperImageInfo: defaultHelperImage,
			},
		},
		{
			Name: "minimal configuration",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image: "test-image",
						Host:  "test-server",
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
		},
		{
			Name: "minimal configuration with namespace isolation",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image:           "test-image",
						Host:            "test-server",
						NamespacePerJob: true,
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
					Variables: []spec.Variable{
						// Try to bypass namespace isolation
						{Key: NamespaceOverwriteVariableName, Value: "ci-job-42"},
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: &overwrites{
					namespace:       "ci-job-0",
					serviceLimits:   api.ResourceList{},
					buildLimits:     api.ResourceList{},
					helperLimits:    api.ResourceList{},
					serviceRequests: api.ResourceList{},
					buildRequests:   api.ResourceList{},
					helperRequests:  api.ResourceList{},
					podRequests:     api.ResourceList{},
					podLimits:       api.ResourceList{},
				},
				helperImageInfo: defaultHelperImage,
			},
		},
		{
			Name: "minimal configuration with pwsh shell",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Shell: shells.SNPwsh,
					Kubernetes: &common.KubernetesConfig{
						Image:        "test-image",
						Host:         "test-server",
						NodeSelector: nodeSelector,
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: &overwrites{
					namespace:       "default",
					nodeSelector:    nodeSelector,
					serviceLimits:   api.ResourceList{},
					buildLimits:     api.ResourceList{},
					helperLimits:    api.ResourceList{},
					serviceRequests: api.ResourceList{},
					buildRequests:   api.ResourceList{},
					helperRequests:  api.ResourceList{},
					podRequests:     api.ResourceList{},
					podLimits:       api.ResourceList{},
				},
				helperImageInfo: pwshHelperImage,
			},
		},
		{
			Name: "image and one service",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host: "test-server",
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
					Image: spec.Image{
						Name:       "test-image",
						Entrypoint: []string{"/init", "run"},
					},
					Services: spec.Services{
						{
							Name:       "test-service",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
						},
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name:       "test-image",
						Entrypoint: []string{"/init", "run"},
					},
					Services: map[string]*spec.Image{
						"svc-0": {
							Name:       "test-service",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
						},
					},
				},
				configurationOverwrites: &overwrites{
					namespace:       "default",
					serviceLimits:   api.ResourceList{},
					buildLimits:     api.ResourceList{},
					helperLimits:    api.ResourceList{},
					serviceRequests: api.ResourceList{},
					buildRequests:   api.ResourceList{},
					helperRequests:  api.ResourceList{},
					podRequests:     api.ResourceList{},
					podLimits:       api.ResourceList{},
					explicitServiceLimits: map[string]api.ResourceList{
						"svc-0": {},
					},
					explicitServiceRequests: map[string]api.ResourceList{
						"svc-0": {},
					},
				},
				helperImageInfo: defaultHelperImage,
			},
		},
		{
			Name: "merge services",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host: "test-server",
						Services: []common.Service{
							{Name: "test-service-k8s", Alias: "alias"},
							{Name: "test-service-k8s2"},
							{Name: ""},
							{
								Name:    "test-service-k8s3",
								Command: []string{"executable", "param1", "param2"},
							},
							{
								Name:       "test-service-k8s4",
								Entrypoint: []string{"executable", "param3", "param4"},
							},
							{
								Name:       "test-service-k8s5",
								Alias:      "alias5",
								Command:    []string{"executable", "param1", "param2"},
								Entrypoint: []string{"executable", "param3", "param4"},
							},
						},
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
					Image: spec.Image{
						Name:       "test-image",
						Entrypoint: []string{"/init", "run"},
					},
					Services: spec.Services{
						{
							Name:       "test-service",
							Alias:      "test-alias",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
						},
						{
							Name: "",
						},
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name:       "test-image",
						Entrypoint: []string{"/init", "run"},
					},
					Services: map[string]*spec.Image{
						"alias": {
							Name:  "test-service-k8s",
							Alias: "alias",
						},
						"svc-0": {
							Name: "test-service-k8s2",
						},
						"svc-1": {
							Name:    "test-service-k8s3",
							Command: []string{"executable", "param1", "param2"},
						},
						"svc-2": {
							Name:       "test-service-k8s4",
							Entrypoint: []string{"executable", "param3", "param4"},
						},
						"alias5": {
							Name:       "test-service-k8s5",
							Alias:      "alias5",
							Command:    []string{"executable", "param1", "param2"},
							Entrypoint: []string{"executable", "param3", "param4"},
						},
						"test-alias": {
							Name:       "test-service",
							Alias:      "test-alias",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
						},
					},
				},
				configurationOverwrites: &overwrites{
					namespace:       "default",
					serviceLimits:   api.ResourceList{},
					buildLimits:     api.ResourceList{},
					helperLimits:    api.ResourceList{},
					serviceRequests: api.ResourceList{},
					buildRequests:   api.ResourceList{},
					helperRequests:  api.ResourceList{},
					podRequests:     api.ResourceList{},
					podLimits:       api.ResourceList{},
					explicitServiceLimits: map[string]api.ResourceList{
						"alias":      {},
						"svc-0":      {},
						"svc-1":      {},
						"svc-2":      {},
						"alias5":     {},
						"test-alias": {},
					},
					explicitServiceRequests: map[string]api.ResourceList{
						"alias":      {},
						"svc-0":      {},
						"svc-1":      {},
						"svc-2":      {},
						"alias5":     {},
						"test-alias": {},
					},
				},
				helperImageInfo: defaultHelperImage,
			},
		},
		{
			Name: "all with limits and request and explicit services limits and requests",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host:                                              "test-server",
						ServiceCPULimitOverwriteMaxAllowed:                "500m",
						ServiceCPULimit:                                   "100m",
						ServiceCPURequestOverwriteMaxAllowed:              "500m",
						ServiceCPURequest:                                 "50m",
						ServiceMemoryLimitOverwriteMaxAllowed:             "1Gi",
						ServiceMemoryLimit:                                "200Mi",
						ServiceMemoryRequestOverwriteMaxAllowed:           "10Gi",
						ServiceMemoryRequest:                              "100Mi",
						ServiceEphemeralStorageLimitOverwriteMaxAllowed:   "10Gi",
						ServiceEphemeralStorageLimit:                      "1Gi",
						ServiceEphemeralStorageRequestOverwriteMaxAllowed: "10Gi",
						ServiceEphemeralStorageRequest:                    "500Mi",
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
					Image: spec.Image{
						Name:       "test-image",
						Entrypoint: []string{"/init", "run"},
					},
					Services: spec.Services{
						{
							Name:       "test-service-explicit-overrides",
							Alias:      "test-alias-0",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
							Variables: []spec.Variable{
								{
									Key:   ServiceCPULimitOverwriteVariableValue,
									Value: "200m",
								},
								{
									Key:   ServiceCPURequestOverwriteVariableValue,
									Value: "100m",
								},
								{
									Key:   ServiceMemoryLimitOverwriteVariableValue,
									Value: "300Mi",
								},
								{
									Key:   ServiceMemoryRequestOverwriteVariableValue,
									Value: "150Mi",
								},
								{
									Key:   ServiceEphemeralStorageLimitOverwriteVariableValue,
									Value: "2Gi",
								},
								{
									Key:   ServiceEphemeralStorageRequestOverwriteVariableValue,
									Value: "1Gi",
								},
							},
						},
						{
							Name:       "test-service-without-explicit-overrides",
							Alias:      "test-alias-1",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
						},
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name:       "test-image",
						Entrypoint: []string{"/init", "run"},
					},
					Services: map[string]*spec.Image{
						"test-alias-0": {
							Name:       "test-service-explicit-overrides",
							Alias:      "test-alias-0",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
							Variables: []spec.Variable{
								{
									Key:   ServiceCPULimitOverwriteVariableValue,
									Value: "200m",
								},
								{
									Key:   ServiceCPURequestOverwriteVariableValue,
									Value: "100m",
								},
								{
									Key:   ServiceMemoryLimitOverwriteVariableValue,
									Value: "300Mi",
								},
								{
									Key:   ServiceMemoryRequestOverwriteVariableValue,
									Value: "150Mi",
								},
								{
									Key:   ServiceEphemeralStorageLimitOverwriteVariableValue,
									Value: "2Gi",
								},
								{
									Key:   ServiceEphemeralStorageRequestOverwriteVariableValue,
									Value: "1Gi",
								},
							},
						},
						"test-alias-1": {
							Name:       "test-service-without-explicit-overrides",
							Alias:      "test-alias-1",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
						},
					},
				},
				configurationOverwrites: &overwrites{
					namespace:       "default",
					serviceLimits:   mustCreateResourceList(t, "100m", "200Mi", "1Gi"),
					buildLimits:     api.ResourceList{},
					helperLimits:    api.ResourceList{},
					serviceRequests: mustCreateResourceList(t, "50m", "100Mi", "500Mi"),
					buildRequests:   api.ResourceList{},
					helperRequests:  api.ResourceList{},
					podRequests:     api.ResourceList{},
					podLimits:       api.ResourceList{},
					explicitServiceLimits: map[string]api.ResourceList{
						"test-alias-0": mustCreateResourceList(t, "200m", "300Mi", "2Gi"),
						"test-alias-1": mustCreateResourceList(t, "100m", "200Mi", "1Gi"),
					},
					explicitServiceRequests: map[string]api.ResourceList{
						"test-alias-0": mustCreateResourceList(t, "100m", "150Mi", "1Gi"),
						"test-alias-1": mustCreateResourceList(t, "50m", "100Mi", "500Mi"),
					},
				},
				helperImageInfo: defaultHelperImage,
			},
		},
		{
			Name: "all with limits and request and explicit services limits and requests without max override",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host:                           "test-server",
						ServiceCPULimit:                "100m",
						ServiceCPURequest:              "50m",
						ServiceMemoryLimit:             "200Mi",
						ServiceMemoryRequest:           "100Mi",
						ServiceEphemeralStorageLimit:   "1Gi",
						ServiceEphemeralStorageRequest: "500Mi",
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
					Image: spec.Image{
						Name:       "test-image",
						Entrypoint: []string{"/init", "run"},
					},
					Services: spec.Services{
						{
							Name:       "test-service-explicit-overrides",
							Alias:      "test-alias-0",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
							Variables: []spec.Variable{
								{
									Key:   ServiceCPULimitOverwriteVariableValue,
									Value: "200m",
								},
								{
									Key:   ServiceCPURequestOverwriteVariableValue,
									Value: "100m",
								},
								{
									Key:   ServiceMemoryLimitOverwriteVariableValue,
									Value: "300Mi",
								},
								{
									Key:   ServiceMemoryRequestOverwriteVariableValue,
									Value: "150Mi",
								},
								{
									Key:   ServiceEphemeralStorageLimitOverwriteVariableValue,
									Value: "2Gi",
								},
								{
									Key:   ServiceEphemeralStorageRequestOverwriteVariableValue,
									Value: "1Gi",
								},
							},
						},
						{
							Name:       "test-service-without-explicit-overrides",
							Alias:      "test-alias-1",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
						},
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name:       "test-image",
						Entrypoint: []string{"/init", "run"},
					},
					Services: map[string]*spec.Image{
						"test-alias-0": {
							Name:       "test-service-explicit-overrides",
							Alias:      "test-alias-0",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
							Variables: []spec.Variable{
								{
									Key:   ServiceCPULimitOverwriteVariableValue,
									Value: "200m",
								},
								{
									Key:   ServiceCPURequestOverwriteVariableValue,
									Value: "100m",
								},
								{
									Key:   ServiceMemoryLimitOverwriteVariableValue,
									Value: "300Mi",
								},
								{
									Key:   ServiceMemoryRequestOverwriteVariableValue,
									Value: "150Mi",
								},
								{
									Key:   ServiceEphemeralStorageLimitOverwriteVariableValue,
									Value: "2Gi",
								},
								{
									Key:   ServiceEphemeralStorageRequestOverwriteVariableValue,
									Value: "1Gi",
								},
							},
						},
						"test-alias-1": {
							Name:       "test-service-without-explicit-overrides",
							Alias:      "test-alias-1",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
						},
					},
				},
				configurationOverwrites: &overwrites{
					namespace:       "default",
					serviceLimits:   mustCreateResourceList(t, "100m", "200Mi", "1Gi"),
					buildLimits:     api.ResourceList{},
					helperLimits:    api.ResourceList{},
					serviceRequests: mustCreateResourceList(t, "50m", "100Mi", "500Mi"),
					buildRequests:   api.ResourceList{},
					helperRequests:  api.ResourceList{},
					podRequests:     api.ResourceList{},
					podLimits:       api.ResourceList{},
					// Explicit service limits and requests are not set because the max override is not set.
					// Default is used.
					explicitServiceLimits: map[string]api.ResourceList{
						"test-alias-0": mustCreateResourceList(t, "100m", "200Mi", "1Gi"),
						"test-alias-1": mustCreateResourceList(t, "100m", "200Mi", "1Gi"),
					},
					explicitServiceRequests: map[string]api.ResourceList{
						"test-alias-0": mustCreateResourceList(t, "50m", "100Mi", "500Mi"),
						"test-alias-1": mustCreateResourceList(t, "50m", "100Mi", "500Mi"),
					},
				},
				helperImageInfo: defaultHelperImage,
			},
		},
		{
			Name: "all with limits and request and explicit services limits and requests without max override and without default values",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host: "test-server",
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
					Image: spec.Image{
						Name:       "test-image",
						Entrypoint: []string{"/init", "run"},
					},
					Services: spec.Services{
						{
							Name:       "test-service-explicit-overrides",
							Alias:      "test-alias-0",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
							Variables: []spec.Variable{
								{
									Key:   ServiceCPULimitOverwriteVariableValue,
									Value: "200m",
								},
								{
									Key:   ServiceCPURequestOverwriteVariableValue,
									Value: "100m",
								},
								{
									Key:   ServiceMemoryLimitOverwriteVariableValue,
									Value: "300Mi",
								},
								{
									Key:   ServiceMemoryRequestOverwriteVariableValue,
									Value: "150Mi",
								},
								{
									Key:   ServiceEphemeralStorageLimitOverwriteVariableValue,
									Value: "2Gi",
								},
								{
									Key:   ServiceEphemeralStorageRequestOverwriteVariableValue,
									Value: "1Gi",
								},
							},
						},
						{
							Name:       "test-service-without-explicit-overrides",
							Alias:      "test-alias-1",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
						},
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name:       "test-image",
						Entrypoint: []string{"/init", "run"},
					},
					Services: map[string]*spec.Image{
						"test-alias-0": {
							Name:       "test-service-explicit-overrides",
							Alias:      "test-alias-0",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
							Variables: []spec.Variable{
								{
									Key:   ServiceCPULimitOverwriteVariableValue,
									Value: "200m",
								},
								{
									Key:   ServiceCPURequestOverwriteVariableValue,
									Value: "100m",
								},
								{
									Key:   ServiceMemoryLimitOverwriteVariableValue,
									Value: "300Mi",
								},
								{
									Key:   ServiceMemoryRequestOverwriteVariableValue,
									Value: "150Mi",
								},
								{
									Key:   ServiceEphemeralStorageLimitOverwriteVariableValue,
									Value: "2Gi",
								},
								{
									Key:   ServiceEphemeralStorageRequestOverwriteVariableValue,
									Value: "1Gi",
								},
							},
						},
						"test-alias-1": {
							Name:       "test-service-without-explicit-overrides",
							Alias:      "test-alias-1",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
						},
					},
				},
				configurationOverwrites: &overwrites{
					namespace:       "default",
					serviceLimits:   api.ResourceList{},
					buildLimits:     api.ResourceList{},
					helperLimits:    api.ResourceList{},
					serviceRequests: api.ResourceList{},
					buildRequests:   api.ResourceList{},
					helperRequests:  api.ResourceList{},
					podRequests:     api.ResourceList{},
					podLimits:       api.ResourceList{},
					// Explicit service limits and requests are not set because the max override is not set
					// nil is used since default is not defined
					explicitServiceLimits: map[string]api.ResourceList{
						"test-alias-0": mustCreateResourceList(t, "", "", ""),
						"test-alias-1": mustCreateResourceList(t, "", "", ""),
					},
					explicitServiceRequests: map[string]api.ResourceList{
						"test-alias-0": mustCreateResourceList(t, "", "", ""),
						"test-alias-1": mustCreateResourceList(t, "", "", ""),
					},
				},
				helperImageInfo: defaultHelperImage,
			},
		},
		{
			Name: "Default helper image",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host: "test-server",
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					Image: spec.Image{
						Name: "test-image",
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				helperImageInfo:         defaultHelperImage,
				configurationOverwrites: defaultOverwrites,
			},
		},
		{
			Name: "helper image with ubuntu flavour default registry",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host:              "test-server",
						HelperImageFlavor: "ubuntu",
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					Image: spec.Image{
						Name: "test-image",
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo: helperimage.Info{
					OSType:       helperimage.OSTypeLinux,
					Architecture: "x86_64",
					Name:         helperimage.GitLabRegistryName,
					Tag:          fmt.Sprintf("ubuntu-x86_64-%s", helperImageTag),
					Prebuilt:     "prebuilt-ubuntu-x86_64",
					Cmd:          []string{"gitlab-runner-build"},
				},
			},
		},
		{
			Name: "helper image from node selector (linux, arm)",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host: "test-server",
						NodeSelector: map[string]string{
							api.LabelArchStable: "arm64",
							api.LabelOSStable:   "linux",
						},
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					Image: spec.Image{
						Name: "test-image",
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: &overwrites{
					namespace: "default",
					nodeSelector: map[string]string{
						api.LabelArchStable: "arm64",
						api.LabelOSStable:   "linux",
					},
					serviceLimits:   api.ResourceList{},
					buildLimits:     api.ResourceList{},
					helperLimits:    api.ResourceList{},
					serviceRequests: api.ResourceList{},
					buildRequests:   api.ResourceList{},
					helperRequests:  api.ResourceList{},
					podRequests:     api.ResourceList{},
					podLimits:       api.ResourceList{},
				},
				helperImageInfo: helperimage.Info{
					OSType:       "linux",
					Architecture: "arm64",
					Name:         helperimage.GitLabRegistryName,
					Tag:          fmt.Sprintf("arm64-%s", helperImageTag),
					Prebuilt:     "prebuilt-alpine-arm64",
					Cmd:          []string{"gitlab-runner-build"},
				},
			},
		},
		{
			Name: "helper image from node selector (windows, amd64)",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host: "test-server",
						NodeSelector: map[string]string{
							api.LabelArchStable:           "amd64",
							api.LabelOSStable:             "windows",
							nodeSelectorWindowsBuildLabel: "10.0.20348",
						},
					},
					Shell: "pwsh",
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					Image: spec.Image{
						Name: "test-image",
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: &overwrites{
					namespace: "default",
					nodeSelector: map[string]string{
						api.LabelArchStable:           "amd64",
						api.LabelOSStable:             "windows",
						nodeSelectorWindowsBuildLabel: "10.0.20348",
					},
					serviceLimits:   api.ResourceList{},
					buildLimits:     api.ResourceList{},
					helperLimits:    api.ResourceList{},
					serviceRequests: api.ResourceList{},
					buildRequests:   api.ResourceList{},
					helperRequests:  api.ResourceList{},
					podRequests:     api.ResourceList{},
					podLimits:       api.ResourceList{},
				},
				helperImageInfo: helperimage.Info{
					OSType:       "windows",
					Architecture: "x86_64",
					Name:         helperimage.GitLabRegistryName,
					Tag:          fmt.Sprintf("x86_64-%s-servercore21H2", helperImageTag),
					Prebuilt:     "prebuilt-windows-servercore-ltsc2022-x86_64",
					Cmd: []string{
						"pwsh",
						"-NoProfile",
						"-NoLogo",
						"-InputFormat",
						"text",
						"-OutputFormat",
						"text",
						"-NonInteractive",
						"-ExecutionPolicy",
						"Bypass",
						"-EncodedCommand",
						"JABPAHUAdABwAHUAdABFAG4AYwBvAGQAaQBuAGcAIAA9ACAAWwBjAG8AbgBzAG8AbABlAF0AOgA6AEkAbgBwAHUAdABFAG4AYwBvAGQAaQBuAGcAIAA9ACAAWwBjAG8AbgBzAG8AbABlAF0AOgA6AE8AdQB0AHAAdQB0AEUAbgBjAG8AZABpAG4AZwAgAD0AIABOAGUAdwAtAE8AYgBqAGUAYwB0ACAAUwB5AHMAdABlAG0ALgBUAGUAeAB0AC4AVQBUAEYAOABFAG4AYwBvAGQAaQBuAGcADQAKAHAAdwBzAGgAIAAtAE4AbwBQAHIAbwBmAGkAbABlACAALQBOAG8AbgBJAG4AdABlAHIAYQBjAHQAaQB2AGUAIAAtAEMAbwBtAG0AYQBuAGQAIAAtAA0ACgBpAGYAKAAhACQAPwApACAAewAgAEUAeABpAHQAIAAmAHsAaQBmACgAJABMAEEAUwBUAEUAWABJAFQAQwBPAEQARQApACAAewAkAEwAQQBTAFQARQBYAEkAVABDAE8ARABFAH0AIABlAGwAcwBlACAAewAxAH0AfQAgAH0A",
					},
				},
			},
		},
		{
			Name: "helper image from node selector (unknown)",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host: "test-server",
						NodeSelector: map[string]string{
							api.LabelArchStable: "riscv64",
							api.LabelOSStable:   "unknown",
						},
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					Image: spec.Image{
						Name: "test-image",
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         helperimage.Info{},
			},
			ErrorRE: regexp.MustCompile(regexp.QuoteMeta(
				`prepare helper image: unsupported OSType "unknown"`,
			)),
		},
		{
			Name: "helper image from node selector overrides (linux+amd overwritten to linux+arm)",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host: "test-server",
						NodeSelector: map[string]string{
							api.LabelArchStable: "amd64",
							api.LabelOSStable:   "linux",
						},
						NodeSelectorOverwriteAllowed: ".*",
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					Image: spec.Image{
						Name: "test-image",
					},
					Variables: []spec.Variable{
						{Key: NodeSelectorOverwriteVariablePrefix + "ARCH", Value: api.LabelArchStable + "=arm64"},
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: &overwrites{
					namespace: "default",
					nodeSelector: map[string]string{
						api.LabelArchStable: "arm64",
						api.LabelOSStable:   "linux",
					},
					serviceLimits:   api.ResourceList{},
					buildLimits:     api.ResourceList{},
					helperLimits:    api.ResourceList{},
					serviceRequests: api.ResourceList{},
					buildRequests:   api.ResourceList{},
					helperRequests:  api.ResourceList{},
					podRequests:     api.ResourceList{},
					podLimits:       api.ResourceList{},
				},
				helperImageInfo: helperimage.Info{
					OSType:       "linux",
					Architecture: "arm64",
					Name:         helperimage.GitLabRegistryName,
					Tag:          fmt.Sprintf("arm64-%s", helperImageTag),
					Prebuilt:     "prebuilt-alpine-arm64",
					Cmd:          []string{"gitlab-runner-build"},
				},
			},
		},
		{
			Name: "builds dir default",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image: "test-image",
						Host:  "test-server",
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ExpectedSharedBuildsDir: false,
		},
		{
			Name: "builds dir user specified empty_dir",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image: "test-image",
						Host:  "test-server",
						Volumes: common.KubernetesVolumes{
							EmptyDirs: []common.KubernetesEmptyDir{
								{
									Name:      "repo",
									MountPath: "/builds",
									Medium:    "Memory",
									SizeLimit: "1G",
								},
							},
						},
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ExpectedSharedBuildsDir: false,
		},
		{
			Name: "builds dir user specified host_path",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image: "test-image",
						Host:  "test-server",
						Volumes: common.KubernetesVolumes{
							HostPaths: []common.KubernetesHostPath{
								{
									Name:      "repo-host",
									MountPath: "/builds",
									HostPath:  "/mnt/builds",
								},
							},
						},
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ExpectedSharedBuildsDir: true,
		},
		{
			Name: "builds dir user specified pvc",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image: "test-image",
						Host:  "test-server",
						Volumes: common.KubernetesVolumes{
							PVCs: []common.KubernetesPVC{
								{
									Name:      "repo-pvc",
									MountPath: "/builds",
								},
							},
						},
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ExpectedSharedBuildsDir: true,
		},
		{
			Name: "runner pull policy is one of allowed pull policies",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image:               "test-image",
						Host:                "test-server",
						PullPolicy:          common.StringOrArray{common.PullPolicyNever},
						AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways, common.PullPolicyNever},
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ExpectedPullPolicy: api.PullNever,
		},
		{
			Name: "runner pull policy is not one of allowed pull policies",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image:               "test-image",
						Host:                "test-server",
						PullPolicy:          common.StringOrArray{common.PullPolicyIfNotPresent},
						AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways, common.PullPolicyNever},
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					Image: spec.Image{
						Name: "test-image",
					},
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
				},
			},
			Expected: &executor{
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ErrorRE: regexp.MustCompile(
				`invalid pull policy for container "(build|helper|init-permissions)": pull_policy ` +
					regexp.QuoteMeta("([IfNotPresent]) defined in Runner config is not one of the allowed_pull_policies ([Always Never])"),
			),
		},
		{
			Name: "image pull policy is one of allowed pull policies",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image:               "test-image",
						Host:                "test-server",
						AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways, common.PullPolicyNever},
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
					Image: spec.Image{
						PullPolicies: []spec.PullPolicy{common.PullPolicyNever},
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ExpectedPullPolicy: api.PullNever,
		},
		{
			Name: "image pull policy is not one of allowed pull policies",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image:               "test-image",
						Host:                "test-server",
						AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways, common.PullPolicyNever},
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
					Image: spec.Image{
						Name:         "test-image",
						PullPolicies: []spec.PullPolicy{common.PullPolicyIfNotPresent},
					},
				},
			},
			Expected: &executor{
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ErrorRE: regexp.MustCompile(
				`invalid pull policy for container "(build|helper|init-permissions)": pull_policy ` +
					regexp.QuoteMeta("([IfNotPresent]) defined in GitLab pipeline config is not one of the allowed_pull_policies ([Always Never])"),
			),
		},
		{
			Name: "both runner and image pull policies are defined",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image:      "test-image",
						Host:       "test-server",
						PullPolicy: common.StringOrArray{common.PullPolicyNever},
						AllowedPullPolicies: []common.DockerPullPolicy{
							common.PullPolicyAlways,
							common.PullPolicyIfNotPresent,
							common.PullPolicyNever,
						},
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
					Image: spec.Image{
						PullPolicies: []spec.PullPolicy{common.PullPolicyIfNotPresent},
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ExpectedPullPolicy: api.PullIfNotPresent,
		},
		{
			Name: "one of allowed pull policies is invalid",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image:               "test-image",
						Host:                "test-server",
						PullPolicy:          common.StringOrArray{common.PullPolicyNever},
						AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways, "invalid"},
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ErrorRE: regexp.MustCompile(regexp.QuoteMeta(
				`allowed_pull_policies config: unsupported pull policy: "invalid"`,
			)),
		},
		{
			Name: "one of config pull policies is invalid",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image:               "test-image",
						Host:                "test-server",
						PullPolicy:          common.StringOrArray{common.PullPolicyNever, "invalid"},
						AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways},
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ErrorRE: regexp.MustCompile(regexp.QuoteMeta(
				`pull_policy config: unsupported pull policy: "invalid"`,
			)),
		},
		{
			Name: "one of image pull policies is invalid",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image:               "test-image",
						Host:                "test-server",
						PullPolicy:          common.StringOrArray{common.PullPolicyNever, common.PullPolicyAlways},
						AllowedPullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways},
					},
				},
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
					Image: spec.Image{
						Name:         "test-image",
						PullPolicies: []spec.PullPolicy{common.PullPolicyAlways, "invalid"},
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ErrorRE: regexp.MustCompile(regexp.QuoteMeta(
				`conversion to Kubernetes policy: unsupported pull policy: "invalid"`,
			)),
		},
		{
			Name: "autoset helper arch and os",
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image:                       "test-image",
						Host:                        "test-server",
						HelperImageAutosetArchAndOS: true,
					},
				},
			},
			WindowsKernelVersionGetter: func() string {
				// this should produce an executor with an auto-detected helper image config as per `getExecutorForHelperAutoset`
				return "10.0.17763"
			},
			Build: &common.Build{
				Job: spec.Job{
					GitInfo: spec.GitInfo{
						Sha: "1234567890",
					},
					Image: spec.Image{
						Name: "test-image",
					},
				},
			},
			Expected: getExecutorForHelperAutoset(),
		},
		{
			Name: "autoset helper arch and os on unsupported windows kernel",
			Precondition: func() (bool, string) {
				if runtime.GOOS != "windows" {
					return false, "skipping test, because we are not running on windows but on " + runtime.GOOS
				}
				return true, ""
			},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image:                       "test-image",
						Host:                        "test-server",
						HelperImageAutosetArchAndOS: true,
					},
				},
			},
			WindowsKernelVersionGetter: func() string { return "unsupported-kernel-version" },
			ErrorRE: regexp.MustCompile(regexp.QuoteMeta(
				`prepare helper image: detecting base image: unsupported Windows version: unsupported-kernel-version`,
			)),
		},
		{
			Name: "autoset helper arch and os on non windows does not need windows kernel version",
			Precondition: func() (bool, string) {
				if runtime.GOOS == "windows" {
					return false, "skipping test, because we are running on windows"
				}
				return true, ""
			},
			WindowsKernelVersionGetter: func() string {
				panic("this should never be called on non-windows")
			},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image:                       "test-image",
						Host:                        "test-server",
						HelperImageAutosetArchAndOS: true,
					},
				},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: spec.Image{
						Name: "test-image",
					},
					Services: map[string]*spec.Image{},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			if precondition := test.Precondition; precondition != nil {
				if shouldRun, msg := precondition(); !shouldRun {
					t.Skip(msg)
				}
			}

			testBuild := test.Build
			if testBuild == nil {
				testBuild = &common.Build{}
			}
			testBuild.Runner = test.RunnerConfig

			e := newExecutor()
			e.newPodWatcher = func(c podWatcherConfig) podWatcher {
				mockPodWatcher := newMockPodWatcher(t)
				mockPodWatcher.On("Start").Return(nil).Maybe()
				return mockPodWatcher
			}
			e.windowsKernelVersion = test.WindowsKernelVersionGetter

			mockTrace := buildlogger.NewMockTrace(t)
			mockTrace.EXPECT().IsStdout().Return(true).Once()
			mockTrace.EXPECT().Write(mock.Anything).Return(0, nil).Maybe()

			// TODO: handle the context properly with https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27932
			prepareOptions := common.ExecutorPrepareOptions{
				Config:      testBuild.Runner,
				Build:       testBuild,
				Context:     t.Context(),
				BuildLogger: buildlogger.New(mockTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{}),
			}

			err := e.Prepare(prepareOptions)
			if err != nil {
				assert.False(t, testBuild.IsSharedEnv())
			}
			if test.ErrorRE != nil {
				assert.Error(t, err)
				assert.Regexp(t, test.ErrorRE, err.Error())
				return
			}
			require.NoError(t, err)

			// Set this to nil so we aren't testing the functionality of the
			// base AbstractExecutor's Prepare method
			e.AbstractExecutor = executors.AbstractExecutor{}

			pullPolicy, err := e.pullManager.GetPullPolicyFor(buildContainerName)
			assert.NoError(t, err)
			assert.Equal(t, test.ExpectedPullPolicy, pullPolicy)

			sharedBuildsDir := e.isSharedBuildsDirRequired()
			assert.Equal(t, test.ExpectedSharedBuildsDir, sharedBuildsDir)

			// we deliberately set some things on the executor to nil, to make the comparison to the expected & artificially
			// constructed executor succeed
			e.kubeClient = nil
			e.kubeConfig = nil
			e.featureChecker = nil
			e.pullManager = nil
			e.requireDefaultBuildsDirVolume = nil
			e.requireSharedBuildsDir = nil
			e.newLogProcessor = nil
			e.remoteProcessTerminated = nil
			e.getKubeConfig = nil
			e.newKubeClient = nil
			e.windowsKernelVersion = nil
			e.options.Image.PullPolicies = nil
			e.newPodWatcher = nil
			e.podWatcher = nil

			if test.Expected.Config.IsProxyExec() {
				test.Expected.helperImageInfo.Cmd = append(
					[]string{"gitlab-runner-helper", "proxy-exec", "--bootstrap"},
					test.Expected.helperImageInfo.Cmd...,
				)
			}

			assert.NoError(t, err)
			assert.Equal(t, test.Expected, e)
		})
	}
}

func TestSetupDefaultExecutorOptions(t *testing.T) {
	tests := map[string]func(*testing.T, *executor){
		"windows": func(t *testing.T, e *executor) {
			assert.Equal(t, e.DefaultBuildsDir, `C:\builds`)
			assert.Equal(t, e.DefaultCacheDir, `C:\cache`)
		},
		"linux": func(t *testing.T, e *executor) {
			assert.Equal(t, e.DefaultBuildsDir, `/builds`)
			assert.Equal(t, e.DefaultCacheDir, `/cache`)
		},
	}

	for os, tc := range tests {
		t.Run(os, func(t *testing.T) {
			e := newExecutor()
			e.setupDefaultExecutorOptions(os)
			tc(t, e)
		})
	}
}

func TestSetupCredentials(t *testing.T) {
	tests := map[string]struct {
		RunnerCredentials *common.RunnerCredentials
		Credentials       []spec.Credentials
		VerifyFn          func(*testing.T, *api.Secret)
	}{
		"no credentials": {
			// don't execute VerifyFn
			VerifyFn: nil,
		},
		"registry credentials": {
			Credentials: []spec.Credentials{
				{
					Type:     "registry",
					URL:      "http://example.com",
					Username: "user",
					Password: "password",
				},
			},
			VerifyFn: func(t *testing.T, secret *api.Secret) {
				assert.Equal(t, api.SecretTypeDockercfg, secret.Type)
				assert.NotEmpty(t, secret.Data[api.DockerConfigKey])
			},
		},
		"other credentials": {
			Credentials: []spec.Credentials{
				{
					Type:     "other",
					URL:      "http://example.com",
					Username: "user",
					Password: "password",
				},
			},
			// don't execute VerifyFn
			VerifyFn: nil,
		},
		"non-DNS-1123-compatible-token": {
			RunnerCredentials: &common.RunnerCredentials{
				Token: "ToK3_?OF",
			},
			Credentials: []spec.Credentials{
				{
					Type:     "registry",
					URL:      "http://example.com",
					Username: "user",
					Password: "password",
				},
			},
			VerifyFn: func(t *testing.T, secret *api.Secret) {
				dns_test.AssertRFC1123Compatibility(t, secret.GetName())
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			fakeKubeClient := testclient.NewClientset()

			createCount := 0
			fakeKubeClient.PrependReactor("create", "secrets", func(action k8stesting.Action) (handled bool, ret kuberuntime.Object, err error) {
				createCount += 1

				createAction, ok := action.(k8stesting.CreateAction)
				require.True(t, ok, "expected action %v to be a create action", action)

				obj := createAction.GetObject()
				secret, ok := obj.(*api.Secret)
				require.True(t, ok, "expected object %v to be a secret", obj)

				if verify := test.VerifyFn; verify != nil {
					verify(t, secret)
				}

				return true, nil, nil
			})

			ex := newExecutor()
			ex.kubeClient = fakeKubeClient
			ex.AbstractExecutor.Config.RunnerSettings.Kubernetes.Namespace = "default"
			ex.AbstractExecutor.Build = &common.Build{
				Job: spec.Job{
					Credentials: test.Credentials,
				},
				Runner: &common.RunnerConfig{},
			}

			if test.RunnerCredentials != nil {
				ex.Build.Runner = &common.RunnerConfig{
					RunnerCredentials: *test.RunnerCredentials,
				}
			}

			err := ex.prepareOverwrites(nil)
			assert.NoError(t, err, "error on prepareOverwrites")

			err = ex.setupCredentials(t.Context())
			assert.NoError(t, err, "error on setupCredentials")

			if test.VerifyFn != nil {
				assert.Equal(t, 1, createCount, "expected %d secret creations, got: %d", 1, createCount)
			} else {
				assert.Equal(t, 0, createCount, "expected %d secret creations, got: %d", 0, createCount)
			}
		})
	}
}

func TestSetupBuildNamespace(t *testing.T) {
	version, _ := testVersionAndCodec()

	type testDef struct {
		NamespaceIsolation bool
		VerifyFn           func(*testing.T, testDef, *api.Namespace, string)
	}
	tests := map[string]testDef{
		"namespace isolation disabled": {
			// don't execute VerifyFn
			NamespaceIsolation: false,
			VerifyFn:           nil,
		},
		"namespace isolation enabled": {
			NamespaceIsolation: true,
			VerifyFn: func(t *testing.T, test testDef, namespace *api.Namespace, method string) {
				assert.Equal(t, "ci-job-0", namespace.Name)
				assert.Equal(t, http.MethodPost, method)
			},
		},
	}

	executed := false
	fakeClientRoundTripper := func(test testDef) func(req *http.Request) (*http.Response, error) {
		return func(req *http.Request) (resp *http.Response, err error) {
			namespaceBytes, err := io.ReadAll(req.Body)
			executed = true

			if err != nil {
				t.Errorf("failed to read request body: %s", err.Error())
				return
			}

			n := new(api.Namespace)

			err = json.Unmarshal(namespaceBytes, n)
			if err != nil {
				t.Errorf("error decoding namespace: %s", err.Error())
				return
			}

			if test.VerifyFn != nil {
				test.VerifyFn(t, test, n, req.Method)
			}

			resp = &http.Response{StatusCode: http.StatusOK, Body: FakeReadCloser{
				Reader: bytes.NewBuffer(namespaceBytes),
			}}
			resp.Header = make(http.Header)
			resp.Header.Add(common.ContentType, "application/json")

			return
		}
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			ex := newExecutor()
			ex.kubeClient = testKubernetesClient(version, fake.CreateHTTPClient(fakeClientRoundTripper(test)))
			ex.options = &kubernetesOptions{}
			ex.AbstractExecutor.Config.RunnerSettings.Kubernetes.Image = "default-image"
			ex.AbstractExecutor.Config.RunnerSettings.Kubernetes.NamespacePerJob = test.NamespaceIsolation
			ex.AbstractExecutor.Build = &common.Build{}

			mockTrace := buildlogger.NewMockTrace(t)
			mockTrace.EXPECT().IsStdout().Return(true).Once()
			mockTrace.EXPECT().Write(mock.Anything).Return(0, nil)
			ex.AbstractExecutor.BuildLogger = buildlogger.New(mockTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})

			executed = false

			err := ex.prepareOverwrites(spec.Variables{})
			assert.NoError(t, err)
			err = ex.checkDefaults()
			assert.NoError(t, err)

			err = ex.setupBuildNamespace(t.Context())
			assert.NoError(t, err)

			if test.VerifyFn != nil {
				assert.True(t, executed)
			} else {
				assert.False(t, executed)
			}
		})
	}
}

func TestTeardownBuildNamespace(t *testing.T) {
	version, _ := testVersionAndCodec()

	type testDef struct {
		NamespaceIsolation bool
		VerifyFn           func(*testing.T, testDef, string, string)
	}
	tests := map[string]testDef{
		"namespace isolation disabled": {
			// don't execute VerifyFn
			NamespaceIsolation: false,
			VerifyFn:           nil,
		},
		"namespace isolation enabled": {
			NamespaceIsolation: true,
			VerifyFn: func(t *testing.T, test testDef, namespace string, method string) {
				assert.Equal(t, "ci-job-0", namespace)
				assert.Equal(t, http.MethodDelete, method)
			},
		},
	}

	executed := false
	fakeClientRoundTripper := func(test testDef) func(req *http.Request) (*http.Response, error) {
		return func(req *http.Request) (resp *http.Response, err error) {
			executed = true

			pathSplit := strings.Split(req.URL.Path, "/")

			if test.VerifyFn != nil {
				test.VerifyFn(t, test, pathSplit[len(pathSplit)-1], req.Method)
			}

			resp = &http.Response{StatusCode: http.StatusOK}
			resp.Header = make(http.Header)
			resp.Header.Add(common.ContentType, "application/json")

			return
		}
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			ex := newExecutor()
			ex.kubeClient = testKubernetesClient(version, fake.CreateHTTPClient(fakeClientRoundTripper(test)))
			ex.options = &kubernetesOptions{}
			ex.AbstractExecutor.Config.RunnerSettings.Kubernetes.Image = "default-image"
			ex.AbstractExecutor.Config.RunnerSettings.Kubernetes.NamespacePerJob = test.NamespaceIsolation
			ex.AbstractExecutor.Build = &common.Build{}

			mockTrace := buildlogger.NewMockTrace(t)
			mockTrace.EXPECT().IsStdout().Return(true).Once()
			mockTrace.EXPECT().Write(mock.Anything).Return(0, nil)
			ex.AbstractExecutor.BuildLogger = buildlogger.New(mockTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})

			executed = false

			err := ex.prepareOverwrites(spec.Variables{})
			assert.NoError(t, err)
			err = ex.checkDefaults()
			assert.NoError(t, err)

			err = ex.teardownBuildNamespace(t.Context())
			assert.NoError(t, err)

			if test.VerifyFn != nil {
				assert.True(t, executed)
			} else {
				assert.False(t, executed)
			}
		})
	}
}

func TestServiceAccountExists(t *testing.T) {
	version, codec := testVersionAndCodec()
	errClientFunc := fmt.Errorf("unexpected request")
	namespace := "default"

	tests := map[string]struct {
		clientFunc func(*http.Request) (*http.Response, error)
		name       string
		found      bool
	}{
		"serviceaccount exists": {
			clientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case p == "/api/"+version+"/namespaces/"+namespace+"/serviceaccounts/my-serviceaccount" &&
					m == http.MethodGet:
					sa := &api.ServiceAccount{
						ObjectMeta: metav1.ObjectMeta{
							Name: "my-serviceaccount",
						},
					}

					return &http.Response{
						Header:     map[string][]string{common.ContentType: {"application/json"}},
						StatusCode: http.StatusOK,
						Status:     http.StatusText(http.StatusOK),
						Body:       io.NopCloser(bytes.NewReader([]byte(kuberuntime.EncodeOrDie(codec, sa)))),
					}, nil
				default:
					t.Errorf("unexpected request: %s %#v\n%#v", req.Method, req.URL, req)
					return nil, errClientFunc
				}
			},
			name:  "my-serviceaccount",
			found: true,
		},
		"serviceaccount does not exist": {
			clientFunc: func(req *http.Request) (*http.Response, error) {
				sa := &api.ServiceAccount{}
				return &http.Response{
					Header:     map[string][]string{common.ContentType: {"application/json"}},
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Body:       io.NopCloser(bytes.NewReader([]byte(kuberuntime.EncodeOrDie(codec, sa)))),
				}, errClientFunc
			},
			name:  "my-serviceaccount-1",
			found: false,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			ex := newExecutor()
			ex.kubeClient = testKubernetesClient(version, fake.CreateHTTPClient(tc.clientFunc))
			ex.AbstractExecutor.Config.RunnerSettings.Kubernetes.Namespace = namespace

			ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
			defer cancel()

			err := ex.prepareOverwrites(make(spec.Variables, 0))
			assert.NoError(t, err)
			assert.Equal(t, tc.found, ex.serviceAccountExists()(ctx, tc.name))
		})
	}
}

func TestSecretExists(t *testing.T) {
	version, codec := testVersionAndCodec()
	errClientFunc := fmt.Errorf("unexpected request")

	tests := map[string]struct {
		clientFunc func(*http.Request) (*http.Response, error)
		name       string
		found      bool
	}{
		"secret exists": {
			clientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case p == "/api/"+version+"/namespaces/"+DefaultResourceIdentifier+"/secrets/my-secret" &&
					m == http.MethodGet:
					s := &api.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name: "my-secret",
						},
					}

					return &http.Response{
						Header:     map[string][]string{common.ContentType: {"application/json"}},
						StatusCode: http.StatusOK,
						Status:     http.StatusText(http.StatusOK),
						Body:       io.NopCloser(bytes.NewReader([]byte(kuberuntime.EncodeOrDie(codec, s)))),
					}, nil
				default:
					t.Errorf("unexpected request: %s %#v\n%#v", req.Method, req.URL, req)
					return nil, errClientFunc
				}
			},
			name:  "my-secret",
			found: true,
		},
		"secret does not exist": {
			clientFunc: func(req *http.Request) (*http.Response, error) {
				sa := &api.ServiceAccount{}
				return &http.Response{
					Header:     map[string][]string{common.ContentType: {"application/json"}},
					StatusCode: http.StatusOK,
					Status:     http.StatusText(http.StatusOK),
					Body:       io.NopCloser(bytes.NewReader([]byte(kuberuntime.EncodeOrDie(codec, sa)))),
				}, errClientFunc
			},
			name:  "my-secret-1",
			found: false,
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			ex := newExecutor()
			ex.kubeClient = testKubernetesClient(version, fake.CreateHTTPClient(tc.clientFunc))
			ex.AbstractExecutor.Config.RunnerSettings.Kubernetes.Namespace = DefaultResourceIdentifier

			ctx, cancel := context.WithTimeout(t.Context(), time.Second*30)
			defer cancel()

			err := ex.prepareOverwrites(make(spec.Variables, 0))
			assert.NoError(t, err)
			assert.Equal(t, tc.found, ex.secretExists()(ctx, tc.name))
		})
	}
}

func TestWaitForResources(t *testing.T) {
	attempt := -1
	version, codec := testVersionAndCodec()
	errClientFunc := fmt.Errorf("unexpected request")

	tests := map[string]struct {
		ctxTimeout       time.Duration
		clientFunc       func(*http.Request) (*http.Response, error)
		resourceType     string
		serviceAccount   string
		imagePullSecrets []string
		expectedErr      error
	}{
		"no service account set": {
			ctxTimeout: time.Second * 30,
			clientFunc: func(req *http.Request) (*http.Response, error) {
				return nil, nil
			},
			resourceType: resourceTypeServiceAccount,
		},
		"no secrets set": {
			ctxTimeout: time.Second * 30,
			clientFunc: func(req *http.Request) (*http.Response, error) {
				return nil, nil
			},
			resourceType: resourceTypePullSecret,
		},
		"service account exists": {
			ctxTimeout: time.Second * 30,
			clientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case p == "/api/"+version+"/namespaces/"+DefaultResourceIdentifier+
					"/serviceaccounts/my-serviceaccount" && m == http.MethodGet:
					sa := &api.ServiceAccount{
						ObjectMeta: metav1.ObjectMeta{
							Name: "my-serviceaccount",
						},
					}

					return &http.Response{
						Header:     map[string][]string{common.ContentType: {"application/json"}},
						StatusCode: http.StatusOK,
						Status:     http.StatusText(http.StatusOK),
						Body:       io.NopCloser(bytes.NewReader([]byte(kuberuntime.EncodeOrDie(codec, sa)))),
					}, nil
				default:
					t.Errorf("unexpected request: %s %#v\n%#v", req.Method, req.URL, req)
					return nil, errClientFunc
				}
			},
			resourceType:   resourceTypeServiceAccount,
			serviceAccount: "my-serviceaccount",
		},
		"secret exists": {
			ctxTimeout: time.Second * 30,
			clientFunc: func(req *http.Request) (*http.Response, error) {
				s := &api.Secret{}
				switch p, m := req.URL.Path, req.Method; {
				case p == "/api/"+version+"/namespaces/"+DefaultResourceIdentifier+"/secrets/my-secret-1" &&
					m == http.MethodGet:
					s.ObjectMeta = metav1.ObjectMeta{
						Name: "my-secret-1",
					}

					return &http.Response{
						Header:     map[string][]string{common.ContentType: {"application/json"}},
						StatusCode: http.StatusOK,
						Status:     http.StatusText(http.StatusOK),
						Body:       io.NopCloser(bytes.NewReader([]byte(kuberuntime.EncodeOrDie(codec, s)))),
					}, nil
				default:
					t.Errorf("unexpected request: %s %#v\n%#v", req.Method, req.URL, req)
					return nil, errClientFunc
				}
			},
			resourceType:     resourceTypePullSecret,
			imagePullSecrets: []string{"my-secret-1"},
		},
		"service account does not exist": {
			ctxTimeout: time.Second * 30,
			clientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case p == "/api/"+version+"/namespaces/"+DefaultResourceIdentifier+
					"/serviceaccounts/my-serviceaccount" && m == http.MethodGet:
					sa := &api.ServiceAccount{}

					return &http.Response{
						Header:     map[string][]string{common.ContentType: {"application/json"}},
						StatusCode: http.StatusOK,
						Status:     http.StatusText(http.StatusOK),
						Body:       io.NopCloser(bytes.NewReader([]byte(kuberuntime.EncodeOrDie(codec, sa)))),
					}, errClientFunc
				default:
					return nil, errClientFunc
				}
			},
			resourceType:   resourceTypeServiceAccount,
			serviceAccount: "my-serviceaccount",
			expectedErr: &resourceCheckError{
				resourceType: resourceTypeServiceAccount,
				resourceName: "my-serviceaccount",
			},
		},
		"secret does not exist": {
			ctxTimeout: time.Second * 30,
			clientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case p == "/api/"+version+"/namespaces/"+DefaultResourceIdentifier+"/secrets/my-secret-1" &&
					m == http.MethodGet:
					s := &api.Secret{}

					return &http.Response{
						Header:     map[string][]string{common.ContentType: {"application/json"}},
						StatusCode: http.StatusOK,
						Status:     http.StatusText(http.StatusOK),
						Body:       io.NopCloser(bytes.NewReader([]byte(kuberuntime.EncodeOrDie(codec, s)))),
					}, errClientFunc
				default:
					return nil, errClientFunc
				}
			},
			resourceType:     resourceTypePullSecret,
			imagePullSecrets: []string{"my-secret-1"},
			expectedErr: &resourceCheckError{
				resourceType: resourceTypePullSecret,
				resourceName: "my-secret-1",
			},
		},
		"secret found after multiple attempts": {
			ctxTimeout: time.Second * 30,
			clientFunc: func(req *http.Request) (*http.Response, error) {
				if attempt < 2 {
					attempt++
					return nil, errClientFunc
				}

				switch p, m := req.URL.Path, req.Method; {
				case p == "/api/"+version+"/namespaces/"+DefaultResourceIdentifier+"/secrets/my-secret-1" &&
					m == http.MethodGet:
					s := &api.Secret{
						TypeMeta: metav1.TypeMeta{
							Kind: "Secret",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name: "my-secret-1",
						},
					}

					return &http.Response{
						Header:     map[string][]string{common.ContentType: {"application/json"}},
						StatusCode: http.StatusOK,
						Status:     http.StatusText(http.StatusOK),
						Body:       io.NopCloser(bytes.NewReader([]byte(kuberuntime.EncodeOrDie(codec, s)))),
					}, nil
				default:
					return nil, errClientFunc
				}
			},
			resourceType:     resourceTypePullSecret,
			imagePullSecrets: []string{"my-secret-1"},
		},
		"service account found after multiple attempts": {
			ctxTimeout: time.Second * 30,
			clientFunc: func(req *http.Request) (*http.Response, error) {
				if attempt < 2 {
					attempt++
					return nil, errClientFunc
				}

				switch p, m := req.URL.Path, req.Method; {
				case p == "/api/"+version+"/namespaces/"+DefaultResourceIdentifier+
					"/serviceaccounts/my-serviceaccount" && m == http.MethodGet:
					sa := &api.ServiceAccount{
						ObjectMeta: metav1.ObjectMeta{
							Name: "my-serviceaccount",
						},
					}

					return &http.Response{
						Header:     map[string][]string{common.ContentType: {"application/json"}},
						StatusCode: http.StatusOK,
						Status:     http.StatusText(http.StatusOK),
						Body:       io.NopCloser(bytes.NewReader([]byte(kuberuntime.EncodeOrDie(codec, sa)))),
					}, nil
				default:
					return nil, errClientFunc
				}
			},
			resourceType:   resourceTypeServiceAccount,
			serviceAccount: "my-serviceaccount",
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			ex := newExecutor()
			ex.kubeClient = testKubernetesClient(version, fake.CreateHTTPClient(tc.clientFunc))
			ex.AbstractExecutor.Config.RunnerSettings.Kubernetes.Namespace = DefaultResourceIdentifier
			ex.AbstractExecutor.Config.RunnerSettings.Kubernetes.ResourceAvailabilityCheckMaxAttempts = 3

			var err error

			ctx, cancel := context.WithTimeout(t.Context(), tc.ctxTimeout)
			defer cancel()

			err = ex.prepareOverwrites(make(spec.Variables, 0))
			assert.NoError(t, err)

			switch tc.resourceType {
			case resourceTypeServiceAccount:
				err = ex.waitForResource(ctx, tc.resourceType, tc.serviceAccount, ex.serviceAccountExists())
			case resourceTypePullSecret:
				if len(tc.imagePullSecrets) > 0 {
					err = ex.waitForResource(ctx, tc.resourceType, tc.imagePullSecrets[0], ex.secretExists())
				}
			}

			if tc.expectedErr != nil {
				assert.Error(t, err, tc.expectedErr.Error())
				return
			}

			assert.NoError(t, err)
		})
	}
}

type setupBuildPodTestDef struct {
	RunnerConfig             common.RunnerConfig
	Variables                []spec.Variable
	Credentials              []spec.Credentials
	Options                  *kubernetesOptions
	InitContainers           []api.Container
	SetHTTPPutResponse       func() (*http.Response, error)
	PrepareFn                func(*testing.T, setupBuildPodTestDef, *executor)
	VerifyFn                 func(*testing.T, setupBuildPodTestDef, *api.Pod)
	VerifyExecutorFn         func(*testing.T, setupBuildPodTestDef, *executor)
	VerifySetupBuildPodErrFn func(*testing.T, error)
}

type setupBuildPodFakeRoundTripper struct {
	t    *testing.T
	test setupBuildPodTestDef

	mu       sync.Mutex
	executed bool
}

func (rt *setupBuildPodFakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	if req.Method == http.MethodGet && strings.Contains(req.URL.Path, "secrets") {
		part := strings.Split(req.URL.Path, "/")
		return buildSecretAPIResponse(rt.t, part[len(part)-1])
	}

	if req.Method == http.MethodGet && strings.Contains(req.URL.Path, "serviceaccounts") {
		part := strings.Split(req.URL.Path, "/")
		return buildServiceAccountAPIResponse(rt.t, part[len(part)-1])
	}

	rt.executed = true
	dataBytes, err := io.ReadAll(req.Body)
	if !assert.NoError(rt.t, err, "failed to read request body") {
		return nil, err
	}

	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: FakeReadCloser{
			Reader: bytes.NewBuffer(dataBytes),
		},
	}
	resp.Header = make(http.Header)
	resp.Header.Add(common.ContentType, "application/json")

	if strings.Contains(req.URL.Path, "pods") {
		p := new(api.Pod)
		err = json.Unmarshal(dataBytes, p)
		if !assert.NoError(rt.t, err, "failed to read request body") {
			return nil, err
		}

		if rt.test.VerifyFn != nil {
			rt.test.VerifyFn(rt.t, rt.test, p)
		}

		return resp, nil
	}

	if req.Method == http.MethodPost && strings.Contains(req.URL.Path, "secrets") {
		s := new(api.Secret)
		err = json.Unmarshal(dataBytes, s)
		if !assert.NoError(rt.t, err, "failed to read request body") {
			return nil, err
		}
		s.SetName("secret-name")
		dataBytes, err = json.Marshal(s)
		if !assert.NoError(rt.t, err, "failed to marshal secret named") {
			return nil, err
		}
		resp = &http.Response{
			StatusCode: http.StatusOK,
			Body: FakeReadCloser{
				Reader: bytes.NewBuffer(dataBytes),
			},
		}
		resp.Header = make(http.Header)
		resp.Header.Add(common.ContentType, "application/json")
		return resp, nil
	}

	if req.Method == http.MethodPut && rt.test.SetHTTPPutResponse != nil {
		return rt.test.SetHTTPPutResponse()
	}

	return resp, nil
}

func buildSecretAPIResponse(t *testing.T, secretName string) (*http.Response, error) {
	s := new(api.Secret)
	s.SetName(secretName)
	dataBytes, err := json.Marshal(s)
	if !assert.NoError(t, err, "failed to marshall secret") {
		return nil, err
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: FakeReadCloser{
			Reader: bytes.NewBuffer(dataBytes),
		},
	}
	resp.Header = make(http.Header)
	resp.Header.Add(common.ContentType, "application/json")
	return resp, nil
}

func buildServiceAccountAPIResponse(t *testing.T, saName string) (*http.Response, error) {
	sa := new(api.ServiceAccount)
	sa.SetName(saName)
	dataBytes, err := json.Marshal(sa)
	if !assert.NoError(t, err, "failed to marshall serviceaccount") {
		return nil, err
	}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: FakeReadCloser{
			Reader: bytes.NewBuffer(dataBytes),
		},
	}
	resp.Header = make(http.Header)
	resp.Header.Add(common.ContentType, "application/json")
	return resp, nil
}

func TestSetupBuildPod(t *testing.T) {
	version, _ := testVersionAndCodec()
	testErr := errors.New("fail")
	ndotsValue := "2"

	tests := map[string]setupBuildPodTestDef{
		"passes node selector setting": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						NodeSelector: map[string]string{
							"a-selector":       "first",
							"another-selector": "second",
						},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Equal(t, test.RunnerConfig.RunnerSettings.Kubernetes.NodeSelector, pod.Spec.NodeSelector)
			},
		},
		"uses configured credentials": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
					},
				},
			},
			PrepareFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				e.credentials = &api.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name: "job-credentials",
					},
				}
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				secrets := []api.LocalObjectReference{{Name: "job-credentials"}}
				assert.Equal(t, secrets, pod.Spec.ImagePullSecrets)
			},
		},
		"uses configured image pull secrets": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						ImagePullSecrets: []string{
							"docker-registry-credentials",
						},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				secrets := []api.LocalObjectReference{{Name: "docker-registry-credentials"}}
				assert.Equal(t, secrets, pod.Spec.ImagePullSecrets)
			},
		},
		"uses image pull secrets from service account": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						ImagePullSecrets: []string{
							"docker-registry-credentials",
						},
						UseServiceAccountImagePullSecrets: true,
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				var secrets []api.LocalObjectReference
				assert.Equal(t, secrets, pod.Spec.ImagePullSecrets)
			},
		},
		"uses default security context flags for containers": {
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				for _, c := range pod.Spec.Containers {
					assert.Empty(
						t,
						c.SecurityContext.Privileged,
						"Container security context Privileged should be empty",
					)
					assert.Nil(
						t,
						c.SecurityContext.AllowPrivilegeEscalation,
						"Container security context AllowPrivilegeEscalation should be empty",
					)
				}
			},
		},
		"configures security context flags for un-privileged containers": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Privileged:               func(b bool) *bool { return &b }(false),
						AllowPrivilegeEscalation: func(b bool) *bool { return &b }(false),
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				for _, c := range pod.Spec.Containers {
					require.NotNil(t, c.SecurityContext.Privileged)
					assert.False(t, *c.SecurityContext.Privileged)
					require.NotNil(t, c.SecurityContext.AllowPrivilegeEscalation)
					assert.False(t, *c.SecurityContext.AllowPrivilegeEscalation)
				}
			},
		},
		"configures security context flags for privileged containers": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Privileged:               func(b bool) *bool { return &b }(true),
						AllowPrivilegeEscalation: func(b bool) *bool { return &b }(true),
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				for _, c := range pod.Spec.Containers {
					require.NotNil(t, c.SecurityContext.Privileged)
					assert.True(t, *c.SecurityContext.Privileged)
					require.NotNil(t, c.SecurityContext.AllowPrivilegeEscalation)
					assert.True(t, *c.SecurityContext.AllowPrivilegeEscalation)
				}
			},
		},
		"configures helper container": {
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				hasHelper := false
				for _, c := range pod.Spec.Containers {
					if c.Name == helperContainerName {
						hasHelper = true
					}
				}
				assert.True(t, hasHelper)
			},
		},
		"uses configured helper image": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "custom/helper-image",
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				for _, c := range pod.Spec.Containers {
					if c.Name == "helper" {
						assert.Equal(t, test.RunnerConfig.RunnerSettings.Kubernetes.HelperImage, c.Image)
					}
				}
			},
		},
		"expands variables for pod labels": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						PodLabels: map[string]string{
							"test":    "label",
							"another": "label",
							"var":     "$test",
						},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				expectedLabels := map[string]string{
					"test":                      "label",
					"another":                   "label",
					"var":                       "sometestvar",
					"job.runner.gitlab.com/pod": "runner--project-0-concurrent-0",
				}
				containsLabels(t, pod.ObjectMeta.Labels, expectedLabels)
			},
			Variables: []spec.Variable{
				{Key: "test", Value: "sometestvar"},
			},
		},
		"overwrite pod labels": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						PodLabels: map[string]string{
							"test":    "label",
							"another": "label",
							"var":     "$test",
						},
						PodLabelsOverwriteAllowed: "another.*",
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				expectedLabels := map[string]string{
					"test":                      "label",
					"another":                   "newlabel",
					"var":                       "sometestvar",
					"another2":                  "sometestvar",
					"job.runner.gitlab.com/pod": "runner--project-0-concurrent-0",
				}
				containsLabels(t, pod.ObjectMeta.Labels, expectedLabels)
			},
			Variables: []spec.Variable{
				{Key: "test", Value: "sometestvar"},
				{Key: "KUBERNETES_POD_LABELS_1", Value: "another=newlabel"},
				{Key: "KUBERNETES_POD_LABELS_2", Value: "another2=$test"},
			},
		},
		"fails to set or overwrite gitlab-internal labels": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						PodLabels: map[string]string{
							"another":                             "label",
							"manager.runner.gitlab.com/foo":       "foo",
							"MANAGER.runner.gitlab.com/uppercase": "foo",
							"RuNnEr.gitlab.com/MiXeDcAsE":         "quux",
						},
						PodLabelsOverwriteAllowed: ".*",
					},
				},
			},
			Variables: []spec.Variable{
				{Key: "KUBERNETES_POD_LABELS_1", Value: "manager.runner.gitlab.com/bar=bar"},
				{Key: "KUBERNETES_POD_LABELS_2", Value: "manager.runner.gitlab.com=ohno"},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				notExpectedLabels := map[string]string{
					"manager.runner.gitlab.com/foo":       "foo",
					"manager.runner.gitlab.com/bar":       "bar",
					"manager.runner.gitlab.com":           "ohno",
					"MANAGER.runner.gitlab.com/uppercase": "foo",
					"RuNnEr.gitlab.com/MiXeDcAsE":         "quux",
				}
				notContainsLabels(t, pod.ObjectMeta.Labels, notExpectedLabels)
			},
		},
		"sets default runner labels on the pod": {
			RunnerConfig: common.RunnerConfig{
				Name: "some-runner-name",
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{},
				},
				RunnerCredentials: common.RunnerCredentials{
					Token: "glrt-aaa_cccbbbdddooo2222",
				},
			},
			PrepareFn: func(t *testing.T, _ setupBuildPodTestDef, e *executor) {
				e.Build.JobInfo.ProjectName = "some-project-name"
				e.Build.JobInfo.ProjectID = 42
			},
			Variables: []spec.Variable{
				{Key: "CI_PROJECT_NAMESPACE_ID", Value: "123"},
				{Key: "CI_PROJECT_NAMESPACE", Value: "some-namespace"},
				{Key: "CI_PROJECT_ROOT_NAMESPACE", Value: "some-root-namespace"},
			},
			VerifyFn: func(t *testing.T, _ setupBuildPodTestDef, pod *api.Pod) {
				expectedLabels := map[string]string{
					"project.runner.gitlab.com/id":             "42",
					"project.runner.gitlab.com/namespace-id":   "123",
					"project.runner.gitlab.com/name":           "some-project-name",
					"project.runner.gitlab.com/namespace":      "some-namespace",
					"project.runner.gitlab.com/root-namespace": "some-root-namespace",

					"manager.runner.gitlab.com/name":     "some-runner-name",
					"manager.runner.gitlab.com/id-short": "aaa_cccbb",
				}
				containsLabels(t, pod.ObjectMeta.Labels, expectedLabels)
			},
		},
		"expands variables for pod annotations": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						PodAnnotations: map[string]string{
							"test":    "annotation",
							"another": "annotation",
							"var":     "$test",
						},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Equal(t, map[string]string{
					"test":    "annotation",
					"another": "annotation",
					"var":     "sometestvar",

					"job.runner.gitlab.com/id":         "0",
					"job.runner.gitlab.com/url":        "/-/jobs/0",
					"job.runner.gitlab.com/sha":        "",
					"job.runner.gitlab.com/before_sha": "",
					"job.runner.gitlab.com/ref":        "",
					"job.runner.gitlab.com/name":       "",
					"job.runner.gitlab.com/timeout":    "2h0m0s",
					"project.runner.gitlab.com/id":     "0",
				}, pod.ObjectMeta.Annotations)
			},
			Variables: []spec.Variable{
				{Key: "test", Value: "sometestvar"},
			},
		},
		"default pod annotations": {
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Equal(t, map[string]string{
					"job.runner.gitlab.com/id":         "0",
					"job.runner.gitlab.com/url":        "/-/jobs/0",
					"job.runner.gitlab.com/sha":        "",
					"job.runner.gitlab.com/before_sha": "",
					"job.runner.gitlab.com/ref":        "",
					"job.runner.gitlab.com/name":       "",
					"job.runner.gitlab.com/timeout":    "2h0m0s",
					"project.runner.gitlab.com/id":     "0",
				}, pod.ObjectMeta.Annotations)
			},
		},
		"overwrite default pod annotations": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						PodAnnotations: map[string]string{
							"job.runner.gitlab.com/id":         "notARealJobID",
							"job.runner.gitlab.com/url":        "overwriteJobURL",
							"job.runner.gitlab.com/sha":        "overwriteJobSHA",
							"job.runner.gitlab.com/before_sha": "overwriteJobBeforeSHA",
							"job.runner.gitlab.com/ref":        "overwriteJobRef",
							"job.runner.gitlab.com/name":       "overwriteJobName",
							"job.runner.gitlab.com/timeout":    "overwriteJobTimeout",
							"project.runner.gitlab.com/id":     "overwriteProjectID",
						},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Equal(t, map[string]string{
					"job.runner.gitlab.com/id":         "notARealJobID",
					"job.runner.gitlab.com/url":        "overwriteJobURL",
					"job.runner.gitlab.com/sha":        "overwriteJobSHA",
					"job.runner.gitlab.com/before_sha": "overwriteJobBeforeSHA",
					"job.runner.gitlab.com/ref":        "overwriteJobRef",
					"job.runner.gitlab.com/name":       "overwriteJobName",
					"job.runner.gitlab.com/timeout":    "overwriteJobTimeout",
					"project.runner.gitlab.com/id":     "overwriteProjectID",
				}, pod.ObjectMeta.Annotations)
			},
		},
		"expands variables for helper image": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "custom/helper-image:${CI_RUNNER_REVISION}",
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				for _, c := range pod.Spec.Containers {
					if c.Name == "helper" {
						assert.Equal(t, "custom/helper-image:"+common.AppVersion.Revision, c.Image)
					}
				}
			},
		},
		"support setting kubernetes pod taint tolerations": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						NodeTolerations: map[string]string{
							"node-role.kubernetes.io/master": "NoSchedule",
							"custom.toleration=value":        "NoSchedule",
							"empty.value=":                   "PreferNoSchedule",
							"onlyKey":                        "",
							"":                               "",
						},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				expectedTolerations := []api.Toleration{
					{
						Key:      "node-role.kubernetes.io/master",
						Operator: api.TolerationOpExists,
						Effect:   api.TaintEffectNoSchedule,
					},
					{
						Key:      "custom.toleration",
						Operator: api.TolerationOpEqual,
						Value:    "value",
						Effect:   api.TaintEffectNoSchedule,
					},
					{
						Key:      "empty.value",
						Operator: api.TolerationOpEqual,
						Value:    "",
						Effect:   api.TaintEffectPreferNoSchedule,
					},
					{
						Key:      "onlyKey",
						Operator: api.TolerationOpExists,
						Effect:   "",
					},
					{
						Operator: api.TolerationOpExists,
					},
				}
				assert.ElementsMatch(t, expectedTolerations, pod.Spec.Tolerations)
			},
		},
		"support setting kubernetes pod taint tolerations via job variables": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						NodeTolerationsOverwriteAllowed: ".*",
					},
				},
			},
			Variables: []spec.Variable{
				{Key: "KUBERNETES_NODE_TOLERATIONS_1", Value: "node-role.kubernetes.io/master:NoSchedule", Public: true},
				{Key: "KUBERNETES_NODE_TOLERATIONS_2", Value: "custom.toleration=value:NoSchedule", Public: true},
				{Key: "KUBERNETES_NODE_TOLERATIONS_3", Value: "empty.value=:PreferNoSchedule", Public: true},
				{Key: "KUBERNETES_NODE_TOLERATIONS_4", Value: "onlyKey:", Public: true},
				{Key: "KUBERNETES_NODE_TOLERATIONS_5", Value: "", Public: true},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				expectedTolerations := []api.Toleration{
					{
						Key:      "node-role.kubernetes.io/master",
						Operator: api.TolerationOpExists,
						Effect:   api.TaintEffectNoSchedule,
					},
					{
						Key:      "custom.toleration",
						Operator: api.TolerationOpEqual,
						Value:    "value",
						Effect:   api.TaintEffectNoSchedule,
					},
					{
						Key:      "empty.value",
						Operator: api.TolerationOpEqual,
						Value:    "",
						Effect:   api.TaintEffectPreferNoSchedule,
					},
					{
						Key:      "onlyKey",
						Operator: api.TolerationOpExists,
						Effect:   "",
					},
					{
						Operator: api.TolerationOpExists,
					},
				}
				assert.ElementsMatch(t, expectedTolerations, pod.Spec.Tolerations)
			},
		},
		"supports extended docker configuration for image and services, FF_USE_DUMB_INIT_WITH_KUBERNETES_EXECUTOR is true": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: spec.Image{
					Name:       "test-image",
					Entrypoint: []string{"/init", "run"},
				},
				Services: map[string]*spec.Image{
					"svc-0": {
						Name:       "test-service",
						Entrypoint: []string{"/init", "run"},
						Command:    []string{"application", "--debug"},
					},
					"svc-1": {
						Name:    "test-service-2",
						Command: []string{"application", "--debug"},
					},
					"svc-2": {
						Name:    "test-service-3",
						Command: []string{"application", "--debug"},
						Variables: []spec.Variable{
							{
								Key:   "SERVICE_VAR",
								Value: "SERVICE_VAR_VALUE",
							},
							{
								Key:   "SERVICE_VAR_REF_BUILD_VAR",
								Value: "$BUILD_VAR",
							},
						},
					},
				},
			},
			Variables: []spec.Variable{
				{Key: "BUILD_VAR", Value: "BUILD_VAR_VALUE", Public: true},
				{Key: "FF_USE_DUMB_INIT_WITH_KUBERNETES_EXECUTOR", Value: "true", Public: true},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				require.Len(t, pod.Spec.Containers, 5)

				assert.Equal(t, "build", pod.Spec.Containers[0].Name)
				assert.Equal(t, "test-image", pod.Spec.Containers[0].Image)
				assert.Equal(t, pod.Spec.Containers[0].Command, []string{"/scripts-0-0/dumb-init", "--"})
				assert.Empty(t, pod.Spec.Containers[0].Args, "Build container args should be empty")

				assert.Equal(t, "helper", pod.Spec.Containers[1].Name)
				assert.Equal(t, "custom/helper-image", pod.Spec.Containers[1].Image)
				assert.Equal(t, pod.Spec.Containers[1].Command, []string{"/scripts-0-0/dumb-init", "--"})
				assert.Empty(t, pod.Spec.Containers[1].Args, "Helper container args should be empty")

				assert.Equal(t, "svc-0", pod.Spec.Containers[2].Name)
				assert.Equal(t, "test-service", pod.Spec.Containers[2].Image)
				assert.Equal(t, []string{"/init", "run"}, pod.Spec.Containers[2].Command)
				assert.Equal(t, []string{"application", "--debug"}, pod.Spec.Containers[2].Args)
				assert.NotContains(
					t, pod.Spec.Containers[2].Env,
					api.EnvVar{Name: "SERVICE_VAR", Value: "SERVICE_VAR_VALUE"},
					"Service env should NOT contain SERVICE_VAR with value VARIABLE_VALUE",
				)

				assert.Equal(t, "svc-1", pod.Spec.Containers[3].Name)
				assert.Equal(t, "test-service-2", pod.Spec.Containers[3].Image)
				assert.Empty(t, pod.Spec.Containers[3].Command, "Service container command should be empty")
				assert.Equal(t, []string{"application", "--debug"}, pod.Spec.Containers[3].Args)
				assert.NotContains(
					t, pod.Spec.Containers[3].Env,
					api.EnvVar{Name: "SERVICE_VAR", Value: "SERVICE_VAR_VALUE"},
					"Service env should NOT contain VARIABLE_NAME with value VARIABLE_VALUE",
				)

				assert.Equal(t, "svc-2", pod.Spec.Containers[4].Name)
				assert.Equal(t, "test-service-3", pod.Spec.Containers[4].Image)
				assert.Contains(
					t, pod.Spec.Containers[4].Env,
					api.EnvVar{Name: "SERVICE_VAR", Value: "SERVICE_VAR_VALUE"},
				)
				assert.Contains(
					t, pod.Spec.Containers[4].Env,
					api.EnvVar{Name: "SERVICE_VAR_REF_BUILD_VAR", Value: "BUILD_VAR_VALUE"},
				)
			},
		},
		"supports extended docker configuration for image and services, FF_USE_DUMB_INIT_WITH_KUBERNETES_EXECUTOR is false": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: spec.Image{
					Name:       "test-image",
					Entrypoint: []string{"/init", "run"},
				},
				Services: map[string]*spec.Image{
					"svc-0": {
						Name:       "test-service",
						Entrypoint: []string{"/init", "run"},
						Command:    []string{"application", "--debug"},
					},
					"svc-1": {
						Name:    "test-service-2",
						Command: []string{"application", "--debug"},
					},
					"svc-2": {
						Name:    "test-service-3",
						Command: []string{"application", "--debug"},
						Variables: []spec.Variable{
							{
								Key:   "SERVICE_VAR",
								Value: "SERVICE_VAR_VALUE",
							},
							{
								Key:   "SERVICE_VAR_REF_BUILD_VAR",
								Value: "$BUILD_VAR",
							},
						},
					},
				},
			},
			Variables: []spec.Variable{
				{Key: "BUILD_VAR", Value: "BUILD_VAR_VALUE", Public: true},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				require.Len(t, pod.Spec.Containers, 5)

				assert.Equal(t, "build", pod.Spec.Containers[0].Name)
				assert.Equal(t, "test-image", pod.Spec.Containers[0].Image)
				assert.Equal(t, []string{"/init", "run"}, pod.Spec.Containers[0].Command)
				assert.Empty(t, pod.Spec.Containers[0].Args, "Build container args should be empty")

				assert.Equal(t, "helper", pod.Spec.Containers[1].Name)
				assert.Equal(t, "custom/helper-image", pod.Spec.Containers[1].Image)
				assert.Empty(t, pod.Spec.Containers[1].Command, "Helper container command should be empty")
				assert.Empty(t, pod.Spec.Containers[1].Args, "Helper container args should be empty")

				assert.Equal(t, "svc-0", pod.Spec.Containers[2].Name)
				assert.Equal(t, "test-service", pod.Spec.Containers[2].Image)
				assert.Equal(t, []string{"/init", "run"}, pod.Spec.Containers[2].Command)
				assert.Equal(t, []string{"application", "--debug"}, pod.Spec.Containers[2].Args)
				assert.NotContains(
					t, pod.Spec.Containers[2].Env,
					api.EnvVar{Name: "SERVICE_VAR", Value: "SERVICE_VAR_VALUE"},
					"Service env should NOT contain SERVICE_VAR with value VARIABLE_VALUE",
				)

				assert.Equal(t, "svc-1", pod.Spec.Containers[3].Name)
				assert.Equal(t, "test-service-2", pod.Spec.Containers[3].Image)
				assert.Empty(t, pod.Spec.Containers[3].Command, "Service container command should be empty")
				assert.Equal(t, []string{"application", "--debug"}, pod.Spec.Containers[3].Args)
				assert.NotContains(
					t, pod.Spec.Containers[3].Env,
					api.EnvVar{Name: "SERVICE_VAR", Value: "SERVICE_VAR_VALUE"},
					"Service env should NOT contain VARIABLE_NAME with value VARIABLE_VALUE",
				)

				assert.Equal(t, "svc-2", pod.Spec.Containers[4].Name)
				assert.Equal(t, "test-service-3", pod.Spec.Containers[4].Image)
				assert.Contains(
					t, pod.Spec.Containers[4].Env,
					api.EnvVar{Name: "SERVICE_VAR", Value: "SERVICE_VAR_VALUE"},
				)
				assert.Contains(
					t, pod.Spec.Containers[4].Env,
					api.EnvVar{Name: "SERVICE_VAR_REF_BUILD_VAR", Value: "BUILD_VAR_VALUE"},
				)
			},
		},
		"creates services in kubernetes if ports are set": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: spec.Image{
					Name: "test-image",
					Ports: []spec.Port{
						{
							Number: 80,
						},
					},
				},
				Services: map[string]*spec.Image{
					"svc-0": {
						Name: "test-service",
						Ports: []spec.Port{
							{
								Number: 82,
							},
							{
								Number: 84,
							},
						},
					},
					"svc-1": {
						Name: "test-service2",
						Ports: []spec.Port{
							{
								Number: 85,
							},
						},
					},
					"svc-2": {
						Name: "test-service3",
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				ownerReferences := e.buildPodReferences()
				expectedServices := []api.Service{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "build",
							Namespace:       "default",
							OwnerReferences: ownerReferences,
						},
						Spec: api.ServiceSpec{
							Ports: []api.ServicePort{
								{
									Port:       80,
									TargetPort: intstr.FromInt32(80),
									Name:       "build-80",
								},
							},
							Selector: map[string]string{"job.runner.gitlab.com/pod": "runner--project-0-concurrent-0"},
							Type:     api.ServiceTypeClusterIP,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "proxy-svc-0",
							Namespace:       "default",
							OwnerReferences: ownerReferences,
						},
						Spec: api.ServiceSpec{
							Ports: []api.ServicePort{
								{
									Port:       82,
									TargetPort: intstr.FromInt32(82),
									Name:       "proxy-svc-0-82",
								},
								{
									Port:       84,
									TargetPort: intstr.FromInt32(84),
									Name:       "proxy-svc-0-84",
								},
							},
							Selector: map[string]string{"job.runner.gitlab.com/pod": "runner--project-0-concurrent-0"},
							Type:     api.ServiceTypeClusterIP,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:            "proxy-svc-1",
							Namespace:       "default",
							OwnerReferences: ownerReferences,
						},
						Spec: api.ServiceSpec{
							Ports: []api.ServicePort{
								{
									Port:       85,
									TargetPort: intstr.FromInt32(85),
									Name:       "proxy-svc-1-85",
								},
							},
							Selector: map[string]string{"job.runner.gitlab.com/pod": "runner--project-0-concurrent-0"},
							Type:     api.ServiceTypeClusterIP,
						},
					},
				}

				// The name of each service is generated beforehand
				// We reset it to empty string before the assert
				// Everything else should be as expected.
				srvs := make([]api.Service, 0)
				for _, s := range e.services {
					s.ObjectMeta.Name = s.ObjectMeta.Name[:len(s.ObjectMeta.Name)-k8sResourcesNameSuffixLength-1]
					srvs = append(srvs, *s.DeepCopy())
				}

				assert.ElementsMatch(t, expectedServices, srvs)
			},
		},
		"the default service name for the build container is build": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: spec.Image{
					Name: "test-image",
					Ports: []spec.Port{
						{
							Number: 80,
						},
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				assert.Equal(
					t, "build",
					e.services[0].GetName()[:len(e.services[0].GetName())-k8sResourcesNameSuffixLength-1],
				)
			},
		},
		"the services have a selector pointing to the pod label in the pod": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: spec.Image{
					Name: "test-image",
					Ports: []spec.Port{
						{
							Number: 80,
						},
					},
				},
				Services: map[string]*spec.Image{
					"svc-0": {
						Name: "test-service",
						Ports: []spec.Port{
							{
								Number: 82,
							},
						},
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				for _, service := range e.services {
					assert.Equal(t, map[string]string{"job.runner.gitlab.com/pod": "runner--project-0-concurrent-0"}, service.Spec.Selector)
				}
			},
		},
		"the service is named as the alias if set": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: spec.Image{
					Name: "test-image",
				},
				Services: map[string]*spec.Image{
					"custom-name": {
						Name:  "test-service",
						Alias: "custom-name",
						Ports: []spec.Port{
							{
								Number: 82,
							},
						},
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				assert.Equal(
					t, "custom-name",
					e.services[0].GetName()[:len(e.services[0].GetName())-k8sResourcesNameSuffixLength-1],
				)
			},
		},
		"proxies are configured if services have been created": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: spec.Image{
					Name: "test-image",
					Ports: []spec.Port{
						{
							Number: 80,
						},
					},
				},
				Services: map[string]*spec.Image{
					"svc-0": {
						Name:  "test-service",
						Alias: "custom_name",
						Ports: []spec.Port{
							{
								Number:   81,
								Name:     "custom_port_name",
								Protocol: "http",
							},
						},
					},
					"svc-1": {
						Name: "test-service2",
						Ports: []spec.Port{
							{
								Number: 82,
							},
						},
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				require.Len(t, e.ProxyPool, 3)

				assert.NotEmpty(t, "proxy-svc-1", e.ProxyPool)
				assert.NotEmpty(t, "custom_name", e.ProxyPool)
				assert.NotEmpty(t, "build", e.ProxyPool)

				port := e.ProxyPool["proxy-svc-1"].Settings.Ports[0]
				assert.Equal(t, 82, port.Number)

				port = e.ProxyPool["custom_name"].Settings.Ports[0]
				assert.Equal(t, 81, port.Number)
				assert.Equal(t, "custom_port_name", port.Name)
				assert.Equal(t, "http", port.Protocol)

				port = e.ProxyPool["build"].Settings.Ports[0]
				assert.Equal(t, 80, port.Number)
			},
		},
		"makes service name compatible with RFC1123": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: spec.Image{
					Name: "test-image",
				},
				Services: map[string]*spec.Image{
					"service": {
						Name:  "test-service",
						Alias: "service,name-.non-compat!ble",
						Ports: []spec.Port{
							{
								Number:   81,
								Name:     "port,name-.non-compat!ble",
								Protocol: "http",
							},
						},
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				sort.Slice(e.services, func(i, j int) bool {
					return e.services[i].GetName() > e.services[j].GetName()
				})
				assert.Equal(
					t, "service",
					e.services[0].GetName()[:len(e.services[0].GetName())-k8sResourcesNameSuffixLength-1],
				)
				assert.Equal(
					t, "name-non-compatble",
					e.services[1].GetName()[:len(e.services[1].GetName())-k8sResourcesNameSuffixLength-1],
				)

				assert.NotEmpty(t, e.ProxyPool["service"])
				assert.NotEmpty(t, e.ProxyPool["name-.non-compat!ble"])
				assert.Equal(
					t,
					"port,name-.non-compat!ble",
					e.ProxyPool["name-.non-compat!ble"].Settings.Ports[0].Name,
				)

				assert.Equal(
					t,
					"port,name-.non-compat!ble",
					e.ProxyPool["service"].Settings.Ports[0].Name,
				)
			},
		},
		"sets command (entrypoint) and args, FF_USE_DUMB_INIT_WITH_KUBERNETES_EXECUTOR is true": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: spec.Image{
					Name: "test-image",
				},
				Services: map[string]*spec.Image{
					"svc-0": {
						Name:    "test-service-0",
						Command: []string{"application", "--debug"},
					},
					"svc-1": {
						Name:       "test-service-1",
						Entrypoint: []string{"application", "--debug"},
					},
					"svc-2": {
						Name:       "test-service-2",
						Entrypoint: []string{"application", "--debug"},
						Command:    []string{"argument1", "argument2"},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				require.Len(t, pod.Spec.Containers, 5)

				assert.Equal(t, "build", pod.Spec.Containers[0].Name)
				assert.Equal(t, "test-image", pod.Spec.Containers[0].Image)
				assert.Equal(t, pod.Spec.Containers[0].Command, []string{"/scripts-0-0/dumb-init", "--"})
				assert.Empty(t, pod.Spec.Containers[0].Args, "Build container args should be empty")

				assert.Equal(t, "helper", pod.Spec.Containers[1].Name)
				assert.Equal(t, "custom/helper-image", pod.Spec.Containers[1].Image)
				assert.Equal(t, pod.Spec.Containers[1].Command, []string{"/scripts-0-0/dumb-init", "--"})
				assert.Empty(t, pod.Spec.Containers[1].Args, "Helper container args should be empty")

				assert.Equal(t, "svc-0", pod.Spec.Containers[2].Name)
				assert.Equal(t, "test-service-0", pod.Spec.Containers[2].Image)
				assert.Empty(t, pod.Spec.Containers[2].Command, "Service container command should be empty")
				assert.Equal(t, []string{"application", "--debug"}, pod.Spec.Containers[2].Args)

				assert.Equal(t, "svc-1", pod.Spec.Containers[3].Name)
				assert.Equal(t, "test-service-1", pod.Spec.Containers[3].Image)
				assert.Equal(t, []string{"application", "--debug"}, pod.Spec.Containers[3].Command)
				assert.Empty(t, pod.Spec.Containers[3].Args, "Service container args should be empty")

				assert.Equal(t, "svc-2", pod.Spec.Containers[4].Name)
				assert.Equal(t, "test-service-2", pod.Spec.Containers[4].Image)
				assert.Equal(t, []string{"application", "--debug"}, pod.Spec.Containers[4].Command)
				assert.Equal(t, []string{"argument1", "argument2"}, pod.Spec.Containers[4].Args)
			},
			Variables: []spec.Variable{
				{Key: "FF_USE_DUMB_INIT_WITH_KUBERNETES_EXECUTOR", Value: "true", Public: true},
			},
		},
		"sets command (entrypoint) and args, FF_USE_DUMB_INIT_WITH_KUBERNETES_EXECUTOR is false": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: spec.Image{
					Name: "test-image",
				},
				Services: map[string]*spec.Image{
					"svc-0": {
						Name:    "test-service-0",
						Command: []string{"application", "--debug"},
					},
					"svc-1": {
						Name:       "test-service-1",
						Entrypoint: []string{"application", "--debug"},
					},
					"svc-2": {
						Name:       "test-service-2",
						Entrypoint: []string{"application", "--debug"},
						Command:    []string{"argument1", "argument2"},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				require.Len(t, pod.Spec.Containers, 5)

				assert.Equal(t, "build", pod.Spec.Containers[0].Name)
				assert.Equal(t, "test-image", pod.Spec.Containers[0].Image)
				assert.Empty(t, pod.Spec.Containers[0].Command, "Build container args should be empty")
				assert.Empty(t, pod.Spec.Containers[0].Args, "Build container args should be empty")

				assert.Equal(t, "helper", pod.Spec.Containers[1].Name)
				assert.Equal(t, "custom/helper-image", pod.Spec.Containers[1].Image)
				assert.Empty(t, pod.Spec.Containers[1].Command, "Helper container command should be empty")
				assert.Empty(t, pod.Spec.Containers[1].Args, "Helper container args should be empty")

				assert.Equal(t, "svc-0", pod.Spec.Containers[2].Name)
				assert.Equal(t, "test-service-0", pod.Spec.Containers[2].Image)
				assert.Empty(t, pod.Spec.Containers[2].Command, "Service container command should be empty")
				assert.Equal(t, []string{"application", "--debug"}, pod.Spec.Containers[2].Args)

				assert.Equal(t, "svc-1", pod.Spec.Containers[3].Name)
				assert.Equal(t, "test-service-1", pod.Spec.Containers[3].Image)
				assert.Equal(t, []string{"application", "--debug"}, pod.Spec.Containers[3].Command)
				assert.Empty(t, pod.Spec.Containers[3].Args, "Service container args should be empty")

				assert.Equal(t, "svc-2", pod.Spec.Containers[4].Name)
				assert.Equal(t, "test-service-2", pod.Spec.Containers[4].Image)
				assert.Equal(t, []string{"application", "--debug"}, pod.Spec.Containers[4].Command)
				assert.Equal(t, []string{"argument1", "argument2"}, pod.Spec.Containers[4].Args)
			},
		},
		"non-DNS-1123-compatible-token": {
			RunnerConfig: common.RunnerConfig{
				RunnerCredentials: common.RunnerCredentials{
					Token: "ToK3_?OF",
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				dns_test.AssertRFC1123Compatibility(t, pod.GetName())
			},
		},
		"supports pod security context": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						PodSecurityContext: common.KubernetesPodSecurityContext{
							FSGroup:            func() *int64 { i := int64(200); return &i }(),
							RunAsGroup:         func() *int64 { i := int64(200); return &i }(),
							RunAsNonRoot:       func() *bool { i := true; return &i }(),
							RunAsUser:          func() *int64 { i := int64(200); return &i }(),
							SupplementalGroups: []int64{200},
							SELinuxType:        "spc_t",
						},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Equal(t, int64(200), *pod.Spec.SecurityContext.FSGroup)
				assert.Equal(t, int64(200), *pod.Spec.SecurityContext.RunAsGroup)
				assert.Equal(t, int64(200), *pod.Spec.SecurityContext.RunAsUser)
				assert.Equal(t, true, *pod.Spec.SecurityContext.RunAsNonRoot)
				assert.Equal(t, []int64{200}, pod.Spec.SecurityContext.SupplementalGroups)
				assert.Equal(t, "spc_t", pod.Spec.SecurityContext.SELinuxOptions.Type)
			},
		},
		"uses default security context when unspecified": {
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Empty(t, pod.Spec.SecurityContext, "Security context should be empty")
			},
		},
		"supports pod node affinities": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Affinity: common.KubernetesAffinity{
							NodeAffinity: &common.KubernetesNodeAffinity{
								PreferredDuringSchedulingIgnoredDuringExecution: []common.PreferredSchedulingTerm{
									{
										Weight: 100,
										Preference: common.NodeSelectorTerm{
											MatchExpressions: []common.NodeSelectorRequirement{
												{
													Key:      "cpu_speed",
													Operator: "In",
													Values:   []string{"fast"},
												},
											},
											MatchFields: []common.NodeSelectorRequirement{
												{
													Key:      "cpu_count",
													Operator: "Gt",
													Values:   []string{"12"},
												},
											},
										},
									},
									{
										Weight: 50,
										Preference: common.NodeSelectorTerm{
											MatchExpressions: []common.NodeSelectorRequirement{
												{
													Key:      "kubernetes.io/e2e-az-name",
													Operator: "In",
													Values:   []string{"e2e-az1", "e2e-az2"},
												},
												{
													Key:      "kubernetes.io/arch",
													Operator: "NotIn",
													Values:   []string{"arm"},
												},
											},
										},
									},
								},
								RequiredDuringSchedulingIgnoredDuringExecution: &common.NodeSelector{
									NodeSelectorTerms: []common.NodeSelectorTerm{
										{
											MatchExpressions: []common.NodeSelectorRequirement{
												{
													Key:      "kubernetes.io/e2e-az-name",
													Operator: "In",
													Values:   []string{"e2e-az1", "e2e-az2"},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				require.NotNil(t, pod.Spec.Affinity)
				require.NotNil(t, pod.Spec.Affinity.NodeAffinity)

				nodeAffinity := pod.Spec.Affinity.NodeAffinity
				preferredNodeAffinity := nodeAffinity.PreferredDuringSchedulingIgnoredDuringExecution

				require.Len(t, preferredNodeAffinity, 2)
				assert.Equal(t, int32(100), preferredNodeAffinity[0].Weight)
				require.Len(t, preferredNodeAffinity[0].Preference.MatchExpressions, 1)
				require.Len(t, preferredNodeAffinity[0].Preference.MatchFields, 1)
				assert.Equal(t, "cpu_speed", preferredNodeAffinity[0].Preference.MatchExpressions[0].Key)
				assert.Equal(t, api.NodeSelectorOperator("In"), preferredNodeAffinity[0].Preference.MatchExpressions[0].Operator)
				assert.Equal(t, []string{"fast"}, preferredNodeAffinity[0].Preference.MatchExpressions[0].Values)
				assert.Equal(t, "cpu_count", preferredNodeAffinity[0].Preference.MatchFields[0].Key)
				assert.Equal(t, api.NodeSelectorOperator("Gt"), preferredNodeAffinity[0].Preference.MatchFields[0].Operator)
				assert.Equal(t, []string{"12"}, preferredNodeAffinity[0].Preference.MatchFields[0].Values)

				assert.Equal(t, int32(50), preferredNodeAffinity[1].Weight)
				require.Len(t, preferredNodeAffinity[1].Preference.MatchExpressions, 2)
				assert.Equal(t, "kubernetes.io/e2e-az-name", preferredNodeAffinity[1].Preference.MatchExpressions[0].Key)
				assert.Equal(t, api.NodeSelectorOperator("In"), preferredNodeAffinity[1].Preference.MatchExpressions[0].Operator)
				assert.Equal(t, []string{"e2e-az1", "e2e-az2"}, preferredNodeAffinity[1].Preference.MatchExpressions[0].Values)
				assert.Equal(t, "kubernetes.io/arch", preferredNodeAffinity[1].Preference.MatchExpressions[1].Key)
				assert.Equal(t, api.NodeSelectorOperator("NotIn"), preferredNodeAffinity[1].Preference.MatchExpressions[1].Operator)
				assert.Equal(t, []string{"arm"}, preferredNodeAffinity[1].Preference.MatchExpressions[1].Values)

				require.NotNil(t, nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution)
				requiredNodeAffinity := nodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution

				require.Len(t, requiredNodeAffinity.NodeSelectorTerms, 1)
				require.Len(t, requiredNodeAffinity.NodeSelectorTerms[0].MatchExpressions, 1)
				require.Len(t, requiredNodeAffinity.NodeSelectorTerms[0].MatchFields, 0)
				assert.Equal(t, "kubernetes.io/e2e-az-name", requiredNodeAffinity.NodeSelectorTerms[0].MatchExpressions[0].Key)
				assert.Equal(t, api.NodeSelectorOperator("In"), requiredNodeAffinity.NodeSelectorTerms[0].MatchExpressions[0].Operator)
				assert.Equal(t, []string{"e2e-az1", "e2e-az2"}, requiredNodeAffinity.NodeSelectorTerms[0].MatchExpressions[0].Values)
			},
		},
		"supports pod affinities": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Affinity: common.KubernetesAffinity{
							PodAffinity: &common.KubernetesPodAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []common.PodAffinityTerm{
									{
										LabelSelector: &common.LabelSelector{
											MatchLabels: map[string]string{"key": "value"},
											MatchExpressions: []common.NodeSelectorRequirement{
												{
													Key:      "cores",
													Operator: "In",
													Values:   []string{"many", "high_count"},
												},
											},
										},
										Namespaces:  []string{"namespace_1", "namespace_2"},
										TopologyKey: "topo_key",
										NamespaceSelector: &common.LabelSelector{
											MatchLabels: map[string]string{"key": "value"},
											MatchExpressions: []common.NodeSelectorRequirement{
												{
													Key:      "cores",
													Operator: "In",
													Values:   []string{"many", "high_count"},
												},
											},
										},
									},
								},
								PreferredDuringSchedulingIgnoredDuringExecution: nil,
							},
						},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				require.NotNil(t, pod.Spec.Affinity)
				require.NotNil(t, pod.Spec.Affinity.PodAffinity)

				podAffinity := pod.Spec.Affinity.PodAffinity
				require.Len(t, podAffinity.RequiredDuringSchedulingIgnoredDuringExecution, 1)
				preferredNodeAffinity := podAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0]

				assert.Equal(t, []string{"namespace_1", "namespace_2"}, preferredNodeAffinity.Namespaces)
				assert.Equal(t, "topo_key", preferredNodeAffinity.TopologyKey)

				require.NotNil(t, preferredNodeAffinity.LabelSelector)
				assert.Equal(t, map[string]string{"key": "value"}, preferredNodeAffinity.LabelSelector.MatchLabels)
				require.Len(t, preferredNodeAffinity.LabelSelector.MatchExpressions, 1)
				preferredMatchExp := preferredNodeAffinity.LabelSelector.MatchExpressions
				assert.Equal(t, "cores", preferredMatchExp[0].Key)
				assert.Equal(t, metav1.LabelSelectorOperator("In"), preferredMatchExp[0].Operator)
				assert.Equal(t, []string{"many", "high_count"}, preferredMatchExp[0].Values)

				require.NotNil(t, preferredNodeAffinity.NamespaceSelector)
				assert.Equal(t, map[string]string{"key": "value"}, preferredNodeAffinity.NamespaceSelector.MatchLabels)
				require.Len(t, preferredNodeAffinity.NamespaceSelector.MatchExpressions, 1)
				preferredMatchExp = preferredNodeAffinity.NamespaceSelector.MatchExpressions
				assert.Equal(t, "cores", preferredMatchExp[0].Key)
				assert.Equal(t, metav1.LabelSelectorOperator("In"), preferredMatchExp[0].Operator)
				assert.Equal(t, []string{"many", "high_count"}, preferredMatchExp[0].Values)
			},
		},
		"supports pod anti-affinities": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Affinity: common.KubernetesAffinity{
							PodAntiAffinity: &common.KubernetesPodAntiAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []common.PodAffinityTerm{
									{
										LabelSelector: &common.LabelSelector{
											MatchLabels: map[string]string{"key": "value"},
											MatchExpressions: []common.NodeSelectorRequirement{
												{
													Key:      "cores",
													Operator: "In",
													Values:   []string{"many", "high_count"},
												},
											},
										},
										Namespaces:  []string{"namespace_1", "namespace_2"},
										TopologyKey: "topo_key",
										NamespaceSelector: &common.LabelSelector{
											MatchLabels: map[string]string{"key": "value"},
											MatchExpressions: []common.NodeSelectorRequirement{
												{
													Key:      "cores",
													Operator: "In",
													Values:   []string{"many", "high_count"},
												},
											},
										},
									},
								},
								PreferredDuringSchedulingIgnoredDuringExecution: nil,
							},
						},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				require.NotNil(t, pod.Spec.Affinity)
				require.NotNil(t, pod.Spec.Affinity.PodAntiAffinity)

				podAntiAffinity := pod.Spec.Affinity.PodAntiAffinity
				require.Len(t, podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution, 1)
				preferredNodeAffinity := podAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution[0]

				assert.Equal(t, []string{"namespace_1", "namespace_2"}, preferredNodeAffinity.Namespaces)
				assert.Equal(t, "topo_key", preferredNodeAffinity.TopologyKey)

				require.NotNil(t, preferredNodeAffinity.LabelSelector)
				assert.Equal(t, map[string]string{"key": "value"}, preferredNodeAffinity.LabelSelector.MatchLabels)
				require.Len(t, preferredNodeAffinity.LabelSelector.MatchExpressions, 1)
				preferredMatchExp := preferredNodeAffinity.LabelSelector.MatchExpressions
				assert.Equal(t, "cores", preferredMatchExp[0].Key)
				assert.Equal(t, metav1.LabelSelectorOperator("In"), preferredMatchExp[0].Operator)
				assert.Equal(t, []string{"many", "high_count"}, preferredMatchExp[0].Values)

				require.NotNil(t, preferredNodeAffinity.NamespaceSelector)
				assert.Equal(t, map[string]string{"key": "value"}, preferredNodeAffinity.NamespaceSelector.MatchLabels)
				require.Len(t, preferredNodeAffinity.NamespaceSelector.MatchExpressions, 1)
				preferredMatchExp = preferredNodeAffinity.NamespaceSelector.MatchExpressions
				assert.Equal(t, "cores", preferredMatchExp[0].Key)
				assert.Equal(t, metav1.LabelSelectorOperator("In"), preferredMatchExp[0].Operator)
				assert.Equal(t, []string{"many", "high_count"}, preferredMatchExp[0].Values)
			},
		},
		"supports services and setting extra hosts using HostAliases": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HostAliases: []common.KubernetesHostAliases{
							{
								IP:        "127.0.0.1",
								Hostnames: []string{"redis"},
							},
							{
								IP:        "8.8.8.8",
								Hostnames: []string{"dns1", "dns2"},
							},
						},
					},
				},
			},
			Options: &kubernetesOptions{
				Services: map[string]*spec.Image{
					"svc-alias": {
						Name:  "test-service",
						Alias: "svc-alias",
					},
					"svc-0": {
						Name: "docker:dind",
					},
					"svc-1": {
						Name: "service-with-port:dind",
						Ports: []spec.Port{{
							Number:   0,
							Protocol: "",
							Name:     "",
						}},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				// the second time this fn is called is to create the proxy service
				if pod.Kind == "Service" {
					return
				}

				require.Len(t, pod.Spec.HostAliases, 3)
				assert.Equal(t, []api.HostAlias{
					{
						IP:        "127.0.0.1",
						Hostnames: []string{"docker", "test-service", "svc-alias"},
					},
					{
						IP:        "127.0.0.1",
						Hostnames: []string{"redis"},
					},
					{
						IP:        "8.8.8.8",
						Hostnames: []string{"dns1", "dns2"},
					},
				}, pod.Spec.HostAliases)
			},
		},
		"ignores non RFC1123 aliases": {
			Options: &kubernetesOptions{
				Services: map[string]*spec.Image{
					"svc-0": {
						Name:  "test-service",
						Alias: "INVALID_ALIAS",
					},
					"svc-1": {
						Name: "docker:dind",
					},
				},
			},
			VerifySetupBuildPodErrFn: func(t *testing.T, err error) {
				var expected *invalidHostAliasDNSError
				assert.ErrorAs(t, err, &expected)
				assert.True(t, expected.Is(err))
				errMsg := err.Error()
				assert.Contains(t, errMsg, "is invalid DNS")
				assert.Contains(t, errMsg, "INVALID_ALIAS")
				assert.Contains(t, errMsg, "test-service")
			},
		},
		"no host aliases when feature is not supported in kubernetes": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HostAliases: []common.KubernetesHostAliases{
							{
								IP:        "127.0.0.1",
								Hostnames: []string{"redis"},
							},
							{
								IP:        "8.8.8.8",
								Hostnames: []string{"google"},
							},
						},
					},
				},
			},
			Options: &kubernetesOptions{
				Services: map[string]*spec.Image{
					"svc-0": {
						Name:  "test-service",
						Alias: "alias",
					},
				},
			},
			PrepareFn: func(t *testing.T, def setupBuildPodTestDef, e *executor) {
				mockFc := newMockFeatureChecker(t)
				mockFc.On("IsHostAliasSupported").Return(false, nil)
				e.featureChecker = mockFc
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Len(t, pod.Spec.Containers, 3)
				assert.Nil(t, pod.Spec.HostAliases)
			},
		},
		"check host aliases with non kubernetes version error": {
			Options: &kubernetesOptions{
				Services: map[string]*spec.Image{
					"svc-0": {
						Name:  "test-service",
						Alias: "alias",
					},
				},
			},
			PrepareFn: func(t *testing.T, def setupBuildPodTestDef, e *executor) {
				mockFc := newMockFeatureChecker(t)
				mockFc.On("IsHostAliasSupported").Return(false, testErr)
				e.featureChecker = mockFc
			},
			VerifySetupBuildPodErrFn: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, testErr)
			},
		},
		"check host aliases with kubernetes version error": {
			Options: &kubernetesOptions{
				Services: map[string]*spec.Image{
					"svc-0": {
						Name:  "test-service",
						Alias: "alias",
					},
				},
			},
			PrepareFn: func(t *testing.T, def setupBuildPodTestDef, e *executor) {
				mockFc := newMockFeatureChecker(t)
				mockFc.On("IsHostAliasSupported").Return(false, &badVersionError{})
				e.featureChecker = mockFc
			},
			VerifySetupBuildPodErrFn: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		"no init container defined": {
			InitContainers: []api.Container{},
			VerifyFn: func(t *testing.T, def setupBuildPodTestDef, pod *api.Pod) {
				assert.Nil(t, pod.Spec.InitContainers)
			},
		},
		"init container defined": {
			InitContainers: []api.Container{
				{
					Name:  "a-init-container",
					Image: "alpine",
				},
			},
			VerifyFn: func(t *testing.T, def setupBuildPodTestDef, pod *api.Pod) {
				require.Equal(t, def.InitContainers, pod.Spec.InitContainers)
			},
		},
		"support setting linux capabilities": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						CapAdd:  []string{"CAP_1", "CAP_2"},
						CapDrop: []string{"CAP_3", "CAP_4"},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				require.NotEmpty(t, pod.Spec.Containers)
				capabilities := pod.Spec.Containers[0].SecurityContext.Capabilities
				require.NotNil(t, capabilities)
				assert.Len(t, capabilities.Add, 2)
				assert.Contains(t, capabilities.Add, api.Capability("CAP_1"))
				assert.Contains(t, capabilities.Add, api.Capability("CAP_2"))
				assert.Len(t, capabilities.Drop, 3)
				assert.Contains(t, capabilities.Drop, api.Capability("CAP_3"))
				assert.Contains(t, capabilities.Drop, api.Capability("CAP_4"))
				assert.Contains(t, capabilities.Drop, api.Capability("NET_RAW"))
			},
		},
		"setting linux capabilities overriding defaults": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						CapAdd: []string{"NET_RAW", "CAP_2"},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				require.NotEmpty(t, pod.Spec.Containers)
				capabilities := pod.Spec.Containers[0].SecurityContext.Capabilities
				require.NotNil(t, capabilities)
				assert.Len(t, capabilities.Add, 2)
				assert.Contains(t, capabilities.Add, api.Capability("NET_RAW"))
				assert.Contains(t, capabilities.Add, api.Capability("CAP_2"))
				assert.Empty(t, capabilities.Drop)
			},
		},
		"setting same linux capabilities, drop wins": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						CapAdd:  []string{"CAP_1"},
						CapDrop: []string{"CAP_1"},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				require.NotEmpty(t, pod.Spec.Containers)
				capabilities := pod.Spec.Containers[0].SecurityContext.Capabilities
				require.NotNil(t, capabilities)
				assert.Empty(t, capabilities.Add)
				assert.Len(t, capabilities.Drop, 2)
				assert.Contains(t, capabilities.Drop, api.Capability("NET_RAW"))
				assert.Contains(t, capabilities.Drop, api.Capability("CAP_1"))
			},
		},
		"support setting linux capabilities on all containers": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						CapAdd:  []string{"CAP_1"},
						CapDrop: []string{"CAP_2"},
					},
				},
			},
			Options: &kubernetesOptions{
				Services: map[string]*spec.Image{
					"svc-0": {
						Name:    "test-service-0",
						Command: []string{"application", "--debug"},
					},
					"svc-1": {
						Name:       "test-service-1",
						Entrypoint: []string{"application", "--debug"},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				require.Len(t, pod.Spec.Containers, 4)

				assertContainerCap := func(container api.Container) {
					t.Run("container-"+container.Name, func(t *testing.T) {
						capabilities := container.SecurityContext.Capabilities
						require.NotNil(t, capabilities)
						assert.Len(t, capabilities.Add, 1)
						assert.Contains(t, capabilities.Add, api.Capability("CAP_1"))
						assert.Len(t, capabilities.Drop, 2)
						assert.Contains(t, capabilities.Drop, api.Capability("CAP_2"))
						assert.Contains(t, capabilities.Drop, api.Capability("NET_RAW"))
					})
				}

				assertContainerCap(pod.Spec.Containers[0])
				assertContainerCap(pod.Spec.Containers[1])
				assertContainerCap(pod.Spec.Containers[2])
				assertContainerCap(pod.Spec.Containers[3])
			},
		},
		"support setting DNS policy to empty string": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						DNSPolicy: "",
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Equal(t, api.DNSClusterFirst, pod.Spec.DNSPolicy)
				assert.Nil(t, pod.Spec.DNSConfig)
			},
		},
		"support setting DNS policy to none": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						DNSPolicy: common.DNSPolicyNone,
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Equal(t, api.DNSNone, pod.Spec.DNSPolicy)
			},
		},
		"support setting DNS policy to default": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						DNSPolicy: common.DNSPolicyDefault,
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Equal(t, api.DNSDefault, pod.Spec.DNSPolicy)
				assert.Nil(t, pod.Spec.DNSConfig)
			},
		},
		"support setting DNS policy to cluster-first": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						DNSPolicy: common.DNSPolicyClusterFirst,
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Equal(t, api.DNSClusterFirst, pod.Spec.DNSPolicy)
				assert.Nil(t, pod.Spec.DNSConfig)
			},
		},
		"support setting DNS policy to cluster-first-with-host-net": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						DNSPolicy: common.DNSPolicyClusterFirstWithHostNet,
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Equal(t, api.DNSClusterFirstWithHostNet, pod.Spec.DNSPolicy)
				assert.Nil(t, pod.Spec.DNSConfig)
			},
		},
		"fail setting DNS policy to invalid value": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						DNSPolicy: "some-invalid-policy",
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Empty(t, pod.Spec.DNSPolicy)
				assert.Nil(t, pod.Spec.DNSConfig)
			},
		},
		"support setting pod DNS config": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						DNSConfig: common.KubernetesDNSConfig{
							Nameservers: []string{"1.2.3.4"},
							Searches:    []string{"ns1.svc.cluster-domain.example", "my.dns.search.suffix"},
							Options: []common.KubernetesDNSConfigOption{
								{Name: "ndots", Value: &ndotsValue},
								{Name: "edns0"},
							},
						},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				require.NotNil(t, pod.Spec.DNSConfig)

				assert.Equal(t, []string{"1.2.3.4"}, pod.Spec.DNSConfig.Nameservers)
				assert.Equal(
					t,
					[]string{
						"ns1.svc.cluster-domain.example",
						"my.dns.search.suffix",
					},
					pod.Spec.DNSConfig.Searches,
				)

				options := pod.Spec.DNSConfig.Options
				require.Len(t, options, 2)
				assert.Equal(t, "ndots", options[0].Name)
				assert.Equal(t, "edns0", options[1].Name)
				require.NotNil(t, options[0].Value)
				assert.Equal(t, ndotsValue, *options[0].Value)
				assert.Nil(t, options[1].Value)
			},
		},
		"windows mode has no default capabilities": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
					},
				},
			},
			PrepareFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				e.helperImageInfo.OSType = helperimage.OSTypeWindows
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				require.NotEmpty(t, pod.Spec.Containers)
				require.Nil(t, pod.Spec.Containers[0].SecurityContext.Capabilities)
			},
		},
		"supports adding ownerReferences to a created service": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: spec.Image{
					Name: "test-image",
					Ports: []spec.Port{
						{
							Number: 80,
						},
					},
				},
				Services: map[string]*spec.Image{
					"svc-0": {
						Name: "test-service",
						Ports: []spec.Port{
							{
								Number: 82,
							},
							{
								Number: 84,
							},
						},
					},
					"svc-1": {
						Name: "test-service2",
						Ports: []spec.Port{
							{
								Number: 85,
							},
						},
					},
					"svc-2": {
						Name: "test-service3",
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				require.Len(t, e.services[0].OwnerReferences, 1)

				ownerReference := e.services[0].OwnerReferences[0]
				assert.Equal(t, apiVersion, ownerReference.APIVersion)
				assert.Equal(t, ownerReferenceKind, ownerReference.Kind)
				assert.Equal(t, e.pod.GetName(), ownerReference.Name)
				assert.Equal(t, e.pod.GetUID(), ownerReference.UID)
			},
		},
		"supports adding ownerReferences to a credentials": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "custom/helper-image",
					},
				},
			},
			Credentials: []spec.Credentials{
				{
					Type:     "registry",
					URL:      "http://example.com",
					Username: "user",
					Password: "password",
				},
			},
			Options: &kubernetesOptions{
				Image: spec.Image{
					Name: "test-image",
					Ports: []spec.Port{
						{
							Number: 80,
						},
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				require.Len(t, e.credentials.OwnerReferences, 1)

				ownerReference := e.credentials.OwnerReferences[0]
				assert.Equal(t, "v1", ownerReference.APIVersion)
				assert.Equal(t, "Pod", ownerReference.Kind)
				assert.Equal(t, e.pod.GetName(), ownerReference.Name)
				assert.Equal(t, e.pod.GetUID(), ownerReference.UID)
			},
		},
		"supports failure to set owner-dependent relationship": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "custom/helper-image",
					},
				},
			},
			Credentials: []spec.Credentials{
				{
					Type:     "registry",
					URL:      "http://example.com",
					Username: "user",
					Password: "password",
				},
			},
			Options: &kubernetesOptions{
				Image: spec.Image{
					Name: "test-image",
					Ports: []spec.Port{
						{
							Number: 80,
						},
					},
				},
			},
			SetHTTPPutResponse: func() (*http.Response, error) {
				return nil, errors.New("cannot set owner-dependent relationship")
			},
			VerifySetupBuildPodErrFn: func(t *testing.T, err error) {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "error setting ownerReferences")
				assert.Contains(t, err.Error(), "cannot set owner-dependent relationship")
			},
		},
		"supports TerminationGracePeriodSeconds through PodTerminationGracePeriodSeconds": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						PodTerminationGracePeriodSeconds: common.Int64Ptr(10),
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				assert.EqualValues(
					t,
					test.RunnerConfig.Kubernetes.PodTerminationGracePeriodSeconds,
					e.pod.Spec.TerminationGracePeriodSeconds,
				)
			},
		},
		"supports runtimeClass": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						RuntimeClassName: func() *string {
							runtimeClassName := "testRunTimeClass"
							return &runtimeClassName
						}(),
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				assert.EqualValues(
					t,
					*test.RunnerConfig.Kubernetes.RuntimeClassName,
					*e.pod.Spec.RuntimeClassName,
				)
			},
		},
		"no runtimeClass when not specified": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				assert.Nil(t, e.pod.Spec.RuntimeClassName)
			},
		},
		"service account and pull image secret set": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						ServiceAccount:                       "my-serviceaccount",
						ImagePullSecrets:                     []string{"my-secret1"},
						ResourceAvailabilityCheckMaxAttempts: 2,
					},
				},
			},
		},
		"resources checking disabled": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						ServiceAccount:                       "my-serviceaccount",
						ImagePullSecrets:                     []string{"my-secret1"},
						ResourceAvailabilityCheckMaxAttempts: 0,
					},
				},
			},
		},
		"support setting Pod Priority Class": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace:         "default",
						PriorityClassName: "priority-1",
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Equal(t, "priority-1", pod.Spec.PriorityClassName)
			},
		},
		"support setting Scheduler Name": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace:     "default",
						SchedulerName: "foobar",
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Equal(t, "foobar", pod.Spec.SchedulerName)
			},
		},
		"add custom podSpec": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Environment: []string{"FF_USE_ADVANCED_POD_SPEC_CONFIGURATION=true"},
					Kubernetes: &common.KubernetesConfig{
						Namespace:      "default",
						SchedulerName:  "foobar",
						ServiceAccount: "my-service-account",
						PodSpec: []common.KubernetesPodSpec{
							{
								Patch:     `serviceAccountName: null`,
								PatchType: common.PatchTypeMergePatchType,
							},
							{
								Patch:     `[{"op": "add", "path": "/nodeSelector", "value": { key1: "val1" }}]`,
								PatchType: common.PatchTypeJSONPatchType,
							},
							{
								Patch: `
containers:
  - name: "new-container"
`,
								PatchType: common.PatchTypeStrategicMergePatchType,
							},
						},
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Equal(t, "", pod.Spec.ServiceAccountName)
				assert.NotNil(t, pod.Spec.NodeSelector["key1"])
				assert.Equal(t, "val1", pod.Spec.NodeSelector["key1"])

				assert.Len(t, pod.Spec.Containers, 3)

				var names []string
				for _, n := range pod.Spec.Containers {
					names = append(names, n.Name)
				}
				assert.Contains(t, names, "helper")
				assert.Contains(t, names, "build")
				assert.Contains(t, names, "new-container")
			},
		},
		"uses default AutomountServiceAccountToken for pod": {
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Nil(
					t,
					pod.Spec.AutomountServiceAccountToken,
					"Pod AutomountServiceAccountToken should be empty",
				)
			},
		},
		"configures to enable AutomountServiceAccountToken for pod": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						AutomountServiceAccountToken: func(b bool) *bool { return &b }(true),
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.True(t, *pod.Spec.AutomountServiceAccountToken)
			},
		},
		"configures to disable AutomountServiceAccountToken for pod": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						AutomountServiceAccountToken: func(b bool) *bool { return &b }(false),
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.False(t, *pod.Spec.AutomountServiceAccountToken)
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			ctx := t.Context()

			vars := test.Variables
			if vars == nil {
				vars = []spec.Variable{}
			}

			creds := test.Credentials
			if creds == nil {
				creds = []spec.Credentials{}
			}

			options := test.Options
			if options == nil {
				options = &kubernetesOptions{}
			}

			if test.RunnerConfig.Kubernetes == nil {
				test.RunnerConfig.Kubernetes = &common.KubernetesConfig{}
			}

			if test.RunnerConfig.Kubernetes.Namespace == "" {
				test.RunnerConfig.Kubernetes.Namespace = "default"
			}

			rt := setupBuildPodFakeRoundTripper{
				t:    t,
				test: test,
			}

			mockFc := newMockFeatureChecker(t)
			mockFc.On("IsHostAliasSupported").Return(true, nil).Maybe()

			mockPullManager := pull.NewMockManager(t)

			mockPodWatcher := newMockPodWatcher(t)
			mockPodWatcher.On("UpdatePodName", mock.AnythingOfType("string")).Maybe()

			ex := newExecutor()
			ex.kubeClient = testKubernetesClient(version, fake.CreateHTTPClient(rt.RoundTrip))
			ex.options = options
			ex.AbstractExecutor.Config = test.RunnerConfig
			ex.AbstractExecutor.BuildShell = &common.ShellConfiguration{}
			ex.AbstractExecutor.Build = &common.Build{
				Job: spec.Job{
					Variables:   vars,
					Credentials: creds,
				},
				Runner: &test.RunnerConfig,
			}
			ex.AbstractExecutor.ProxyPool = proxy.NewPool()
			ex.featureChecker = mockFc
			ex.pullManager = mockPullManager
			ex.podWatcher = mockPodWatcher

			if ex.options.Image.Name == "" {
				// Ensure we have a valid Docker image name in the configuration,
				// if nothing is specified in the test case
				ex.options.Image.Name = "build-image"
			}

			if test.PrepareFn != nil {
				test.PrepareFn(t, test, ex)
			}

			if test.Options != nil && test.Options.Services != nil {
				for name := range test.Options.Services {
					mockPullManager.On("GetPullPolicyFor", name).
						Return(api.PullAlways, nil).
						Once()
				}
			}

			mockPullManager.On("GetPullPolicyFor", helperContainerName).
				Return(api.PullAlways, nil).
				Maybe()
			mockPullManager.On("GetPullPolicyFor", buildContainerName).
				Return(api.PullAlways, nil).
				Maybe()

			err := ex.prepareOverwrites(test.Variables)
			assert.NoError(t, err, "error preparing overwrites")

			if test.Credentials != nil {
				err = ex.setupCredentials(ctx)
				assert.NoError(t, err, "error setting up credentials")
			}

			err = ex.setupBuildPod(ctx, test.InitContainers)
			if test.VerifySetupBuildPodErrFn == nil {
				assert.NoError(t, err, "error setting up build pod")
				assert.True(t, rt.executed, "RoundTrip for kubernetes client should be executed")
			} else {
				test.VerifySetupBuildPodErrFn(t, err)
			}

			if test.VerifyExecutorFn != nil {
				test.VerifyExecutorFn(t, test, ex)
			}
		})
	}
}

func TestPodWatcherSetup(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	podLabels := map[string]string{
		"foo": "bar",
	}

	build := &common.Build{
		Job: spec.Job{},
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Kubernetes: &common.KubernetesConfig{
					Image:                  "some-build-image",
					Namespace:              "some-namespace",
					RequestRetryBackoffMax: 1234,
					PodLabels:              podLabels,
				},
			},
		},
	}

	fakeKubeClient := testclient.NewClientset()
	mockPodWatcher := newMockPodWatcher(t)

	ex := newExecutor()
	ex.getKubeConfig = func(conf *common.KubernetesConfig, overwrites *overwrites) (*restclient.Config, error) {
		return nil, nil
	}
	ex.newKubeClient = func(config *restclient.Config) (kubernetes.Interface, error) {
		return fakeKubeClient, nil
	}
	ex.newPodWatcher = func(c podWatcherConfig) podWatcher {
		assert.Equal(t, fakeKubeClient, c.kubeClient)
		assert.Equal(t, "some-namespace", c.namespace)
		assert.Equal(t, ex.featureChecker, c.featureChecker)
		assert.Equal(t, time.Millisecond*1234, c.maxSyncDuration)
		assert.Subset(t, c.labels, podLabels)
		assert.Contains(t, c.labels, "pod")
		return mockPodWatcher
	}

	mockPodWatcher.On("Start").Return(nil).Once()

	mockTrace := buildlogger.NewMockTrace(t)
	mockTrace.EXPECT().IsStdout().Return(true).Once()
	mockTrace.EXPECT().Write(mock.Anything).Return(0, nil)

	err := ex.Prepare(common.ExecutorPrepareOptions{
		Context:     ctx,
		Build:       build,
		Config:      build.Runner,
		BuildLogger: buildlogger.New(mockTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{}),
	})
	assert.NoError(t, err, "preparing the executor")
	assert.NotNil(t, ex.featureChecker, "expected feature checker to be set")
	assert.NotNil(t, ex.podWatcher, "expected pod watcher to be set")

	mockPodWatcher.On("UpdatePodName", mock.AnythingOfType("string")).Once()
	err = ex.setupBuildPod(ctx, nil)
	assert.NoError(t, err, "setting up the  build pod")

	mockPodWatcher.On("Stop").Once()
	ex.Finish(nil)
}

func TestPodWatcherGracefulDegrade(t *testing.T) {
	tests := map[string]struct {
		allowed            bool
		reason             string
		err                error
		expectedPodWatcher podWatcher
		expectedLog        string
		expectedCallCount  int
	}{
		"all allowed": {
			allowed:            true,
			expectedPodWatcher: &watchers.PodWatcher{},
			expectedCallCount:  2,
		},
		"some error": {
			err:                fmt.Errorf("some error when creating the review"),
			expectedPodWatcher: watchers.NoopPodWatcher{},
			expectedLog:        `WARNING: won't use informers: "some error when creating the review", see: https://docs.gitlab.com/runner/executors/kubernetes/#informers`,
			expectedCallCount:  1,
		},
		"not allowed": {
			expectedPodWatcher: watchers.NoopPodWatcher{},
			expectedLog:        `WARNING: won't use informers: "", see: https://docs.gitlab.com/runner/executors/kubernetes/#informers`,
			expectedCallCount:  1,
		},
		"not allowed, with reason": {
			reason:             "some reason",
			expectedPodWatcher: watchers.NoopPodWatcher{},
			expectedLog:        `WARNING: won't use informers: "some reason", see: https://docs.gitlab.com/runner/executors/kubernetes/#informers`,
			expectedCallCount:  1,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			ex := newExecutor()

			ctx := t.Context()
			podGvr := metav1.GroupVersionResource{Version: "v1", Resource: "pods"}

			mockTrace := common.NewMockJobTrace(t)
			mockFeatureChecker := newMockFeatureChecker(t)

			mockTrace.On("IsStdout").Return(false).Once()
			if test.expectedLog != "" {
				mockTrace.On("Write", mock.MatchedBy(func(b []byte) bool {
					return strings.Contains(string(b), test.expectedLog)
				})).Return(0, nil).Once()
			}

			mockFeatureChecker.
				On("IsResourceVerbAllowed", ctx, podGvr, "some-namespace", mock.MatchedBy(func(verb string) bool {
					return verb == "list" || verb == "watch"
				})).
				Return(test.allowed, test.reason, test.err).
				Times(test.expectedCallCount)

			logger := buildlogger.New(mockTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})
			podWatcher := ex.newPodWatcher(podWatcherConfig{
				ctx:            ctx,
				logger:         &logger,
				namespace:      "some-namespace",
				featureChecker: mockFeatureChecker,
				retryProvider:  ex,
			})

			assert.NotNil(t, podWatcher, "expected pod watcher not to be nil")
			assert.IsType(t, test.expectedPodWatcher, podWatcher)
		})
	}
}

func TestProcessLogs(t *testing.T) {
	tests := map[string]struct {
		lineCh           chan string
		errCh            chan error
		expectedExitCode int
		expectedScript   string
		run              func(ch chan string, errCh chan error)
	}{
		"Successful Processing": {
			lineCh:           make(chan string, 2),
			errCh:            make(chan error, 1),
			expectedExitCode: 1,
			expectedScript:   "script",
			run: func(ch chan string, errCh chan error) {
				ch <- getCommandExitStatus(1, "script")
			},
		},
		"Reattach failure with CodeExitError": {
			lineCh:           make(chan string, 1),
			errCh:            make(chan error, 1),
			expectedExitCode: 2,
			expectedScript:   "",
			run: func(ch chan string, errCh chan error) {
				errCh <- exec.CodeExitError{
					Err:  fmt.Errorf("giving up reattaching to log"),
					Code: 2,
				}
			},
		},
		"Reattach failure with EOF error": {
			lineCh:           make(chan string, 1),
			errCh:            make(chan error, 1),
			expectedExitCode: unknownLogProcessorExitCode,
			expectedScript:   "",
			run: func(ch chan string, errCh chan error) {
				errCh <- fmt.Errorf("Custom error for test with EOF %w", io.EOF)
			},
		},
		"Reattach failure with custom error": {
			lineCh:           make(chan string, 1),
			errCh:            make(chan error, 1),
			expectedExitCode: unknownLogProcessorExitCode,
			expectedScript:   "",
			run: func(ch chan string, errCh chan error) {
				errCh <- errors.New("Custom error")
			},
		},
		"Error channel closed before line channel": {
			lineCh:           make(chan string, 2),
			errCh:            make(chan error, 1),
			expectedExitCode: 3,
			expectedScript:   "script",
			run: func(ch chan string, errCh chan error) {
				close(errCh)
				ch <- getCommandExitStatus(3, "script")
				close(ch)
			},
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()

			waitForLineWritten := make(chan struct{})

			mockTrace := common.NewMockJobTrace(t)
			mockTrace.On("Write", []byte("line\n")).
				Run(func(args mock.Arguments) {
					close(waitForLineWritten)
				}).
				Return(0, nil).
				Once()

			mockTrace.On("IsStdout").Return(true).Maybe()
			mockTrace.On("Write", mock.Anything).
				Return(0, nil).
				Maybe()

			mockLogProcessor := newMockLogProcessor(t)

			tc.lineCh <- "line\n"
			mockLogProcessor.On("Process", mock.Anything).
				Return((<-chan string)(tc.lineCh), (<-chan error)(tc.errCh)).
				Once()

			tc.run(tc.lineCh, tc.errCh)

			e := newExecutor()
			e.BuildLogger = buildlogger.New(mockTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})
			e.pod = &api.Pod{}
			e.pod.Name = "pod_name"
			e.pod.Namespace = "namespace"
			e.newLogProcessor = func() logProcessor {
				return mockLogProcessor
			}

			go e.processLogs(t.Context())

			exitStatus := <-e.remoteProcessTerminated
			assert.Equal(t, tc.expectedExitCode, *exitStatus.CommandExitCode)
			if tc.expectedScript != "" {
				assert.Equal(t, tc.expectedScript, *exitStatus.Script)
			}

			select {
			case <-waitForLineWritten:
			case <-ctx.Done():
			}
		})
	}
}

func getCommandExitStatus(exitCode int, script string) string {
	return fmt.Sprintf(`{"command_exit_code": %v, "script": %q}`, exitCode, script)
}

func TestRunAttachCheckPodStatus(t *testing.T) {
	version, codec := testVersionAndCodec()

	respErr := errors.New("err")

	type podResponse struct {
		response *http.Response
		err      error
	}

	tests := map[string]struct {
		responses []podResponse
		verifyErr func(t *testing.T, errCh <-chan error)
	}{
		"no error": {
			responses: []podResponse{
				{
					response: &http.Response{StatusCode: http.StatusOK},
					err:      nil,
				},
			},
			verifyErr: func(t *testing.T, errCh <-chan error) {
				assert.NoError(t, <-errCh)
			},
		},
		"pod phase failed": {
			responses: []podResponse{
				{
					response: &http.Response{
						StatusCode: http.StatusOK,
						Body:       objBody(codec, execPodWithPhase(api.PodFailed)),
					},
					err: nil,
				},
			},
			verifyErr: func(t *testing.T, errCh <-chan error) {
				err := <-errCh
				require.Error(t, err)
				var phaseErr *podPhaseError
				assert.ErrorAs(t, err, &phaseErr)
				assert.Equal(t, api.PodFailed, phaseErr.phase)
			},
		},
		"pod not found": {
			responses: []podResponse{
				{
					response: nil,
					err: &kubeerrors.StatusError{
						ErrStatus: metav1.Status{
							Code: http.StatusNotFound,
							Details: &metav1.StatusDetails{
								Kind: "pods",
							},
						},
					},
				},
			},
			verifyErr: func(t *testing.T, errCh <-chan error) {
				err := <-errCh
				require.Error(t, err)
				var statusErr *kubeerrors.StatusError
				assert.ErrorAs(t, err, &statusErr)
				assert.Equal(t, int32(http.StatusNotFound), statusErr.ErrStatus.Code)
			},
		},
		"pod service OOM": {
			responses: []podResponse{
				{
					response: &http.Response{
						StatusCode: http.StatusOK,
						Body: objBody(codec, &api.Pod{
							ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "test", ResourceVersion: "10"},
							Spec: api.PodSpec{
								RestartPolicy: api.RestartPolicyAlways,
								DNSPolicy:     api.DNSClusterFirst,
								Containers: []api.Container{
									{
										Name: "bar",
									},
								},
							},
							Status: api.PodStatus{
								Phase: api.PodRunning,
								ContainerStatuses: []api.ContainerStatus{
									{
										Name: "bar",
										State: api.ContainerState{
											Terminated: &api.ContainerStateTerminated{
												Reason:   "OOMKilled",
												ExitCode: 137,
											},
										},
									},
								},
							},
						}),
					},
					err: nil,
				},
			},
			verifyErr: func(t *testing.T, errCh <-chan error) {
				err := <-errCh
				require.Error(t, err)
				var serviceError *podContainerError
				assert.ErrorAs(t, err, &serviceError)
				assert.Equal(t, "bar", serviceError.containerName)
				assert.Equal(t, 137, serviceError.exitCode)
				assert.Equal(t, "OOMKilled", serviceError.reason)
			},
		},
		"pod service error": {
			responses: []podResponse{
				{
					response: &http.Response{
						StatusCode: http.StatusOK,
						Body: objBody(codec, &api.Pod{
							ObjectMeta: metav1.ObjectMeta{Name: "foo", Namespace: "test", ResourceVersion: "10"},
							Spec: api.PodSpec{
								RestartPolicy: api.RestartPolicyAlways,
								DNSPolicy:     api.DNSClusterFirst,
								Containers: []api.Container{
									{
										Name: "bar",
									},
								},
							},
							Status: api.PodStatus{
								Phase: api.PodRunning,
								ContainerStatuses: []api.ContainerStatus{
									{
										Name: "bar",
										State: api.ContainerState{
											Terminated: &api.ContainerStateTerminated{
												Reason:   "Error",
												ExitCode: 1,
											},
										},
									},
								},
							},
						}),
					},
					err: nil,
				},
			},
			verifyErr: func(t *testing.T, errCh <-chan error) {
				err := <-errCh
				require.Error(t, err)
				var serviceError *podContainerError
				assert.ErrorAs(t, err, &serviceError)
				assert.Equal(t, "bar", serviceError.containerName)
				assert.Equal(t, 1, serviceError.exitCode)
				assert.Equal(t, "Error", serviceError.reason)
			},
		},
		"general error continues": {
			responses: []podResponse{
				{
					response: nil,
					err:      respErr,
				},
				{
					response: nil,
					err:      respErr,
				},
				{
					response: nil,
					err:      respErr,
				},
			},
			verifyErr: func(t *testing.T, errCh <-chan error) {
				select {
				case err, more := <-errCh:
					assert.False(t, more)
					assert.NoError(t, err)
				case <-time.After(10 * time.Second):
					require.Fail(t, "Should not get any error")
				}
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			i := 0
			fakeClient := fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case p == "/api/v1/namespaces/namespace/pods/pod" && m == http.MethodGet:
					res := tt.responses[i]
					i++
					if i == len(tt.responses) {
						cancel()
					}

					if res.response == nil {
						return nil, res.err
					}

					res.response.Header = map[string][]string{
						common.ContentType: {"application/json"},
					}
					if res.response.Body == nil {
						res.response.Body = objBody(codec, execPod())
					}

					return res.response, nil
				default:
					return nil, fmt.Errorf("unexpected request")
				}
			})

			client := testKubernetesClient(version, fakeClient)

			e := newExecutor()
			e.Config.Kubernetes = &common.KubernetesConfig{
				PollInterval: 1,
				PollTimeout:  2,
			}
			e.kubeClient = client
			e.pod = &api.Pod{}
			e.pod.Name = "pod"
			e.pod.Namespace = "namespace"

			tt.verifyErr(t, e.watchPodStatus(ctx, &podContainerStatusChecker{}))
		})
	}
}

func fakeKubeDeleteResponse(status int) *http.Response {
	_, codec := testVersionAndCodec()

	body := objBody(codec, &metav1.Status{Code: int32(status)})
	return &http.Response{StatusCode: status, Body: body, Header: map[string][]string{
		common.ContentType: {"application/json"},
	}}
}

func TestNewLogStreamerStream(t *testing.T) {
	abortErr := errors.New("abort")

	pod := new(api.Pod)
	pod.Namespace = "k8s_namespace"
	pod.Name = "k8s_pod_name"

	client := mockKubernetesClientWithHost("", "", nil)
	output := new(bytes.Buffer)
	offset := 15

	e := newExecutor()
	e.pod = pod
	e.Build = &common.Build{
		Runner: new(common.RunnerConfig),
	}

	remoteExecutor := NewMockRemoteExecutor(t)
	urlMatcher := mock.MatchedBy(func(url *url.URL) bool {
		query := url.Query()
		assert.Equal(t, helperContainerName, query.Get("container"))
		assert.Equal(t, "true", query.Get("stdout"))
		assert.Equal(t, "true", query.Get("stderr"))
		command := query["command"]
		assert.Equal(t, []string{
			"gitlab-runner-helper",
			"read-logs",
			"--path",
			e.logFile(),
			"--offset",
			strconv.Itoa(offset),
			"--wait-file-timeout",
			waitLogFileTimeout.String(),
		}, command)

		return true
	})
	remoteExecutor.
		On("Execute", mock.Anything, http.MethodPost, urlMatcher, mock.Anything, nil, output, output, false).
		Return(abortErr)

	p, ok := e.newLogProcessor().(*kubernetesLogProcessor)
	require.True(t, ok)
	p.logsOffset = int64(offset)

	s, ok := p.logStreamer.(*kubernetesLogStreamer)
	require.True(t, ok)
	s.client = client
	s.executor = remoteExecutor

	assert.Equal(t, pod.Name, s.pod)
	assert.Equal(t, pod.Namespace, s.namespace)

	err := s.Stream(t.Context(), int64(offset), output)
	assert.ErrorIs(t, err, abortErr)
}

type FakeReadCloser struct {
	io.Reader
}

func (f FakeReadCloser) Close() error {
	return nil
}

type FakeBuildTrace struct {
	testWriter
}

func (f FakeBuildTrace) Success() error                                                             { return nil }
func (f FakeBuildTrace) Fail(err error, failureData common.JobFailureData) error                    { return nil }
func (f FakeBuildTrace) Finish()                                                                    {}
func (f FakeBuildTrace) Notify(func())                                                              {}
func (f FakeBuildTrace) SetCancelFunc(cancelFunc context.CancelFunc)                                {}
func (f FakeBuildTrace) Cancel() bool                                                               { return false }
func (f FakeBuildTrace) SetAbortFunc(cancelFunc context.CancelFunc)                                 {}
func (f FakeBuildTrace) Abort() bool                                                                { return false }
func (f FakeBuildTrace) SetFailuresCollector(fc common.FailuresCollector)                           {}
func (f FakeBuildTrace) SetSupportedFailureReasonMapper(filter common.SupportedFailureReasonMapper) {}
func (f FakeBuildTrace) SetDebugModeEnabled(isEnabled bool)                                         {}
func (f FakeBuildTrace) IsStdout() bool {
	return false
}

func TestCommandTerminatedError_Is(t *testing.T) {
	tests := map[string]struct {
		err error

		expectedIsResult bool
	}{
		"nil": {
			err:              nil,
			expectedIsResult: false,
		},
		"EOF": {
			err:              io.EOF,
			expectedIsResult: false,
		},
		"commandTerminatedError": {
			err:              &commandTerminatedError{},
			expectedIsResult: true,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			if tt.expectedIsResult {
				assert.ErrorIs(t, tt.err, new(commandTerminatedError))
				return
			}

			assert.NotErrorIs(t, tt.err, new(commandTerminatedError))
		})
	}
}

func TestExecutor_buildPermissionsInitContainer(t *testing.T) {
	gitlabRegistry, err := helperimage.Get(common.AppVersion.Version, helperimage.Config{
		OSType:       helperimage.OSTypeLinux,
		Architecture: "amd64",
	})
	require.NoError(t, err)

	tests := map[string]struct {
		expectedImage string
		config        common.RunnerConfig
	}{
		"default helper image": {
			expectedImage: gitlabRegistry.String(),
			config: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image:      "alpine:3.14",
						PullPolicy: common.StringOrArray{common.PullPolicyIfNotPresent},
						Host:       "127.0.0.1",
					},
				},
			},
		},
		"configured helper image": {
			expectedImage: "config-image",
			config: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "config-image",
						Image:       "alpine:3.14",
						PullPolicy:  common.StringOrArray{common.PullPolicyIfNotPresent},
						Host:        "127.0.0.1",
					},
				},
			},
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
			e := newExecutor()
			e.AbstractExecutor.Build = &common.Build{
				Runner: &tt.config,
			}
			e.newPodWatcher = func(c podWatcherConfig) podWatcher {
				mockPodWatcher := newMockPodWatcher(t)
				mockPodWatcher.On("Start").Return(nil).Once()
				return mockPodWatcher
			}

			mockTrace := buildlogger.NewMockTrace(t)
			mockTrace.EXPECT().IsStdout().Return(true).Once()
			mockTrace.EXPECT().Write(mock.Anything).Return(0, nil)

			prepareOptions := common.ExecutorPrepareOptions{
				Config:      &tt.config,
				Build:       e.Build,
				Context:     t.Context(),
				BuildLogger: buildlogger.New(mockTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{}),
			}

			err := e.Prepare(prepareOptions)
			require.NoError(t, err)

			c, err := e.buildPermissionsInitContainer(helperimage.OSTypeLinux)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedImage, c.Image)
			assert.Equal(t, api.PullIfNotPresent, c.ImagePullPolicy)
			assert.Len(t, c.VolumeMounts, 3)
			assert.Len(t, c.Command, 3)
		})
	}
}

func TestExecutor_buildPermissionsInitContainer_FailPullPolicy(t *testing.T) {
	mockPullManager := pull.NewMockManager(t)

	e := newExecutor()
	e.pullManager = mockPullManager

	mockPullManager.On("GetPullPolicyFor", mock.Anything).
		Return(api.PullAlways, assert.AnError).
		Once()

	_, err := e.buildPermissionsInitContainer(helperimage.OSTypeLinux)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestExecutor_buildPermissionsInitContainer_CheckResources(t *testing.T) {
	mockPullManager := pull.NewMockManager(t)
	cpu := resource.MustParse("1")
	memory := resource.MustParse("1Gi")

	e := newExecutor()
	e.AbstractExecutor.Build = &common.Build{}
	e.pullManager = mockPullManager
	e.configurationOverwrites = &overwrites{
		helperLimits: api.ResourceList{
			"cpu":    cpu,
			"memory": memory,
		},
		helperRequests: api.ResourceList{
			"cpu":    cpu,
			"memory": memory,
		},
	}

	mockPullManager.On("GetPullPolicyFor", mock.Anything).
		Return(api.PullAlways, nil).
		Once()

	container, err := e.buildPermissionsInitContainer(helperimage.OSTypeLinux)

	require.NoError(t, err)

	assert.True(t, container.Resources.Limits.Cpu().Equal(cpu))
	assert.True(t, container.Resources.Requests.Cpu().Equal(cpu))

	assert.True(t, container.Resources.Limits.Memory().Equal(memory))
	assert.True(t, container.Resources.Requests.Memory().Equal(memory))
}

func TestShellRetrieval(t *testing.T) {
	successfulResponse, err := common.GetRemoteSuccessfulMultistepBuild()
	require.NoError(t, err)

	tests := map[string]struct {
		executor     *executor
		expectedName string
		expectedErr  error
	}{
		"retrieve bash": {
			executor:     setupExecutor("bash", successfulResponse),
			expectedName: "bash",
		},
		"retrieve pwsh": {
			executor:     setupExecutor(shells.SNPwsh, successfulResponse),
			expectedName: shells.SNPwsh,
		},
		"failure for no shell": {
			executor:    setupExecutor("no shell", successfulResponse),
			expectedErr: errIncorrectShellType,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			shell, err := tt.executor.retrieveShell()
			assert.Equal(t, err, tt.expectedErr, "The retrievalShell error and the expected one should be the same")
			if tt.expectedErr == nil {
				assert.Equal(t, tt.expectedName, shell.GetName())
			}
		})
	}
}

func TestGetContainerInfo(t *testing.T) {
	successfulResponse, err := common.GetRemoteSuccessfulMultistepBuild()
	require.NoError(t, err)

	tests := map[string]struct {
		executor              *executor
		command               common.ExecutorCommand
		expectedContainerName string
		getExpectedCommand    func(e *executor, cmd common.ExecutorCommand) []string
	}{
		"bash container info": {
			executor: setupExecutor("bash", successfulResponse),
			command: common.ExecutorCommand{
				Stage: common.BuildStagePrepare,
			},
			expectedContainerName: buildContainerName,
			getExpectedCommand: func(e *executor, cmd common.ExecutorCommand) []string {
				return []string{
					"sh",
					"-c",
					fmt.Sprintf("'(%s %s %s) &'",
						e.scriptPath(detectShellScriptName),
						e.scriptPath(cmd.Stage),
						e.buildRedirectionCmd("bash"),
					),
				}
			},
		},
		"predefined bash container info": {
			executor: setupExecutor("bash", successfulResponse),
			command: common.ExecutorCommand{
				Stage:      common.BuildStagePrepare,
				Predefined: true,
			},
			expectedContainerName: helperContainerName,
			getExpectedCommand: func(e *executor, cmd common.ExecutorCommand) []string {
				return append(
					e.helperImageInfo.Cmd,
					"<<<",
					e.scriptPath(cmd.Stage),
					e.buildRedirectionCmd("bash"),
				)
			},
		},
		"pwsh container info": {
			executor: setupExecutor(shells.SNPwsh, successfulResponse),
			command: common.ExecutorCommand{
				Stage: common.BuildStagePrepare,
			},
			expectedContainerName: buildContainerName,
			getExpectedCommand: func(e *executor, cmd common.ExecutorCommand) []string {
				return []string{
					e.scriptPath(pwshJSONTerminationScriptName),
					e.scriptPath(cmd.Stage),
					e.buildRedirectionCmd("pwsh"),
				}
			},
		},
		"predefined pwsh container info": {
			executor: setupExecutor(shells.SNPwsh, successfulResponse),
			command: common.ExecutorCommand{
				Stage:      common.BuildStagePrepare,
				Predefined: true,
			},
			expectedContainerName: helperContainerName,
			getExpectedCommand: func(e *executor, cmd common.ExecutorCommand) []string {
				return []string{
					e.scriptPath(pwshJSONTerminationScriptName),
					e.scriptPath(cmd.Stage),
					e.buildRedirectionCmd("pwsh"),
				}
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			containerName, containerCommand := tt.executor.getContainerInfo(tt.command)
			assert.Equal(t, tt.expectedContainerName, containerName)
			assert.Equal(t, tt.getExpectedCommand(tt.executor, tt.command), containerCommand)
		})
	}
}

func setupExecutor(shell string, successfulResponse spec.Job) *executor {
	build := &common.Build{
		Job: successfulResponse,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: common.ExecutorKubernetes,
				Shell:    shell,
			},
		},
	}

	e := newExecutor()
	e.helperImageInfo = helperimage.Info{
		Cmd: []string{"custom", "command"},
	}
	e.AbstractExecutor.Build = build
	e.AbstractExecutor.ExecutorOptions = executors.ExecutorOptions{
		DefaultBuildsDir: "/builds",
		DefaultCacheDir:  "/cache",
		Shell: common.ShellScriptInfo{
			Shell: shell,
			Build: build,
		},
	}
	return e
}

func TestLifecyclePrepare(t *testing.T) {
	initExecutor := func(lifecycleCfg common.KubernetesContainerLifecyle) *executor {
		e := newExecutor()
		e.AbstractExecutor.Config.RunnerSettings.Kubernetes.ContainerLifecycle = lifecycleCfg
		return e
	}

	execHandler := &api.ExecAction{
		Command: []string{"ls", "-alF"},
	}

	httpGetHandler := &api.HTTPGetAction{
		Port:        intstr.FromInt32(8080),
		Path:        "/test",
		Host:        "localhost",
		HTTPHeaders: []api.HTTPHeader{},
	}

	tcpSocketHander := &api.TCPSocketAction{
		Port: intstr.FromInt32(8080),
		Host: "localhost",
	}

	tests := map[string]struct {
		lifecycleCfg        common.KubernetesContainerLifecyle
		validateHookHandler func(*testing.T, *api.Lifecycle)
	}{
		"empty container lifecycle": {
			lifecycleCfg: common.KubernetesContainerLifecyle{},
			validateHookHandler: func(t *testing.T, lifecycle *api.Lifecycle) {
				assert.Nil(t, lifecycle)
			},
		},
		"valid preStop exec hook configuration": {
			lifecycleCfg: common.KubernetesContainerLifecyle{
				PreStop: &common.KubernetesLifecycleHandler{
					Exec: &common.KubernetesLifecycleExecAction{
						Command: []string{"ls", "-alF"},
					},
				},
			},
			validateHookHandler: func(t *testing.T, lifecycle *api.Lifecycle) {
				assert.Nil(t, lifecycle.PostStart)

				assert.Equal(t, execHandler, lifecycle.PreStop.Exec)
				assert.Nil(t, lifecycle.PreStop.HTTPGet)
				assert.Nil(t, lifecycle.PreStop.TCPSocket)
			},
		},
		"valid preStop httpGet hook configuration": {
			lifecycleCfg: common.KubernetesContainerLifecyle{
				PreStop: &common.KubernetesLifecycleHandler{
					HTTPGet: &common.KubernetesLifecycleHTTPGet{
						Port: 8080,
						Host: "localhost",
						Path: "/test",
					},
				},
			},
			validateHookHandler: func(t *testing.T, lifecycle *api.Lifecycle) {
				assert.Nil(t, lifecycle.PostStart)

				assert.Equal(t, httpGetHandler, lifecycle.PreStop.HTTPGet)
				assert.Nil(t, lifecycle.PreStop.Exec)
				assert.Nil(t, lifecycle.PreStop.TCPSocket)
			},
		},
		"valid preStop TCPSocket hook configuration": {
			lifecycleCfg: common.KubernetesContainerLifecyle{
				PreStop: &common.KubernetesLifecycleHandler{
					TCPSocket: &common.KubernetesLifecycleTCPSocket{
						Port: 8080,
						Host: "localhost",
					},
				},
			},
			validateHookHandler: func(t *testing.T, lifecycle *api.Lifecycle) {
				assert.Nil(t, lifecycle.PostStart)

				assert.Equal(t, tcpSocketHander, lifecycle.PreStop.TCPSocket)
				assert.Nil(t, lifecycle.PreStop.Exec)
				assert.Nil(t, lifecycle.PreStop.HTTPGet)
			},
		},
		"valid postStart exec hook configuration": {
			lifecycleCfg: common.KubernetesContainerLifecyle{
				PostStart: &common.KubernetesLifecycleHandler{
					Exec: &common.KubernetesLifecycleExecAction{
						Command: []string{"ls", "-alF"},
					},
				},
			},
			validateHookHandler: func(t *testing.T, lifecycle *api.Lifecycle) {
				assert.Nil(t, lifecycle.PreStop)

				assert.Equal(t, execHandler, lifecycle.PostStart.Exec)
				assert.Nil(t, lifecycle.PostStart.HTTPGet)
				assert.Nil(t, lifecycle.PostStart.TCPSocket)
			},
		},
		"valid postStart httpGet hook configuration": {
			lifecycleCfg: common.KubernetesContainerLifecyle{
				PostStart: &common.KubernetesLifecycleHandler{
					HTTPGet: &common.KubernetesLifecycleHTTPGet{
						Port: 8080,
						Host: "localhost",
						Path: "/test",
					},
				},
			},
			validateHookHandler: func(t *testing.T, lifecycle *api.Lifecycle) {
				assert.Nil(t, lifecycle.PreStop)

				assert.Equal(t, httpGetHandler, lifecycle.PostStart.HTTPGet)
				assert.Nil(t, lifecycle.PostStart.Exec)
				assert.Nil(t, lifecycle.PostStart.TCPSocket)
			},
		},
		"valid postStart TCPSocket hook configuration": {
			lifecycleCfg: common.KubernetesContainerLifecyle{
				PostStart: &common.KubernetesLifecycleHandler{
					TCPSocket: &common.KubernetesLifecycleTCPSocket{
						Port: 8080,
						Host: "localhost",
					},
				},
			},
			validateHookHandler: func(t *testing.T, lifecycle *api.Lifecycle) {
				assert.Nil(t, lifecycle.PreStop)

				assert.Equal(t, tcpSocketHander, lifecycle.PostStart.TCPSocket)
				assert.Nil(t, lifecycle.PostStart.Exec)
				assert.Nil(t, lifecycle.PostStart.HTTPGet)
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			executor := initExecutor(tt.lifecycleCfg)
			lifecycle := executor.prepareLifecycleHooks()

			if tt.validateHookHandler != nil {
				tt.validateHookHandler(t, lifecycle)
			}
		})
	}
}

func TestScriptsBaseDir(t *testing.T) {
	tests := map[string]struct {
		base_dir      string
		expected_path string
	}{
		"scripts_base_dir not set or empty": {
			base_dir:      "",
			expected_path: "/scripts-0-0",
		},
		"scripts_base_dir set": {
			base_dir:      "/tmp",
			expected_path: "/tmp/scripts-0-0",
		},
		"scripts_base_dir trailing slash": {
			base_dir:      "/tmp/",
			expected_path: "/tmp/scripts-0-0",
		},
		"scripts_base_dir multiple trailing slash": {
			base_dir:      "/tmp//",
			expected_path: "/tmp/scripts-0-0",
		},
	}
	mockPullManager := pull.NewMockManager(t)
	mockPullManager.On("GetPullPolicyFor", mock.Anything).
		Return(api.PullAlways, nil).
		Times(4)

	executor := newExecutor()
	executor.pullManager = mockPullManager
	executor.Build = &common.Build{
		Runner: new(common.RunnerConfig),
	}
	executor.Config.Kubernetes = new(common.KubernetesConfig)

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			executor.Config.Kubernetes.ScriptsBaseDir = tt.base_dir
			opts := containerBuildOpts{
				name: buildContainerName,
			}
			container, err := executor.buildContainer(opts)
			require.NoError(t, err)
			for _, mount := range container.VolumeMounts {
				if mount.Name != "scripts" {
					continue
				}
				assert.Equal(t, tt.expected_path, mount.MountPath)
				break
			}
		})
	}
}

func TestLogsBaseDir(t *testing.T) {
	tests := map[string]struct {
		base_dir      string
		expected_path string
	}{
		"logs_base_dir not set or empty": {
			base_dir:      "",
			expected_path: "/logs-0-0",
		},
		"logs_base_dir set": {
			base_dir:      "/tmp",
			expected_path: "/tmp/logs-0-0",
		},
		"logs_base_dir trailing slash": {
			base_dir:      "/tmp/",
			expected_path: "/tmp/logs-0-0",
		},
		"logs_base_dir multiple trailing slash": {
			base_dir:      "/tmp//",
			expected_path: "/tmp/logs-0-0",
		},
	}
	mockPullManager := pull.NewMockManager(t)
	mockPullManager.On("GetPullPolicyFor", mock.Anything).
		Return(api.PullAlways, nil).
		Times(4)

	executor := newExecutor()
	executor.pullManager = mockPullManager
	executor.Build = &common.Build{
		Runner: new(common.RunnerConfig),
	}
	executor.Config.Kubernetes = new(common.KubernetesConfig)

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			executor.Config.Kubernetes.LogsBaseDir = tt.base_dir
			opts := containerBuildOpts{
				name: buildContainerName,
			}
			container, err := executor.buildContainer(opts)
			require.NoError(t, err)
			for _, mount := range container.VolumeMounts {
				if mount.Name != "logs" {
					continue
				}
				assert.Equal(t, tt.expected_path, mount.MountPath)
				break
			}
		})
	}
}

func TestBuildContainerSecurityContext(t *testing.T) {
	tests := map[string]struct {
		getSecurityContext func() *api.SecurityContext
	}{
		"build security context": {
			getSecurityContext: func() *api.SecurityContext {
				runAsNonRoot := true
				readOnlyRootFileSystem := true
				privileged := false
				allowPrivilageEscalation := false
				var uid int64 = 1000
				var gid int64 = 1000
				return &api.SecurityContext{
					RunAsNonRoot:             &runAsNonRoot,
					ReadOnlyRootFilesystem:   &readOnlyRootFileSystem,
					Privileged:               &privileged,
					AllowPrivilegeEscalation: &allowPrivilageEscalation,
					RunAsUser:                &uid,
					RunAsGroup:               &gid,
					Capabilities: &api.Capabilities{
						Drop: []api.Capability{"ALL"},
					},
				}
			},
		},
		"no security context": {
			getSecurityContext: func() *api.SecurityContext {
				return nil
			},
		},
	}

	mockPullManager := pull.NewMockManager(t)
	mockPullManager.On("GetPullPolicyFor", mock.Anything).
		Return(api.PullAlways, nil).
		Times(2)

	executor := newExecutor()
	executor.pullManager = mockPullManager
	executor.Build = &common.Build{
		Runner: new(common.RunnerConfig),
	}
	executor.Config.Kubernetes = new(common.KubernetesConfig)

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			opts := containerBuildOpts{
				name:            buildContainerName,
				securityContext: tt.getSecurityContext(),
			}
			container, err := executor.buildContainer(opts)
			require.NoError(t, err)
			assert.Equal(t, tt.getSecurityContext(), container.SecurityContext)
		})
	}
}

func TestInitPermissionContainerSecurityContext(t *testing.T) {
	runAsNonRoot := true
	readOnlyRootFileSystem := true
	privileged := false
	allowPrivilageEscalation := false
	var uid int64 = 1000
	var gid int64 = 1000

	tests := map[string]struct {
		getConfig          common.KubernetesContainerSecurityContext
		getSecurityContext *api.SecurityContext
	}{
		"init permission security context": {
			getConfig: common.KubernetesContainerSecurityContext{
				Capabilities: &common.KubernetesContainerCapabilities{
					Add:  nil,
					Drop: []api.Capability{"ALL"},
				},
				Privileged:               &privileged,
				RunAsUser:                &uid,
				RunAsGroup:               &gid,
				RunAsNonRoot:             &runAsNonRoot,
				ReadOnlyRootFilesystem:   &readOnlyRootFileSystem,
				AllowPrivilegeEscalation: &allowPrivilageEscalation,
			},
			getSecurityContext: &api.SecurityContext{
				RunAsNonRoot:             &runAsNonRoot,
				ReadOnlyRootFilesystem:   &readOnlyRootFileSystem,
				Privileged:               &privileged,
				AllowPrivilegeEscalation: &allowPrivilageEscalation,
				RunAsUser:                &uid,
				RunAsGroup:               &gid,
				Capabilities: &api.Capabilities{
					Drop: []api.Capability{"ALL"},
				},
			},
		},
		"no security context": {
			getConfig: common.KubernetesContainerSecurityContext{
				Capabilities:             nil,
				Privileged:               nil,
				RunAsUser:                nil,
				RunAsGroup:               nil,
				RunAsNonRoot:             nil,
				ReadOnlyRootFilesystem:   nil,
				AllowPrivilegeEscalation: nil,
			},
			getSecurityContext: &api.SecurityContext{
				Capabilities: &api.Capabilities{
					// default Drop Capabilities
					Drop: []api.Capability{"NET_RAW"},
				},
				Privileged:               nil,
				RunAsUser:                nil,
				RunAsGroup:               nil,
				RunAsNonRoot:             nil,
				ReadOnlyRootFilesystem:   nil,
				AllowPrivilegeEscalation: nil,
			},
		},
	}

	mockPullManager := pull.NewMockManager(t)
	mockPullManager.On("GetPullPolicyFor", mock.Anything).
		Return(api.PullAlways, nil).
		Times(len(tests))

	executor := newExecutor()
	executor.pullManager = mockPullManager
	executor.Build = &common.Build{
		Runner: new(common.RunnerConfig),
	}

	executor.Config.Kubernetes = new(common.KubernetesConfig)

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			executor.Config.Kubernetes.InitPermissionsContainerSecurityContext = tt.getConfig
			container, err := executor.buildPermissionsInitContainer(executor.helperImageInfo.OSType)
			require.NoError(t, err)
			assert.Equal(t, tt.getSecurityContext, container.SecurityContext)
		})
	}
}

func TestInitBuildUidGidCollectorSecurityContext(t *testing.T) {
	runAsNonRoot := true
	readOnlyRootFileSystem := true
	privileged := false
	allowPrivilageEscalation := false
	var uid int64 = 1000
	var gid int64 = 1000

	tests := map[string]struct {
		jobUser            string
		allowedUsers       []string
		allowedGroups      []string
		getConfig          common.KubernetesContainerSecurityContext
		getSecurityContext *api.SecurityContext
		expectWarning      string
	}{
		"init uid/gid collector inherits job user when no security context": {
			jobUser: "1500:1600",
			getConfig: common.KubernetesContainerSecurityContext{
				Capabilities:             nil,
				Privileged:               nil,
				RunAsUser:                nil,
				RunAsGroup:               nil,
				RunAsNonRoot:             nil,
				ReadOnlyRootFilesystem:   nil,
				AllowPrivilegeEscalation: nil,
			},
			getSecurityContext: &api.SecurityContext{
				Capabilities: &api.Capabilities{
					Drop: []api.Capability{"NET_RAW"},
				},
				Privileged:               nil,
				RunAsUser:                common.Int64Ptr(1500),
				RunAsGroup:               common.Int64Ptr(1600),
				RunAsNonRoot:             nil,
				ReadOnlyRootFilesystem:   nil,
				AllowPrivilegeEscalation: nil,
			},
		},
		"init uid/gid collector security context overrides job user": {
			jobUser: "1500:1600",
			getConfig: common.KubernetesContainerSecurityContext{
				Capabilities: &common.KubernetesContainerCapabilities{
					Add:  nil,
					Drop: []api.Capability{"ALL"},
				},
				Privileged:               &privileged,
				RunAsUser:                &uid,
				RunAsGroup:               &gid,
				RunAsNonRoot:             &runAsNonRoot,
				ReadOnlyRootFilesystem:   &readOnlyRootFileSystem,
				AllowPrivilegeEscalation: &allowPrivilageEscalation,
			},
			getSecurityContext: &api.SecurityContext{
				RunAsNonRoot:             &runAsNonRoot,
				ReadOnlyRootFilesystem:   &readOnlyRootFileSystem,
				Privileged:               &privileged,
				AllowPrivilegeEscalation: &allowPrivilageEscalation,
				RunAsUser:                &uid,
				RunAsGroup:               &gid,
				Capabilities: &api.Capabilities{
					Drop: []api.Capability{"ALL"},
				},
			},
		},
		"init uid/gid collector bypasses allowlist with security context": {
			jobUser:       "1500:1600",
			allowedUsers:  []string{"2000"},
			allowedGroups: []string{"2100"},
			getConfig: common.KubernetesContainerSecurityContext{
				RunAsUser:  &uid,
				RunAsGroup: &gid,
			},
			getSecurityContext: &api.SecurityContext{
				Capabilities: &api.Capabilities{
					Drop: []api.Capability{"NET_RAW"},
				},
				Privileged:               nil,
				RunAsUser:                &uid,
				RunAsGroup:               &gid,
				RunAsNonRoot:             nil,
				ReadOnlyRootFilesystem:   nil,
				AllowPrivilegeEscalation: nil,
			},
		},
		"init uid/gid collector job user blocked by allowlist": {
			jobUser:       "1500:1600",
			allowedUsers:  []string{"2000"},
			allowedGroups: []string{"2100"},
			getConfig: common.KubernetesContainerSecurityContext{
				Capabilities:             nil,
				Privileged:               nil,
				RunAsUser:                nil,
				RunAsGroup:               nil,
				RunAsNonRoot:             nil,
				ReadOnlyRootFilesystem:   nil,
				AllowPrivilegeEscalation: nil,
			},
			getSecurityContext: &api.SecurityContext{
				Capabilities: &api.Capabilities{
					Drop: []api.Capability{"NET_RAW"},
				},
				Privileged:               nil,
				RunAsUser:                nil, // Validation failure returns -1, which doesn't get set
				RunAsGroup:               nil, // Validation failure returns -1, which doesn't get set
				RunAsNonRoot:             nil,
				ReadOnlyRootFilesystem:   nil,
				AllowPrivilegeEscalation: nil,
			},
			expectWarning: `user "1500" is not in the allowed list`,
		},
		"init uid/gid collector can run as root via security context despite allowlist": {
			allowedUsers:  []string{"1000"},
			allowedGroups: []string{"1000"},
			getConfig: common.KubernetesContainerSecurityContext{
				RunAsUser:  common.Int64Ptr(0),
				RunAsGroup: common.Int64Ptr(0),
			},
			getSecurityContext: &api.SecurityContext{
				Capabilities: &api.Capabilities{
					Drop: []api.Capability{"NET_RAW"},
				},
				Privileged:               nil,
				RunAsUser:                common.Int64Ptr(0),
				RunAsGroup:               common.Int64Ptr(0),
				RunAsNonRoot:             nil,
				ReadOnlyRootFilesystem:   nil,
				AllowPrivilegeEscalation: nil,
			},
		},
		"init uid/gid collector with only user in security context": {
			jobUser: "1500:1600",
			getConfig: common.KubernetesContainerSecurityContext{
				RunAsUser: &uid,
			},
			getSecurityContext: &api.SecurityContext{
				Capabilities: &api.Capabilities{
					Drop: []api.Capability{"NET_RAW"},
				},
				Privileged:               nil,
				RunAsUser:                &uid,
				RunAsGroup:               common.Int64Ptr(1600),
				RunAsNonRoot:             nil,
				ReadOnlyRootFilesystem:   nil,
				AllowPrivilegeEscalation: nil,
			},
		},
		"init uid/gid collector with only group in security context": {
			jobUser: "1500:1600",
			getConfig: common.KubernetesContainerSecurityContext{
				RunAsGroup: &gid,
			},
			getSecurityContext: &api.SecurityContext{
				Capabilities: &api.Capabilities{
					Drop: []api.Capability{"NET_RAW"},
				},
				Privileged:               nil,
				RunAsUser:                common.Int64Ptr(1500),
				RunAsGroup:               &gid,
				RunAsNonRoot:             nil,
				ReadOnlyRootFilesystem:   nil,
				AllowPrivilegeEscalation: nil,
			},
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			mockPullManager := pull.NewMockManager(t)
			mockPullManager.On("GetPullPolicyFor", mock.Anything).
				Return(api.PullAlways, nil).
				Maybe()

			executor := newExecutor()
			executor.pullManager = mockPullManager
			executor.configurationOverwrites = &overwrites{
				namespace:       "default",
				serviceLimits:   api.ResourceList{},
				buildLimits:     api.ResourceList{},
				helperLimits:    api.ResourceList{},
				serviceRequests: api.ResourceList{},
				buildRequests:   api.ResourceList{},
				helperRequests:  api.ResourceList{},
				podRequests:     api.ResourceList{},
				podLimits:       api.ResourceList{},
			}
			executor.helperImageInfo = helperimage.Info{
				Architecture: "x86_64",
				OSType:       helperimage.OSTypeLinux,
				Name:         "registry.gitlab.com/gitlab-org/gitlab-runner/gitlab-runner-helper",
				Tag:          "x86_64-latest",
			}
			executor.Build = &common.Build{
				Runner: new(common.RunnerConfig),
			}
			executor.Config.Kubernetes = new(common.KubernetesConfig)
			executor.Config.Kubernetes.BuildContainerSecurityContext = tt.getConfig
			executor.Config.Kubernetes.AllowedUsers = tt.allowedUsers
			executor.Config.Kubernetes.AllowedGroups = tt.allowedGroups
			executor.options = &kubernetesOptions{
				Image: spec.Image{
					ExecutorOptions: spec.ImageExecutorOptions{
						Kubernetes: spec.ImageKubernetesOptions{
							User: spec.StringOrInt64(tt.jobUser),
						},
					},
				},
			}

			// Set up build logger to capture warnings
			var logOutput strings.Builder
			buildTrace := FakeBuildTrace{
				testWriter: testWriter{
					call: func(b []byte) (int, error) {
						logOutput.Write(b)
						return len(b), nil
					},
				},
			}
			executor.BuildLogger = buildlogger.New(buildTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})

			container, err := executor.buildUiGidCollector(executor.helperImageInfo.OSType)

			require.NoError(t, err)
			assert.Equal(t, tt.getSecurityContext, container.SecurityContext)
			assert.Equal(t, "init-build-uid-gid-collector", container.Name)

			// Check for expected warnings
			if tt.expectWarning != "" {
				assert.Contains(t, logOutput.String(), tt.expectWarning,
					"Expected warning message not found in log output: %s", logOutput.String())
			}
		})
	}
}

func Test_Executor_captureContainerLogs(t *testing.T) {
	const (
		cName = "some container"
		msg   = "pretend this is a log generated by a process in a container"
	)

	fakeRoundTripper := func(body io.ReadCloser, err error) func(req *http.Request) (*http.Response, error) {
		return func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "GET", req.Method, "expected a GET request, got: %s", req.Method)
			path := req.URL.Path
			query := req.URL.Query()

			// the fake response for the watch request
			if path == "/api/v1/namespaces/test-ns/pods" && query["fieldSelector"][0] == "status.phase=Running,metadata.name=test-pod" {
				return &http.Response{
					StatusCode: http.StatusOK,
				}, nil
			}

			// the fake response for the pods/log request
			if path == "/api/v1/namespaces/test-ns/pods/test-pod/log" && query["container"][0] == "some container" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       body,
				}, err
			}

			err := fmt.Errorf("unexpected request: %+v", req)
			require.NoError(t, err)
			return nil, err
		}
	}

	version, _ := testVersionAndCodec()

	tests := map[string]struct {
		wantLog    string
		wantErr    error
		readCloser func(io.ReadCloser) io.ReadCloser
	}{
		"success": {
			wantLog:    msg,
			readCloser: func(rc io.ReadCloser) io.ReadCloser { return rc },
		},
		"read error": {
			wantLog:    "error streaming logs for container some container:",
			readCloser: func(rc io.ReadCloser) io.ReadCloser { return &failingReadCloser{rc} },
		},
		"connect error": {
			wantLog:    "failed to open log stream for container " + cName,
			wantErr:    errors.New("blammo"),
			readCloser: func(_ io.ReadCloser) io.ReadCloser { return nil },
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			e := newExecutor()
			e.pod = &api.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"}}

			buf, err := trace.New()
			require.NoError(t, err)
			defer buf.Close()

			trace := &common.Trace{Writer: buf}
			e.BuildLogger = buildlogger.New(trace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})

			isw := service_helpers.NewInlineServiceLogWriter(cName, trace)

			// we'll write into pw, which will be copied to pr and simulate a process in
			// a container writing to stdout.
			pr, pw := io.Pipe()
			defer pw.Close() // ... for the failure case

			httpClient := fake.CreateHTTPClient(fakeRoundTripper(tt.readCloser(pr), tt.wantErr))
			e.kubeClient = testKubernetesClient(version, httpClient)

			err = e.captureContainerLogs(t.Context(), cName, isw)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantLog)
				return
			}

			require.NoError(t, err)

			// this will be copied to pr...
			_, err = pw.Write([]byte(msg))
			require.NoError(t, err)
			pw.Close() // this will also close pr

			assert.EventuallyWithT(t, func(t *assert.CollectT) {
				contents, err := buf.Bytes(0, math.MaxInt64)
				assert.NoError(t, err)
				assert.Contains(t, string(contents), tt.wantLog)
			}, time.Second*1, time.Millisecond+100)
		})
	}
}

// A simple type to simulate a read error. The embedded ReadCloser must be
// read/drained or the writing code could block (e.g. if using an io.Pipe).
type failingReadCloser struct {
	rc io.ReadCloser
}

func (frc *failingReadCloser) Read(p []byte) (int, error) {
	_, _ = frc.rc.Read(p)
	return 0, errors.New("failed to read")
}

func (frc *failingReadCloser) Close() error {
	return nil
}

func Test_Executor_captureServiceContainersLogs(t *testing.T) {
	containers := []api.Container{
		{Name: "not a service container"},
		{Name: "svc-0-a service container", Image: "postgres"},
		{Name: "svc-1-another service container", Image: "redis:latest"},
		{Name: "also not a service container"},
	}

	logs := bytes.Buffer{}
	lentry := logrus.New()
	lentry.Out = &logs

	stop := errors.New("don't actually try to stream the container's logs")
	fakeRoundTripper := func(req *http.Request) (*http.Response, error) {
		// have the call to GetLogs return an error so we don't have to mock
		// more behaviour. that functionality is tested elsewhere.
		return &http.Response{
			StatusCode: http.StatusInternalServerError,
			Header:     http.Header{},
		}, stop
	}

	version, _ := testVersionAndCodec()
	e := newExecutor()
	e.pod = &api.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"}}
	e.kubeClient = testKubernetesClient(version, fake.CreateHTTPClient(fakeRoundTripper))
	e.BuildLogger = buildlogger.New(&common.Trace{Writer: &logs}, logrus.NewEntry(lentry), buildlogger.Options{})

	ctx := t.Context()

	tests := map[string]struct {
		debugServicePolicy string
		assert             func(t *testing.T)
	}{
		"enabled": {
			debugServicePolicy: "true",
			assert: func(t *testing.T) {
				for _, c := range containers {
					if !strings.HasPrefix(c.Name, serviceContainerPrefix) {
						continue
					}
					assert.Contains(t, logs.String(), "WARNING: failed to open log stream for container "+c.Name)
					assert.Contains(t, logs.String(), stop.Error())
				}
			},
		},
		"disabled": {
			debugServicePolicy: "false",
			assert:             func(t *testing.T) { assert.Empty(t, logs.String()) },
		},
		"bogus": {
			debugServicePolicy: "blammo",
			assert:             func(t *testing.T) { assert.Empty(t, logs.String()) },
		},
	}

	for name, tt := range tests {
		logs.Reset()
		t.Run(name, func(t *testing.T) {
			e.Build = &common.Build{}
			e.Build.Services = spec.Services{
				{Name: "postgres", Alias: "db"},
				{Name: "redis:latest", Alias: "cache"},
			}
			e.Build.Variables = spec.Variables{
				{Key: "CI_DEBUG_SERVICES", Value: tt.debugServicePolicy, Public: true},
			}

			e.prepareOptions(e.Build)
			e.captureServiceContainersLogs(ctx, containers)
			tt.assert(t)
		})
	}
}

func TestDoPodSpecMerge(t *testing.T) {
	verifyFn := func(t *testing.T, patchedPodSpec *api.PodSpec) {
		assert.NotNil(t, patchedPodSpec)
		assert.Equal(t, "", patchedPodSpec.NodeName)
		assert.Equal(t, "my-service-account-name", patchedPodSpec.ServiceAccountName)
		assert.NotNil(t, patchedPodSpec.NodeSelector["key1"])
		assert.Equal(t, "val1", patchedPodSpec.NodeSelector["key1"])
	}

	tests := map[string]struct {
		getOriginal func() *api.PodSpec
		podSpec     common.KubernetesPodSpec
		verifyFn    func(*testing.T, *api.PodSpec)
		expectedErr error
	}{
		// Merge strategy as documented : https://datatracker.ietf.org/doc/html/rfc7386
		"successful simple yaml with merge patch type": {
			getOriginal: func() *api.PodSpec {
				return &api.PodSpec{NodeName: "my-node-name"}
			},
			podSpec: common.KubernetesPodSpec{
				Patch: `
nodeName: null
serviceAccountName: "my-service-account-name"
nodeSelector:
  key1: val1
`,
				PatchType: common.PatchTypeMergePatchType,
			},
			verifyFn: verifyFn,
		},
		"successful simple json with merge patch type": {
			getOriginal: func() *api.PodSpec {
				return &api.PodSpec{NodeName: "my-node-name"}
			},
			podSpec: common.KubernetesPodSpec{
				Patch: `
{
	nodeName: null,
	serviceAccountName: "my-service-account-name",
	nodeSelector: {
		key1: "val1"
	}
}`,
				PatchType: common.PatchTypeMergePatchType,
			},
			verifyFn: verifyFn,
		},
		// JSON strategy as documented : https://datatracker.ietf.org/doc/html/rfc7386
		"successful simple json with json patch type": {
			getOriginal: func() *api.PodSpec {
				return &api.PodSpec{NodeName: "my-node-name"}
			},
			podSpec: common.KubernetesPodSpec{
				Patch: `
[
	{ "op": "remove", "path": "/nodeName" },
	{ "op": "add", "path": "/serviceAccountName", "value": "my-service-account-name" },
	{ "op": "add", "path": "/nodeSelector", "value": { key1: "val1" } }
]
`,
				PatchType: common.PatchTypeJSONPatchType,
			},
			verifyFn: verifyFn,
		},
		// strategic strategy as documented
		// https://kubernetes.io/docs/tasks/manage-kubernetes-objects/update-api-object-kubectl-patch/#notes-on-the-strategic-merge-patch
		"successful simple json with strategic patch type on containers": {
			getOriginal: func() *api.PodSpec {
				return &api.PodSpec{
					NodeName: "my-node-name",
					Containers: []api.Container{
						{
							Name: "first-container",
						},
					},
				}
			},
			podSpec: common.KubernetesPodSpec{
				Patch: `
containers:
  - name: "second-container"
`,
				PatchType: common.PatchTypeStrategicMergePatchType,
			},
			verifyFn: func(t *testing.T, patchedPodSpec *api.PodSpec) {
				assert.NotNil(t, patchedPodSpec)
				assert.Len(t, patchedPodSpec.Containers, 2)

				var names []string
				for _, n := range patchedPodSpec.Containers {
					names = append(names, n.Name)
				}
				assert.Contains(t, names, "first-container")
				assert.Contains(t, names, "second-container")
			},
		},
		"successful pod-level resources strategic patch type on pod": {
			getOriginal: func() *api.PodSpec {
				return &api.PodSpec{
					NodeName: "my-node-name",
				}
			},
			podSpec: common.KubernetesPodSpec{
				Patch: `
resources:
  requests:
    cpu: "1.5"
    memory: "3Gi"
  limits:
    cpu: "2"
    memory: "5Gi"
`,
				PatchType: common.PatchTypeStrategicMergePatchType,
			},
			verifyFn: func(t *testing.T, patchedPodSpec *api.PodSpec) {
				assert.NotNil(t, patchedPodSpec)
				resources := patchedPodSpec.Resources
				expectedRequests := mustCreateResourceList(t, "1.5", "3Gi", "")
				expectedLimits := mustCreateResourceList(t, "2", "5Gi", "")
				assert.NotNil(t, resources)
				assert.Equal(t, expectedRequests, resources.Requests)
				assert.Equal(t, expectedLimits, resources.Limits)
			},
		},
		"unsupported patch type": {
			getOriginal: func() *api.PodSpec {
				return &api.PodSpec{
					NodeName: "my-node-name",
					Containers: []api.Container{
						{
							Name: "first-container",
						},
					},
				}
			},
			podSpec: common.KubernetesPodSpec{
				Patch: `
containers:
  - name: "second-container"
`,
				PatchType: "unknown",
			},
			expectedErr: fmt.Errorf("unsupported patch type unknown"),
		},
	}

	for tn, tc := range tests {
		t.Run(tn, func(t *testing.T) {
			patchedData, err := json.Marshal(tc.getOriginal())
			require.NoError(t, err)

			patchedData, err = doPodSpecMerge(patchedData, tc.podSpec)
			if tc.expectedErr != nil {
				assert.Error(t, err)
				assert.Equal(t, tc.expectedErr.Error(), err.Error())
				return
			}

			require.NoError(t, err)

			var patchedPodSpec api.PodSpec
			err = json.Unmarshal(patchedData, &patchedPodSpec)
			assert.NoError(t, err)

			tc.verifyFn(t, &patchedPodSpec)
		})
	}
}

func TestRetryLimits(t *testing.T) {
	tests := map[string]struct {
		config                   common.KubernetesConfig
		err                      error
		expectedRetryCount       int
		expectedLastRetryBackoff time.Duration
	}{
		"no retry limits": {
			config:                   common.KubernetesConfig{},
			err:                      syscall.ECONNRESET,
			expectedRetryCount:       5,
			expectedLastRetryBackoff: common.DefaultRequestRetryBackoffMax,
		},
		"retry limits": {
			config: common.KubernetesConfig{
				RequestRetryLimit: 6,
				RequestRetryLimits: map[string]int{
					syscall.ECONNRESET.Error(): 3,
				},
			},
			err:                      syscall.ECONNRESET,
			expectedRetryCount:       3,
			expectedLastRetryBackoff: common.DefaultRequestRetryBackoffMax,
		},
		"retry limits fallback to default": {
			config: common.KubernetesConfig{
				RequestRetryLimit: 6,
				RequestRetryLimits: map[string]int{
					syscall.ECONNRESET.Error(): 3,
				},
			},
			err:                      syscall.ECONNABORTED,
			expectedRetryCount:       6,
			expectedLastRetryBackoff: common.DefaultRequestRetryBackoffMax,
		},
		"retry limits with manually constructed error": {
			config: common.KubernetesConfig{
				RequestRetryLimit: 6,
				RequestRetryLimits: map[string]int{
					"error dialing backend": 2,
				},
			},
			err:                      errors.New("error dialing backend"),
			expectedRetryCount:       2,
			expectedLastRetryBackoff: 1000 * time.Millisecond,
		},
		"retry limits with manually constructed error that contains retry key": {
			config: common.KubernetesConfig{
				RequestRetryLimit: 6,
				RequestRetryLimits: map[string]int{
					"error dialing backend": 2,
				},
			},
			err:                      errors.New("--error dialing backend--"),
			expectedRetryCount:       2,
			expectedLastRetryBackoff: 1000 * time.Millisecond,
		},
		"retry with custom backoff max": {
			config: common.KubernetesConfig{
				RequestRetryBackoffMax: 4000,
				RequestRetryLimits: map[string]int{
					"error dialing backend": 6,
				},
			},
			err:                      errors.New("--error dialing backend--"),
			expectedRetryCount:       6,
			expectedLastRetryBackoff: 4000 * time.Millisecond,
		},
		"retry with custom backoff max lower than default backoff min": {
			config: common.KubernetesConfig{
				RequestRetryBackoffMax: 300,
				RequestRetryLimits: map[string]int{
					"error dialing backend": 3,
				},
			},
			err:                      errors.New("--error dialing backend--"),
			expectedRetryCount:       3,
			expectedLastRetryBackoff: common.RequestRetryBackoffMin,
		},
		"retry with custom backoff max between min and default max": {
			config: common.KubernetesConfig{
				RequestRetryBackoffMax: 1100,
				RequestRetryLimits: map[string]int{
					"error dialing backend": 3,
				},
			},
			err:                      errors.New("--error dialing backend--"),
			expectedRetryCount:       3,
			expectedLastRetryBackoff: 1100 * time.Millisecond,
		},
		"etcd request timeout": {
			config: common.KubernetesConfig{
				RequestRetryLimit: 5,
			},
			err:                      errors.New("etcdserver: request timed out"),
			expectedRetryCount:       5,
			expectedLastRetryBackoff: common.DefaultRequestRetryBackoffMax,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			e := newExecutor()
			e.Config.Kubernetes = &tt.config

			retryBackoffConfig := e.getRetryBackoffConfig()

			backoff := &backoff.Backoff{Min: retryBackoffConfig.min, Max: retryBackoffConfig.max}

			var counter int
			var lastRetryBackoff time.Duration

			err := retry.WithFn(e, func() error {
				counter++
				lastRetryBackoff = backoff.Duration()
				return tt.err
			}).Run()

			require.Equal(t, err, tt.err)
			require.Equal(t, tt.expectedRetryCount, counter)
			require.Equal(t, tt.expectedLastRetryBackoff, lastRetryBackoff)
		})
	}
}

// TestContainerPullPolicies assert that all containers have the proper pull policies as configured in the job
// definition
// TODO(hhoerl): do we need to test this with any feature flags?
func TestContainerPullPolicies(t *testing.T) {
	allPullPolicies := []common.DockerPullPolicy{
		"",
		common.PullPolicyNever,
		common.PullPolicyAlways,
		common.PullPolicyIfNotPresent,
	}

	testCases := map[string]struct {
		Services            spec.Services
		ServicesFromConfig  []common.Service
		AllowedPullPolicies []common.DockerPullPolicy
		DefaultPullPolicies common.StringOrArray

		ExpectedPullPolicyPerContainer map[string]api.PullPolicy
	}{
		"with explicitly all pull policies enabled and services": {
			AllowedPullPolicies: allPullPolicies,
			Services: spec.Services{
				{Name: "withNone"},
				{Name: "withAlways", PullPolicies: []spec.PullPolicy{common.PullPolicyAlways}},
				{Name: "withINP", PullPolicies: []spec.PullPolicy{common.PullPolicyIfNotPresent}},
				{Name: "withNever", PullPolicies: []spec.PullPolicy{common.PullPolicyNever}},
			},
			ExpectedPullPolicyPerContainer: map[string]api.PullPolicy{
				"build":  api.PullPolicy(""),
				"helper": api.PullPolicy(""),
				"svc-0":  api.PullPolicy(""),
				"svc-1":  api.PullAlways,
				"svc-2":  api.PullIfNotPresent,
				"svc-3":  api.PullNever,
			},
		},
		"with explicit default pull policies": {
			DefaultPullPolicies: common.StringOrArray{"always", "never"},
			ExpectedPullPolicyPerContainer: map[string]api.PullPolicy{
				"build":  api.PullAlways,
				"helper": api.PullAlways,
			},
		},
		"with allowed pull policies from build container pull policy": {
			DefaultPullPolicies: common.StringOrArray{"never", "always"},
			Services: spec.Services{
				{Name: "foo"},
				{Name: "bar", PullPolicies: []spec.PullPolicy{"always"}},
			},
			ExpectedPullPolicyPerContainer: map[string]api.PullPolicy{
				"build":  api.PullNever,
				"helper": api.PullNever,
				"svc-0":  api.PullNever,
				"svc-1":  api.PullAlways,
			},
		},
		"with nothing re pull policies set": {
			Services: spec.Services{
				{Name: "foo"},
			},
			ExpectedPullPolicyPerContainer: map[string]api.PullPolicy{
				"build":  api.PullPolicy(""),
				"helper": api.PullPolicy(""),
				"svc-0":  api.PullPolicy(""),
			},
		},
		"services from config use the correct pull policy": {
			DefaultPullPolicies: common.StringOrArray{"never", "if-not-present"},
			ServicesFromConfig: []common.Service{
				{Name: "from-toml"},
			},
			Services: spec.Services{
				{Name: "from-yaml-0", PullPolicies: []spec.PullPolicy{"if-not-present"}},
				{Name: "from-yaml-1"},
			},
			ExpectedPullPolicyPerContainer: map[string]api.PullPolicy{
				"build":  api.PullNever,
				"helper": api.PullNever,
				// services from config.toml come first
				"svc-0": api.PullNever,
				// then the services from the .gitlab-ci.yaml
				"svc-1": api.PullIfNotPresent,
				"svc-2": api.PullNever,
			},
		},
	}

	for tn, tc := range testCases {
		t.Run(tn, func(t *testing.T) {
			fakeKubeClient := testclient.NewClientset()

			runnerConfig := &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image:               "some-build-image",
						AllowedPullPolicies: tc.AllowedPullPolicies,
						PullPolicy:          tc.DefaultPullPolicies,
						Services:            tc.ServicesFromConfig,
					},
				},
			}

			build := &common.Build{
				Job: spec.Job{
					Services: tc.Services,
				},
				Runner: &common.RunnerConfig{
					RunnerSettings: common.RunnerSettings{
						Kubernetes: &common.KubernetesConfig{},
					},
				},
			}

			executor := newExecutor()
			executor.newKubeClient = func(_ *restclient.Config) (kubernetes.Interface, error) {
				return fakeKubeClient, nil
			}
			executor.getKubeConfig = func(_ *common.KubernetesConfig, _ *overwrites) (*restclient.Config, error) {
				return nil, nil
			}
			executor.newPodWatcher = func(c podWatcherConfig) podWatcher {
				mockPodWatcher := newMockPodWatcher(t)
				mockPodWatcher.On("Start").Return(nil).Once()
				mockPodWatcher.On("UpdatePodName", mock.AnythingOfType("string")).Once()
				return mockPodWatcher
			}

			mockTrace := buildlogger.NewMockTrace(t)
			mockTrace.EXPECT().IsStdout().Return(true).Once()
			mockTrace.EXPECT().Write(mock.Anything).Return(0, nil)

			prepareOptions := common.ExecutorPrepareOptions{
				Config:      runnerConfig,
				Build:       build,
				BuildLogger: buildlogger.New(mockTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{}),
			}

			err := executor.Prepare(prepareOptions)
			require.NoError(t, err)

			err = executor.setupBuildPod(t.Context(), []api.Container{})
			require.NoError(t, err)

			// get all pods we've observed create requests for
			pods := getActionObjects[*api.Pod](fakeKubeClient.Actions(), "create")
			require.Len(t, pods, 1, "expected to observe exactly 1 pod creation")

			pod := pods[0]
			for containerName, expectedPullPolicy := range tc.ExpectedPullPolicyPerContainer {
				container, err := containerByName(pod.Spec.Containers, containerName)
				require.NoError(t, err, "container not found on pod")

				actualPullPolicy := container.ImagePullPolicy
				assert.Equal(t, expectedPullPolicy, actualPullPolicy, "expected pull policy %q on container %q, but got %q", expectedPullPolicy, containerName, actualPullPolicy)
			}
		})
	}
}

func TestNoContainerEnvDups(t *testing.T) {
	const (
		varName        = "duplicateVar"
		varValRunner   = "runner.toml"
		varValPipeline = ".gitlab-ci.yaml"
	)

	fakeKubeClient := testclient.NewClientset()

	build := &common.Build{
		Job: spec.Job{
			Variables: spec.Variables{
				spec.Variable{Key: varName, Value: varValPipeline, Public: true},
			},
		},
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Environment: []string{
					varName + "=" + varValRunner,
				},
				Kubernetes: &common.KubernetesConfig{
					Image: "some-build-image",
				},
			},
		},
	}

	executor := newExecutor()
	executor.newKubeClient = func(_ *restclient.Config) (kubernetes.Interface, error) {
		return fakeKubeClient, nil
	}
	executor.getKubeConfig = func(_ *common.KubernetesConfig, _ *overwrites) (*restclient.Config, error) {
		return nil, nil
	}

	mockTrace := buildlogger.NewMockTrace(t)
	mockTrace.EXPECT().IsStdout().Return(true).Once()
	mockTrace.EXPECT().Write(mock.Anything).Return(0, nil)

	prepareOptions := common.ExecutorPrepareOptions{
		Config:      build.Runner,
		Build:       build,
		BuildLogger: buildlogger.New(mockTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{}),
	}

	err := executor.Prepare(prepareOptions)
	require.NoError(t, err)

	fakeKubeClient.PrependReactor("*", "pods", func(action k8stesting.Action) (handled bool, ret kuberuntime.Object, err error) {
		pod := action.(k8stesting.CreateAction).GetObject().(*api.Pod)

		for _, container := range slices.Concat(pod.Spec.Containers, pod.Spec.InitContainers, pod.Spec.InitContainers) {
			seen := map[string]struct{}{}
			for _, envVar := range container.Env {
				if _, ok := seen[envVar.Name]; ok {
					assert.Fail(t, "duplicate env var", "env var %q already set on container %s", envVar.Name, container.Name)
				}
				seen[envVar.Name] = struct{}{}

				if envVar.Name == varName {
					assert.Equal(t, varValPipeline, envVar.Value, "expected for env vars from the pipeline to win over ones from the runner config")
				}
			}
		}

		return false, nil, nil
	})

	err = executor.setupBuildPod(t.Context(), []api.Container{})
	require.NoError(t, err)
}

func getActionObjects[T kuberuntime.Object](actions []k8stesting.Action, verb string) []T {
	res := []T{}

	for _, action := range actions {
		if action.GetVerb() != verb {
			continue
		}

		objectAction, ok := action.(interface {
			GetObject() kuberuntime.Object
		})
		if !ok {
			continue
		}

		obj, ok := objectAction.GetObject().(T)
		if !ok {
			continue
		}

		res = append(res, obj)
	}

	return res
}

func containerByName(containers []api.Container, name string) (api.Container, error) {
	availableContainers := make([]string, len(containers))

	for i, c := range containers {
		availableContainers[i] = c.Name
		if c.Name == name {
			return c, nil
		}
	}

	return api.Container{}, fmt.Errorf("container %q not found, available containers: %v", name, availableContainers)
}

func TestGetContainerUidGid(t *testing.T) {
	tests := []struct {
		name            string
		jobUser         string
		securityContext common.KubernetesContainerSecurityContext
		allowedUsers    []string
		allowedGroups   []string
		expectedUID     int64
		expectedGID     int64
		expectWarning   string
	}{
		{
			name:        "no user specified anywhere",
			jobUser:     "",
			expectedUID: -1,
			expectedGID: -1,
		},
		{
			name:        "job user only - valid",
			jobUser:     "1000:1001",
			expectedUID: 1000,
			expectedGID: 1001,
		},
		{
			name:        "job user only - user part only",
			jobUser:     "1000",
			expectedUID: 1000,
			expectedGID: -1,
		},
		{
			name:          "job user blocked by allowlist",
			jobUser:       "1000:1001",
			allowedUsers:  []string{"2000"},
			expectedUID:   -1, // Validation failure returns -1
			expectedGID:   1001,
			expectWarning: "user \"1000\" is not in the allowed list:",
		},
		{
			name:          "job group blocked by allowlist",
			jobUser:       "1000:1001",
			allowedUsers:  []string{"1000"},
			allowedGroups: []string{"2001"},
			expectedUID:   1000,
			expectedGID:   -1, // Validation failure returns -1
			expectWarning: "group \"1001\" is not in the allowed list:",
		},
		{
			name:    "security context overrides job user",
			jobUser: "1000:1001",
			securityContext: common.KubernetesContainerSecurityContext{
				RunAsUser:  common.Int64Ptr(2000),
				RunAsGroup: common.Int64Ptr(2001),
			},
			expectedUID: 2000,
			expectedGID: 2001,
		},
		{
			name:    "security context user not validated against allowlist",
			jobUser: "1000:1001",
			securityContext: common.KubernetesContainerSecurityContext{
				RunAsUser: common.Int64Ptr(2000),
			},
			allowedUsers: []string{"1000"},
			expectedUID:  2000,
			expectedGID:  1001,
		},
		{
			name:    "security context root allowed (admin override)",
			jobUser: "1000:1001",
			securityContext: common.KubernetesContainerSecurityContext{
				RunAsUser: common.Int64Ptr(0),
			},
			allowedUsers: []string{"1000", "65534"},
			expectedUID:  0,
			expectedGID:  1001,
		},
		{
			name:    "security context partial override - user only",
			jobUser: "1000:1001",
			securityContext: common.KubernetesContainerSecurityContext{
				RunAsUser: common.Int64Ptr(2000),
			},
			expectedUID: 2000,
			expectedGID: 1001,
		},
		{
			name:    "security context partial override - group only",
			jobUser: "1000:1001",
			securityContext: common.KubernetesContainerSecurityContext{
				RunAsGroup: common.Int64Ptr(2001),
			},
			expectedUID: 1000,
			expectedGID: 2001,
		},
		{
			name:          "invalid job user format",
			jobUser:       "invalid:1001",
			expectedUID:   -1, // Parsing failure returns -1
			expectedGID:   1001,
			expectWarning: "failed to parse UID",
		},
		{
			name:          "invalid job group format",
			jobUser:       "1000:invalid",
			expectedUID:   1000,
			expectedGID:   -1, // Parsing failure returns -1
			expectWarning: "failed to parse GID",
		},
		{
			name:    "root user blocked by default (no allowlist)",
			jobUser: "0:0",
			// No allowedUsers = root blocked, non-root allowed
			expectedUID:   -1, // Validation failure returns -1
			expectedGID:   -1, // Root group also blocked by default
			expectWarning: "user \"0\" is not in the allowed list:",
		},
		{
			name:          "root user blocked by allowlist",
			jobUser:       "0:0",
			allowedUsers:  []string{"1000", "65534"}, // Root (0) not in list
			expectedUID:   -1,                        // Validation failure returns -1
			expectedGID:   -1,                        // Root group also blocked by default
			expectWarning: "user \"0\" is not in the allowed list:",
		},
		{
			name:          "root user explicitly allowed in allowlist",
			jobUser:       "0:0",
			allowedUsers:  []string{"0", "1000", "65534"}, // Root (0) explicitly allowed
			allowedGroups: []string{"0", "1000", "65534"}, // Root (0) explicitly allowed
			expectedUID:   0,
			expectedGID:   0,
		},
		{
			name:    "root group blocked by default (no allowlist)",
			jobUser: "1000:0", // Non-root user, root group
			// No allowedGroups = root group blocked, non-root groups allowed
			expectedUID:   1000,
			expectedGID:   -1, // Validation failure returns -1
			expectWarning: "group \"0\" is not in the allowed list:",
		},
		{
			name:        "non-root user allowed without allowlist",
			jobUser:     "1000:1001",
			expectedUID: 1000,
			expectedGID: 1001,
		},
		{
			name:    "backwards compatibility - non-root security context without allowed_users",
			jobUser: "1000:1001",
			securityContext: common.KubernetesContainerSecurityContext{
				RunAsUser:  common.Int64Ptr(1000),
				RunAsGroup: common.Int64Ptr(1000),
			},
			expectedUID: 1000,
			expectedGID: 1000,
		},
		{
			name:    "security context bypasses user allowlist completely",
			jobUser: "1000:1001",
			securityContext: common.KubernetesContainerSecurityContext{
				RunAsUser: common.Int64Ptr(9999),
			},
			allowedUsers: []string{"1000", "2000"},
			expectedUID:  9999,
			expectedGID:  1001,
		},
		{
			name:    "security context bypasses group allowlist completely",
			jobUser: "1000:1001",
			securityContext: common.KubernetesContainerSecurityContext{
				RunAsGroup: common.Int64Ptr(9999),
			},
			allowedGroups: []string{"1001", "2001"},
			expectedUID:   1000,
			expectedGID:   9999,
		},
		{
			name:    "security context can set both root uid and gid despite empty allowlists",
			jobUser: "",
			securityContext: common.KubernetesContainerSecurityContext{
				RunAsUser:  common.Int64Ptr(0),
				RunAsGroup: common.Int64Ptr(0),
			},
			expectedUID: 0,
			expectedGID: 0,
		},
		{
			name:        "empty job user with colon returns unset values",
			jobUser:     ":",
			expectedUID: -1,
			expectedGID: -1,
		},
		{
			name:          "job user with multiple colons causes parse error",
			jobUser:       "1000:1001:1002",
			expectedUID:   1000,
			expectedGID:   -1, // Parsing failure returns -1
			expectWarning: "failed to parse GID",
		},
		{
			name:          "negative job user uid not allowed",
			jobUser:       "-100:1000",
			expectedUID:   -1, // Parsing failure returns -1
			expectedGID:   1000,
			expectWarning: "failed to parse UID: negative values not allowed",
		},
		{
			name:          "negative job group gid not allowed",
			jobUser:       "1000:-100",
			expectedUID:   1000,
			expectedGID:   -1, // Parsing failure returns -1
			expectWarning: "failed to parse GID: negative values not allowed",
		},
		{
			name:    "invalid job user format with container security context warns and continues",
			jobUser: "invalid:1000",
			securityContext: common.KubernetesContainerSecurityContext{
				RunAsUser: common.Int64Ptr(2000),
			},
			expectedUID:   2000,
			expectedGID:   1000,
			expectWarning: "Overriding user for container \"build\" to \"invalid\" is not allowed: user is set to 2000 in container security context",
		},
		{
			name:    "invalid job group format with container security context warns and continues",
			jobUser: "1000:invalid",
			securityContext: common.KubernetesContainerSecurityContext{
				RunAsGroup: common.Int64Ptr(2000),
			},
			expectedUID:   1000,
			expectedGID:   2000,
			expectWarning: "Overriding group for container \"build\" to \"invalid\" is not allowed: group is set to 2000 in container security context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := newExecutor()
			executor.Config.RunnerSettings.Kubernetes.AllowedUsers = tt.allowedUsers
			executor.Config.RunnerSettings.Kubernetes.AllowedGroups = tt.allowedGroups

			// Set up build logger to capture warnings
			var logOutput strings.Builder
			buildTrace := FakeBuildTrace{
				testWriter: testWriter{
					call: func(b []byte) (int, error) {
						logOutput.Write(b)
						return len(b), nil
					},
				},
			}
			executor.BuildLogger = buildlogger.New(buildTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})

			uid, gid := executor.getContainerUIDGID(tt.jobUser, "build", tt.securityContext)

			assert.Equal(t, tt.expectedUID, uid)
			assert.Equal(t, tt.expectedGID, gid)

			// Check for expected warnings
			if tt.expectWarning != "" {
				assert.Contains(t, logOutput.String(), tt.expectWarning,
					"Expected warning message not found in log output: %s", logOutput.String())
			}
		})
	}
}

func TestGetContainerUIDGIDWithPodSecurityContext(t *testing.T) {
	tests := []struct {
		name                     string
		jobUser                  string
		containerSecurityContext common.KubernetesContainerSecurityContext
		podSecurityContext       common.KubernetesPodSecurityContext
		allowedUsers             []string
		allowedGroups            []string
		expectedUID              int64
		expectedGID              int64
		expectWarning            string
	}{
		{
			name:    "pod security context provides defaults when job and container have no values",
			jobUser: "",
			podSecurityContext: common.KubernetesPodSecurityContext{
				RunAsUser:  common.Int64Ptr(2000),
				RunAsGroup: common.Int64Ptr(2001),
			},
			expectedUID: 2000,
			expectedGID: 2001,
		},
		{
			name:    "container security context overrides pod security context",
			jobUser: "",
			containerSecurityContext: common.KubernetesContainerSecurityContext{
				RunAsUser:  common.Int64Ptr(3000),
				RunAsGroup: common.Int64Ptr(3001),
			},
			podSecurityContext: common.KubernetesPodSecurityContext{
				RunAsUser:  common.Int64Ptr(2000),
				RunAsGroup: common.Int64Ptr(2001),
			},
			expectedUID: 3000,
			expectedGID: 3001,
		},
		{
			name:    "pod security context overrides job configuration",
			jobUser: "1000:1001",
			podSecurityContext: common.KubernetesPodSecurityContext{
				RunAsUser:  common.Int64Ptr(2000),
				RunAsGroup: common.Int64Ptr(2001),
			},
			expectedUID: 2000,
			expectedGID: 2001,
		},
		{
			name:    "container security context overrides both pod and job",
			jobUser: "1000:1001",
			containerSecurityContext: common.KubernetesContainerSecurityContext{
				RunAsUser:  common.Int64Ptr(3000),
				RunAsGroup: common.Int64Ptr(3001),
			},
			podSecurityContext: common.KubernetesPodSecurityContext{
				RunAsUser:  common.Int64Ptr(2000),
				RunAsGroup: common.Int64Ptr(2001),
			},
			expectedUID: 3000,
			expectedGID: 3001,
		},
		{
			name:    "partial container override - user only, pod provides group",
			jobUser: "1000:1001",
			containerSecurityContext: common.KubernetesContainerSecurityContext{
				RunAsUser: common.Int64Ptr(3000),
			},
			podSecurityContext: common.KubernetesPodSecurityContext{
				RunAsUser:  common.Int64Ptr(2000),
				RunAsGroup: common.Int64Ptr(2001),
			},
			expectedUID: 3000,
			expectedGID: 2001,
		},
		{
			name:    "partial pod override - group only, job provides user",
			jobUser: "1000:1001",
			podSecurityContext: common.KubernetesPodSecurityContext{
				RunAsGroup: common.Int64Ptr(2001),
			},
			expectedUID: 1000,
			expectedGID: 2001,
		},
		{
			name:    "pod security context bypasses job user allowlist validation",
			jobUser: "1000:1001",
			podSecurityContext: common.KubernetesPodSecurityContext{
				RunAsUser:  common.Int64Ptr(9999),
				RunAsGroup: common.Int64Ptr(9998),
			},
			allowedUsers:  []string{"1000", "2000"},
			allowedGroups: []string{"1001", "2001"},
			expectedUID:   9999,
			expectedGID:   9998,
		},
		{
			name:    "container security context bypasses validation while pod provides fallback",
			jobUser: "",
			containerSecurityContext: common.KubernetesContainerSecurityContext{
				RunAsUser: common.Int64Ptr(0), // root - normally blocked
			},
			podSecurityContext: common.KubernetesPodSecurityContext{
				RunAsUser:  common.Int64Ptr(2000),
				RunAsGroup: common.Int64Ptr(2001),
			},
			allowedUsers: []string{"1000", "65534"}, // root not allowed
			expectedUID:  0,                         // container overrides and bypasses validation
			expectedGID:  2001,                      // pod provides group
		},
		{
			name:          "job validation still applies when no pod/container values provided",
			jobUser:       "9999:9998",
			allowedUsers:  []string{"1000", "2000"},
			expectedUID:   -1,   // User validation failure returns -1
			expectedGID:   9998, // Group validation succeeds (no allowedGroups restriction, 9998 is non-root)
			expectWarning: "user \"9999\" is not in the allowed list:",
		},
		{
			name:    "pod allows root despite default blocking (admin override)",
			jobUser: "",
			podSecurityContext: common.KubernetesPodSecurityContext{
				RunAsUser:  common.Int64Ptr(0),
				RunAsGroup: common.Int64Ptr(0),
			},
			expectedUID: 0,
			expectedGID: 0,
		},
		{
			name:    "precedence test: all three sources provided",
			jobUser: "1000:1001",
			containerSecurityContext: common.KubernetesContainerSecurityContext{
				RunAsGroup: common.Int64Ptr(3001), // only group at container level
			},
			podSecurityContext: common.KubernetesPodSecurityContext{
				RunAsUser:  common.Int64Ptr(2000),
				RunAsGroup: common.Int64Ptr(2001),
			},
			expectedUID: 2000, // pod user (container doesn't provide)
			expectedGID: 3001, // container group (overrides pod)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := newExecutor()
			executor.Config.RunnerSettings.Kubernetes.AllowedUsers = tt.allowedUsers
			executor.Config.RunnerSettings.Kubernetes.AllowedGroups = tt.allowedGroups
			executor.Config.RunnerSettings.Kubernetes.PodSecurityContext = tt.podSecurityContext

			// Set up build logger to capture warnings
			var logOutput strings.Builder
			buildTrace := FakeBuildTrace{
				testWriter: testWriter{
					call: func(b []byte) (int, error) {
						logOutput.Write(b)
						return len(b), nil
					},
				},
			}
			executor.BuildLogger = buildlogger.New(buildTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})

			uid, gid := executor.getContainerUIDGID(tt.jobUser, "build", tt.containerSecurityContext)
			assert.Equal(t, tt.expectedUID, uid)
			assert.Equal(t, tt.expectedGID, gid)

			// Check for expected warnings
			if tt.expectWarning != "" {
				assert.Contains(t, logOutput.String(), tt.expectWarning,
					"Expected warning message not found in log output: %s", logOutput.String())
			}
		})
	}
}

func TestGetContainerUIDGIDLogging(t *testing.T) {
	tests := []struct {
		name                     string
		jobUser                  string
		containerName            string
		containerSecurityContext common.KubernetesContainerSecurityContext
		podSecurityContext       common.KubernetesPodSecurityContext
		expectedLogPattern       string
		expectNoLog              bool
	}{
		{
			name:          "container security context overrides job user",
			jobUser:       "1000:1001",
			containerName: "build",
			containerSecurityContext: common.KubernetesContainerSecurityContext{
				RunAsUser:  common.Int64Ptr(2000),
				RunAsGroup: common.Int64Ptr(2001),
			},
			expectedLogPattern: `Overriding user for container "build" to "1000" is not allowed: user is set to 2000 in container security context` + "\n" +
				`Overriding group for container "build" to "1001" is not allowed: group is set to 2001 in container security context`,
		},
		{
			name:          "pod security context overrides job user",
			jobUser:       "1000:1001",
			containerName: "helper",
			podSecurityContext: common.KubernetesPodSecurityContext{
				RunAsUser:  common.Int64Ptr(3000),
				RunAsGroup: common.Int64Ptr(3001),
			},
			expectedLogPattern: `Overriding user for container "helper" to "1000" is not allowed: user is set to 3000 in pod security context` + "\n" +
				`Overriding group for container "helper" to "1001" is not allowed: group is set to 3001 in pod security context`,
		},
		{
			name:          "only user is overridden by container context",
			jobUser:       "1000:1001",
			containerName: "service-0",
			containerSecurityContext: common.KubernetesContainerSecurityContext{
				RunAsUser: common.Int64Ptr(2000),
				// No group override
			},
			expectedLogPattern: `Overriding user for container "service-0" to "1000" is not allowed: user is set to 2000 in container security context` + "\n",
		},
		{
			name:          "only group is overridden by pod context",
			jobUser:       "1000:1001",
			containerName: "service-1",
			podSecurityContext: common.KubernetesPodSecurityContext{
				RunAsGroup: common.Int64Ptr(3001),
				// No user override
			},
			expectedLogPattern: `Overriding group for container "service-1" to "1001" is not allowed: group is set to 3001 in pod security context`,
		},
		{
			name:          "no logging when job user is empty",
			jobUser:       "",
			containerName: "build",
			containerSecurityContext: common.KubernetesContainerSecurityContext{
				RunAsUser:  common.Int64Ptr(2000),
				RunAsGroup: common.Int64Ptr(2001),
			},
			expectNoLog: true,
		},
		{
			name:          "no logging when values come from job",
			jobUser:       "1000:1001",
			containerName: "build",
			// No container or pod security context
			expectNoLog: true,
		},
		{
			name:          "container context takes precedence over pod context",
			jobUser:       "1000:1001",
			containerName: "build",
			containerSecurityContext: common.KubernetesContainerSecurityContext{
				RunAsUser:  common.Int64Ptr(4000),
				RunAsGroup: common.Int64Ptr(4001),
			},
			podSecurityContext: common.KubernetesPodSecurityContext{
				RunAsUser:  common.Int64Ptr(3000),
				RunAsGroup: common.Int64Ptr(3001),
			},
			expectedLogPattern: `Overriding user for container "build" to "1000" is not allowed: user is set to 4000 in container security context` + "\n" +
				`Overriding group for container "build" to "1001" is not allowed: group is set to 4001 in container security context`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a buffer to capture log output
			var logBuffer bytes.Buffer
			logger := logrus.New()
			logger.SetOutput(&logBuffer)

			// Create trace that writes to buffer
			trace := &common.Trace{Writer: &logBuffer}

			executor := newExecutor()
			executor.Config.RunnerSettings.Kubernetes.PodSecurityContext = tt.podSecurityContext
			executor.BuildLogger = buildlogger.New(trace, logrus.NewEntry(logger), buildlogger.Options{})

			uid, gid := executor.getContainerUIDGID(tt.jobUser, tt.containerName, tt.containerSecurityContext)
			// Verify that we get valid UIDs/GIDs
			assert.True(t, uid >= -1)
			assert.True(t, gid >= -1)

			logOutput := logBuffer.String()
			// Strip ANSI escape sequences for easier comparison
			re := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
			cleanLogOutput := re.ReplaceAllString(logOutput, "")

			if tt.expectNoLog {
				// Verify no override messages are logged
				assert.NotContains(t, cleanLogOutput, "Overriding user")
				assert.NotContains(t, cleanLogOutput, "Overriding group")
			} else {
				// Verify the expected log pattern is present
				assert.Contains(t, cleanLogOutput, tt.expectedLogPattern)
			}
		})
	}
}

func TestPickSecurityContextID(t *testing.T) {
	tests := []struct {
		name           string
		containerRunAs *int64
		podRunAs       *int64
		jobRunAs       func() int64
		expectedID     int64
		expectedSource securityContextIDSource
	}{
		{
			name:           "container value takes precedence",
			containerRunAs: common.Int64Ptr(1000),
			podRunAs:       common.Int64Ptr(2000),
			jobRunAs: func() int64 {
				return 3000
			},
			expectedID:     1000,
			expectedSource: securityContextIDSourceContainer,
		},
		{
			name:     "pod value used when container is nil",
			podRunAs: common.Int64Ptr(2000),
			jobRunAs: func() int64 {
				return 3000
			},
			expectedID:     2000,
			expectedSource: securityContextIDSourcePod,
		},
		{
			name: "job value used when container and pod are nil",
			jobRunAs: func() int64 {
				return 3000
			},
			expectedID:     3000,
			expectedSource: securityContextIDSourceJob,
		},
		{
			name:     "job parsing failure returns failed value when container and pod are nil",
			podRunAs: nil,
			jobRunAs: func() int64 {
				// Simulate parsing failure by returning -1 or 0
				return 0
			},
			expectedID:     0,
			expectedSource: securityContextIDSourceJob,
		},
		{
			name:           "container value bypasses job parsing failure",
			containerRunAs: common.Int64Ptr(1000),
			jobRunAs: func() int64 {
				// This would normally fail parsing, but container value takes precedence
				return 0
			},
			expectedID:     1000,
			expectedSource: securityContextIDSourceContainer,
		},
		{
			name:     "pod value bypasses job parsing failure",
			podRunAs: common.Int64Ptr(2000),
			jobRunAs: func() int64 {
				// This would normally fail parsing, but pod value takes precedence
				return 0
			},
			expectedID:     2000,
			expectedSource: securityContextIDSourcePod,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := newExecutor()

			id, source := executor.pickSecurityContextID(tt.containerRunAs, tt.podRunAs, tt.jobRunAs)

			assert.Equal(t, tt.expectedID, id)
			if tt.expectedSource != "" {
				assert.Equal(t, tt.expectedSource, source)
			}
		})
	}
}

func TestHelperContainerDoesNotInheritJobUser(t *testing.T) {
	tests := []struct {
		name                  string
		helperSecurityContext common.KubernetesContainerSecurityContext
		allowedUsers          []string
		allowedGroups         []string
		expectedRunAsUser     *int64
		expectedRunAsGroup    *int64
	}{
		{
			name: "helper container gets no user when no security context provided",
		},
		{
			name: "helper container uses its own security context",
			helperSecurityContext: common.KubernetesContainerSecurityContext{
				RunAsUser:  common.Int64Ptr(2000),
				RunAsGroup: common.Int64Ptr(2001),
			},
			expectedRunAsUser:  common.Int64Ptr(2000),
			expectedRunAsGroup: common.Int64Ptr(2001),
		},
		{
			name:         "helper container can run as root via security context despite allowlist",
			allowedUsers: []string{"1000", "1001"},
			helperSecurityContext: common.KubernetesContainerSecurityContext{
				RunAsUser:  common.Int64Ptr(0),
				RunAsGroup: common.Int64Ptr(0),
			},
			expectedRunAsUser:  common.Int64Ptr(0),
			expectedRunAsGroup: common.Int64Ptr(0),
		},
		{
			name:          "helper container bypasses both user and group allowlists",
			allowedUsers:  []string{"1000"},
			allowedGroups: []string{"1001"},
			helperSecurityContext: common.KubernetesContainerSecurityContext{
				RunAsUser:  common.Int64Ptr(9999),
				RunAsGroup: common.Int64Ptr(8888),
			},
			expectedRunAsUser:  common.Int64Ptr(9999),
			expectedRunAsGroup: common.Int64Ptr(8888),
		},
		{
			name: "helper container with only user in security context",
			helperSecurityContext: common.KubernetesContainerSecurityContext{
				RunAsUser: common.Int64Ptr(5000),
			},
			expectedRunAsUser: common.Int64Ptr(5000),
		},
		{
			name: "helper container with only group in security context",
			helperSecurityContext: common.KubernetesContainerSecurityContext{
				RunAsGroup: common.Int64Ptr(5001),
			},
			expectedRunAsGroup: common.Int64Ptr(5001),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := newExecutor()
			executor.Config.RunnerSettings.Kubernetes.AllowedUsers = tt.allowedUsers
			executor.Config.RunnerSettings.Kubernetes.AllowedGroups = tt.allowedGroups

			ctx := executor.getSecurityContextWithUIDGID("", "helper", tt.helperSecurityContext)

			if tt.expectedRunAsUser != nil {
				assert.Equal(t, *tt.expectedRunAsUser, *ctx.RunAsUser)
			} else {
				assert.Nil(t, ctx.RunAsUser)
			}

			if tt.expectedRunAsGroup != nil {
				assert.Equal(t, *tt.expectedRunAsGroup, *ctx.RunAsGroup)
			} else {
				assert.Nil(t, ctx.RunAsGroup)
			}
		})
	}
}

func TestServiceContainerSecurityContext(t *testing.T) {
	tests := []struct {
		name                   string
		jobUser                string
		serviceSecurityContext common.KubernetesContainerSecurityContext
		allowedUsers           []string
		allowedGroups          []string
		expectedRunAsUser      *int64
		expectedRunAsGroup     *int64
		expectWarning          string
	}{
		{
			name:               "service container inherits job user when no security context",
			jobUser:            "1000:1001",
			expectedRunAsUser:  common.Int64Ptr(1000),
			expectedRunAsGroup: common.Int64Ptr(1001),
		},
		{
			name:    "service container security context overrides job user",
			jobUser: "1000:1001",
			serviceSecurityContext: common.KubernetesContainerSecurityContext{
				RunAsUser:  common.Int64Ptr(3000),
				RunAsGroup: common.Int64Ptr(3001),
			},
			expectedRunAsUser:  common.Int64Ptr(3000),
			expectedRunAsGroup: common.Int64Ptr(3001),
		},
		{
			name:         "service container bypasses allowlist with security context",
			allowedUsers: []string{"1000"},
			serviceSecurityContext: common.KubernetesContainerSecurityContext{
				RunAsUser: common.Int64Ptr(9999),
			},
			expectedRunAsUser: common.Int64Ptr(9999),
		},
		{
			name:               "service container job user blocked by allowlist",
			jobUser:            "2000:2001",
			allowedUsers:       []string{"1000"},
			expectedRunAsUser:  nil,                   // Validation failure returns -1, which doesn't get set
			expectedRunAsGroup: common.Int64Ptr(2001), // Group validation succeeds (no allowedGroups restriction)
			expectWarning:      "user \"2000\" is not in the allowed list:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := newExecutor()
			executor.Config.RunnerSettings.Kubernetes.AllowedUsers = tt.allowedUsers
			executor.Config.RunnerSettings.Kubernetes.AllowedGroups = tt.allowedGroups

			// Set up build logger to capture warnings
			var logOutput strings.Builder
			buildTrace := FakeBuildTrace{
				testWriter: testWriter{
					call: func(b []byte) (int, error) {
						logOutput.Write(b)
						return len(b), nil
					},
				},
			}
			executor.BuildLogger = buildlogger.New(buildTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})

			ctx := executor.getSecurityContextWithUIDGID(tt.jobUser, "service", tt.serviceSecurityContext)

			require.NotNil(t, ctx)

			// Check for expected warnings
			if tt.expectWarning != "" {
				assert.Contains(t, logOutput.String(), tt.expectWarning,
					"Expected warning message not found in log output: %s", logOutput.String())
			}

			if tt.expectedRunAsUser != nil {
				require.NotNil(t, ctx.RunAsUser, "expected RunAsUser to be set")
				assert.Equal(t, *tt.expectedRunAsUser, *ctx.RunAsUser)
			} else {
				assert.Nil(t, ctx.RunAsUser)
			}

			if tt.expectedRunAsGroup != nil {
				require.NotNil(t, ctx.RunAsGroup, "expected RunAsGroup to be set")
				assert.Equal(t, *tt.expectedRunAsGroup, *ctx.RunAsGroup)
			} else {
				assert.Nil(t, ctx.RunAsGroup)
			}
		})
	}
}

func TestGetSecurityContextWithUidGid(t *testing.T) {
	tests := []struct {
		name               string
		jobUser            string
		securityContext    common.KubernetesContainerSecurityContext
		allowedUsers       []string
		allowedGroups      []string
		expectedRunAsUser  *int64
		expectedRunAsGroup *int64
		expectWarning      string
	}{
		{
			name:    "no user specified",
			jobUser: "",
		},
		{
			name:               "job user applied to security context",
			jobUser:            "1000:1001",
			expectedRunAsUser:  common.Int64Ptr(1000),
			expectedRunAsGroup: common.Int64Ptr(1001),
		},
		{
			name:    "security context overrides job user",
			jobUser: "1000:1001",
			securityContext: common.KubernetesContainerSecurityContext{
				RunAsUser:  common.Int64Ptr(2000),
				RunAsGroup: common.Int64Ptr(2001),
			},
			expectedRunAsUser:  common.Int64Ptr(2000),
			expectedRunAsGroup: common.Int64Ptr(2001),
		},
		{
			name:               "user validation error propagated",
			jobUser:            "1000",
			allowedUsers:       []string{"2000"},
			expectedRunAsUser:  nil, // Validation failure returns -1, which doesn't get set
			expectedRunAsGroup: nil,
			expectWarning:      "user \"1000\" is not in the allowed list:",
		},
		{
			name:               "root user from job blocked by allowlist",
			jobUser:            "0:0",
			allowedUsers:       []string{"1000", "65534"},
			expectedRunAsUser:  nil, // Validation failure returns -1, which doesn't get set
			expectedRunAsGroup: nil, // Validation failure returns -1, which doesn't get set
			expectWarning:      "user \"0\" is not in the allowed list:",
		},
		{
			name:               "backwards compatibility - non-root allowed without allowed_users",
			jobUser:            "1000:1000",
			expectedRunAsUser:  common.Int64Ptr(1000),
			expectedRunAsGroup: common.Int64Ptr(1000),
		},
		{
			name:         "security context can set any UID despite restrictive allowlist",
			allowedUsers: []string{"1000"},
			securityContext: common.KubernetesContainerSecurityContext{
				RunAsUser: common.Int64Ptr(9999),
			},
			expectedRunAsUser: common.Int64Ptr(9999),
		},
		{
			name:          "security context can set any GID despite restrictive allowlist",
			allowedGroups: []string{"1001"},
			securityContext: common.KubernetesContainerSecurityContext{
				RunAsGroup: common.Int64Ptr(8888),
			},
			expectedRunAsGroup: common.Int64Ptr(8888),
		},
		{
			name: "security context root bypasses empty allowlist",
			securityContext: common.KubernetesContainerSecurityContext{
				RunAsUser:  common.Int64Ptr(0),
				RunAsGroup: common.Int64Ptr(0),
			},
			expectedRunAsUser:  common.Int64Ptr(0),
			expectedRunAsGroup: common.Int64Ptr(0),
		},
		{
			name:    "partial security context with job user fallback",
			jobUser: "1000:1001",
			securityContext: common.KubernetesContainerSecurityContext{
				RunAsUser: common.Int64Ptr(5000),
			},
			expectedRunAsUser:  common.Int64Ptr(5000),
			expectedRunAsGroup: common.Int64Ptr(1001),
		},
		{
			name:               "job group validation with restrictive allowlist",
			jobUser:            "1000:2000",
			allowedUsers:       []string{"1000"},
			allowedGroups:      []string{"3000"},
			expectedRunAsUser:  common.Int64Ptr(1000), // User validation succeeds
			expectedRunAsGroup: nil,                   // Group validation failure returns -1, which doesn't get set
			expectWarning:      "group \"2000\" is not in the allowed list:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := newExecutor()
			executor.Config.RunnerSettings.Kubernetes.AllowedUsers = tt.allowedUsers
			executor.Config.RunnerSettings.Kubernetes.AllowedGroups = tt.allowedGroups

			// Set up build logger to capture warnings
			var logOutput strings.Builder
			buildTrace := FakeBuildTrace{
				testWriter: testWriter{
					call: func(b []byte) (int, error) {
						logOutput.Write(b)
						return len(b), nil
					},
				},
			}
			executor.BuildLogger = buildlogger.New(buildTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})

			ctx := executor.getSecurityContextWithUIDGID(tt.jobUser, "build", tt.securityContext)
			require.NotNil(t, ctx)

			// Check for expected warnings
			if tt.expectWarning != "" {
				assert.Contains(t, logOutput.String(), tt.expectWarning,
					"Expected warning message not found in log output: %s", logOutput.String())
			}

			if tt.expectedRunAsUser != nil {
				require.NotNil(t, ctx.RunAsUser, "expected RunAsUser to be set")
				assert.Equal(t, *tt.expectedRunAsUser, *ctx.RunAsUser)
			} else {
				assert.Nil(t, ctx.RunAsUser)
			}

			if tt.expectedRunAsGroup != nil {
				require.NotNil(t, ctx.RunAsGroup, "expected RunAsGroup to be set")
				assert.Equal(t, *tt.expectedRunAsGroup, *ctx.RunAsGroup)
			} else {
				assert.Nil(t, ctx.RunAsGroup)
			}
		})
	}
}

type podWarningEventsScenario struct {
	logs     string
	executor *executor
	logged   []*api.Event
	ignored  []*api.Event
}

func preparePodWarningEventsScenario(t *testing.T) podWarningEventsScenario {
	t.Helper()

	newEvent := func(uid, message string, ts time.Time, count int32) *api.Event {
		return &api.Event{
			ObjectMeta: metav1.ObjectMeta{
				UID:               types.UID(uid),
				Name:              uid,
				Namespace:         "default",
				CreationTimestamp: metav1.NewTime(ts),
			},
			InvolvedObject: api.ObjectReference{Name: "test-pod"},
			Type:           "Warning",
			Message:        message,
			LastTimestamp:  metav1.NewTime(ts),
			Count:          count,
		}
	}

	now := time.Now()
	eventOld := newEvent("old", "should skip old but its not in the cache", now.Add(-25*time.Minute), 1)
	eventSeen := newEvent("seen", "already seen which means it is in the cache", now.Add(-10*time.Minute), 1)
	dupFirst := newEvent("dup", "duplicate allowed", now.Add(-6*time.Minute), 1)
	firstNew := newEvent("new-1", "first new", now.Add(-5*time.Minute), 1)
	dupAggregated := newEvent("dup", "duplicate aggregated update should also log", now.Add(-4*time.Minute), 2)
	secondNew := newEvent("new-2", "second new", now.Add(-3*time.Minute), 1)

	pageOne := &api.EventList{
		Items: []api.Event{*eventOld, *eventSeen, *dupFirst, *dupAggregated},
		ListMeta: metav1.ListMeta{
			Continue: "page-2",
		},
	}
	pageTwo := &api.EventList{Items: []api.Event{*firstNew, *secondNew}}

	fakeClient := testclient.NewClientset()
	listCall := 0
	fakeClient.Fake.PrependReactor("list", "events", func(action k8stesting.Action) (bool, kuberuntime.Object, error) {
		listCall++
		switch listCall {
		case 1:
			return true, pageOne, nil
		case 2:
			return true, pageTwo, nil
		default:
			return true, &api.EventList{}, nil
		}
	})

	executor := newExecutor()
	executor.options = &kubernetesOptions{}
	executor.pod = &api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}
	executor.kubeClient = fakeClient

	var logOutput strings.Builder
	buildTrace := FakeBuildTrace{
		testWriter: testWriter{
			call: func(b []byte) (int, error) {
				logOutput.Write(b)
				return len(b), nil
			},
		},
	}
	executor.BuildLogger = buildlogger.New(buildTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})

	executor.initPodEventState()
	executor.podEventState.lastFetched = now.Add(-15 * time.Minute)
	executor.podEventState.seen.Add(eventKey(eventSeen), eventLastOccurredTimestamp(eventSeen))

	executor.logPodWarningEvents(t.Context(), "Warning")

	return podWarningEventsScenario{
		logs:     logOutput.String(),
		executor: executor,
		logged: []*api.Event{
			dupFirst,
			firstNew,
			dupAggregated,
			secondNew,
		},
		ignored: []*api.Event{
			eventOld,
			eventSeen,
		},
	}
}

func TestExecutor_logPodWarningEvents(t *testing.T) {
	t.Run("pod state is not initialized when no pod is present", func(t *testing.T) {
		executor := newExecutor()

		executor.logPodWarningEvents(t.Context(), "Warning")
		assert.Nil(t, executor.podEventState)
	})

	t.Run("initializes pod event state when missing", func(t *testing.T) {
		fakeClient := testclient.NewClientset()

		executor := newExecutor()
		executor.options = &kubernetesOptions{}
		executor.pod = &api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
			},
		}
		executor.kubeClient = fakeClient

		var logOutput strings.Builder
		buildTrace := FakeBuildTrace{
			testWriter: testWriter{
				call: func(b []byte) (int, error) {
					logOutput.Write(b)
					return len(b), nil
				},
			},
		}

		executor.BuildLogger = buildlogger.New(buildTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})

		executor.logPodWarningEvents(t.Context(), "Warning")

		assert.NotNil(t, executor.podEventState)
		assert.Empty(t, logOutput.String())
	})

	t.Run("filters paginated events and updates state", func(t *testing.T) {
		scenario := preparePodWarningEventsScenario(t)

		t.Run("logs only relevant warning events", func(t *testing.T) {
			logs := scenario.logs
			for _, ev := range scenario.logged {
				assert.Contains(t, logs, fmt.Sprintf("Event retrieved from the cluster: %s", ev.Message))
			}
		})

		t.Run("logs events in chronological order", func(t *testing.T) {
			logs := scenario.logs
			previous := -1
			for _, ev := range scenario.logged {
				idx := strings.Index(logs, ev.Message)
				require.NotEqual(t, -1, idx)
				assert.Greater(t, idx, previous, "event %s logged out of order", ev.Message)
				previous = idx
			}
		})

		t.Run("does not log old and already logged events", func(t *testing.T) {
			logs := scenario.logs
			for _, ev := range scenario.ignored {
				assert.NotContains(t, logs, ev.Message)
			}
		})

		t.Run("updates executor pod event state", func(t *testing.T) {
			for _, ev := range scenario.logged {
				assert.True(t, scenario.executor.podEventState.seen.Contains(eventKey(ev)), "event %s not marked as seen", ev.Message)
			}
			lastLogged := scenario.logged[len(scenario.logged)-1]
			assert.Equal(t, eventLastOccurredTimestamp(lastLogged), scenario.executor.podEventState.lastFetched)
		})
	})
}

func TestExecutor_logNewPodEvents(t *testing.T) {
	t.Run("sorts events and updates state", func(t *testing.T) {
		executor := newExecutor()

		var logOutput strings.Builder
		buildTrace := FakeBuildTrace{
			testWriter: testWriter{
				call: func(b []byte) (int, error) {
					logOutput.Write(b)
					return len(b), nil
				},
			},
		}

		executor.BuildLogger = buildlogger.New(buildTrace, logrus.WithFields(logrus.Fields{}), buildlogger.Options{})
		executor.initPodEventState()

		now := time.Now()
		earlier := &api.Event{
			ObjectMeta: metav1.ObjectMeta{
				UID:       "early",
				Name:      "early",
				Namespace: "default",
			},
			InvolvedObject: api.ObjectReference{Name: "test-pod"},
			Type:           "Warning",
			Message:        "earlier",
			LastTimestamp:  metav1.NewTime(now.Add(-2 * time.Minute)),
			Count:          1,
		}
		later := &api.Event{
			ObjectMeta: metav1.ObjectMeta{
				UID:       "later",
				Name:      "later",
				Namespace: "default",
			},
			InvolvedObject: api.ObjectReference{Name: "test-pod"},
			Type:           "Warning",
			Message:        "later",
			LastTimestamp:  metav1.NewTime(now.Add(-1 * time.Minute)),
			Count:          1,
		}

		executor.logNewPodEvents([]*api.Event{later, earlier})

		logs := logOutput.String()
		idxEarlier := strings.Index(logs, "earlier")
		idxLater := strings.Index(logs, "later")
		require.NotEqual(t, -1, idxEarlier)
		require.NotEqual(t, -1, idxLater)
		assert.Less(t, idxEarlier, idxLater)

		assert.True(t, executor.podEventState.seen.Contains(eventKey(earlier)))
		assert.True(t, executor.podEventState.seen.Contains(eventKey(later)))
		assert.Equal(t, eventLastOccurredTimestamp(later), executor.podEventState.lastFetched)
	})
}
