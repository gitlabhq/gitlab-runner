package kubernetes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gitlab.com/gitlab-org/gitlab-runner/session"

	api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest/fake"

	"gitlab.com/gitlab-org/gitlab-runner/common"
	"gitlab.com/gitlab-org/gitlab-runner/executors"
	"gitlab.com/gitlab-org/gitlab-runner/helpers"
)

var (
	TRUE = true
)

const (
	TestTimeout = 15 * time.Second
)

func TestLimits(t *testing.T) {
	tests := []struct {
		CPU, Memory string
		Expected    api.ResourceList
	}{
		{
			CPU:    "100m",
			Memory: "100Mi",
			Expected: api.ResourceList{
				api.ResourceCPU:    resource.MustParse("100m"),
				api.ResourceMemory: resource.MustParse("100Mi"),
			},
		},
		{
			CPU: "100m",
			Expected: api.ResourceList{
				api.ResourceCPU: resource.MustParse("100m"),
			},
		},
		{
			Memory: "100Mi",
			Expected: api.ResourceList{
				api.ResourceMemory: resource.MustParse("100Mi"),
			},
		},
		{
			CPU:      "100j",
			Expected: api.ResourceList{},
		},
		{
			Memory:   "100j",
			Expected: api.ResourceList{},
		},
		{
			Expected: api.ResourceList{},
		},
	}

	for _, test := range tests {
		res, _ := limits(test.CPU, test.Memory)
		assert.Equal(t, test.Expected, res)
	}
}

func TestVolumeMounts(t *testing.T) {
	tests := []struct {
		GlobalConfig *common.Config
		RunnerConfig common.RunnerConfig
		Build        *common.Build

		Expected []api.VolumeMount
	}{
		{
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
				{Name: "repo"},
			},
		},
		{
			GlobalConfig: &common.Config{},
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Volumes: common.KubernetesVolumes{
							HostPaths: []common.KubernetesHostPath{
								{Name: "docker", MountPath: "/var/run/docker.sock", HostPath: "/var/run/docker.sock"},
							},
							PVCs: []common.KubernetesPVC{
								{Name: "PVC", MountPath: "/path/to/whatever"},
							},
							EmptyDirs: []common.KubernetesEmptyDir{
								{Name: "emptyDir", MountPath: "/path/to/empty/dir"},
							},
						},
					},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: []api.VolumeMount{
				{Name: "repo"},
				{Name: "docker", MountPath: "/var/run/docker.sock"},
				{Name: "PVC", MountPath: "/path/to/whatever"},
				{Name: "emptyDir", MountPath: "/path/to/empty/dir"},
			},
		},
		{
			GlobalConfig: &common.Config{},
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Volumes: common.KubernetesVolumes{
							HostPaths: []common.KubernetesHostPath{
								{Name: "test", MountPath: "/opt/test/readonly", ReadOnly: true, HostPath: "/opt/test/rw"},
								{Name: "docker", MountPath: "/var/run/docker.sock"},
							},
							ConfigMaps: []common.KubernetesConfigMap{
								{Name: "configMap", MountPath: "/path/to/configmap", ReadOnly: true},
							},
							Secrets: []common.KubernetesSecret{
								{Name: "secret", MountPath: "/path/to/secret", ReadOnly: true},
							},
						},
					},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: []api.VolumeMount{
				{Name: "repo"},
				{Name: "test", MountPath: "/opt/test/readonly", ReadOnly: true},
				{Name: "docker", MountPath: "/var/run/docker.sock"},
				{Name: "secret", MountPath: "/path/to/secret", ReadOnly: true},
				{Name: "configMap", MountPath: "/path/to/configmap", ReadOnly: true},
			},
		},
	}

	for _, test := range tests {
		e := &executor{
			AbstractExecutor: executors.AbstractExecutor{
				ExecutorOptions: executorOptions,
				Build:           test.Build,
				Config:          test.RunnerConfig,
			},
		}

		mounts := e.getVolumeMounts()
		for _, expected := range test.Expected {
			assert.Contains(t, mounts, expected, "Expected volumeMount definition for %s was not found", expected.Name)
		}
	}
}

func TestVolumes(t *testing.T) {
	tests := []struct {
		GlobalConfig *common.Config
		RunnerConfig common.RunnerConfig
		Build        *common.Build

		Expected []api.Volume
	}{
		{
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
			},
		},
		{
			GlobalConfig: &common.Config{},
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Volumes: common.KubernetesVolumes{
							HostPaths: []common.KubernetesHostPath{
								{Name: "docker", MountPath: "/var/run/docker.sock"},
								{Name: "host-path", MountPath: "/path/two", HostPath: "/path/one"},
							},
							PVCs: []common.KubernetesPVC{
								{Name: "PVC", MountPath: "/path/to/whatever"},
							},
							ConfigMaps: []common.KubernetesConfigMap{
								{Name: "ConfigMap", MountPath: "/path/to/config", Items: map[string]string{"key_1": "/path/to/key_1"}},
							},
							Secrets: []common.KubernetesSecret{
								{Name: "secret", MountPath: "/path/to/secret", ReadOnly: true, Items: map[string]string{"secret_1": "/path/to/secret_1"}},
							},
							EmptyDirs: []common.KubernetesEmptyDir{
								{Name: "emptyDir", MountPath: "/path/to/empty/dir", Medium: "Memory"},
							},
						},
					},
				},
			},
			Build: &common.Build{
				Runner: &common.RunnerConfig{},
			},
			Expected: []api.Volume{
				{Name: "repo", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{}}},
				{Name: "docker", VolumeSource: api.VolumeSource{HostPath: &api.HostPathVolumeSource{Path: "/var/run/docker.sock"}}},
				{Name: "host-path", VolumeSource: api.VolumeSource{HostPath: &api.HostPathVolumeSource{Path: "/path/one"}}},
				{Name: "PVC", VolumeSource: api.VolumeSource{PersistentVolumeClaim: &api.PersistentVolumeClaimVolumeSource{ClaimName: "PVC"}}},
				{Name: "emptyDir", VolumeSource: api.VolumeSource{EmptyDir: &api.EmptyDirVolumeSource{Medium: "Memory"}}},
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
					Name: "secret",
					VolumeSource: api.VolumeSource{
						Secret: &api.SecretVolumeSource{
							SecretName: "secret",
							Items:      []api.KeyToPath{{Key: "secret_1", Path: "/path/to/secret_1"}},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		e := &executor{
			AbstractExecutor: executors.AbstractExecutor{
				ExecutorOptions: executorOptions,
				Build:           test.Build,
				Config:          test.RunnerConfig,
			},
		}

		volumes := e.getVolumes()
		for _, expected := range test.Expected {
			assert.Contains(t, volumes, expected, "Expected volume definition for %s was not found", expected.Name)
		}
	}
}

func fakeKubeDeleteResponse(status int) *http.Response {
	_, codec := testVersionAndCodec()

	body := objBody(codec, &metav1.Status{Code: int32(status)})
	return &http.Response{StatusCode: status, Body: body, Header: map[string][]string{
		"Content-Type": []string{"application/json"},
	}}
}

func TestCleanup(t *testing.T) {
	version, _ := testVersionAndCodec()

	objectMeta := metav1.ObjectMeta{Name: "test-resource", Namespace: "test-ns"}

	tests := []struct {
		Name        string
		Pod         *api.Pod
		Credentials *api.Secret
		ClientFunc  func(*http.Request) (*http.Response, error)
		Error       bool
	}{
		{
			Name: "Proper Cleanup",
			Pod:  &api.Pod{ObjectMeta: objectMeta},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == "DELETE" && p == "/api/"+version+"/namespaces/test-ns/pods/test-resource":
					return fakeKubeDeleteResponse(http.StatusOK), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
		},
		{
			Name: "Delete failure",
			Pod:  &api.Pod{ObjectMeta: objectMeta},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("delete failed")
			},
			Error: true,
		},
		{
			Name: "POD already deleted",
			Pod:  &api.Pod{ObjectMeta: objectMeta},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == "DELETE" && p == "/api/"+version+"/namespaces/test-ns/pods/test-resource":
					return fakeKubeDeleteResponse(http.StatusNotFound), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
			Error: true,
		},
		{
			Name:        "POD creation failed, Secretes provided",
			Pod:         nil, // a failed POD create request will cause a nil Pod
			Credentials: &api.Secret{ObjectMeta: objectMeta},
			ClientFunc: func(req *http.Request) (*http.Response, error) {
				switch p, m := req.URL.Path, req.Method; {
				case m == "DELETE" && p == "/api/"+version+"/namespaces/test-ns/secrets/test-resource":
					return fakeKubeDeleteResponse(http.StatusNotFound), nil
				default:
					return nil, fmt.Errorf("unexpected request. method: %s, path: %s", m, p)
				}
			},
			Error: true,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			ex := executor{
				kubeClient:  testKubernetesClient(version, fake.CreateHTTPClient(test.ClientFunc)),
				pod:         test.Pod,
				credentials: test.Credentials,
			}
			ex.configurationOverwrites = &overwrites{namespace: "test-ns"}
			errored := false
			buildTrace := FakeBuildTrace{
				testWriter{
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
	tests := []struct {
		GlobalConfig *common.Config
		RunnerConfig *common.RunnerConfig
		Build        *common.Build

		Expected *executor
		Error    bool
	}{
		{
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host:               "test-server",
						ServiceCPULimit:    "100m",
						ServiceMemoryLimit: "200Mi",
						CPULimit:           "1.5",
						MemoryLimit:        "4Gi",
						HelperCPULimit:     "50m",
						HelperMemoryLimit:  "100Mi",
						Privileged:         true,
						PullPolicy:         "if-not-present",
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
				configurationOverwrites: &overwrites{namespace: "default"},
				serviceLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("100m"),
					api.ResourceMemory: resource.MustParse("200Mi"),
				},
				buildLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("1.5"),
					api.ResourceMemory: resource.MustParse("4Gi"),
				},
				helperLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("50m"),
					api.ResourceMemory: resource.MustParse("100Mi"),
				},
				serviceRequests: api.ResourceList{},
				buildRequests:   api.ResourceList{},
				helperRequests:  api.ResourceList{},
				pullPolicy:      "IfNotPresent",
			},
		},
		{
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
						CPULimit:                       "1.5",
						MemoryLimit:                    "4Gi",
						HelperCPULimit:                 "50m",
						HelperMemoryLimit:              "100Mi",
						ServiceCPURequest:              "99m",
						ServiceMemoryRequest:           "5Mi",
						CPURequest:                     "1",
						MemoryRequest:                  "1.5Gi",
						HelperCPURequest:               "0.5m",
						HelperMemoryRequest:            "42Mi",
						Privileged:                     false,
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
				configurationOverwrites: &overwrites{namespace: "default", serviceAccount: "not-default"},
				serviceLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("100m"),
					api.ResourceMemory: resource.MustParse("200Mi"),
				},
				buildLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("1.5"),
					api.ResourceMemory: resource.MustParse("4Gi"),
				},
				helperLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("50m"),
					api.ResourceMemory: resource.MustParse("100Mi"),
				},
				serviceRequests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("99m"),
					api.ResourceMemory: resource.MustParse("5Mi"),
				},
				buildRequests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("1"),
					api.ResourceMemory: resource.MustParse("1.5Gi"),
				},
				helperRequests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("0.5m"),
					api.ResourceMemory: resource.MustParse("42Mi"),
				},
			},
			Error: false,
		},

		{
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Host:                           "test-server",
						ServiceAccount:                 "default",
						ServiceAccountOverwriteAllowed: "allowed-.*",
						ServiceCPULimit:                "100m",
						ServiceMemoryLimit:             "200Mi",
						CPULimit:                       "1.5",
						MemoryLimit:                    "4Gi",
						HelperCPULimit:                 "50m",
						HelperMemoryLimit:              "100Mi",
						ServiceCPURequest:              "99m",
						ServiceMemoryRequest:           "5Mi",
						CPURequest:                     "1",
						MemoryRequest:                  "1.5Gi",
						HelperCPURequest:               "0.5m",
						HelperMemoryRequest:            "42Mi",
						Privileged:                     false,
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
				configurationOverwrites: &overwrites{namespace: "namespacee"},
				serviceLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("100m"),
					api.ResourceMemory: resource.MustParse("200Mi"),
				},
				buildLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("1.5"),
					api.ResourceMemory: resource.MustParse("4Gi"),
				},
				helperLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("50m"),
					api.ResourceMemory: resource.MustParse("100Mi"),
				},
				serviceRequests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("99m"),
					api.ResourceMemory: resource.MustParse("5Mi"),
				},
				buildRequests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("1"),
					api.ResourceMemory: resource.MustParse("1.5Gi"),
				},
				helperRequests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("0.5m"),
					api.ResourceMemory: resource.MustParse("42Mi"),
				},
			},
			Error: true,
		},
		{
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
						CPULimit:                       "1.5",
						MemoryLimit:                    "4Gi",
						HelperCPULimit:                 "50m",
						HelperMemoryLimit:              "100Mi",
						ServiceCPURequest:              "99m",
						ServiceMemoryRequest:           "5Mi",
						CPURequest:                     "1",
						MemoryRequest:                  "1.5Gi",
						HelperCPURequest:               "0.5m",
						HelperMemoryRequest:            "42Mi",
						Privileged:                     false,
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
						{Key: NamespaceOverwriteVariableName, Value: "namespacee"},
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
				configurationOverwrites: &overwrites{namespace: "namespacee", serviceAccount: "a_service_account"},
				serviceLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("100m"),
					api.ResourceMemory: resource.MustParse("200Mi"),
				},
				buildLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("1.5"),
					api.ResourceMemory: resource.MustParse("4Gi"),
				},
				helperLimits: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("50m"),
					api.ResourceMemory: resource.MustParse("100Mi"),
				},
				serviceRequests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("99m"),
					api.ResourceMemory: resource.MustParse("5Mi"),
				},
				buildRequests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("1"),
					api.ResourceMemory: resource.MustParse("1.5Gi"),
				},
				helperRequests: api.ResourceList{
					api.ResourceCPU:    resource.MustParse("0.5m"),
					api.ResourceMemory: resource.MustParse("42Mi"),
				},
			},
			Error: true,
		},
		{
			GlobalConfig: &common.Config{},
			RunnerConfig: &common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "namespace",
						Host:      "test-server",
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
						{Key: NamespaceOverwriteVariableName, Value: "namespace"},
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
				configurationOverwrites: &overwrites{namespace: "namespace"},
				serviceLimits:           api.ResourceList{},
				buildLimits:             api.ResourceList{},
				helperLimits:            api.ResourceList{},
				serviceRequests:         api.ResourceList{},
				buildRequests:           api.ResourceList{},
				helperRequests:          api.ResourceList{},
			},
		},
		{
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
				configurationOverwrites: &overwrites{namespace: "default"},
				serviceLimits:           api.ResourceList{},
				buildLimits:             api.ResourceList{},
				helperLimits:            api.ResourceList{},
				serviceRequests:         api.ResourceList{},
				buildRequests:           api.ResourceList{},
				helperRequests:          api.ResourceList{},
			},
		},
		{
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
				configurationOverwrites: &overwrites{namespace: "default"},
				serviceLimits:           api.ResourceList{},
				buildLimits:             api.ResourceList{},
				helperLimits:            api.ResourceList{},
				serviceRequests:         api.ResourceList{},
				buildRequests:           api.ResourceList{},
				helperRequests:          api.ResourceList{},
			},
		},
	}

	for index, test := range tests {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			e := &executor{
				AbstractExecutor: executors.AbstractExecutor{
					ExecutorOptions: executorOptions,
				},
			}

			prepareOptions := common.ExecutorPrepareOptions{
				Config:  test.RunnerConfig,
				Build:   test.Build,
				Context: context.TODO(),
			}

			err := e.Prepare(prepareOptions)

			if err != nil {
				assert.False(t, test.Build.IsSharedEnv())
				if test.Error {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
				if !test.Error {
					t.Errorf("Got error. Expected: %v", test.Expected)
				}
				return
			}

			// Set this to nil so we aren't testing the functionality of the
			// base AbstractExecutor's Prepare method
			e.AbstractExecutor = executors.AbstractExecutor{}

			// TODO: Improve this so we don't have to nil-ify the kubeClient.
			// It currently contains some moving parts that are failing, meaning
			// we'll need to mock _something_
			e.kubeClient = nil
			assert.Equal(t, test.Expected, e)
		})
	}
}

// This test reproduces the bug reported in https://gitlab.com/gitlab-org/gitlab-runner/issues/2583
func TestPrepareIssue2583(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	namespace := "my_namespace"
	serviceAccount := "my_account"

	runnerConfig := &common.RunnerConfig{
		RunnerSettings: common.RunnerSettings{
			Executor: "kubernetes",
			Kubernetes: &common.KubernetesConfig{
				Image:                          "an/image:latest",
				Namespace:                      namespace,
				NamespaceOverwriteAllowed:      ".*",
				ServiceAccount:                 serviceAccount,
				ServiceAccountOverwriteAllowed: ".*",
			},
		},
	}

	build := &common.Build{
		JobResponse: common.JobResponse{
			Variables: []common.JobVariable{
				{Key: NamespaceOverwriteVariableName, Value: "namespace"},
				{Key: ServiceAccountOverwriteVariableName, Value: "sa"},
			},
		},
		Runner: &common.RunnerConfig{},
	}

	e := &executor{
		AbstractExecutor: executors.AbstractExecutor{
			ExecutorOptions: executorOptions,
		},
	}

	prepareOptions := common.ExecutorPrepareOptions{
		Config:  runnerConfig,
		Build:   build,
		Context: context.TODO(),
	}

	err := e.Prepare(prepareOptions)
	assert.NoError(t, err)
	assert.Equal(t, namespace, runnerConfig.Kubernetes.Namespace)
	assert.Equal(t, serviceAccount, runnerConfig.Kubernetes.ServiceAccount)
}

func TestSetupCredentials(t *testing.T) {
	version, _ := testVersionAndCodec()

	type testDef struct {
		Credentials []common.Credentials
		VerifyFn    func(*testing.T, testDef, *api.Secret)
	}
	tests := []testDef{
		{
			// don't execute VerifyFn
			VerifyFn: nil,
		},
		{
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
		{
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

	for _, test := range tests {
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
	}
}

type setupBuildPodTestDef struct {
	RunnerConfig common.RunnerConfig
	Variables    []common.JobVariable
	Options      *kubernetesOptions
	PrepareFn    func(*testing.T, setupBuildPodTestDef, *executor)
	VerifyFn     func(*testing.T, setupBuildPodTestDef, *api.Pod)
}

type setupBuildPodFakeRoundTripper struct {
	t        *testing.T
	test     setupBuildPodTestDef
	executed bool
}

func (rt *setupBuildPodFakeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.executed = true
	podBytes, err := ioutil.ReadAll(req.Body)
	if !assert.NoError(rt.t, err, "failed to read request body") {
		return nil, err
	}

	p := new(api.Pod)
	err = json.Unmarshal(podBytes, p)
	if !assert.NoError(rt.t, err, "failed to read request body") {
		return nil, err
	}

	rt.test.VerifyFn(rt.t, rt.test, p)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body: FakeReadCloser{
			Reader: bytes.NewBuffer(podBytes),
		},
	}
	resp.Header = make(http.Header)
	resp.Header.Add("Content-Type", "application/json")

	return resp, nil
}

func TestSetupBuildPod(t *testing.T) {
	version, _ := testVersionAndCodec()

	tests := map[string]setupBuildPodTestDef{
		"passes node selector setting": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
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
						Namespace: "default",
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
		"configures helper container": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace: "default",
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				hasHelper := false
				for _, c := range pod.Spec.Containers {
					if c.Name == "helper" {
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
						Namespace:   "default",
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
						Namespace: "default",
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
						Namespace: "default",
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
						Namespace:   "default",
						HelperImage: "custom/helper-image:${CI_RUNNER_REVISION}",
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				for _, c := range pod.Spec.Containers {
					if c.Name == "helper" {
						assert.Equal(t, "custom/helper-image:HEAD", c.Image)
					}
				}
			},
		},
		"supports extended docker configuration for image and services": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace:   "default",
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
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				require.Len(t, pod.Spec.Containers, 3)

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
			},
		},
		"supports pod security context": {
			RunnerConfig: common.RunnerConfig{
				RunnerSettings: common.RunnerSettings{
					Kubernetes: &common.KubernetesConfig{
						Namespace:   "default",
						PodSecurityContext:   common.KubernetesSecurityContext{
							FSGroup: 200,
							RunAsGroup: 200,
							RunAsNonRoot: true,
							RunAsUser: 200,
							SupplementalGroups: []int64{200},
						}, 
					},
				},
			},
			VerifyFn: func(t *testing.T, test setupBuildPodTestDef, pod *api.Pod) {
				assert.Equal(t, int64(200), pod.Spec.SecurityContext.FSGroup)
				assert.Equal(t, int64(200), pod.Spec.SecurityContext.RunAsGroup)
				assert.Equal(t, int64(200), pod.Spec.SecurityContext.RunAsUser)
				assert.Equal(t, true, pod.Spec.SecurityContext.RunAsNonRoot)
				assert.Equal(t, []int64{200}, pod.Spec.SecurityContext.SupplementalGroups)
			},
		},
	}

	for testName, test := range tests {
		t.Run(testName, func(t *testing.T) {
			vars := test.Variables
			if vars == nil {
				vars = []common.JobVariable{}
			}

			options := test.Options
			if options == nil {
				options = &kubernetesOptions{}
			}

			rt := setupBuildPodFakeRoundTripper{
				t:    t,
				test: test,
			}

			ex := executor{
				kubeClient: testKubernetesClient(version, fake.CreateHTTPClient(rt.RoundTrip)),
				options:    options,
				AbstractExecutor: executors.AbstractExecutor{
					Config:     test.RunnerConfig,
					BuildShell: &common.ShellConfiguration{},
					Build: &common.Build{
						JobResponse: common.JobResponse{
							Variables: vars,
						},
						Runner: &test.RunnerConfig,
					},
				},
			}

			if test.PrepareFn != nil {
				test.PrepareFn(t, test, &ex)
			}

			err := ex.prepareOverwrites(make(common.JobVariables, 0))
			assert.NoError(t, err, "error preparing overwrites")

			err = ex.setupBuildPod()
			assert.NoError(t, err, "error setting up build pod")

			assert.True(t, rt.executed, "RoundTrip for kubernetes client should be executed")
		})
	}
}

func TestKubernetesSuccessRun(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	successfulBuild, err := common.GetRemoteSuccessfulBuild()
	assert.NoError(t, err)
	successfulBuild.Image.Name = common.TestDockerGitImage
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func TestKubernetesNoRootImage(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	successfulBuild, err := common.GetRemoteSuccessfulBuildWithDumpedVariables()

	assert.NoError(t, err)
	successfulBuild.Image.Name = common.TestAlpineNoRootImage
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
	}

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.NoError(t, err)
}

func TestKubernetesBuildFail(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	failedBuild, err := common.GetRemoteFailedBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: failedBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
	}
	build.Image.Name = common.TestDockerGitImage

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err, "error")
	assert.IsType(t, err, &common.BuildError{})
	assert.Contains(t, err.Error(), "command terminated with exit code")
}

func TestKubernetesMissingImage(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	failedBuild, err := common.GetRemoteFailedBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: failedBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor:   "kubernetes",
				Kubernetes: &common.KubernetesConfig{},
			},
		},
	}
	build.Image.Name = "some/non-existing/image"

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.IsType(t, err, &common.BuildError{})
	assert.Contains(t, err.Error(), "image pull failed")
}

func TestKubernetesMissingTag(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	failedBuild, err := common.GetRemoteFailedBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: failedBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor:   "kubernetes",
				Kubernetes: &common.KubernetesConfig{},
			},
		},
	}
	build.Image.Name = "docker:missing-tag"

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.IsType(t, err, &common.BuildError{})
	assert.Contains(t, err.Error(), "image pull failed")
}

func TestKubernetesBuildAbort(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	failedBuild, err := common.GetRemoteFailedBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: failedBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
		SystemInterrupt: make(chan os.Signal, 1),
	}
	build.Image.Name = common.TestDockerGitImage

	abortTimer := time.AfterFunc(time.Second, func() {
		t.Log("Interrupt")
		build.SystemInterrupt <- os.Interrupt
	})
	defer abortTimer.Stop()

	timeoutTimer := time.AfterFunc(time.Minute, func() {
		t.Log("Timedout")
		t.FailNow()
	})
	defer timeoutTimer.Stop()

	err = build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	assert.EqualError(t, err, "aborted: interrupt")
}

func TestKubernetesBuildCancel(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	failedBuild, err := common.GetRemoteFailedBuild()
	assert.NoError(t, err)
	build := &common.Build{
		JobResponse: failedBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					PullPolicy: common.PullPolicyIfNotPresent,
				},
			},
		},
		SystemInterrupt: make(chan os.Signal, 1),
	}
	build.Image.Name = common.TestDockerGitImage

	trace := &common.Trace{Writer: os.Stdout}

	abortTimer := time.AfterFunc(time.Second, func() {
		t.Log("Interrupt")
		trace.CancelFunc()
	})
	defer abortTimer.Stop()

	timeoutTimer := time.AfterFunc(time.Minute, func() {
		t.Log("Timedout")
		t.FailNow()
	})
	defer timeoutTimer.Stop()

	err = build.Run(&common.Config{}, trace)
	assert.IsType(t, err, &common.BuildError{})
	assert.EqualError(t, err, "canceled")
}

func TestOverwriteNamespaceNotMatch(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	build := &common.Build{
		JobResponse: common.JobResponse{
			GitInfo: common.GitInfo{
				Sha: "1234567890",
			},
			Image: common.Image{
				Name: "test-image",
			},
			Variables: []common.JobVariable{
				{Key: NamespaceOverwriteVariableName, Value: "namespace"},
			},
		},
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					NamespaceOverwriteAllowed: "^not_a_match$",
					PullPolicy:                common.PullPolicyIfNotPresent,
				},
			},
		},
		SystemInterrupt: make(chan os.Signal, 1),
	}
	build.Image.Name = common.TestDockerGitImage

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestOverwriteServiceAccountNotMatch(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	build := &common.Build{
		JobResponse: common.JobResponse{
			GitInfo: common.GitInfo{
				Sha: "1234567890",
			},
			Image: common.Image{
				Name: "test-image",
			},
			Variables: []common.JobVariable{
				{Key: ServiceAccountOverwriteVariableName, Value: "service-account"},
			},
		},
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					ServiceAccountOverwriteAllowed: "^not_a_match$",
					PullPolicy:                     common.PullPolicyIfNotPresent,
				},
			},
		},
		SystemInterrupt: make(chan os.Signal, 1),
	}
	build.Image.Name = common.TestDockerGitImage

	err := build.Run(&common.Config{}, &common.Trace{Writer: os.Stdout})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestInteractiveTerminal(t *testing.T) {
	if helpers.SkipIntegrationTests(t, "kubectl", "cluster-info") {
		return
	}

	client, err := getKubeClient(&common.KubernetesConfig{}, &overwrites{})
	require.NoError(t, err)
	secrets, err := client.CoreV1().Secrets("default").List(metav1.ListOptions{})
	require.NoError(t, err)

	successfulBuild, err := common.GetRemoteBuildResponse("sleep 5")
	require.NoError(t, err)
	successfulBuild.Image.Name = "docker:git"
	build := &common.Build{
		JobResponse: successfulBuild,
		Runner: &common.RunnerConfig{
			RunnerSettings: common.RunnerSettings{
				Executor: "kubernetes",
				Kubernetes: &common.KubernetesConfig{
					BearerToken: string(secrets.Items[0].Data["token"]),
				},
			},
		},
	}

	sess, err := session.NewSession(nil)
	build.Session = sess

	outBuffer := bytes.NewBuffer(nil)
	outCh := make(chan string)

	go func() {
		err = build.Run(
			&common.Config{
				SessionServer: common.SessionServer{
					SessionTimeout: 2,
				},
			},
			&common.Trace{Writer: outBuffer},
		)
		require.NoError(t, err)

		outCh <- outBuffer.String()
	}()

	for build.Session.Mux() == nil {
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(5 * time.Second)

	srv := httptest.NewServer(build.Session.Mux())
	defer srv.Close()

	u := url.URL{Scheme: "ws", Host: srv.Listener.Addr().String(), Path: build.Session.Endpoint + "/exec"}
	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), http.Header{"Authorization": []string{build.Session.Token}})
	defer conn.Close()
	require.NoError(t, err)
	assert.Equal(t, resp.StatusCode, http.StatusSwitchingProtocols)

	out := <-outCh
	t.Log(out)

	assert.Contains(t, out, "Terminal is connected, will time out in 2s...")
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

func (f FakeBuildTrace) Success()                                              {}
func (f FakeBuildTrace) Fail(err error, failureReason common.JobFailureReason) {}
func (f FakeBuildTrace) Notify(func())                                         {}
func (f FakeBuildTrace) SetCancelFunc(cancelFunc context.CancelFunc)           {}
func (f FakeBuildTrace) SetFailuresCollector(fc common.FailuresCollector)      {}
func (f FakeBuildTrace) IsStdout() bool {
	return false
}
