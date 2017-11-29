package kubernetes

import (
	"os"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func buildOverwriteVariables(namespace, serviceAccount, bearerToken string) common.JobVariables {
	variables := make(common.JobVariables, 3)

	if namespace != "" {
		variables = append(variables, common.JobVariable{Key: NamespaceOverwriteVariableName, Value: namespace})
	}

	if serviceAccount != "" {
		variables = append(variables, common.JobVariable{Key: ServiceAccountOverwriteVariableName, Value: serviceAccount})
	}

	if bearerToken != "" {
		variables = append(variables, common.JobVariable{Key: BearerTokenVariableName, Value: bearerToken})
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
	}

	tests := []struct {
		Name                                 string
		Config                               *common.KubernetesConfig
		NamespaceOverwriteVariableValue      string
		ServiceAccountOverwriteVariableValue string
		BearerTokenOverwriteVariableValue    string
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
			Expected: &overwrites{
				namespace:      "my_namespace",
				serviceAccount: "my_service_account",
				bearerToken:    "my_bearer_token",
			},
		},
		{
			Name: "No overwrites allowed",
			Config: &common.KubernetesConfig{
				Namespace:      "my_namespace",
				ServiceAccount: "my_service_account",
				BearerToken:    "my_bearer_token",
			},
			NamespaceOverwriteVariableValue:      "another_namespace",
			ServiceAccountOverwriteVariableValue: "another_service_account",
			BearerTokenOverwriteVariableValue:    "another_bearer_token",
			Expected: &overwrites{
				namespace:      "my_namespace",
				serviceAccount: "my_service_account",
				bearerToken:    "my_bearer_token",
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
			Name: "BearerToken failure",
			Config: &common.KubernetesConfig{
				BearerTokenOverwriteAllowed: false,
			},
			BearerTokenOverwriteVariableValue: "my_bearer_token",
			Error: true,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)

			values, err := createOverwrites(test.Config, buildOverwriteVariables(
				test.NamespaceOverwriteVariableValue,
				test.ServiceAccountOverwriteVariableValue,
				test.BearerTokenOverwriteVariableValue), logger)
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
