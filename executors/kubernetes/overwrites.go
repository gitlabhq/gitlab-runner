package kubernetes

import (
	"fmt"
	"regexp"
	"strings"

	api "k8s.io/api/core/v1"
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
	// PodLabelsOverwriteVariablePrefix is the prefix for all the JobVariable keys containing
	// user overwritten PodLabels
	PodLabelsOverwriteVariablePrefix = "KUBERNETES_POD_LABELS_"
	// PodAnnotationsOverwriteVariablePrefix is the prefix for all the JobVariable keys containing
	// user overwritten PodAnnotations
	PodAnnotationsOverwriteVariablePrefix = "KUBERNETES_POD_ANNOTATIONS_"
	// NodeSelectorOverwriteVariablePrefix is the prefix for all the JobVariable keys containing
	// user overwritten NodeSelectors
	NodeSelectorOverwriteVariablePrefix = "KUBERNETES_NODE_SELECTOR_"
	// CPULimitOverwriteVariableValue is the key for the JobVariable containing user overwritten cpu limit
	CPULimitOverwriteVariableValue = "KUBERNETES_CPU_LIMIT"
	// CPURequestOverwriteVariableValue is the key for the JobVariable containing user overwritten cpu limit
	CPURequestOverwriteVariableValue = "KUBERNETES_CPU_REQUEST"
	// MemoryLimitOverwriteVariableValue is the key for the JobVariable containing user overwritten memory limit
	MemoryLimitOverwriteVariableValue = "KUBERNETES_MEMORY_LIMIT"
	// MemoryRequestOverwriteVariableValue is the key for the JobVariable containing user overwritten memory limit
	MemoryRequestOverwriteVariableValue = "KUBERNETES_MEMORY_REQUEST"
	// EphemeralStorageLimitOverwriteVariableValue is the key for the JobVariable containing user overwritten
	// ephemeral storage limit
	EphemeralStorageLimitOverwriteVariableValue = "KUBERNETES_EPHEMERAL_STORAGE_LIMIT"
	// EphemeralStorageRequestOverwriteVariableValue is the key for the JobVariable containing user overwritten
	// ephemeral storage limit
	EphemeralStorageRequestOverwriteVariableValue = "KUBERNETES_EPHEMERAL_STORAGE_REQUEST"
	// ServiceCPULimitOverwriteVariableValue is the key for the JobVariable containing user overwritten service cpu
	// limit
	ServiceCPULimitOverwriteVariableValue = "KUBERNETES_SERVICE_CPU_LIMIT"
	// ServiceCPURequestOverwriteVariableValue is the key for the JobVariable containing user overwritten service cpu
	// limit
	ServiceCPURequestOverwriteVariableValue = "KUBERNETES_SERVICE_CPU_REQUEST"
	// ServiceMemoryLimitOverwriteVariableValue is the key for the JobVariable containing user overwritten service
	// memory limit
	ServiceMemoryLimitOverwriteVariableValue = "KUBERNETES_SERVICE_MEMORY_LIMIT"
	// ServiceMemoryRequestOverwriteVariableValue is the key for the JobVariable containing user overwritten service
	// memory limit
	ServiceMemoryRequestOverwriteVariableValue = "KUBERNETES_SERVICE_MEMORY_REQUEST"
	// ServiceEphemeralStorageLimitOverwriteVariableValue is the key for the JobVariable containing user overwritten
	// service ephemeral storage
	ServiceEphemeralStorageLimitOverwriteVariableValue = "KUBERNETES_SERVICE_EPHEMERAL_STORAGE_LIMIT"
	// ServiceEphemeralStorageRequestOverwriteVariableValue is the key for the JobVariable containing user overwritten
	// service ephemeral storage
	ServiceEphemeralStorageRequestOverwriteVariableValue = "KUBERNETES_SERVICE_EPHEMERAL_STORAGE_REQUEST"
	// HelperCPULimitOverwriteVariableValue is the key for the JobVariable containing user overwritten helper cpu limit
	HelperCPULimitOverwriteVariableValue = "KUBERNETES_HELPER_CPU_LIMIT"
	// HelperCPURequestOverwriteVariableValue is the key for the JobVariable containing user overwritten helper cpu
	// limit
	HelperCPURequestOverwriteVariableValue = "KUBERNETES_HELPER_CPU_REQUEST"
	// HelperMemoryLimitOverwriteVariableValue is the key for the JobVariable containing user overwritten helper memory
	// limit
	HelperMemoryLimitOverwriteVariableValue = "KUBERNETES_HELPER_MEMORY_LIMIT"
	// HelperMemoryRequestOverwriteVariableValue is the key for the JobVariable containing user overwritten helper
	// memory limit
	HelperEphemeralStorageRequestOverwriteVariableValue = "KUBERNETES_HELPER_EPHEMERAL_STORAGE_REQUEST"
	// HelperEphemeralStorageLimitOverwriteVariableValue is the key for the JobVariable containing user overwritten
	// helper ephemeral storage
	HelperEphemeralStorageLimitOverwriteVariableValue = "KUBERNETES_HELPER_EPHEMERAL_STORAGE_LIMIT"
	// HelperEphemeralStorageRequestOverwriteVariableValue is the key for the JobVariable containing user overwritten
	// ephemeral storage
	HelperMemoryRequestOverwriteVariableValue = "KUBERNETES_HELPER_MEMORY_REQUEST"
)

type overwriteTooHighError struct {
	resource  string
	max       string
	overwrite string
}

func (o *overwriteTooHighError) Error() string {
	return fmt.Sprintf("the resource %q requested %q is higher than limit allowed %q", o.resource, o.overwrite, o.max)
}

func (o *overwriteTooHighError) Is(err error) bool {
	_, ok := err.(*overwriteTooHighError)
	return ok
}

type malformedOverwriteError struct {
	value   string
	pattern string
}

func (m *malformedOverwriteError) Error() string {
	return fmt.Sprintf("provided value %q does not match %q", m.value, m.pattern)
}

func (m *malformedOverwriteError) Is(err error) bool {
	_, ok := err.(*malformedOverwriteError)
	return ok
}

type overwrites struct {
	namespace      string
	serviceAccount string
	bearerToken    string
	podLabels      map[string]string
	podAnnotations map[string]string
	nodeSelector   map[string]string

	buildLimits     api.ResourceList
	serviceLimits   api.ResourceList
	helperLimits    api.ResourceList
	buildRequests   api.ResourceList
	serviceRequests api.ResourceList
	helperRequests  api.ResourceList
}

//nolint:funlen
func createOverwrites(
	config *common.KubernetesConfig,
	variables common.JobVariables,
	logger common.BuildLogger,
) (*overwrites, error) {
	var err error
	o := &overwrites{}

	variables = variables.Expand()

	namespaceOverwrite := variables.Get(NamespaceOverwriteVariableName)
	o.namespace, err = o.evaluateOverwrite(
		"Namespace",
		config.Namespace,
		config.NamespaceOverwriteAllowed,
		namespaceOverwrite,
		logger,
	)
	if err != nil {
		return nil, err
	}

	serviceAccountOverwrite := variables.Get(ServiceAccountOverwriteVariableName)
	o.serviceAccount, err = o.evaluateOverwrite(
		"ServiceAccount",
		config.ServiceAccount,
		config.ServiceAccountOverwriteAllowed,
		serviceAccountOverwrite,
		logger,
	)
	if err != nil {
		return nil, err
	}

	bearerTokenOverwrite := variables.Get(BearerTokenOverwriteVariableValue)
	o.bearerToken, err = o.evaluateBoolControlledOverwrite(
		"BearerToken",
		config.BearerToken,
		config.BearerTokenOverwriteAllowed,
		bearerTokenOverwrite,
		logger,
	)
	if err != nil {
		return nil, err
	}

	o.podLabels, err = o.evaluateMapOverwrite(
		"PodLabels",
		config.PodLabels,
		config.PodLabelsOverwriteAllowed,
		variables,
		PodLabelsOverwriteVariablePrefix,
		logger,
	)
	if err != nil {
		return nil, err
	}

	o.podAnnotations, err = o.evaluateMapOverwrite(
		"PodAnnotations",
		config.PodAnnotations,
		config.PodAnnotationsOverwriteAllowed,
		variables,
		PodAnnotationsOverwriteVariablePrefix,
		logger,
	)
	if err != nil {
		return nil, err
	}

	o.nodeSelector, err = o.evaluateMapOverwrite(
		"NodeSelector",
		config.NodeSelector,
		config.NodeSelectorOverwriteAllowed,
		variables,
		NodeSelectorOverwriteVariablePrefix,
		logger,
	)
	if err != nil {
		return nil, err
	}

	err = o.evaluateMaxBuildResourcesOverwrite(config, variables, logger)
	if err != nil {
		return nil, err
	}

	err = o.evaluateMaxServiceResourcesOverwrite(config, variables, logger)
	if err != nil {
		return nil, err
	}

	err = o.evaluateMaxHelperResourcesOverwrite(config, variables, logger)
	if err != nil {
		return nil, err
	}

	return o, nil
}

func (o *overwrites) evaluateMaxBuildResourcesOverwrite(
	config *common.KubernetesConfig,
	variables common.JobVariables,
	logger common.BuildLogger,
) (err error) {
	o.buildRequests, err = o.evaluateMaxResourceListOverwrite(
		"CPURequest",
		"MemoryRequest",
		"EphemeralStorageRequest",
		config.CPURequest,
		config.MemoryRequest,
		config.EphemeralStorageRequest,
		config.CPURequestOverwriteMaxAllowed,
		config.MemoryRequestOverwriteMaxAllowed,
		config.EphemeralStorageRequestOverwriteMaxAllowed,
		variables.Value(CPURequestOverwriteVariableValue),
		variables.Value(MemoryRequestOverwriteVariableValue),
		variables.Value(EphemeralStorageRequestOverwriteVariableValue),
		logger,
	)
	if err != nil {
		return fmt.Errorf("invalid build requests specified: %w", err)
	}

	o.buildLimits, err = o.evaluateMaxResourceListOverwrite(
		"CPULimit",
		"MemoryLimit",
		"EphemeralStorageLimit",
		config.CPULimit,
		config.MemoryLimit,
		config.EphemeralStorageLimit,
		config.CPULimitOverwriteMaxAllowed,
		config.MemoryLimitOverwriteMaxAllowed,
		config.EphemeralStorageLimitOverwriteMaxAllowed,
		variables.Value(CPULimitOverwriteVariableValue),
		variables.Value(MemoryLimitOverwriteVariableValue),
		variables.Value(EphemeralStorageLimitOverwriteVariableValue),
		logger,
	)
	if err != nil {
		return fmt.Errorf("invalid build limits specified: %w", err)
	}

	return nil
}

func (o *overwrites) evaluateMaxServiceResourcesOverwrite(
	config *common.KubernetesConfig,
	variables common.JobVariables,
	logger common.BuildLogger,
) (err error) {
	o.serviceRequests, err = o.evaluateMaxResourceListOverwrite(
		"ServiceCPURequest",
		"ServiceMemoryRequest",
		"ServiceEphemeralStorageRequest",
		config.ServiceCPURequest,
		config.ServiceMemoryRequest,
		config.ServiceEphemeralStorageRequest,
		config.ServiceCPURequestOverwriteMaxAllowed,
		config.ServiceMemoryRequestOverwriteMaxAllowed,
		config.ServiceEphemeralStorageRequestOverwriteMaxAllowed,
		variables.Value(ServiceCPURequestOverwriteVariableValue),
		variables.Value(ServiceMemoryRequestOverwriteVariableValue),
		variables.Value(ServiceEphemeralStorageRequestOverwriteVariableValue),
		logger,
	)
	if err != nil {
		return fmt.Errorf("invalid service requests specified: %w", err)
	}

	o.serviceLimits, err = o.evaluateMaxResourceListOverwrite(
		"ServiceCPULimit",
		"ServiceMemoryLimit",
		"ServiceEphemeralStorageLimit",
		config.ServiceCPULimit,
		config.ServiceMemoryLimit,
		config.ServiceEphemeralStorageLimit,
		config.ServiceCPULimitOverwriteMaxAllowed,
		config.ServiceMemoryLimitOverwriteMaxAllowed,
		config.ServiceEphemeralStorageLimitOverwriteMaxAllowed,
		variables.Value(ServiceCPULimitOverwriteVariableValue),
		variables.Value(ServiceMemoryLimitOverwriteVariableValue),
		variables.Value(ServiceEphemeralStorageLimitOverwriteVariableValue),
		logger,
	)
	if err != nil {
		return fmt.Errorf("invalid service limits specified: %w", err)
	}

	return nil
}

func (o *overwrites) evaluateMaxHelperResourcesOverwrite(
	config *common.KubernetesConfig,
	variables common.JobVariables,
	logger common.BuildLogger,
) (err error) {
	o.helperRequests, err = o.evaluateMaxResourceListOverwrite(
		"HelperCPURequest",
		"HelperMemoryRequest",
		"HelperEphemeralStorageRequest",
		config.HelperCPURequest,
		config.HelperMemoryRequest,
		config.HelperEphemeralStorageRequest,
		config.HelperCPURequestOverwriteMaxAllowed,
		config.HelperMemoryRequestOverwriteMaxAllowed,
		config.HelperEphemeralStorageRequestOverwriteMaxAllowed,
		variables.Value(HelperCPURequestOverwriteVariableValue),
		variables.Value(HelperMemoryRequestOverwriteVariableValue),
		variables.Value(HelperEphemeralStorageRequestOverwriteVariableValue),
		logger,
	)
	if err != nil {
		return fmt.Errorf("invalid helper requests specified: %w", err)
	}

	o.helperLimits, err = o.evaluateMaxResourceListOverwrite(
		"HelperCPULimit",
		"HelperMemoryLimit",
		"HelperEphemeralStorageLimit",
		config.HelperCPULimit,
		config.HelperMemoryLimit,
		config.HelperEphemeralStorageLimit,
		config.HelperCPULimitOverwriteMaxAllowed,
		config.HelperMemoryLimitOverwriteMaxAllowed,
		config.HelperEphemeralStorageLimitOverwriteMaxAllowed,
		variables.Value(HelperCPULimitOverwriteVariableValue),
		variables.Value(HelperMemoryLimitOverwriteVariableValue),
		variables.Value(HelperEphemeralStorageLimitOverwriteVariableValue),
		logger,
	)
	if err != nil {
		return fmt.Errorf("invalid helper limits specified: %w", err)
	}

	return nil
}

func (o *overwrites) evaluateBoolControlledOverwrite(
	fieldName, value string,
	canOverride bool,
	overwriteValue string,
	logger common.BuildLogger,
) (string, error) {
	if canOverride {
		return o.evaluateOverwrite(fieldName, value, ".+", overwriteValue, logger)
	}
	return o.evaluateOverwrite(fieldName, value, "", overwriteValue, logger)
}

func (o *overwrites) evaluateOverwrite(
	fieldName, value, regex, overwriteValue string,
	logger common.BuildLogger,
) (string, error) {
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
		return &malformedOverwriteError{value: value, pattern: regex}
	}
	return nil
}

// splitMapOverwrite splits provided string on the first "=" and returns (key, value, nil).
// If the argument cannot be split an error is returned
func splitMapOverwrite(str string) (string, string, error) {
	if split := strings.SplitN(str, "=", 2); len(split) > 1 {
		return split[0], split[1], nil
	}

	return "", "", &malformedOverwriteError{value: str, pattern: "k=v"}
}

func (o *overwrites) evaluateMapOverwrite(
	fieldName string,
	values map[string]string,
	regex string,
	variables common.JobVariables,
	variablesSelector string,
	logger common.BuildLogger,
) (map[string]string, error) {
	if regex == "" {
		logger.Debugln("Regex allowing overrides for", fieldName, "is empty, disabling override.")
		return values, nil
	}

	finalValues := make(map[string]string)
	for k, v := range values {
		finalValues[k] = v
	}

	for _, variable := range variables {
		if !strings.HasPrefix(variable.Key, variablesSelector) {
			continue
		}

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
	return finalValues, nil
}

func (o *overwrites) evaluateMaxResourceListOverwrite(
	cpuFieldName,
	memoryFieldName,
	ephemeralStorageFieldName,
	currentCPU,
	currentMemory,
	currentEphemeralStorage,
	maxCPU,
	maxMemory,
	maxEphemeralStorage,
	overwriteCPU,
	overwriteMemory string,
	overwriteEphemeralStorage string,
	logger common.BuildLogger,
) (api.ResourceList, error) {
	cpu, err := o.evaluateMaxResourceOverwrite(cpuFieldName, currentCPU, maxCPU, overwriteCPU, logger)
	if err != nil {
		return nil, err
	}

	memory, err := o.evaluateMaxResourceOverwrite(memoryFieldName, currentMemory, maxMemory, overwriteMemory, logger)
	if err != nil {
		return nil, err
	}

	ephemeralStorage, err := o.evaluateMaxResourceOverwrite(
		ephemeralStorageFieldName,
		currentEphemeralStorage,
		maxEphemeralStorage,
		overwriteEphemeralStorage,
		logger,
	)
	if err != nil {
		return nil, err
	}

	return createResourceList(cpu, memory, ephemeralStorage)
}

func (o *overwrites) evaluateMaxResourceOverwrite(
	fieldName,
	value,
	maxResource,
	overwriteValue string,
	logger common.BuildLogger,
) (string, error) {
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
		return value, fmt.Errorf("parsing resource limit: %q", err.Error())
	}

	if rOverwriteValue, err = resource.ParseQuantity(overwriteValue); err != nil {
		return value, fmt.Errorf("parsing resource limit: %q", err.Error())
	}

	cmp := rOverwriteValue.Cmp(rMaxResource)
	if cmp == 1 {
		return "", &overwriteTooHighError{
			resource:  fieldName,
			max:       maxResource,
			overwrite: overwriteValue,
		}
	}

	logger.Println(fmt.Sprintf("%q overwritten with %q", fieldName, overwriteValue))

	return overwriteValue, nil
}
