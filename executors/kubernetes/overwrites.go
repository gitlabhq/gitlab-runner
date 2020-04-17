package kubernetes

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/api/resource"

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
	// CPULimitOverwriteVariableValue is the key for the JobVariable containing user overwritten cpu limit
	CPULimitOverwriteVariableValue = "KUBERNETES_CPU_LIMIT"
	// CPURequestOverwriteVariableValue is the key for the JobVariable containing user overwritten cpu limit
	CPURequestOverwriteVariableValue = "KUBERNETES_CPU_REQUEST"
	// MemoryLimitOverwriteVariableValue is the key for the JobVariable containing user overwritten memory limit
	MemoryLimitOverwriteVariableValue = "KUBERNETES_MEMORY_LIMIT"
	// MemoryRequestOverwriteVariableValue is the key for the JobVariable containing user overwritten memory limit
	MemoryRequestOverwriteVariableValue = "KUBERNETES_MEMORY_REQUEST"
)

type overwrites struct {
	namespace      string
	serviceAccount string
	bearerToken    string
	podAnnotations map[string]string
	cpuLimit       string
	cpuRequest     string
	memoryLimit    string
	memoryRequest  string
}

func createOverwrites(config *common.KubernetesConfig, variables common.JobVariables, logger common.BuildLogger) (*overwrites, error) {
	var err error
	o := &overwrites{}

	variables = variables.Expand()

	namespaceOverwrite := variables.Get(NamespaceOverwriteVariableName)
	o.namespace, err = o.evaluateOverwrite("Namespace", config.Namespace, config.NamespaceOverwriteAllowed, namespaceOverwrite, logger)
	if err != nil {
		return nil, err
	}

	serviceAccountOverwrite := variables.Get(ServiceAccountOverwriteVariableName)
	o.serviceAccount, err = o.evaluateOverwrite("ServiceAccount", config.ServiceAccount, config.ServiceAccountOverwriteAllowed, serviceAccountOverwrite, logger)
	if err != nil {
		return nil, err
	}

	bearerTokenOverwrite := variables.Get(BearerTokenOverwriteVariableValue)
	o.bearerToken, err = o.evaluateBoolControlledOverwrite("BearerToken", config.BearerToken, config.BearerTokenOverwriteAllowed, bearerTokenOverwrite, logger)
	if err != nil {
		return nil, err
	}

	o.podAnnotations, err = o.evaluateMapOverwrite("PodAnnotations", config.PodAnnotations, config.PodAnnotationsOverwriteAllowed, variables, PodAnnotationsOverwriteVariablePrefix, logger)
	if err != nil {
		return nil, err
	}

	cpuLimitOverwrite := variables.Get(CPULimitOverwriteVariableValue)
	o.cpuLimit, err = o.evaluateMaxResourceOverwrite("CPULimit", config.CPULimit, config.CPULimitOverwriteMaxAllowed, cpuLimitOverwrite, logger)
	if err != nil {
		return nil, err
	}

	cpuRequestOverwrite := variables.Get(CPURequestOverwriteVariableValue)
	o.cpuRequest, err = o.evaluateMaxResourceOverwrite("CPURequest", config.CPURequest, config.CPURequestOverwriteMaxAllowed, cpuRequestOverwrite, logger)
	if err != nil {
		return nil, err
	}

	memoryLimitOverwrite := variables.Get(MemoryLimitOverwriteVariableValue)
	o.memoryLimit, err = o.evaluateMaxResourceOverwrite("MemoryLimit", config.MemoryLimit, config.MemoryLimitOverwriteMaxAllowed, memoryLimitOverwrite, logger)
	if err != nil {
		return nil, err
	}

	memoryRequestOverwrite := variables.Get(MemoryRequestOverwriteVariableValue)
	o.memoryRequest, err = o.evaluateMaxResourceOverwrite("MemoryRequest", config.MemoryRequest, config.MemoryRequestOverwriteMaxAllowed, memoryRequestOverwrite, logger)
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
		return fmt.Errorf("provided value %q does not match regex %q", value, regex)
	}
	return nil
}

// splitMapOverwrite splits provided string on the first "=" and returns (key, value, nil).
// If the argument cannot be split an error is returned
func splitMapOverwrite(str string) (string, string, error) {
	if split := strings.SplitN(str, "=", 2); len(split) > 1 {
		return split[0], split[1], nil
	}

	return "", "", fmt.Errorf("provided value %q is malformed, does not match k=v", str)
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

func (o *overwrites) evaluateMaxResourceOverwrite(fieldName, value, maxResource, overwriteValue string, logger common.BuildLogger) (string, error) {
	if maxResource == "" {
		logger.Debugln("setting allowing overrides for", fieldName, "is empty, disabling override.")
		return value, nil
	}

	if overwriteValue == "" {
		return value, nil
	}

	var rMaxResource, rOverwriteValue resource.Quantity
	var err error

	if rMaxResource, err = resource.ParseQuantity(maxResource); err != nil {
		return value, fmt.Errorf("error parsing resource limit: %q", err.Error())
	}

	if rOverwriteValue, err = resource.ParseQuantity(overwriteValue); err != nil {
		return value, fmt.Errorf("error parsing resource limit: %q", err.Error())
	}

	ov := rOverwriteValue.Value()
	mr := rMaxResource.Value()

	if ov > mr {
		return value, fmt.Errorf("the resource %q requested by the build %q does not match or is less than limit allowed %q", fieldName, overwriteValue, maxResource)
	}

	logger.Println(fmt.Sprintf("%q overwritten with %q", fieldName, overwriteValue))

	return overwriteValue, nil
}
