package kubernetes

import (
	"os"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

func buildOverwriteVariables(namespace, serviceAccount string) common.JobVariables {
	variables := make(common.JobVariables, 2)

	if namespace != "" {
		variables = append(variables, common.JobVariable{Key: NamespaceOverwriteVariableName, Value: namespace})
	}

	if serviceAccount != "" {
		variables = append(variables, common.JobVariable{Key: ServiceAccountOverwriteVariableName, Value: serviceAccount})
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
	}

	tests := []struct {
		Name                                 string
		Config                               *common.KubernetesConfig
		NamespaceOverwriteVariableValue      string
		ServiceAccountOverwriteVariableValue string
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
			Expected: &overwrites{namespace: "my_namespace", serviceAccount: "my_service_account"},
		},
		{
			Name:   "No overwrites allowed",
			Config: &common.KubernetesConfig{Namespace: "my_namespace", ServiceAccount: "my_service_account"},
			NamespaceOverwriteVariableValue:      "another_namespace",
			ServiceAccountOverwriteVariableValue: "another_service_account",
			Expected: &overwrites{namespace: "my_namespace", serviceAccount: "my_service_account"},
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
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			assert := assert.New(t)

			values, err := createOverwrites(test.Config, buildOverwriteVariables(test.NamespaceOverwriteVariableValue, test.ServiceAccountOverwriteVariableValue), logger)
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
