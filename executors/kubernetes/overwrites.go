package kubernetes

import (
	"fmt"
	"regexp"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const (
	// NamespaceOverwriteVariableName is the key for the JobVariable containing user overwritten Namespace
	NamespaceOverwriteVariableName = "KUBERNETES_NAMESPACE_OVERWRITE"
	// ServiceAccountOverwriteVariableName is the key for the JobVariable containing user overwritten ServiceAccount
	ServiceAccountOverwriteVariableName = "KUBERNETES_SERVICE_ACCOUNT_OVERWRITE"
	// BearerTokenOverwriteVariableValue is the key for the JobVariable containing user overwritten BearerToken
	BearerTokenOverwriteVariableValue = "KUBERNETES_BEARER_TOKEN"
)

type overwrites struct {
	namespace      string
	serviceAccount string
	bearerToken    string
}

func createOverwrites(config *common.KubernetesConfig, variables common.JobVariables, logger common.BuildLogger) (*overwrites, error) {
	var err error
	o := &overwrites{}

	namespaceOverwrite := variables.Expand().Get(NamespaceOverwriteVariableName)
	o.namespace, err = o.evaluateOverwrite("Namespace", config.Namespace, config.NamespaceOverwriteAllowed, namespaceOverwrite, logger)
	if err != nil {
		return nil, err
	}

	serviceAccountOverwrite := variables.Expand().Get(ServiceAccountOverwriteVariableName)
	o.serviceAccount, err = o.evaluateOverwrite("ServiceAccount", config.ServiceAccount, config.ServiceAccountOverwriteAllowed, serviceAccountOverwrite, logger)
	if err != nil {
		return nil, err
	}

	bearerTokenOverwrite := variables.Expand().Get(BearerTokenOverwriteVariableValue)
	o.bearerToken, err = o.evaluateBoolControlledOverwrite("BearerToken", config.BearerToken, config.BearerTokenOverwriteAllowed, bearerTokenOverwrite, logger)
	if err != nil {
		return nil, err
	}

	return o, nil
}

func (o *overwrites) evaluateBoolControlledOverwrite(fieldName, value string, canOverride bool, overwriteValue string, logger common.BuildLogger) (string, error) {
	if canOverride {
		return o.evaluateOverwrite(fieldName, value, ".+", overwriteValue, logger)
	}
	return o.evaluateOverwrite(fieldName, value, "", overwriteValue, logger)
}

func (o *overwrites) evaluateOverwrite(fieldName, value, regex, overwriteValue string, logger common.BuildLogger) (string, error) {
	if regex == "" {
		logger.Debugln("Regex allowing overrides for", fieldName, "is empty, disabling override.")
		return value, nil
	}

	if overwriteValue == "" {
		return value, nil
	}

	if err := overwriteRegexCheck(regex, overwriteValue); err != nil {
		return value, err
	}

	logValue := overwriteValue
	if fieldName == "BearerToken" {
		logValue = logValue[0:14] + "XXXXXXXX..."
	}

	logger.Println(fmt.Sprintf("%q overwritten with %q", fieldName, logValue))

	return overwriteValue, nil
}

func overwriteRegexCheck(regex, value string) error {
	var err error
	var r *regexp.Regexp
	if r, err = regexp.Compile(regex); err != nil {
		return err
	}

	if match := r.MatchString(value); !match {
		return fmt.Errorf("Provided value %q does not match regex %q", value, regex)
	}
	return nil
}
