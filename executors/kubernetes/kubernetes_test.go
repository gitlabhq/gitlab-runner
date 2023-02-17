//go:build !integration

package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	api "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kuberuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/rest/fake"
	"k8s.io/client-go/util/exec"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/common/buildtest"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/executors/kubernetes/internal/pull"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/container/helperimage"
	dns_test "gitlab.com/gitlab-org/gitlab-runner/helpers/dns/test"
	"gitlab.com/gitlab-org/gitlab-runner/helpers/featureflags"
	service_helpers "gitlab.com/gitlab-org/gitlab-runner/helpers/service"
	"gitlab.com/gitlab-org/gitlab-runner/session/proxy"
	"gitlab.com/gitlab-org/gitlab-runner/shells"
)

type featureFlagTest func(t *testing.T, flagName string, flagValue bool)

func mustCreateResourceList(t *testing.T, cpu, memory, ephemeralStorage string) api.ResourceList {
	resources, err := createResourceList(cpu, memory, ephemeralStorage)
	require.NoError(t, err)

	return resources
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
				JobResponse: common.JobResponse{
					Variables: []common.JobVariable{
						{Key: "DOCKER_SOCKET", Value: "/var/run/docker.sock"},
						{Key: "PATH_TWO", Value: "/path/two"},
						{Key: "SUB_PATH", Value: "subpath"},
						{Key: "PATH_TO_WHATEVER", Value: "path/to/whatever"},
						{Key: "KEYWORD_CSI", Value: "csi"},
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
			e := &executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: executorOptions,
					Build:           tt.Build,
					Config:          tt.RunnerConfig,
				},
			}

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
	//nolint:lll
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
								{Name: "emptyDir", MountPath: "/path/to/empty/dir", Medium: "Memory"},
								{
									Name:      "emptyDir-subpath",
									MountPath: "/subpath",
									Medium:    "Memory",
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
					{Name: "emptyDir", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{Medium: "Memory"}}},
					{Name: "emptyDir-subpath", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{Medium: "Memory"}}},
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
			e := &executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: executorOptions,
					Build:           tt.Build,
					Config:          tt.RunnerConfig,
				},
			}

			buildtest.SetBuildFeatureFlag(e.Build, featureFlagName, featureFlagValue)
			assert.Equal(t, tt.Expected(featureFlagValue), e.getVolumes())
		})
	}
}

func testSetupBuildPodServiceCreationErrorFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	version, _ := testVersionAndCodec()
	helperImageInfo, err := helperimage.Get(common.REVISION, helperimage.Config{
		OSType:       helperimage.OSTypeLinux,
		Architecture: "amd64",
	})
	require.NoError(t, err)

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
		resp.Header.Add("Content-Type", "application/json")

		return resp, nil
	}

	mockFc := &mockFeatureChecker{}
	mockFc.On("IsHostAliasSupported").Return(true, nil)
	mockPullManager := &pull.MockManager{}
	defer mockPullManager.AssertExpectations(t)
	ex := executor{
		kubeClient: testKubernetesClient(version, fake.CreateHTTPClient(fakeRoundTripper)),
		options: &kubernetesOptions{
			Image: common.Image{
				Name:  "test-image",
				Ports: []common.Port{{Number: 80}},
			},
			Services: common.Services{
				{
					Name:  "test-service",
					Alias: "custom_name",
					Ports: []common.Port{
						{
							Number:   81,
							Name:     "custom_port_name",
							Protocol: "http",
						},
					},
				},
			},
		},
		AbstractExecutor: executors.AbstractExecutor{
			Config:     runnerConfig,
			BuildShell: &common.ShellConfiguration{},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					Variables: []common.JobVariable{},
				},
				Runner: &runnerConfig,
			},
			ProxyPool: proxy.NewPool(),
		},
		helperImageInfo: helperImageInfo,
		featureChecker:  mockFc,
		pullManager:     mockPullManager,
	}
	buildtest.SetBuildFeatureFlag(ex.Build, featureFlagName, featureFlagValue)

	mockPullManager.On("GetPullPolicyFor", ex.options.Services[0].Name).
		Return(api.PullAlways, nil).
		Once()
	mockPullManager.On("GetPullPolicyFor", ex.options.Image.Name).
		Return(api.PullAlways, nil).
		Once()
	mockPullManager.On("GetPullPolicyFor", runnerConfig.RunnerSettings.Kubernetes.HelperImage).
		Return(api.PullAlways, nil).
		Once()

	err = ex.prepareOverwrites(make(common.JobVariables, 0))
	assert.NoError(t, err)

	err = ex.setupBuildPod(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error creating the proxy service")
}

func testSetupBuildPodFailureGetPullPolicyFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	for _, failOnImage := range []string{
		"test-service",
		"test-helper",
		"test-build",
	} {
		t.Run(failOnImage, func(t *testing.T) {
			runnerConfig := common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "test-helper",
					},
				},
			}

			mockFc := &mockFeatureChecker{}
			defer mockFc.AssertExpectations(t)
			mockFc.On("IsHostAliasSupported").Return(true, nil).Maybe()

			mockPullManager := &pull.MockManager{}
			defer mockPullManager.AssertExpectations(t)

			e := executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-build",
					},
					Services: common.Services{
						{
							Name: "test-service",
						},
					},
				},
				AbstractExecutor: executors.AbstractExecutor{
					Config:     runnerConfig,
					BuildShell: &common.ShellConfiguration{},
					Build: &common.Build{
						JobResponse: common.JobResponse{},
						Runner:      &runnerConfig,
					},
				},
				featureChecker: mockFc,
				pullManager:    mockPullManager,
			}
			buildtest.SetBuildFeatureFlag(e.Build, featureFlagName, featureFlagValue)

			mockPullManager.On("GetPullPolicyFor", failOnImage).
				Return(api.PullAlways, assert.AnError).
				Once()

			mockPullManager.On("GetPullPolicyFor", mock.Anything).
				Return(api.PullAlways, nil).
				Maybe()

			err := e.prepareOverwrites(make(common.JobVariables, 0))
			assert.NoError(t, err)

			err = e.setupBuildPod(nil)
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
			e := &executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: executorOptions,
					Build: &common.Build{
						JobResponse: common.JobResponse{
							RunnerInfo: common.RunnerInfo{
								Timeout: tc.timeoutSeconds,
							},
						},
						Runner: &common.RunnerConfig{},
					},
					Config: common.RunnerConfig{
						RunnerSettings: common.RunnerSettings{
							Kubernetes: &common.KubernetesConfig{},
						},
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
	configMapsEndpointURI :=
		"/api/" + version + "/namespaces/" + objectMeta.Namespace + "/configmaps/" + objectMeta.Name

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
				switch p, m := req.URL.Path, req.Method; {
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
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
			Name: "Pod cleanup specifies GracePeriodSeconds with TerminationGracePeriodSeconds set",
			Config: &common.KubernetesConfig{
				TerminationGracePeriodSeconds: common.Int64Ptr(15),
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

					assert.EqualValues(t, common.Int64Ptr(15), opts.GracePeriodSeconds)
					return fakeKubeDeleteResponse(http.StatusOK), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
			Pod: &api.Pod{ObjectMeta: objectMeta},
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
			ex := executor{
				kubeClient: testKubernetesClient(
					version,
					fake.CreateHTTPClient(func(req *http.Request) (*http.Response, error) {
						return test.ClientFunc(t, req)
					}),
				),
				pod:         test.Pod,
				credentials: test.Credentials,
				services:    test.Services,
			}
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
			ex.AbstractExecutor.Trace = buildTrace
			ex.AbstractExecutor.BuildLogger = common.NewBuildLogger(buildTrace, logrus.WithFields(logrus.Fields{}))

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

func TestPrepare(t *testing.T) {
	helperImageTag := "latest"
	// common.REVISION is overridden at build time.
	if common.REVISION != "HEAD" {
		helperImageTag = common.REVISION
	}

	defaultOverwrites := &overwrites{
		namespace:       "default",
		serviceLimits:   api.ResourceList{},
		buildLimits:     api.ResourceList{},
		helperLimits:    api.ResourceList{},
		serviceRequests: api.ResourceList{},
		buildRequests:   api.ResourceList{},
		helperRequests:  api.ResourceList{},
	}

	defaultHelperImage := helperimage.Info{
		Architecture:            "x86_64",
		OSType:                  helperimage.OSTypeLinux,
		Name:                    helperimage.GitLabRegistryName,
		Tag:                     fmt.Sprintf("x86_64-%s", helperImageTag),
		IsSupportingLocalImport: true,
		Cmd:                     []string{"gitlab-runner-build"},
	}
	os := helperimage.OSTypeLinux
	if runtime.GOOS == helperimage.OSTypeWindows {
		os = helperimage.OSTypeWindows
	}
	pwshHelperImage, err := helperimage.Get(helperImageTag, helperimage.Config{
		Architecture: "x86_64",
		OSType:       os,
		Shell:        shells.SNPwsh,
	})
	require.NoError(t, err)

	tests := []struct {
		Name  string
		Error string

		GlobalConfig *common.Config
		RunnerConfig *common.RunnerConfig
		Build        *common.Build

		Expected                *executor
		ExpectedPullPolicy      api.PullPolicy
		ExpectedSharedBuildsDir bool
	}{
		{
			Name:         "all with limits",
			GlobalConfig: &common.Config{},
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
						Privileged:                   func(b bool) *bool { return &b }(true),
						PullPolicy:                   common.StringOrArray{common.PullPolicyIfNotPresent},
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
					Image: common.Image{
						Name: "test-image",
					},
					Variables: []common.JobVariable{
						{Key: "privileged", Value: "true"},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: &overwrites{
					namespace:       "default",
					buildLimits:     mustCreateResourceList(t, "1.5", "4Gi", "6Gi"),
					serviceLimits:   mustCreateResourceList(t, "100m", "200Mi", "1Gi"),
					helperLimits:    mustCreateResourceList(t, "50m", "100Mi", "200Mi"),
					buildRequests:   api.ResourceList{},
					serviceRequests: api.ResourceList{},
					helperRequests:  api.ResourceList{},
				},
				helperImageInfo: defaultHelperImage,
			},
			ExpectedPullPolicy: api.PullIfNotPresent,
		},
		{
			Name:         "all with limits and requests",
			GlobalConfig: &common.Config{},
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
						ServiceCPURequest:              "99m",
						ServiceMemoryRequest:           "5Mi",
						ServiceEphemeralStorageRequest: "200Mi",
						CPURequest:                     "1",
						MemoryRequest:                  "1.5Gi",
						EphemeralStorageRequest:        "1.3Gi",
						HelperCPURequest:               "0.5m",
						HelperMemoryRequest:            "42Mi",
						HelperEphemeralStorageRequest:  "99Mi",
						Privileged:                     func(b bool) *bool { return &b }(false),
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
					Image: common.Image{
						Name: "test-image",
					},
					Variables: []common.JobVariable{
						{Key: ServiceAccountOverwriteVariableName, Value: "not-default"},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
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
				},
				helperImageInfo: defaultHelperImage,
			},
		},
		{
			Name:         "unmatched service account",
			Error:        "couldn't prepare overwrites: provided value \"not-default\" does not match \"allowed-.*\"",
			GlobalConfig: &common.Config{},
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
						ServiceCPURequest:              "99m",
						ServiceMemoryRequest:           "5Mi",
						ServiceEphemeralStorageRequest: "50Mi",
						CPURequest:                     "1",
						MemoryRequest:                  "1.5Gi",
						EphemeralStorageRequest:        "40Mi",
						HelperCPURequest:               "0.5m",
						HelperMemoryRequest:            "42Mi",
						HelperEphemeralStorageRequest:  "52Mi",
						Privileged:                     func(b bool) *bool { return &b }(false),
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
					Image: common.Image{
						Name: "test-image",
					},
					Variables: []common.JobVariable{
						{Key: ServiceAccountOverwriteVariableName, Value: "not-default"},
					},
				},
				Runner: &common.RunnerConfig{},
			},
		},
		{
			Name:         "regexp match on service account and namespace",
			GlobalConfig: &common.Config{},
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
						ServiceCPURequest:              "99m",
						ServiceMemoryRequest:           "5Mi",
						ServiceEphemeralStorageRequest: "15Mi",
						CPURequest:                     "1",
						MemoryRequest:                  "1.5Gi",
						EphemeralStorageRequest:        "1.7Gi",
						HelperCPURequest:               "0.5m",
						HelperMemoryRequest:            "42Mi",
						HelperEphemeralStorageRequest:  "32Mi",
						Privileged:                     func(b bool) *bool { return &b }(false),
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
					Image: common.Image{
						Name: "test-image",
					},
					Variables: []common.JobVariable{
						{Key: NamespaceOverwriteVariableName, Value: "new-namespace-name"},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
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
				},
				helperImageInfo: defaultHelperImage,
			},
		},
		{
			Name:         "regexp match on namespace",
			GlobalConfig: &common.Config{},
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
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
					Image: common.Image{
						Name: "test-image",
					},
					Variables: []common.JobVariable{
						{Key: NamespaceOverwriteVariableName, Value: "namespace-$CI_CONCURRENT_ID"},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: &overwrites{
					namespace:       "namespace-0",
					serviceLimits:   api.ResourceList{},
					buildLimits:     api.ResourceList{},
					helperLimits:    api.ResourceList{},
					serviceRequests: api.ResourceList{},
					buildRequests:   api.ResourceList{},
					helperRequests:  api.ResourceList{},
				},
				helperImageInfo: defaultHelperImage,
			},
		},
		{
			Name:         "minimal configuration",
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image: "test-image",
						Host:  "test-server",
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
		},
		{
			Name:         "minimal configuration with pwsh shell",
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Shell: shells.SNPwsh,
					Kubernetes: &common.KubernetesConfig{
						Image: "test-image",
						Host:  "test-server",
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         pwshHelperImage,
			},
		},
		{
			Name:         "image and one service",
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host: "test-server",
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
					Image: common.Image{
						Name:       "test-image",
						Entrypoint: []string{"/init", "run"},
					},
					Services: common.Services{
						{
							Name:       "test-service",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
						},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name:       "test-image",
						Entrypoint: []string{"/init", "run"},
					},
					Services: common.Services{
						{
							Name:       "test-service",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
						},
					},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
		},
		{
			Name:         "merge services",
			GlobalConfig: &common.Config{},
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
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
					Image: common.Image{
						Name:       "test-image",
						Entrypoint: []string{"/init", "run"},
					},
					Services: common.Services{
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
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name:       "test-image",
						Entrypoint: []string{"/init", "run"},
					},
					Services: common.Services{
						{
							Name:  "test-service-k8s",
							Alias: "alias",
						},
						{
							Name: "test-service-k8s2",
						},
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
						{
							Name:       "test-service",
							Alias:      "test-alias",
							Entrypoint: []string{"/init", "run"},
							Command:    []string{"application", "--debug"},
						},
					},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
		},
		{
			Name:         "Default helper image",
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host: "test-server",
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					Image: common.Image{
						Name: "test-image",
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				helperImageInfo:         defaultHelperImage,
				configurationOverwrites: defaultOverwrites,
			},
		},
		{
			Name:         "helper image with ubuntu flavour default registry",
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host:              "test-server",
						HelperImageFlavor: "ubuntu",
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					Image: common.Image{
						Name: "test-image",
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo: helperimage.Info{
					OSType:                  os,
					Architecture:            "x86_64",
					Name:                    helperimage.GitLabRegistryName,
					Tag:                     fmt.Sprintf("ubuntu-x86_64-%s", helperImageTag),
					IsSupportingLocalImport: true,
					Cmd:                     []string{"gitlab-runner-build"},
				},
			},
		},
		{
			Name:         "helper image from node selector (linux, arm)",
			GlobalConfig: &common.Config{},
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
				JobResponse: common.JobResponse{
					Image: common.Image{
						Name: "test-image",
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
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
				},
				helperImageInfo: helperimage.Info{
					OSType:                  "linux",
					Architecture:            "arm64",
					Name:                    helperimage.GitLabRegistryName,
					Tag:                     fmt.Sprintf("arm64-%s", helperImageTag),
					IsSupportingLocalImport: true,
					Cmd:                     []string{"gitlab-runner-build"},
				},
			},
		},
		{
			Name:         "helper image from node selector (windows, amd64)",
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host: "test-server",
						NodeSelector: map[string]string{
							api.LabelArchStable:           "amd64",
							api.LabelOSStable:             "windows",
							nodeSelectorWindowsBuildLabel: "10.0.19041",
						},
					},
					Shell: "pwsh",
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					Image: common.Image{
						Name: "test-image",
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: &overwrites{
					namespace: "default",
					nodeSelector: map[string]string{
						api.LabelArchStable:           "amd64",
						api.LabelOSStable:             "windows",
						nodeSelectorWindowsBuildLabel: "10.0.19041",
					},
					serviceLimits:   api.ResourceList{},
					buildLimits:     api.ResourceList{},
					helperLimits:    api.ResourceList{},
					serviceRequests: api.ResourceList{},
					buildRequests:   api.ResourceList{},
					helperRequests:  api.ResourceList{},
				},
				helperImageInfo: helperimage.Info{
					OSType:                  "windows",
					Architecture:            "x86_64",
					Name:                    helperimage.GitLabRegistryName,
					Tag:                     fmt.Sprintf("x86_64-%s-servercore2004", helperImageTag),
					IsSupportingLocalImport: false,
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
						//nolint:lll
						"JABPAHUAdABwAHUAdABFAG4AYwBvAGQAaQBuAGcAIAA9ACAAWwBjAG8AbgBzAG8AbABlAF0AOgA6AEkAbgBwAHUAdABFAG4AYwBvAGQAaQBuAGcAIAA9ACAAWwBjAG8AbgBzAG8AbABlAF0AOgA6AE8AdQB0AHAAdQB0AEUAbgBjAG8AZABpAG4AZwAgAD0AIABOAGUAdwAtAE8AYgBqAGUAYwB0ACAAUwB5AHMAdABlAG0ALgBUAGUAeAB0AC4AVQBUAEYAOABFAG4AYwBvAGQAaQBuAGcADQAKAHAAdwBzAGgAIAAtAEMAbwBtAG0AYQBuAGQAIAAtAA0ACgA=",
					},
				},
			},
		},
		{
			Name:         "helper image from node selector (unknown)",
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host: "test-server",
						NodeSelector: map[string]string{
							api.LabelArchStable: "riscv64",
							api.LabelOSStable:   "freebsd",
						},
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					Image: common.Image{
						Name: "test-image",
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         helperimage.Info{},
			},
			Error: `prepare helper image: unsupported OSType "freebsd"`,
		},
		{
			Name:         "builds dir default",
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Image: "test-image",
						Host:  "test-server",
					},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ExpectedSharedBuildsDir: false,
		},
		{
			Name:         "builds dir user specified empty_dir",
			GlobalConfig: &common.Config{},
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
								},
							},
						},
					},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ExpectedSharedBuildsDir: false,
		},
		{
			Name:         "builds dir user specified host_path",
			GlobalConfig: &common.Config{},
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
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ExpectedSharedBuildsDir: true,
		},
		{
			Name:         "builds dir user specified pvc",
			GlobalConfig: &common.Config{},
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
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ExpectedSharedBuildsDir: true,
		},
		{
			Name:         "runner pull policy is one of allowed pull policies",
			GlobalConfig: &common.Config{},
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
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ExpectedPullPolicy: api.PullNever,
		},
		{
			Name:         "runner pull policy is not one of allowed pull policies",
			GlobalConfig: &common.Config{},
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
				JobResponse: common.JobResponse{
					Image: common.Image{
						Name: "test-image",
					},
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			Error: "failed to pull image 'test-image': " +
				fmt.Sprintf(common.IncompatiblePullPolicy, "[IfNotPresent]", "Runner config", "[Always Never]"),
		},
		{
			Name:         "image pull policy is one of allowed pull policies",
			GlobalConfig: &common.Config{},
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
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
					Image: common.Image{
						PullPolicies: []common.DockerPullPolicy{common.PullPolicyNever},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ExpectedPullPolicy: api.PullNever,
		},
		{
			Name:         "image pull policy is not one of allowed pull policies",
			GlobalConfig: &common.Config{},
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
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
					Image: common.Image{
						Name:         "test-image",
						PullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			Error: "failed to pull image 'test-image': " +
				//nolint:lll
				fmt.Sprintf(common.IncompatiblePullPolicy, "[IfNotPresent]", "GitLab pipeline config", "[Always Never]"),
		},
		{
			Name:         "both runner and image pull policies are defined",
			GlobalConfig: &common.Config{},
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
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
					Image: common.Image{
						PullPolicies: []common.DockerPullPolicy{common.PullPolicyIfNotPresent},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			ExpectedPullPolicy: api.PullIfNotPresent,
		},
		{
			Name:         "one of allowed pull policies is invalid",
			GlobalConfig: &common.Config{},
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
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			Error: "allowed_pull_policies config: unsupported pull policy: \"invalid\"",
		},
		{
			Name:         "one of config pull policies is invalid",
			GlobalConfig: &common.Config{},
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
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			Error: "pull_policy config: unsupported pull policy: \"invalid\"",
		},
		{
			Name:         "one of image pull policies is invalid",
			GlobalConfig: &common.Config{},
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
				JobResponse: common.JobResponse{
					GitInfo: common.GitInfo{
						Sha: "1234567890",
					},
					Image: common.Image{
						Name:         "test-image",
						PullPolicies: []common.DockerPullPolicy{common.PullPolicyAlways, "invalid"},
					},
				},
				Runner: &common.RunnerConfig{},
			},
			Expected: &executor{
				options: &kubernetesOptions{
					Image: common.Image{
						Name: "test-image",
					},
				},
				configurationOverwrites: defaultOverwrites,
				helperImageInfo:         defaultHelperImage,
			},
			Error: "conversion to Kubernetes policy: unsupported pull policy: \"invalid\"",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			e := &executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: executorOptions,
				},
				options: &kubernetesOptions{},
			}

			// TODO: handle the context properly with https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27932
			prepareOptions := common.ExecutorPrepareOptions{
				Config:  test.RunnerConfig,
				Build:   test.Build,
				Context: context.TODO(),
			}
			prepareOptions.Build.Runner.Executor = common.ExecutorKubernetes

			err := e.Prepare(prepareOptions)
			if err != nil {
				assert.False(t, test.Build.IsSharedEnv())
			}
			if test.Error != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), test.Error)
				return
			}

			// Set this to nil so we aren't testing the functionality of the
			// base AbstractExecutor's Prepare method
			e.AbstractExecutor = executors.AbstractExecutor{}

			pullPolicy, err := e.pullManager.GetPullPolicyFor(prepareOptions.Build.Image.Name)
			assert.NoError(t, err)
			assert.Equal(t, test.ExpectedPullPolicy, pullPolicy)

			sharedBuildsDir := e.isSharedBuildsDirRequired()
			assert.Equal(t, test.ExpectedSharedBuildsDir, sharedBuildsDir)

			e.kubeClient = nil
			e.kubeConfig = nil
			e.featureChecker = nil
			e.pullManager = nil
			e.requireDefaultBuildsDirVolume = nil
			e.requireSharedBuildsDir = nil

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
			e := &executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: executorOptions,
				},
			}

			e.setupDefaultExecutorOptions(os)
			tc(t, e)
		})
	}
}

func TestSetupCredentials(t *testing.T) {
	version, _ := testVersionAndCodec()

	type testDef struct {
		RunnerCredentials *common.RunnerCredentials
		Credentials       []common.Credentials
		VerifyFn          func(*testing.T, testDef, *api.Secret)
	}
	tests := map[string]testDef{
		"no credentials": {
			// don't execute VerifyFn
			VerifyFn: nil,
		},
		"registry credentials": {
			Credentials: []common.Credentials{
				{
					Type:     "registry",
					URL:      "http://example.com",
					Username: "user",
					Password: "password",
				},
			},
			VerifyFn: func(t *testing.T, test testDef, secret *api.Secret) {
				assert.Equal(t, api.SecretTypeDockercfg, secret.Type)
				assert.NotEmpty(t, secret.Data[api.DockerConfigKey])
			},
		},
		"other credentials": {
			Credentials: []common.Credentials{
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
			Credentials: []common.Credentials{
				{
					Type:     "registry",
					URL:      "http://example.com",
					Username: "user",
					Password: "password",
				},
			},
			VerifyFn: func(t *testing.T, test testDef, secret *api.Secret) {
				dns_test.AssertRFC1123Compatibility(t, secret.GetGenerateName())
			},
		},
	}

	executed := false
	fakeClientRoundTripper := func(test testDef) func(req *http.Request) (*http.Response, error) {
		return func(req *http.Request) (resp *http.Response, err error) {
			podBytes, err := io.ReadAll(req.Body)
			executed = true

			if err != nil {
				t.Errorf("failed to read request body: %s", err.Error())
				return
			}

			p := new(api.Secret)

			err = json.Unmarshal(podBytes, p)

			if err != nil {
				t.Errorf("error decoding pod: %s", err.Error())
				return
			}

			if test.VerifyFn != nil {
				test.VerifyFn(t, test, p)
			}

			resp = &http.Response{StatusCode: http.StatusOK, Body: FakeReadCloser{
				Reader: bytes.NewBuffer(podBytes),
			}}
			resp.Header = make(http.Header)
			resp.Header.Add("Content-Type", "application/json")

			return
		}
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			ex := executor{
				kubeClient: testKubernetesClient(version, fake.CreateHTTPClient(fakeClientRoundTripper(test))),
				options:    &kubernetesOptions{},
				AbstractExecutor: executors.AbstractExecutor{
					Config: common.RunnerConfig{
						RunnerSettings: common.RunnerSettings{
							Kubernetes: &common.KubernetesConfig{
								Namespace: "default",
							},
						},
					},
					BuildShell: &common.ShellConfiguration{},
					Build: &common.Build{
						JobResponse: common.JobResponse{
							Variables:   []common.JobVariable{},
							Credentials: test.Credentials,
						},
						Runner: &common.RunnerConfig{},
					},
				},
			}

			if test.RunnerCredentials != nil {
				ex.Build.Runner = &common.RunnerConfig{
					RunnerCredentials: *test.RunnerCredentials,
				}
			}

			executed = false

			err := ex.prepareOverwrites(make(common.JobVariables, 0))
			assert.NoError(t, err)

			err = ex.setupCredentials()
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
						Header:     map[string][]string{"Content-Type": {"application/json"}},
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
					Header:     map[string][]string{"Content-Type": {"application/json"}},
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
			ex := executor{
				kubeClient: testKubernetesClient(version, fake.CreateHTTPClient(tc.clientFunc)),
				options:    &kubernetesOptions{},
				AbstractExecutor: executors.AbstractExecutor{
					Config: common.RunnerConfig{
						RunnerSettings: common.RunnerSettings{
							Kubernetes: &common.KubernetesConfig{
								Namespace: namespace,
							},
						},
					},
				},
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			defer cancel()

			err := ex.prepareOverwrites(make(common.JobVariables, 0))
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
						Header:     map[string][]string{"Content-Type": {"application/json"}},
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
					Header:     map[string][]string{"Content-Type": {"application/json"}},
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
			ex := executor{
				kubeClient: testKubernetesClient(version, fake.CreateHTTPClient(tc.clientFunc)),
				options:    &kubernetesOptions{},
				AbstractExecutor: executors.AbstractExecutor{
					Config: common.RunnerConfig{
						RunnerSettings: common.RunnerSettings{
							Kubernetes: &common.KubernetesConfig{
								Namespace: DefaultResourceIdentifier,
							},
						},
					},
				},
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			defer cancel()

			err := ex.prepareOverwrites(make(common.JobVariables, 0))
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
						Header:     map[string][]string{"Content-Type": {"application/json"}},
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
						Header:     map[string][]string{"Content-Type": {"application/json"}},
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
						Header:     map[string][]string{"Content-Type": {"application/json"}},
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
						Header:     map[string][]string{"Content-Type": {"application/json"}},
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
						Header:     map[string][]string{"Content-Type": {"application/json"}},
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
						Header:     map[string][]string{"Content-Type": {"application/json"}},
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
			ex := executor{
				kubeClient: testKubernetesClient(version, fake.CreateHTTPClient(tc.clientFunc)),
				options:    &kubernetesOptions{},
				AbstractExecutor: executors.AbstractExecutor{
					Config: common.RunnerConfig{
						RunnerSettings: common.RunnerSettings{
							Kubernetes: &common.KubernetesConfig{
								Namespace:                            DefaultResourceIdentifier,
								ServiceAccount:                       tc.serviceAccount,
								ImagePullSecrets:                     tc.imagePullSecrets,
								ResourceAvailabilityCheckMaxAttempts: 3,
							},
						},
					},
				},
			}

			var err error

			ctx, cancel := context.WithTimeout(context.Background(), tc.ctxTimeout)
			defer cancel()

			err = ex.prepareOverwrites(make(common.JobVariables, 0))
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
	Variables                []common.JobVariable
	Credentials              []common.Credentials
	Options                  *kubernetesOptions
	InitContainers           []api.Container
	SetHTTPPutResponse       func() (*http.Response, error)
	PrepareFn                func(*testing.T, setupBuildPodTestDef, *executor)
	VerifyFn                 func(*testing.T, setupBuildPodTestDef, *api.Pod)
	VerifyExecutorFn         func(*testing.T, setupBuildPodTestDef, *executor)
	VerifySetupBuildPodErrFn func(*testing.T, error)
}

type setupBuildPodFakeRoundTripper struct {
	t        *testing.T
	test     setupBuildPodTestDef
	executed bool
}

func (rt *setupBuildPodFakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
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
	resp.Header.Add("Content-Type", "application/json")

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
		resp.Header.Add("Content-Type", "application/json")
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
	resp.Header.Add("Content-Type", "application/json")
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
	resp.Header.Add("Content-Type", "application/json")
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
				assert.Equal(t, map[string]string{
					"test":    "label",
					"another": "label",
					"var":     "sometestvar",
					"pod":     pod.GenerateName,
				}, pod.ObjectMeta.Labels)
			},
			Variables: []common.JobVariable{
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
				assert.Equal(t, map[string]string{
					"test":     "label",
					"another":  "newlabel",
					"var":      "sometestvar",
					"another2": "sometestvar",
					"pod":      pod.GenerateName,
				}, pod.ObjectMeta.Labels)
			},
			Variables: []common.JobVariable{
				{Key: "test", Value: "sometestvar"},
				{Key: "KUBERNETES_POD_LABELS_1", Value: "another=newlabel"},
				{Key: "KUBERNETES_POD_LABELS_2", Value: "another2=$test"},
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
					"project.runner.gitlab.com/id":     "0",
				}, pod.ObjectMeta.Annotations)
			},
			Variables: []common.JobVariable{
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
						assert.Equal(t, "custom/helper-image:"+common.REVISION, c.Image)
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
				}
				assert.ElementsMatch(t, expectedTolerations, pod.Spec.Tolerations)
			},
		},
		"supports extended docker configuration for image and services": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: common.Image{
					Name:       "test-image",
					Entrypoint: []string{"/init", "run"},
				},
				Services: common.Services{
					{
						Name:       "test-service",
						Entrypoint: []string{"/init", "run"},
						Command:    []string{"application", "--debug"},
					},
					{
						Name:    "test-service-2",
						Command: []string{"application", "--debug"},
					},
					{
						Name:    "test-service-3",
						Command: []string{"application", "--debug"},
						Variables: []common.JobVariable{
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
			Variables: []common.JobVariable{
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
				Image: common.Image{
					Name: "test-image",
					Ports: []common.Port{
						{
							Number: 80,
						},
					},
				},
				Services: common.Services{
					{
						Name: "test-service",
						Ports: []common.Port{
							{
								Number: 82,
							},
							{
								Number: 84,
							},
						},
					},
					{
						Name: "test-service2",
						Ports: []common.Port{
							{
								Number: 85,
							},
						},
					},
					{
						Name: "test-service3",
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				ownerReferences := e.buildPodReferences()
				expectedServices := []api.Service{
					{
						ObjectMeta: metav1.ObjectMeta{
							GenerateName:    "build",
							Namespace:       "default",
							OwnerReferences: ownerReferences,
						},
						Spec: api.ServiceSpec{
							Ports: []api.ServicePort{
								{
									Port:       80,
									TargetPort: intstr.FromInt(80),
									Name:       "build-80",
								},
							},
							Selector: map[string]string{"pod": e.pod.GenerateName},
							Type:     api.ServiceTypeClusterIP,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							GenerateName:    "proxy-svc-0",
							Namespace:       "default",
							OwnerReferences: ownerReferences,
						},
						Spec: api.ServiceSpec{
							Ports: []api.ServicePort{
								{
									Port:       82,
									TargetPort: intstr.FromInt(82),
									Name:       "proxy-svc-0-82",
								},
								{
									Port:       84,
									TargetPort: intstr.FromInt(84),
									Name:       "proxy-svc-0-84",
								},
							},
							Selector: map[string]string{"pod": e.pod.GenerateName},
							Type:     api.ServiceTypeClusterIP,
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							GenerateName:    "proxy-svc-1",
							Namespace:       "default",
							OwnerReferences: ownerReferences,
						},
						Spec: api.ServiceSpec{
							Ports: []api.ServicePort{
								{
									Port:       85,
									TargetPort: intstr.FromInt(85),
									Name:       "proxy-svc-1-85",
								},
							},
							Selector: map[string]string{"pod": e.pod.GenerateName},
							Type:     api.ServiceTypeClusterIP,
						},
					},
				}

				assert.ElementsMatch(t, expectedServices, e.services)
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
				Image: common.Image{
					Name: "test-image",
					Ports: []common.Port{
						{
							Number: 80,
						},
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				assert.Equal(t, "build", e.services[0].GenerateName)
			},
		},
		"the services have a selector pointing to the 'pod' label in the pod": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: common.Image{
					Name: "test-image",
					Ports: []common.Port{
						{
							Number: 80,
						},
					},
				},
				Services: common.Services{
					{
						Name: "test-service",
						Ports: []common.Port{
							{
								Number: 82,
							},
						},
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				for _, service := range e.services {
					assert.Equal(t, map[string]string{"pod": e.pod.GenerateName}, service.Spec.Selector)
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
				Image: common.Image{
					Name: "test-image",
				},
				Services: common.Services{
					{
						Name:  "test-service",
						Alias: "custom-name",
						Ports: []common.Port{
							{
								Number: 82,
							},
						},
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				assert.Equal(t, "custom-name", e.services[0].GenerateName)
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
				Image: common.Image{
					Name: "test-image",
					Ports: []common.Port{
						{
							Number: 80,
						},
					},
				},
				Services: common.Services{
					{
						Name:  "test-service",
						Alias: "custom_name",
						Ports: []common.Port{
							{
								Number:   81,
								Name:     "custom_port_name",
								Protocol: "http",
							},
						},
					},
					{
						Name: "test-service2",
						Ports: []common.Port{
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
				Image: common.Image{
					Name: "test-image",
				},
				Services: common.Services{
					{
						Name:  "test-service",
						Alias: "service,name-.non-compat!ble",
						Ports: []common.Port{
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
					return e.services[i].GenerateName > e.services[j].GenerateName
				})
				assert.Equal(t, "service", e.services[0].GenerateName)
				assert.Equal(t, "name-non-compatble", e.services[1].GenerateName)

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
		"sets command (entrypoint) and args": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						HelperImage: "custom/helper-image",
					},
				},
			},
			Options: &kubernetesOptions{
				Image: common.Image{
					Name: "test-image",
				},
				Services: common.Services{
					{
						Name:    "test-service-0",
						Command: []string{"application", "--debug"},
					},
					{
						Name:       "test-service-1",
						Entrypoint: []string{"application", "--debug"},
					},
					{
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
				assert.Empty(t, pod.Spec.Containers[0].Command, "Build container command should be empty")
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
				dns_test.AssertRFC1123Compatibility(t, pod.GetGenerateName())
			},
		},
		"supports pod security context": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						PodSecurityContext: common.KubernetesPodSecurityContext{
							FSGroup:            func() *int64 { i := int64(200); return &i }(),
							RunAsGroup:         func() *int64 { i := int64(200); return &i }(),
							RunAsNonRoot:       func() *bool { i := bool(true); return &i }(),
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
			//nolint:lll
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
				Services: common.Services{
					{
						Name:  "test-service",
						Alias: "svc-alias",
					},
					{
						Name: "docker:dind",
					},
					{
						Name: "service-with-port:dind",
						Ports: []common.Port{{
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
						Hostnames: []string{"test-service", "svc-alias", "docker"},
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
				Services: common.Services{
					{
						Name:  "test-service",
						Alias: "INVALID_ALIAS",
					},
					{
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
				Services: common.Services{
					{
						Name:  "test-service",
						Alias: "alias",
					},
				},
			},
			PrepareFn: func(t *testing.T, def setupBuildPodTestDef, e *executor) {
				mockFc := &mockFeatureChecker{}
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
				Services: common.Services{
					{
						Name:  "test-service",
						Alias: "alias",
					},
				},
			},
			PrepareFn: func(t *testing.T, def setupBuildPodTestDef, e *executor) {
				mockFc := &mockFeatureChecker{}
				mockFc.On("IsHostAliasSupported").Return(false, testErr)
				e.featureChecker = mockFc
			},
			VerifySetupBuildPodErrFn: func(t *testing.T, err error) {
				assert.ErrorIs(t, err, testErr)
			},
		},
		"check host aliases with kubernetes version error": {
			Options: &kubernetesOptions{
				Services: common.Services{
					{
						Name:  "test-service",
						Alias: "alias",
					},
				},
			},
			PrepareFn: func(t *testing.T, def setupBuildPodTestDef, e *executor) {
				mockFc := &mockFeatureChecker{}
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
				Services: common.Services{
					{
						Name:    "test-service-0",
						Command: []string{"application", "--debug"},
					},
					{
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
				Image: common.Image{
					Name: "test-image",
					Ports: []common.Port{
						{
							Number: 80,
						},
					},
				},
				Services: common.Services{
					{
						Name: "test-service",
						Ports: []common.Port{
							{
								Number: 82,
							},
							{
								Number: 84,
							},
						},
					},
					{
						Name: "test-service2",
						Ports: []common.Port{
							{
								Number: 85,
							},
						},
					},
					{
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
			Credentials: []common.Credentials{
				{
					Type:     "registry",
					URL:      "http://example.com",
					Username: "user",
					Password: "password",
				},
			},
			Options: &kubernetesOptions{
				Image: common.Image{
					Name: "test-image",
					Ports: []common.Port{
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
			Credentials: []common.Credentials{
				{
					Type:     "registry",
					URL:      "http://example.com",
					Username: "user",
					Password: "password",
				},
			},
			Options: &kubernetesOptions{
				Image: common.Image{
					Name: "test-image",
					Ports: []common.Port{
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
		"supports TerminationGracePeriodSeconds": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						TerminationGracePeriodSeconds: common.Int64Ptr(10),
					},
				},
			},
			VerifyExecutorFn: func(t *testing.T, test setupBuildPodTestDef, e *executor) {
				assert.EqualValues(
					t,
					test.RunnerConfig.Kubernetes.TerminationGracePeriodSeconds,
					e.pod.Spec.TerminationGracePeriodSeconds,
				)
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
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			helperImageInfo, err := helperimage.Get(common.REVISION, helperimage.Config{
				OSType:       helperimage.OSTypeLinux,
				Architecture: "amd64",
			})
			require.NoError(t, err)

			vars := test.Variables
			if vars == nil {
				vars = []common.JobVariable{}
			}

			creds := test.Credentials
			if creds == nil {
				creds = []common.Credentials{}
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

			mockFc := &mockFeatureChecker{}
			mockFc.On("IsHostAliasSupported").Return(true, nil)

			mockPullManager := &pull.MockManager{}
			defer mockPullManager.AssertExpectations(t)

			ex := executor{
				kubeClient: testKubernetesClient(version, fake.CreateHTTPClient(rt.RoundTrip)),
				options:    options,
				AbstractExecutor: executors.AbstractExecutor{
					Config:     test.RunnerConfig,
					BuildShell: &common.ShellConfiguration{},
					Build: &common.Build{
						JobResponse: common.JobResponse{
							Variables:   vars,
							Credentials: creds,
						},
						Runner: &test.RunnerConfig,
					},
					ProxyPool: proxy.NewPool(),
				},
				helperImageInfo: helperImageInfo,
				featureChecker:  mockFc,
				pullManager:     mockPullManager,
			}

			if ex.options.Image.Name == "" {
				// Ensure we have a valid Docker image name in the configuration,
				// if nothing is specified in the test case
				ex.options.Image.Name = "build-image"
			}

			if test.PrepareFn != nil {
				test.PrepareFn(t, test, &ex)
			}

			if test.Options != nil && test.Options.Services != nil {
				for _, service := range test.Options.Services {
					mockPullManager.On("GetPullPolicyFor", service.Name).
						Return(api.PullAlways, nil).
						Once()
				}
			}

			mockPullManager.On("GetPullPolicyFor", ex.getHelperImage()).
				Return(api.PullAlways, nil).
				Maybe()
			mockPullManager.On("GetPullPolicyFor", ex.options.Image.Name).
				Return(api.PullAlways, nil).
				Maybe()

			err = ex.prepareOverwrites(test.Variables)
			assert.NoError(t, err, "error preparing overwrites")

			if test.Credentials != nil {
				err = ex.setupCredentials()
				assert.NoError(t, err, "error setting up credentials")
			}

			err = ex.setupBuildPod(test.InitContainers)
			if test.VerifySetupBuildPodErrFn == nil {
				assert.NoError(t, err, "error setting up build pod")
				assert.True(t, rt.executed, "RoundTrip for kubernetes client should be executed")
			} else {
				test.VerifySetupBuildPodErrFn(t, err)
			}

			if test.VerifyExecutorFn != nil {
				test.VerifyExecutorFn(t, test, &ex)
			}
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
				errCh <- fmt.Errorf("Custom error for test with EOF %s", io.EOF)
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
			mockTrace := &common.MockJobTrace{}
			defer mockTrace.AssertExpectations(t)
			mockTrace.On("Write", []byte("line\n")).Return(0, nil).Once()

			mockLogProcessor := new(mockLogProcessor)
			defer mockLogProcessor.AssertExpectations(t)

			tc.lineCh <- "line\n"
			mockLogProcessor.On("Process", mock.Anything).
				Return((<-chan string)(tc.lineCh), (<-chan error)(tc.errCh)).
				Once()

			tc.run(tc.lineCh, tc.errCh)

			e := newExecutor()
			e.Trace = mockTrace
			e.pod = &api.Pod{}
			e.pod.Name = "pod_name"
			e.pod.Namespace = "namespace"
			e.newLogProcessor = func() logProcessor {
				return mockLogProcessor
			}

			go e.processLogs(context.Background())

			exitStatus := <-e.remoteProcessTerminated
			assert.Equal(t, tc.expectedExitCode, exitStatus.CommandExitCode)
			if tc.expectedScript != "" {
				assert.Equal(t, tc.expectedScript, *exitStatus.Script)
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
			ctx, cancel := context.WithCancel(context.Background())
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
						"Content-Type": {"application/json"},
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

			e := executor{}
			e.Config = common.RunnerConfig{}
			e.Config.Kubernetes = &common.KubernetesConfig{
				PollInterval: 1,
				PollTimeout:  2,
			}
			e.kubeClient = client
			e.remoteProcessTerminated = make(chan shells.TrapCommandExitStatus)
			e.Trace = &common.Trace{Writer: os.Stdout}
			e.pod = &api.Pod{}
			e.pod.Name = "pod"
			e.pod.Namespace = "namespace"

			tt.verifyErr(t, e.watchPodStatus(ctx))
		})
	}
}

func fakeKubeDeleteResponse(status int) *http.Response {
	_, codec := testVersionAndCodec()

	body := objBody(codec, &metav1.Status{Code: int32(status)})
	return &http.Response{StatusCode: status, Body: body, Header: map[string][]string{
		"Content-Type": {"application/json"},
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

	remoteExecutor := new(MockRemoteExecutor)
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
		On("Execute", http.MethodPost, urlMatcher, mock.Anything, nil, output, output, false).
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

	err := s.Stream(int64(offset), output)
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

func (f FakeBuildTrace) Success()                                          {}
func (f FakeBuildTrace) Fail(err error, failureData common.JobFailureData) {}
func (f FakeBuildTrace) Notify(func())                                     {}
func (f FakeBuildTrace) SetCancelFunc(cancelFunc context.CancelFunc)       {}
func (f FakeBuildTrace) Cancel() bool                                      { return false }
func (f FakeBuildTrace) SetAbortFunc(cancelFunc context.CancelFunc)        {}
func (f FakeBuildTrace) Abort() bool                                       { return false }
func (f FakeBuildTrace) SetFailuresCollector(fc common.FailuresCollector)  {}
func (f FakeBuildTrace) SetMasked(opts common.MaskOptions)                 {}
func (f FakeBuildTrace) SetDebugModeEnabled(isEnabled bool)                {}
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
	gitlabRegistry, err := helperimage.Get(common.REVISION, helperimage.Config{
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
			e := &executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: executorOptions,
					Build: &common.Build{
						Runner: &tt.config,
					},
					Config: tt.config,
				},
			}

			prepareOptions := common.ExecutorPrepareOptions{
				Config:  &tt.config,
				Build:   e.Build,
				Context: context.Background(),
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
	mockPullManager := &pull.MockManager{}
	defer mockPullManager.AssertExpectations(t)

	e := &executor{
		AbstractExecutor: executors.AbstractExecutor{
			ExecutorOptions: executorOptions,
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Config: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{},
				},
			},
		},
		pullManager: mockPullManager,
	}

	mockPullManager.On("GetPullPolicyFor", mock.Anything).
		Return(api.PullAlways, assert.AnError).
		Once()

	_, err := e.buildPermissionsInitContainer(helperimage.OSTypeLinux)
	assert.ErrorIs(t, err, assert.AnError)
}

func TestExecutor_buildPermissionsInitContainer_CheckResources(t *testing.T) {
	mockPullManager := &pull.MockManager{}
	cpu := resource.MustParse("1")
	memory := resource.MustParse("1Gi")
	e := &executor{
		AbstractExecutor: executors.AbstractExecutor{
			ExecutorOptions: executorOptions,
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Config: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{},
				},
			},
		},
		pullManager: mockPullManager,
		configurationOverwrites: &overwrites{
			helperLimits: api.ResourceList{
				"cpu":    cpu,
				"memory": memory,
			},
			helperRequests: api.ResourceList{
				"cpu":    cpu,
				"memory": memory,
			},
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
					e.scriptPath(detectShellScriptName),
					e.scriptPath(cmd.Stage),
					e.buildRedirectionCmd("bash"),
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

func setupExecutor(shell string, successfulResponse common.JobResponse) *executor {
	build := &common.Build{
		JobResponse: successfulResponse,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: common.ExecutorKubernetes,
				Shell:    shell,
			},
		},
	}

	return &executor{
		helperImageInfo: helperimage.Info{
			Cmd: []string{"custom", "command"},
		},
		AbstractExecutor: executors.AbstractExecutor{
			ExecutorOptions: executors.ExecutorOptions{
				DefaultBuildsDir: "/builds",
				DefaultCacheDir:  "/cache",
				Shell: common.ShellScriptInfo{
					Shell: shell,
					Build: build,
				},
			},
			Build: build,
		},
	}
}

func TestLifecyclePrepare(t *testing.T) {
	initExecutor := func(lifecycleCfg common.KubernetesContainerLifecyle) *executor {
		return &executor{
			AbstractExecutor: executors.AbstractExecutor{
				ExecutorOptions: executorOptions,
				Build: &common.Build{
					Runner: &common.RunnerConfig{},
				},
				Config: common.RunnerConfig{
					RunnerSettings: common.RunnerSettings{
						Kubernetes: &common.KubernetesConfig{
							ContainerLifecycle: lifecycleCfg,
						},
					},
				},
			},
		}
	}

	execHandler := &api.ExecAction{
		Command: []string{"ls", "-alF"},
	}

	httpGetHandler := &api.HTTPGetAction{
		Port:        intstr.FromInt(8080),
		Path:        "/test",
		Host:        "localhost",
		HTTPHeaders: []api.HTTPHeader{},
	}

	tcpSocketHander := &api.TCPSocketAction{
		Port: intstr.FromInt(8080),
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

	mockPullManager := &pull.MockManager{}
	mockPullManager.On("GetPullPolicyFor", mock.Anything).
		Return(api.PullAlways, nil).
		Times(2)

	defer mockPullManager.AssertExpectations(t)

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

	mockPullManager := &pull.MockManager{}
	mockPullManager.On("GetPullPolicyFor", mock.Anything).
		Return(api.PullAlways, nil).
		Times(len(tests))

	defer mockPullManager.AssertExpectations(t)

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

func Test_Executor_captureContainerLogs(t *testing.T) {
	cName := "some container"
	msg := []byte("pretend this is a log generated by a process in a container")
	ctx := context.Background()
	logs := bytes.Buffer{}

	fakeRoundTripper := func(body io.ReadCloser, err error) func(req *http.Request) (*http.Response, error) {
		return func(_ *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       body,
			}, err
		}
	}

	version, _ := testVersionAndCodec()
	e := &executor{
		pod: &api.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"}},
	}

	lentry := logrus.New()
	lentry.Out = &logs
	e.BuildLogger = common.NewBuildLogger(&common.Trace{Writer: &logs}, logrus.NewEntry(lentry))

	isw := service_helpers.NewInlineServiceLogWriter(cName, &logs)

	tests := map[string]struct {
		wantLog    string
		wantErr    error
		readCloser func(io.ReadCloser) io.ReadCloser
	}{
		"success": {
			wantLog:    string(msg),
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
			logs.Reset()
			// we'll write into pw, which will be copied to pr and simulate a process in
			// a container writing to stdout.
			pr, pw := io.Pipe()
			defer pw.Close() // ... for the failure case

			httpClient := fake.CreateHTTPClient(fakeRoundTripper(tt.readCloser(pr), tt.wantErr))
			e.kubeClient = testKubernetesClient(version, httpClient)

			err := e.captureContainerLogs(ctx, cName, isw)

			if tt.wantErr != nil {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantLog)
			} else {
				require.NoError(t, err)

				// this will be copied to pr...
				_, err := pw.Write(msg)
				require.NoError(t, err)
				pw.Close() // this will also close pr

				assert.Eventually(t, func() bool {
					return assert.Contains(t, logs.String(), tt.wantLog)
				}, time.Second*1, time.Millisecond+100)
			}
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

func Test_Executor_captureContainersLogs(t *testing.T) {
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
	e := executor{
		pod:        &api.Pod{ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"}},
		kubeClient: testKubernetesClient(version, fake.CreateHTTPClient(fakeRoundTripper)),
	}
	e.BuildLogger = common.NewBuildLogger(&common.Trace{Writer: &logs}, logrus.NewEntry(lentry))

	ctx := context.Background()

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
			e.Build.Services = common.Services{
				{Name: "postgres", Alias: "db"},
				{Name: "redis:latest", Alias: "cache"},
			}
			e.Build.Variables = common.JobVariables{
				{Key: "CI_DEBUG_SERVICES", Value: tt.debugServicePolicy, Public: true},
			}

			e.captureContainersLogs(ctx, containers)
			tt.assert(t)
		})
	}
}
