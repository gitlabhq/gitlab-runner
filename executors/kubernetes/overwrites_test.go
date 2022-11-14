//go:build !integration

package kubernetes

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	api "k8s.io/api/core/v1"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

type variableOverwrites map[string]string

func buildOverwriteVariables(overwrites variableOverwrites, globOverwrites ...map[string]string) common.JobVariables {
	variables := make(common.JobVariables, 8)

	for variableKey, overwriteValue := range overwrites {
		if overwriteValue != "" {
			variables = append(variables, common.JobVariable{Key: variableKey, Value: overwriteValue})
		}
	}

	// KUBERNETES_NODE_SELECTOR_*
	// KUBERNETES_POD_ANNOTATIONS_*
	// KUBERNETES_POD_LABELS_*
	for _, glob := range globOverwrites {
		for k, v := range glob {
			variables = append(variables, common.JobVariable{Key: k, Value: v})
		}
	}

	return variables
}

func stdoutLogger() common.BuildLogger {
	return common.NewBuildLogger(&common.Trace{Writer: os.Stdout}, logrus.WithFields(logrus.Fields{}))
}

func TestOverwrites(t *testing.T) {
	logger := stdoutLogger()

	//nolint:lll
	tests := []struct {
		Name                                 string
		Config                               *common.KubernetesConfig
		NamespaceOverwriteVariableValue      string
		ServiceAccountOverwriteVariableValue string
		BearerTokenOverwriteVariableValue    string
		NodeSelectorOverwriteValues          map[string]string
		PodLabelsOverwriteValues             map[string]string
		PodAnnotationsOverwriteValues        map[string]string
		Expected                             *overwrites
		Error                                error

		CPULimitOverwriteVariableValue                string
		MemoryLimitOverwriteVariableValue             string
		EphemeralStorageLimitOverwriteVariableValue   string
		CPURequestOverwriteVariableValue              string
		MemoryRequestOverwriteVariableValue           string
		EphemeralStorageRequestOverwriteVariableValue string

		ServiceCPULimitOverwriteVariableValue                string
		ServiceMemoryLimitOverwriteVariableValue             string
		ServiceEphemeralStorageLimitOverwriteVariableValue   string
		ServiceCPURequestOverwriteVariableValue              string
		ServiceMemoryRequestOverwriteVariableValue           string
		ServiceEphemeralStorageRequestOverwriteVariableValue string

		HelperCPULimitOverwriteVariableValue                string
		HelperMemoryLimitOverwriteVariableValue             string
		HelperEphemeralStorageLimitOverwriteVariableValue   string
		HelperCPURequestOverwriteVariableValue              string
		HelperMemoryRequestOverwriteVariableValue           string
		HelperEphemeralStorageRequestOverwriteVariableValue string
	}{
		{
			Name:   "Empty Configuration",
			Config: &common.KubernetesConfig{},
			Expected: &overwrites{
				buildLimits:     api.ResourceList{},
				buildRequests:   api.ResourceList{},
				serviceLimits:   api.ResourceList{},
				serviceRequests: api.ResourceList{},
				helperLimits:    api.ResourceList{},
				helperRequests:  api.ResourceList{},
			},
		},
		{
			Name: "All overwrites allowed",
			Config: &common.KubernetesConfig{
				NamespaceOverwriteAllowed:      ".*",
				ServiceAccountOverwriteAllowed: ".*",
				BearerTokenOverwriteAllowed:    true,
				NodeSelectorOverwriteAllowed:   ".*",
				NodeSelector: map[string]string{
					"test1":                          "test1",
					"test2":                          "test2",
					"kubernetes.io/arch":             "amd64",
					"eks.amazonaws.com/capacityType": "SPOT",
				},
				PodLabelsOverwriteAllowed:      ".*",
				PodAnnotationsOverwriteAllowed: ".*",
				PodLabels: map[string]string{
					"app":               "gitlab-runner",
					"chart":             "gitlab-runner-0.27.0",
					"heritage":          "Helm",
					"pod-template-hash": "84dbf9bc67",
					"release":           "gitlab-runner",
				},
				PodAnnotations: map[string]string{
					"test1":                     "test1",
					"test2":                     "test2",
					"test3":                     "test3",
					"org.gitlab/runner-version": "v10.4.0",
					"org.gitlab/gitlab-host":    "https://gitlab.example.com",
					"iam.amazonaws.com/role":    "arn:aws:iam::123456789012:role/",
				},
				CPULimit:                                          "1.5",
				CPULimitOverwriteMaxAllowed:                       "3.5",
				MemoryLimit:                                       "5Gi",
				MemoryLimitOverwriteMaxAllowed:                    "10Gi",
				EphemeralStorageLimit:                             "15Gi",
				EphemeralStorageLimitOverwriteMaxAllowed:          "115Gi",
				CPURequest:                                        "1",
				CPURequestOverwriteMaxAllowed:                     "2",
				MemoryRequest:                                     "1.5Gi",
				MemoryRequestOverwriteMaxAllowed:                  "8Gi",
				EphemeralStorageRequest:                           "12Gi",
				EphemeralStorageRequestOverwriteMaxAllowed:        "110Gi",
				ServiceCPULimit:                                   "100m",
				ServiceCPULimitOverwriteMaxAllowed:                "1000m",
				ServiceMemoryLimit:                                "200Mi",
				ServiceMemoryLimitOverwriteMaxAllowed:             "2000Mi",
				ServiceEphemeralStorageLimit:                      "300Mi",
				ServiceEphemeralStorageLimitOverwriteMaxAllowed:   "3000Mi",
				ServiceCPURequest:                                 "99m",
				ServiceCPURequestOverwriteMaxAllowed:              "900m",
				ServiceMemoryRequest:                              "5m",
				ServiceMemoryRequestOverwriteMaxAllowed:           "55Mi",
				ServiceEphemeralStorageRequest:                    "16Mi",
				ServiceEphemeralStorageRequestOverwriteMaxAllowed: "165Mi",
				HelperCPULimit:                                    "50m",
				HelperCPULimitOverwriteMaxAllowed:                 "555m",
				HelperMemoryLimit:                                 "100Mi",
				HelperMemoryLimitOverwriteMaxAllowed:              "1010Mi",
				HelperEphemeralStorageLimit:                       "200Mi",
				HelperEphemeralStorageLimitOverwriteMaxAllowed:    "2010Mi",
				HelperCPURequest:                                  "0.5m",
				HelperCPURequestOverwriteMaxAllowed:               "9.5m",
				HelperMemoryRequest:                               "42Mi",
				HelperMemoryRequestOverwriteMaxAllowed:            "126Mi",
				HelperEphemeralStorageRequest:                     "62Mi",
				HelperEphemeralStorageRequestOverwriteMaxAllowed:  "127Mi",
			},
			NamespaceOverwriteVariableValue:      "my_namespace",
			ServiceAccountOverwriteVariableValue: "my_service_account",
			BearerTokenOverwriteVariableValue:    "my_bearer_token",
			NodeSelectorOverwriteValues: map[string]string{
				"KUBERNETES_NODE_SELECTOR_SPOT": "eks.amazonaws.com/capacityType=ON_DEMAND",
				"KUBERNETES_NODE_SELECTOR_ARCH": "kubernetes.io/arch=arm64",
			},
			PodLabelsOverwriteValues: map[string]string{
				"KUBERNETES_POD_LABELS_1":     "test5=test6=1",
				"KUBERNETES_POD_LABELS_2":     "test7=test8",
				"KUBERNETES_POD_LABELS_chart": "chart=gitlab-runner-0.27.0-override",
			},
			PodAnnotationsOverwriteValues: map[string]string{
				"KUBERNETES_POD_ANNOTATIONS_1":            "test3=test3=1",
				"KUBERNETES_POD_ANNOTATIONS_2":            "test4=test4",
				"KUBERNETES_POD_ANNOTATIONS_gilabversion": "org.gitlab/runner-version=v10.4.0-override",
				"KUBERNETES_POD_ANNOTATIONS_kube2iam":     "iam.amazonaws.com/role=arn:aws:iam::kjcbs;dkjbck=jxzweopiu:role/",
			},
			CPULimitOverwriteVariableValue:                       "3",
			MemoryLimitOverwriteVariableValue:                    "10Gi",
			EphemeralStorageLimitOverwriteVariableValue:          "16Gi",
			CPURequestOverwriteVariableValue:                     "2",
			MemoryRequestOverwriteVariableValue:                  "3Gi",
			EphemeralStorageRequestOverwriteVariableValue:        "11Gi",
			ServiceCPULimitOverwriteVariableValue:                "200m",
			ServiceMemoryLimitOverwriteVariableValue:             "400Mi",
			ServiceEphemeralStorageLimitOverwriteVariableValue:   "600Mi",
			ServiceCPURequestOverwriteVariableValue:              "198m",
			ServiceMemoryRequestOverwriteVariableValue:           "10Mi",
			ServiceEphemeralStorageRequestOverwriteVariableValue: "110Mi",
			HelperCPULimitOverwriteVariableValue:                 "105m",
			HelperMemoryLimitOverwriteVariableValue:              "202Mi",
			HelperEphemeralStorageLimitOverwriteVariableValue:    "303Mi",
			HelperCPURequestOverwriteVariableValue:               "4.5m",
			HelperMemoryRequestOverwriteVariableValue:            "84Mi",
			HelperEphemeralStorageRequestOverwriteVariableValue:  "96Mi",
			Expected: &overwrites{
				namespace:      "my_namespace",
				serviceAccount: "my_service_account",
				bearerToken:    "my_bearer_token",
				nodeSelector: map[string]string{
					"test1":                          "test1",
					"test2":                          "test2",
					"eks.amazonaws.com/capacityType": "ON_DEMAND",
					"kubernetes.io/arch":             "arm64",
				},
				podLabels: map[string]string{
					"app":               "gitlab-runner",
					"chart":             "gitlab-runner-0.27.0-override",
					"heritage":          "Helm",
					"pod-template-hash": "84dbf9bc67",
					"release":           "gitlab-runner",
					"test5":             "test6=1",
					"test7":             "test8",
				},
				podAnnotations: map[string]string{
					"test1":                     "test1",
					"test2":                     "test2",
					"test3":                     "test3=1",
					"test4":                     "test4",
					"org.gitlab/runner-version": "v10.4.0-override",
					"org.gitlab/gitlab-host":    "https://gitlab.example.com",
					"iam.amazonaws.com/role":    "arn:aws:iam::kjcbs;dkjbck=jxzweopiu:role/",
				},
				buildLimits:     mustCreateResourceList(t, "3", "10Gi", "16Gi"),
				buildRequests:   mustCreateResourceList(t, "2", "3Gi", "11Gi"),
				serviceLimits:   mustCreateResourceList(t, "200m", "400Mi", "600Mi"),
				serviceRequests: mustCreateResourceList(t, "198m", "10Mi", "110Mi"),
				helperLimits:    mustCreateResourceList(t, "105m", "202Mi", "303Mi"),
				helperRequests:  mustCreateResourceList(t, "4.5m", "84Mi", "96Mi"),
			},
		},
		{
			Name: "No overwrites allowed",
			Config: &common.KubernetesConfig{
				Namespace:      "my_namespace",
				ServiceAccount: "my_service_account",
				BearerToken:    "my_bearer_token",
				NodeSelector: map[string]string{
					"test1": "test1",
					"test2": "test2",
				},
				PodLabels: map[string]string{
					"test5": "test5",
					"test6": "test6",
				},
				PodAnnotations: map[string]string{
					"test1": "test1",
					"test2": "test2",
				},
				CPULimit:                       "1.5",
				MemoryLimit:                    "4Gi",
				EphemeralStorageLimit:          "3Gi",
				CPURequest:                     "1",
				MemoryRequest:                  "1.5Gi",
				EphemeralStorageRequest:        "3Gi",
				ServiceCPULimit:                "100m",
				ServiceMemoryLimit:             "200Mi",
				ServiceEphemeralStorageLimit:   "300Mi",
				ServiceCPURequest:              "99m",
				ServiceMemoryRequest:           "5Mi",
				ServiceEphemeralStorageRequest: "10Mi",
				HelperCPULimit:                 "50m",
				HelperMemoryLimit:              "100Mi",
				HelperEphemeralStorageLimit:    "200Mi",
				HelperCPURequest:               "0.5m",
				HelperMemoryRequest:            "42Mi",
				HelperEphemeralStorageRequest:  "38Mi",
			},
			NamespaceOverwriteVariableValue:      "another_namespace",
			ServiceAccountOverwriteVariableValue: "another_service_account",
			BearerTokenOverwriteVariableValue:    "another_bearer_token",
			NodeSelectorOverwriteValues: map[string]string{
				"KUBERNETES_NODE_SELECTOR_1": "test3=test3",
				"KUBERNETES_NODE_SELECTOR_2": "test4=test4",
			},
			PodLabelsOverwriteValues: map[string]string{
				"KUBERNETES_POD_LABELS_1": "test7=test7",
				"KUBERNETES_POD_LABELS_2": "test8=test8",
			},
			PodAnnotationsOverwriteValues: map[string]string{
				"KUBERNETES_POD_ANNOTATIONS_1": "test3=test3",
				"KUBERNETES_POD_ANNOTATIONS_2": "test4=test4",
			},
			CPULimitOverwriteVariableValue:                       "3",
			MemoryLimitOverwriteVariableValue:                    "10Gi",
			EphemeralStorageLimitOverwriteVariableValue:          "16Gi",
			CPURequestOverwriteVariableValue:                     "2",
			MemoryRequestOverwriteVariableValue:                  "3Gi",
			EphemeralStorageRequestOverwriteVariableValue:        "11Gi",
			ServiceCPULimitOverwriteVariableValue:                "200m",
			ServiceMemoryLimitOverwriteVariableValue:             "400Mi",
			ServiceEphemeralStorageLimitOverwriteVariableValue:   "17Gi",
			ServiceCPURequestOverwriteVariableValue:              "198m",
			ServiceMemoryRequestOverwriteVariableValue:           "10Mi",
			ServiceEphemeralStorageRequestOverwriteVariableValue: "12Gi",
			HelperCPULimitOverwriteVariableValue:                 "105m",
			HelperMemoryLimitOverwriteVariableValue:              "202Mi",
			HelperEphemeralStorageLimitOverwriteVariableValue:    "18Gi",
			HelperCPURequestOverwriteVariableValue:               "4.5m",
			HelperMemoryRequestOverwriteVariableValue:            "84Mi",
			HelperEphemeralStorageRequestOverwriteVariableValue:  "13Gi",
			Expected: &overwrites{
				namespace:      "my_namespace",
				serviceAccount: "my_service_account",
				bearerToken:    "my_bearer_token",
				nodeSelector: map[string]string{
					"test1": "test1",
					"test2": "test2",
				},
				podLabels: map[string]string{
					"test5": "test5",
					"test6": "test6",
				},
				podAnnotations: map[string]string{
					"test1": "test1",
					"test2": "test2",
				},
				buildLimits:     mustCreateResourceList(t, "1.5", "4Gi", "3Gi"),
				buildRequests:   mustCreateResourceList(t, "1", "1.5Gi", "3Gi"),
				serviceLimits:   mustCreateResourceList(t, "100m", "200Mi", "300Mi"),
				serviceRequests: mustCreateResourceList(t, "99m", "5Mi", "10Mi"),
				helperLimits:    mustCreateResourceList(t, "50m", "100Mi", "200Mi"),
				helperRequests:  mustCreateResourceList(t, "0.5m", "42Mi", "38Mi"),
			},
		},
		{
			Name: "Resource overwrites the same",
			Config: &common.KubernetesConfig{
				CPURequestOverwriteMaxAllowed:              "10",
				CPULimitOverwriteMaxAllowed:                "12",
				MemoryRequestOverwriteMaxAllowed:           "10Gi",
				MemoryLimitOverwriteMaxAllowed:             "12Gi",
				EphemeralStorageRequestOverwriteMaxAllowed: "10Gi",
				EphemeralStorageLimitOverwriteMaxAllowed:   "13Gi",
			},
			CPURequestOverwriteVariableValue:              "10",
			CPULimitOverwriteVariableValue:                "12",
			MemoryRequestOverwriteVariableValue:           "10Gi",
			MemoryLimitOverwriteVariableValue:             "12Gi",
			EphemeralStorageRequestOverwriteVariableValue: "10Gi",
			EphemeralStorageLimitOverwriteVariableValue:   "13Gi",
			Expected: &overwrites{
				buildLimits:     mustCreateResourceList(t, "12", "12Gi", "13Gi"),
				buildRequests:   mustCreateResourceList(t, "10", "10Gi", "10Gi"),
				serviceLimits:   api.ResourceList{},
				serviceRequests: api.ResourceList{},
				helperLimits:    api.ResourceList{},
				helperRequests:  api.ResourceList{},
			},
		},
		{
			Name: "Namespace failure",
			Config: &common.KubernetesConfig{
				NamespaceOverwriteAllowed: "not-a-match",
			},
			NamespaceOverwriteVariableValue: "my_namespace",
			Error:                           new(malformedOverwriteError),
		},
		{
			Name: "ServiceAccount failure",
			Config: &common.KubernetesConfig{
				ServiceAccountOverwriteAllowed: "not-a-match",
			},
			ServiceAccountOverwriteVariableValue: "my_service_account",
			Error:                                new(malformedOverwriteError),
		},
		{
			Name: "NodeSelector failure",
			Config: &common.KubernetesConfig{
				NodeSelectorOverwriteAllowed: "not-a-match",
			},
			NodeSelectorOverwriteValues: map[string]string{
				"KUBERNETES_NODE_SELECTOR_1": "test1=test1",
			},
			Error: new(malformedOverwriteError),
		},
		{
			Name: "NodeSelector malformed key",
			Config: &common.KubernetesConfig{
				NodeSelectorOverwriteAllowed: ".*",
			},
			NodeSelectorOverwriteValues: map[string]string{
				"KUBERNETES_NODE_SELECTOR_1": "test1",
			},
			Error: new(malformedOverwriteError),
		},
		{
			Name: "PodLabels failure",
			Config: &common.KubernetesConfig{
				PodLabelsOverwriteAllowed: "not-a-match",
			},
			PodLabelsOverwriteValues: map[string]string{
				"KUBERNETES_POD_LABELS_1": "test1=test1",
			},
			Error: new(malformedOverwriteError),
		},
		{
			Name: "PodLabels malformed key",
			Config: &common.KubernetesConfig{
				PodLabelsOverwriteAllowed: ".*",
			},
			PodLabelsOverwriteValues: map[string]string{
				"KUBERNETES_POD_LABELS_1": "test1",
			},
			Error: new(malformedOverwriteError),
		},
		{
			Name: "PodAnnotations failure",
			Config: &common.KubernetesConfig{
				PodAnnotationsOverwriteAllowed: "not-a-match",
			},
			PodAnnotationsOverwriteValues: map[string]string{
				"KUBERNETES_POD_ANNOTATIONS_1": "test1=test1",
			},
			Error: new(malformedOverwriteError),
		},
		{
			Name: "PodAnnotations malformed key",
			Config: &common.KubernetesConfig{
				PodAnnotationsOverwriteAllowed: ".*",
			},
			PodAnnotationsOverwriteValues: map[string]string{
				"KUBERNETES_POD_ANNOTATIONS_1": "test1",
			},
			Error: new(malformedOverwriteError),
		},
		{
			Name: "CPULimit too high",
			Config: &common.KubernetesConfig{
				CPULimitOverwriteMaxAllowed: "10",
			},
			CPULimitOverwriteVariableValue: "12",
			Error:                          new(overwriteTooHighError),
		},
		{
			Name: "CPULimit too high using millicpu",
			Config: &common.KubernetesConfig{
				CPULimitOverwriteMaxAllowed: "500m",
			},
			CPULimitOverwriteVariableValue: "600m",
			Error:                          new(overwriteTooHighError),
		},
		{
			Name: "CPURequest too high",
			Config: &common.KubernetesConfig{
				CPURequestOverwriteMaxAllowed: "10",
			},
			CPURequestOverwriteVariableValue: "12",
			Error:                            new(overwriteTooHighError),
		},
		{
			Name: "CPURequest too high using millicpu",
			Config: &common.KubernetesConfig{
				CPURequestOverwriteMaxAllowed: "500m",
			},
			CPURequestOverwriteVariableValue: "600m",
			Error:                            new(overwriteTooHighError),
		},
		{
			Name: "MemoryLimit too high",
			Config: &common.KubernetesConfig{
				MemoryLimitOverwriteMaxAllowed: "2Gi",
			},
			MemoryLimitOverwriteVariableValue: "10Gi",
			Error:                             new(overwriteTooHighError),
		},
		{
			Name: "MemoryLimit too high Mi",
			Config: &common.KubernetesConfig{
				MemoryLimitOverwriteMaxAllowed: "20Mi",
			},
			MemoryLimitOverwriteVariableValue: "10Gi",
			Error:                             new(overwriteTooHighError),
		},
		{
			Name: "MemoryRequest too high",
			Config: &common.KubernetesConfig{
				MemoryRequestOverwriteMaxAllowed: "2Gi",
			},
			MemoryRequestOverwriteVariableValue: "10Gi",
			Error:                               new(overwriteTooHighError),
		},
		{
			Name: "MemoryRequest too high Mi",
			Config: &common.KubernetesConfig{
				MemoryRequestOverwriteMaxAllowed: "20Mi",
			},
			MemoryRequestOverwriteVariableValue: "100Mi",
			Error:                               new(overwriteTooHighError),
		},
		{
			Name: "MemoryRequest too high different suffix",
			Config: &common.KubernetesConfig{
				MemoryRequestOverwriteMaxAllowed: "2Gi",
			},
			MemoryRequestOverwriteVariableValue: "5000Mi",
			Error:                               new(overwriteTooHighError),
		},

		{
			Name: "EphemeralStorageLimit too high",
			Config: &common.KubernetesConfig{
				EphemeralStorageLimitOverwriteMaxAllowed: "2Gi",
			},
			EphemeralStorageLimitOverwriteVariableValue: "10Gi",
			Error: new(overwriteTooHighError),
		},
		{
			Name: "EphemeralStorageLimit too high Mi",
			Config: &common.KubernetesConfig{
				EphemeralStorageLimitOverwriteMaxAllowed: "20Mi",
			},
			EphemeralStorageLimitOverwriteVariableValue: "10Gi",
			Error: new(overwriteTooHighError),
		},
		{
			Name: "EphemeralStorageRequest too high",
			Config: &common.KubernetesConfig{
				EphemeralStorageRequestOverwriteMaxAllowed: "2Gi",
			},
			EphemeralStorageRequestOverwriteVariableValue: "10Gi",
			Error: new(overwriteTooHighError),
		},
		{
			Name: "EphemeralStorageRequest too high Mi",
			Config: &common.KubernetesConfig{
				EphemeralStorageRequestOverwriteMaxAllowed: "20Mi",
			},
			EphemeralStorageRequestOverwriteVariableValue: "100Mi",
			Error: new(overwriteTooHighError),
		},
		{
			Name: "EphemeralStorageRequest too high different suffix",
			Config: &common.KubernetesConfig{
				EphemeralStorageRequestOverwriteMaxAllowed: "2Gi",
			},
			EphemeralStorageRequestOverwriteVariableValue: "5000Mi",
			Error: new(overwriteTooHighError),
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			//nolint:lll
			variables := buildOverwriteVariables(
				variableOverwrites{
					NamespaceOverwriteVariableName:                       test.NamespaceOverwriteVariableValue,
					ServiceAccountOverwriteVariableName:                  test.ServiceAccountOverwriteVariableValue,
					BearerTokenOverwriteVariableValue:                    test.BearerTokenOverwriteVariableValue,
					CPULimitOverwriteVariableValue:                       test.CPULimitOverwriteVariableValue,
					CPURequestOverwriteVariableValue:                     test.CPURequestOverwriteVariableValue,
					MemoryLimitOverwriteVariableValue:                    test.MemoryLimitOverwriteVariableValue,
					MemoryRequestOverwriteVariableValue:                  test.MemoryRequestOverwriteVariableValue,
					EphemeralStorageLimitOverwriteVariableValue:          test.EphemeralStorageLimitOverwriteVariableValue,
					EphemeralStorageRequestOverwriteVariableValue:        test.EphemeralStorageRequestOverwriteVariableValue,
					ServiceCPULimitOverwriteVariableValue:                test.ServiceCPULimitOverwriteVariableValue,
					ServiceCPURequestOverwriteVariableValue:              test.ServiceCPURequestOverwriteVariableValue,
					ServiceMemoryLimitOverwriteVariableValue:             test.ServiceMemoryLimitOverwriteVariableValue,
					ServiceMemoryRequestOverwriteVariableValue:           test.ServiceMemoryRequestOverwriteVariableValue,
					ServiceEphemeralStorageLimitOverwriteVariableValue:   test.ServiceEphemeralStorageLimitOverwriteVariableValue,
					ServiceEphemeralStorageRequestOverwriteVariableValue: test.ServiceEphemeralStorageRequestOverwriteVariableValue,
					HelperCPULimitOverwriteVariableValue:                 test.HelperCPULimitOverwriteVariableValue,
					HelperCPURequestOverwriteVariableValue:               test.HelperCPURequestOverwriteVariableValue,
					HelperMemoryLimitOverwriteVariableValue:              test.HelperMemoryLimitOverwriteVariableValue,
					HelperMemoryRequestOverwriteVariableValue:            test.HelperMemoryRequestOverwriteVariableValue,
					HelperEphemeralStorageLimitOverwriteVariableValue:    test.HelperEphemeralStorageLimitOverwriteVariableValue,
					HelperEphemeralStorageRequestOverwriteVariableValue:  test.HelperEphemeralStorageRequestOverwriteVariableValue,
				},
				test.NodeSelectorOverwriteValues,
				test.PodLabelsOverwriteValues,
				test.PodAnnotationsOverwriteValues,
			)

			values, err := createOverwrites(test.Config, variables, logger)
			assert.ErrorIs(t, err, test.Error)
			assert.Equal(t, test.Expected, values)
		})
	}
}

func Test_overwriteTooHighError_Is(t *testing.T) {
	tests := []struct {
		err        error
		expectedIs bool
	}{
		{
			err:        errors.New("false"),
			expectedIs: false,
		},
		{
			err:        new(emptyTestError),
			expectedIs: false,
		},
		{
			err:        new(overwriteTooHighError),
			expectedIs: true,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%T", tt.err), func(t *testing.T) {
			err := overwriteTooHighError{}
			assert.Equal(t, tt.expectedIs, err.Is(tt.err))
		})
	}
}

type emptyTestError struct{}

func (e *emptyTestError) Error() string {
	return ""
}
