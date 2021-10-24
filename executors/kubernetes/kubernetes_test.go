//go:build !integration
// +build !integration

package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"runtime"
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
	tests := map[string]struct {
		GlobalConfig *common.Config
		RunnerConfig common.RunnerConfig
		Build        *common.Build

		Expected []api.VolumeMount
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
			Expected: []api.VolumeMount{
				{Name: "repo", MountPath: "/builds"},
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
			Expected: []api.VolumeMount{
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
			Expected: []api.VolumeMount{
				{Name: "test", MountPath: "/opt/test/readonly", ReadOnly: true},
				{Name: "docker", MountPath: "/var/run/docker.sock"},
				{Name: "secret", MountPath: "/path/to/secret", ReadOnly: true},
				{Name: "configMap", MountPath: "/path/to/configmap", ReadOnly: true},
				{Name: "csi", MountPath: "/path/to/csi/volume", ReadOnly: true},
				{Name: "repo", MountPath: "/builds"},
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
			Expected: []api.VolumeMount{
				{
					Name:      "repo",
					MountPath: "/path/to/builds/dir",
				},
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
			Expected: []api.VolumeMount{
				{Name: "user-provided", MountPath: "/path/to/builds/dir"},
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
			assert.Equal(t, tt.Expected, e.getVolumeMounts())
		})
	}
}

func testVolumesFeatureFlag(t *testing.T, featureFlagName string, featureFlagValue bool) {
	csiVolFSType := "ext4"
	csiVolReadOnly := false
	mode := int32(0777)
	optional := false
	//nolint:lll
	tests := map[string]struct {
		GlobalConfig *common.Config
		RunnerConfig common.RunnerConfig
		Build        *common.Build

		Expected []api.Volume
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
			Expected: []api.Volume{
				{Name: "repo", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{}}},
				{
					Name: "scripts", VolumeSource: api.VolumeSource{
						ConfigMap: &api.ConfigMapVolumeSource{
							LocalObjectReference: api.LocalObjectReference{
								Name: fakeConfigMap().Name,
							},
							DefaultMode: &mode,
							Optional:    &optional,
						},
					},
				},
				{Name: "logs", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{}}},
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
			Expected: []api.Volume{
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
				{
					Name: "scripts", VolumeSource: api.VolumeSource{
						ConfigMap: &api.ConfigMapVolumeSource{
							LocalObjectReference: api.LocalObjectReference{
								Name: fakeConfigMap().Name,
							},
							DefaultMode: &mode,
							Optional:    &optional,
						},
					},
				},
				{Name: "logs", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{}}},
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
			Expected: []api.Volume{
				{Name: "repo", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{}}},
				{
					Name: "scripts", VolumeSource: api.VolumeSource{
						ConfigMap: &api.ConfigMapVolumeSource{
							LocalObjectReference: api.LocalObjectReference{
								Name: fakeConfigMap().Name,
							},
							DefaultMode: &mode,
							Optional:    &optional,
						},
					},
				},
				{Name: "logs", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{}}},
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
			Expected: []api.Volume{
				{Name: "user-provided", VolumeSource: api.VolumeSource{HostPath: &api.HostPathVolumeSource{Path: "/path/to/builds/dir"}}},
				{
					Name: "scripts", VolumeSource: api.VolumeSource{
						ConfigMap: &api.ConfigMapVolumeSource{
							LocalObjectReference: api.LocalObjectReference{
								Name: fakeConfigMap().Name,
							},
							DefaultMode: &mode,
							Optional:    &optional,
						},
					},
				},
				{Name: "logs", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{}}},
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
				configMap: fakeConfigMap(),
			}

			buildtest.SetBuildFeatureFlag(e.Build, featureFlagName, featureFlagValue)
			assert.Equal(t, tt.Expected, e.getVolumes())
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
		body, errRT := ioutil.ReadAll(req.Body)
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
		configMap:       fakeConfigMap(),
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
			Name:      "ConfigMap cleanup failed",
			ConfigMap: &api.ConfigMap{ObjectMeta: objectMeta},
			ClientFunc: func(t *testing.T, req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == http.MethodDelete && p == configMapsEndpointURI:
					return fakeKubeDeleteResponse(http.StatusNotFound), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
			Error: true,
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
					b, err := ioutil.ReadAll(req.Body)
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
					b, err := ioutil.ReadAll(req.Body)
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
				configMap:   test.ConfigMap,
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
		Architecture:   "x86_64",
		OSType:         os,
		Shell:          shells.SNPwsh,
		GitLabRegistry: true,
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
						PullPolicy:                   common.StringOrArray{"if-not-present"},
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
			Name:         "DockerHub helper image",
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
					Variables: common.JobVariables{
						common.JobVariable{
							Key:      featureflags.GitLabRegistryHelperImage,
							Value:    "false",
							Public:   false,
							Internal: false,
							File:     false,
							Masked:   false,
							Raw:      false,
						},
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
				helperImageInfo: helperimage.Info{
					OSType:                  os,
					Architecture:            "x86_64",
					Name:                    helperimage.DockerHubName,
					Tag:                     fmt.Sprintf("x86_64-%s", helperImageTag),
					IsSupportingLocalImport: true,
					Cmd:                     []string{"gitlab-runner-build"},
				},
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
			Name:         "helper image with ubuntu flavour DockerHub registry",
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
					Variables: common.JobVariables{
						common.JobVariable{
							Key:      featureflags.GitLabRegistryHelperImage,
							Value:    "false",
							Public:   false,
							Internal: false,
							File:     false,
							Masked:   false,
							Raw:      false,
						},
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
					serviceLimits:   api.ResourceList{},
					buildLimits:     api.ResourceList{},
					helperLimits:    api.ResourceList{},
					serviceRequests: api.ResourceList{},
					buildRequests:   api.ResourceList{},
					helperRequests:  api.ResourceList{},
				},
				helperImageInfo: helperimage.Info{
					OSType:                  os,
					Architecture:            "x86_64",
					Name:                    helperimage.DockerHubName,
					Tag:                     fmt.Sprintf("ubuntu-x86_64-%s", helperImageTag),
					IsSupportingLocalImport: true,
					Cmd:                     []string{"gitlab-runner-build"},
				},
			},
		},
		{
			Name:         "helper image with ubuntu flavour DockerHub registry",
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host:              "test-server",
						HelperImageFlavor: "alpine3.13",
					},
				},
			},
			Build: &common.Build{
				JobResponse: common.JobResponse{
					Image: common.Image{
						Name: "test-image",
					},
					Variables: common.JobVariables{
						common.JobVariable{
							Key:      featureflags.GitLabRegistryHelperImage,
							Value:    "false",
							Public:   false,
							Internal: false,
							File:     false,
							Masked:   false,
							Raw:      false,
						},
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
					serviceLimits:   api.ResourceList{},
					buildLimits:     api.ResourceList{},
					helperLimits:    api.ResourceList{},
					serviceRequests: api.ResourceList{},
					buildRequests:   api.ResourceList{},
					helperRequests:  api.ResourceList{},
				},
				helperImageInfo: helperimage.Info{
					OSType:                  os,
					Architecture:            "x86_64",
					Name:                    helperimage.DockerHubName,
					Tag:                     fmt.Sprintf("alpine3.13-x86_64-%s", helperImageTag),
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
				configurationOverwrites: defaultOverwrites,
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
					OSType:                  "windows",
					Architecture:            "x86_64",
					Name:                    helperimage.GitLabRegistryName,
					Tag:                     fmt.Sprintf("x86_64-%s-servercore2004", helperImageTag),
					IsSupportingLocalImport: false,
					Cmd: []string{
						"powershell",
						"-NoProfile",
						"-NoLogo",
						"-InputFormat",
						"text",
						"-OutputFormat",
						"text",
						"-NonInteractive",
						"-ExecutionPolicy",
						"Bypass",
						"-Command",
						"-",
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
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			e := &executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: executorOptions,
				},
			}

			// TODO: handle the context properly with https://gitlab.com/gitlab-org/gitlab-runner/-/issues/27932
			prepareOptions := common.ExecutorPrepareOptions{
				Config:  test.RunnerConfig,
				Build:   test.Build,
				Context: context.TODO(),
			}
			prepareOptions.Build.Runner.Executor = "kubernetes"

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
			podBytes, err := ioutil.ReadAll(req.Body)
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
	rt.executed = true
	dataBytes, err := ioutil.ReadAll(req.Body)
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
				}, pod.ObjectMeta.Annotations)
			},
			Variables: []common.JobVariable{
				{Key: "test", Value: "sometestvar"},
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
				assert.Equal(t, "servicename-non-compatble", e.services[0].GenerateName)
				assert.NotEmpty(t, e.ProxyPool["service,name-.non-compat!ble"])
				assert.Equal(
					t,
					"port,name-.non-compat!ble",
					e.ProxyPool["service,name-.non-compat!ble"].Settings.Ports[0].Name,
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
		"supports adding ownerReferences to a configMap": {
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
				require.Len(t, e.configMap.OwnerReferences, 1)

				ownerReference := e.configMap.OwnerReferences[0]
				assert.Equal(t, "v1", ownerReference.APIVersion)
				assert.Equal(t, "Pod", ownerReference.Kind)
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
				configMap:  fakeConfigMap(),
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

			err = ex.prepareOverwrites(make(common.JobVariables, 0))
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

			tc.lineCh <- "line"
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

func fakeConfigMap() *api.ConfigMap {
	configMap := &api.ConfigMap{}
	configMap.Name = "fake"
	return configMap
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
func (f FakeBuildTrace) SetMasked(masked []string)                         {}
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

func TestGenerateScripts(t *testing.T) {
	testErr := errors.New("testErr")

	successfulResponse, err := common.GetRemoteSuccessfulMultistepBuild()
	require.NoError(t, err)

	extension := func(shell *common.ShellScriptInfo) string {
		switch shell.Shell {
		case shells.SNPwsh, shells.SNPowershell:
			return "ps1"
		}
		return ""
	}

	setupMockShellGenerateScript := func(m *common.MockShell, e *executor, stages []common.BuildStage) {
		m.On("GetConfiguration", mock.Anything).
			Return(&common.ShellConfiguration{Extension: extension(e.Shell())}, nil).
			Maybe()

		for _, s := range stages {
			m.On("GenerateScript", s, e.ExecutorOptions.Shell).
				Return("OK", nil).
				Once()
		}
	}

	setupScripts := func(e *executor, stages []common.BuildStage) map[string]string {
		scripts := map[string]string{}
		shell := e.Shell().Shell
		ext := extension(e.Shell())
		if ext != "" {
			ext = "." + ext
		}
		switch shell {
		case shells.SNPwsh, shells.SNPowershell:
			scripts[pwshJSONTerminationScriptName+ext] = shells.PwshJSONTerminationScript(shell)
		default:
			scripts[detectShellScriptName+ext] = shells.BashDetectShellScript
		}

		for _, s := range stages {
			scripts[string(s)+ext] = "OK"
		}

		return scripts
	}

	tests := map[string]struct {
		getExecutor        func() *executor
		setupMockShell     func(e *executor) *common.MockShell
		getExpectedScripts func(e *executor) map[string]string
		expectedErr        error
	}{
		"all stages OK": {
			getExecutor: func() *executor {
				return setupExecutor("bash", successfulResponse)
			},
			setupMockShell: func(e *executor) *common.MockShell {
				buildStages := e.Build.BuildStages()
				m := new(common.MockShell)
				setupMockShellGenerateScript(m, e, buildStages)

				return m
			},
			getExpectedScripts: func(e *executor) map[string]string {
				buildStages := e.Build.BuildStages()
				return setupScripts(e, buildStages)
			},
			expectedErr: nil,
		},
		"all stages OK with pwsh": {
			getExecutor: func() *executor {
				return setupExecutor(shells.SNPwsh, successfulResponse)
			},
			setupMockShell: func(e *executor) *common.MockShell {
				buildStages := e.Build.BuildStages()
				m := new(common.MockShell)
				setupMockShellGenerateScript(m, e, buildStages)

				return m
			},
			getExpectedScripts: func(e *executor) map[string]string {
				buildStages := e.Build.BuildStages()
				return setupScripts(e, buildStages)
			},
			expectedErr: nil,
		},
		"stage returns skip build stage error": {
			getExecutor: func() *executor {
				return setupExecutor("bash", successfulResponse)
			},
			setupMockShell: func(e *executor) *common.MockShell {
				buildStages := e.Build.BuildStages()
				m := new(common.MockShell)
				m.On("GenerateScript", buildStages[0], e.ExecutorOptions.Shell).
					Return("", common.ErrSkipBuildStage).
					Once()

				setupMockShellGenerateScript(m, e, buildStages[1:])

				return m
			},
			getExpectedScripts: func(e *executor) map[string]string {
				buildStages := e.Build.BuildStages()
				return setupScripts(e, buildStages[1:])
			},
			expectedErr: nil,
		},
		"stage returns error": {
			getExecutor: func() *executor {
				return setupExecutor("bash", successfulResponse)
			},
			setupMockShell: func(e *executor) *common.MockShell {
				buildStages := e.Build.BuildStages()
				m := new(common.MockShell)
				m.On("GetConfiguration", mock.Anything).
					Return(&common.ShellConfiguration{Extension: extension(e.Shell())}, nil).
					Maybe()
				m.On("GenerateScript", buildStages[0], e.ExecutorOptions.Shell).
					Return("", testErr).
					Once()

				return m
			},
			getExpectedScripts: func(e *executor) map[string]string {
				return nil
			},
			expectedErr: testErr,
		},
	}

	for tn, tt := range tests {
		t.Run(tn, func(t *testing.T) {
			e := tt.getExecutor()
			expectedScripts := tt.getExpectedScripts(e)
			m := tt.setupMockShell(e)
			defer m.AssertExpectations(t)

			scripts, err := e.generateScripts(m)
			assert.ErrorIs(t, err, tt.expectedErr)
			assert.Equal(t, expectedScripts, scripts)
		})
	}
}

func TestExecutor_buildPermissionsInitContainer(t *testing.T) {
	dockerHub, err := helperimage.Get(common.REVISION, helperimage.Config{
		OSType:       helperimage.OSTypeLinux,
		Architecture: "amd64",
	})
	require.NoError(t, err)

	gitlabRegistry, err := helperimage.Get(common.REVISION, helperimage.Config{
		OSType:         helperimage.OSTypeLinux,
		Architecture:   "amd64",
		GitLabRegistry: true,
	})
	require.NoError(t, err)

	tests := map[string]struct {
		expectedImage string
		jobVariables  common.JobVariables
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
		"helper image from DockerHub": {
			expectedImage: dockerHub.String(),
			jobVariables: []common.JobVariable{
				{
					Key:    featureflags.GitLabRegistryHelperImage,
					Value:  "false",
					Public: true,
				},
			},
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
						JobResponse: common.JobResponse{
							Variables: tt.jobVariables,
						},
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
			assert.Len(t, c.VolumeMounts, 1)
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
				Executor: "kubernetes",
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
	executor.Build = new(common.Build)
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
