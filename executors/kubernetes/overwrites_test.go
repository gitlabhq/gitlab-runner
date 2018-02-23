package kubernetes

import (
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func buildOverwriteVariables(namespace, serviceAccount, bearerToken string, podAnnotations map[string]string) common.JobVariables {
	variables := make(common.JobVariables, 4)

	if namespace != "" {
		variables = append(variables, common.JobVariable{Key: NamespaceOverwriteVariableName, Value: namespace})
	}

	if serviceAccount != "" {
		variables = append(variables, common.JobVariable{Key: ServiceAccountOverwriteVariableName, Value: serviceAccount})
	}

	if bearerToken != "" {
		variables = append(variables, common.JobVariable{Key: BearerTokenOverwriteVariableValue, Value: bearerToken})
	}

	for k, v := range podAnnotations {
		variables = append(variables, common.JobVariable{Key: k, Value: v})
	}
	return variables
}

func stdoutLogger() common.BuildLogger {
	return common.NewBuildLogger(&common.Trace{Writer: os.Stdout}, logrus.WithFields(logrus.Fields{}))
}

func TestOverwrites(t *testing.T) {
	logger := stdoutLogger()
	overwritesAllowedConfig := &common.KubernetesConfig{
		NamespaceOverwriteAllowed:      ".*",
		ServiceAccountOverwriteAllowed: ".*",
		BearerTokenOverwriteAllowed:    true,
		PodAnnotationsOverwriteAllowed: ".*",
		PodAnnotations: map[string]string{
			"test1":                     "test1",
			"test2":                     "test2",
			"test3":                     "test3",
			"org.gitlab/runner-version": "v10.4.0",
			"org.gitlab/gitlab-host":    "https://gitlab.example.com",
			"iam.amazonaws.com/role":    "arn:aws:iam::123456789012:role/",
		},
	}

	tests := []struct {
		Name                                 string
		Config                               *common.KubernetesConfig
		NamespaceOverwriteVariableValue      string
		ServiceAccountOverwriteVariableValue string
		BearerTokenOverwriteVariableValue    string
		PodAnnotationsOverwriteValues        map[string]string
		Expected                             *overwrites
		Error                                bool
	}{
		{
			Name:     "Empty Configuration",
			Config:   &common.KubernetesConfig{},
			Expected: &overwrites{},
		},
		{
			Name:   "All overwrites allowed",
			Config: overwritesAllowedConfig,
			NamespaceOverwriteVariableValue:      "my_namespace",
			ServiceAccountOverwriteVariableValue: "my_service_account",
			BearerTokenOverwriteVariableValue:    "my_bearer_token",
			PodAnnotationsOverwriteValues: map[string]string{
				"KUBERNETES_POD_ANNOTATIONS_1":            "test3=test3=1",
				"KUBERNETES_POD_ANNOTATIONS_2":            "test4=test4",
				"KUBERNETES_POD_ANNOTATIONS_gilabversion": "org.gitlab/runner-version=v10.4.0-override",
				"KUBERNETES_POD_ANNOTATIONS_kube2iam":     "iam.amazonaws.com/role=arn:aws:iam::kjcbs;dkjbck=jxzweopiu:role/",
			},
			Expected: &overwrites{
				namespace:      "my_namespace",
				serviceAccount: "my_service_account",
				bearerToken:    "my_bearer_token",
				podAnnotations: map[string]string{
					"test1":                     "test1",
					"test2":                     "test2",
					"test3":                     "test3=1",
					"test4":                     "test4",
					"org.gitlab/runner-version": "v10.4.0-override",
					"org.gitlab/gitlab-host":    "https://gitlab.example.com",
					"iam.amazonaws.com/role":    "arn:aws:iam::kjcbs;dkjbck=jxzweopiu:role/",
				},
			},
		},
		{
			Name: "No overwrites allowed",
			Config: &common.KubernetesConfig{
				Namespace:      "my_namespace",
				ServiceAccount: "my_service_account",
				BearerToken:    "my_bearer_token",
				PodAnnotations: map[string]string{
					"test1": "test1",
					"test2": "test2",
				},
			},
			NamespaceOverwriteVariableValue:      "another_namespace",
			ServiceAccountOverwriteVariableValue: "another_service_account",
			BearerTokenOverwriteVariableValue:    "another_bearer_token",
			PodAnnotationsOverwriteValues: map[string]string{
				"KUBERNETES_POD_ANNOTATIONS_1": "test3=test3",
				"KUBERNETES_POD_ANNOTATIONS_2": "test4=test4",
			},
			Expected: &overwrites{
				namespace:      "my_namespace",
				serviceAccount: "my_service_account",
				bearerToken:    "my_bearer_token",
				podAnnotations: map[string]string{
					"test1": "test1",
					"test2": "test2",
				},
			},
		},
		{
			Name: "Namespace failure",
			Config: &common.KubernetesConfig{
				NamespaceOverwriteAllowed: "not-a-match",
			},
			NamespaceOverwriteVariableValue: "my_namespace",
			Error: true,
		},
		{
			Name: "ServiceAccount failure",
			Config: &common.KubernetesConfig{
				ServiceAccountOverwriteAllowed: "not-a-match",
			},
			ServiceAccountOverwriteVariableValue: "my_service_account",
			Error: true,
		},
		{
			Name: "PodAnnotations failure",
			Config: &common.KubernetesConfig{
				PodAnnotationsOverwriteAllowed: "not-a-match",
			},
			PodAnnotationsOverwriteValues: map[string]string{
				"KUBERNETES_POD_ANNOTATIONS_1": "test1=test1",
			},
			Error: true,
		},
		{
			Name: "PodAnnotations malformed key",
			Config: &common.KubernetesConfig{
				PodAnnotationsOverwriteAllowed: ".*",
			},
			PodAnnotationsOverwriteValues: map[string]string{
				"KUBERNETES_POD_ANNOTATIONS_1": "test1",
			},
			Error: true,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)
			variables := buildOverwriteVariables(test.NamespaceOverwriteVariableValue, test.ServiceAccountOverwriteVariableValue, test.BearerTokenOverwriteVariableValue, test.PodAnnotationsOverwriteValues)
			values, err := createOverwrites(test.Config, variables, logger)
			if test.Error {
				assert.Error(err)
				assert.Contains(err.Error(), "does not match")
			} else {
				assert.NoError(err)
				assert.Equal(test.Expected, values)
			}
		})
	}
}
