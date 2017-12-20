package kubernetes

import (
	"fmt"
	"regexp"
	"strings"

	"gitlab.com/gitlab-org/gitlab-runner/common"
)

const (
	// NamespaceOverwriteVariableName is the key for the JobVariable containing user overwritten Namespace
	NamespaceOverwriteVariableName = "KUBERNETES_NAMESPACE_OVERWRITE"
	// ServiceAccountOverwriteVariableName is the key for the JobVariable containing user overwritten ServiceAccount
	ServiceAccountOverwriteVariableName = "KUBERNETES_SERVICE_ACCOUNT_OVERWRITE"
	// BearerTokenOverwriteVariableValue is the key for the JobVariable containing user overwritten BearerToken
	BearerTokenOverwriteVariableValue = "KUBERNETES_BEARER_TOKEN"
	// PodAnnotationsOverwriteVariablePrefix is the prefix for all the JobVariable keys containing user overwritten PodAnnotations
	PodAnnotationsOverwriteVariablePrefix = "KUBERNETES_POD_ANNOTATIONS_"
)

type overwrites struct {
	namespace      string
	serviceAccount string
	bearerToken    string
	podAnnotations map[string]string
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

	o.podAnnotations, err = o.evaluateMapOverwrite("PodAnnotations", config.PodAnnotations, config.PodAnnotationsOverwriteAllowed, variables, PodAnnotationsOverwriteVariablePrefix, logger)
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
		logValue = "XXXXXXXX..."
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

// splitMapOverwrite splits provided string on the first "=" and returns (key, value, nil).
// If the argument cannot be split an error is returned
func splitMapOverwrite(str string) (string, string, error) {
	if split := strings.SplitN(str, "=", 2); len(split) > 1 {
		return split[0], split[1], nil
	}

	return "", "", fmt.Errorf("Provided value %q is malformed, does not match k=v", str)
}

func (o *overwrites) evaluateMapOverwrite(fieldName string, values map[string]string, regex string, variables common.JobVariables, variablesSelector string, logger common.BuildLogger) (map[string]string, error) {
	if regex == "" {
		logger.Debugln("Regex allowing overrides for", fieldName, "is empty, disabling override.")
		return values, nil
	}

	finalValues := make(map[string]string)
	for k, v := range values {
		finalValues[k] = v
	}

	for _, variable := range variables {
		if strings.HasPrefix(variable.Key, variablesSelector) {
			if err := overwriteRegexCheck(regex, variable.Value); err != nil {
				return nil, err
			}

			key, value, err := splitMapOverwrite(variable.Value)
			if err != nil {
				return nil, err
			}

			finalValues[key] = value
			logger.Println(fmt.Sprintf("%q %q overwritten with %q", fieldName, key, value))
		}
	}
	return finalValues, nil
}
